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
	"time"

	"github.com/go-logr/logr"
	servicecatalogv1beta1 "github.com/kubernetes-sigs/service-catalog/pkg/apis/servicecatalog/v1beta1"
	"github.com/prometheus/common/log"
	clusterv1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/cluster/v1alpha1"
	util "github.com/tmax-cloud/hypercloud-multi-operator/controllers/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
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
)

const (
	CAPI_SYSTEM_NAMESPACE       = "capi-system"
	CLAIM_API_GROUP             = "claim.tmax.io"
	CLUSTER_API_GROUP           = "cluster.tmax.io"
	CLAIM_API_Kind              = "clusterclaims"
	CLAIM_API_GROUP_VERSION     = "claim.tmax.io/v1alpha1"
	HYPERCLOUD_SYSTEM_NAMESPACE = ""
	deleteRequeueAfter          = 10 * time.Second
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

	// Handle deletion reconciliation loop.
	if !clusterManager.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(context.TODO(), clusterManager)
	}

	// Handle normal reconciliation loop.
	return r.reconcile(context.TODO(), clusterManager)
}

// reconcile handles cluster reconciliation.
func (r *ClusterManagerReconciler) reconcile(ctx context.Context, clusterManager *clusterv1alpha1.ClusterManager) (ctrl.Result, error) {
	phases := []func(context.Context, *clusterv1alpha1.ClusterManager) (ctrl.Result, error){
		r.CreateServiceInstance,
		// r.CreateClusterMnagerOwnerRole,
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

	// delete ingress controller from remote cluster
	if restConfig, err := getConfigFromSecret(r.Client, clusterManager); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Kubeconfig secret is already deleted.")
		} else {
			log.Error(err, "Failed to get kubeconfig secret")
			return ctrl.Result{}, err
		}
	} else {
		remoteScheme := runtime.NewScheme()
		utilruntime.Must(corev1.AddToScheme(remoteScheme))
		if remoteClient, err := client.New(restConfig, client.Options{Scheme: remoteScheme}); err == nil || remoteClient != nil {
			ingressNamespaceKey := types.NamespacedName{Name: "ingress-nginx", Namespace: ""}
			ingressNamespace := corev1.Namespace{}
			if err := remoteClient.Get(context.TODO(), ingressNamespaceKey, &ingressNamespace); err != nil {
				if errors.IsNotFound(err) {
					log.Info("Ingress-nginx namespace is already deleted.")
				} else {
					log.Error(err, "Failed to get Ingress-nginx namespace")
					return ctrl.Result{}, err
				}
			} else {
				if err := remoteClient.Delete(context.TODO(), &ingressNamespace); err != nil {
					log.Error(err, "Failed to delete Ingress-nginx namespace")
					return ctrl.Result{}, err
				}
			}
		} else {
			log.Error(err, "Failed to get remoteclient")
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
			if err := util.Delete(clusterManager.Name); err != nil {
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
	log.Info("Cluster is deleteing... reenqueue request")
	return ctrl.Result{RequeueAfter: deleteRequeueAfter}, nil
}

func (r *ClusterManagerReconciler) reconcilePhase(_ context.Context, clusterManager *clusterv1alpha1.ClusterManager) {
	if clusterManager.Status.Phase == "" {
		clusterManager.Status.SetTypedPhase(clusterv1alpha1.ClusterManagerPhaseProvisioning)
	}

	if clusterManager.Status.Ready {
		clusterManager.Status.SetTypedPhase(clusterv1alpha1.ClusterManagerPhaseProvisioned)
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
	log := r.Log.WithValues("objectMapper", "clusterToClusterManager", "namespace", c.Namespace, "kubeadmcontrolplane", c.Name)

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

	clm.Status.Ready = c.Status.ControlPlaneInitialized

	return nil
}

func (r *ClusterManagerReconciler) requeueClusterManagersForKubeadmControlPlane(o handler.MapObject) []ctrl.Request {
	cp := o.Object.(*controlplanev1.KubeadmControlPlane)
	log := r.Log.WithValues("objectMapper", "kubeadmControlPlaneToClusterManagers", "namespace", cp.Namespace, "kubeadmcontrolplane", cp.Name)

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
				// Avoid reconciling if the event triggering the reconciliation is related to incremental status updates
				// for kubefedcluster resources only
				CreateFunc: func(e event.CreateEvent) bool {
					return true
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					return true
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

				if &newc.Status != nil && oldc.Status.ControlPlaneInitialized == false && newc.Status.ControlPlaneInitialized == true {
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

func getConfigFromSecret(c client.Client, clusterManager *clusterv1alpha1.ClusterManager) (*restclient.Config, error) {
	secret := &corev1.Secret{}

	if err := c.Get(context.TODO(), types.NamespacedName{Name: clusterManager.Name + "-kubeconfig", Namespace: clusterManager.Namespace}, secret); err != nil {
		if errors.IsNotFound(err) {
			log.Info(err, clusterManager.Name+"-Kubeconfig secret is already deleted.")
			return nil, err
		} else {
			log.Info(err, "Failed to get kubeconfig")
			return nil, err
		}
	} else {
		if value, ok := secret.Data["value"]; ok {
			if clientConfig, err := clientcmd.NewClientConfigFromBytes(value); err == nil {
				if restConfig, err := clientConfig.ClientConfig(); err == nil {
					return restConfig, nil
				}
			}
		}
	}
	return nil, errors.NewBadRequest("getClientConfig Error")
}
