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
	b64 "encoding/base64"
	"fmt"
	"os"
	"regexp"

	clusterV1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/cluster/v1alpha1"
	util "github.com/tmax-cloud/hypercloud-multi-operator/controllers/util"

	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"

	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *ClusterRegistrationReconciler) CheckValidation(ctx context.Context, ClusterRegistration *clusterV1alpha1.ClusterRegistration) (ctrl.Result, error) {
	if ClusterRegistration.Status.Phase != "" {
		return ctrl.Result{}, nil
	}
	log := r.Log.WithValues("ClusterRegistration", ClusterRegistration.GetNamespacedName())
	log.Info("Start to reconcile phase for CheckValidation")

	ClusterRegistration.Status.ClusterValidated = false
	// decode base64 encoded kubeconfig file
	encodedKubeConfig, err := b64.StdEncoding.DecodeString(ClusterRegistration.Spec.KubeConfig)
	if err != nil {
		log.Error(err, "Failed to decode ClusterRegistration.Spec.KubeConfig, maybe wrong kubeconfig file")
		ClusterRegistration.Status.SetTypedPhase(clusterV1alpha1.ClusterRegistrationPhaseError)
		ClusterRegistration.Status.SetTypedReason(clusterV1alpha1.ClusterRegistrationReasonInvalidKubeconfig)
		return ctrl.Result{}, nil
	}

	// validate remote cluster
	remoteClientset, err := util.GetRemoteK8sClientByKubeConfig(encodedKubeConfig)
	if err != nil {
		log.Error(err, "Failed to get client for remote cluster")
		ClusterRegistration.Status.SetTypedPhase(clusterV1alpha1.ClusterRegistrationPhaseError)
		ClusterRegistration.Status.SetTypedReason(clusterV1alpha1.ClusterRegistrationReasonInvalidKubeconfig)
		return ctrl.Result{}, nil
	}

	if !util.IsClusterHealthy(remoteClientset) {
		log.Info("Cluster[" + ClusterRegistration.Spec.ClusterName + "] is invalid")
		ClusterRegistration.Status.SetTypedPhase(clusterV1alpha1.ClusterRegistrationPhaseError)
		ClusterRegistration.Status.SetTypedReason(clusterV1alpha1.ClusterRegistrationReasonClusterNotFound)
		return ctrl.Result{}, nil
	}

	// validate cluster manager duplication
	key := types.NamespacedName{
		Name:      ClusterRegistration.Spec.ClusterName,
		Namespace: ClusterRegistration.Namespace,
	}
	if err := r.Client.Get(context.TODO(), key, &clusterV1alpha1.ClusterManager{}); err != nil && !errors.IsNotFound(err) {
		log.Error(err, "Failed to get clusterManager")
		return ctrl.Result{}, err
	} else if err == nil {
		log.Info("ClusterManager is already existed")
		ClusterRegistration.Status.SetTypedPhase(clusterV1alpha1.ClusterRegistrationPhaseError)
		ClusterRegistration.Status.SetTypedReason(clusterV1alpha1.ClusterRegistrationReasonClusterNameDuplicated)
		return ctrl.Result{}, nil
	}

	// ClusterRegistration.Status.SetTypedPhase(clusterV1alpha1.ClusterRegistrationPhaseValidated)
	ClusterRegistration.Status.ClusterValidated = true
	return ctrl.Result{}, nil
}

func (r *ClusterRegistrationReconciler) CreateKubeconfigSecret(ctx context.Context, ClusterRegistration *clusterV1alpha1.ClusterRegistration) (ctrl.Result, error) {
	if !ClusterRegistration.Status.ClusterValidated || ClusterRegistration.Status.SecretReady {
		return ctrl.Result{}, nil
	}
	log := r.Log.WithValues("ClusterRegistration", ClusterRegistration.GetNamespacedName())
	log.Info("Start to reconcile phase for CreateKubeconfigSecret")

	key := types.NamespacedName{
		Name:      ClusterRegistration.Spec.ClusterName,
		Namespace: ClusterRegistration.Namespace,
	}
	clm := &clusterV1alpha1.ClusterManager{}
	if err := r.Client.Get(context.TODO(), key, clm); errors.IsNotFound(err) {
		log.Info("Wait for creating cluster manager")
		return ctrl.Result{}, err
	} else if err != nil {
		log.Error(err, "Failed to get cluster manager")
		return ctrl.Result{}, err
	}

	decodedKubeConfig, _ := b64.StdEncoding.DecodeString(ClusterRegistration.Spec.KubeConfig)
	kubeConfig, err := clientcmd.Load(decodedKubeConfig)
	if err != nil {
		log.Error(err, "Failed to get secret")
		return ctrl.Result{}, err
	}

	serverURI := kubeConfig.Clusters[kubeConfig.Contexts[kubeConfig.CurrentContext].Cluster].Server
	argoSecretName, err := util.URIToSecretName("cluster", serverURI)
	if err != nil {
		log.Error(err, "Failed to parse server uri")
		return ctrl.Result{}, err
	}

	kubeconfigSecretName := ClusterRegistration.Spec.ClusterName + util.KubeconfigSuffix
	key = types.NamespacedName{
		Name:      kubeconfigSecretName,
		Namespace: ClusterRegistration.Namespace,
	}
	kubeconfigSecret := &coreV1.Secret{}
	if err := r.Client.Get(context.TODO(), key, kubeconfigSecret); errors.IsNotFound(err) {
		kubeconfigSecret = &coreV1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      kubeconfigSecretName,
				Namespace: ClusterRegistration.Namespace,
				Annotations: map[string]string{
					util.AnnotationKeyOwner:             ClusterRegistration.Annotations[util.AnnotationKeyCreator],
					util.AnnotationKeyCreator:           ClusterRegistration.Annotations[util.AnnotationKeyCreator],
					util.AnnotationKeyArgoClusterSecret: argoSecretName,
				},
				Labels: map[string]string{
					util.LabelKeyClmSecretType:           util.ClmSecretTypeKubeconfig,
					clusterV1alpha1.LabelKeyClrName:      ClusterRegistration.Name,
					clusterV1alpha1.LabelKeyClmName:      ClusterRegistration.Spec.ClusterName,
					clusterV1alpha1.LabelKeyClmNamespace: ClusterRegistration.Namespace,
				},
			},
			StringData: map[string]string{
				"value": string(decodedKubeConfig),
			},
		}
		if err = r.Create(context.TODO(), kubeconfigSecret); err != nil {
			log.Error(err, "Failed to create kubeconfig Secret")
			return ctrl.Result{}, err
		}
		log.Info("Create kubeconfig Secret successfully")
	} else if err != nil {
		log.Error(err, "Failed to get kubeconfig Secret")
		return ctrl.Result{}, err
	} else if !kubeconfigSecret.GetDeletionTimestamp().IsZero() {
		return ctrl.Result{Requeue: true}, nil
	}

	// ClusterRegistration.Status.SetTypedPhase(clusterV1alpha1.ClusterRegistrationPhaseSecretCreated)
	ClusterRegistration.Status.SecretReady = true
	return ctrl.Result{}, nil
}

func (r *ClusterRegistrationReconciler) CreateClusterManager(ctx context.Context, clusterRegistration *clusterV1alpha1.ClusterRegistration) (ctrl.Result, error) {
	if clusterRegistration.Status.Phase != clusterV1alpha1.ClusterRegistrationPhaseRegistered {
		return ctrl.Result{}, nil
	}
	log := r.Log.WithValues("ClusterRegistration", clusterRegistration.GetNamespacedName())
	log.Info("Start to reconcile phase for CreateClusterManager")

	endpoint, err := GetRegWorkloadClusterEndpoint(clusterRegistration.Spec.KubeConfig)
	if err != nil {
		return ctrl.Result{}, err
	}

	key := clusterRegistration.GetCluterManagerNamespacedName()

	clm := &clusterV1alpha1.ClusterManager{}
	if err := r.Client.Get(context.TODO(), key, &clusterV1alpha1.ClusterManager{}); errors.IsNotFound(err) {
		clm = ConstructClusterManagerByRegistration(clusterRegistration)
		clm.Annotations[clusterV1alpha1.AnnotationKeyClmApiserver] = endpoint
		clm.Annotations[clusterV1alpha1.AnnotationKeyClmDomain] = os.Getenv(util.HC_DOMAIN)

		if err = r.Client.Create(context.TODO(), clm); err != nil {
			log.Error(err, "Failed to create ClusterManager for ["+clusterRegistration.Spec.ClusterName+"]")
			return ctrl.Result{}, err
		}
	} else if err != nil {
		log.Error(err, "Failed to get ClusterManager")
		return ctrl.Result{}, err
	}

	if err := util.Insert(clm); err != nil {
		log.Error(err, "Failed to insert cluster info into cluster_member table")
		return ctrl.Result{}, err
	}

	clusterRegistration.Status.SetTypedPhase(clusterV1alpha1.ClusterRegistrationPhaseRegistered)
	return ctrl.Result{}, nil
}

func ConstructClusterManagerByRegistration(clusterRegistration *clusterV1alpha1.ClusterRegistration) *clusterV1alpha1.ClusterManager {
	clm := &clusterV1alpha1.ClusterManager{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterRegistration.Spec.ClusterName,
			Namespace: clusterRegistration.Namespace,
			Annotations: map[string]string{
				util.AnnotationKeyOwner:   clusterRegistration.Annotations[util.AnnotationKeyCreator],
				util.AnnotationKeyCreator: clusterRegistration.Annotations[util.AnnotationKeyCreator],
			},
			Labels: map[string]string{
				clusterV1alpha1.LabelKeyClmClusterType: clusterV1alpha1.ClusterTypeRegistered,
				clusterV1alpha1.LabelKeyClrName:        clusterRegistration.Name,
			},
		},
		Spec: clusterV1alpha1.ClusterManagerSpec{},
	}
	return clm
}

func GetRegWorkloadClusterEndpoint(kubeconfig string) (string, error) {
	decodedKubeConfig, _ := b64.StdEncoding.DecodeString(kubeconfig)
	reg, _ := regexp.Compile(`https://[0-9a-zA-Z./-]+`)
	endpoint := reg.FindString(string(decodedKubeConfig))[len("https://"):]
	if endpoint == "" {
		return "", fmt.Errorf("endpoint is empty")
	}

	return endpoint, nil
}
