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

	clusterV1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/cluster/v1alpha1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	capiV1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *ClusterManagerReconciler) requeueClusterManagersForCluster(o client.Object) []ctrl.Request {
	c := o.DeepCopyObject().(*capiV1alpha3.Cluster)
	log := r.Log.WithValues("objectMapper", "clusterToClusterManager", "namespace", c.Namespace, c.Kind, c.Name)
	log.Info("Start to requeueClusterManagersForCluster mapping...")

	//get ClusterManager
	key := types.NamespacedName{
		Name:      c.Name,
		Namespace: c.Namespace,
	}
	clm := &clusterV1alpha1.ClusterManager{}
	if err := r.Client.Get(context.TODO(), key, clm); errors.IsNotFound(err) {
		log.Info("ClusterManager resource not found. Ignoring since object must be deleted")
		return nil
	} else if err != nil {
		log.Error(err, "Failed to get ClusterManager")
		return nil
	}

	//create helper for patch
	helper, _ := patch.NewHelper(clm, r.Client)
	defer func() {
		if err := helper.Patch(context.TODO(), clm); err != nil {
			log.Error(err, "ClusterManager patch error")
		}
	}()
	// clm.Status.SetTypedPhase(clusterV1alpha1.ClusterManagerPhaseProvisioned)
	clm.Status.ControlPlaneReady = c.Status.ControlPlaneInitialized

	return nil
}

func (r *ClusterManagerReconciler) requeueClusterManagersForKubeadmControlPlane(o client.Object) []ctrl.Request {
	cp := o.DeepCopyObject().(*controlplanev1.KubeadmControlPlane)
	log := r.Log.WithValues("objectMapper", "kubeadmControlPlaneToClusterManagers", "namespace", cp.Namespace, cp.Kind, cp.Name)

	// Don't handle deleted kubeadmcontrolplane
	if !cp.ObjectMeta.DeletionTimestamp.IsZero() {
		log.V(4).Info("kubeadmcontrolplane has a deletion timestamp, skipping mapping.")
		return nil
	}

	clusterName, ok := cp.Labels[LabelKeyCAPIClusterName]
	if !ok {
		log.Info("clusterName is not exist")
		return nil
	}

	key := types.NamespacedName{
		Name:      clusterName,
		Namespace: cp.Namespace,
	}

	clm := &clusterV1alpha1.ClusterManager{}
	if err := r.Client.Get(context.TODO(), key, clm); errors.IsNotFound(err) {
		log.Info("ClusterManager resource not found. Ignoring since object must be deleted")
		return nil
	} else if err != nil {
		log.Error(err, "Failed to get clusterManager")
		return nil
	}

	// cluster manager status masterRun update
	if clm.Status.MasterRun != int(cp.Status.ReadyReplicas) {
		clm.Status.MasterRun = int(cp.Status.ReadyReplicas)
		err := r.Client.Status().Update(context.Background(), clm)
		if err != nil {
			log.Error(err, "Failed to update clusterManager")
			return nil
		}
		log.Info("Update clusterManager status", "masterRun", clm.Status.MasterRun)
	}

	// kubeadmcontrolplane spec replicas update
	if cp.Spec.Replicas != nil && *cp.Spec.Replicas != int32(clm.Spec.MasterNum) {
		masterNum := int32(clm.Spec.MasterNum)
		cp.Spec.Replicas = &masterNum
		err := r.Client.Update(context.Background(), cp)
		// TODO : conflict error 처리
		if errors.IsConflict(err) {
			// 이미 업데이트 되었을 경우
			log.Info("Already updated kubeadmcontrolplane replicas")
			return nil
		} else if err != nil {
			log.Error(err, "Failed to update kubeadmcontrolplane")
			return nil
		}
		log.Info("Resource changes are detected and changed to existing values. Update kubeadmcontrolplane", "replicas", cp.Spec.Replicas)
		return nil
	}

	return nil
}

func (r *ClusterManagerReconciler) requeueClusterManagersForMachineDeployment(o client.Object) []ctrl.Request {
	md := o.DeepCopyObject().(*capiV1alpha3.MachineDeployment)
	log := r.Log.WithValues("objectMapper", "MachineDeploymentToClusterManagers", "namespace", md.Namespace, md.Kind, md.Name)

	// Don't handle deleted machinedeployment
	if !md.ObjectMeta.DeletionTimestamp.IsZero() {
		log.V(4).Info("machinedeployment has a deletion timestamp, skipping mapping.")
		return nil
	}

	//get ClusterManager
	clusterName, ok := md.Labels[LabelKeyCAPIClusterName]
	if !ok {
		log.Info("clusterName is not exist")
		return nil
	}

	key := types.NamespacedName{
		Name:      clusterName,
		Namespace: md.Namespace,
	}

	clm := &clusterV1alpha1.ClusterManager{}
	if err := r.Client.Get(context.TODO(), key, clm); errors.IsNotFound(err) {
		log.Info("ClusterManager is deleted")
		return nil
	} else if err != nil {
		log.Error(err, "Failed to get ClusterManager")
		return nil
	}

	// cluster manager status workerRun update
	if clm.Status.WorkerRun != int(md.Status.ReadyReplicas) {
		clm.Status.WorkerRun = int(md.Status.ReadyReplicas)
		err := r.Client.Status().Update(context.Background(), clm)
		if err != nil {
			log.Error(err, "Failed to update clusterManager")
			return nil
		}
		log.Info("Update clusterManager status", "workerRun", clm.Status.WorkerRun)
		return nil
	}

	// machine deployment spec replicas update
	if md.Spec.Replicas != nil && *md.Spec.Replicas != int32(clm.Spec.WorkerNum) {
		workerNum := int32(clm.Spec.WorkerNum)
		md.Spec.Replicas = &workerNum
		err := r.Client.Update(context.Background(), md)
		// TODO : conflict error 처리
		if errors.IsConflict(err) {
			// 이미 존재하는 경우
			log.Info("Already updated machinedeployment replicas")
			return nil
		} else if err != nil {
			log.Error(err, "Failed to update MachineDeployment")
			return nil
		}
		log.Info("Resource changes are detected and changed to existing values. Update machinedeployment", "replicas", md.Spec.Replicas)
		return nil
	}

	return nil
}

func (r *ClusterManagerReconciler) requeueClusterManagersForSubresources(o client.Object) []ctrl.Request {
	log := r.Log.WithValues("objectMapper", "SubresourcesToClusterManagers", "namespace", o.GetNamespace(), "name", o.GetName())

	//get ClusterManager
	key := types.NamespacedName{
		Name:      o.GetLabels()[clusterV1alpha1.LabelKeyClmName],
		Namespace: o.GetNamespace(),
	}
	clm := &clusterV1alpha1.ClusterManager{}
	if err := r.Client.Get(context.TODO(), key, clm); errors.IsNotFound(err) {
		log.Info("ClusterManager is deleted")
		return nil
	} else if err != nil {
		log.Error(err, "Failed to get ClusterManager")
		return nil
	}

	if !clm.GetDeletionTimestamp().IsZero() {
		return nil
	}

	isGateway := strings.Contains(o.GetName(), "gateway")
	if isGateway {
		clm.Status.GatewayReady = false
	} else {
		clm.Status.TraefikReady = false
	}

	err := r.Status().Update(context.TODO(), clm)
	if err != nil {
		log.Error(err, "Failed to update ClusterManager status")
		return nil //??
	}

	return nil
}
