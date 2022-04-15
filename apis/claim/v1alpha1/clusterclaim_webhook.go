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
	"errors"
	"reflect"
	"regexp"

	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var clusterclaimlog = logf.Log.WithName("clusterclaim-resource")

func (r *ClusterClaim) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-claim-tmax-io-v1alpha1-clusterclaim,mutating=true,failurePolicy=fail,groups=claim.tmax.io,resources=clusterclaims,verbs=update,versions=v1alpha1,name=mutation.webhook.clusterclaim

var _ webhook.Defaulter = &ClusterClaim{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *ClusterClaim) Default() {
	clusterclaimlog.Info("default", "name", r.Name)
	// if len(r.Name) > maxGeneratedNameLength {
	// r.Name = r.Name[:maxGeneratedNameLength]
	// }
	// return fmt.Sprintf("%s%s", base, utilrand.String(randomLength))

	// r.Name = r.Name + "-" + utilrand.String(randomLength)
	// r.GenerateName = r.GenerateName + "-"
	// utilrand.String(randomLength)
	// r.Name = r.Name + r.Annotations["creator"]
	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// +kubebuilder:webhook:verbs=update;delete,path=/validate-claim-tmax-io-v1alpha1-clusterclaim,mutating=false,failurePolicy=fail,groups=claim.tmax.io,resources=clusterclaims;clusterclaims/status,versions=v1alpha1,name=validation.webhook.clusterclaim

var _ webhook.Validator = &ClusterClaim{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ClusterClaim) ValidateCreate() error {
	clusterclaimlog.Info("validate create", "name", r.Name)

	reg, _ := regexp.Compile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)
	if !reg.MatchString(r.Spec.ClusterName) {
		//return errors.NewInvalid()
		errList := []*field.Error{
			{
				Type:     field.ErrorTypeInvalid,
				Field:    "spec.clusterName",
				BadValue: r.Spec.ClusterName,
				Detail:   "a DNS-1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')",
			},
		}
		return k8sErrors.NewInvalid(r.GroupVersionKind().GroupKind(), "InvalidSpecClusterName", errList)
	}

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *ClusterClaim) ValidateUpdate(old runtime.Object) error {
	oldClusterClaim := old.(*ClusterClaim).DeepCopy()

	if !r.ObjectMeta.DeletionTimestamp.IsZero() {
		return nil
	}

	if oldClusterClaim.Status.Phase == "Approved" || oldClusterClaim.Status.Phase == "Rejected" || oldClusterClaim.Status.Phase == "ClusterDeleted" {
		if !reflect.DeepEqual(oldClusterClaim.Spec, r.Spec) {
			return errors.New("cannot modify clusterClaim after approval")
		}
	}
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *ClusterClaim) ValidateDelete() error {
	clusterclaimlog.Info("validate delete", "name", r.Name)

	// if r.Status.Phase == "Awaiting" || r.Status.Phase == "" {
	// 	return nil
	// }
	// return errors.New("Cannot modify clusterClaim after approval")
	return nil
}
