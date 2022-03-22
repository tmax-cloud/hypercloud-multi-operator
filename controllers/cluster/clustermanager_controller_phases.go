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
	"fmt"
	"regexp"
	"strings"

	argocdv1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	servicecatalogv1beta1 "github.com/kubernetes-sigs/service-catalog/pkg/apis/servicecatalog/v1beta1"
	clusterv1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/cluster/v1alpha1"
	util "github.com/tmax-cloud/hypercloud-multi-operator/controllers/util"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"

	capiv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"
)

func (r *ClusterManagerReconciler) UpdateClusterManagerStatus(ctx context.Context, clusterManager *clusterv1alpha1.ClusterManager) (ctrl.Result, error) {
	if clusterManager.Status.ControlPlaneReady {
		return ctrl.Result{}, nil
	}
	log := r.Log.WithValues("clustermanager", clusterManager.GetNamespacedName())
	log.Info("Start to reconcile phase for UpdateClusterManagerStatus")

	key := types.NamespacedName{
		Name:      clusterManager.Name + util.KubeconfigSuffix,
		Namespace: clusterManager.Namespace,
	}
	kubeconfigSecret := &corev1.Secret{}
	if err := r.Get(context.TODO(), key, kubeconfigSecret); errors.IsNotFound(err) {
		log.Info("Wait for creating kubeconfig secret")
		return ctrl.Result{RequeueAfter: requeueAfter10Sec}, nil
	} else if err != nil {
		log.Error(err, "Failed to get kubeconfig secret")
		return ctrl.Result{}, err
	}

	remoteClientset, err := util.GetRemoteK8sClient(kubeconfigSecret)
	if err != nil {
		log.Error(err, "Failed to get remoteK8sClient")
		return ctrl.Result{}, err
	}

	kubeadmConfig, err := remoteClientset.
		CoreV1().
		ConfigMaps(util.KubeNamespace).
		Get(context.TODO(), "kubeadm-config", metav1.GetOptions{})
	if err != nil {
		log.Error(err, "Failed to get kubeadm-config configmap from remote cluster")
		return ctrl.Result{}, err
	}

	jsonData, _ := yaml.YAMLToJSON([]byte(kubeadmConfig.Data["ClusterConfiguration"]))
	data := make(map[string]interface{})
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return ctrl.Result{}, err
	}
	clusterManager.Spec.Version = fmt.Sprintf("%v", data["kubernetesVersion"])

	nodeList, err := remoteClientset.
		CoreV1().
		Nodes().
		List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Error(err, "Failed to list remote K8s nodeList")
		return ctrl.Result{}, err
	}

	// var machineList *capiv1.machineList
	// if machineList, err =
	// todo - shkim
	// node list가 아닌 machine list를 불러서 ready체크를 해야 확실하지 않을까?
	clusterManager.Spec.MasterNum = 0
	clusterManager.Status.MasterRun = 0
	clusterManager.Spec.WorkerNum = 0
	clusterManager.Status.WorkerRun = 0
	clusterManager.Spec.Provider = util.ProviderUnknown
	clusterManager.Status.Provider = util.ProviderUnknown
	for _, node := range nodeList.Items {
		if _, ok := node.Labels["node-role.kubernetes.io/master"]; ok {
			clusterManager.Spec.MasterNum++
			if node.Status.Conditions[len(node.Status.Conditions)-1].Type == "Ready" {
				clusterManager.Status.MasterRun++
			}
		} else {
			clusterManager.Spec.WorkerNum++
			if node.Status.Conditions[len(node.Status.Conditions)-1].Type == "Ready" {
				clusterManager.Status.WorkerRun++
			}
		}

		if clusterManager.Spec.Provider == util.ProviderUnknown && node.Spec.ProviderID != "" {
			providerID, err := util.GetProviderName(
				strings.Split(node.Spec.ProviderID, "://")[0],
			)
			if err != nil {
				log.Error(err, "Cannot found given provider name.")
			}
			clusterManager.Status.Provider = providerID
			clusterManager.Spec.Provider = providerID
		}
	}

	if clusterManager.Spec.Provider == util.ProviderUnknown {
		reg, _ := regexp.Compile("cloud-provider: [a-zA-Z-_ ]+")
		matchString := reg.FindString(kubeadmConfig.Data["ClusterConfiguration"])
		if matchString != "" {
			cloudProvider, err := util.GetProviderName(
				matchString[len("cloud-provider: "):],
			)
			if err != nil {
				log.Error(err, "Cannot found given provider name.")
			}
			clusterManager.Status.Provider = cloudProvider
			clusterManager.Spec.Provider = cloudProvider
		}
	}

	// health check
	resp, err := remoteClientset.
		RESTClient().
		Get().
		AbsPath("/readyz").
		DoRaw(context.TODO())
	if err != nil {
		log.Error(err, "Failed to get remote cluster status")
		return ctrl.Result{}, err
	}
	if string(resp) == "ok" {
		clusterManager.Status.ControlPlaneReady = true
		//clusterManager.Status.AgentReady = true
		clusterManager.Status.Ready = true
	} else {
		// err := errors.NewBadRequest("Failed to healthcheck")
		// log.Error(err, "Failed to healthcheck")
		log.Info("Remote cluster is not ready... wait...")
		return ctrl.Result{RequeueAfter: requeueAfter30Sec}, nil
	}

	log.Info("Update status of ClusterManager successfully")
	generatedSuffix := util.CreateSuffixString()
	clusterManager.Annotations[clusterv1alpha1.AnnotationKeyClmSuffix] = generatedSuffix
	return ctrl.Result{}, nil
}

func (r *ClusterManagerReconciler) SetEndpoint(ctx context.Context, clusterManager *clusterv1alpha1.ClusterManager) (ctrl.Result, error) {
	if clusterManager.Annotations[clusterv1alpha1.AnnotationKeyClmApiserver] != "" {
		return ctrl.Result{}, nil
	}
	log := r.Log.WithValues("clustermanager", clusterManager.GetNamespacedName())
	log.Info("Start to reconcile phase for SetEndpoint")

	key := clusterManager.GetNamespacedName()
	cluster := &capiv1.Cluster{}
	if err := r.Get(context.TODO(), key, cluster); errors.IsNotFound(err) {
		log.Info("Failed to get cluster. Requeue after 20sec")
		return ctrl.Result{RequeueAfter: requeueAfter20Sec}, err
	} else if err != nil {
		log.Error(err, "Failed to get cluster")
		return ctrl.Result{}, err
	}

	if cluster.Spec.ControlPlaneEndpoint.Host == "" {
		log.Info("ControlPlain endpoint is not ready yet. requeue after 20sec")
		return ctrl.Result{RequeueAfter: requeueAfter20Sec}, nil
	}
	clusterManager.Annotations[clusterv1alpha1.AnnotationKeyClmApiserver] = cluster.Spec.ControlPlaneEndpoint.Host

	return ctrl.Result{}, nil
}
func (r *ClusterManagerReconciler) CreateTraefikResources(ctx context.Context, clusterManager *clusterv1alpha1.ClusterManager) (ctrl.Result, error) {
	if clusterManager.Status.TraefikReady {
		return ctrl.Result{}, nil
	}
	log := r.Log.WithValues("clustermanager", clusterManager.GetNamespacedName())
	log.Info("Start to reconcile phase for CreateTraefikResources")

	if err := r.CreateCertificate(clusterManager); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.CreateIngress(clusterManager); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.CreateService(clusterManager); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.CreateMiddleware(clusterManager); err != nil {
		return ctrl.Result{}, err
	}

	if !util.IsIpAddress(clusterManager.Annotations[clusterv1alpha1.AnnotationKeyClmApiserver]) {
		clusterManager.Status.TraefikReady = true
		return ctrl.Result{}, nil
	}

	if err := r.CreateEndpoint(clusterManager); err != nil {
		return ctrl.Result{}, err
	}

	clusterManager.Status.TraefikReady = true
	return ctrl.Result{}, nil
}

func (r *ClusterManagerReconciler) DeployAndUpdateAgentEndpoint(ctx context.Context, clusterManager *clusterv1alpha1.ClusterManager) (ctrl.Result, error) {
	log := r.Log.WithValues("clustermanager", clusterManager.GetNamespacedName())
	log.Info("Start to reconcile phase for DeployAndUpdateAgentEndpoint")

	// secret controller에서 clustermanager.status.controleplaneendpoint를 채워줄 때 까지 기다림
	if !clusterManager.Status.ControlPlaneReady {
		return ctrl.Result{RequeueAfter: requeueAfter1Min}, nil
	}

	kubeconfigSecret := &corev1.Secret{}
	key := types.NamespacedName{
		Name:      clusterManager.Name + util.KubeconfigSuffix,
		Namespace: clusterManager.Namespace,
	}
	if err := r.Get(context.TODO(), key, kubeconfigSecret); errors.IsNotFound(err) {
		log.Info("Wait for creating kubeconfig secret.")
		return ctrl.Result{RequeueAfter: requeueAfter10Sec}, nil
	} else if err != nil {
		log.Error(err, "Failed to get kubeconfig secret")
		return ctrl.Result{}, err
	}

	remoteClientset, err := util.GetRemoteK8sClient(kubeconfigSecret)
	if err != nil {
		log.Error(err, "Failed to get remoteK8sClient")
		return ctrl.Result{}, err
	}

	// ingress controller 존재하는지 먼저 확인하고 없으면 배포부터해.. 그전에 join되었는지도 먼저 확인해야하나...
	_, err = remoteClientset.
		CoreV1().
		Namespaces().
		Get(context.TODO(), util.IngressNginxNamespace, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		log.Info("Cannot found ingress namespace. Ingress-nginx is creating. Requeue after 30sec")
		return ctrl.Result{RequeueAfter: requeueAfter1Min}, nil
	} else if err != nil {
		log.Error(err, "Failed to get ingress-nginx namespace from remote cluster")
		return ctrl.Result{}, err
	} else {
		ingressController, err := remoteClientset.
			AppsV1().
			Deployments(util.IngressNginxNamespace).
			Get(context.TODO(), util.IngressNginxName, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			log.Info("Cannot found ingress controller. Ingress-nginx is creating. Requeue after 30sec")
			return ctrl.Result{RequeueAfter: requeueAfter1Min}, nil
		} else if err != nil {
			log.Error(err, "Failed to get ingress controller from remote cluster")
			return ctrl.Result{}, err
		} else {
			// 하나라도 ready라면..
			if ingressController.Status.ReadyReplicas == 0 {
				log.Info("Ingress controller is not ready. Requeue after 60sec")
				return ctrl.Result{RequeueAfter: requeueAfter1Min}, nil
			}
		}
	}

	clusterManager.Status.Ready = true
	return ctrl.Result{}, nil
}

func (r *ClusterManagerReconciler) CreateServiceInstance(ctx context.Context, clusterManager *clusterv1alpha1.ClusterManager) (ctrl.Result, error) {
	if clusterManager.Annotations[clusterv1alpha1.AnnotationKeyClmSuffix] != "" {
		return ctrl.Result{}, nil
	}
	log := r.Log.WithValues("clustermanager", clusterManager.GetNamespacedName())
	log.Info("Start to reconcile phase for CreateServiceInstance")

	key := types.NamespacedName{
		Name:      clusterManager.Name + clusterManager.Annotations[clusterv1alpha1.AnnotationKeyClmSuffix],
		Namespace: clusterManager.Namespace,
	}
	if err := r.Get(context.TODO(), key, &servicecatalogv1beta1.ServiceInstance{}); errors.IsNotFound(err) {
		clusterJson, err := json.Marshal(
			&ClusterParameter{
				Namespace:         clusterManager.Namespace,
				ClusterName:       clusterManager.Name,
				Owner:             clusterManager.Annotations[util.AnnotationKeyOwner],
				KubernetesVersion: clusterManager.Spec.Version,
				MasterNum:         clusterManager.Spec.MasterNum,
				WorkerNum:         clusterManager.Spec.WorkerNum,
			},
		)
		if err != nil {
			log.Error(err, "Failed to marshal cluster parameters")
		}

		var providerJson []byte
		switch strings.ToUpper(clusterManager.Spec.Provider) {
		case util.ProviderAws:
			providerJson, err = json.Marshal(
				&AwsParameter{
					SshKey:     clusterManager.AwsSpec.SshKey,
					Region:     clusterManager.AwsSpec.Region,
					MasterType: clusterManager.AwsSpec.MasterType,
					WorkerType: clusterManager.AwsSpec.WorkerType,
				},
			)
			if err != nil {
				log.Error(err, "Failed to marshal cluster parameters")
				return ctrl.Result{}, err
			}
		case util.ProviderVsphere:
			providerJson, err = json.Marshal(
				&VsphereParameter{
					PodCidr:             clusterManager.VsphereSpec.PodCidr,
					VcenterIp:           clusterManager.VsphereSpec.VcenterIp,
					VcenterId:           clusterManager.VsphereSpec.VcenterId,
					VcenterPassword:     clusterManager.VsphereSpec.VcenterPassword,
					VcenterThumbprint:   clusterManager.VsphereSpec.VcenterThumbprint,
					VcenterNetwork:      clusterManager.VsphereSpec.VcenterNetwork,
					VcenterDataCenter:   clusterManager.VsphereSpec.VcenterDataCenter,
					VcenterDataStore:    clusterManager.VsphereSpec.VcenterDataStore,
					VcenterFolder:       clusterManager.VsphereSpec.VcenterFolder,
					VcenterResourcePool: clusterManager.VsphereSpec.VcenterResourcePool,
					VcenterKcpIp:        clusterManager.VsphereSpec.VcenterKcpIp,
					VcenterCpuNum:       clusterManager.VsphereSpec.VcenterCpuNum,
					VcenterMemSize:      clusterManager.VsphereSpec.VcenterMemSize,
					VcenterDiskSize:     clusterManager.VsphereSpec.VcenterDiskSize,
					VcenterTemplate:     clusterManager.VsphereSpec.VcenterTemplate,
				},
			)
			if err != nil {
				log.Error(err, "Failed to marshal cluster parameters")
				return ctrl.Result{}, err
			}
		}

		clusterJson = util.MergeJson(clusterJson, providerJson)
		generatedSuffix := util.CreateSuffixString()
		serviceInstance := &servicecatalogv1beta1.ServiceInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterManager.Name + "-" + generatedSuffix,
				Namespace: clusterManager.Namespace,
				Annotations: map[string]string{
					util.AnnotationKeyOwner:   clusterManager.Annotations[util.AnnotationKeyCreator],
					util.AnnotationKeyCreator: clusterManager.Annotations[util.AnnotationKeyCreator],
				},
			},
			Spec: servicecatalogv1beta1.ServiceInstanceSpec{
				PlanReference: servicecatalogv1beta1.PlanReference{
					ClusterServiceClassExternalName: "capi-" + strings.ToLower(clusterManager.Spec.Provider) + "-template",
					ClusterServicePlanExternalName:  "capi-" + strings.ToLower(clusterManager.Spec.Provider) + "-template-plan-default",
				},
				Parameters: &runtime.RawExtension{
					Raw: clusterJson,
				},
			},
		}
		if err = r.Create(context.TODO(), serviceInstance); err != nil {
			log.Error(err, "Failed to create ServiceInstance")
			return ctrl.Result{}, err
		}

		ctrl.SetControllerReference(clusterManager, serviceInstance, r.Scheme)
		clusterManager.Annotations[clusterv1alpha1.AnnotationKeyClmSuffix] = generatedSuffix
	} else if err != nil {
		log.Error(err, "Failed to get ServiceInstance")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ClusterManagerReconciler) kubeadmControlPlaneUpdate(ctx context.Context, clusterManager *clusterv1alpha1.ClusterManager) (ctrl.Result, error) {
	log := r.Log.WithValues("clustermanager", clusterManager.GetNamespacedName())
	log.Info("Start to reconcile phase for kubeadmControlPlaneUpdate")

	key := types.NamespacedName{
		Name:      clusterManager.Name + "-control-plane",
		Namespace: clusterManager.Namespace,
	}
	kcp := &controlplanev1.KubeadmControlPlane{}
	if err := r.Get(context.TODO(), key, kcp); errors.IsNotFound(err) {
		return ctrl.Result{}, nil
	} else if err != nil {
		log.Error(err, "Failed to get clusterRole")
		return ctrl.Result{}, err
	}

	//create helper for patch
	helper, _ := patch.NewHelper(kcp, r.Client)
	defer func() {
		if err := helper.Patch(context.TODO(), kcp); err != nil {
			r.Log.Error(err, "KubeadmControlPlane patch error")
		}
	}()

	if *kcp.Spec.Replicas != int32(clusterManager.Spec.MasterNum) {
		*kcp.Spec.Replicas = int32(clusterManager.Spec.MasterNum)
	}

	if kcp.Spec.Version != clusterManager.Spec.Version {
		kcp.Spec.Version = clusterManager.Spec.Version
	}

	return ctrl.Result{}, nil
}

func (r *ClusterManagerReconciler) machineDeploymentUpdate(ctx context.Context, clusterManager *clusterv1alpha1.ClusterManager) (ctrl.Result, error) {
	log := r.Log.WithValues("clustermanager", clusterManager.GetNamespacedName())
	log.Info("Start to reconcile phase for machineDeploymentUpdate")

	key := types.NamespacedName{
		Name:      clusterManager.Name + "-md-0",
		Namespace: clusterManager.Namespace,
	}
	md := &capiv1.MachineDeployment{}
	if err := r.Get(context.TODO(), key, md); errors.IsNotFound(err) {
		return ctrl.Result{}, nil
	} else if err != nil {
		log.Error(err, "Failed to get clusterRole")
		return ctrl.Result{}, err
	}

	//create helper for patch
	helper, _ := patch.NewHelper(md, r.Client)
	defer func() {
		if err := helper.Patch(context.TODO(), md); err != nil {
			r.Log.Error(err, "kubeadmcontrolplane patch error")
		}
	}()

	if *md.Spec.Replicas != int32(clusterManager.Spec.WorkerNum) {
		*md.Spec.Replicas = int32(clusterManager.Spec.WorkerNum)
	}

	if *md.Spec.Template.Spec.Version != clusterManager.Spec.Version {
		*md.Spec.Template.Spec.Version = clusterManager.Spec.Version
	}

	return ctrl.Result{}, nil
}

func (r *ClusterManagerReconciler) CreateArgocdClusterSecret(ctx context.Context, clusterManager *clusterv1alpha1.ClusterManager) (ctrl.Result, error) {
	if clusterManager.Status.ArgoReady {
		return ctrl.Result{}, nil
	}
	log := r.Log.WithValues("ClusterManager", clusterManager.GetNamespacedName())
	log.Info("Start to reconcile phase for CreateArgocdClusterSecret")

	key := types.NamespacedName{
		Name:      clusterManager.Name + util.KubeconfigSuffix,
		Namespace: clusterManager.Namespace,
	}
	kubeConfigSecret := &corev1.Secret{}
	if err := r.Get(context.TODO(), key, kubeConfigSecret); errors.IsNotFound(err) {
		log.Info("KubeConfig Secret not found. Wait for creating")
		return ctrl.Result{RequeueAfter: requeueAfter10Sec}, err
	} else if err != nil {
		log.Error(err, "Failed to get kubeconfig Secret")
		return ctrl.Result{}, err
	}

	kubeConfig, err := clientcmd.Load(kubeConfigSecret.Data["value"])
	if err != nil {
		log.Error(err, "Failed to get kubeconfig data from secret")
		return ctrl.Result{}, err
	}

	configJson, err := json.Marshal(
		&argocdv1alpha1.ClusterConfig{
			TLSClientConfig: argocdv1alpha1.TLSClientConfig{
				Insecure: false,
				CertData: kubeConfig.AuthInfos[kubeConfig.Contexts[kubeConfig.CurrentContext].AuthInfo].ClientCertificateData,
				KeyData:  kubeConfig.AuthInfos[kubeConfig.Contexts[kubeConfig.CurrentContext].AuthInfo].ClientKeyData,
				CAData:   kubeConfig.Clusters[kubeConfig.Contexts[kubeConfig.CurrentContext].Cluster].CertificateAuthorityData,
			},
		},
	)
	if err != nil {
		log.Error(err, "Failed to marshal cluster authorization parameters")
	}

	clusterName := strings.Split(kubeConfigSecret.Name, util.KubeconfigSuffix)[0]
	key = types.NamespacedName{
		Name:      kubeConfigSecret.Annotations[util.AnnotationKeyArgoClusterSecret],
		Namespace: util.ArgoNamespace,
	}
	argocdClusterSecret := &corev1.Secret{}
	if err := r.Get(context.TODO(), key, argocdClusterSecret); errors.IsNotFound(err) {
		argocdClusterSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      kubeConfigSecret.Annotations[util.AnnotationKeyArgoClusterSecret],
				Namespace: util.ArgoNamespace,
				Annotations: map[string]string{
					util.AnnotationKeyOwner:         kubeConfigSecret.Annotations[util.AnnotationKeyOwner],
					util.AnnotationKeyCreator:       kubeConfigSecret.Annotations[util.AnnotationKeyCreator],
					util.AnnotationKeyArgoManagedBy: util.ArgoApiGroup,
				},
				Labels: map[string]string{
					util.LabelKeyClmSecretType:           util.ClmSecretTypeArgo,
					util.LabelKeyArgoSecretType:          util.ArgoSecretTypeCluster,
					clusterv1alpha1.LabelKeyClmName:      clusterManager.Name,
					clusterv1alpha1.LabelKeyClmNamespace: clusterManager.Namespace,
				},
				Finalizers: []string{
					clusterv1alpha1.ClusterManagerFinalizer,
				},
			},
			StringData: map[string]string{
				"config": string(configJson),
				"name":   clusterName,
				"server": kubeConfig.Clusters[kubeConfig.Contexts[kubeConfig.CurrentContext].Cluster].Server,
			},
		}
		if err := r.Create(context.TODO(), argocdClusterSecret); err != nil {
			log.Error(err, "Cannot create Argocd Secret for remote cluster")
			return ctrl.Result{}, err
		}
		log.Info("Create Argocd Secret for remote cluster successfully")
	} else if err != nil {
		log.Error(err, "Failed to get Argocd Secret for remote cluster")
		return ctrl.Result{}, err
	} else if !argocdClusterSecret.GetDeletionTimestamp().IsZero() {
		return ctrl.Result{Requeue: true}, nil
	}

	clusterManager.Status.ArgoReady = true
	return ctrl.Result{}, nil
}

func (r *ClusterManagerReconciler) UpdateGatewayService(ctx context.Context, clusterManager *clusterv1alpha1.ClusterManager) (reconcile.Result, error) {
	if clusterManager.Status.MonitoringReady && clusterManager.Status.PrometheusReady {
		return ctrl.Result{}, nil
	}
	log := r.Log.WithValues("clustermanager", clusterManager.GetNamespacedName())
	log.Info("Start to reconcile phase for UpdateGatewayService")

	key := types.NamespacedName{
		Name:      clusterManager.Name + util.KubeconfigSuffix,
		Namespace: clusterManager.Namespace,
	}
	kubeconfigSecret := &corev1.Secret{}
	if err := r.Get(context.TODO(), key, kubeconfigSecret); errors.IsNotFound(err) {
		log.Info("Wait for creating kubeconfig secret")
		return ctrl.Result{RequeueAfter: requeueAfter10Sec}, nil
	} else if err != nil {
		log.Error(err, "Failed to get kubeconfig secret")
		return ctrl.Result{}, err
	}

	remoteClientset, err := util.GetRemoteK8sClient(kubeconfigSecret)
	if err != nil {
		log.Error(err, "Failed to get remoteK8sClient")
		return ctrl.Result{}, err
	}

	gatewayService, err := remoteClientset.
		CoreV1().
		Services(util.ApiGatewayNamespace).
		Get(context.TODO(), "gateway", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		log.Error(err, "Cannot found Service for gateway. Wait for installing api-gateway. Requeue after 1 min")
		return ctrl.Result{Requeue: true, RequeueAfter: requeueAfter1Min}, err
	} else if err != nil {
		log.Error(err, "Failed to get Service for gateway")
		return ctrl.Result{}, err
	} else {
		ingress := gatewayService.Status.LoadBalancer.Ingress[0]
		hostnameOrIp := ingress.Hostname + ingress.IP
		if hostnameOrIp == "" {
			err := fmt.Errorf("Service for gateway doesn't have both hostname and ip address")
			log.Error(err, "Service for api-gateway is not Ready. Requeue after 1 min")
			return ctrl.Result{Requeue: true, RequeueAfter: requeueAfter1Min}, err
		}

		clusterManager.Annotations[clusterv1alpha1.AnnotationKeyClmGateway] = hostnameOrIp
	}

	if err := r.CreateGatewayService(clusterManager); err != nil {
		return ctrl.Result{}, err
	}

	if !util.IsIpAddress(clusterManager.Annotations[clusterv1alpha1.AnnotationKeyClmApiserver]) {
		clusterManager.Status.MonitoringReady = true
		clusterManager.Status.PrometheusReady = true
		return ctrl.Result{}, nil
	}

	if err := r.CreateGatewayEndpoint(clusterManager); err != nil {
		return ctrl.Result{}, err
	}

	clusterManager.Status.MonitoringReady = true
	clusterManager.Status.PrometheusReady = true
	return ctrl.Result{}, nil
}

func (r *ClusterManagerReconciler) DeleteTraefikResources(clusterManager *clusterv1alpha1.ClusterManager) error {
	if err := r.DeleteCertificate(clusterManager); err != nil {
		return err
	}

	if err := r.DeleteCertSecret(clusterManager); err != nil {
		return err
	}

	if err := r.DeleteIngress(clusterManager); err != nil {
		return err
	}

	if err := r.DeleteService(clusterManager); err != nil {
		return err
	}

	if err := r.DeleteEndpoint(clusterManager); err != nil {
		return err
	}

	if err := r.DeleteMiddleware(clusterManager); err != nil {
		return err
	}

	if err := r.DeleteGatewayService(clusterManager); err != nil {
		return err
	}

	if err := r.DeleteGatewayEndpoint(clusterManager); err != nil {
		return err
	}

	return nil
}
