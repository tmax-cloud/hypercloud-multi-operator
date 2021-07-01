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
	"time"

	// "k8s.io/apimachinery/pkg/util/intstr"

	"github.com/go-logr/logr"
	servicecatalogv1beta1 "github.com/kubernetes-sigs/service-catalog/pkg/apis/servicecatalog/v1beta1"
	console "github.com/tmax-cloud/console-operator/api/v1"
	clusterv1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/cluster/v1alpha1"
	util "github.com/tmax-cloud/hypercloud-multi-operator/controllers/util"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"

	// networkingv1 "k8s.io/api/networking/v1"

	// "k8s.io/kubernetes/pkg/apis/networking"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
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
	CAPI_SYSTEM_NAMESPACE       = "capi-system"
	CLAIM_API_GROUP             = "claim.tmax.io"
	CLUSTER_API_GROUP           = "cluster.tmax.io"
	CLAIM_API_Kind              = "clusterclaims"
	CLAIM_API_GROUP_VERSION     = "claim.tmax.io/v1alpha1"
	HYPERCLOUD_SYSTEM_NAMESPACE = ""
	requeueAfter10Sec           = 10 * time.Second
	requeueAfter20Sec           = 20 * time.Second
	requeueAfter30Sec           = 30 * time.Second
	requeueAfter60Sec           = 60 * time.Second
	requeueAfter120Sec          = 120 * time.Second
)

type ClusterParameter struct {
	ClusterName       string
	AWSRegion         string
	SshKey            string
	MasterNum         int
	MasterType        string
	WorkerNum         int
	WorkerType        string
	Owner             string
	KubernetesVersion string
}

// ClusterManagerReconciler reconciles a ClusterManager object
type ClusterManagerReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=cluster.tmax.io,resources=clustermanagers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.tmax.io,resources=clustermanagers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machinedeployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machinedeployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=kubeadmcontrolplanes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=kubeadmcontrolplanes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=servicecatalog.k8s.io,resources=serviceinstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=servicecatalog.k8s.io,resources=serviceinstances/status,verbs=get;update;patch

func (r *ClusterManagerReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
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
			return r.reconcileDeleteForRegisteredClusterManager(context.TODO(), clusterManager)
		}
		return r.reconcileForRegisteredClusterManager(context.TODO(), clusterManager)
	}

	// Handle deletion reconciliation loop.
	if !clusterManager.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(context.TODO(), clusterManager)
	}

	return r.reconcile(context.TODO(), clusterManager)
}

// reconcile handles cluster reconciliation.
func (r *ClusterManagerReconciler) reconcileForRegisteredClusterManager(ctx context.Context, clusterManager *clusterv1alpha1.ClusterManager) (ctrl.Result, error) {
	phases := []func(context.Context, *clusterv1alpha1.ClusterManager) (ctrl.Result, error){
		r.UpdateClusterManagerStatus,
		r.CreateProxyConfiguration,
		r.DeployAndUpdateAgentEndpoint,
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
	// secret이 만들어진걸 여기서 알 수 있을까?? controller는 cache를 공유할까?
	kubeconfigSecret := &corev1.Secret{}
	kubeconfigSecretKey := types.NamespacedName{Name: clusterManager.Name + "-kubeconfig", Namespace: clusterManager.Namespace}
	if err := r.Get(context.TODO(), kubeconfigSecretKey, kubeconfigSecret); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Cannot found kubeconfig secret. Wait to create kubeconfig secret.")
			return ctrl.Result{RequeueAfter: requeueAfter10Sec}, nil
		} else {
			log.Error(err, "Failed to get kubeconfig secret")
			return ctrl.Result{}, err
		}
	}
	// clr 가져오지 말고 여기서 바로 secret으로부터 remote cluster의 정보를 가져옥자

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

	// delete update에 대해서만 들어와서 초기화 굳이 필요 없을 듯
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
		clusterManager.Status.Ready = true
	} else {
		// err := errors.NewBadRequest("Failed to healthcheck")
		// log.Error(err, "Failed to healthcheck")
		// 잠시 오류난걸 수도 있으니까.. 근데 무한장 wait할 수는 없어서 requeue 횟수를 지정할 수 있으면 좋겠네
		log.Info("Remote cluster is not ready... watit...")
		return ctrl.Result{RequeueAfter: requeueAfter30Sec}, nil
	}

	return ctrl.Result{}, nil
}

func (r *ClusterManagerReconciler) CreateProxyConfiguration(ctx context.Context, clusterManager *clusterv1alpha1.ClusterManager) (ctrl.Result, error) {
	log := r.Log.WithValues("clustermanager", types.NamespacedName{Name: clusterManager.Name, Namespace: clusterManager.Namespace})

	// secret controller에서 clustermanager.status.controleplaneendpint를 채워줄 때 까지 기다림

	if !clusterManager.Status.ControlPlaneReady {
		// requeue (wait cluster controller)
		log.Info("ClusterManager controleplane is not ready... requeue after 120 sec")
		return ctrl.Result{RequeueAfter: requeueAfter120Sec}, nil
	} else if clusterManager.Status.ControlPlaneEndpoint != "" {
		log.Info("ClusterManager is ready... create proxy configuration ")
		proxyConfig := &console.Console{}
		key := types.NamespacedName{Name: util.ReversePorxyObjectName, Namespace: util.ReversePorxyObjectNamespace}

		router := &console.Router{
			Server: clusterManager.Status.ControlPlaneEndpoint,
			Rule:   "PathPrefix(`/api/" + clusterManager.Namespace + "/" + clusterManager.Name + "`)",
			Path:   "/api/" + clusterManager.Namespace + "/" + clusterManager.Name + "/",
		}
		routerNamespacedName := clusterManager.Namespace + "-" + clusterManager.Name
		if err := r.Get(context.TODO(), key, proxyConfig); err != nil {
			if errors.IsNotFound(err) {
				log.Info("Cannot found console object. Start to create console object.")
				proxyConfig = &console.Console{
					ObjectMeta: metav1.ObjectMeta{
						Name:      util.ReversePorxyObjectName,
						Namespace: util.ReversePorxyObjectNamespace,
					},
					Spec: console.ConsoleSpec{
						Configuration: console.Configuration{
							Routers: map[string]*console.Router{
								routerNamespacedName: router,
							},
						},
					},
				}

				if err := r.Create(context.TODO(), proxyConfig); err != nil {
					log.Error(err, "Failed to create console object")
					return ctrl.Result{}, err
				}
			} else {
				log.Error(err, "Failed to get console object")
				return ctrl.Result{}, err
			}
		} else {
			// nil map
			if proxyConfig.Spec.Configuration.Routers == nil {
				proxyConfig.Spec.Configuration.Routers = map[string]*console.Router{
					routerNamespacedName: router,
				}
			} else if _, ok := proxyConfig.Spec.Configuration.Routers[routerNamespacedName]; !ok {
				proxyConfig.Spec.Configuration.Routers[routerNamespacedName] = router
			} else {
				log.Info("Routing config is already existed")
				return ctrl.Result{}, nil
			}

			if err := r.Update(context.TODO(), proxyConfig); err != nil {
				log.Error(err, "Failed to update console object")
				return ctrl.Result{}, err
			}
		}

	}

	return ctrl.Result{}, nil
}

func (r *ClusterManagerReconciler) DeployAndUpdateAgentEndpoint(ctx context.Context, clusterManager *clusterv1alpha1.ClusterManager) (ctrl.Result, error) {
	log := r.Log.WithValues("clustermanager", types.NamespacedName{Name: clusterManager.Name, Namespace: clusterManager.Namespace})
	// secret controller에서 clustermanager.status.controleplaneendpint를 채워줄 때 까지 기다림
	if !clusterManager.Status.ControlPlaneReady {
		// requeue (wait cluster controller)
		return ctrl.Result{RequeueAfter: requeueAfter60Sec}, nil
	} else if !clusterManager.Status.AgentReady {
		kubeconfigSecret := &corev1.Secret{}
		kubeconfigSecretKey := types.NamespacedName{Name: clusterManager.Name + "-kubeconfig", Namespace: clusterManager.Namespace}
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

		if _, err = remoteClientset.CoreV1().Namespaces().Get(context.TODO(), util.IngressNginxNamespace, metav1.GetOptions{}); err != nil {
			if errors.IsNotFound(err) {
				log.Info("Ingress is not installed .. ")
				return ctrl.Result{RequeueAfter: requeueAfter60Sec}, nil
			} else {
				log.Error(err, "Failed to get ingress-nginx namespace from remote cluster")
				return ctrl.Result{}, err
			}
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
			if ingressController, err = remoteClientset.AppsV1().Deployments(util.IngressNginxNamespace).Get(context.TODO(), util.IngressNginxDeployment, metav1.GetOptions{}); err != nil {
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

		var agentIngress *extensionsv1beta1.Ingress
		if agentIngress, err = remoteClientset.ExtensionsV1beta1().Ingresses(util.KubeFedNamespace).Get(context.TODO(), util.AGENT_INGRESS_NAME, metav1.GetOptions{}); err != nil {
			if errors.IsNotFound(err) {
				log.Info("Deploy ingress.... ")

				// create ingress
				ingress := &extensionsv1beta1.Ingress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      util.AGENT_INGRESS_NAME,
						Namespace: util.KubeFedNamespace,
						Annotations: map[string]string{
							"nginx.ingress.kubernetes.io/rewrite-target": "/$2",
						},
					},
					Spec: extensionsv1beta1.IngressSpec{
						Rules: []extensionsv1beta1.IngressRule{
							{
								IngressRuleValue: extensionsv1beta1.IngressRuleValue{
									HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
										Paths: []extensionsv1beta1.HTTPIngressPath{
											{
												Path: "/prometheus(/|$)(.*)",
												// PathType: extensionsv1beta1.PathTypePrefix,
												Backend: extensionsv1beta1.IngressBackend{
													ServiceName: "monitoring-service",
													ServicePort: intstr.IntOrString{
														IntVal: 9090,
													},
												},
											},
											{
												Path: "/agent(/|$)(.*)",
												// PathType: extensionsv1beta1.PathType(PathTypePrefix),
												Backend: extensionsv1beta1.IngressBackend{
													ServiceName: "hypercloud-multi-agent-service",
													ServicePort: intstr.IntOrString{
														IntVal: 80,
													},
												},
											},
										},
									},
								},
							},
						},
					},
				}

				if _, err := remoteClientset.ExtensionsV1beta1().Ingresses(util.KubeFedNamespace).Create(context.TODO(), ingress, metav1.CreateOptions{}); err != nil {
					log.Error(err, "Failed to create agentIngress")
					return ctrl.Result{}, err
				}

				log.Info("Wait address to be assigned..  requeue 60sec")
				return ctrl.Result{RequeueAfter: requeueAfter60Sec}, nil
			} else {
				log.Error(err, "Failed to get agent ingress object from remote cluster")
				return ctrl.Result{}, err
			}
		} else {
			if len(agentIngress.Status.LoadBalancer.Ingress) != 0 && agentIngress.Status.LoadBalancer.Ingress[0].Hostname != "" {
				clusterManager.Status.AgentEndpoint = agentIngress.Status.LoadBalancer.Ingress[0].Hostname
				clusterManager.Status.AgentReady = true
				clusterManager.Status.Ready = true
			} else {
				log.Info("Failed to get agent ingress address.. [ " + agentIngress.Status.LoadBalancer.Ingress[0].Hostname + "] requeue 60 sec")
				return ctrl.Result{RequeueAfter: requeueAfter60Sec}, nil
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *ClusterManagerReconciler) reconcileDeleteForRegisteredClusterManager(ctx context.Context, clusterManager *clusterv1alpha1.ClusterManager) (reconcile.Result, error) {
	log := r.Log.WithValues("clustermanager", types.NamespacedName{Name: clusterManager.Name, Namespace: clusterManager.Namespace})
	log.Info("Start to reconcileDeleteForRegisteredClusterManager reconcile for [" + clusterManager.Name + "]")

	proxyConfig := &console.Console{}
	key := types.NamespacedName{Name: util.ReversePorxyObjectName, Namespace: util.ReversePorxyObjectNamespace}
	routerNamespacedName := clusterManager.Namespace + "-" + clusterManager.Name
	if err := r.Get(context.TODO(), key, proxyConfig); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Cannot not found console object. The cluster has never been created.")
		} else {
			log.Error(err, "Failed to get console object")
			return ctrl.Result{}, err
		}
	} else {
		delete(proxyConfig.Spec.Configuration.Routers, routerNamespacedName)
		if err := r.Update(context.TODO(), proxyConfig); err != nil {
			log.Error(err, "Failed to update proxyConfig")
			return ctrl.Result{}, err
		}
	}

	kubeconfigSecret := &corev1.Secret{}
	kubeconfigSecretKey := types.NamespacedName{Name: clusterManager.Name + "-kubeconfig", Namespace: clusterManager.Namespace}
	if err := r.Get(context.TODO(), kubeconfigSecretKey, kubeconfigSecret); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Kubeconfig is succesefully delete.. Delete clustermanager finalizer")
			controllerutil.RemoveFinalizer(clusterManager, util.ClusterManagerFinalizer)
		} else {
			log.Error(err, "Failed to get kubeconfig secret")
			return ctrl.Result{}, err
		}
	} else {
		// delete 를 계~~ 속 보내겠는데...?
		log.Info("Start to delete kubeconfig secret")
		if err := r.Delete(context.TODO(), kubeconfigSecret); err != nil {
			log.Error(err, "Failed to delete kubeconfigSecret")
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: requeueAfter10Sec}, nil
	}

	return ctrl.Result{}, nil
}

// reconcile handles cluster reconciliation.
func (r *ClusterManagerReconciler) reconcile(ctx context.Context, clusterManager *clusterv1alpha1.ClusterManager) (ctrl.Result, error) {
	phases := []func(context.Context, *clusterv1alpha1.ClusterManager) (ctrl.Result, error){
		r.CreateServiceInstance,
		r.CreateProxyConfiguration,
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
	routerNamespacedName := clusterManager.Namespace + "-" + clusterManager.Name
	// Delete (reverse proxy) console object
	proxyConfig := &console.Console{}
	key := types.NamespacedName{Name: util.ReversePorxyObjectName, Namespace: util.ReversePorxyObjectNamespace}

	if err := r.Get(context.TODO(), key, proxyConfig); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Cannot not found console object. The cluster has never been created.")
		} else {
			log.Error(err, "Failed to get console object")
			return ctrl.Result{}, err
		}
	} else {
		delete(proxyConfig.Spec.Configuration.Routers, routerNamespacedName)
		if err := r.Update(context.TODO(), proxyConfig); err != nil {
			log.Error(err, "Failed to update proxyConfig")
			return ctrl.Result{}, err
		}
	}

	kubeconfigSecret := &corev1.Secret{}
	kubeconfigSecretKey := types.NamespacedName{Name: clusterManager.Name + "-kubeconfig", Namespace: clusterManager.Namespace}
	if err := r.Get(context.TODO(), kubeconfigSecretKey, kubeconfigSecret); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Kubeconfig Secret is already deleted. Waiting cluster to be deleted...")
		} else {
			log.Error(err, "Failed to get kubeconfig secret")
			return ctrl.Result{}, err
		}
	} else {
		// // instance가 삭제되었을 때 call 안하도록 처리가 필요함
		// // } else if controllerutil.ContainsFinalizer(kubeconfigSecret, util.SecretFinalizerForClusterManager) {
		// remoteClientset, err := util.GetRemoteK8sClient(kubeconfigSecret)
		// if err != nil {
		// 	log.Error(err, "Failed to get remoteK8sClient")
		// 	return ctrl.Result{}, err
		// }

		// // secret은 존재하는데.. 실제 instance가 없어서 에러 발생
		// if _, err = remoteClientset.CoreV1().Namespaces().Get(context.TODO(), util.IngressNginxNamespace, metav1.GetOptions{}); err != nil {
		// 	if errors.IsNotFound(err) {
		// 		log.Info("Ingress-nginx namespace is already deleted.")
		// 	} else {
		// 		log.Info("Failed to get Ingress-nginx namespace... may be instance was deleted before secret was deleted...")
		// 		// error 처리 필요
		// 	}
		// } else {
		// 	if err := remoteClientset.CoreV1().Namespaces().Delete(context.TODO(), util.IngressNginxNamespace, metav1.DeleteOptions{}); err != nil {
		// 		log.Error(err, "Failed to delete Ingress-nginx namespace")
		// 		return ctrl.Result{}, err
		// 	}
		// }
		if err := r.Delete(context.TODO(), kubeconfigSecret); err != nil {
			log.Error(err, "Failed to delete kubeconfigSecret")
			return ctrl.Result{}, err
		}
	}

	// delete serviceinstance
	serviceInstance := &servicecatalogv1beta1.ServiceInstance{}
	serviceInstanceKey := types.NamespacedName{Name: clusterManager.Name, Namespace: clusterManager.Namespace}

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
	cluster := &clusterv1.Cluster{}
	clusterKey := types.NamespacedName{Name: clusterManager.Name, Namespace: clusterManager.Namespace}

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
	serviceInstance := &servicecatalogv1beta1.ServiceInstance{}
	serviceInstanceKey := types.NamespacedName{Name: clusterManager.Name, Namespace: clusterManager.Namespace}
	if err := r.Get(context.TODO(), serviceInstanceKey, serviceInstance); err != nil {
		if errors.IsNotFound(err) {
			clusterParameter := ClusterParameter{
				ClusterName:       clusterManager.Name,
				AWSRegion:         clusterManager.Spec.Region,
				SshKey:            clusterManager.Spec.SshKey,
				MasterNum:         clusterManager.Spec.MasterNum,
				MasterType:        clusterManager.Spec.MasterType,
				WorkerNum:         clusterManager.Spec.WorkerNum,
				WorkerType:        clusterManager.Spec.WorkerType,
				Owner:             clusterManager.Annotations["owner"],
				KubernetesVersion: clusterManager.Spec.Version,
			}

			byte, err := json.Marshal(&clusterParameter)
			if err != nil {
				log.Error(err, "Failed to marshal cluster parameters")
				return ctrl.Result{}, err
			}

			newServiceInstance := &servicecatalogv1beta1.ServiceInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterManager.Name,
					Namespace: clusterManager.Namespace,
				},
				Spec: servicecatalogv1beta1.ServiceInstanceSpec{
					PlanReference: servicecatalogv1beta1.PlanReference{
						ClusterServiceClassExternalName: "capi-aws-template",
						ClusterServicePlanExternalName:  "capi-aws-template-plan-default",
					},
					Parameters: &runtime.RawExtension{
						Raw: byte,
					},
				},
			}

			ctrl.SetControllerReference(clusterManager, newServiceInstance, r.Scheme)
			err = r.Create(context.TODO(), newServiceInstance)
			if err != nil {
				log.Error(err, "Failed to create "+clusterManager.Name+" serviceInstance")
				return ctrl.Result{}, err
			}
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
	key := types.NamespacedName{Name: clusterManager.Name + "-control-plane", Namespace: clusterManager.Namespace}

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

	md := &clusterv1.MachineDeployment{}
	key := types.NamespacedName{Name: clusterManager.Name + "-md-0", Namespace: clusterManager.Namespace}

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

func (r *ClusterManagerReconciler) requeueClusterManagersForCluster(o handler.MapObject) []ctrl.Request {
	c := o.Object.(*clusterv1.Cluster)
	log := r.Log.WithValues("objectMapper", "clusterToClusterManager", "namespace", c.Namespace, c.Kind, c.Name)
	log.Info("Start to requeueClusterManagersForCluster mapping...")

	//get ClusterManager
	clm := &clusterv1alpha1.ClusterManager{}
	key := types.NamespacedName{Namespace: c.Namespace, Name: c.Name}

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

func (r *ClusterManagerReconciler) requeueClusterManagersForKubeadmControlPlane(o handler.MapObject) []ctrl.Request {
	cp := o.Object.(*controlplanev1.KubeadmControlPlane)
	log := r.Log.WithValues("objectMapper", "kubeadmControlPlaneToClusterManagers", "namespace", cp.Namespace, cp.Kind, cp.Name)

	// Don't handle deleted kubeadmcontrolplane
	if !cp.ObjectMeta.DeletionTimestamp.IsZero() {
		log.V(4).Info("kubeadmcontrolplane has a deletion timestamp, skipping mapping.")
		return nil
	}

	//get ClusterManager
	clm := &clusterv1alpha1.ClusterManager{}
	key := types.NamespacedName{Namespace: cp.Namespace, Name: cp.Name[0 : len(cp.Name)-len("-control-plane")]}
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

func (r *ClusterManagerReconciler) requeueClusterManagersForMachineDeployment(o handler.MapObject) []ctrl.Request {
	md := o.Object.(*clusterv1.MachineDeployment)
	log := r.Log.WithValues("objectMapper", "kubeadmControlPlaneToClusterManagers", "namespace", md.Namespace, "machinedeployment", md.Name)

	// Don't handle deleted machinedeployment
	if !md.ObjectMeta.DeletionTimestamp.IsZero() {
		log.V(4).Info("machinedeployment has a deletion timestamp, skipping mapping.")
		return nil
	}

	//get ClusterManager
	clm := &clusterv1alpha1.ClusterManager{}
	key := types.NamespacedName{Namespace: md.Namespace, Name: md.Name[0 : len(md.Name)-len("-md-0")]}
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
					isAgentEndpointUpdate := oldclm.Status.AgentEndpoint == "" && newclm.Status.AgentEndpoint != ""
					if isDelete || isControlPlaneEndpointUpdate || isFinalized || isAgentEndpointUpdate {
						return true
					} else {
						if newclm.Labels[util.ClusterTypeKey] == util.ClusterTypeCreated {
							// diff := clusterv1alpha1.ClusterManager{}
							// olddata, _ := json.Marshal(oldclm)
							// newdata, _ := json.Marshal(newclm)
							// mergePatch, _ := jsonpatch.CreateMergePatch(olddata, newdata)
							// _ = json.Unmarshal(mergePatch, &diff)

							// fmt.Println("###### ")
							// fmt.Println(string(mergePatch))
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
		&source.Kind{Type: &clusterv1.Cluster{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(r.requeueClusterManagersForCluster),
		},
		predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldc := e.ObjectOld.(*clusterv1.Cluster)
				newc := e.ObjectNew.(*clusterv1.Cluster)

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
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(r.requeueClusterManagersForKubeadmControlPlane),
		},
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

	return controller.Watch(
		&source.Kind{Type: &clusterv1.MachineDeployment{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(r.requeueClusterManagersForMachineDeployment),
		},
		predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldMd := e.ObjectOld.(*clusterv1.MachineDeployment)
				newMd := e.ObjectNew.(*clusterv1.MachineDeployment)

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
