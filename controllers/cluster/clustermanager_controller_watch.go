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

	clusterv1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/cluster/v1alpha1"
	"github.com/tmax-cloud/hypercloud-multi-operator/controllers/util"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	capiv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *ClusterManagerReconciler) requeueClusterManagersForCluster(o client.Object) []ctrl.Request {
	c := o.DeepCopyObject().(*capiv1.Cluster)
	log := r.Log.WithValues("objectMapper", "clusterToClusterManager", "namespace", c.Namespace, c.Kind, c.Name)
	log.Info("Start to requeueClusterManagersForCluster mapping...")

	//get ClusterManager
	key := types.NamespacedName{
		Name:      c.Name,
		Namespace: c.Namespace,
	}
	clm := &clusterv1alpha1.ClusterManager{}
	if err := r.Get(context.TODO(), key, clm); err != nil {
		if errors.IsNotFound(err) {
			log.Info("ClusterManager resource not found. Ignoring since object must be deleted")
			return nil
		}

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
	// clm.Status.SetTypedPhase(clusterv1alpha1.ClusterManagerPhaseProvisioned)
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

	//get ClusterManager
	// CpName := cp.Name
	// pivot := strings.Index(cp.Name, "-control-plane")
	// if pivot != -1 {
	// 	CpName = cp.Name[0:pivot]
	// }
	key := types.NamespacedName{
		//Name:      CpName,
		Name:      strings.Split(cp.Name, "-control-plane")[0],
		Namespace: cp.Namespace,
	}
	clm := &clusterv1alpha1.ClusterManager{}
	if err := r.Get(context.TODO(), key, clm); err != nil {
		if errors.IsNotFound(err) {
			log.Info("ClusterManager resource not found. Ignoring since object must be deleted")
			return nil
		}

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

	clm.Status.MasterRun = int(cp.Status.Replicas)

	return nil
}

func (r *ClusterManagerReconciler) requeueClusterManagersForMachineDeployment(o client.Object) []ctrl.Request {
	md := o.DeepCopyObject().(*capiv1.MachineDeployment)
	log := r.Log.WithValues("objectMapper", "MachineDeploymentToClusterManagers", "namespace", md.Namespace, md.Kind, md.Name)

	// Don't handle deleted machinedeployment
	if !md.ObjectMeta.DeletionTimestamp.IsZero() {
		log.V(4).Info("machinedeployment has a deletion timestamp, skipping mapping.")
		return nil
	}

	//get ClusterManager
	key := types.NamespacedName{
		//Name:      md.Name[0 : len(md.Name)-len("-md-0")],
		Name:      strings.Split(md.Name, "-md-0")[0],
		Namespace: md.Namespace,
	}
	clm := &clusterv1alpha1.ClusterManager{}
	if err := r.Get(context.TODO(), key, clm); err != nil {
		if errors.IsNotFound(err) {
			log.Info("ClusterManager is deleted deleted")
			// return nil
		}

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

	clm.Status.WorkerRun = int(md.Status.Replicas)

	return nil
}

// func (r *ClusterManagerReconciler) requeueClusterManagersForCertificate(o client.Object) []ctrl.Request {
// 	certificate := o.DeepCopyObject().(*certmanagerv1.Certificate)
// 	log := r.Log.WithValues("objectMapper", "SecretToClusterManagers", "namespace", certificate.Namespace, certificate.Kind, certificate.Name)

// 	// Don't handle deleted certificate
// 	if !certificate.ObjectMeta.DeletionTimestamp.IsZero() {
// 		log.V(4).Info("machinedeployment has a deletion timestamp, skipping mapping.")
// 		return nil
// 	}

// 	//get ClusterManager
// 	key := types.NamespacedName{
// 		Name:      certificate.Labels[clusterv1alpha1.LabelKeyClmName],
// 		Namespace: certificate.Namespace,
// 	}
// 	clm := &clusterv1alpha1.ClusterManager{}
// 	if err := r.Get(context.TODO(), key, clm); err != nil {
// 		if errors.IsNotFound(err) {
// 			log.Info("ClusterManager is deleted deleted")
// 			// return nil
// 		}

// 		log.Error(err, "Failed to get ClusterManager")
// 		return nil
// 	}

// 	clm.Status.TraefikReady = false
// 	err := r.Status().Update(context.TODO(), clm)
// 	if err != nil {
// 		log.Error(err, "Failed to update ClusterManager status")
// 		return nil //??
// 	}

// 	return nil
// }

// func (r *ClusterManagerReconciler) requeueClusterManagersForIngress(o client.Object) []ctrl.Request {
// 	ingress := o.DeepCopyObject().(*networkingv1.Ingress)
// 	log := r.Log.WithValues("objectMapper", "IngressToClusterManagers", "namespace", ingress.Namespace, ingress.Kind, ingress.Name)

// 	log.Info(o.DeepCopyObject().GetObjectKind().GroupVersionKind().Kind)
// 	// Don't handle deleted certificate
// 	if !ingress.ObjectMeta.DeletionTimestamp.IsZero() {
// 		log.V(4).Info("machinedeployment has a deletion timestamp, skipping mapping.")
// 		return nil
// 	}

// 	//get ClusterManager
// 	key := types.NamespacedName{
// 		Name:      ingress.Labels[clusterv1alpha1.LabelKeyClmName],
// 		Namespace: ingress.Namespace,
// 	}
// 	clm := &clusterv1alpha1.ClusterManager{}
// 	if err := r.Get(context.TODO(), key, clm); err != nil {
// 		if errors.IsNotFound(err) {
// 			log.Info("ClusterManager is deleted deleted")
// 			// return nil
// 		}

// 		log.Error(err, "Failed to get ClusterManager")
// 		return nil
// 	}

// 	clm.Status.TraefikReady = false
// 	err := r.Status().Update(context.TODO(), clm)
// 	if err != nil {
// 		log.Error(err, "Failed to update ClusterManager status")
// 		return nil //??
// 	}

// 	return nil
// }

func (r *ClusterManagerReconciler) requeueClusterManagersForSubresources(o client.Object) []ctrl.Request {
	log := r.Log.WithValues("objectMapper", "SecretToClusterManagers", "namespace", o.GetNamespace(), "name", o.GetName())

	//get ClusterManager
	key := types.NamespacedName{
		Name:      o.GetLabels()[clusterv1alpha1.LabelKeyClmName],
		Namespace: o.GetNamespace(),
	}
	clm := &clusterv1alpha1.ClusterManager{}
	if err := r.Get(context.TODO(), key, clm); err != nil {
		if errors.IsNotFound(err) {
			log.Info("ClusterManager is deleted deleted")
			// return nil
		}

		log.Error(err, "Failed to get ClusterManager")
		return nil
	}

	isPrometheus := strings.Contains(o.GetName(), "prometheus")
	_, isArgo := o.GetLabels()[util.LabelKeyArgoSecretType]
	if isPrometheus {
		clm.Status.PrometheusReady = false
	} else if isArgo {
		clm.Status.ArgoReady = false
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

// func (r *ClusterManagerReconciler) requeueClusterManagersForSecret(o client.Object) []ctrl.Request {
// 	secret := o.DeepCopyObject().(*corev1.Secret)
// 	log := r.Log.WithValues("objectMapper", "SecretToClusterManagers", "namespace", secret.Namespace, secret.Kind, secret.Name)

// 	// Don't handle deleted secret
// 	if !secret.ObjectMeta.DeletionTimestamp.IsZero() {
// 		log.V(4).Info("machinedeployment has a deletion timestamp, skipping mapping.")
// 		return nil
// 	}

// 	//get ClusterManager
// 	key := types.NamespacedName{
// 		Name:      secret.Labels[clusterv1alpha1.LabelKeyClmName],
// 		Namespace: secret.Labels[clusterv1alpha1.LabelKeyClmNamespace],
// 	}
// 	clm := &clusterv1alpha1.ClusterManager{}
// 	if err := r.Get(context.TODO(), key, clm); err != nil {
// 		if errors.IsNotFound(err) {
// 			log.Info("ClusterManager is deleted deleted")
// 			// return nil
// 		}

// 		log.Error(err, "Failed to get ClusterManager")
// 		return nil
// 	}

// 	clm.Status.ArgoReady = false
// 	err := r.Status().Update(context.TODO(), clm)
// 	if err != nil {
// 		log.Error(err, "Failed to update ClusterManager status")
// 		return nil //??
// 	}

// 	return nil
// }
