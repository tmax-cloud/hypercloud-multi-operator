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
	b64 "encoding/base64"
	// "strconv"

	"github.com/go-logr/logr"
	clusterv1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/cluster/v1alpha1"
	util "github.com/tmax-cloud/hypercloud-multi-operator/controllers/util"

	// yaml "gopkg.in/yaml.v2"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"

	// "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"

	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// ClusterRegistrationReconciler reconciles a ClusterRegistration object
type ClusterRegistrationReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=cluster.tmax.io,resources=clusterregistrations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.tmax.io,resources=clusterregistrations/status,verbs=get;update;patch

func (r *ClusterRegistrationReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	_ = context.Background()
	log := r.Log.WithValues("clusterregistration", req.NamespacedName)

	ClusterRegistration := &clusterv1alpha1.ClusterRegistration{}
	if err := r.Get(context.TODO(), req.NamespacedName, ClusterRegistration); err != nil {
		if errors.IsNotFound(err) {
			log.Info("ClusterRegistration resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get ClusterRegistration")
		return ctrl.Result{}, err
	}

	patchHelper, err := patch.NewHelper(ClusterRegistration, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	defer func() {
		// Always reconcile the Status.Phase field.
		r.reconcilePhase(context.TODO(), ClusterRegistration)

		if err := patchHelper.Patch(context.TODO(), ClusterRegistration); err != nil {
			// if err := patchClusterRegistration(context.TODO(), patchHelper, ClusterRegistration, patchOpts...); err != nil {
			// reterr = kerrors.NewAggregate([]error{reterr, err})
			reterr = err
		}
	}()

	// Handle normal reconciliation loop.
	return r.reconcile(context.TODO(), ClusterRegistration)
}

// reconcile handles cluster reconciliation.
func (r *ClusterRegistrationReconciler) reconcile(ctx context.Context, ClusterRegistration *clusterv1alpha1.ClusterRegistration) (ctrl.Result, error) {
	phases := []func(context.Context, *clusterv1alpha1.ClusterRegistration) (ctrl.Result, error){
		r.CheckValidation,
		r.CreateClusterManager,
		r.CreateKubeconfigSecret,
	}

	res := ctrl.Result{}
	errs := []error{}
	for _, phase := range phases {
		// Call the inner reconciliation methods.
		phaseResult, err := phase(ctx, ClusterRegistration)
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
func (r *ClusterRegistrationReconciler) CheckValidation(ctx context.Context, ClusterRegistration *clusterv1alpha1.ClusterRegistration) (ctrl.Result, error) {
	log := r.Log.WithValues("ClusterRegistration", types.NamespacedName{Name: ClusterRegistration.Name, Namespace: ClusterRegistration.Namespace})
	log.Info("Start to CheckValidation reconcile for [" + ClusterRegistration.Name + "]")
	ClusterRegistration.Status.SetTypedPhase(clusterv1alpha1.ClusterRegistrationPhaseValidating)

	// decode base64 encoded kubeconfig file
	if encodedKubeConfig, err := b64.StdEncoding.DecodeString(ClusterRegistration.Spec.KubeConfig); err != nil {
		log.Error(err, "Failed to decode ClusterRegistration.Spec.KubeConfig, maybe wrong kubeconfig file")
		ClusterRegistration.Status.SetTypedPhase(clusterv1alpha1.ClusterRegistrationPhaseFailed)
		ClusterRegistration.Status.SetTypedReason(clusterv1alpha1.ClusterRegistrationReasonInvalidKubeconfig)
		return ctrl.Result{Requeue: false}, err
	} else {
		// validate remote cluster
		log.Info("Start to CheckKubeconfigValidation reconcile for [" + ClusterRegistration.Name + "]")
		if remoteClientset, err := util.GetRemoteK8sClientByKubeConfig(encodedKubeConfig); err != nil {
			log.Error(err, "Failed to get client for ["+ClusterRegistration.Spec.ClusterName+"]")
			return ctrl.Result{}, err
		} else {
			// TODO
			// nodelist가 아닌 api-server call검증 api는 따로 없나...?
			if nodeList, err := remoteClientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{}); err != nil {
				if len(nodeList.Items) == 0 {
					// this may cause functinon panic
					// go to defer func
					log.Info("Failed to get nodes for [" + ClusterRegistration.Spec.ClusterName + "]")
					panic(err)					
				}
				defer func() (ctrl.Result, error) {
					recover()
					ClusterRegistration.Status.SetTypedPhase(clusterv1alpha1.ClusterRegistrationPhaseFailed)
					ClusterRegistration.Status.SetTypedReason(clusterv1alpha1.ClusterRegistrationReasonClusterNotFound)
					return ctrl.Result{Requeue: false}, nil
				}()
			}
		}
	}

	// validate cluster manger duplication
	clm := clusterv1alpha1.ClusterManager{}
	clmKey := types.NamespacedName{Name: ClusterRegistration.Spec.ClusterName, Namespace: ClusterRegistration.Namespace}
	if err := r.Get(context.TODO(), clmKey, &clm); err != nil {
		if errors.IsNotFound(err) {
			log.Info("ClusterManager [" + ClusterRegistration.Spec.ClusterName + "] does not exist. Duplication condition is passed")
		} else {
			log.Error(err, "Failed to get clusterManager")
			return ctrl.Result{}, err
		}
	} else {
		log.Info("ClusterManager [" + clm.Name + "] is already existed")
		ClusterRegistration.Status.SetTypedPhase(clusterv1alpha1.ClusterRegistrationPhaseFailed)
		ClusterRegistration.Status.SetTypedReason(clusterv1alpha1.ClusterRegistrationReasonClusterNameDuplicated)
		return ctrl.Result{Requeue: false}, err
	}

	ClusterRegistration.Status.SetTypedPhase(clusterv1alpha1.ClusterRegistrationPhaseValidated)
	return ctrl.Result{}, nil
}

func (r *ClusterRegistrationReconciler) CreateKubeconfigSecret(ctx context.Context, ClusterRegistration *clusterv1alpha1.ClusterRegistration) (ctrl.Result, error) {
	log := r.Log.WithValues("ClusterRegistration", types.NamespacedName{Name: ClusterRegistration.Name, Namespace: ClusterRegistration.Namespace})
	log.Info("Wait for ClusterRegistration validation.....")

	if ClusterRegistration.Status.Phase == string(clusterv1alpha1.ClusterRegistrationPhaseFailed) {
		return ctrl.Result{}, nil
	} else if ClusterRegistration.Status.Phase != string(clusterv1alpha1.ClusterRegistrationPhaseValidated) {
		return ctrl.Result{Requeue: true}, nil
	}

	log.Info("Start to CreateKubeconfigSecret reconcile for [" + ClusterRegistration.Name + "]")

	kubeconfigSecret := &corev1.Secret{}
	kubeconfigSecretKey := types.NamespacedName{Name: ClusterRegistration.Spec.ClusterName + "-kubeconfig", Namespace: ClusterRegistration.Namespace}
	if err := r.Get(context.TODO(), kubeconfigSecretKey, kubeconfigSecret); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Cannot found kubeconfigSecret, starting to create kubeconfigSecret for [" + ClusterRegistration.Name + "-kubeconfig" + "]")
			if encodedKubeConfig, err := b64.StdEncoding.DecodeString(ClusterRegistration.Spec.KubeConfig); err != nil {
				log.Error(err, "Failed to decode ClusterRegistration.Spec.KubeConfig, maybe wrong kubeconfig file")
				return ctrl.Result{}, err
			} else {
				kubeconfigSecret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      ClusterRegistration.Spec.ClusterName + "-kubeconfig",
						Namespace: ClusterRegistration.Namespace,
						Finalizers: []string{
							util.SecretFinalizer,
						},
					},
					StringData: map[string]string{
						"value": string(encodedKubeConfig),
					},
				}
			}

			if err = r.Create(context.TODO(), kubeconfigSecret); err != nil {
				log.Error(err, "Failed to create ["+ClusterRegistration.Spec.ClusterName+"-kubeconfig] secret")
				return ctrl.Result{}, err
			}
		} else {
			log.Error(err, "Failed to get kubeconfigSecret")
			return ctrl.Result{}, err
		}
	} else {
		log.Info("Kubeconfig secret is already exist")
		// 왜 시크릿이 이미 존재하지...? 케이스 정리부터 필요한거같기도
		// 이미 존재하는 경우에는 어떻게 처리할까...?
	}

	return ctrl.Result{}, nil
}

func (r *ClusterRegistrationReconciler) CreateClusterManager(ctx context.Context, ClusterRegistration *clusterv1alpha1.ClusterRegistration) (ctrl.Result, error) {
	log := r.Log.WithValues("ClusterRegistration", types.NamespacedName{Name: ClusterRegistration.Name, Namespace: ClusterRegistration.Namespace})
	log.Info("Wait for ClusterRegistration validation.....")

	if ClusterRegistration.Status.Phase == string(clusterv1alpha1.ClusterRegistrationPhaseFailed) {
		return ctrl.Result{}, nil
	} else if ClusterRegistration.Status.Phase != string(clusterv1alpha1.ClusterRegistrationPhaseValidated) {
		return ctrl.Result{Requeue: true}, nil
	}

	log.Info("Start to CreateClusterManager reconcile for [" + ClusterRegistration.Name + "]")

	clm := &clusterv1alpha1.ClusterManager{}
	clmKey := types.NamespacedName{Name: ClusterRegistration.Spec.ClusterName, Namespace: ClusterRegistration.Namespace}
	if err := r.Get(context.TODO(), clmKey, clm); err != nil {
		if errors.IsNotFound(err) {
			clm = &clusterv1alpha1.ClusterManager{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ClusterRegistration.Spec.ClusterName,
					Namespace: ClusterRegistration.Namespace,
					Annotations: map[string]string{
						"owner":   ClusterRegistration.Annotations["creator"],
						"creator": ClusterRegistration.Annotations["creator"],
					},
					Labels: map[string]string{
						util.ClusterTypeKey: util.ClusterTypeRegistered,
						"parent":            ClusterRegistration.Name,
					},
				},
				Spec: clusterv1alpha1.ClusterManagerSpec{},
			}
			if err = r.Create(context.TODO(), clm); err != nil {
				log.Error(err, "Failed to create ClusterManager for ["+ClusterRegistration.Spec.ClusterName+"]")
				return ctrl.Result{}, err
			}

			if err := util.Insert(clm); err != nil {
				log.Error(err, "Failed to insert cluster info into cluster_member table")
				return ctrl.Result{}, err
			}

		} else {
			log.Error(err, "Failed to get ClusterManager")
			return ctrl.Result{}, err
		}
	} else {
		log.Info("Cannot create ClusterManager. ClusterManager is already exsist")
		// 왜 clm 이미 존재하지...? 케이스 정리부터 필요한거같기도
		// 이미 존재하는 경우에는 어떻게 처리할까...?
	}

	return ctrl.Result{}, nil
}

func (r *ClusterRegistrationReconciler) reconcilePhase(_ context.Context, ClusterRegistration *clusterv1alpha1.ClusterRegistration) {
	if ClusterRegistration.Status.Phase == "validated" {
		ClusterRegistration.Status.SetTypedPhase(clusterv1alpha1.ClusterRegistrationPhaseSuccess)
	}
}

func (r *ClusterRegistrationReconciler) requeueClusterRegistrationsForClusterManager(o handler.MapObject) []ctrl.Request {
	clm := o.Object.(*clusterv1alpha1.ClusterManager)
	log := r.Log.WithValues("ClusterRegistration-ObjectMapper", "clusterManagerToClusterClusterRegistrations", "ClusterRegistration", clm.Name)
	//get clusterManager
	clr := &clusterv1alpha1.ClusterRegistration{}
	key := types.NamespacedName{Namespace: clm.Namespace, Name: clm.Labels["parent"]}
	if err := r.Get(context.TODO(), key, clr); err != nil {
		if errors.IsNotFound(err) {
			log.Info("ClusterRegistration resource not found. Ignoring since object must be deleted.")
			return nil
		}
		log.Error(err, "Failed to get ClusterRegistration")
		return nil
	}

	if clr.Status.Phase != "Success" {
		log.Info("ClusterRegistration for ClusterManager [" + clr.Spec.ClusterName + "] is already delete... Do not update cc status to delete ")
		return nil
	}
	clr.Status.Phase = "Deleted"
	clr.Status.Reason = "cluster is deleted"
	err := r.Status().Update(context.TODO(), clr)
	if err != nil {
		log.Error(err, "Failed to update ClusterClaim status")
		return nil //??
	}
	return nil
}

func (r *ClusterRegistrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	controller, err := ctrl.NewControllerManagedBy(mgr).
		For(&clusterv1alpha1.ClusterRegistration{}).
		WithEventFilter(
			predicate.Funcs{
				// Avoid reconciling if the event triggering the reconciliation is related to incremental status updates
				// for kubefedcluster resources only
				CreateFunc: func(e event.CreateEvent) bool {
					// phase success 일 때 한번 들어오는데.. 왜 그러냐... controller 재기동 돼서?
					clr := e.Object.(*clusterv1alpha1.ClusterRegistration)
					if clr.Status.Phase == "" {
						return true
					} else {
						return false
					}
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
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

	return controller.Watch(
		&source.Kind{Type: &clusterv1alpha1.ClusterManager{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(r.requeueClusterRegistrationsForClusterManager),
		},
		predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				return false
			},
			CreateFunc: func(e event.CreateEvent) bool {
				return false
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				clm := e.Object.(*clusterv1alpha1.ClusterManager)
				if val, ok := clm.Labels[util.ClusterTypeKey]; ok && val == util.ClusterTypeRegistered {
					return true
				}
				return false
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return false
			},
		},
	)

}
