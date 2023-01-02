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
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// ClusterRegistrationSpec defines the desired state of ClusterRegistration
type ClusterRegistrationSpec struct {
	// +kubebuilder:validation:Required
	// The name of the cluster to be registered
	ClusterName string `json:"clusterName"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Format:="data-url"
	// The kubeconfig file of the cluster to be registered
	KubeConfig string `json:"kubeConfig"`
	// WithPrometheus string `json:"withPrometheus,omitempty"`
}

// ClusterRegistrationStatus defines the observed state of ClusterRegistration
type ClusterRegistrationStatus struct {
	Provider         string                    `json:"provider,omitempty"`
	Version          string                    `json:"version,omitempty"`
	Ready            bool                      `json:"ready,omitempty"`
	MasterNum        int                       `json:"masterNum,omitempty"`
	MasterRun        int                       `json:"masterRun,omitempty"`
	WorkerNum        int                       `json:"workerNum,omitempty"`
	WorkerRun        int                       `json:"workerRun,omitempty"`
	NodeInfo         []coreV1.NodeSystemInfo   `json:"nodeInfo,omitempty"`
	Phase            ClusterRegistrationPhase  `json:"phase,omitempty"`
	Reason           ClusterRegistrationReason `json:"reason,omitempty"`
	ClusterValidated bool                      `json:"clusterValidated,omitempty"`
	SecretReady      bool                      `json:"secretReady,omitempty"`
}

type ClusterRegistrationPhase string

type ClusterRegistrationReason string

const (
	// 클러스터가 등록된 상태
	ClusterRegistrationPhaseRegistered = ClusterRegistrationPhase("Registered")
	// 에러가 발생하여 클러스터가 등록되지 못한 상태
	// 세가지 종류가 존재함
	// 1. kubeconfig 파일이 invalid한 경우(yaml파일이 아니거나 yaml형식이 맞지 않는 등)
	// 2. cluster가 invalid한 경우(network call이 가지 않는 경우)
	// 3. 동일한 이름의 클러스터가 이미 존재하는 경우
	ClusterRegistrationPhaseError = ClusterRegistrationPhase("Error")
	// 클러스터가 삭제된 상태
	ClusterRegistrationPhaseClusterDeleted = ClusterRegistrationPhase("Cluster Deleted")
)

const (
	ClusterRegistrationDeprecatedPhaseSuccess = ClusterRegistrationPhase("Success")
	ClusterRegistrationDeprecatedPhaseDeleted = ClusterRegistrationPhase("Deleted")
)

const (
	// // ClusterRegistrationPhaseSuccess is the state when cluster is registered successfully
	// ClusterRegistrationPhaseSuccess = ClusterRegistrationPhase("Success")

	// // ClusterRegistrationPhaseFailed is the when failed to register cluster
	// // Cluster registration failure can occur in following cases
	// // 1. kubeconfig is invalid
	// // 2. cluster is invalid
	// ClusterRegistrationPhaseFailed = ClusterRegistrationPhase("Failed")

	// // ClusterRegistrationPhaseSecretCreated
	// ClusterRegistrationPhaseSecretCreated = ClusterRegistrationPhase("SecretCreated")

	// // ClusterRegistrationPhaseValidated
	// ClusterRegistrationPhaseValidated = ClusterRegistrationPhase("Validated")

	// // ClusterRegistrationPhaseDeleting is the Cluster state when a delete
	// // request has been sent to the API Server,
	// // but its infrastructure has not yet been fully deleted.
	// ClusterRegistrationPhaseDeleting = ClusterRegistrationPhase("Deleting")

	// // ClusterRegistrationPhaseUnknown is returned if the Cluster state cannot be determined.
	// ClusterRegistrationPhaseUnknown = ClusterRegistrationPhase("Unknown")

	// ClusterRegistrationReasonClusterNotFound is returned if the Cluster not found
	ClusterRegistrationReasonClusterNotFound = ClusterRegistrationReason("ClusterNotFound")

	// ClusterRegistrationReasonClusterNotFound is returned if the Input Kubeconfig is invalid
	ClusterRegistrationReasonInvalidKubeconfig = ClusterRegistrationReason("InvalidKubeconfig")

	// ClusterRegistrationReasonClusterNameDuplicated is returned if the cluster name is duplicated
	ClusterRegistrationReasonClusterNameDuplicated = ClusterRegistrationReason("ClusterNameDuplicated")
)

func (c *ClusterRegistrationStatus) SetTypedPhase(p ClusterRegistrationPhase) {
	c.Phase = p
}

func (c *ClusterRegistrationStatus) SetTypedReason(p ClusterRegistrationReason) {
	c.Reason = p
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=clusterregistrations,scope=Namespaced,shortName=clr
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="cluster status phase"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.reason",description="cluster status reason"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// ClusterRegistration is the Schema for the clusterregistrations API
type ClusterRegistration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterRegistrationSpec   `json:"spec"`
	Status ClusterRegistrationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// ClusterRegistrationList contains a list of ClusterRegistration
type ClusterRegistrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterRegistration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterRegistration{}, &ClusterRegistrationList{})
}

func (c *ClusterRegistration) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Name:      c.Name,
		Namespace: c.Namespace,
	}
}

func (c *ClusterRegistration) GetCluterManagerNamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Name:      c.Spec.ClusterName,
		Namespace: c.Namespace,
	}
}
