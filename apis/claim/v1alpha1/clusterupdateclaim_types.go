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

type ClusterUpdateClaimPhase string

const (
	// 클러스터 업데이트 클레임이 생성되고, 관리자의 승인/거절을 기다리는 상태
	ClusterUpdateClaimPhaseAwaiting = ClusterUpdateClaimPhase("Awaiting")
	// 관리자에 의해 클레임이 승인된 상태
	ClusterUpdateClaimPhaseApproved = ClusterUpdateClaimPhase("Approved")
	// 관리자에 의해 클레임이 거절된 상태
	ClusterUpdateClaimPhaseRejected = ClusterUpdateClaimPhase("Rejected")
	// 클러스터 업데이트 과정에서 에러가 발생한 상태
	ClusterUpdateClaimPhaseError = ClusterUpdateClaimPhase("Error")
)

type ClusterUpdateClaimReason string

const (
	ClusterUpdateClaimReasonClusterNotFound   = ClusterUpdateClaimReason("Cluster not found")
	ClusterUpdateClaimReasonClusterIsDeleting = ClusterUpdateClaimReason("Cluster is deleting")
	ClusterUpdateClaimReasonAdminApproved     = ClusterUpdateClaimReason("Admin approved")
	ClusterUpdateClaimReasonAdminAwaiting     = ClusterUpdateClaimReason("Waiting for admin approval")
	ClusterUpdateClaimReasonConcurruencyError = ClusterUpdateClaimReason("The number of nodes at the time of creation of the clusterupdataclaim differs from the current number of nodes.")
	ClusterUpdateClaimReasonInvalidCluster    = ClusterUpdateClaimReason("Cluster type is not created type")
)

type ClusterUpdateType string

const (
	ClusterUpdateTypeNodeScale = ClusterUpdateType("NodeScale")
)

// ClusterUpdateClaimSpec defines the desired state of ClusterUpdateClaim
type ClusterUpdateClaimSpec struct {
	// +kubebuilder:validation:Required
	// Cluster name created using clusterclaim.
	ClusterName string `json:"clusterName"`
	// +kubebuilder:validation:Minimum:=1
	// The number of master nodes to update.
	UpdatedMasterNum int `json:"updatedMasterNum,omitempty"`
	// +kubebuilder:validation:Minimum:=1
	// The number of worker nodes to update.
	UpdatedWorkerNum int `json:"updatedWorkerNum,omitempty"`
}

// ClusterUpdateClaimStatus defines the observed state of ClusterUpdateClaim
type ClusterUpdateClaimStatus struct {
	// Reason of the phase.
	Reason ClusterUpdateClaimReason `json:"reason,omitempty" protobuf:"bytes,3,opt,name=reason"`

	// +kubebuilder:validation:Enum=Awaiting;Approved;Rejected;Error;Cluster Deleted;
	// Phase of the clusterupdateclaim.
	Phase ClusterUpdateClaimPhase `json:"phase,omitempty" protobuf:"bytes,4,opt,name=phase"`

	// The number of current master node.
	CurrentMasterNum int `json:"currentMasterNum,omitempty"`
	// The number of current worker node.
	CurrentWorkerNum int `json:"currentWorkerNum,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=clusterupdateclaims,shortName=cuc,scope=Namespaced
// +kubebuilder:printcolumn:name="Cluster",type=string,JSONPath=`.spec.clusterName`
// +kubebuilder:printcolumn:name="masternum",type=integer,JSONPath=`.spec.updatedMasterNum`
// +kubebuilder:printcolumn:name="workernum",type=integer,JSONPath=`.spec.updatedWorkerNum`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.reason`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// ClusterUpdateClaim is the Schema for the clusterupdateclaims API
type ClusterUpdateClaim struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterUpdateClaimSpec   `json:"spec,omitempty"`
	Status ClusterUpdateClaimStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ClusterUpdateClaimList contains a list of ClusterUpdateClaim
type ClusterUpdateClaimList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterUpdateClaim `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterUpdateClaim{}, &ClusterUpdateClaimList{})
}

func (c *ClusterUpdateClaimStatus) SetTypedPhase(p ClusterUpdateClaimPhase) {
	c.Phase = p
}

func (c *ClusterUpdateClaimStatus) SetTypedReason(r ClusterUpdateClaimReason) {
	c.Reason = r
}

func (c *ClusterUpdateClaim) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Name:      c.Name,
		Namespace: c.Namespace,
	}
}

func (c *ClusterUpdateClaim) GetClusterNamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Name:      c.Spec.ClusterName,
		Namespace: c.Namespace,
	}
}

func (c *ClusterUpdateClaim) IsPhaseError() bool {
	if c.Status.Phase == ClusterUpdateClaimPhaseError {
		return true
	}
	return false
}

func (c *ClusterUpdateClaim) IsPhaseApproved() bool {
	if c.Status.Phase == ClusterUpdateClaimPhaseApproved {
		return true
	}
	return false
}

func (c *ClusterUpdateClaim) IsPhaseRejected() bool {
	if c.Status.Phase == ClusterUpdateClaimPhaseRejected {
		return true
	}
	return false
}

func (c *ClusterUpdateClaim) IsPhaseAwaiting() bool {
	if c.Status.Phase == ClusterUpdateClaimPhaseAwaiting {
		return true
	}
	return false
}

func (c *ClusterUpdateClaim) IsPhaseEmpty() bool {
	if c.Status.Phase == "" {
		return true
	}
	return false
}
