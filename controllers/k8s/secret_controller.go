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
	clusterV1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/cluster/v1alpha1"
	"github.com/tmax-cloud/hypercloud-multi-operator/controllers/util"

	coreV1 "k8s.io/api/core/v1"
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
	secretName := req.Name
	isArgoSecret := strings.Contains(secretName, "cluster-")
	if !isArgoSecret && !strings.Contains(secretName, util.KubeconfigSuffix) {
		secretName += util.KubeconfigSuffix
	}
	key := types.NamespacedName{
		Name:      secretName,
		Namespace: req.Namespace,
	}
	log := r.Log.WithValues("Secret", key)
	log.Info("Start to reconcile secret")

	//get secret
	secret := &coreV1.Secret{}
	if err := r.Get(context.TODO(), key, secret); errors.IsNotFound(err) {
		log.Info("Secret resource not found. Ignoring since object must be deleted")
		return ctrl.Result{}, nil
	} else if err != nil {
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
	if !controllerutil.ContainsFinalizer(secret, clusterV1alpha1.ClusterManagerFinalizer) {
		controllerutil.AddFinalizer(secret, clusterV1alpha1.ClusterManagerFinalizer)
		return ctrl.Result{}, nil
	}

	// Handle deletion reconciliation loop.
	if !secret.ObjectMeta.GetDeletionTimestamp().IsZero() {
		return r.reconcileDelete(context.TODO(), secret)
	}

	return r.reconcile(context.TODO(), secret)
}

// reconcile handles cluster reconciliation.
func (r *SecretReconciler) reconcile(ctx context.Context, secret *coreV1.Secret) (ctrl.Result, error) {
	phases := []func(context.Context, *coreV1.Secret) (ctrl.Result, error){
		r.UpdateClusterManagerControlPlaneEndpoint,
		r.DeployRolebinding,
		r.DeployArgocdResources,
		// r.DeployOpensearchResources,
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

func (r *SecretReconciler) reconcileDelete(ctx context.Context, secret *coreV1.Secret) (reconcile.Result, error) {
	key := types.NamespacedName{
		Name:      secret.Name,
		Namespace: secret.Namespace,
	}
	log := r.Log.WithValues("secret", key)
	log.Info("Start to reconcile delete")

	key = types.NamespacedName{
		Name:      secret.Labels[clusterV1alpha1.LabelKeyClmName],
		Namespace: secret.Labels[clusterV1alpha1.LabelKeyClmNamespace],
	}
	clm := &clusterV1alpha1.ClusterManager{}
	if err := r.Get(context.TODO(), key, clm); err != nil {
		log.Error(err, "Failed to get ClusterManager")
		return ctrl.Result{}, err
	}

	if clm.GetDeletionTimestamp().IsZero() {
		if secret.Labels[util.LabelKeyClmSecretType] == util.ClmSecretTypeArgo {
			helper, _ := patch.NewHelper(clm, r.Client)
			defer func() {
				if err := helper.Patch(context.TODO(), clm); err != nil {
					r.Log.Error(err, "ClusterManager patch error")
				}
			}()

			clm.Status.ArgoReady = false
			controllerutil.RemoveFinalizer(secret, clusterV1alpha1.ClusterManagerFinalizer)
		} else {
			key = types.NamespacedName{
				Name:      clm.Labels[clusterV1alpha1.LabelKeyClrName],
				Namespace: secret.Labels[clusterV1alpha1.LabelKeyClmNamespace],
			}
			clr := &clusterV1alpha1.ClusterRegistration{}
			if err := r.Get(context.TODO(), key, clr); err != nil {
				log.Error(err, "Failed to get ClusterRegistration")
				return ctrl.Result{}, err
			}

			helper, _ := patch.NewHelper(clr, r.Client)
			defer func() {
				if err := helper.Patch(context.TODO(), clr); err != nil {
					r.Log.Error(err, "ClusterRegistration patch error")
				}
			}()

			clr.Status.Phase = "Validated"
			clr.Status.Reason = "kubeconfig secret is deleted"
			controllerutil.RemoveFinalizer(secret, clusterV1alpha1.ClusterManagerFinalizer)
		}

		controllerutil.RemoveFinalizer(secret, clusterV1alpha1.ClusterManagerFinalizer)
		return ctrl.Result{}, nil
	}

	if secret.Labels[util.LabelKeyClmSecretType] == util.ClmSecretTypeArgo {
		controllerutil.RemoveFinalizer(secret, clusterV1alpha1.ClusterManagerFinalizer)
		return ctrl.Result{}, nil
	}

	remoteClientset, err := util.GetRemoteK8sClient(secret)
	if err != nil {
		log.Error(err, "Failed to get remoteK8sClient")
		return ctrl.Result{}, err
	}

	saList := []types.NamespacedName{
		{
			Name:      util.ArgoServiceAccount,
			Namespace: util.KubeNamespace,
		},
	}
	for _, targetSa := range saList {
		_, err := remoteClientset.
			CoreV1().
			ServiceAccounts(targetSa.Namespace).
			Get(context.TODO(), targetSa.Name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			log.Info("Cannot found ServiceAccount [" + targetSa.String() + "] from remote cluster. Maybe already deleted")
		} else if err != nil {
			log.Error(err, "Failed to get ServiceAccount ["+targetSa.String()+"] from remote cluster")
			return ctrl.Result{}, err
		} else {
			err := remoteClientset.
				CoreV1().
				ServiceAccounts(targetSa.Namespace).
				Delete(context.TODO(), targetSa.Name, metav1.DeleteOptions{})
			if err != nil {
				log.Error(err, "Cannot delete ServiceAccount ["+targetSa.String()+"] from remote cluster")
				return ctrl.Result{}, err
			}
			log.Info("Delete ServiceAccount [" + targetSa.String() + "] from remote cluster successfully")
		}
	}

	secretList := []types.NamespacedName{
		{
			Name:      util.ArgoServiceAccountTokenSecret,
			Namespace: util.KubeNamespace,
		},
	}
	for _, targetSecret := range secretList {
		_, err := remoteClientset.
			CoreV1().
			Secrets(targetSecret.Namespace).
			Get(context.TODO(), targetSecret.Name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			log.Info("Cannot found Secret [" + targetSecret.String() + "] from remote cluster. Maybe already deleted")
		} else if err != nil {
			log.Error(err, "Failed to get Secret ["+targetSecret.String()+"] from remote cluster")
			return ctrl.Result{}, err
		} else {
			err := remoteClientset.
				CoreV1().
				Secrets(targetSecret.Namespace).
				Delete(context.TODO(), targetSecret.Name, metav1.DeleteOptions{})
			if err != nil {
				log.Error(err, "Cannot delete Secret ["+targetSecret.String()+"] from remote cluster")
				return ctrl.Result{}, err
			}
			log.Info("Delete Secret [" + targetSecret.String() + "] from remote cluster successfully")
		}
	}

	crbList := []string{
		"cluster-owner-crb-" + secret.Annotations[util.AnnotationKeyOwner],
		//"hypercloud-admin-clusterrolebinding",
		util.ArgoClusterRoleBinding,
	}
	for _, targetCrb := range crbList {
		_, err := remoteClientset.
			RbacV1().
			ClusterRoleBindings().
			Get(context.TODO(), targetCrb, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			log.Info("Cannot found ClusterRoleBinding [" + targetCrb + "] from remote cluster. Maybe already deleted")
		} else if err != nil {
			log.Error(err, "Failed to get clusterrolebinding ["+targetCrb+"] from remote cluster")
			return ctrl.Result{}, err
		} else {
			err := remoteClientset.
				RbacV1().
				ClusterRoleBindings().
				Delete(context.TODO(), targetCrb, metav1.DeleteOptions{})
			if err != nil {
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
		_, err := remoteClientset.
			RbacV1().
			ClusterRoles().
			Get(context.TODO(), targetCr, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			log.Info("Cannot found ClusterRole [" + targetCr + "] from remote cluster. Maybe already deleted")
		} else if err != nil {
			log.Error(err, "Failed to get ClusterRole ["+targetCr+"] from remote cluster")
			return ctrl.Result{}, err
		} else {
			err := remoteClientset.
				RbacV1().
				ClusterRoles().
				Delete(context.TODO(), targetCr, metav1.DeleteOptions{})
			if err != nil {
				log.Error(err, "Cannot delete ClusterRole ["+targetCr+"] from remote cluster")
				return ctrl.Result{}, err
			}
			log.Info("Delete ClusterRole [" + targetCr + "] from remote cluster successfully")
		}
	}

	key = types.NamespacedName{
		Name:      secret.Annotations[util.AnnotationKeyArgoClusterSecret],
		Namespace: util.ArgoNamespace,
	}
	argoClusterSecret := &coreV1.Secret{}
	if err := r.Get(context.TODO(), key, argoClusterSecret); errors.IsNotFound(err) {
		log.Info("Cannot found Secret for argocd external cluster [" + argoClusterSecret.Name + "]. Maybe already deleted")
	} else if err != nil {
		log.Error(err, "Failed to get Secret for argocd external cluster ["+argoClusterSecret.Name+"]")
		return ctrl.Result{}, err
	} else {
		if err := r.Delete(context.TODO(), argoClusterSecret); err != nil {
			log.Error(err, "Cannot delete Secret for argocd external cluster ["+argoClusterSecret.Name+"]")
			return ctrl.Result{}, err
		}
		log.Info("Delete Secret for argocd external cluster [" + argoClusterSecret.Name + "] successfully")
	}
	// 클러스터를 사용중이던 사용자의 crb도 지워야되나.. db에서 읽어서 지워야 하는데?

	// _, err = remoteClientset.
	// 	CoreV1().
	// 	Secrets(util.OpenSearchNamespace).
	// 	Get(context.TODO(), "hyperauth-ca", metav1.GetOptions{})
	// if errors.IsNotFound(err) {
	// 	log.Info("Cannot found Secret for opensearch. Maybe already deleted")
	// } else if err != nil {
	// 	log.Error(err, "Cannot found Secret for search")
	// 	return ctrl.Result{}, err
	// } else {
	// 	err = remoteClientset.
	// 		CoreV1().
	// 		Secrets(util.OpenSearchNamespace).
	// 		Delete(context.TODO(), "hyperauth-ca", metav1.DeleteOptions{})
	// 	if err != nil {
	// 		log.Error(err, "Failed to delete secret for opensearch")
	// 		return ctrl.Result{}, err
	// 	}
	// 	log.Info("Delete secret for opensearch successfully")
	// }

	controllerutil.RemoveFinalizer(secret, clusterV1alpha1.ClusterManagerFinalizer)
	return ctrl.Result{}, nil
}

func (r *SecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	controller, err := ctrl.NewControllerManagedBy(mgr).
		For(&coreV1.Secret{}).
		WithEventFilter(
			predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					return false
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldSecret := e.ObjectOld.(*coreV1.Secret).DeepCopy()
					newSecret := e.ObjectNew.(*coreV1.Secret).DeepCopy()
					_, isTarget := oldSecret.Labels[util.LabelKeyClmSecretType]
					isDelete := oldSecret.GetDeletionTimestamp().IsZero() && !newSecret.GetDeletionTimestamp().IsZero()
					isFinalized := !controllerutil.ContainsFinalizer(oldSecret, clusterV1alpha1.ClusterManagerFinalizer) &&
						controllerutil.ContainsFinalizer(newSecret, clusterV1alpha1.ClusterManagerFinalizer)
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
		&source.Kind{Type: &clusterV1alpha1.ClusterManager{}},
		&handler.EnqueueRequestForObject{},
		predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldClm := e.ObjectOld.(*clusterV1alpha1.ClusterManager)
				newClm := e.ObjectNew.(*clusterV1alpha1.ClusterManager)
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
