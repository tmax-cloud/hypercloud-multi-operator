package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DEPRECATED - This group version of ClusterConfiguration is deprecated by apis/kubeadm/v1beta2/ClusterConfiguration.
// ClusterConfiguration contains cluster-wide configuration for a kubeadm cluster
type ClusterConfiguration struct {
	metav1.TypeMeta `json:",inline"`

	// KubernetesVersion is the target version of the control plane.
	KubernetesVersion string `json:"kubernetesVersion"`
}
