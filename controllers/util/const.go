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
	KUBE_NAMESPACE                     = "kube-system"
	WatchAnnotationJoinValue           = "join"
	WatchAnnotationJoinSuccess         = "complete"
	WatchAnnotationUnJoinValue         = "unjoin"
	KubeconfigSuffix                   = "-kubeconfig"
	ClusterNamespace                   = "default"
	MonitoringNamespace                = "monitoring"
	HypercloudNamespace                = "hypercloud5-system"
	HostClusterName                    = "hostcluster"
	MultiApiServerNamespace            = "hypercloud4-multi-system"
	MultiApiServerServiceName          = "hypercloud4-multi-api-server-service"
	SecretFinalizer                    = "secret/finalizers"
	ClusterManagerFinalizer            = "clusterManager.cluster.tmax.io"
	SecretFinalizerForClusterManager   = "secretforclustermanager/finalizers"
	MultiApiServerServiceSelectorKey   = "hypercloud4"
	MultiApiServerServiceSelectorValue = "multi-api-server"
	ClusterOwnerKey                    = "owner"
	ClusterTypeKey                     = "type"
	ClusterTypeCreated                 = "created"
	ClusterTypeRegistered              = "registered"
	ReversePorxyObjectName             = "reverse-proxy-configuration"
	ReversePorxyObjectNamespace        = "console-system"
	IngressNginxNamespace              = "ingress-nginx"
	IngressNginxName                   = "ingress-nginx-controller"
	CLUSTER_API_Kind                   = "clustermanagers"
	CLUSTER_API_GROUP_VERSION          = "cluster.tmax.io/v1alpha1"
	AGENT_INGRESS_NAME                 = "hypercloud-ingress"
	SUFFIX_DIGIT                       = 5
	INGRESS_CLASS                      = "tmax-cloud"
	PROVIDER_AWS                       = "AWS"
	PROVIDER_VSPHERE                   = "VSPHERE"
	PROVIDER_UNKNOWN                   = "Unkown"
	ARGOCD_NAMESPACE                   = "argocd"
	ARGOCD_MANAGER                     = "argocd-manager"
	ARGOCD_MANAGER_ROLE                = "argocd-manager-role"
	ARGOCD_MANAGER_ROLE_BINDING        = "argocd-manager-role-binding"
	//PROVIDER_REGISTER                  = "NONE"
)
