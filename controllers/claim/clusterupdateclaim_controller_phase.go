package controllers

import (
	"context"
	"fmt"

	claimV1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/claim/v1alpha1"
	clusterV1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/cluster/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	// "k8s.io/apimachinery/pkg/api/errors"
)

// updateclaim에 대한 낙관적 동시성 처리
func (r *ClusterUpdateClaimReconciler) CheckValidClaim(clm *clusterV1alpha1.ClusterManager, cuc *claimV1alpha1.ClusterUpdateClaim) error {
	// awaiting 상태가 되었을 시점의 master, worker node 수
	statusMasterNum := cuc.Status.CurrentMasterNum
	statusWorkerNum := cuc.Status.CurrentWorkerNum

	// 실제 master, worker 노드 수
	realMasterNum := clm.Spec.MasterNum
	realWorkerNum := clm.Spec.WorkerNum

	// 두 노드 수가 다르면 Error
	if statusMasterNum != realMasterNum || statusWorkerNum != realWorkerNum {
		return fmt.Errorf(string(claimV1alpha1.ClusterUpdateClaimReasonConcurruencyError))
	}
	return nil
}

// 노드를 스케일링할 때 사용하는 메소드
func (r *ClusterUpdateClaimReconciler) UpdateNodeNum(clm *clusterV1alpha1.ClusterManager, cuc *claimV1alpha1.ClusterUpdateClaim) error {

	realMasterNum := clm.Spec.MasterNum
	realWorkerNum := clm.Spec.WorkerNum

	if realMasterNum != cuc.Spec.UpdatedMasterNum {
		clm.Spec.MasterNum = cuc.Spec.UpdatedMasterNum
	}

	if realWorkerNum != cuc.Spec.UpdatedWorkerNum {
		clm.Spec.WorkerNum = cuc.Spec.UpdatedWorkerNum
	}

	if err := r.Update(context.TODO(), clm); err != nil {
		return err
	}
	return nil
}

// cluster manager 삭제시 cluster manager와 관련된 모든 cluster update claim을 reconcile loop로 보낸다.
func (r *ClusterUpdateClaimReconciler) RequeueClusterUpdateClaimsForClusterManager(o client.Object) []ctrl.Request {
	clm := o.DeepCopyObject().(*clusterV1alpha1.ClusterManager)
	cucs := &claimV1alpha1.ClusterUpdateClaimList{}
	opts := []client.ListOption{client.InNamespace(clm.Namespace),
		client.MatchingLabels{LabelKeyClmName: clm.Name},
	}
	reqs := []ctrl.Request{}

	log := r.Log.WithValues("objectMapper", "clusterManagerToClusterUpdateClaim", "clusterManager", clm.Name)
	log.Info("Start to clusterManagerToClusterUpdateClaim mapping...")

	if err := r.List(context.TODO(), cucs, opts...); err != nil {
		log.Error(err, "Failed to list clusterupdateclaims")
		return nil
	}

	for _, cuc := range cucs.Items {
		if cuc.IsPhaseApproved() || cuc.IsPhaseRejected() {
			continue
		}
		reqs = append(reqs, ctrl.Request{NamespacedName: cuc.GetNamespacedName()})
	}

	return reqs
}

// clusterupdateclaim 초기 세팅을 하는 메서드
func (r *ClusterUpdateClaimReconciler) SetupClaim(clusterUpdateClaim *claimV1alpha1.ClusterUpdateClaim, clusterManager *clusterV1alpha1.ClusterManager) {
	if clusterUpdateClaim.Labels == nil {
		clusterUpdateClaim.Labels = map[string]string{}
	}

	if _, ok := clusterUpdateClaim.Labels[LabelKeyClmName]; !ok {
		clusterUpdateClaim.Labels[LabelKeyClmName] = clusterUpdateClaim.Spec.ClusterName
	}

	// phase in (공백, error, rejected) 인 경우, awaiting으로 수정
	if clusterUpdateClaim.IsPhaseEmpty() || clusterUpdateClaim.IsPhaseError() || clusterUpdateClaim.IsPhaseRejected() {
		clusterUpdateClaim.Status.SetTypedPhase(claimV1alpha1.ClusterUpdateClaimPhaseAwaiting)
		clusterUpdateClaim.Status.SetTypedReason(claimV1alpha1.ClusterUpdateClaimReasonAdminAwaiting)

		clusterUpdateClaim.Status.CurrentMasterNum = clusterManager.Spec.MasterNum
		clusterUpdateClaim.Status.CurrentWorkerNum = clusterManager.Spec.WorkerNum

		if clusterUpdateClaim.Spec.UpdatedMasterNum == 0 {
			clusterUpdateClaim.Spec.UpdatedMasterNum = clusterManager.Spec.MasterNum
		}
		if clusterUpdateClaim.Spec.UpdatedWorkerNum == 0 {
			clusterUpdateClaim.Spec.UpdatedWorkerNum = clusterManager.Spec.WorkerNum
		}
	}
}
