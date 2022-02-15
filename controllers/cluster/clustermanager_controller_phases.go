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

	certmanagerv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	servicecatalogv1beta1 "github.com/kubernetes-sigs/service-catalog/pkg/apis/servicecatalog/v1beta1"
	clusterv1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/cluster/v1alpha1"
	util "github.com/tmax-cloud/hypercloud-multi-operator/controllers/util"
	traefikv2 "github.com/traefik/traefik/v2/pkg/provider/kubernetes/crd/traefik/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	capiv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"
)

func (r *ClusterManagerReconciler) UpdateClusterManagerStatus(ctx context.Context, clusterManager *clusterv1alpha1.ClusterManager) (ctrl.Result, error) {
	log := r.Log.WithValues("clustermanager", types.NamespacedName{Name: clusterManager.Name, Namespace: clusterManager.Namespace})
	log.Info("Start to UpdateClusterManagerStatus")

	kubeconfigSecret := &corev1.Secret{}
	kubeconfigSecretKey := types.NamespacedName{
		Name:      clusterManager.Name + util.KubeconfigSuffix,
		Namespace: clusterManager.Namespace,
	}
	if err := r.Get(context.TODO(), kubeconfigSecretKey, kubeconfigSecret); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Cannot found kubeconfig secret. Wait to create kubeconfig secret.")
			return ctrl.Result{RequeueAfter: requeueAfter10Sec}, nil
		} else {
			log.Error(err, "Failed to get kubeconfig secret")
			return ctrl.Result{}, err
		}
	}

	remoteClientset, err := util.GetRemoteK8sClient(kubeconfigSecret)
	if err != nil {
		log.Error(err, "Failed to get remoteK8sClient")
		return ctrl.Result{}, err
	}

	var kubeadmConfig *corev1.ConfigMap
	if kubeadmConfig, err =
		remoteClientset.
			CoreV1().
			ConfigMaps(util.KubeNamespace).
			Get(
				context.TODO(),
				"kubeadm-config",
				metav1.GetOptions{},
			); err != nil {
		log.Error(err, "Failed to get kubeadm-config configmap from remote cluster")
		return ctrl.Result{}, err
	}

	jsonData, _ := yaml.YAMLToJSON([]byte(kubeadmConfig.Data["ClusterConfiguration"]))
	data := make(map[string]interface{})
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return ctrl.Result{}, err
	}
	clusterManager.Spec.Version = fmt.Sprintf("%v", data["kubernetesVersion"])

	var nodeList *corev1.NodeList
	if nodeList, err =
		remoteClientset.
			CoreV1().
			Nodes().
			List(
				context.TODO(),
				metav1.ListOptions{},
			); err != nil {
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
	var resp []byte
	if resp, err =
		remoteClientset.
			RESTClient().
			Get().
			AbsPath("/readyz").
			DoRaw(
				context.TODO(),
			); err != nil {
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

	generatedSuffix := util.CreateSuffixString()
	clusterManager.Annotations[util.AnnotationKeyClmSuffix] = generatedSuffix
	return ctrl.Result{}, nil
}

func (r *ClusterManagerReconciler) SetEndpoint(ctx context.Context, clusterManager *clusterv1alpha1.ClusterManager) (ctrl.Result, error) {
	log := r.Log.WithValues("clustermanager", types.NamespacedName{Name: clusterManager.Name, Namespace: clusterManager.Namespace})
	log.Info("Start setting endpoint configuration to clusterManager")

	if clusterManager.Annotations[util.AnnotationKeyClmApiserverEndpoint] != "" {
		log.Info("Endpoint already configured.")
		return ctrl.Result{}, nil
	}

	cluster := &capiv1.Cluster{}
	clusterKey := types.NamespacedName{
		Name:      clusterManager.Name,
		Namespace: clusterManager.Namespace,
	}
	if err := r.Get(context.TODO(), clusterKey, cluster); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Fail to get cluster. Requeue after 20sec.")
			return ctrl.Result{RequeueAfter: requeueAfter20Sec}, err
		} else {
			log.Error(err, "Failed to get cluster information")
			return ctrl.Result{}, err
		}
	}

	if cluster.Spec.ControlPlaneEndpoint.Host == "" {
		log.Info("ControlPlain endpoint is not ready yet. requeue after 20sec.")
		return ctrl.Result{RequeueAfter: requeueAfter20Sec}, nil
	}
	clusterManager.Annotations[util.AnnotationKeyClmApiserverEndpoint] = cluster.Spec.ControlPlaneEndpoint.Host

	return ctrl.Result{}, nil
}
func (r *ClusterManagerReconciler) CreateTraefikResources(ctx context.Context, clusterManager *clusterv1alpha1.ClusterManager) (ctrl.Result, error) {
	log := r.Log.WithValues("clustermanager", types.NamespacedName{Name: clusterManager.Name, Namespace: clusterManager.Namespace})

	if clusterManager.Annotations[util.AnnotationKeyClmApiserverEndpoint] == "" {
		log.Info("Wait for recognize remote apiserver endpoint")
		return ctrl.Result{RequeueAfter: requeueAfter20Sec}, nil
	}

	traefikCertificate := &certmanagerv1.Certificate{}
	certificateKey := types.NamespacedName{
		//Name:      clusterManager.Name + "-certificate-" + clusterManager.Annotations[util.AnnotationKeyClmSuffix],
		Name:      clusterManager.Name + "-certificate",
		Namespace: clusterManager.Namespace,
	}
	if err := r.Get(context.TODO(), certificateKey, traefikCertificate); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Start creating Certificate")
			traefikCertificate = util.CreateCertificate(clusterManager)
			if err := r.Create(context.TODO(), traefikCertificate); err != nil {
				log.Error(err, "Failed to create Certificate")
				return ctrl.Result{}, err
			}
			ctrl.SetControllerReference(clusterManager, traefikCertificate, r.Scheme)
		} else {
			log.Error(err, "Failed to get Certificate information")
			return ctrl.Result{}, err
		}
	} else {
		log.Info("Certificate is already existed")
	}

	traefikIngress := &networkingv1.Ingress{}
	ingressKey := types.NamespacedName{
		//Name:      clusterManager.Name + "-ingress-" + clusterManager.Annotations[util.AnnotationKeyClmSuffix],
		Name:      clusterManager.Name + "-ingress",
		Namespace: clusterManager.Namespace,
	}
	if err := r.Get(context.TODO(), ingressKey, traefikIngress); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Start creating Ingress")
			traefikIngress = util.CreateIngress(clusterManager)
			if err := r.Create(context.TODO(), traefikIngress); err != nil {
				log.Error(err, "Failed to create Ingress")
				return ctrl.Result{}, err
			}
			ctrl.SetControllerReference(clusterManager, traefikIngress, r.Scheme)
		} else {
			log.Error(err, "Failed to get Ingress information")
			return ctrl.Result{}, err
		}
	} else {
		log.Info("Ingress is already existed")
	}

	traefikService := &corev1.Service{}
	serviceKey := types.NamespacedName{
		//Name:      clusterManager.Name + "-service-" + clusterManager.Annotations[util.AnnotationKeyClmSuffix],
		Name:      clusterManager.Name + "-service",
		Namespace: clusterManager.Namespace,
	}
	if err := r.Get(context.TODO(), serviceKey, traefikService); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Start creating Service")
			traefikService = util.CreateService(clusterManager)
			if err := r.Create(context.TODO(), traefikService); err != nil {
				log.Error(err, "Failed to create Service")
				return ctrl.Result{}, err
			}
			ctrl.SetControllerReference(clusterManager, traefikService, r.Scheme)
		} else {
			log.Error(err, "Failed to get Service information")
			return ctrl.Result{}, err
		}
	} else {
		log.Info("Service is already existed")
	}

	if util.IsIpAddress(clusterManager.Annotations[util.AnnotationKeyClmApiserverEndpoint]) {
		traefikEndpoint := &corev1.Endpoints{}
		endpointKey := types.NamespacedName{
			// Name: clusterManager.Name + "-service" + clusterManager.Annotations[util.AnnotationKeyClmSuffix],
			Name:      clusterManager.Name + "-service",
			Namespace: clusterManager.Namespace,
		}
		if err := r.Get(context.TODO(), endpointKey, traefikEndpoint); err != nil {
			if errors.IsNotFound(err) {
				log.Info("Start creating endpoint")
				traefikEndpoint = util.CreateEndpoint(clusterManager)
				if err := r.Create(context.TODO(), traefikEndpoint); err != nil {
					log.Error(err, "Failed to create Endpoint")
					return ctrl.Result{}, err
				}
				ctrl.SetControllerReference(clusterManager, traefikEndpoint, r.Scheme)
			} else {
				log.Error(err, "Failed to get Endpoint information")
				return ctrl.Result{}, err
			}
		} else {
			log.Info("Endpoint is already existed")
		}
	}

	traefikMiddleware := &traefikv2.Middleware{}
	middlewareKey := types.NamespacedName{
		//Name:      clusterManager.Name + "-prefix-" + clusterManager.Annotations[util.AnnotationKeyClmSuffix],
		Name:      clusterManager.Name + "-prefix",
		Namespace: clusterManager.Namespace,
	}
	if err := r.Get(context.TODO(), middlewareKey, traefikMiddleware); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Start creating Middleware")
			traefikMiddleware = util.CreateMiddleware(clusterManager)
			if err := r.Create(context.TODO(), traefikMiddleware); err != nil {
				log.Error(err, "Failed to create Middleware")
				return ctrl.Result{}, err
			}
			ctrl.SetControllerReference(clusterManager, traefikMiddleware, r.Scheme)
		} else {
			log.Error(err, "Failed to get Middleware information")
			return ctrl.Result{}, err
		}
	} else {
		log.Info("Middleware is already existed")
	}

	return ctrl.Result{}, nil
}

func (r *ClusterManagerReconciler) DeployAndUpdateAgentEndpoint(ctx context.Context, clusterManager *clusterv1alpha1.ClusterManager) (ctrl.Result, error) {
	log := r.Log.WithValues("clustermanager", types.NamespacedName{Name: clusterManager.Name, Namespace: clusterManager.Namespace})
	// secret controller에서 clustermanager.status.controleplaneendpoint를 채워줄 때 까지 기다림
	if !clusterManager.Status.ControlPlaneReady {
		// requeue (wait cluster controller)
		return ctrl.Result{RequeueAfter: requeueAfter60Sec}, nil
	} else /*if !clusterManager.Status.AgentReady*/ {
		kubeconfigSecret := &corev1.Secret{}
		kubeconfigSecretKey := types.NamespacedName{
			Name:      clusterManager.Name + util.KubeconfigSuffix,
			Namespace: clusterManager.Namespace,
		}
		if err := r.Get(context.TODO(), kubeconfigSecretKey, kubeconfigSecret); err != nil {
			if errors.IsNotFound(err) {
				log.Info("Cannot found kubeconfig secret. Wait to create kubeconfig secret.")
				return ctrl.Result{RequeueAfter: requeueAfter10Sec}, nil
			} else {
				log.Error(err, "Failed to get kubeconfig secret")
				return ctrl.Result{}, err
			}
		}

		remoteClientset, err := util.GetRemoteK8sClient(kubeconfigSecret)
		if err != nil {
			log.Error(err, "Failed to get remoteK8sClient")
			return ctrl.Result{}, err
		}

		// ingress controller 존재하는지 먼저 확인하고 없으면 배포부터해.. 그전에 join되었는지도 먼저 확인해야하나...
		if _, err =
			remoteClientset.
				CoreV1().
				Namespaces().
				Get(
					context.TODO(),
					util.IngressNginxNamespace,
					metav1.GetOptions{},
				); err != nil {
			if errors.IsNotFound(err) {
				log.Info("Cannot found ingress namespace... ingress-nginx is creating... requeue after 30sec")
				return ctrl.Result{RequeueAfter: requeueAfter60Sec}, nil
			} else {
				log.Error(err, "Failed to get ingress-nginx namespace from remote cluster")
				return ctrl.Result{}, err
			}
		} else {
			var ingressController *appsv1.Deployment
			if ingressController, err =
				remoteClientset.
					AppsV1().
					Deployments(util.IngressNginxNamespace).
					Get(
						context.TODO(),
						util.IngressNginxName,
						metav1.GetOptions{},
					); err != nil {
				if errors.IsNotFound(err) {
					log.Info("Cannot found ingress controller... ingress-nginx is creating... requeue after 30sec")
					return ctrl.Result{RequeueAfter: requeueAfter60Sec}, nil
				} else {
					log.Error(err, "Failed to get ingress controller from remote cluster")
					return ctrl.Result{}, err
				}
			} else {
				// 하나라도 ready라면..
				if ingressController.Status.ReadyReplicas == 0 {
					log.Info("Ingress controller is not ready...  requeue after 60sec")
					return ctrl.Result{RequeueAfter: requeueAfter60Sec}, nil
				}
			}
		}
	}

	clusterManager.Status.Ready = true
	return ctrl.Result{}, nil
}

func (r *ClusterManagerReconciler) UpdatePrometheusService(ctx context.Context, clusterManager *clusterv1alpha1.ClusterManager) (reconcile.Result, error) {
	targetClm := types.NamespacedName{
		Name:      clusterManager.GetName(),
		Namespace: clusterManager.GetNamespace(),
	}
	log := r.Log.WithValues("clustermanager", targetClm)
	log.Info("Start to reconcile UpdatePrometheusService... ")

	kubeconfigSecret := &corev1.Secret{}
	kubeconfigSecretKey := types.NamespacedName{
		Name:      clusterManager.Name + util.KubeconfigSuffix,
		Namespace: clusterManager.Namespace,
	}
	if err := r.Get(context.TODO(), kubeconfigSecretKey, kubeconfigSecret); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Cannot found kubeconfig secret. Wait to create kubeconfig secret.")
			return ctrl.Result{RequeueAfter: requeueAfter10Sec}, nil
		} else {
			log.Error(err, "Failed to get kubeconfig secret")
			return ctrl.Result{}, err
		}
	}

	remoteClientset, err := util.GetRemoteK8sClient(kubeconfigSecret)
	if err != nil {
		log.Error(err, "Failed to get remoteK8sClient")
		return ctrl.Result{}, err
	}

	if gatewayService, err :=
		remoteClientset.
			CoreV1().
			Services(util.ApiGatewayNamespace).
			Get(
				context.TODO(),
				"gateway",
				metav1.GetOptions{},
			); err != nil {
		if errors.IsNotFound(err) {
			log.Error(err, "Cannot found gateway Service. Wait for installing api-gateway. Requeue after 10 mins")
			return ctrl.Result{RequeueAfter: requeueAfter10min}, err
		}

		log.Error(err, "Failed to get gateway Service")
		return ctrl.Result{}, err
	} else {
		if gatewayService.Status.LoadBalancer.Ingress[0].Hostname == "" &&
			gatewayService.Status.LoadBalancer.Ingress[0].IP == "" {
			err := fmt.Errorf("gateway service doesn't have hostname or ip address")
			log.Error(err, "Service for api-gateway is not Ready")
			return ctrl.Result{RequeueAfter: requeueAfter10min}, err
		}

		clusterManager.Annotations[util.AnnotationKeyClmGatewayEndpoint] =
			gatewayService.Status.LoadBalancer.Ingress[0].Hostname + gatewayService.Status.LoadBalancer.Ingress[0].IP
	}

	prometheusService := &corev1.Service{}
	serviceKey := types.NamespacedName{
		Name:      clusterManager.GetName() + "-prometheus-service",
		Namespace: clusterManager.GetNamespace(),
	}
	if err := r.Get(context.TODO(), serviceKey, prometheusService); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Service for prometheus is not found. start to create")
			prometheusService = util.CreatePrometheusService(clusterManager)
			if err := r.Create(context.TODO(), prometheusService); err != nil {
				log.Error(err, "Failed to create Service for prometheus")
				return ctrl.Result{}, err
			}
			ctrl.SetControllerReference(clusterManager, prometheusService, r.Scheme)
		} else {
			log.Error(err, "Failed to get Service for prometheus")
			return ctrl.Result{}, err
		}
	} else {
		log.Info("Service for prometheus is already existed")
	}

	if util.IsIpAddress(clusterManager.Annotations[util.AnnotationKeyClmApiserverEndpoint]) {
		prometheusEndpoint := &corev1.Endpoints{}
		endpointKey := types.NamespacedName{
			Name:      clusterManager.GetName() + "-prometheus-service",
			Namespace: clusterManager.GetNamespace(),
		}
		if err := r.Get(context.TODO(), endpointKey, prometheusEndpoint); err != nil {
			if errors.IsNotFound(err) {
				log.Info("Endpoint for prometheus is not found. start to create")
				prometheusEndpoint = util.CreatePrometheusEndpoint(clusterManager)
				if err := r.Create(context.TODO(), prometheusEndpoint); err != nil {
					log.Error(err, "Failed to create Endpoint for prometheus")
					return ctrl.Result{}, err
				}
				ctrl.SetControllerReference(clusterManager, prometheusEndpoint, r.Scheme)
			} else {
				log.Error(err, "Failed to get Endpoint for prometheus")
				return ctrl.Result{}, err
			}
		} else {
			log.Info("Endpoint for prometheus is already existed")
		}
	}

	//clusterManager.Status.
	return ctrl.Result{}, nil
}

func (r *ClusterManagerReconciler) CreateServiceInstance(ctx context.Context, clusterManager *clusterv1alpha1.ClusterManager) (ctrl.Result, error) {
	log := r.Log.WithValues("clustermanager", types.NamespacedName{Name: clusterManager.Name, Namespace: clusterManager.Namespace})

	if clusterManager.Annotations[util.AnnotationKeyClmSuffix] != "" {
		return ctrl.Result{}, nil
	}

	serviceInstance := &servicecatalogv1beta1.ServiceInstance{}
	serviceInstanceKey := types.NamespacedName{
		Name:      clusterManager.Name,
		Namespace: clusterManager.Namespace,
	}
	if err := r.Get(context.TODO(), serviceInstanceKey, serviceInstance); err != nil {
		if errors.IsNotFound(err) {
			var clusterJson, providerJson []byte
			if clusterJson, err = json.Marshal(
				&ClusterParameter{
					Namespace:         clusterManager.Namespace,
					ClusterName:       clusterManager.Name,
					Owner:             clusterManager.Annotations[util.AnnotationKeyOwner],
					KubernetesVersion: clusterManager.Spec.Version,
					MasterNum:         clusterManager.Spec.MasterNum,
					WorkerNum:         clusterManager.Spec.WorkerNum,
				},
			); err != nil {
				log.Error(err, "Failed to marshal cluster parameters")
			}

			switch strings.ToUpper(clusterManager.Spec.Provider) {
			case util.ProviderAws:
				if providerJson, err = json.Marshal(
					&AwsParameter{
						SshKey:     clusterManager.AwsSpec.SshKey,
						Region:     clusterManager.AwsSpec.Region,
						MasterType: clusterManager.AwsSpec.MasterType,
						WorkerType: clusterManager.AwsSpec.WorkerType,
					},
				); err != nil {
					log.Error(err, "Failed to marshal cluster parameters")
					return ctrl.Result{}, err
				}
			case util.ProviderVsphere:
				if providerJson, err = json.Marshal(
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
				); err != nil {
					log.Error(err, "Failed to marshal cluster parameters")
					return ctrl.Result{}, err
				}
			}
			clusterJson = util.MergeJson(clusterJson, providerJson)

			generatedSuffix := util.CreateSuffixString()
			newServiceInstance := &servicecatalogv1beta1.ServiceInstance{
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

			ctrl.SetControllerReference(clusterManager, newServiceInstance, r.Scheme)
			err = r.Create(context.TODO(), newServiceInstance)
			if err != nil {
				log.Error(err, "Failed to create "+clusterManager.Name+" serviceInstance")
				return ctrl.Result{}, err
			}
			clusterManager.Annotations[util.AnnotationKeyClmSuffix] = generatedSuffix
		} else {
			log.Error(err, "Failed to get serviceInstance")
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *ClusterManagerReconciler) kubeadmControlPlaneUpdate(ctx context.Context, clusterManager *clusterv1alpha1.ClusterManager) (ctrl.Result, error) {
	log := r.Log.WithValues("clustermanager", types.NamespacedName{Name: clusterManager.Name, Namespace: clusterManager.Namespace})

	kcp := &controlplanev1.KubeadmControlPlane{}
	key := types.NamespacedName{
		Name:      clusterManager.Name + "-control-plane",
		Namespace: clusterManager.Namespace,
	}

	if err := r.Get(context.TODO(), key, kcp); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		} else {
			log.Error(err, "Failed to get clusterRole")
			return ctrl.Result{}, err
		}
	}

	//create helper for patch
	helper, _ := patch.NewHelper(kcp, r.Client)
	defer func() {
		if err := helper.Patch(context.TODO(), kcp); err != nil {
			r.Log.Error(err, "kubeadmcontrolplane patch error")
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
	log := r.Log.WithValues("clustermanager", types.NamespacedName{Name: clusterManager.Name, Namespace: clusterManager.Namespace})

	md := &capiv1.MachineDeployment{}
	key := types.NamespacedName{
		Name:      clusterManager.Name + "-md-0",
		Namespace: clusterManager.Namespace,
	}

	if err := r.Get(context.TODO(), key, md); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		} else {
			log.Error(err, "Failed to get clusterRole")
			return ctrl.Result{}, err
		}
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
