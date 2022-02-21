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
	"os"
	"regexp"

	clusterv1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/cluster/v1alpha1"
	util "github.com/tmax-cloud/hypercloud-multi-operator/controllers/util"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"

	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *ClusterRegistrationReconciler) CheckValidation(ctx context.Context, ClusterRegistration *clusterv1alpha1.ClusterRegistration) (ctrl.Result, error) {
	if ClusterRegistration.Status.Phase != "" {
		return ctrl.Result{}, nil
	}
	log := r.Log.WithValues("ClusterRegistration", ClusterRegistration.GetNamespacedName())
	log.Info("Start to CheckValidation reconcile for [" + ClusterRegistration.Name + "]")

	// decode base64 encoded kubeconfig file
	if encodedKubeConfig, err := b64.StdEncoding.DecodeString(ClusterRegistration.Spec.KubeConfig); err != nil {
		log.Error(err, "Failed to decode ClusterRegistration.Spec.KubeConfig, maybe wrong kubeconfig file")
		ClusterRegistration.Status.SetTypedPhase(clusterv1alpha1.ClusterRegistrationPhaseFailed)
		ClusterRegistration.Status.SetTypedReason(clusterv1alpha1.ClusterRegistrationReasonInvalidKubeconfig)
		return ctrl.Result{Requeue: false}, err
	} else {
		// validate remote cluster
		log.Info("Start to CheckKubeconfigValidation reconcile for [" + ClusterRegistration.Name + "]")
		if remoteClientset, err := util.GetRemoteK8sClientByKubeConfig(encodedKubeConfig); err != nil {
			log.Error(err, "Failed to get client for ["+ClusterRegistration.Spec.ClusterName+"]")
			ClusterRegistration.Status.SetTypedPhase(clusterv1alpha1.ClusterRegistrationPhaseFailed)
			ClusterRegistration.Status.SetTypedReason(clusterv1alpha1.ClusterRegistrationReasonInvalidKubeconfig)
			return ctrl.Result{Requeue: false}, err
		} else {
			// TODO
			// nodelist가 아닌 api-server call검증 api는 따로 없나...?
			if nodeList, err :=
				remoteClientset.
					CoreV1().
					Nodes().
					List(
						context.TODO(),
						metav1.ListOptions{},
					); err != nil {
				if nodeList.Items == nil {
					log.Info("Failed to get nodes for [" + ClusterRegistration.Spec.ClusterName + "]")
					ClusterRegistration.Status.SetTypedPhase(clusterv1alpha1.ClusterRegistrationPhaseFailed)
					ClusterRegistration.Status.SetTypedReason(clusterv1alpha1.ClusterRegistrationReasonClusterNotFound)
					return ctrl.Result{}, nil
				}
			}
		}
	}

	// validate cluster manger duplication
	clm := clusterv1alpha1.ClusterManager{}
	clmKey := types.NamespacedName{
		Name:      ClusterRegistration.Spec.ClusterName,
		Namespace: ClusterRegistration.Namespace,
	}
	if err := r.Get(context.TODO(), clmKey, &clm); err != nil {
		if errors.IsNotFound(err) {
			log.Info("ClusterManager [" + ClusterRegistration.Spec.ClusterName + "] does not exist. Duplication condition is passed")
		} else {
			log.Error(err, "Failed to get clusterManager")
			return ctrl.Result{}, err
		}
	} else {
		log.Info("ClusterManager [" + clm.Name + "] is already existed")
		ClusterRegistration.Status.SetTypedPhase(clusterv1alpha1.ClusterRegistrationPhaseFailed)
		ClusterRegistration.Status.SetTypedReason(clusterv1alpha1.ClusterRegistrationReasonClusterNameDuplicated)
		return ctrl.Result{Requeue: false}, err
	}

	ClusterRegistration.Status.SetTypedPhase(clusterv1alpha1.ClusterRegistrationPhaseValidated)
	return ctrl.Result{}, nil
}

func (r *ClusterRegistrationReconciler) CreateKubeconfigSecret(ctx context.Context, ClusterRegistration *clusterv1alpha1.ClusterRegistration) (ctrl.Result, error) {
	if ClusterRegistration.Status.Phase != clusterv1alpha1.ClusterRegistrationPhaseValidated {
		return ctrl.Result{}, nil
	}
	log := r.Log.WithValues("ClusterRegistration", ClusterRegistration.GetNamespacedName())
	log.Info("Start to reconcile phase for CreateKubeconfigSecret")

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

	kubeconfigSecret := &corev1.Secret{}
	kubeconfigSecretName := ClusterRegistration.Spec.ClusterName + util.KubeconfigSuffix
	kubeconfigSecretKey := types.NamespacedName{
		Name:      kubeconfigSecretName,
		Namespace: ClusterRegistration.Namespace,
	}

	if err := r.Get(context.TODO(), kubeconfigSecretKey, kubeconfigSecret); err != nil {
		if errors.IsNotFound(err) {
			kubeconfigSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      kubeconfigSecretName,
					Namespace: ClusterRegistration.Namespace,
					Annotations: map[string]string{
						util.AnnotationKeyOwner:             ClusterRegistration.Annotations[util.AnnotationKeyCreator],
						util.AnnotationKeyCreator:           ClusterRegistration.Annotations[util.AnnotationKeyCreator],
						util.AnnotationKeyArgoClusterSecret: argoSecretName,
					},
					Labels: map[string]string{
						util.LabelKeyClmSecretType:      util.ClmSecretTypeKubeconfig,
						clusterv1alpha1.LabelKeyClmName: ClusterRegistration.Spec.ClusterName,
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
		} else {
			log.Error(err, "Failed to get kubeconfigSecret")
			return ctrl.Result{}, err
		}
	}

	ClusterRegistration.Status.SetTypedPhase(clusterv1alpha1.ClusterRegistrationPhaseSecretCreated)
	return ctrl.Result{}, nil
}

func (r *ClusterRegistrationReconciler) CreateClusterManager(ctx context.Context, ClusterRegistration *clusterv1alpha1.ClusterRegistration) (ctrl.Result, error) {
	if ClusterRegistration.Status.Phase != clusterv1alpha1.ClusterRegistrationPhaseSecretCreated {
		return ctrl.Result{}, nil
	}
	log := r.Log.WithValues("ClusterRegistration", ClusterRegistration.GetNamespacedName())
	log.Info("Start to reconcile phase for CreateClusterManager ")

	decodedKubeConfig, _ := b64.StdEncoding.DecodeString(ClusterRegistration.Spec.KubeConfig)
	reg, _ := regexp.Compile("https://[0-9a-zA-Z./-]+")
	endpoint := reg.FindString(string(decodedKubeConfig))[len("https://"):]

	clm := &clusterv1alpha1.ClusterManager{}
	clmKey := types.NamespacedName{
		Name:      ClusterRegistration.Spec.ClusterName,
		Namespace: ClusterRegistration.Namespace,
	}
	if err := r.Get(context.TODO(), clmKey, clm); err != nil {
		if errors.IsNotFound(err) {
			clm = &clusterv1alpha1.ClusterManager{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ClusterRegistration.Spec.ClusterName,
					Namespace: ClusterRegistration.Namespace,
					Annotations: map[string]string{
						util.AnnotationKeyOwner:                   ClusterRegistration.Annotations[util.AnnotationKeyCreator],
						util.AnnotationKeyCreator:                 ClusterRegistration.Annotations[util.AnnotationKeyCreator],
						clusterv1alpha1.AnnotationKeyClmApiserver: endpoint,
						clusterv1alpha1.AnnotationKeyClmDomain:    os.Getenv("HC_DOMAIN"),
					},
					Labels: map[string]string{
						clusterv1alpha1.LabelKeyClmClusterType: clusterv1alpha1.ClusterTypeRegistered,
						clusterv1alpha1.LabelKeyClrName:        ClusterRegistration.Name,
					},
				},
				Spec: clusterv1alpha1.ClusterManagerSpec{},
			}
			if err = r.Create(context.TODO(), clm); err != nil {
				log.Error(err, "Failed to create ClusterManager for ["+ClusterRegistration.Spec.ClusterName+"]")
				return ctrl.Result{}, err
			}

			if err := util.Insert(clm); err != nil {
				log.Error(err, "Failed to insert cluster info into cluster_member table")
				return ctrl.Result{}, err
			}

		} else {
			log.Error(err, "Failed to get ClusterManager")
			return ctrl.Result{}, err
		}
	} else {
		log.Info("Cannot create ClusterManager. ClusterManager is already exist")
	}

	ClusterRegistration.Status.SetTypedPhase(clusterv1alpha1.ClusterRegistrationPhaseSuccess)
	return ctrl.Result{}, nil
}
