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

	"github.com/go-logr/logr"
	clusterv1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/cluster/v1alpha1"
	"github.com/tmax-cloud/hypercloud-multi-operator/controllers/util"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"

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

// ClusterReconciler reconciles a Memcached object
type SecretReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups="",resources=secrets;namespaces;serviceaccounts,verbs=create;delete;get;list;patch;post;update;watch;

func (r *SecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	_ = context.Background()
	log := r.Log.WithValues("Secret", req.NamespacedName)
	log.Info("Start to reconcile kubeconfig secret")
	//get secret
	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{
		Name:      strings.Split(req.NamespacedName.Name, util.KubeconfigSuffix)[0] + util.KubeconfigSuffix,
		Namespace: req.NamespacedName.Namespace,
	}
	if err := r.Get(context.TODO(), secretKey /* req.NamespacedName */, secret); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Secret resource not found. Ignoring since object must be deleted")

			return ctrl.Result{}, nil
		}

		log.Error(err, "Failed to get secret")
		return ctrl.Result{}, err
	}

	//set patch helper
	patchHelper, err := patch.NewHelper(secret, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	defer func() {
		if err := patchHelper.Patch(context.TODO(), secret); err != nil {
			reterr = err
		}
	}()

	// Add finalizer first if not exist to avoid the race condition between init and delete
	// if !controllerutil.ContainsFinalizer(secret, clusterv1alpha1.ClusterManagerFinalizer) {
	// 	controllerutil.AddFinalizer(secret, clusterv1alpha1.ClusterManagerFinalizer)
	// 	return ctrl.Result{}, nil
	// }

	// Handle deletion reconciliation loop.
	if !secret.ObjectMeta.GetDeletionTimestamp().IsZero() {
		// clm := &clusterv1alpha1.ClusterManager{}
		// clmKey := types.NamespacedName{
		// 	Name:      secret.Labels[clusterv1alpha1.LabelKeyClmName],
		// 	Namespace: secret.Labels[clusterv1alpha1.LabelKeyClmNamespace],
		// }
		// if err := r.Get(context.TODO(), clmKey, clm); err != nil {
		// 	// ...
		// }

		// if clm.ObjectMeta.GetDeletionTimestamp().IsZero() {
		// 	return r.reconcileRecreate(context.TODO(), secret)
		// }
		return r.reconcileDelete(context.TODO(), secret)
	}

	return r.reconcile(context.TODO(), secret)
}

// reconcile handles cluster reconciliation.
func (r *SecretReconciler) reconcile(ctx context.Context, secret *corev1.Secret) (ctrl.Result, error) {
	phases := []func(context.Context, *corev1.Secret) (ctrl.Result, error){
		r.UpdateClusterManagerControlPlaneEndpoint,
		r.DeployRolebinding,
		r.DeployArgocdResources,
	}

	res := ctrl.Result{}
	errs := []error{}
	for _, phase := range phases {
		// Call the inner reconciliation methods.
		phaseResult, err := phase(ctx, secret)
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

func (r *SecretReconciler) reconcileDelete(ctx context.Context, secret *corev1.Secret) (reconcile.Result, error) {
	log := r.Log.WithValues(
		"secret",
		types.NamespacedName{
			Name:      secret.Name,
			Namespace: secret.Namespace,
		},
	)
	log.Info("Start to reconcile delete")

	remoteClientset, err := util.GetRemoteK8sClient(secret)
	if err != nil {
		log.Error(err, "Failed to get remoteK8sClient")
		return ctrl.Result{}, err
	}

	saList := []string{
		util.ArgoServiceAccount,
	}
	for _, targetSa := range saList {
		if _, err :=
			remoteClientset.
				CoreV1().
				ServiceAccounts(util.KubeNamespace).
				Get(
					context.TODO(),
					targetSa,
					metav1.GetOptions{},
				); err != nil {
			if errors.IsNotFound(err) {
				log.Info("Cannot found ServiceAccount [" + targetSa + "] from remote cluster. Maybe already deleted")
			} else {
				log.Error(err, "Failed to get ServiceAccount ["+targetSa+"] from remote cluster")
				return ctrl.Result{}, err
			}
		} else {
			log.Info("Start to delete ServiceAccount [" + targetSa + "] from remote cluster")
			if err :=
				remoteClientset.
					CoreV1().
					ServiceAccounts(util.KubeNamespace).
					Delete(
						context.TODO(),
						targetSa,
						metav1.DeleteOptions{},
					); err != nil {
				log.Error(err, "Cannot delete ServiceAccount ["+targetSa+"] from remote cluster")
				return ctrl.Result{}, err
			}
			log.Info("Delete ServiceAccount [" + targetSa + "] from remote cluster successfully")
		}
	}

	crbList := []string{
		"cluster-owner-crb-" + secret.Annotations[util.AnnotationKeyOwner],
		//"hypercloud-admin-clusterrolebinding",
		util.ArgoClusterRoleBinding,
	}
	for _, targetCrb := range crbList {
		if _, err :=
			remoteClientset.
				RbacV1().
				ClusterRoleBindings().
				Get(
					context.TODO(),
					targetCrb,
					metav1.GetOptions{},
				); err != nil {
			if errors.IsNotFound(err) {
				log.Info("Cannot found ClusterRoleBinding [" + targetCrb + "] from remote cluster. Maybe already deleted")
			} else {
				log.Error(err, "Failed to get clusterrolebinding ["+targetCrb+"] from remote cluster")
				return ctrl.Result{}, err
			}
		} else {
			log.Info("Start to delete clusterrolebinding [" + targetCrb + "] from remote cluster")
			if err :=
				remoteClientset.
					RbacV1().
					ClusterRoleBindings().
					Delete(
						context.TODO(),
						targetCrb,
						metav1.DeleteOptions{},
					); err != nil {
				log.Error(err, "Cannot delete ClusterRoleBinding ["+targetCrb+"] from remote cluster")
				return ctrl.Result{}, err
			}
			log.Info("Delete ClusterRoleBinding [" + targetCrb + "] from remote cluster successfully")
		}
	}

	crList := []string{
		"developer",
		"guest",
		util.ArgoClusterRole,
	}
	for _, targetCr := range crList {
		if _, err :=
			remoteClientset.
				RbacV1().
				ClusterRoles().
				Get(
					context.TODO(),
					targetCr,
					metav1.GetOptions{},
				); err != nil {
			if errors.IsNotFound(err) {
				log.Info("Cannot found ClusterRole [" + targetCr + "] from remote cluster. Maybe already deleted")
			} else {
				log.Error(err, "Failed to get ClusterRole ["+targetCr+"] from remote cluster")
				return ctrl.Result{}, err
			}
		} else {
			log.Info("Start to delete ClusterRole [" + targetCr + "] from remote cluster")
			if err :=
				remoteClientset.
					RbacV1().
					ClusterRoles().
					Delete(
						context.TODO(),
						targetCr,
						metav1.DeleteOptions{},
					); err != nil {
				log.Error(err, "Cannot delete ClusterRole ["+targetCr+"] from remote cluster")
				return ctrl.Result{}, err
			}
			log.Info("Delete ClusterRole [" + targetCr + "] from remote cluster successfully")
		}
	}

	argoClusterSecret := &corev1.Secret{}
	secretKey := types.NamespacedName{
		Name:      secret.Annotations[util.AnnotationKeyArgoClusterSecret],
		Namespace: util.ArgoNamespace,
	}
	if err := r.Get(context.TODO(), secretKey, argoClusterSecret); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Cannot found Secret [" + argoClusterSecret.Name + "]. Maybe already deleted")
		} else {
			log.Error(err, "Failed to get Secret ["+argoClusterSecret.Name+"]")
			return ctrl.Result{}, err
		}
	} else {
		log.Info("Start to delete Secret [" + argoClusterSecret.Name + "]")
		if err := r.Delete(context.TODO(), argoClusterSecret); err != nil {
			log.Error(err, "Cannot delete Secret ["+argoClusterSecret.Name+"]")
		}
		log.Info("Delete Secret [" + argoClusterSecret.Name + "] successfully")
	}
	// 클러스터를 사용중이던 사용자의 crb도 지워야되나.. db에서 읽어서 지워야 하는데?

	controllerutil.RemoveFinalizer(secret, clusterv1alpha1.ClusterManagerFinalizer)
	// controllerutil.RemoveFinalizer(argoClusterSecret, clusterv1alpha1.ClusterManagerFinalizer)
	return ctrl.Result{}, nil
}

func (r *SecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	controller, err := ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		WithEventFilter(
			predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					// secret := e.Object.(*corev1.Secret).DeepCopy()
					// if secret.Labels[util.LabelKeyClmSecretType] == util.ClmSecretTypeArgo {
					// 	return true
					// }
					return false
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldSecret := e.ObjectOld.(*corev1.Secret).DeepCopy()
					newSecret := e.ObjectNew.(*corev1.Secret).DeepCopy()
					isTarget := strings.Contains(oldSecret.Name, util.KubeconfigSuffix)
					isDelete := oldSecret.GetDeletionTimestamp().IsZero() && !newSecret.GetDeletionTimestamp().IsZero()
					isFinalized := !controllerutil.ContainsFinalizer(oldSecret, clusterv1alpha1.ClusterManagerFinalizer) &&
						controllerutil.ContainsFinalizer(newSecret, clusterv1alpha1.ClusterManagerFinalizer)

					if isTarget && (isDelete || isFinalized) {
						return true
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

	return controller.Watch(
		&source.Kind{Type: &clusterv1alpha1.ClusterManager{}},
		&handler.EnqueueRequestForObject{},
		predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldClm := e.ObjectOld.(*clusterv1alpha1.ClusterManager)
				newClm := e.ObjectNew.(*clusterv1alpha1.ClusterManager)
				if !oldClm.Status.ControlPlaneReady && newClm.Status.ControlPlaneReady {
					return true
				}
				return false
			},
			CreateFunc: func(e event.CreateEvent) bool {
				return false
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
