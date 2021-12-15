/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	//fedv1b1 "sigs.k8s.io/kubefed/pkg/apis/core/v1beta1"

	// "k8s.io/apimachinery/pkg/util/intstr"

	"github.com/go-logr/logr"
	certmanagerv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	servicecatalogv1beta1 "github.com/kubernetes-sigs/service-catalog/pkg/apis/servicecatalog/v1beta1"
	clusterv1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/cluster/v1alpha1"
	"github.com/tmax-cloud/hypercloud-multi-operator/controllers/util"
	traefikv2 "github.com/traefik/traefik/v2/pkg/provider/kubernetes/crd/traefik/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	capiv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sigs.k8s.io/yaml"
)

const (
	// CAPI_SYSTEM_NAMESPACE       = "capi-system"
	// CLAIM_API_GROUP             = "claim.tmax.io"
	// CLUSTER_API_GROUP           = "cluster.tmax.io"
	// CLAIM_API_Kind              = "clusterclaims"
	// CLAIM_API_GROUP_VERSION     = "claim.tmax.io/v1alpha1"
	// HYPERCLOUD_SYSTEM_NAMESPACE = ""
	requeueAfter10Sec  = 10 * time.Second
	requeueAfter20Sec  = 20 * time.Second
	requeueAfter30Sec  = 30 * time.Second
	requeueAfter60Sec  = 60 * time.Second
	requeueAfter120Sec = 120 * time.Second
)

type ClusterParameter struct {
	Namespace         string
	ClusterName       string
	MasterNum         int
	WorkerNum         int
	Owner             string
	KubernetesVersion string
}

// type AwsParameter struct {
// 	SshKey     string
// 	Region     string
// 	MasterType string
// 	WorkerType string
// }

// type VsphereParameter struct {
// 	PodCidr             string
// 	VcenterIp           string
// 	VcenterId           string
// 	VcenterPassword     string
// 	VcenterThumbprint   string
// 	VcenterNetwork      string
// 	VcenterDataCenter   string
// 	VcenterDataStore    string
// 	VcenterFolder       string
// 	VcenterResourcePool string
// 	VcenterKcpIp        string
// 	VcenterCpuNum       int
// 	VcenterMemSize      int
// 	VcenterDiskSize     int
// 	VcenterTemplate     string
// }

// ClusterManagerReconciler reconciles a ClusterManager object
type ClusterManagerReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=cluster.tmax.io,resources=clustermanagers,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=cluster.tmax.io,resources=clustermanagers/status,verbs=get;list;patch;update;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machinedeployments,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machinedeployments/status,verbs=get;list;patch;update;watch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings;roles;rolebindings,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=kubeadmcontrolplanes,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=kubeadmcontrolplanes/status,verbs=get;list;patch;update;watch
// +kubebuilder:rbac:groups=servicecatalog.k8s.io,resources=serviceinstances,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=servicecatalog.k8s.io,resources=serviceinstances/status,verbs=get;list;patch;update;watch
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups="",resources=services;endpoints,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=traefik.containo.us,resources=middlewares,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;update;watch

func (r *ClusterManagerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	_ = context.Background()
	log := r.Log.WithValues("clustermanager", req.NamespacedName)

	//get ClusterManager
	clusterManager := &clusterv1alpha1.ClusterManager{}
	if err := r.Get(context.TODO(), req.NamespacedName, clusterManager); err != nil {
		if errors.IsNotFound(err) {
			log.Info("ClusterManager resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get ClusterManager")
		return ctrl.Result{}, err
	}

	//set patch helper
	patchHelper, err := patch.NewHelper(clusterManager, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	defer func() {
		// Always reconcile the Status.Phase field.
		r.reconcilePhase(context.TODO(), clusterManager)

		if err := patchHelper.Patch(context.TODO(), clusterManager); err != nil {
			// if err := patchClusterManager(context.TODO(), patchHelper, clusterManager, patchOpts...); err != nil {
			// reterr = kerrors.NewAggregate([]error{reterr, err})
			reterr = err
		}
	}()

	// Add finalizer first if not exist to avoid the race condition between init and delete
	if !controllerutil.ContainsFinalizer(clusterManager, util.ClusterManagerFinalizer) {
		controllerutil.AddFinalizer(clusterManager, util.ClusterManagerFinalizer)
		return ctrl.Result{}, nil
	}

	// Handle normal reconciliation loop.
	if clusterManager.Labels[util.ClusterTypeKey] == util.ClusterTypeRegistered {
		// Handle deletion reconciliation loop.
		if !clusterManager.ObjectMeta.DeletionTimestamp.IsZero() {
			clusterManager.Status.Ready = false
			return r.reconcileDeleteForRegisteredClusterManager(context.TODO(), clusterManager)
		}
		return r.reconcileForRegisteredClusterManager(context.TODO(), clusterManager)
	}

	// Handle deletion reconciliation loop.
	if !clusterManager.ObjectMeta.DeletionTimestamp.IsZero() {
		clusterManager.Status.Ready = false
		return r.reconcileDelete(context.TODO(), clusterManager)
	}

	return r.reconcile(context.TODO(), clusterManager)
}

// reconcile handles cluster reconciliation.
func (r *ClusterManagerReconciler) reconcileForRegisteredClusterManager(ctx context.Context, clusterManager *clusterv1alpha1.ClusterManager) (ctrl.Result, error) {
	phases := []func(context.Context, *clusterv1alpha1.ClusterManager) (ctrl.Result, error){
		r.UpdateClusterManagerStatus,
		r.CreateTraefikResources,
		// r.DeployAndUpdateAgentEndpoint,
	}

	res := ctrl.Result{}
	errs := []error{}
	for _, phase := range phases {
		// Call the inner reconciliation methods.
		phaseResult, err := phase(ctx, clusterManager)
		if err != nil {
			errs = append(errs, err)
		}
		if len(errs) > 0 {
			continue
		}
		res = util.LowestNonZeroResult(res, phaseResult)
	}
	return res, kerrors.NewAggregate(errs)
}

func (r *ClusterManagerReconciler) UpdateClusterManagerStatus(ctx context.Context, clusterManager *clusterv1alpha1.ClusterManager) (ctrl.Result, error) {
	log := r.Log.WithValues("clustermanager", types.NamespacedName{Name: clusterManager.Name, Namespace: clusterManager.Namespace})
	log.Info("Start to UpdateClusterManagerStatus")

	kubeconfigSecret := &corev1.Secret{}
	kubeconfigSecretKey := types.NamespacedName{
		Name:      clusterManager.Name + util.KubeconfigPostfix,
		Namespace: clusterManager.Namespace,
	}
	if err := r.Get(context.TODO(), kubeconfigSecretKey, kubeconfigSecret); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Cannot found kubeconfig secret. Wait to create kubeconfig secret.")
			return ctrl.Result{RequeueAfter: requeueAfter10Sec}, nil
		} else {
			log.Error(err, "Failed to get kubeconfig secret")
			return ctrl.Result{}, err
		}
	}

	remoteClientset, err := util.GetRemoteK8sClient(kubeconfigSecret)
	if err != nil {
		log.Error(err, "Failed to get remoteK8sClient")
		return ctrl.Result{}, err
	}

	var kubeadmConfig *corev1.ConfigMap
	if kubeadmConfig, err = remoteClientset.CoreV1().ConfigMaps("kube-system").Get(context.TODO(), "kubeadm-config", metav1.GetOptions{}); err != nil {
		log.Error(err, "Failed to get kubeadm-config configmap from remote cluster")
		return ctrl.Result{}, err
	}

	jsonData, _ := yaml.YAMLToJSON([]byte(kubeadmConfig.Data["ClusterConfiguration"]))
	data := make(map[string]interface{})
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return ctrl.Result{}, err
	}

	clusterManager.Spec.Version = fmt.Sprintf("%v", data["kubernetesVersion"])

	var nodeList *corev1.NodeList
	if nodeList, err = remoteClientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{}); err != nil {
		log.Error(err, "Failed to list remote K8s nodeList")
		return ctrl.Result{}, err
	}

	// var machineList *capiv1.machineList
	// if machineList, err =
	// todo - shkim
	// node list가 아닌 machine list를 불러서 ready체크를 해야 확실하지 않을까?
	clusterManager.Spec.MasterNum = 0
	clusterManager.Status.MasterRun = 0
	clusterManager.Spec.WorkerNum = 0
	clusterManager.Status.WorkerRun = 0
	for _, node := range nodeList.Items {
		if _, ok := node.Labels["node-role.kubernetes.io/master"]; ok {
			clusterManager.Spec.MasterNum++
			if node.Status.Conditions[len(node.Status.Conditions)-1].Type == "Ready" {
				clusterManager.Status.MasterRun++
			}
		} else {
			clusterManager.Spec.WorkerNum++
			if node.Status.Conditions[len(node.Status.Conditions)-1].Type == "Ready" {
				clusterManager.Status.WorkerRun++
			}
		}
		clusterManager.Status.Provider = node.Spec.ProviderID
		clusterManager.Spec.Provider = node.Spec.ProviderID
	}
	if clusterManager.Status.Provider == "" {
		clusterManager.Spec.Provider = "Unknown"
		clusterManager.Status.Provider = "Unknown"
	}

	// health check

	var resp []byte
	if resp, err = remoteClientset.RESTClient().Get().AbsPath("/readyz").DoRaw(context.TODO()); err != nil {
		log.Error(err, "Failed to get remote cluster status")
		return ctrl.Result{}, err
	}
	if string(resp) == "ok" {
		clusterManager.Status.ControlPlaneReady = true
		//clusterManager.Status.AgentReady = true
		clusterManager.Status.Ready = true
	} else {
		// err := errors.NewBadRequest("Failed to healthcheck")
		// log.Error(err, "Failed to healthcheck")
		// 잠시 오류난걸 수도 있으니까.. 근데 무한장 wait할 수는 없어서 requeue 횟수를 지정할 수 있으면 좋겠네
		log.Info("Remote cluster is not ready... watit...")
		return ctrl.Result{RequeueAfter: requeueAfter30Sec}, nil
	}

	generatedSuffix := util.CreateSuffixString()
	clusterManager.Annotations["suffix"] = generatedSuffix
	return ctrl.Result{}, nil
}

func (r *ClusterManagerReconciler) SetEndpoint(ctx context.Context, clusterManager *clusterv1alpha1.ClusterManager) (ctrl.Result, error) {
	log := r.Log.WithValues("clustermanager", types.NamespacedName{Name: clusterManager.Name, Namespace: clusterManager.Namespace})
	log.Info("Start setting endpoint configuration to clusterManager")

	if clusterManager.Annotations["Endpoint"] != "" {
		log.Info("Endpoint already configured.")
		return ctrl.Result{}, nil
	}

	cluster := &capiv1.Cluster{}
	clusterKey := types.NamespacedName{
		Name:      clusterManager.Name,
		Namespace: clusterManager.Namespace,
	}
	if err := r.Get(context.TODO(), clusterKey, cluster); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Fail to get cluster. Requeue after 20sec.")
			return ctrl.Result{RequeueAfter: requeueAfter20Sec}, err
		} else {
			log.Error(err, "Failed to get cluster information")
			return ctrl.Result{}, err
		}
	}

	if cluster.Spec.ControlPlaneEndpoint.Host == "" {
		log.Info("ControlPlain endpoint is not ready yet. requeue after 20sec.")
		return ctrl.Result{RequeueAfter: requeueAfter20Sec}, nil
	}
	clusterManager.Annotations["Endpoint"] = cluster.Spec.ControlPlaneEndpoint.Host

	return ctrl.Result{}, nil
}
func (r *ClusterManagerReconciler) CreateTraefikResources(ctx context.Context, clusterManager *clusterv1alpha1.ClusterManager) (ctrl.Result, error) {
	log := r.Log.WithValues("clustermanager", types.NamespacedName{Name: clusterManager.Name, Namespace: clusterManager.Namespace})

	if clusterManager.Annotations["Endpoint"] == "" {
		log.Info("Wati for recognize remote apiserver endpoint")
		return ctrl.Result{RequeueAfter: requeueAfter20Sec}, nil
	}

	traefikCertificate := &certmanagerv1.Certificate{}
	certificateKey := types.NamespacedName{
		//Name:      clusterManager.Name + "-certificate-" + clusterManager.Annotations["suffix"],
		Name:      clusterManager.Name + "-certificate",
		Namespace: clusterManager.Namespace,
	}
	if err := r.Get(context.TODO(), certificateKey, traefikCertificate); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Start creating Certificate")
			traefikCertificate = util.CreateCertificate(clusterManager)
			if objCreateErr := r.Create(context.TODO(), traefikCertificate); objCreateErr != nil {
				log.Error(objCreateErr, "Failed to create Certificate")
				return ctrl.Result{}, objCreateErr
			}
			ctrl.SetControllerReference(clusterManager, traefikCertificate, r.Scheme)
		} else {
			log.Error(err, "Failed to get Certificate information")
			return ctrl.Result{}, err
		}
	} else {
		log.Info("Certificate is already existed")
	}

	traefikIngress := &networkingv1.Ingress{}
	ingressKey := types.NamespacedName{
		//Name:      clusterManager.Name + "-ingress-" + clusterManager.Annotations["suffix"],
		Name:      clusterManager.Name + "-ingress",
		Namespace: clusterManager.Namespace,
	}
	if err := r.Get(context.TODO(), ingressKey, traefikIngress); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Start creating Ingress")
			traefikIngress = util.CreateIngress(clusterManager)
			if objCreateErr := r.Create(context.TODO(), traefikIngress); objCreateErr != nil {
				log.Error(objCreateErr, "Failed to create Ingress")
				return ctrl.Result{}, objCreateErr
			}
			ctrl.SetControllerReference(clusterManager, traefikIngress, r.Scheme)
		} else {
			log.Error(err, "Failed to get Ingress information")
			return ctrl.Result{}, err
		}
	} else {
		log.Info("Ingress is already existed")
	}

	traefikService := &corev1.Service{}
	serviceKey := types.NamespacedName{
		//Name:      clusterManager.Name + "-service-" + clusterManager.Annotations["suffix"],
		Name:      clusterManager.Name + "-service",
		Namespace: clusterManager.Namespace,
	}
	if err := r.Get(context.TODO(), serviceKey, traefikService); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Start creating Service")
			traefikService = util.CreateService(clusterManager)
			if objCreateErr := r.Create(context.TODO(), traefikService); objCreateErr != nil {
				log.Error(objCreateErr, "Failed to create Service")
				return ctrl.Result{}, objCreateErr
			}
			ctrl.SetControllerReference(clusterManager, traefikService, r.Scheme)
		} else {
			log.Error(err, "Failed to get Service information")
			return ctrl.Result{}, err
		}
	} else {
		log.Info("Service is already existed")
	}

	if strings.ToUpper(clusterManager.Spec.Provider) == util.PROVIDER_VSPHERE {
		traefikEndpoint := &corev1.Endpoints{}
		endpointKey := types.NamespacedName{
			// Name: clusterManager.Name + "-endpoint" + clusterManager.Annotations["suffix"],
			Name:      clusterManager.Name + "-endpoint",
			Namespace: clusterManager.Namespace,
		}
		if err := r.Get(context.TODO(), endpointKey, traefikEndpoint); err != nil {
			if errors.IsNotFound(err) {
				log.Info("Start creating endpoint")
				traefikEndpoint = util.CreateEndpoint(clusterManager)
				if objCreateErr := r.Create(context.TODO(), traefikEndpoint); objCreateErr != nil {
					log.Error(objCreateErr, "Failed to create Endpoint")
					return ctrl.Result{}, objCreateErr
				}
				ctrl.SetControllerReference(clusterManager, traefikEndpoint, r.Scheme)
			} else {
				log.Error(err, "Failed to get Endpoint information")
				return ctrl.Result{}, err
			}
		} else {
			log.Info("Endpoint is already existed")
		}
	}

	traefikMiddleware := &traefikv2.Middleware{}
	middlewareKey := types.NamespacedName{
		//Name:      clusterManager.Name + "-prefix-" + clusterManager.Annotations["suffix"],
		Name:      clusterManager.Name + "-prefix",
		Namespace: clusterManager.Namespace,
	}
	if err := r.Get(context.TODO(), middlewareKey, traefikMiddleware); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Start creating Middleware")
			traefikMiddleware = util.CreateMiddleware(clusterManager)
			if objCreateErr := r.Create(context.TODO(), traefikMiddleware); objCreateErr != nil {
				log.Error(objCreateErr, "Failed to create Middleware")
				return ctrl.Result{}, objCreateErr
			}
			ctrl.SetControllerReference(clusterManager, traefikMiddleware, r.Scheme)
		} else {
			log.Error(err, "Failed to get Middleware information")
			return ctrl.Result{}, err
		}
	} else {
		log.Info("Middleware is already existed")
	}

	return ctrl.Result{}, nil
}

func (r *ClusterManagerReconciler) DeployAndUpdateAgentEndpoint(ctx context.Context, clusterManager *clusterv1alpha1.ClusterManager) (ctrl.Result, error) {
	log := r.Log.WithValues("clustermanager", types.NamespacedName{Name: clusterManager.Name, Namespace: clusterManager.Namespace})
	// secret controller에서 clustermanager.status.controleplaneendpoint를 채워줄 때 까지 기다림
	if !clusterManager.Status.ControlPlaneReady {
		// requeue (wait cluster controller)
		return ctrl.Result{RequeueAfter: requeueAfter60Sec}, nil
	} else /*if !clusterManager.Status.AgentReady*/ {
		kubeconfigSecret := &corev1.Secret{}
		kubeconfigSecretKey := types.NamespacedName{
			Name:      clusterManager.Name + util.KubeconfigPostfix,
			Namespace: clusterManager.Namespace,
		}
		if err := r.Get(context.TODO(), kubeconfigSecretKey, kubeconfigSecret); err != nil {
			if errors.IsNotFound(err) {
				log.Info("Cannot found kubeconfig secret. Wait to create kubeconfig secret.")
				return ctrl.Result{RequeueAfter: requeueAfter10Sec}, nil
			} else {
				log.Error(err, "Failed to get kubeconfig secret")
				return ctrl.Result{}, err
			}
		}

		remoteClientset, err := util.GetRemoteK8sClient(kubeconfigSecret)
		if err != nil {
			log.Error(err, "Failed to get remoteK8sClient")
			return ctrl.Result{}, err
		}

		// ingress controller 존재하는지 먼저 확인하고 없으면 배포부터해.. 그전에 join되었는지도 먼저 확인해야하나...
		if _, err = remoteClientset.CoreV1().Namespaces().Get(context.TODO(), util.IngressNginxNamespace, metav1.GetOptions{}); err != nil {
			if errors.IsNotFound(err) {
				log.Info("Cannot found ingress namespace... ingress-nginx is creating... requeue after 30sec")
				return ctrl.Result{RequeueAfter: requeueAfter60Sec}, nil
			} else {
				log.Error(err, "Failed to get ingress-nginx namespace from remote cluster")
				return ctrl.Result{}, err
			}
		} else {
			var ingressController *appsv1.Deployment
			if ingressController, err = remoteClientset.AppsV1().Deployments(util.IngressNginxNamespace).Get(context.TODO(), util.IngressNginxName, metav1.GetOptions{}); err != nil {
				if errors.IsNotFound(err) {
					log.Info("Cannot found ingress controller... ingress-nginx is creating... requeue after 30sec")
					return ctrl.Result{RequeueAfter: requeueAfter60Sec}, nil
				} else {
					log.Error(err, "Failed to get ingress controller from remote cluster")
					return ctrl.Result{}, err
				}
			} else {
				// 하나라도 ready라면..
				if ingressController.Status.ReadyReplicas == 0 {
					log.Info("Ingress controller is not ready...  requeue after 60sec")
					return ctrl.Result{RequeueAfter: requeueAfter60Sec}, nil
				}
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *ClusterManagerReconciler) reconcileDeleteForRegisteredClusterManager(ctx context.Context, clusterManager *clusterv1alpha1.ClusterManager) (reconcile.Result, error) {
	log := r.Log.WithValues("clustermanager", types.NamespacedName{Name: clusterManager.Name, Namespace: clusterManager.Namespace})
	log.Info("Start to reconcileDeleteForRegisteredClusterManager reconcile for [" + clusterManager.Name + "]")

	kubeconfigSecret := &corev1.Secret{}
	kubeconfigSecretKey := types.NamespacedName{
		Name:      clusterManager.Name + util.KubeconfigPostfix,
		Namespace: clusterManager.Namespace,
	}
	if err := r.Get(context.TODO(), kubeconfigSecretKey, kubeconfigSecret); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Kubeconfig is succesefully deleted.. Delete clustermanager finalizer")
			if err := util.Delete(clusterManager.Namespace, clusterManager.Name); err != nil {
				log.Error(err, "Failed to delete cluster info from cluster_member table")
				return ctrl.Result{}, err
			}
		} else {
			log.Error(err, "Failed to get kubeconfig secret")
			return ctrl.Result{}, err
		}
	}

	// delete 를 계~~ 속 보내겠는데...?
	log.Info("Start to delete kubeconfig secret")
	if err := r.Delete(context.TODO(), kubeconfigSecret); err != nil {
		log.Error(err, "Failed to delete kubeconfigSecret")
		return ctrl.Result{}, err
	}

	traefikCertificate := &certmanagerv1.Certificate{}
	certificateKey := types.NamespacedName{
		//Name:      clusterManager.Name + "-certificate-" + clusterManager.Annotations["suffix"],
		Name:      clusterManager.Name + "-certificate",
		Namespace: clusterManager.Namespace,
	}
	if err := r.Get(context.TODO(), certificateKey, traefikCertificate); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Certificate is already deleted.")
			//if err := util.Delete(clusterManager.Namespace, clusterManager.)
		} else {
			log.Error(err, "Failed to get Certificate information")
			return ctrl.Result{}, err
		}
	} else {
		if objDeleteErr := r.Delete(context.TODO(), traefikCertificate); objDeleteErr != nil {
			log.Error(objDeleteErr, "Failed to delete Certificate")
			return ctrl.Result{}, objDeleteErr
		}
	}

	traefikCertSecret := &corev1.Secret{}
	certSecretKey := types.NamespacedName{
		//Name:      clusterManager.Name + "-certificate-" + clusterManager.Annotations["suffix"],
		Name:      clusterManager.Name + "-service-cert",
		Namespace: clusterManager.Namespace,
	}
	if err := r.Get(context.TODO(), certSecretKey, traefikCertSecret); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Cert-Secret is already deleted.")
			//if err := util.Delete(clusterManager.Namespace, clusterManager.)
		} else {
			log.Error(err, "Failed to get Cert-Secret information")
			return ctrl.Result{}, err
		}
	} else {
		if objDeleteErr := r.Delete(context.TODO(), traefikCertSecret); objDeleteErr != nil {
			log.Error(objDeleteErr, "Failed to delete Cert-Secret")
			return ctrl.Result{}, objDeleteErr
		}
	}

	traefikIngress := &networkingv1.Ingress{}
	ingressKey := types.NamespacedName{
		//Name:      clusterManager.Name + "-certificate-" + clusterManager.Annotations["suffix"],
		Name:      clusterManager.Name + "-ingress",
		Namespace: clusterManager.Namespace,
	}
	if err := r.Get(context.TODO(), ingressKey, traefikIngress); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Ingress is already deleted.")
			//if err := util.Delete(clusterManager.Namespace, clusterManager.)
		} else {
			log.Error(err, "Failed to get Certificate information")
			return ctrl.Result{}, err
		}
	} else {
		if objDeleteErr := r.Delete(context.TODO(), traefikIngress); objDeleteErr != nil {
			log.Error(objDeleteErr, "Failed to delete Ingress")
			return ctrl.Result{}, objDeleteErr
		}
	}

	traefikService := &corev1.Service{}
	serviceKey := types.NamespacedName{
		//Name:      clusterManager.Name + "-service-" + clusterManager.Annotations["suffix"],
		Name:      clusterManager.Name + "-service",
		Namespace: clusterManager.Namespace,
	}
	if err := r.Get(context.TODO(), serviceKey, traefikService); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Service is already deleted.")
			//if err := util.Delete(clusterManager.Namespace, clusterManager.)
		} else {
			log.Error(err, "Failed to get Service information")
			return ctrl.Result{}, err
		}
	} else {
		if objDeleteErr := r.Delete(context.TODO(), traefikService); objDeleteErr != nil {
			log.Error(objDeleteErr, "Failed to delete Service")
			return ctrl.Result{}, objDeleteErr
		}
	}

	traefikMiddleware := &traefikv2.Middleware{}
	middlewareKey := types.NamespacedName{
		//Name:      clusterManager.Name + "-prefix-" + clusterManager.Annotations["suffix"],
		Name:      clusterManager.Name + "-prefix",
		Namespace: clusterManager.Namespace,
	}
	if err := r.Get(context.TODO(), middlewareKey, traefikMiddleware); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Middleware is already deleted.")
		} else {
			log.Error(err, "Failed to get Middleware information")
			return ctrl.Result{}, err
		}
	} else {
		if objDeleteErr := r.Delete(context.TODO(), traefikMiddleware); objDeleteErr != nil {
			log.Error(objDeleteErr, "Failed to delete Service")
			return ctrl.Result{}, objDeleteErr
		}
	}

	controllerutil.RemoveFinalizer(clusterManager, util.ClusterManagerFinalizer)
	return ctrl.Result{}, nil
}

// reconcile handles cluster reconciliation.
func (r *ClusterManagerReconciler) reconcile(ctx context.Context, clusterManager *clusterv1alpha1.ClusterManager) (ctrl.Result, error) {
	phases := []func(context.Context, *clusterv1alpha1.ClusterManager) (ctrl.Result, error){
		r.CreateServiceInstance,
		r.SetEndpoint,
		r.CreateTraefikResources,
		r.DeployAndUpdateAgentEndpoint,
		r.kubeadmControlPlaneUpdate,
		r.machineDeploymentUpdate,
	}

	res := ctrl.Result{}
	errs := []error{}
	for _, phase := range phases {
		// Call the inner reconciliation methods.
		phaseResult, err := phase(ctx, clusterManager)
		if err != nil {
			errs = append(errs, err)
		}
		if len(errs) > 0 {
			continue
		}
		res = util.LowestNonZeroResult(res, phaseResult)
	}
	return res, kerrors.NewAggregate(errs)
}

func (r *ClusterManagerReconciler) reconcileDelete(ctx context.Context, clusterManager *clusterv1alpha1.ClusterManager) (reconcile.Result, error) {
	log := r.Log.WithValues("clustermanager", types.NamespacedName{Name: clusterManager.Name, Namespace: clusterManager.Namespace})

	kubeconfigSecret := &corev1.Secret{}
	kubeconfigSecretKey := types.NamespacedName{
		Name:      clusterManager.Name + util.KubeconfigPostfix,
		Namespace: clusterManager.Namespace,
	}
	if err := r.Get(context.TODO(), kubeconfigSecretKey, kubeconfigSecret); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Kubeconfig Secret is already deleted. Waiting cluster to be deleted...")
		} else {
			log.Error(err, "Failed to get kubeconfig secret")
			return ctrl.Result{}, err
		}
	} else {
		// clusterManagerNamespacedName := clusterManager.GetNamespace() + "-" + clusterManager.GetName()
		// kfc := &fedv1b1.KubeFedCluster{}
		// kfcKey := types.NamespacedName{Name: clusterManagerNamespacedName, Namespace: util.KubeFedNamespace}
		// if err := r.Get(context.TODO(), kfcKey, kfc); err != nil {
		// 	if errors.IsNotFound(err) {
		// 		log.Info("Cannot found kubefedCluster. Already unjoined")
		// 		// return ctrl.Result{}, nil
		// 	} else {
		// 		log.Error(err, "Failed to get kubefedCluster")
		// 		return ctrl.Result{}, err
		// 	}
		// } else if kfc.Status.Conditions[len(kfc.Status.Conditions)-1].Type == "Offline" {
		// 	// offline이면 kfc delete
		// 	log.Info("Cannot unjoin cluster.. because cluster is already delete.. delete directly kubefedcluster object")
		// 	if err := r.Delete(context.TODO(), kfc); err != nil {
		// 		log.Error(err, "Failed to delete kubefedCluster")
		// 		return ctrl.Result{}, err
		// 	}
		// } else {
		// 	clientRestConfig, err := getKubeConfig(*kubeconfigSecret)
		// 	if err != nil {
		// 		log.Error(err, "Unable to get rest config from secret")
		// 		return ctrl.Result{}, err
		// 	}
		// 	masterRestConfig := ctrl.GetConfigOrDie()
		// 	// cluster

		// 	kubefedConfig := &fedv1b1.KubeFedConfig{}
		// 	key := types.NamespacedName{Namespace: util.KubeFedNamespace, Name: "kubefed"}

		// 	if err := r.Get(context.TODO(), key, kubefedConfig); err != nil {
		// 		log.Error(err, "Failed to get kubefedconfig")
		// 		return ctrl.Result{}, err
		// 	}

		// 	if err := kubefedctl.UnjoinCluster(masterRestConfig, clientRestConfig,
		// 		util.KubeFedNamespace, util.HostClusterName, "", clusterManagerNamespacedName, false, false); err != nil {
		// 		log.Info("ClusterManager [" + strings.Split(kubeconfigSecret.Name, util.KubeconfigPostfix)[0] + "] is already unjoined... " + err.Error())
		// 	}
		// }

		traefikCertificate := &certmanagerv1.Certificate{}
		certificateKey := types.NamespacedName{
			//Name:      clusterManager.Name + "-certificate-" + clusterManager.Annotations["suffix"],
			Name:      clusterManager.Name + "-certificate",
			Namespace: clusterManager.Namespace,
		}
		if err := r.Get(context.TODO(), certificateKey, traefikCertificate); err != nil {
			if errors.IsNotFound(err) {
				log.Info("Certificate is already deleted.")
				//if err := util.Delete(clusterManager.Namespace, clusterManager.)
			} else {
				log.Error(err, "Failed to get Certificate information")
				return ctrl.Result{}, err
			}
		} else {
			if objDeleteErr := r.Delete(context.TODO(), traefikCertificate); objDeleteErr != nil {
				log.Error(objDeleteErr, "Failed to delete Certificate")
				return ctrl.Result{}, objDeleteErr
			}
		}

		traefikCertSecret := &corev1.Secret{}
		certSecretKey := types.NamespacedName{
			//Name:      clusterManager.Name + "-certificate-" + clusterManager.Annotations["suffix"],
			Name:      clusterManager.Name + "-service-cert",
			Namespace: clusterManager.Namespace,
		}
		if err := r.Get(context.TODO(), certSecretKey, traefikCertSecret); err != nil {
			if errors.IsNotFound(err) {
				log.Info("Cert-Secret is already deleted.")
				//if err := util.Delete(clusterManager.Namespace, clusterManager.)
			} else {
				log.Error(err, "Failed to get Cert-Secret information")
				return ctrl.Result{}, err
			}
		} else {
			if objDeleteErr := r.Delete(context.TODO(), traefikCertSecret); objDeleteErr != nil {
				log.Error(objDeleteErr, "Failed to delete Cert-Secret")
				return ctrl.Result{}, objDeleteErr
			}
		}

		traefikIngress := &networkingv1.Ingress{}
		ingressKey := types.NamespacedName{
			//Name:      clusterManager.Name + "-certificate-" + clusterManager.Annotations["suffix"],
			Name:      clusterManager.Name + "-ingress",
			Namespace: clusterManager.Namespace,
		}
		if err := r.Get(context.TODO(), ingressKey, traefikIngress); err != nil {
			if errors.IsNotFound(err) {
				log.Info("Ingress is already deleted.")
				//if err := util.Delete(clusterManager.Namespace, clusterManager.)
			} else {
				log.Error(err, "Failed to get Certificate information")
				return ctrl.Result{}, err
			}
		} else {
			if objDeleteErr := r.Delete(context.TODO(), traefikIngress); objDeleteErr != nil {
				log.Error(objDeleteErr, "Failed to delete Ingress")
				return ctrl.Result{}, objDeleteErr
			}
		}

		traefikService := &corev1.Service{}
		serviceKey := types.NamespacedName{
			//Name:      clusterManager.Name + "-service-" + clusterManager.Annotations["suffix"],
			Name:      clusterManager.Name + "-service",
			Namespace: clusterManager.Namespace,
		}
		if err := r.Get(context.TODO(), serviceKey, traefikService); err != nil {
			if errors.IsNotFound(err) {
				log.Info("Service is already deleted.")
				//if err := util.Delete(clusterManager.Namespace, clusterManager.)
			} else {
				log.Error(err, "Failed to get Service information")
				return ctrl.Result{}, err
			}
		} else {
			if objDeleteErr := r.Delete(context.TODO(), traefikService); objDeleteErr != nil {
				log.Error(objDeleteErr, "Failed to delete Service")
				return ctrl.Result{}, objDeleteErr
			}
		}

		traefikMiddleware := &traefikv2.Middleware{}
		middlewareKey := types.NamespacedName{
			//Name:      clusterManager.Name + "-prefix-" + clusterManager.Annotations["suffix"],
			Name:      clusterManager.Name + "-prefix",
			Namespace: clusterManager.Namespace,
		}
		if err := r.Get(context.TODO(), middlewareKey, traefikMiddleware); err != nil {
			if errors.IsNotFound(err) {
				log.Info("Middleware is already deleted.")
			} else {
				log.Error(err, "Failed to get Middleware information")
				return ctrl.Result{}, err
			}
		} else {
			if objDeleteErr := r.Delete(context.TODO(), traefikMiddleware); objDeleteErr != nil {
				log.Error(objDeleteErr, "Failed to delete Service")
				return ctrl.Result{}, objDeleteErr
			}
		}

		remoteClientset, err := util.GetRemoteK8sClient(kubeconfigSecret)
		if err != nil {
			log.Error(err, "Failed to get remoteK8sClient")
			return ctrl.Result{}, err
		}

		// secret은 존재하는데.. 실제 instance가 없어서 에러 발생
		if _, err = remoteClientset.CoreV1().Namespaces().Get(context.TODO(), util.IngressNginxNamespace, metav1.GetOptions{}); err != nil {
			if errors.IsNotFound(err) {
				log.Info("Ingress-nginx namespace is already deleted.")
			} else {
				log.Info(err.Error())
				log.Info("Failed to get Ingress-nginx loadbalancer service... may be instance was deleted before secret was deleted...")
				// log.Info("###################### Never excuted... ############################")
				// error 처리 필요
			}
		} else {
			if err := remoteClientset.CoreV1().Namespaces().Delete(context.TODO(), util.IngressNginxNamespace, metav1.DeleteOptions{}); err != nil {
				log.Error(err, "Failed to delete Ingress-nginx namespace")
				return ctrl.Result{}, err
			}
		}
	}

	// aaa
	// delete serviceinstance
	serviceInstance := &servicecatalogv1beta1.ServiceInstance{}
	serviceInstanceKey := types.NamespacedName{
		Name:      clusterManager.Name + "-" + clusterManager.Annotations["suffix"],
		Namespace: clusterManager.Namespace,
	}

	if err := r.Get(context.TODO(), serviceInstanceKey, serviceInstance); err != nil {
		if errors.IsNotFound(err) {
			log.Info("ServiceInstance is already deleted. Waiting cluster to be deleted")
		} else {
			log.Error(err, "Failed to get serviceInstance")
			return ctrl.Result{}, err
		}
	} else {
		if err := r.Delete(context.TODO(), serviceInstance); err != nil {
			log.Error(err, "Failed to delete serviceInstance")
			return ctrl.Result{}, err
		}
	}

	//delete handling
	cluster := &capiv1.Cluster{}
	clusterKey := types.NamespacedName{
		Name:      clusterManager.Name,
		Namespace: clusterManager.Namespace,
	}

	if err := r.Get(context.TODO(), clusterKey, cluster); err != nil {
		if errors.IsNotFound(err) {
			if err := util.Delete(clusterManager.Namespace, clusterManager.Name); err != nil {
				log.Error(err, "Failed to delete cluster info from cluster_member table")
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(clusterManager, util.ClusterManagerFinalizer)
			return ctrl.Result{}, nil
		} else {
			log.Error(err, "Failed to get cluster")
			return ctrl.Result{}, err
		}
	}
	log.Info("Cluster is deleteing... requeue request")
	return ctrl.Result{RequeueAfter: requeueAfter60Sec}, nil
}

func (r *ClusterManagerReconciler) reconcilePhase(_ context.Context, clusterManager *clusterv1alpha1.ClusterManager) {
	if clusterManager.Status.Phase == "" {
		if clusterManager.Labels[util.ClusterTypeKey] == util.ClusterTypeRegistered {
			clusterManager.Status.SetTypedPhase(clusterv1alpha1.ClusterManagerPhaseRegistering)
		} else {
			clusterManager.Status.SetTypedPhase(clusterv1alpha1.ClusterManagerPhaseProvisioning)
		}
	}

	if clusterManager.Status.Ready {
		if clusterManager.Labels[util.ClusterTypeKey] == util.ClusterTypeRegistered {
			clusterManager.Status.SetTypedPhase(clusterv1alpha1.ClusterManagerPhaseRegistered)
		} else {
			clusterManager.Status.SetTypedPhase(clusterv1alpha1.ClusterManagerPhaseProvisioned)
		}
	}

	if !clusterManager.DeletionTimestamp.IsZero() {
		clusterManager.Status.SetTypedPhase(clusterv1alpha1.ClusterManagerPhaseDeleting)
	}
}

func (r *ClusterManagerReconciler) CreateServiceInstance(ctx context.Context, clusterManager *clusterv1alpha1.ClusterManager) (ctrl.Result, error) {
	log := r.Log.WithValues("clustermanager", types.NamespacedName{Name: clusterManager.Name, Namespace: clusterManager.Namespace})

	if clusterManager.Annotations["suffix"] != "" {
		return ctrl.Result{}, nil
	}

	serviceInstance := &servicecatalogv1beta1.ServiceInstance{}
	serviceInstanceKey := types.NamespacedName{
		Name:      clusterManager.Name,
		Namespace: clusterManager.Namespace,
	}
	if err := r.Get(context.TODO(), serviceInstanceKey, serviceInstance); err != nil {
		if errors.IsNotFound(err) {
			var clusterJson, providerJson []byte
			//var ClusterServiceClassExternalName, ClusterServicePlanExternalName string
			//ClusterParameter := clusterManager.Spec
			// ClusterParameter := ClusterParameter{
			// 	Namespace:         clusterManager.Namespace,
			// 	ClusterName:       clusterManager.Name,
			// 	Owner:             clusterManager.Annotations["owner"],
			// 	KubernetesVersion: clusterManager.Spec.Version,
			// 	MasterNum:         clusterManager.Spec.MasterNum,
			// 	WorkerNum:         clusterManager.Spec.WorkerNum,
			// }
			if clusterJson, err = json.Marshal(
				&ClusterParameter{
					Namespace:         clusterManager.Namespace,
					ClusterName:       clusterManager.Name,
					Owner:             clusterManager.Annotations["owner"],
					KubernetesVersion: clusterManager.Spec.Version,
					MasterNum:         clusterManager.Spec.MasterNum,
					WorkerNum:         clusterManager.Spec.WorkerNum,
				},
			); err != nil {
				log.Error(err, "Failed to marshal cluster parameters")
			}

			switch strings.ToUpper(clusterManager.Spec.Provider) {
			case util.PROVIDER_AWS:
				//AwsParameter := clusterManager.AwsSpec
				// AwsParameter := AwsParameter{
				// 	SshKey:     clusterManager.AwsSpec.SshKey,
				// 	Region:     clusterManager.AwsSpec.Region,
				// 	MasterType: clusterManager.AwsSpec.MasterType,
				// 	WorkerType: clusterManager.AwsSpec.WorkerType,
				// }

				if providerJson, err = json.Marshal(&clusterManager.AwsSpec); err != nil {
					log.Error(err, "Failed to marshal cluster parameters")
					return ctrl.Result{}, err
				}
			case util.PROVIDER_VSPHERE:
				//VsphereParameter := clusterManager.VsphereSpec
				// VsphereParameter := VsphereParameter{
				// 	PodCidr:             clusterManager.VsphereSpec.PodCidr,
				// 	VcenterIp:           clusterManager.VsphereSpec.VcenterIp,
				// 	VcenterId:           clusterManager.VsphereSpec.VcenterId,
				// 	VcenterPassword:     clusterManager.VsphereSpec.VcenterPassword,
				// 	VcenterThumbprint:   clusterManager.VsphereSpec.VcenterThumbprint,
				// 	VcenterNetwork:      clusterManager.VsphereSpec.VcenterNetwork,
				// 	VcenterDataCenter:   clusterManager.VsphereSpec.VcenterDataCenter,
				// 	VcenterDataStore:    clusterManager.VsphereSpec.VcenterDataStore,
				// 	VcenterFolder:       clusterManager.VsphereSpec.VcenterFolder,
				// 	VcenterResourcePool: clusterManager.VsphereSpec.VcenterResourcePool,
				// 	VcenterKcpIp:        clusterManager.VsphereSpec.VcenterKcpIp,
				// 	VcenterCpuNum:       clusterManager.VsphereSpec.VcenterCpuNum,
				// 	VcenterMemSize:      clusterManager.VsphereSpec.VcenterMemSize,
				// 	VcenterDiskSize:     clusterManager.VsphereSpec.VcenterDiskSize,
				// 	VcenterTemplate:     clusterManager.VsphereSpec.VcenterTemplate,
				// }

				if providerJson, err = json.Marshal(&clusterManager.VsphereSpec); err != nil {
					log.Error(err, "Failed to marshal cluster parameters")
					return ctrl.Result{}, err
				}
			}
			clusterJson = util.MergeJson(clusterJson, providerJson)

			generatedSuffix := util.CreateSuffixString()
			newServiceInstance := &servicecatalogv1beta1.ServiceInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterManager.Name + "-" + generatedSuffix,
					Namespace: clusterManager.Namespace,
				},
				Spec: servicecatalogv1beta1.ServiceInstanceSpec{
					PlanReference: servicecatalogv1beta1.PlanReference{
						ClusterServiceClassExternalName: "capi-" + strings.ToLower(clusterManager.Spec.Provider) + "-template",
						ClusterServicePlanExternalName:  "capi-" + strings.ToLower(clusterManager.Spec.Provider) + "-template-plan-default",
					},
					Parameters: &runtime.RawExtension{
						Raw: clusterJson,
					},
				},
			}

			ctrl.SetControllerReference(clusterManager, newServiceInstance, r.Scheme)
			err = r.Create(context.TODO(), newServiceInstance)
			if err != nil {
				log.Error(err, "Failed to create "+clusterManager.Name+" serviceInstance")
				return ctrl.Result{}, err
			}
			clusterManager.Annotations["suffix"] = generatedSuffix
		} else {
			log.Error(err, "Failed to get serviceInstance")
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *ClusterManagerReconciler) kubeadmControlPlaneUpdate(ctx context.Context, clusterManager *clusterv1alpha1.ClusterManager) (ctrl.Result, error) {
	log := r.Log.WithValues("clustermanager", types.NamespacedName{Name: clusterManager.Name, Namespace: clusterManager.Namespace})

	kcp := &controlplanev1.KubeadmControlPlane{}
	key := types.NamespacedName{
		Name:      clusterManager.Name + "-control-plane",
		Namespace: clusterManager.Namespace,
	}

	if err := r.Get(context.TODO(), key, kcp); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		} else {
			log.Error(err, "Failed to get clusterRole")
			return ctrl.Result{}, err
		}
	}

	//create helper for patch
	helper, _ := patch.NewHelper(kcp, r.Client)
	defer func() {
		if err := helper.Patch(context.TODO(), kcp); err != nil {
			r.Log.Error(err, "kubeadmcontrolplane patch error")
		}
	}()

	if *kcp.Spec.Replicas != int32(clusterManager.Spec.MasterNum) {
		*kcp.Spec.Replicas = int32(clusterManager.Spec.MasterNum)
	}
	if kcp.Spec.Version != clusterManager.Spec.Version {
		kcp.Spec.Version = clusterManager.Spec.Version
	}
	return ctrl.Result{}, nil
}

func (r *ClusterManagerReconciler) machineDeploymentUpdate(ctx context.Context, clusterManager *clusterv1alpha1.ClusterManager) (ctrl.Result, error) {
	log := r.Log.WithValues("clustermanager", types.NamespacedName{Name: clusterManager.Name, Namespace: clusterManager.Namespace})

	md := &capiv1.MachineDeployment{}
	key := types.NamespacedName{
		Name:      clusterManager.Name + "-md-0",
		Namespace: clusterManager.Namespace,
	}

	if err := r.Get(context.TODO(), key, md); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		} else {
			log.Error(err, "Failed to get clusterRole")
			return ctrl.Result{}, err
		}
	}
	//create helper for patch
	helper, _ := patch.NewHelper(md, r.Client)
	defer func() {
		if err := helper.Patch(context.TODO(), md); err != nil {
			r.Log.Error(err, "kubeadmcontrolplane patch error")
		}
	}()

	if *md.Spec.Replicas != int32(clusterManager.Spec.WorkerNum) {
		*md.Spec.Replicas = int32(clusterManager.Spec.WorkerNum)
	}

	if *md.Spec.Template.Spec.Version != clusterManager.Spec.Version {
		*md.Spec.Template.Spec.Version = clusterManager.Spec.Version
	}
	return ctrl.Result{}, nil
}

func (r *ClusterManagerReconciler) requeueClusterManagersForCluster(o client.Object) []ctrl.Request {
	c := o.DeepCopyObject().(*capiv1.Cluster)
	log := r.Log.WithValues("objectMapper", "clusterToClusterManager", "namespace", c.Namespace, c.Kind, c.Name)
	log.Info("Start to requeueClusterManagersForCluster mapping...")

	//get ClusterManager
	clm := &clusterv1alpha1.ClusterManager{}
	key := types.NamespacedName{
		Name:      c.Name,
		Namespace: c.Namespace,
	}

	if err := r.Get(context.TODO(), key, clm); err != nil {
		if errors.IsNotFound(err) {
			log.Info("ClusterManager resource not found. Ignoring since object must be deleted.")
			return nil
		}

		log.Error(err, "Failed to get ClusterManager")
		return nil
	}

	//create helper for patch
	helper, _ := patch.NewHelper(clm, r.Client)
	defer func() {
		if err := helper.Patch(context.TODO(), clm); err != nil {
			log.Error(err, "ClusterManager patch error")
		}
	}()
	// clm.Status.SetTypedPhase(clusterv1alpha1.ClusterManagerPhaseProvisioned)
	clm.Status.ControlPlaneReady = c.Status.ControlPlaneInitialized

	return nil
}

func (r *ClusterManagerReconciler) requeueClusterManagersForKubeadmControlPlane(o client.Object) []ctrl.Request {
	cp := o.DeepCopyObject().(*controlplanev1.KubeadmControlPlane)
	log := r.Log.WithValues("objectMapper", "kubeadmControlPlaneToClusterManagers", "namespace", cp.Namespace, cp.Kind, cp.Name)

	// Don't handle deleted kubeadmcontrolplane
	if !cp.ObjectMeta.DeletionTimestamp.IsZero() {
		log.V(4).Info("kubeadmcontrolplane has a deletion timestamp, skipping mapping.")
		return nil
	}

	//get ClusterManager
	clm := &clusterv1alpha1.ClusterManager{}
	CpName := cp.Name
	pivot := strings.Index(cp.Name, "-control-plane")
	if pivot != -1 {
		CpName = cp.Name[0:pivot]
	}
	key := types.NamespacedName{
		// Name:      strings.Split(cp.Name, "-control-plane")[0],
		Name:      CpName,
		Namespace: cp.Namespace,
	}
	if err := r.Get(context.TODO(), key, clm); err != nil {
		if errors.IsNotFound(err) {
			log.Info("ClusterManager resource not found. Ignoring since object must be deleted.")
			return nil
		}

		log.Error(err, "Failed to get ClusterManager")
		return nil
	}

	//create helper for patch
	helper, _ := patch.NewHelper(clm, r.Client)
	defer func() {
		if err := helper.Patch(context.TODO(), clm); err != nil {
			log.Error(err, "ClusterManager patch error")
		}
	}()

	clm.Status.MasterRun = int(cp.Status.Replicas)

	return nil
}

func (r *ClusterManagerReconciler) requeueClusterManagersForMachineDeployment(o client.Object) []ctrl.Request {
	md := o.DeepCopyObject().(*capiv1.MachineDeployment)
	log := r.Log.WithValues("objectMapper", "kubeadmControlPlaneToClusterManagers", "namespace", md.Namespace, "machinedeployment", md.Name)

	// Don't handle deleted machinedeployment
	if !md.ObjectMeta.DeletionTimestamp.IsZero() {
		log.V(4).Info("machinedeployment has a deletion timestamp, skipping mapping.")
		return nil
	}

	//get ClusterManager
	clm := &clusterv1alpha1.ClusterManager{}
	key := types.NamespacedName{
		//Name:      md.Name[0 : len(md.Name)-len("-md-0")],
		Name:      strings.Split(md.Name, "-md-0")[0],
		Namespace: md.Namespace,
	}
	if err := r.Get(context.TODO(), key, clm); err != nil {
		if errors.IsNotFound(err) {
			log.Info("ClusterManager is deleted deleted.")
			// return nil
		}

		log.Error(err, "Failed to get ClusterManager")
		return nil
	}

	//create helper for patch
	helper, _ := patch.NewHelper(clm, r.Client)
	defer func() {
		if err := helper.Patch(context.TODO(), clm); err != nil {
			log.Error(err, "ClusterManager patch error")
		}
	}()

	clm.Status.WorkerRun = int(md.Status.Replicas)

	return nil
}

func (r *ClusterManagerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// controller, err := ctrl.NewControllerManagedBy(mgr).
	controller, err := ctrl.NewControllerManagedBy(mgr).
		For(&clusterv1alpha1.ClusterManager{}).
		WithEventFilter(
			predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					return true
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					// created clm은 update 필요가 있지만 registerd는 clm update가 필요 없다
					// 다만 registred인 경우 deleteinotimestamp가 있는경우 delete 수행을 위해 reconcile을 수행하긴 해야한다.
					oldclm := e.ObjectOld.(*clusterv1alpha1.ClusterManager)
					newclm := e.ObjectNew.(*clusterv1alpha1.ClusterManager)

					isFinalized := !controllerutil.ContainsFinalizer(oldclm, util.ClusterManagerFinalizer) && controllerutil.ContainsFinalizer(newclm, util.ClusterManagerFinalizer)
					isDelete := oldclm.DeletionTimestamp.IsZero() && !newclm.DeletionTimestamp.IsZero()
					isControlPlaneEndpointUpdate := oldclm.Status.ControlPlaneEndpoint == "" && newclm.Status.ControlPlaneEndpoint != ""
					//isAgentEndpointUpdate := oldclm.Status.AgentEndpoint == "" && newclm.Status.AgentEndpoint != ""
					if isDelete || isControlPlaneEndpointUpdate || isFinalized /*|| isAgentEndpointUpdate*/ {
						return true
					} else {
						if newclm.Labels[util.ClusterTypeKey] == util.ClusterTypeCreated {

							return true
						}
						if newclm.Labels[util.ClusterTypeKey] == util.ClusterTypeRegistered {
							return false
						}
					}
					return false
				},
				DeleteFunc: func(e event.DeleteEvent) bool {
					return false
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			},
		).
		Build(r)

	if err != nil {
		return err
	}

	controller.Watch(
		&source.Kind{Type: &capiv1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(r.requeueClusterManagersForCluster),
		predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldc := e.ObjectOld.(*capiv1.Cluster)
				newc := e.ObjectNew.(*capiv1.Cluster)

				if &newc.Status != nil && !oldc.Status.ControlPlaneInitialized && newc.Status.ControlPlaneInitialized {
					return true
				}
				return false
			},
			CreateFunc: func(e event.CreateEvent) bool {
				return false
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return true
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return false
			},
		},
	)

	controller.Watch(
		&source.Kind{Type: &controlplanev1.KubeadmControlPlane{}},
		handler.EnqueueRequestsFromMapFunc(r.requeueClusterManagersForKubeadmControlPlane),
		predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldKcp := e.ObjectOld.(*controlplanev1.KubeadmControlPlane)
				newKcp := e.ObjectNew.(*controlplanev1.KubeadmControlPlane)

				if oldKcp.Status.Replicas != newKcp.Status.Replicas {
					return true
				}
				return false
			},
			CreateFunc: func(e event.CreateEvent) bool {
				return true
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return false
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return false
			},
		},
	)

	// controller.Watch(
	// 	&source.Kind{Type: &certmanagerv1.Certificate{}},
	// 	handler.EnqueueRequestsFromMapFunc(r.),
	// 	predicate.Funcs{

	// 	}
	// )

	return controller.Watch(
		&source.Kind{Type: &capiv1.MachineDeployment{}},
		handler.EnqueueRequestsFromMapFunc(r.requeueClusterManagersForMachineDeployment),
		predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldMd := e.ObjectOld.(*capiv1.MachineDeployment)
				newMd := e.ObjectNew.(*capiv1.MachineDeployment)

				if oldMd.Status.Replicas != newMd.Status.Replicas {
					return true
				}
				return false
			},
			CreateFunc: func(e event.CreateEvent) bool {
				return true
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return false
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return false
			},
		},
	)
}

// func getKubeConfig(secret corev1.Secret) (*rest.Config, error) {
// 	if value, ok := secret.Data["value"]; ok {
// 		if clientConfig, err := clientcmd.NewClientConfigFromBytes(value); err == nil {
// 			if restConfig, err := clientConfig.ClientConfig(); err == nil {
// 				return restConfig, nil
// 			}
// 		}
// 	}
// 	return nil, errors.NewBadRequest("getClientConfig Error")
// }
