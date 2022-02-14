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

package util

const (
	KubeNamespace                      = "kube-system"
	WatchAnnotationJoinValue           = "join"
	WatchAnnotationJoinSuccess         = "complete"
	WatchAnnotationUnJoinValue         = "unjoin"
	KubeconfigSuffix                   = "-kubeconfig"
	ClusterNamespace                   = "default"
	MonitoringNamespace                = "monitoring"
	HYPERCLOUD_SYSTEM_NAMESPACE        = "hypercloud5-system"
	MultiApiServerNamespace            = "hypercloud4-multi-system"
	MultiApiServerServiceName          = "hypercloud4-multi-api-server-service"
	SecretFinalizer                    = "secret/finalizers"
	ClusterManagerFinalizer            = "clusterManager.cluster.tmax.io"
	SecretFinalizerForClusterManager   = "secretforclustermanager/finalizers"
	MultiApiServerServiceSelectorKey   = "hypercloud4"
	MultiApiServerServiceSelectorValue = "multi-api-server"
	ClusterTypeCreated                 = "created"
	ClusterTypeRegistered              = "registered"
	IngressNginxNamespace              = "ingress-nginx"
	IngressNginxName                   = "ingress-nginx-controller"
	ClusterApiKind                     = "clustermanagers"
	ClusterApiGroupVersion             = "cluster.tmax.io/v1alpha1"
	AgentIngressName                   = "hypercloud-ingress"
	SuffixDigit                        = 5
	HypercloudIngressClass             = "tmax-cloud"
	HypercloudMultiIngressClass        = "multicluster"
	HypercloudMultiIngressSubdomain    = "multicluster"

	ProviderAws     = "AWS"
	ProviderVsphere = "VSPHERE"
	ProviderUnknown = "Unknown"

	ProviderAwsLogo     = "AWS"
	ProviderVsphereLogo = "vSphere"

	ArgoApiGroup           = "argocd.argoproj.io"
	ArgoNamespace          = "argocd"
	ArgoServiceAccount     = "argocd-manager"
	ArgoClusterRole        = "argocd-manager-role"
	ArgoClusterRoleBinding = "argocd-manager-role-binding"
	ArgoSecretTypeCluster  = "cluster"

	AnnotationKeyOwner                  = "owner"
	AnnotationKeyCreator                = "creator"
	AnnotationKeyArgoClusterSecret      = "argocd.argoproj.io/cluster.secret"
	AnnotationKeyArgoManagedBy          = "managed-by"
	AnnotationKeyTraefikServerTransport = "traefik.ingress.kubernetes.io/service.serverstransport"
	AnnotationKeyTraefikEntrypoints     = "traefik.ingress.kubernetes.io/router.entrypoints"
	AnnotationKeyTraefikMiddlewares     = "traefik.ingress.kubernetes.io/router.middlewares"
	AnnotationKeyClmEndpoint            = "clustermanager.tmax.io/endpoint"
	AnnotationKeyClmSuffix              = "clustermanager.tmax.io/suffix"
	AnnotationKeyClmDns                 = "clustermanager.tmax.io/dns"

	LabelKeyHypercloudIngress = "ingress.tmaxcloud.org/name"
	LabelKeyClmRef            = "clustermanager.tmax.io/clm.ref"
	LabelKeyClmParent         = "parent"
	LabelKeyClmClusterType    = "type"
	LabelKeyArgoSecretType    = "argocd.argoproj.io/secret-type"
)
