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
	"strings"
	"time"

	"github.com/go-logr/logr"
	certmanagerv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	servicecatalogv1beta1 "github.com/kubernetes-sigs/service-catalog/pkg/apis/servicecatalog/v1beta1"
	clusterv1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/cluster/v1alpha1"
	util "github.com/tmax-cloud/hypercloud-multi-operator/controllers/util"
	traefikv2 "github.com/traefik/traefik/v2/pkg/provider/kubernetes/crd/traefik/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
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
)

const (
	requeueAfter10Sec  = 10 * time.Second
	requeueAfter20Sec  = 20 * time.Second
	requeueAfter30Sec  = 30 * time.Second
	requeueAfter60Sec  = 60 * time.Second
	requeueAfter120Sec = 120 * time.Second
	requeueAfter10min  = 600 * time.Second
)

type ClusterParameter struct {
	Namespace         string
	ClusterName       string
	MasterNum         int
	WorkerNum         int
	Owner             string
	KubernetesVersion string
}

type AwsParameter struct {
	SshKey     string
	Region     string
	MasterType string
	WorkerType string
}

type VsphereParameter struct {
	PodCidr             string
	VcenterIp           string
	VcenterId           string
	VcenterPassword     string
	VcenterThumbprint   string
	VcenterNetwork      string
	VcenterDataCenter   string
	VcenterDataStore    string
	VcenterFolder       string
	VcenterResourcePool string
	VcenterKcpIp        string
	VcenterCpuNum       int
	VcenterMemSize      int
	VcenterDiskSize     int
	VcenterTemplate     string
}

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
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=create;delete;get;list;patch;update;watch

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
	if clusterManager.Labels[util.LabelKeyClmClusterType] == util.ClusterTypeRegistered {
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
		r.UpdatePrometheusService,
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

func (r *ClusterManagerReconciler) reconcileDeleteForRegisteredClusterManager(ctx context.Context, clusterManager *clusterv1alpha1.ClusterManager) (reconcile.Result, error) {
	log := r.Log.WithValues("clustermanager", types.NamespacedName{Name: clusterManager.Name, Namespace: clusterManager.Namespace})
	log.Info("Start to reconcileDeleteForRegisteredClusterManager reconcile for [" + clusterManager.Name + "]")

	kubeconfigSecret := &corev1.Secret{}
	kubeconfigSecretKey := types.NamespacedName{
		Name:      clusterManager.Name + util.KubeconfigSuffix,
		Namespace: clusterManager.Namespace,
	}
	if err := r.Get(context.TODO(), kubeconfigSecretKey, kubeconfigSecret); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Kubeconfig is successfully deleted.. Delete clustermanager finalizer")
			if err := util.Delete(clusterManager.Namespace, clusterManager.Name); err != nil {
				log.Error(err, "Failed to delete cluster info from cluster_member table")
				return ctrl.Result{}, err
			}
		} else {
			log.Error(err, "Failed to get kubeconfig secret")
			return ctrl.Result{}, err
		}
	} else {
		log.Info("Start to delete kubeconfig secret")
		if err := r.Delete(context.TODO(), kubeconfigSecret); err != nil {
			log.Error(err, "Failed to delete kubeconfigSecret")
			return ctrl.Result{}, err
		}
	}

	traefikCertificate := &certmanagerv1.Certificate{}
	certificateKey := types.NamespacedName{
		//Name:      clusterManager.Name + "-certificate-" + clusterManager.Annotations[util.AnnotationKeyClmSuffix],
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
		if err := r.Delete(context.TODO(), traefikCertificate); err != nil {
			log.Error(err, "Failed to delete Certificate")
			return ctrl.Result{}, err
		}
	}

	traefikCertSecret := &corev1.Secret{}
	certSecretKey := types.NamespacedName{
		//Name:      clusterManager.Name + "-certificate-" + clusterManager.Annotations[util.AnnotationKeyClmSuffix],
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
		if err := r.Delete(context.TODO(), traefikCertSecret); err != nil {
			log.Error(err, "Failed to delete Cert-Secret")
			return ctrl.Result{}, err
		}
	}

	traefikIngress := &networkingv1.Ingress{}
	ingressKey := types.NamespacedName{
		//Name:      clusterManager.Name + "-certificate-" + clusterManager.Annotations[util.AnnotationKeyClmSuffix],
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
		if err := r.Delete(context.TODO(), traefikIngress); err != nil {
			log.Error(err, "Failed to delete Ingress")
			return ctrl.Result{}, err
		}
	}

	traefikService := &corev1.Service{}
	serviceKey := types.NamespacedName{
		//Name:      clusterManager.Name + "-service-" + clusterManager.Annotations[util.AnnotationKeyClmSuffix],
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
		if err := r.Delete(context.TODO(), traefikService); err != nil {
			log.Error(err, "Failed to delete Service")
			return ctrl.Result{}, err
		}
	}

	traefikEndpoint := &corev1.Endpoints{}
	endpointKey := types.NamespacedName{
		//Name:      clusterManager.Name + "-service-" + clusterManager.Annotations[util.AnnotationKeyClmSuffix],
		Name:      clusterManager.Name + "-service",
		Namespace: clusterManager.Namespace,
	}
	if err := r.Get(context.TODO(), endpointKey, traefikEndpoint); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Endpoint is already deleted.")
		} else {
			log.Error(err, "Failed to get Endpoint information")
			return ctrl.Result{}, err
		}
	} else {
		if err := r.Delete(context.TODO(), traefikEndpoint); err != nil {
			log.Error(err, "Failed to delete Endpoint")
			return ctrl.Result{}, err
		}
	}

	traefikMiddleware := &traefikv2.Middleware{}
	middlewareKey := types.NamespacedName{
		//Name:      clusterManager.Name + "-prefix-" + clusterManager.Annotations[util.AnnotationKeyClmSuffix],
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
		if err := r.Delete(context.TODO(), traefikMiddleware); err != nil {
			log.Error(err, "Failed to delete Middleware")
			return ctrl.Result{}, err
		}
	}

	prometheusService := &corev1.Service{}
	prometheusServiceKey := types.NamespacedName{
		Name:      clusterManager.GetName() + "-prometheus-service",
		Namespace: clusterManager.GetNamespace(),
	}
	if err := r.Get(context.TODO(), prometheusServiceKey, prometheusService); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Service for prometheus is already deleted.")
		} else {
			log.Error(err, "Failed to get Service for prometheus information")
			return ctrl.Result{}, err
		}
	} else {
		if err := r.Delete(context.TODO(), prometheusService); err != nil {
			log.Error(err, "Failed to delete Service for prometheus")
			return ctrl.Result{}, err
		}
	}

	prometheusEndpoint := &corev1.Endpoints{}
	prometheusEndpointKey := types.NamespacedName{
		Name:      clusterManager.GetName() + "-prometheus-service",
		Namespace: clusterManager.GetNamespace(),
	}
	if err := r.Get(context.TODO(), prometheusEndpointKey, prometheusEndpoint); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Endpoint for prometheus is already deleted.")
		} else {
			log.Error(err, "Failed to get Endpoint for prometheus information")
			return ctrl.Result{}, err
		}
	} else {
		if err := r.Delete(context.TODO(), prometheusEndpoint); err != nil {
			log.Error(err, "Failed to delete Endpoint for prometheus")
			return ctrl.Result{}, err
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
		r.UpdatePrometheusService,
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
		Name:      clusterManager.Name + util.KubeconfigSuffix,
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
		traefikCertificate := &certmanagerv1.Certificate{}
		certificateKey := types.NamespacedName{
			//Name:      clusterManager.Name + "-certificate-" + clusterManager.Annotations[util.AnnotationKeyClmSuffix],
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
			if err := r.Delete(context.TODO(), traefikCertificate); err != nil {
				log.Error(err, "Failed to delete Certificate")
				return ctrl.Result{}, err
			}
		}

		traefikCertSecret := &corev1.Secret{}
		certSecretKey := types.NamespacedName{
			//Name:      clusterManager.Name + "-certificate-" + clusterManager.Annotations[util.AnnotationKeyClmSuffix],
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
			if err := r.Delete(context.TODO(), traefikCertSecret); err != nil {
				log.Error(err, "Failed to delete Cert-Secret")
				return ctrl.Result{}, err
			}
		}

		traefikIngress := &networkingv1.Ingress{}
		ingressKey := types.NamespacedName{
			//Name:      clusterManager.Name + "-certificate-" + clusterManager.Annotations[util.AnnotationKeyClmSuffix],
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
			if err := r.Delete(context.TODO(), traefikIngress); err != nil {
				log.Error(err, "Failed to delete Ingress")
				return ctrl.Result{}, err
			}
		}

		traefikService := &corev1.Service{}
		serviceKey := types.NamespacedName{
			//Name:      clusterManager.Name + "-service-" + clusterManager.Annotations[util.AnnotationKeyClmSuffix],
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
			if err := r.Delete(context.TODO(), traefikService); err != nil {
				log.Error(err, "Failed to delete Service")
				return ctrl.Result{}, err
			}
		}

		traefikEndpoint := &corev1.Endpoints{}
		endpointKey := types.NamespacedName{
			//Name:      clusterManager.Name + "-service-" + clusterManager.Annotations[util.AnnotationKeyClmSuffix],
			Name:      clusterManager.Name + "-service",
			Namespace: clusterManager.Namespace,
		}
		if err := r.Get(context.TODO(), endpointKey, traefikEndpoint); err != nil {
			if errors.IsNotFound(err) {
				log.Info("Endpoint is already deleted.")
			} else {
				log.Error(err, "Failed to get Endpoint information")
				return ctrl.Result{}, err
			}
		} else {
			if err := r.Delete(context.TODO(), traefikEndpoint); err != nil {
				log.Error(err, "Failed to delete Endpoint")
				return ctrl.Result{}, err
			}
		}

		traefikMiddleware := &traefikv2.Middleware{}
		middlewareKey := types.NamespacedName{
			//Name:      clusterManager.Name + "-prefix-" + clusterManager.Annotations[util.AnnotationKeyClmSuffix],
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
			if err := r.Delete(context.TODO(), traefikMiddleware); err != nil {
				log.Error(err, "Failed to delete Service")
				return ctrl.Result{}, err
			}
		}

		prometheusService := &corev1.Service{}
		prometheusServiceKey := types.NamespacedName{
			Name:      clusterManager.GetName() + "-prometheus-service",
			Namespace: clusterManager.GetNamespace(),
		}
		if err := r.Get(context.TODO(), prometheusServiceKey, prometheusService); err != nil {
			if errors.IsNotFound(err) {
				log.Info("Service for prometheus is already deleted.")
			} else {
				log.Error(err, "Failed to get Service for prometheus information")
				return ctrl.Result{}, err
			}
		} else {
			if err := r.Delete(context.TODO(), prometheusService); err != nil {
				log.Error(err, "Failed to delete Service for prometheus")
				return ctrl.Result{}, err
			}
		}

		prometheusEndpoint := &corev1.Endpoints{}
		prometheusEndpointKey := types.NamespacedName{
			Name:      clusterManager.GetName() + "-prometheus-service",
			Namespace: clusterManager.GetNamespace(),
		}
		if err := r.Get(context.TODO(), prometheusEndpointKey, prometheusEndpoint); err != nil {
			if errors.IsNotFound(err) {
				log.Info("Endpoint for prometheus is already deleted.")
			} else {
				log.Error(err, "Failed to get Endpoint for prometheus information")
				return ctrl.Result{}, err
			}
		} else {
			if err := r.Delete(context.TODO(), prometheusEndpoint); err != nil {
				log.Error(err, "Failed to delete Endpoint for prometheus")
				return ctrl.Result{}, err
			}
		}
		// todo - shkim
		// traefik endpoint 삭제 추가 필요?

		remoteClientset, err := util.GetRemoteK8sClient(kubeconfigSecret)
		if err != nil {
			log.Error(err, "Failed to get remoteK8sClient")
			return ctrl.Result{}, err
		}

		// secret은 존재하는데.. 실제 instance가 없어서 에러 발생
		if _, err =
			remoteClientset.
				CoreV1().
				Namespaces().
				Get(
					context.TODO(),
					util.IngressNginxNamespace,
					metav1.GetOptions{},
				); err != nil {
			if errors.IsNotFound(err) {
				log.Info("Ingress-nginx namespace is already deleted.")
			} else {
				log.Info(err.Error())
				log.Info("Failed to get Ingress-nginx loadbalancer service... may be instance was deleted before secret was deleted...")
				// log.Info("###################### Never executed... ############################")
				// error 처리 필요
			}
		} else {
			if err :=
				remoteClientset.
					CoreV1().
					Namespaces().
					Delete(
						context.TODO(),
						util.IngressNginxNamespace,
						metav1.DeleteOptions{},
					); err != nil {
				log.Error(err, "Failed to delete Ingress-nginx namespace")
				return ctrl.Result{}, err
			}
		}
	}

	// aaa
	// delete serviceinstance
	serviceInstance := &servicecatalogv1beta1.ServiceInstance{}
	serviceInstanceKey := types.NamespacedName{
		Name:      clusterManager.Name + "-" + clusterManager.Annotations[util.AnnotationKeyClmSuffix],
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
		if clusterManager.Labels[util.LabelKeyClmClusterType] == util.ClusterTypeRegistered {
			clusterManager.Status.SetTypedPhase(clusterv1alpha1.ClusterManagerPhaseRegistering)
		} else {
			clusterManager.Status.SetTypedPhase(clusterv1alpha1.ClusterManagerPhaseProvisioning)
		}
	}

	if clusterManager.Status.Ready {
		if clusterManager.Labels[util.LabelKeyClmClusterType] == util.ClusterTypeRegistered {
			clusterManager.Status.SetTypedPhase(clusterv1alpha1.ClusterManagerPhaseRegistered)
		} else {
			clusterManager.Status.SetTypedPhase(clusterv1alpha1.ClusterManagerPhaseProvisioned)
		}
	}

	if !clusterManager.DeletionTimestamp.IsZero() {
		clusterManager.Status.SetTypedPhase(clusterv1alpha1.ClusterManagerPhaseDeleting)
	}
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
					// created clm은 update 필요가 있지만 registered는 clm update가 필요 없다
					// 다만 registered인 경우 deletiontimestamp가 있는경우 delete 수행을 위해 reconcile을 수행하긴 해야한다.
					oldclm := e.ObjectOld.(*clusterv1alpha1.ClusterManager)
					newclm := e.ObjectNew.(*clusterv1alpha1.ClusterManager)

					isFinalized := !controllerutil.ContainsFinalizer(oldclm, util.ClusterManagerFinalizer) && controllerutil.ContainsFinalizer(newclm, util.ClusterManagerFinalizer)
					isDelete := oldclm.DeletionTimestamp.IsZero() && !newclm.DeletionTimestamp.IsZero()
					isControlPlaneEndpointUpdate := oldclm.Status.ControlPlaneEndpoint == "" && newclm.Status.ControlPlaneEndpoint != ""
					//isAgentEndpointUpdate := oldclm.Status.AgentEndpoint == "" && newclm.Status.AgentEndpoint != ""
					if isDelete || isControlPlaneEndpointUpdate || isFinalized /*|| isAgentEndpointUpdate*/ {
						return true
					} else {
						if newclm.Labels[util.LabelKeyClmClusterType] == util.ClusterTypeCreated {

							return true
						}
						if newclm.Labels[util.LabelKeyClmClusterType] == util.ClusterTypeRegistered {
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
