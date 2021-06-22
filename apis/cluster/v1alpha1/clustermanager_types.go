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

// type NodeInfo struct {
// 	Name      string         `json:"name,omitempty"`
// 	Ip        string         `json:"ip,omitempty"`
// 	IsMaster  bool           `json:"isMaster,omitempty"`
// 	Resources []ResourceType `json:"resources,omitempty"`
// }

type ResourceType struct {
	Type     string `json:"type,omitempty"`
	Capacity string `json:"capacity,omitempty"`
	Usage    string `json:"usage,omitempty"`
}

// ClusterManagerSpec defines the desired state of ClusterManager
type ClusterManagerSpec struct {
	// The name of cloud provider where VM is created
	Provider string `json:"provider,omitempty"`
	// The region where VM is working
	Region string `json:"region,omitempty"`
	// The version of kubernetes
	Version string `json:"version,omitempty"`
	// The number of master node
	MasterNum int `json:"masterNum,omitempty"`
	// The type of VM for master node
	MasterType string `json:"masterType,omitempty"`
	// The number of worker node
	WorkerNum int `json:"workerNum,omitempty"`
	// The type of VM for worker node
	WorkerType string `json:"workerType,omitempty"`
	// The ssh key info to access VM
	SshKey string `json:"sshKey,omitempty"`
}

// ClusterManagerStatus defines the observed state of ClusterManager
type ClusterManagerStatus struct {
	Provider             string                  `json:"provider,omitempty"`
	Version              string                  `json:"version,omitempty"`
	Ready                bool                    `json:"ready,omitempty"`
	MasterRun            int                     `json:"masterRun,omitempty"`
	WorkerRun            int                     `json:"workerRun,omitempty"`
	NodeInfo             []corev1.NodeSystemInfo `json:"nodeInfo,omitempty"`
	Phase                string                  `json:"phase,omitempty"`
	ControlPlaneEndpoint string                  `json:"controlPlaneEndpoint,omitempty"`
	// ObservedGeneration is the latest generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}
type ClusterManagerPhase string

const (
	// ClusterManagerPhasePending is the first state a Cluster is assigned by
	// Cluster API Cluster controller after being created.
	ClusterManagerPhasePending = ClusterManagerPhase("Pending")

	// ClusterManagerPhaseProvisioning is the state when the Cluster has a provider infrastructure
	// object associated and can start provisioning.
	ClusterManagerPhaseProvisioning = ClusterManagerPhase("Provisioning")

	// object associated and can start provisioning.
	ClusterManagerPhaseRegistering = ClusterManagerPhase("Registering")

	// ClusterManagerPhaseProvisioned is the state when its
	// infrastructure has been created and configured.
	ClusterManagerPhaseProvisioned = ClusterManagerPhase("Provisioned")

	// infrastructure has been created and configured.
	ClusterManagerPhaseRegistered = ClusterManagerPhase("Registered")

	// ClusterManagerPhaseDeleting is the Cluster state when a delete
	// request has been sent to the API Server,
	// but its infrastructure has not yet been fully deleted.
	ClusterManagerPhaseDeleting = ClusterManagerPhase("Deleting")

	// ClusterManagerPhaseFailed is the Cluster state when the system
	// might require user intervention.
	ClusterManagerPhaseFailed = ClusterManagerPhase("Failed")

	// ClusterManagerPhaseUnknown is returned if the Cluster state cannot be determined.
	ClusterManagerPhaseUnknown = ClusterManagerPhase("Unknown")
)

func (c *ClusterManagerStatus) SetTypedPhase(p ClusterManagerPhase) {
	c.Phase = string(p)
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=clustermanagers,scope=Namespaced,shortName=clm
// +kubebuilder:printcolumn:name="Provider",type="string",JSONPath=".spec.provider",description="provider"
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.version",description="k8s version"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="is running"
// +kubebuilder:printcolumn:name="MasterNum",type="string",JSONPath=".spec.masterNum",description="replica number of master"
// +kubebuilder:printcolumn:name="MasterRun",type="string",JSONPath=".status.masterRun",description="running of master"
// +kubebuilder:printcolumn:name="WorkerNum",type="string",JSONPath=".spec.workerNum",description="replica number of worker"
// +kubebuilder:printcolumn:name="WorkerRun",type="string",JSONPath=".status.workerRun",description="running of worker"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="cluster status phase"

// ClusterManager is the Schema for the clustermanagers API
type ClusterManager struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterManagerSpec   `json:"spec,omitempty"`
	Status ClusterManagerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterManagerList contains a list of ClusterManager
type ClusterManagerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterManager `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterManager{}, &ClusterManagerList{})
}
