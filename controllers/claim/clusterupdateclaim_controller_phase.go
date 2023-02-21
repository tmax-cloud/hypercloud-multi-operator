package controllers

import (
	"context"

	claimV1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/claim/v1alpha1"
	clusterV1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/cluster/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	// "k8s.io/apimachinery/pkg/api/errors"
)

// cluster update claim의 type에 맞게 clusterManager를 변경한다.
func (r *ClusterUpdateClaimReconciler) UpdateClusterManager(clm *clusterV1alpha1.ClusterManager, cuc *claimV1alpha1.ClusterUpdateClaim) error {

	masterNum := cuc.Spec.UpdatedMasterNum
	workerNum := cuc.Spec.UpdatedWorkerNum
	if err := r.UpdateNodeNum(clm, masterNum, workerNum); err != nil {
		return err
	}
	return nil
}

// 노드를 스케일링할 때 사용하는 메소드
func (r *ClusterUpdateClaimReconciler) UpdateNodeNum(clm *clusterV1alpha1.ClusterManager, masterNum int, workerNum int) error {

	if masterNum != 0 {
		clm.Spec.MasterNum = masterNum
	}

	if workerNum != 0 {
		clm.Spec.WorkerNum = workerNum
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
		if cuc.Status.Phase == claimV1alpha1.ClusterUpdateClaimPhaseApproved ||
			cuc.Status.Phase == claimV1alpha1.ClusterUpdateClaimPhaseRejected {
			continue
		}
		key := types.NamespacedName{Name: cuc.Name, Namespace: cuc.Namespace}
		reqs = append(reqs, ctrl.Request{NamespacedName: key})
	}

	return reqs
}

// clusterupdateclaim 초기 세팅을 하는 메서드
func (r *ClusterUpdateClaimReconciler) SetupClaimStatus(clusterUpdateClaim *claimV1alpha1.ClusterUpdateClaim, clusterManager *clusterV1alpha1.ClusterManager) error {
	if clusterUpdateClaim.Labels == nil {
		clusterUpdateClaim.Labels = map[string]string{}
	}

	if _, ok := clusterUpdateClaim.Labels[LabelKeyClmName]; !ok {
		clusterUpdateClaim.Labels[LabelKeyClmName] = clusterUpdateClaim.Spec.ClusterName
	}

	// phase가 error인 경우, 수정되어 들어올 수 있으므로
	if clusterUpdateClaim.Status.Phase != "Approved" {
		clusterUpdateClaim.Status.SetTypedPhase(claimV1alpha1.ClusterUpdateClaimPhaseAwaiting)
		clusterUpdateClaim.Status.Reason = "Waiting for admin approval"
	}

	return nil
}
