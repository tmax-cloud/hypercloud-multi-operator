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
	clusterv1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/cluster/v1alpha1"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/tmax-cloud/hypercloud-multi-operator/controllers/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	//	fedv1b1 "sigs.k8s.io/kubefed/pkg/apis/core/v1beta1"
)

// ClusterReconciler reconciles a Memcached object
type SecretReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

const (
	requeueAfter5Sec = 5 * time.Second
	// requeueAfter5Sec = 5 * time.Second
)

// +kubebuilder:rbac:groups="",resources=secrets;namespaces,verbs=create;delete;get;list;patch;post;update;watch;

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
	if err := r.Get(context.TODO(), secretKey, secret); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Secret resource not found. Ignoring since object must be deleted.")
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
	if !controllerutil.ContainsFinalizer(secret, util.SecretFinalizer) {
		controllerutil.AddFinalizer(secret, util.SecretFinalizer)
		return ctrl.Result{}, nil
	}

	// Handle deletion reconciliation loop.
	if !secret.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(context.TODO(), secret)
	}

	return r.reconcile(context.TODO(), secret)
}

// reconcile handles cluster reconciliation.
func (r *SecretReconciler) reconcile(ctx context.Context, secret *corev1.Secret) (ctrl.Result, error) {
	phases := []func(context.Context, *corev1.Secret) (ctrl.Result, error){
		r.UpdateClusterManagerControlPlaneEndpoint,
		//r.KubefedJoin,
		r.deployRolebinding,
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

func (r *SecretReconciler) deployRolebinding(ctx context.Context, secret *corev1.Secret) (ctrl.Result, error) {
	log := r.Log.WithValues("secret", types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace})

	log.Info("Start to reconcile.. Deploy rolebinding to remote")

	clm := &clusterv1alpha1.ClusterManager{}
	clmKey := types.NamespacedName{
		Name:      strings.Split(secret.Name, util.KubeconfigSuffix)[0],
		Namespace: secret.Namespace,
	}
	if err := r.Get(context.TODO(), clmKey, clm); err != nil {
		log.Error(err, "Failed to get ClusterManager")
		return ctrl.Result{}, err
	}

	remoteClientset, err := util.GetRemoteK8sClient(secret)
	if err != nil {
		log.Error(err, "Failed to get remoteK8sClient")
		return ctrl.Result{}, err
	}

	clusterAdminCRBName := "cluster-owner-crb-" + clm.Annotations["owner"]
	clusterAdminCRB := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterAdminCRBName,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.SchemeGroupVersion.Group, //"rbac.authorization.k8s.io"
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     rbacv1.UserKind,  //"User",
				APIGroup: rbacv1.GroupName, //"rbac.authorization.k8s.io",
				Name:     clm.Annotations["owner"],
			},
		},
	}

	targetGroups := []string{
		"",
		"apps",
		"autoscaling",
		"batch",
		"extensions",
		"policy",
		"networking.k8s.io",
		"snapshot.storage.k8s.io",
		"storage.k8s.io",
		"apiextensions.k8s.io",
		"metrics.k8s.io",
	}

	allRule := &rbacv1.PolicyRule{}
	allRule.APIGroups = append(allRule.APIGroups, targetGroups...)
	//allRule.APIGroups = append(allRule.APIGroups, "", "apps", "autoscaling", "batch", "extensions", "policy", "networking.k8s.io", "snapshot.storage.k8s.io", "storage.k8s.io", "apiextensions.k8s.io", "metrics.k8s.io")
	allRule.Resources = append(allRule.Resources, rbacv1.ResourceAll)
	allRule.Verbs = append(allRule.Verbs, rbacv1.VerbAll)

	if _, err := remoteClientset.RbacV1().ClusterRoleBindings().Get(context.TODO(), clusterAdminCRBName, metav1.GetOptions{}); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Cannot found cluster-admin crb from remote cluster. Start to create cluster-admin clusterrolebinding to remote")
			if _, err := remoteClientset.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterAdminCRB, metav1.CreateOptions{}); err != nil {
				log.Error(err, "Cannnot create clusterrolebinding for cluster-admin")
				return ctrl.Result{}, err
			}
		} else {
			log.Error(err, "Failed to get cluster-admin clusterrolebinding from remote cluster")
			return ctrl.Result{}, err
		}
	}

	developerClusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "developer",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: targetGroups,
				Resources: []string{rbacv1.ResourceAll},
				Verbs:     []string{rbacv1.VerbAll},
			},
			{
				APIGroups: []string{"apiregistration.k8s.io"},
				Resources: []string{rbacv1.ResourceAll},
				Verbs:     []string{"get", "list", "watch"}},
		},
	}

	if _, err := remoteClientset.RbacV1().ClusterRoles().Get(context.TODO(), "developer", metav1.GetOptions{}); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Cannot found developer cr from remote cluster. Start to create developer clusterrole to remote")
			if _, err := remoteClientset.RbacV1().ClusterRoles().Create(context.TODO(), developerClusterRole, metav1.CreateOptions{}); err != nil {
				log.Error(err, "Cannnot create clusterrole for developer")
				return ctrl.Result{}, err
			}
		} else {
			log.Error(err, "Failed to get developer clusterrole from remote cluster")
			return ctrl.Result{}, err
		}
	}

	guestClusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "guest",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: targetGroups,
				Resources: []string{rbacv1.ResourceAll},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"apiregistration.k8s.io"},
				Resources: []string{rbacv1.ResourceAll},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}

	if _, err := remoteClientset.RbacV1().ClusterRoles().Get(context.TODO(), "guest", metav1.GetOptions{}); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Cannot found guest cr from remote cluster. Start to create guest clusterrole to remote")
			if _, err := remoteClientset.RbacV1().ClusterRoles().Create(context.TODO(), guestClusterRole, metav1.CreateOptions{}); err != nil {
				log.Error(err, "Cannnot create clusterrole for guest")
				return ctrl.Result{}, err
			}
		} else {
			log.Error(err, "Failed to get guest clusterrole from remote cluster")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *SecretReconciler) UpdateClusterManagerControlPlaneEndpoint(ctx context.Context, secret *corev1.Secret) (ctrl.Result, error) {
	log := r.Log.WithValues("secret", types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace})

	log.Info("Start to reconcile UpdateClusterManagerControleplaneEndpoint... ")
	kubeConfig, err := clientcmd.Load(secret.Data["value"])
	if err != nil {
		log.Error(err, "Failed to get secret")
		return ctrl.Result{}, err
	}
	clm := &clusterv1alpha1.ClusterManager{}
	clmKey := types.NamespacedName{Name: strings.Split(secret.Name, util.KubeconfigSuffix)[0], Namespace: secret.Namespace}
	if err := r.Get(context.TODO(), clmKey, clm); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Cannot found clusterManager")
			// return ctrl.Result{RequeueAfter: requeueAfter5Sec}, nil
		} else {
			log.Error(err, "Failed to get Secret")
			return ctrl.Result{}, err
		}
	} else {
		if !strings.EqualFold(clm.Status.ControlPlaneEndpoint, kubeConfig.Clusters[kubeConfig.Contexts[kubeConfig.CurrentContext].Cluster].Server) {
			log.Info("Update clustermanager status.. add controleplane endpoint")
			helper, _ := patch.NewHelper(clm, r.Client)
			defer func() {
				if err := helper.Patch(context.TODO(), clm); err != nil {
					log.Error(err, "ClusterManager patch error")
				}
			}()
			clm.Status.ControlPlaneEndpoint = kubeConfig.Clusters[kubeConfig.Contexts[kubeConfig.CurrentContext].Cluster].Server
		}
	}

	return ctrl.Result{}, nil
}

func (r *SecretReconciler) reconcileDelete(ctx context.Context, secret *corev1.Secret) (reconcile.Result, error) {
	log := r.Log.WithValues("secret", types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace})
	log.Info("Start to reconcile reconcileDelete... ")

	remoteClientset, err := util.GetRemoteK8sClient(secret)
	if err != nil {
		log.Error(err, "Failed to get remoteK8sClient")
		return ctrl.Result{}, err
	}

	crbList := []string{
		"cluster-owner-crb-" + secret.Annotations["owner"],
		//"hypercloud-admin-clusterrolebinding",
		"developer",
		"guest",
	}
	for _, targetCrb := range crbList {
		if _, err := remoteClientset.RbacV1().ClusterRoleBindings().Get(context.TODO(), targetCrb, metav1.GetOptions{}); err != nil {
			if errors.IsNotFound(err) {
				log.Info("Cannot found crb [" + targetCrb + "] from remote cluster. Maybe already deleted")
			} else {
				log.Error(err, "Failed to get clusterrolebinding ["+targetCrb+"] from remote cluster")
				return ctrl.Result{}, err
			}
		} else {
			if err := remoteClientset.RbacV1().ClusterRoleBindings().Delete(context.TODO(), targetCrb, metav1.DeleteOptions{}); err != nil {
				log.Error(err, "Cannnot delete clusterrolebinding ["+targetCrb+"]")
				return ctrl.Result{}, err
			}
		}
	}

	// 클러스터를 사용중이던 사용자의 crb도 지워야되나.. db에서 읽어서 지워야 하는데?

	controllerutil.RemoveFinalizer(secret, util.SecretFinalizer)

	return ctrl.Result{}, nil
}

// func (r *SecretReconciler) requeueSecretForClusterManager(o handler.MapObject) []ctrl.Request {
// 	clm := o.Object.(*clusterv1alpha1.ClusterManager)
// 	log := r.Log.WithValues("objectMapper", "ClusterManagersToSecret", "namespace", clm.Namespace, "clustermanager", clm.Name)
// 	log.Info("Start to requeueSecretForClusterManager mapping")

// 	// Don't handle deleted machinedeployment
// 	if !clm.ObjectMeta.DeletionTimestamp.IsZero() {
// 		log.Info("clustermanager has a deletion timestamp, skipping mapping.")
// 		return nil
// 	}

// 	reconcileRequests := []ctrl.Request{}
// 	reconcileRequests = append(reconcileRequests, ctrl.Request{
// 		NamespacedName: types.NamespacedName{
// 			Namespace: clm.Namespace,
// 			Name:      clm.Name + util.KubeconfigSuffix,
// 		},
// 	})
// 	return reconcileRequests
// }

func (r *SecretReconciler) SetupWithManager(mgr ctrl.Manager) error {

	controller, err := ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		WithEventFilter(
			predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					return false
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldSecret := e.ObjectOld.(*corev1.Secret).DeepCopy()
					newSecret := e.ObjectNew.(*corev1.Secret).DeepCopy()
					isTarget := strings.Contains(oldSecret.Name, util.KubeconfigSuffix)
					isDelete := oldSecret.DeletionTimestamp.IsZero() && !newSecret.DeletionTimestamp.IsZero()
					isFinalized := !controllerutil.ContainsFinalizer(oldSecret, util.SecretFinalizer) &&
						controllerutil.ContainsFinalizer(newSecret, util.SecretFinalizer)

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

func getKubeConfig(secret corev1.Secret) (*rest.Config, error) {
	if value, ok := secret.Data["value"]; ok {
		if clientConfig, err := clientcmd.NewClientConfigFromBytes(value); err == nil {
			if restConfig, err := clientConfig.ClientConfig(); err == nil {
				return restConfig, nil
			}
		}
	}
	return nil, errors.NewBadRequest("getClientConfig Error")
}
