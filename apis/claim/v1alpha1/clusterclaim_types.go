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
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ClusterClaimSpec defines the desired state of ClusterClaim
type ClusterClaimSpec struct {
	// +kubebuilder:validation:Required
	// The name of cloud provider where VM is created
	Provider string `json:"provider"`
	// +kubebuilder:validation:Required
	// The region where VM is working
	Region string `json:"region"`
	// +kubebuilder:validation:Required
	// The name of the cluster to be created
	ClusterName string `json:"clusterName"`
	// +kubebuilder:validation:Required
	// The version of kubernetes
	Version string `json:"version"`
	// +kubebuilder:validation:Required
	// The number of master node
	MasterNum int `json:"masterNum"`
	// +kubebuilder:validation:Required
	// The type of VM for master node
	MasterType string `json:"masterType"`
	// +kubebuilder:validation:Required
	// The number of worker node
	WorkerNum int `json:"workerNum"`
	// +kubebuilder:validation:Required
	// The type of VM for worker node
	WorkerType string `json:"workerType"`
	// +kubebuilder:validation:Required
	// The ssh key info to access VM
	SshKey string `json:"sshKey"`
	// +kubebuilder:validation:Required
	// The internal IP address cider block for pods
	PodCidr string `json:"podCidr,omitempty"`
	// +kubebuilder:validation:Required
	// The IP address of vCenter Server Application(VCSA)
	VcenterIp string `json:"vcenterIp,omitempty"`
	// +kubebuilder:validation:Required
	// The user id of VCSA
	VcenterId string `json:"vcenterId,omitempty"`
	// +kubebuilder:validation:Required
	// The password of VCSA
	VcenterPwd string `json:"vcenterPwd,omitempty"`
	// +kubebuilder:validation:Required
	// The TLS thumbprint of machine certificate
	VcenterThumbprint string `json:"vcenterThumbprint,omitempty"`
	// +kubebuilder:validation:Required
	// The name of network
	VcenterNetwork string `json:"vcenterNetwork,omitempty"`
	// +kubebuilder:validation:Required
	// The name of data center
	VcenterDataCenter string `json:"vcenterDataCenter,omitempty"`
	// +kubebuilder:validation:Required
	// The name of data store
	VcenterDataStore string `json:"vcenterDataStore,omitempty"`
	// +kubebuilder:validation:Required
	// The name of folder
	VcenterFolder string `json:"vcenterFolder,omitempty"`
	// +kubebuilder:validation:Required
	// The name of resource pool
	VcenterResourcePool string `json:"vcenterResourcePool,omitempty"`
	// +kubebuilder:validation:Required
	// The IP address of control plane for remote cluster(vip)
	VcenterKcpIp string `json:"vcenterKcpIp,omitempty"`
}

// ClusterClaimStatus defines the observed state of ClusterClaim
type ClusterClaimStatus struct {
	Message string `json:"message,omitempty" protobuf:"bytes,2,opt,name=message"`
	Reason  string `json:"reason,omitempty" protobuf:"bytes,3,opt,name=reason"`

	// +kubebuilder:validation:Enum=Awaiting;Admitted;Approved;Rejected;Error;ClusterDeleted;
	Phase string `json:"phase,omitempty" protobuf:"bytes,4,opt,name=phase"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=cc,scope=Namespaced
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.reason`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// ClusterClaim is the Schema for the clusterclaims API
type ClusterClaim struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterClaimSpec   `json:"spec,omitempty"`
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
