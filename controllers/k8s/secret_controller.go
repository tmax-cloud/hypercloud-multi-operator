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

	// "github.com/tmax-cloud/hypercloud-multi-operator/controllers/util"
	"github.com/tmax-cloud/hypercloud-multi-operator/controllers/util"
	constant "github.com/tmax-cloud/hypercloud-multi-operator/controllers/util"
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
	fedv1b1 "sigs.k8s.io/kubefed/pkg/apis/core/v1beta1"
	"sigs.k8s.io/kubefed/pkg/kubefedctl"
)

// ClusterReconciler reconciles a Memcached object
type SecretReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

const (
	ClusterManagerWaitingRequeueAfter = 5 * time.Second
)

// +kubebuilder:rbac:groups="",resources=secrets;namespaces;,verbs=get;update;patch;list;watch;create;post;delete;
// +kubebuilder:rbac:groups="core.kubefed.io",resources=kubefedconfigs;kubefedclusters;,verbs=get;update;patch;list;watch;create;post;delete;

func (r *SecretReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	_ = context.Background()
	log := r.Log.WithValues("Secret", req.NamespacedName)
	log.Info("Start to reconcile kubeconfig secret")
	//get secret
	secret := &corev1.Secret{}
	if err := r.Get(context.TODO(), req.NamespacedName, secret); err != nil {
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
		r.UpdateClusterManagerControleplaneEndpoint,
		r.KubefedJoin,
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
	clmKey := types.NamespacedName{Name: strings.Split(secret.Name, constant.KubeconfigPostfix)[0], Namespace: secret.Namespace}
	if err := r.Get(context.TODO(), clmKey, clm); err != nil {
		log.Error(err, "Failed to get ClusterManager")
		return ctrl.Result{}, err
	}

	remoteClientset, err := util.GetRemoteK8sClient(secret)
	if err != nil {
		log.Error(err, "Failed to get remoteK8sClient")
		return ctrl.Result{}, err
	}

	clusterAdminCRB := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-owner-crb-" + clm.Annotations["owner"],
			Namespace: clm.Namespace,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     "User",
				APIGroup: "rbac.authorization.k8s.io",
				Name:     clm.Annotations["owner"],
			},
		},
	}

	allRule := &rbacv1.PolicyRule{}
	allRule.APIGroups = append(allRule.APIGroups, "", "apps", "autoscaling", "batch", "extensions", "policy", "networking.k8s.io", "snapshot.storage.k8s.io", "storage.k8s.io", "apiextensions.k8s.io", "metrics.k8s.io")
	allRule.Resources = append(allRule.Resources, "*")
	allRule.Verbs = append(allRule.Verbs, "*")

	if _, err := remoteClientset.RbacV1().ClusterRoleBindings().Get(context.TODO(), "cluster-owner-crb-"+clm.Annotations["owner"], metav1.GetOptions{}); err != nil {
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
			{APIGroups: []string{"", "apps", "autoscaling", "batch", "extensions", "policy", "networking.k8s.io", "snapshot.storage.k8s.io", "storage.k8s.io", "apiextensions.k8s.io", "metrics.k8s.io"}, Resources: []string{"*"},
				Verbs: []string{"*"}},
			{APIGroups: []string{"apiregistration.k8s.io"}, Resources: []string{"*"},
				Verbs: []string{"get", "list", "watch"}},
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
			{APIGroups: []string{"", "apps", "autoscaling", "batch", "extensions", "policy", "networking.k8s.io", "snapshot.storage.k8s.io", "storage.k8s.io", "apiextensions.k8s.io", "metrics.k8s.io"}, Resources: []string{"*"},
				Verbs: []string{"get", "list", "watch"}},
			{APIGroups: []string{"apiregistration.k8s.io"}, Resources: []string{"*"},
				Verbs: []string{"get", "list", "watch"}},
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

func (r *SecretReconciler) KubefedJoin(ctx context.Context, secret *corev1.Secret) (ctrl.Result, error) {
	log := r.Log.WithValues("secret", types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace})

	log.Info("Start to reconcile.. Join clustermanager")

	clusterManagerNamespacedName := secret.GetNamespace() + "-" + strings.Split(secret.Name, constant.KubeconfigPostfix)[0]

	clientRestConfig, err := getKubeConfig(*secret)
	if err != nil {
		log.Error(err, "Unable to get rest config from secret")
		return ctrl.Result{}, err
	}
	masterRestConfig := ctrl.GetConfigOrDie()
	// cluster

	kubefedConfig := &fedv1b1.KubeFedConfig{}
	key := types.NamespacedName{Namespace: constant.KubeFedNamespace, Name: "kubefed"}

	if err := r.Get(context.TODO(), key, kubefedConfig); err != nil {
		log.Error(err, "Failed to get kubefedconfig")
		return ctrl.Result{}, err
	}

	if _, err := kubefedctl.JoinCluster(masterRestConfig, clientRestConfig, constant.KubeFedNamespace, constant.HostClusterName,
		clusterManagerNamespacedName, "", kubefedConfig.Spec.Scope, false, false); err != nil {
		log.Error(err, "Failed to join cluster")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *SecretReconciler) UpdateClusterManagerControleplaneEndpoint(ctx context.Context, secret *corev1.Secret) (ctrl.Result, error) {
	log := r.Log.WithValues("secret", types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace})

	log.Info("Start to reconcile UpdateClusterManagerControleplaneEndpoint... ")
	kubeConfig, err := clientcmd.Load(secret.Data["value"])
	if err != nil {
		log.Error(err, "Failed to get secret")
		return ctrl.Result{}, err
	}
	clm := &clusterv1alpha1.ClusterManager{}
	clmKey := types.NamespacedName{Name: strings.Split(secret.Name, constant.KubeconfigPostfix)[0], Namespace: secret.Namespace}
	if err := r.Get(context.TODO(), clmKey, clm); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Cannot found clusterManager")
			// return ctrl.Result{RequeueAfter: ClusterManagerWaitingRequeueAfter}, nil
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
			//create helper for patch

			// if err := r.Status().Update(context.TODO(), clm); err != nil {
			// 	if errors.IsConflict(err) {
			// 		log.Info("Retry... because resource is modified.." + err.Error())
			// 		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
			// 	} else {
			// 		log.Error(err, "Failed to update clustermanager.status.ControlPlaneEndpoint")
			// 		return ctrl.Result{}, err
			// 	}
			// }
		}
	}

	return ctrl.Result{}, nil
}

// delete kfc is necessary?
// kfc := &fedcore.KubeFedCluster{}
// kfcKey := types.NamespacedName{Name: clusterName, Namespace: constant.KubeFedNamespace}

// if err := r.Get(context.TODO(), kfcKey, kfc); err == nil {
// 	r.Delete(context.TODO(), kfc)
// }

// 1. clm request가 mapping되기 이전에... secret의 삭제인 경우
// clm이 없을 수도 있고..
// clm 있어도 (ready가 되기 이전일 수도 있으니까... secret reconcile 수행 안했을 수도 있다.)
// unjoin 필요가 있는지 clm
// 2. clm req가 mapping으로 모든 작업이 완료된 이후에 secret의 삭제인 경우
// clm에 endpoint 넣어주고   (이건 언제 넣어주어도 상관 없으나 clm이 ready인 경우에 join하면서 같이 넣어주면 될 것 같고)
// kubefed join 해주는 과정 (clm이 ready여야하고.. )  --> kubefedcluster 리소스가 생성되고..

func (r *SecretReconciler) reconcileDelete(ctx context.Context, secret *corev1.Secret) (reconcile.Result, error) {
	log := r.Log.WithValues("secret", types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace})
	log.Info("Start to reconcile reconcileDelete... ")
	clusterManagerNamespacedName := secret.GetNamespace() + "-" + strings.Split(secret.Name, constant.KubeconfigPostfix)[0]

	clm := &clusterv1alpha1.ClusterManager{}
	clmKey := types.NamespacedName{Name: strings.Split(secret.Name, constant.KubeconfigPostfix)[0], Namespace: secret.Namespace}
	if err := r.Get(context.TODO(), clmKey, clm); err != nil {
		log.Error(err, "Failed to get ClusterManager")
		return ctrl.Result{}, err
	}
	// delete deployed role/binding
	remoteClientset, err := util.GetRemoteK8sClient(secret)
	if err != nil {
		log.Error(err, "Failed to get remoteK8sClient")
		return ctrl.Result{}, err
	}

	if _, err := remoteClientset.RbacV1().ClusterRoleBindings().Get(context.TODO(), "cluster-owner-crb-"+clm.Annotations["owner"], metav1.GetOptions{}); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Cannot found cluster-admin crb from remote cluster. Cluster-owner-crb clusterrolebinding is already deleted")
		} else {
			log.Error(err, "Failed to get cluster-owner-crb clusterrolebinding from remote cluster")
			return ctrl.Result{}, err
		}
	} else {
		if err := remoteClientset.RbacV1().ClusterRoleBindings().Delete(context.TODO(), "cluster-owner-crb-"+clm.Annotations["owner"], metav1.DeleteOptions{}); err != nil {
			log.Error(err, "Cannnot delete cluster-owner-crb clusterrolebinding")
			return ctrl.Result{}, err
		}
	}

	if _, err := remoteClientset.RbacV1().ClusterRoles().Get(context.TODO(), "developer", metav1.GetOptions{}); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Cannot found developer cr from remote cluster.  Developer clusterrole is already deleted")
		} else {
			log.Error(err, "Failed to get developer clusterrole from remote cluster")
			return ctrl.Result{}, err
		}
	} else {
		if err := remoteClientset.RbacV1().ClusterRoles().Delete(context.TODO(), "developer", metav1.DeleteOptions{}); err != nil {
			log.Error(err, "Cannnot delete developer clusterrole")
			return ctrl.Result{}, err
		}
	}

	if _, err := remoteClientset.RbacV1().ClusterRoles().Get(context.TODO(), "guest", metav1.GetOptions{}); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Cannot found guest cr from remote cluster. Guest clusterrole is already deleted")
		} else {
			log.Error(err, "Failed to get guest clusterrole from remote cluster")
			return ctrl.Result{}, err
		}
	} else {
		if err := remoteClientset.RbacV1().ClusterRoles().Delete(context.TODO(), "guest", metav1.DeleteOptions{}); err != nil {
			log.Error(err, "Cannnot delete guest clusterrole")
			return ctrl.Result{}, err
		}
	}

	kfc := &fedv1b1.KubeFedCluster{}
	kfcKey := types.NamespacedName{Name: clusterManagerNamespacedName, Namespace: constant.KubeFedNamespace}
	if err := r.Get(context.TODO(), kfcKey, kfc); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Cannot found kubefedCluster. Already unjoined")
			// return ctrl.Result{}, nil
		} else {
			log.Error(err, "Failed to get kubefedCluster")
			return ctrl.Result{}, err
		}
	} else {
		if err := r.Delete(context.TODO(), kfc); err != nil {
			log.Error(err, "Failed to delete kubefedCluster")
			return ctrl.Result{}, err
		}
	}
	controllerutil.RemoveFinalizer(secret, constant.SecretFinalizer)

	return ctrl.Result{}, nil
}

func (r *SecretReconciler) requeueSecretForClusterManager(o handler.MapObject) []ctrl.Request {
	clm := o.Object.(*clusterv1alpha1.ClusterManager)
	log := r.Log.WithValues("objectMapper", "ClusterManagersToSecret", "namespace", clm.Namespace, "clustermanager", clm.Name)
	log.Info("Start to requeueSecretForClusterManager mapping")

	// Don't handle deleted machinedeployment
	if !clm.ObjectMeta.DeletionTimestamp.IsZero() {
		log.Info("clustermanager has a deletion timestamp, skipping mapping.")
		return nil
	}

	reconcileRequests := []ctrl.Request{}
	reconcileRequests = append(reconcileRequests, ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: clm.Namespace,
			Name:      clm.Name + constant.KubeconfigPostfix,
		},
	})

	return reconcileRequests
}

func (r *SecretReconciler) SetupWithManager(mgr ctrl.Manager) error {

	controller, err := ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		WithEventFilter(
			predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					return false
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					osecret := e.ObjectOld.(*corev1.Secret).DeepCopy()
					nsecret := e.ObjectNew.(*corev1.Secret).DeepCopy()
					isTarget := strings.Contains(osecret.Name, constant.KubeconfigPostfix)
					isDelete := osecret.DeletionTimestamp.IsZero() && !nsecret.DeletionTimestamp.IsZero()
					isFinalized := !controllerutil.ContainsFinalizer(osecret, util.SecretFinalizer) && controllerutil.ContainsFinalizer(nsecret, util.SecretFinalizer)

					if isTarget || isDelete || isFinalized {
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
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(r.requeueSecretForClusterManager),
		},
		predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldclm := e.ObjectOld.(*clusterv1alpha1.ClusterManager)
				newclm := e.ObjectNew.(*clusterv1alpha1.ClusterManager)
				if !oldclm.Status.Ready && newclm.Status.Ready {
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
