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
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var ClusterUpdateClaimWebhookLogger = logf.Log.WithName("clusterupdateclaim-resource")

func (r *ClusterUpdateClaim) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update;delete,path=/validate-claim-tmax-io-v1alpha1-clusterupdateclaim,mutating=false,failurePolicy=fail,groups=claim.tmax.io,resources=clusterupdateclaims;clusterupdateclaims/status,versions=v1alpha1,name=validation.webhook.clusterupdateclaim,admissionReviewVersions=v1beta1;v1,sideEffects=NoneOnDryRun

var _ webhook.Validator = &ClusterUpdateClaim{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ClusterUpdateClaim) ValidateCreate() error {

	masterNum := r.Spec.UpdatedMasterNum

	// masterNum이 짝수면 안됨
	if masterNum != 0 && masterNum%2 == 0 {
		return fmt.Errorf("r.Spec.UpdatedMasterNum cannot be even")
	}

	ClusterUpdateClaimWebhookLogger.Info("validate create", "name", r.Name)
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *ClusterUpdateClaim) ValidateUpdate(old runtime.Object) error {
	// oc := old.(*ClusterUpdateClaim).DeepCopy()

	// masterNum을 짝수로 변경하는 경우
	masterNum := r.Spec.UpdatedMasterNum
	if masterNum != 0 && masterNum%2 == 0 {
		return fmt.Errorf("r.Spec.UpdatedMasterNum cannot be even")
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *ClusterUpdateClaim) ValidateDelete() error {
	ClusterUpdateClaimWebhookLogger.Info("validate delete", "name", r.Name)

	return nil
}
