package controllers

import (
	"context"

	claimV1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/claim/v1alpha1"
	clusterV1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/cluster/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// cluster update claim의 type에 맞게 clusterManager를 변경한다.
func (r *ClusterUpdateClaimReconciler) UpdateClusterManager(cuc *claimV1alpha1.ClusterUpdateClaim, clm *clusterV1alpha1.ClusterManager) error {

	if cuc.Spec.UpdateType == claimV1alpha1.ClusterUpdateTypeNodeScale {
		if err := r.UpdateNodeNum(clm, cuc); err != nil {
			return err
		}
		return nil
	} else {
		// 추가될 update type
	}

	return nil
}

// 노드를 스케일링할 때 사용하는 메소드
func (r *ClusterUpdateClaimReconciler) UpdateNodeNum(clm *clusterV1alpha1.ClusterManager, cuc *claimV1alpha1.ClusterUpdateClaim) error {

	if cuc.Spec.ExpectedMasterNum != 0 {
		clm.Spec.MasterNum = cuc.Spec.ExpectedMasterNum
	}

	if cuc.Spec.ExpectedWorkerNum != 0 {
		clm.Spec.WorkerNum = cuc.Spec.ExpectedWorkerNum
	}

	if err := r.Update(context.TODO(), clm); err != nil {
		return err
	}
	return nil
}

// cluster manager 삭제시 cluster manager와 관련된 모든 cluster update claim을 reconcile loop로 보낸다.
func (r *ClusterUpdateClaimReconciler) RequeueClusterUpdateClaimsForClusterManager(o client.Object) []ctrl.Request {
	clm := o.DeepCopyObject().(*clusterV1alpha1.ClusterManager)
	log := r.Log.WithValues("objectMapper", "clusterManagerToClusterUpdateClaim", "clusterManager", clm.Name)
	log.Info("Start to clusterManagerToClusterUpdateClaim mapping...")

	cucs := &claimV1alpha1.ClusterUpdateClaimList{}

	opts := []client.ListOption{client.InNamespace(clm.Namespace),
		client.MatchingLabels{LabelKeyClmName: clm.Name},
	}

	if err := r.List(context.TODO(), cucs, opts...); err != nil {
		log.Error(err, "Failed to list clusterupdateclaims")
		return nil
	}

	reqs := []ctrl.Request{}
	for _, cuc := range cucs.Items {
		key := types.NamespacedName{Name: cuc.Name, Namespace: cuc.Namespace}
		reqs = append(reqs, ctrl.Request{NamespacedName: key})
	}

	return reqs
}

// phase가 approve거나 reject일 때 필요한 동작을 수행하는 메서드
func (r *ClusterUpdateClaimReconciler) ProcessPhase(clusterUpdateClaim *claimV1alpha1.ClusterUpdateClaim, clusterManager *clusterV1alpha1.ClusterManager) error {
	if clusterUpdateClaim.Status.Phase == claimV1alpha1.ClusterUpdateClaimPhaseApproved {
		// 승인
		if err := r.UpdateClusterManager(clusterUpdateClaim, clusterManager); err != nil {
			clusterUpdateClaim.Status.SetTypedPhase(claimV1alpha1.ClusterUpdateClaimPhaseError)
			clusterUpdateClaim.Status.Reason = err.Error()
			return nil
		}
		clusterUpdateClaim.Status.Reason = "Admin approved"

		return nil
	} else if clusterUpdateClaim.Status.Phase == claimV1alpha1.ClusterUpdateClaimPhaseRejected {
		// 거절
		clusterUpdateClaim.Status.Reason = "Admin rejected"
		return nil
	}

	return nil
}

// cluster manager가 삭제되었거나 원래부터 없었던 경우에 대한 처리
func (r *ClusterUpdateClaimReconciler) ProcessDeletePhase(clusterUpdateClaim *claimV1alpha1.ClusterUpdateClaim) error {

	if clusterUpdateClaim.Status.Phase == "" {
		clusterUpdateClaim.Status.SetTypedPhase(claimV1alpha1.ClusterUpdateClaimPhaseError)
		clusterUpdateClaim.Status.Reason = "Cluster not found"
	} else if clusterUpdateClaim.Status.Phase == claimV1alpha1.ClusterUpdateClaimPhaseAwaiting {
		// cluster manager 삭제 후, reconcile loop로 들어온 awaiting 상태의 cluster update claim에 대한 cluster deleted 삭제 처리
		clusterUpdateClaim.Status.SetTypedPhase(claimV1alpha1.ClusterUpdateClaimPhaseClusterDeleted)
		clusterUpdateClaim.Status.Reason = "Cluster deleted"
	}

	return nil
}

// clusterupdateclaim 초기 세팅을 하는 메서드
func (r *ClusterUpdateClaimReconciler) ReconcileReady(clusterUpdateClaim *claimV1alpha1.ClusterUpdateClaim, clusterManager *clusterV1alpha1.ClusterManager) error {
	log := r.Log.WithValues("ClusterUpdateClaim", clusterUpdateClaim.GetNamespacedName())
	log.Info("Reconcile ready")

	if clusterUpdateClaim.Labels == nil {
		clusterUpdateClaim.Labels = map[string]string{}
	}
	if _, ok := clusterUpdateClaim.Labels[LabelKeyClmName]; !ok {
		clusterUpdateClaim.Labels[LabelKeyClmName] = clusterUpdateClaim.Spec.ClusterName
	}

	if clusterUpdateClaim.Status.Phase == "" {
		clusterUpdateClaim.Status.SetTypedPhase(claimV1alpha1.ClusterUpdateClaimPhaseAwaiting)
		clusterUpdateClaim.Status.Reason = "Waiting for admin approval"
	}

	return nil
}
