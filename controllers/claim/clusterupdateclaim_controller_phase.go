package controllers

import (
	"context"
	"fmt"

	claimV1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/claim/v1alpha1"
	clusterV1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/cluster/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// cluster update claim의 type에 맞게 clusterManager를 변경한다.
func (r *ClusterUpdateClaimReconciler) UpdateClusterManager(ctx context.Context, cuc *claimV1alpha1.ClusterUpdateClaim) error {

	key := types.NamespacedName{
		Name:      cuc.Spec.ClusterName,
		Namespace: cuc.Namespace,
	}

	// clustermanager를 가져올 수 있는지 체크
	clm := &clusterV1alpha1.ClusterManager{}

	if err := r.Get(context.TODO(), key, clm); errors.IsNotFound(err) {
		return fmt.Errorf("Cannot find cluster:[%s]", cuc.Spec.ClusterName)
	} else if err != nil {
		return err
	}

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

// cluster manager 삭제시 awaiting으로 남아 있는 cluster update claim은 모두 cluster deleted로 표기된다.
func (r *ClusterUpdateClaimReconciler) RequeueClusterUpdateClaimsForClusterManager(o client.Object) []ctrl.Request {
	clm := o.DeepCopyObject().(*clusterV1alpha1.ClusterManager)
	log := r.Log.WithValues("objectMapper", "clusterManagerToClusterUpdateClaim", "clusterManager", clm.Name)
	log.Info("Start to clusterManagerToClusterUpdateClaim mapping...")

	cucs := &claimV1alpha1.ClusterUpdateClaimList{}

	opts := []client.ListOption{client.InNamespace(clm.Namespace),
		client.MatchingLabels{LabelKeyClmName: clm.Name},
	}

	if err := r.List(context.TODO(), cucs, opts...); err != nil {
		return nil
	}

	c := &claimV1alpha1.ClusterUpdateClaim{}

	for _, cuc := range cucs.Items {
		if cuc.Status.Phase == claimV1alpha1.ClusterUpdateClaimPhaseAwaiting {
			key := types.NamespacedName{Namespace: cuc.Namespace, Name: cuc.Name}
			if err := r.Get(context.TODO(), key, c); errors.IsNotFound(err) {
				log.Error(err, "Cannot find clusterupdateclaim", key)
				return nil
			} else if err != nil {
				log.Error(err, "Failed to get clusterupdateclaim", key)
				return nil
			}

			c.Status.SetTypedPhase(claimV1alpha1.ClusterUpdateClaimPhaseClusterDeleted)
			c.Status.Reason = "cluster is deleted"
			if err := r.Update(context.TODO(), c); err != nil {
				return nil
			}
			log.Info("Updated clusterupdatecalim successfully", key)
		}
	}
	return nil
}
