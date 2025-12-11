/*
Copyright 2025.

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

package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/flavio/podlock/api/v1alpha1"
	"github.com/flavio/podlock/pkg/constants"
)

// LandlockProfileReconciler reconciles a LandlockProfile object
type LandlockProfileReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=podlock.kubewarden.io,resources=landlockprofiles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=podlock.kubewarden.io,resources=landlockprofiles/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=podlock.kubewarden.io,resources=landlockprofiles/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the LandlockProfile object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.22.4/pkg/reconcile
func (r *LandlockProfileReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	profile := &v1alpha1.LandlockProfile{}
	if err := r.Get(ctx, req.NamespacedName, profile); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get LandlockProfile '%s/%s': %w", req.Namespace, req.Name, err)
	}

	// Check if the profile is being deleted
	if !profile.DeletionTimestamp.IsZero() {
		return handleDeletion(ctx, r, profile)
	}

	return ctrl.Result{}, nil
}

func handleDeletion(ctx context.Context, r *LandlockProfileReconciler, profile *v1alpha1.LandlockProfile) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(profile, v1alpha1.LandlockProfileFinalizer) {
		// Check if any pods are using this profile
		// Note, this usses a PartialObjectMetadataList because the Pod cache
		// is configured to only store metadata
		podList := &metav1.PartialObjectMetadataList{}
		podList.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "PodList",
		})

		err := r.List(ctx, podList,
			client.InNamespace(profile.Namespace),
			client.MatchingLabels{constants.PodProfileLabel: profile.Name},
		)
		if err != nil {
			logger.Error(err, "Failed to list pods using profile")
			return ctrl.Result{}, fmt.Errorf("failed to list pods using profile: %w", err)
		}

		if len(podList.Items) > 0 {
			logger.Info("Cannot remove finalizer: profile still in use by pods",
				"profile", profile.Name,
				"podCount", len(podList.Items))
			// Requeue to check again later
			return ctrl.Result{RequeueAfter: 90 * time.Second}, nil
		}

		// No pods using this profile, safe to remove finalizer
		original := profile.DeepCopy()
		controllerutil.RemoveFinalizer(profile, v1alpha1.LandlockProfileFinalizer)
		if err := r.Patch(ctx, profile, client.MergeFrom(original)); err != nil {
			logger.Error(err, "Failed to remove finalizer")
			return ctrl.Result{}, fmt.Errorf("failed to remove finalizer from LandlockProfile '%s/%s': %w", profile.Namespace, profile.Name, err)
		}
		logger.Info("Removed finalizer from LandlockProfile", "profile", profile.Name)
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LandlockProfileReconciler) SetupWithManager(mgr ctrl.Manager) error {
	err := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.LandlockProfile{}).
		Watches(
			&corev1.Pod{},
			handler.EnqueueRequestsFromMapFunc(r.findProfilesForPod),
			builder.OnlyMetadata,
		).
		Named("landlockprofile").
		Complete(r)
	if err != nil {
		return fmt.Errorf("unable to set up LandlockProfile controller: %w", err)
	}
	return nil
}

// findProfilesForPod maps a Pod to the LandlockProfile(s) it references
func (r *LandlockProfileReconciler) findProfilesForPod(_ context.Context, pod client.Object) []ctrl.Request {
	profileName, ok := pod.GetLabels()[constants.PodProfileLabel]
	if !ok {
		return nil
	}

	return []ctrl.Request{
		{
			NamespacedName: client.ObjectKey{
				Name:      profileName,
				Namespace: pod.GetNamespace(),
			},
		},
	}
}
