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
	"strings"

	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var logger = logf.Log.WithName("clusterregistration-resource")

func (r *ClusterRegistration) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// +kubebuilder:webhook:verbs=create;update,path=/validate-cluster-tmax-io-v1alpha1-clusterregistration,mutating=false,failurePolicy=fail,groups=cluster.tmax.io,resources=clusterregistrations,versions=v1alpha1,name=validation.webhook.clusterregistration

var _ webhook.Validator = &ClusterRegistration{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ClusterRegistration) ValidateCreate() error {
	logger.Info("validate create", "name", r.Name)

	// clusterclaim_webhook.go의 주석내용 참조
	// reg, _ := regexp.Compile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)
	reg, _ := regexp.Compile(`^[a-z]([-a-z0-9]*[a-z0-9])?$`)
	if !reg.MatchString(r.Spec.ClusterName) {
		errList := []*field.Error{
			{
				Type:     field.ErrorTypeInvalid,
				Field:    "spec.clusterName",
				BadValue: r.Spec.ClusterName,
				Detail: strings.Join(
					[]string{
						"a DNS-1035 label must consist of lower case alphanumeric characters or '-', ",
						"start with an alphabetic character, and end with an alphanumeric character ",
						"(e.g. 'my-name',  or 'abc-123', regex used for validation is '[a-z]([-a-z0-9]*[a-z0-9])?')",
						// "a DNS-1123 subdomain must consist of lower case alphanumeric characters, '-' or '.',",
						// " and must start and end with an alphanumeric character (e.g. 'example.com',",
						// " regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')",
					},
					"",
				),
			},
		}
		return k8sErrors.NewInvalid(r.GroupVersionKind().GroupKind(), "InvalidSpecClusterName", errList)
	}

	// if len(r.Spec.ClusterName) > 253 {
	if len(r.Spec.ClusterName) > 63 {
		errList := []*field.Error{
			{
				Type:     field.ErrorTypeInvalid,
				Field:    "spec.clusterName",
				BadValue: r.Spec.ClusterName,
				Detail:   "must be no more than 63 characters",
				// Detail:   "must be no more than 253 characters",
			},
		}
		return k8sErrors.NewInvalid(r.GroupVersionKind().GroupKind(), "InvalidSpecClusterName", errList)
	}

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *ClusterRegistration) ValidateUpdate(old runtime.Object) error {
	logger.Info("validate update", "name", r.Name)
	oldClusterRegistration := old.(*ClusterRegistration).DeepCopy()

	if !r.ObjectMeta.DeletionTimestamp.IsZero() {
		return nil
	}

	if oldClusterRegistration.Status.Phase == "Success" || oldClusterRegistration.Status.Phase == "Deleted" {
		if !reflect.DeepEqual(oldClusterRegistration.Spec, r.Spec) {
			return errors.New("cannot modify ClusterRegistration after approval")
		}
	}
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *ClusterRegistration) ValidateDelete() error {
	logger.Info("validate delete", "name", r.Name)

	// cluster가 남아있으면 cluster claim을 삭제하지 못하도록 처리
	if r.Status.Phase != "ClusterDeleted" &&
		r.Status.Phase != "Awaiting" &&
		r.Status.Phase != "Rejected" {
		return k8sErrors.NewBadRequest("Deleting cluster must precedes deleting cluster registration.")
	}
	// if r.Status.Phase == "Awaiting" || r.Status.Phase == "" {
	// 	return nil
	// }
	// return errors.New("Cannot modify clusterClaim after approval")
	return nil
}
