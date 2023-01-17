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

package controllers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	claimV1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/claim/v1alpha1"
	clusterV1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/cluster/v1alpha1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var AutoAdmit bool

const (
	requeueAfter10Second = 10 * time.Second
	requeueAfter20Second = 20 * time.Second
	requeueAfter30Second = 30 * time.Second
	requeueAfter1Minute  = 1 * time.Minute
)

// ClusterClaimReconciler reconciles a ClusterClaim object
type ClusterClaimReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=claim.tmax.io,resources=clusterclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=claim.tmax.io,resources=clusterclaims/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.tmax.io,resources=clustermanagers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.tmax.io,resources=clustermanagers/status,verbs=get;update;patch

// cluster claim 이 생성되면, reconcile 함수는 해당 cluster claim 의 status 를 awaiting 으로 변경해준다.
// 해당 claim 으로 생성한 cluster 에 대한 cluster manager 의 생성은 hypercloud-api-server 에서 진행된다.
func (r *ClusterClaimReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	log := r.Log.WithValues("ClusterClaim", req.NamespacedName)

	clusterClaim := &claimV1alpha1.ClusterClaim{}
	if err := r.Client.Get(context.TODO(), req.NamespacedName, clusterClaim); errors.IsNotFound(err) {
		log.Info("ClusterClaim resource not found. Ignoring since object must be deleted")
		return ctrl.Result{}, nil
	} else if err != nil {
		log.Error(err, "Failed to get ClusterClaim")
		return ctrl.Result{}, err
	}

	if !AutoAdmit {
		Awaiting := clusterClaim.Status.Phase == claimV1alpha1.ClusterClaimPhaseAwaiting
		if clusterClaim.Status.Phase == "" {
			clusterClaim.Status.SetTypedPhase(claimV1alpha1.ClusterClaimPhaseAwaiting)
			clusterClaim.Status.SetReason("Waiting for admin approval")
			err := r.Status().Update(context.TODO(), clusterClaim)
			if err != nil {
				log.Error(err, "Failed to update ClusterClaim status")
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		} else if Awaiting {
			return ctrl.Result{}, nil
		}
	}

	// console로부터 approved로 변경시 clustermanager 생성
	Approved := clusterClaim.Status.Phase == claimV1alpha1.ClusterClaimPhaseApproved
	if Approved {
		if err := r.CreateClusterManager(context.TODO(), clusterClaim); err != nil {
			log.Error(err, "Failed to Create ClusterManager")
			return ctrl.Result{RequeueAfter: requeueAfter10Second}, nil
		}
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func (r *ClusterClaimReconciler) RequeueClusterClaimsForClusterManager(o client.Object) []ctrl.Request {
	clm := o.DeepCopyObject().(*clusterV1alpha1.ClusterManager)
	log := r.Log.WithValues("objectMapper", "clusterManagerToClusterClaim", "clusterManager", clm.Name)
	log.Info("Start to clusterManagerToClusterClaim mapping...")

	//get clusterManager
	cc := &claimV1alpha1.ClusterClaim{}
	key := types.NamespacedName{
		Name:      clm.Labels[clusterV1alpha1.LabelKeyClcName],
		Namespace: clm.Namespace,
	}
	if err := r.Client.Get(context.TODO(), key, cc); errors.IsNotFound(err) {
		log.Info("ClusterClaim resource not found. Ignoring since object must be deleted")
		return nil
	} else if err != nil {
		log.Error(err, "Failed to get ClusterClaim")
		return nil
	}

	NotApproved := cc.Status.Phase != claimV1alpha1.ClusterClaimPhaseApproved
	if NotApproved {
		log.Info("ClusterClaims for ClusterManager [" + cc.Spec.ClusterName + "] is already delete... Do not update cc status to delete ")
		return nil
	}

	cc.Status.SetTypedPhase(claimV1alpha1.ClusterClaimPhaseClusterDeleted)
	cc.Status.SetReason("cluster is deleted")
	err := r.Status().Update(context.TODO(), cc)
	if err != nil {
		log.Error(err, "Failed to update ClusterClaim status")
		return nil //??
	}
	return nil
}

func (r *ClusterClaimReconciler) SetupWithManager(mgr ctrl.Manager) error {
	controller, err := ctrl.NewControllerManagedBy(mgr).
		For(&claimV1alpha1.ClusterClaim{}).
		WithEventFilter(
			predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					return true
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					return true
				},
				DeleteFunc: func(e event.DeleteEvent) bool {
					return false
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			},
		).
		Build(r)

	if err != nil {
		return err
	}

	return controller.Watch(
		&source.Kind{Type: &clusterV1alpha1.ClusterManager{}},
		handler.EnqueueRequestsFromMapFunc(r.RequeueClusterClaimsForClusterManager),
		predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				return false
			},
			CreateFunc: func(e event.CreateEvent) bool {
				return false
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				clm := e.Object.(*clusterV1alpha1.ClusterManager)
				if clm.GetClusterType() != clusterV1alpha1.ClusterTypeRegistered {
					return true
				}
				return false
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return false
			},
		},
	)
}
