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
	"strings"
	"time"

	"github.com/go-logr/logr"
	clusterv1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/cluster/v1alpha1"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"

	argocdv1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
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
)

// ClusterReconciler reconciles a Memcached object
type SecretReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

const (
	requeueAfter5Sec  = 5 * time.Second
	requeueAfter10min = 600 * time.Second
)

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
	if !secret.ObjectMeta.GetDeletionTimestamp().IsZero() {
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

func (r *SecretReconciler) DeployRolebinding(ctx context.Context, secret *corev1.Secret) (ctrl.Result, error) {
	log := r.Log.WithValues("secret", types.NamespacedName{Name: secret.GetName(), Namespace: secret.GetNamespace()})
	log.Info("Start to reconcile.. Deploy rolebinding to remote")

	clm := &clusterv1alpha1.ClusterManager{}
	clmKey := types.NamespacedName{
		Name:      strings.Split(secret.GetName(), util.KubeconfigSuffix)[0],
		Namespace: secret.GetNamespace(),
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

	clusterAdminCRBName := "cluster-owner-crb-" + clm.Annotations[util.AnnotationKeyOwner]
	clusterAdminCRB := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterAdminCRBName,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     rbacv1.UserKind,
				APIGroup: rbacv1.GroupName,
				Name:     clm.Annotations[util.AnnotationKeyOwner],
			},
		},
	}

	targetGroup := []string{
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
	allRule.APIGroups = append(allRule.APIGroups, targetGroup...)
	allRule.Resources = append(allRule.Resources, rbacv1.ResourceAll)
	allRule.Verbs = append(allRule.Verbs, rbacv1.VerbAll)

	if _, err :=
		remoteClientset.
			RbacV1().
			ClusterRoleBindings().
			Get(
				context.TODO(),
				clusterAdminCRBName,
				metav1.GetOptions{},
			); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Cannot found cluster-admin crb from remote cluster. Start to create cluster-admin clusterrolebinding to remote")
			if _, err :=
				remoteClientset.
					RbacV1().
					ClusterRoleBindings().
					Create(
						context.TODO(),
						clusterAdminCRB,
						metav1.CreateOptions{},
					); err != nil {
				log.Error(err, "Cannot create clusterrolebinding for cluster-admin")
				return ctrl.Result{}, err
			}
		} else {
			log.Error(err, "Failed to get cluster-admin clusterrolebinding from remote cluster")
			return ctrl.Result{}, err
		}
	}

	crList := []*rbacv1.ClusterRole{
		createClusterRole("developer", targetGroup, []string{rbacv1.VerbAll}),
		createClusterRole("guest", targetGroup, []string{"get", "list", "watch"}),
	}

	for _, targetCr := range crList {
		if _, err :=
			remoteClientset.
				RbacV1().
				ClusterRoles().
				Get(
					context.TODO(),
					targetCr.GetName(),
					metav1.GetOptions{},
				); err != nil {
			if errors.IsNotFound(err) {
				log.Info("Cannot found cr [" + targetCr.GetName() + "] from remote cluster. Start to create [" + targetCr.GetName() + "] clusterrole to remote")
				if _, err :=
					remoteClientset.
						RbacV1().
						ClusterRoles().
						Create(
							context.TODO(),
							targetCr,
							metav1.CreateOptions{},
						); err != nil {
					log.Error(err, "Cannot create clusterrole ["+targetCr.GetName()+"]")
					return ctrl.Result{}, err
				}
			} else {
				log.Error(err, "Failed to get clusterrole ["+targetCr.GetName()+"] from remote cluster")
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *SecretReconciler) DeployArgocdResources(ctx context.Context, secret *corev1.Secret) (ctrl.Result, error) {
	log := r.Log.WithValues("secret", types.NamespacedName{Name: secret.GetName(), Namespace: secret.GetNamespace()})
	log.Info("Start to reconcile.. Deploy argocd resources to remote")

	remoteClientset, err := util.GetRemoteK8sClient(secret)
	if err != nil {
		log.Error(err, "Failed to get remoteK8sClient")
		return ctrl.Result{}, err
	}

	argocdManager := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: util.ArgoServiceAccount,
		},
	}
	if _, err :=
		remoteClientset.
			CoreV1().
			ServiceAccounts(util.KubeNamespace).
			Get(
				context.TODO(),
				util.ArgoServiceAccount,
				metav1.GetOptions{},
			); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Cannot found ServiceAccount for argocd from remote cluster. Start to create.")
			_, err :=
				remoteClientset.
					CoreV1().
					ServiceAccounts(util.KubeNamespace).
					Create(
						context.TODO(),
						argocdManager,
						metav1.CreateOptions{},
					)
			if err != nil {
				log.Error(err, "Cannot create ServiceAccount for ["+util.ArgoServiceAccount+"]")
				return ctrl.Result{}, err
			}
		} else {
			log.Error(err, "Failed to get ServiceAccount ["+util.ArgoServiceAccount+"] from remote cluster")
			return ctrl.Result{}, err
		}
	}
	log.Info("ServiceAccount for argocd is already crated")

	argocdManagerRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: util.ArgoClusterRole,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{rbacv1.APIGroupAll},
				Resources: []string{rbacv1.ResourceAll},
				Verbs:     []string{rbacv1.VerbAll},
			},
			{
				NonResourceURLs: []string{rbacv1.NonResourceAll},
				Verbs:           []string{rbacv1.VerbAll},
			},
		},
	}
	if _, err :=
		remoteClientset.
			RbacV1().
			ClusterRoles().
			Get(context.TODO(), util.ArgoClusterRole, metav1.GetOptions{}); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Cannot found ClusterRole for argocd from remote cluster. Start to create.")
			if _, err :=
				remoteClientset.
					RbacV1().
					ClusterRoles().
					Create(
						context.TODO(),
						argocdManagerRole,
						metav1.CreateOptions{},
					); err != nil {
				log.Error(err, "Cannot create ClusterRole for ["+util.ArgoClusterRole+"]")
				return ctrl.Result{}, err
			}
		} else {
			log.Error(err, "Failed to get ClusterRole ["+util.ArgoClusterRole+"] from remote cluster")
			return ctrl.Result{}, err
		}
	}
	log.Info("ClusterRole for argocd is already crated")

	argocdManagerRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: util.ArgoClusterRoleBinding,
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			APIGroup: rbacv1.GroupName,
			Name:     util.ArgoClusterRole,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      util.ArgoServiceAccount,
				Namespace: util.ArgoNamespace,
			},
		},
	}
	if _, err :=
		remoteClientset.
			RbacV1().
			ClusterRoleBindings().
			Get(
				context.TODO(),
				util.ArgoClusterRoleBinding,
				metav1.GetOptions{},
			); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Cannot found ClusterRoleBinding for argocd from remote cluster. Start to create.")
			if _, err :=
				remoteClientset.
					RbacV1().
					ClusterRoleBindings().
					Create(
						context.TODO(),
						argocdManagerRoleBinding,
						metav1.CreateOptions{},
					); err != nil {
				log.Error(err, "Cannot create ClusterRoleBinding for ["+util.ArgoClusterRoleBinding+"]")
				return ctrl.Result{}, err
			}
		} else {
			log.Error(err, "Failed to get ClusterRoleBinding ["+util.ArgoClusterRoleBinding+"] from remote cluster")
			return ctrl.Result{}, err
		}
	}
	log.Info("ClusterRoleBinding for argocd is already crated")

	kubeConfig, err := clientcmd.Load(secret.Data["value"])
	if err != nil {
		log.Error(err, "Failed to get kubeconfig data from secret")
		return ctrl.Result{}, err
	}

	configJson, err := json.Marshal(
		&argocdv1alpha1.ClusterConfig{
			TLSClientConfig: argocdv1alpha1.TLSClientConfig{
				Insecure: false,
				CertData: kubeConfig.AuthInfos[string(secret.Data["user"])].ClientCertificateData,
				KeyData:  kubeConfig.AuthInfos[string(secret.Data["user"])].ClientKeyData,
				CAData:   kubeConfig.Clusters[string(secret.Data["cluster"])].CertificateAuthorityData,
			},
		},
	)
	if err != nil {
		log.Error(err, "Failed to marshal cluster authorization parameters")
	}

	clusterName := strings.Split(secret.GetName(), util.KubeconfigSuffix)[0]
	argocdClusterSecret := &corev1.Secret{}
	secretKey := types.NamespacedName{
		Name:      secret.Annotations[util.AnnotationKeyArgoClusterSecret],
		Namespace: util.ArgoNamespace,
	}
	if err := r.Get(context.TODO(), secretKey, argocdClusterSecret); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Cannot found Argocd Secret for remote cluster [" + clusterName + "]. Start to create.")
			argocdClusterSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secret.Annotations[util.AnnotationKeyArgoClusterSecret],
					Namespace: util.ArgoNamespace,
					Annotations: map[string]string{
						util.AnnotationKeyOwner:         secret.Annotations[util.AnnotationKeyOwner],
						util.AnnotationKeyCreator:       secret.Annotations[util.AnnotationKeyCreator],
						util.AnnotationKeyArgoManagedBy: util.ArgoApiGroup,
					},
					Labels: map[string]string{
						util.LabelKeyArgoSecretType: util.ArgoSecretTypeCluster,
					},
				},
				StringData: map[string]string{
					"config": string(configJson),
					"name":   clusterName,
					"server": kubeConfig.Clusters[string(secret.Data["cluster"])].Server,
				},
			}
			if err := r.Create(context.TODO(), argocdClusterSecret); err != nil {
				log.Error(err, "Cannot create Argocd Secret for remote cluster ["+clusterName+"]")
			}
		} else {
			log.Error(err, "Failed to get Argocd Secret for remote cluster ["+clusterName+"]")
			return ctrl.Result{}, err
		}
	}
	log.Info("Argocd Secret for remote cluster [" + clusterName + "] is already crated")

	return ctrl.Result{}, nil
}

func (r *SecretReconciler) UpdateClusterManagerControlPlaneEndpoint(ctx context.Context, secret *corev1.Secret) (ctrl.Result, error) {
	log := r.Log.WithValues("secret", types.NamespacedName{Name: secret.GetName(), Namespace: secret.GetNamespace()})
	log.Info("Start to reconcile UpdateClusterManagerControleplaneEndpoint... ")

	kubeConfig, err := clientcmd.Load(secret.Data["value"])
	if err != nil {
		log.Error(err, "Failed to get kubeconfig data from secret")
		return ctrl.Result{}, err
	}
	clm := &clusterv1alpha1.ClusterManager{}
	clmKey := types.NamespacedName{
		Name:      strings.Split(secret.GetName(), util.KubeconfigSuffix)[0],
		Namespace: secret.GetNamespace(),
	}
	if err := r.Get(context.TODO(), clmKey, clm); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Cannot found clusterManager")
			// return ctrl.Result{RequeueAfter: requeueAfter5Sec}, nil
		} else {
			log.Error(err, "Failed to get clusterManager + ["+clm.GetName()+"]")
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
	log := r.Log.WithValues("secret", types.NamespacedName{Name: secret.GetName(), Namespace: secret.GetNamespace()})
	log.Info("Start to reconcile reconcileDelete... ")

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
			if err :=
				remoteClientset.
					CoreV1().
					ServiceAccounts(util.KubeNamespace).
					Delete(
						context.TODO(),
						targetSa,
						metav1.DeleteOptions{},
					); err != nil {
				log.Error(err, "Cannot delete ServiceAccount ["+targetSa+"]")
				return ctrl.Result{}, err
			}
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
			if err :=
				remoteClientset.
					RbacV1().
					ClusterRoleBindings().
					Delete(
						context.TODO(),
						targetCrb,
						metav1.DeleteOptions{},
					); err != nil {
				log.Error(err, "Cannot delete ClusterRoleBinding ["+targetCrb+"]")
				return ctrl.Result{}, err
			}
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
			if err :=
				remoteClientset.
					RbacV1().
					ClusterRoles().
					Delete(
						context.TODO(),
						targetCr,
						metav1.DeleteOptions{},
					); err != nil {
				log.Error(err, "Cannot delete ClusterRole ["+targetCr+"]")
				return ctrl.Result{}, err
			}
		}
	}

	argoClusterSecret := &corev1.Secret{}
	secretKey := types.NamespacedName{
		Name:      secret.Annotations[util.AnnotationKeyArgoClusterSecret],
		Namespace: util.ArgoNamespace,
	}
	if err := r.Get(context.TODO(), secretKey, argoClusterSecret); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Cannot found Secret [" + argoClusterSecret.GetName() + "] from remote cluster. Maybe already deleted")
		} else {
			log.Error(err, "Failed to get Secret ["+argoClusterSecret.GetName()+"] from remote cluster")
			return ctrl.Result{}, err
		}
	} else {
		if err := r.Delete(context.TODO(), argoClusterSecret); err != nil {
			log.Error(err, "Cannot delete Secret ["+argoClusterSecret.GetName()+"]")
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
// 	if !clm.ObjectMeta.GetDeletionTimestamp().IsZero() {
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
					isTarget := strings.Contains(oldSecret.GetName(), util.KubeconfigSuffix)
					isDelete := oldSecret.GetDeletionTimestamp().IsZero() && !newSecret.GetDeletionTimestamp().IsZero()
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

func createClusterRole(name string, targetGroup []string, verbList []string) *rbacv1.ClusterRole {
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: targetGroup,
				Resources: []string{rbacv1.ResourceAll},
				Verbs:     verbList,
			},
			{
				APIGroups: []string{"apiregistration.k8s.io"},
				Resources: []string{rbacv1.ResourceAll},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}

	return clusterRole
}
