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
	"fmt"

	"github.com/go-logr/logr"
	claimV1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/claim/v1alpha1"
	clusterV1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/cluster/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// ClusterClaimReconciler reconciles a ClusterClaim object
type ClusterUpdateClaimReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

const (
	LabelKeyClmName = "clustermanager.cluster.tmax.io/clm-name"
)

// +kubebuilder:rbac:groups=claim.tmax.io,resources=clusterupdateclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=claim.tmax.io,resources=clusterupdateclaims/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.tmax.io,resources=clustermanagers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.tmax.io,resources=clustermanagers/status,verbs=get;update;patch

// cluster update claim reconcile loop
func (r *ClusterUpdateClaimReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	_ = context.Background()
	log := r.Log.WithValues("ClusterUpdateClaim", req.NamespacedName)

	cuc := &claimV1alpha1.ClusterUpdateClaim{}
	if err := r.Client.Get(context.TODO(), req.NamespacedName, cuc); errors.IsNotFound(err) {
		log.Info("ClusterUpdateClaim resource not found. Ignoring since object must be deleted")
		return ctrl.Result{}, nil
	} else if err != nil {
		log.Error(err, "Failed to get ClusterUpdateClaim")
		return ctrl.Result{}, err
	}

	//set patch helper
	patchHelper, err := patch.NewHelper(cuc, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	defer func() {
		if err := patchHelper.Patch(context.TODO(), cuc); err != nil {
			log.Error(err, "Failed to patch clusterupdateclaim")
			reterr = err
		}
	}()

	return r.reconcile(ctx, cuc)
}

// reconcile handles clusterupdateclaim reconciliation.
func (r *ClusterUpdateClaimReconciler) reconcile(ctx context.Context, cuc *claimV1alpha1.ClusterUpdateClaim) (ctrl.Result, error) {
	log := r.Log.WithValues("ClusterUpdateClaim", cuc.GetNamespacedName())
	clmKey := cuc.GetClusterNamespacedName()
	clm := &clusterV1alpha1.ClusterManager{}

	if err := r.Client.Get(context.TODO(), clmKey, clm); errors.IsNotFound(err) {
		log.Info(fmt.Sprintf("Not found clustermanager [%s]. cannot use cluster update claim.", cuc.Spec.ClusterName))
		// cluster가 없는 경우
		cuc.Status.SetTypedPhase(claimV1alpha1.ClusterUpdateClaimPhaseError)
		cuc.Status.SetTypedReason(claimV1alpha1.ClusterUpdateClaimReasonClusterNotFound)
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, err
	}

	if !clm.GetDeletionTimestamp().IsZero() {
		log.Info(fmt.Sprintf("Deleting clustermanager [%s]. cannot use cluster update claim.", cuc.Spec.ClusterName))
		cuc.Status.SetTypedPhase(claimV1alpha1.ClusterUpdateClaimPhaseError)
		cuc.Status.SetTypedReason(claimV1alpha1.ClusterUpdateClaimReasonClusterIsDeleting)
		return ctrl.Result{RequeueAfter: requeueAfter10Second}, nil
	}

	log.Info(fmt.Sprintf("Found clustermanager [%s]. Start clusterupdateclaim reconcile phase", cuc.Spec.ClusterName))

	r.SetupClaim(cuc, clm)

	if cuc.IsPhaseError() || cuc.IsPhaseAwaiting() {
		return ctrl.Result{}, nil
	}

	if cuc.IsPhaseApproved() {
		log.Info("Approved clusterupdateclaim")

		if err := r.CheckValidClaim(clm, cuc); err != nil {
			log.Error(err, "Failed to approve")
			cuc.Status.SetTypedPhase(claimV1alpha1.ClusterUpdateClaimPhaseError)
			cuc.Status.SetTypedReason(claimV1alpha1.ClusterUpdateClaimReasonConcurruencyError)
			return ctrl.Result{}, nil
		}

		if err := r.UpdateNodeNum(clm, cuc); err != nil {
			log.Error(err, "Failed to approve")
			cuc.Status.SetTypedPhase(claimV1alpha1.ClusterUpdateClaimPhaseError)
			cuc.Status.SetTypedReason(claimV1alpha1.ClusterUpdateClaimReason(err.Error()))
			return ctrl.Result{}, err
		}

		cuc.Status.SetTypedPhase(claimV1alpha1.ClusterUpdateClaimPhaseApproved)
		cuc.Status.SetTypedReason(claimV1alpha1.ClusterUpdateClaimReasonAdminApproved)
		return ctrl.Result{}, nil
	} else if cuc.IsPhaseRejected() {
		log.Info("Rejected clusterupdateclaim")
		// 거절된 이유는 API 서버에서 입력
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, nil
}

func (r *ClusterUpdateClaimReconciler) SetupWithManager(mgr ctrl.Manager) error {
	controller, err := ctrl.NewControllerManagedBy(mgr).
		For(&claimV1alpha1.ClusterUpdateClaim{}).
		WithEventFilter(
			predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					c := e.Object.(*claimV1alpha1.ClusterUpdateClaim)
					if _, ok := c.Labels[LabelKeyClmName]; ok {
						return false
					}
					return true
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					oc := e.ObjectOld.(*claimV1alpha1.ClusterUpdateClaim)
					nc := e.ObjectNew.(*claimV1alpha1.ClusterUpdateClaim)

					if oc.IsPhaseApproved() || (!oc.IsPhaseRejected() && nc.IsPhaseRejected()) {
						return false
					}

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

	return controller.Watch(
		&source.Kind{Type: &clusterV1alpha1.ClusterManager{}},
		handler.EnqueueRequestsFromMapFunc(r.RequeueClusterUpdateClaimsForClusterManager),
		predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				return false
			},
			CreateFunc: func(e event.CreateEvent) bool {
				return false
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				clm := e.Object.(*clusterV1alpha1.ClusterManager)
				if clm.GetClusterType() == clusterV1alpha1.ClusterTypeCreated {
					return true
				}
				return false
				// return true
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return false
			},
		},
	)
}
