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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type ClusterClaimPhase string

const (
	// 클러스터 클레임이 생성되고, 관리자의 승인/거절을 기다리는 상태
	ClusterClaimPhaseAwaiting = ClusterClaimPhase("Awaiting")
	// 관리자에 의해 클레임이 승인된 상태
	ClusterClaimPhaseApproved = ClusterClaimPhase("Approved")
	// 관리자에 의헤 클레임이 거절된 상태
	ClusterClaimPhaseRejected = ClusterClaimPhase("Rejected")
	// 클러스터가 삭제된 상태
	ClusterClaimPhaseClusterDeleted = ClusterClaimPhase("Cluster Deleted")
	// 클러스터 생성과정에서 에러가 발생한 상태
	ClusterClaimPhaseError = ClusterClaimPhase("Error")
)

const (
	ClusterClaimDeprecatedPhaseClusterDeleted = ClusterClaimPhase("ClusterDeleted")
)

// ClusterClaimSpec defines the desired state of ClusterClaim
type ClusterClaimSpec struct {
	// +kubebuilder:validation:Required
	// The name of the cluster to be created.
	ClusterName string `json:"clusterName"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=^v[0-9].[0-9]+.[0-9]+
	// The version of kubernetes. Example: v1.19.6
	Version string `json:"version"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum:=AWS;vSphere
	// The type of provider.
	Provider string `json:"provider"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum:=1
	// The number of master node. Example: 3
	MasterNum int `json:"masterNum"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum:=1
	// The number of worker node. Example: 2
	WorkerNum int `json:"workerNum"`
	// Provider Aws Spec.
	ProviderAwsSpec AwsClaimSpec `json:"providerAwsSpec,omitempty"`
	// Provider vSphere Spec.
	ProviderVsphereSpec VsphereClaimSpec `json:"providerVsphereSpec,omitempty"`
}

type AwsClaimSpec struct {
	// The ssh key info to access VM.
	SshKey string `json:"sshKey,omitempty"`
	// +kubebuilder:validation:Enum:=ap-northeast-1;ap-northeast-2;ap-south-1;ap-southeast-1;ap-northeast-2;ca-central-1;eu-central-1;eu-west-1;eu-west-2;eu-west-3;sa-east-1;us-east-1;us-east-2;us-west-1;us-west-2
	// The region where VM is working. Defaults to ap-northeast-2.
	Region string `json:"region,omitempty"`
	// The type of VM for master node. Defaults to t3.medium. See: https://aws.amazon.com/ec2/instance-types
	MasterType string `json:"masterType,omitempty"`
	// +kubebuilder:validation:Minimum:=8
	// The disk size of VM for master node. Defaults to 20.
	MasterDiskSize int `json:"masterDiskSize,omitempty"`
	// The type of VM for worker node. Defaults to t3.medium. See: https://aws.amazon.com/ec2/instance-types
	WorkerType string `json:"workerType,omitempty"`
	// +kubebuilder:validation:Minimum:=8
	// The disk size of VM for worker node. Defaults to 20.
	WorkerDiskSize int `json:"workerDiskSize,omitempty"`
}

type VsphereClaimSpec struct {
	// The internal IP address cidr block for pods. Defaults to 10.0.0.0/16.
	// +kubebuilder:validation:Pattern:=^[0-9]+.[0-9]+.[0-9]+.[0-9]+\/[0-9]+
	PodCidr string `json:"podCidr,omitempty"`
	// The IP address of vCenter Server Application(VCSA).
	VcenterIp string `json:"vcenterIp,omitempty"`
	// The TLS thumbprint of machine certificate. Example: F881E17883D123700CAE0B14F7DA75DE8F3287D1
	VcenterThumbprint string `json:"vcenterThumbprint,omitempty"`
	// The name of network. Defaults to VM Network.
	VcenterNetwork string `json:"vcenterNetwork,omitempty"`
	// The name of datacenter.
	VcenterDataCenter string `json:"vcenterDataCenter,omitempty"`
	// The name of datastore.
	VcenterDataStore string `json:"vcenterDataStore,omitempty"`
	// The name of folder. Defaults to vm.
	VcenterFolder string `json:"vcenterFolder,omitempty"`
	// The name of resource pool. Example: 192.168.9.30/Resources
	VcenterResourcePool string `json:"vcenterResourcePool,omitempty"`
	// The IP address of control plane for remote cluster(vip).
	VcenterKcpIp string `json:"vcenterKcpIp,omitempty"`
	// +kubebuilder:validation:Minimum:=2
	// The number of cpus for vm. Defaults to 2.
	VcenterCpuNum int `json:"vcenterCpuNum,omitempty"`
	// +kubebuilder:validation:Minimum:=2048
	// The memory size for vm, write as MB without unit. Defaults to 4096.
	VcenterMemSize int `json:"vcenterMemSize,omitempty"`
	// +kubebuilder:validation:Minimum:=20
	// The disk size for vm, write as GB without unit. Defaults to 20.
	VcenterDiskSize int `json:"vcenterDiskSize,omitempty"`
	// The template name to use in vsphere.
	VcenterTemplate string `json:"vcenterTemplate,omitempty"`
	// The root user password for virtual machine. Defaults to random.
	VMPassword string `json:"vmPassword,omitempty"`
}

// ClusterClaimStatus defines the observed state of ClusterClaim
type ClusterClaimStatus struct {
	Message string `json:"message,omitempty" protobuf:"bytes,2,opt,name=message"`
	Reason  string `json:"reason,omitempty" protobuf:"bytes,3,opt,name=reason"`

	// +kubebuilder:validation:Enum=Awaiting;Admitted;Approved;Rejected;Error;ClusterDeleted;Cluster Deleted;
	Phase ClusterClaimPhase `json:"phase,omitempty" protobuf:"bytes,4,opt,name=phase"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=clusterclaims,shortName=cc,scope=Namespaced
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.reason`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// ClusterClaim is the Schema for the clusterclaims API
type ClusterClaim struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterClaimSpec   `json:"spec"`
	Status ClusterClaimStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// ClusterClaimList contains a list of ClusterClaim
type ClusterClaimList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterClaim `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterClaim{}, &ClusterClaimList{})
}

func (c *ClusterClaimStatus) SetTypedPhase(p ClusterClaimPhase) {
	c.Phase = p
}

func (c *ClusterClaimStatus) SetReason(r string) {
	c.Reason = r
}

func (c *ClusterClaim) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Name:      c.Name,
		Namespace: c.Namespace,
	}
}

func (c *ClusterClaim) GetClusterManagerNamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Name:      c.Spec.ClusterName,
		Namespace: c.Namespace,
	}
}
