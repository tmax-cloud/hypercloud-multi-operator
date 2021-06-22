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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ClusterRegistrationSpec defines the desired state of ClusterRegistration
type ClusterRegistrationSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The name of the cluster to be created
	ClusterName string `json:"clusterName"`
	// Foo is an example field of ClusterRegistration. Edit ClusterRegistration_types.go to remove/update
	KubeConfig string `json:"kubeConfig,omitempty"`
	// WithPrometheus string `json:"withPrometheus,omitempty"`
}

// ClusterRegistrationStatus defines the observed state of ClusterRegistration
type ClusterRegistrationStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Provider  string                  `json:"provider,omitempty"`
	Version   string                  `json:"version,omitempty"`
	Ready     bool                    `json:"ready,omitempty"`
	MasterNum int                     `json:"masterNum,omitempty"`
	MasterRun int                     `json:"masterRun,omitempty"`
	WorkerNum int                     `json:"workerNum,omitempty"`
	WorkerRun int                     `json:"workerRun,omitempty"`
	NodeInfo  []corev1.NodeSystemInfo `json:"nodeInfo,omitempty"`
	Phase     string                  `json:"phase,omitempty"`
	Reason    string                  `json:"reason,omitempty"`
}

type ClusterRegistrationPhase string

const (
	// ClusterRegistrationPhasePending is the first state a Cluster is assigned by
	// Cluster API Cluster controller after being created.
	ClusterRegistrationPhasePending = ClusterRegistrationPhase("Pending")

	// ClusterRegistrationPhaseProvisioning is the state when the Cluster has a provider infrastructure
	// object associated and can start provisioning.
	ClusterRegistrationPhaseProvisioning = ClusterRegistrationPhase("Provisioning")

	ClusterRegistrationPhaseSuccess = ClusterRegistrationPhase("Success")

	// ClusterRegistrationPhaseProvisioned is the state when its
	// infrastructure has been created and configured.
	ClusterRegistrationPhaseProvisioned = ClusterRegistrationPhase("Provisioned")

	// ClusterRegistrationPhaseDeleting is the Cluster state when a delete
	// request has been sent to the API Server,
	// but its infrastructure has not yet been fully deleted.
	ClusterRegistrationPhaseDeleting = ClusterRegistrationPhase("Deleting")

	// ClusterRegistrationPhaseFailed is the Cluster state when the system
	// might require user intervention.
	ClusterRegistrationPhaseFailed = ClusterRegistrationPhase("Failed")

	// ClusterRegistrationPhaseUnknown is returned if the Cluster state cannot be determined.
	ClusterRegistrationPhaseUnknown = ClusterRegistrationPhase("Unknown")
)

func (c *ClusterRegistrationStatus) SetTypedPhase(p ClusterRegistrationPhase) {
	c.Phase = string(p)
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=clusterregistrations,scope=Namespaced,shortName=clr
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="cluster status phase"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.reason",description="cluster status phase"

// ClusterRegistration is the Schema for the clusterregistrations API
type ClusterRegistration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterRegistrationSpec   `json:"spec,omitempty"`
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
