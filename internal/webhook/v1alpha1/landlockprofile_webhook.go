package v1alpha1

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/flavio/podlock/api/v1alpha1"
)

// SetupRegistryWebhookWithManager registers the webhook for Registry in the manager.
func SetupRegistryWebhookWithManager(mgr ctrl.Manager) error {
	err := ctrl.NewWebhookManagedBy(mgr).For(&v1alpha1.LandlockProfile{}).
		WithValidator(&LandlockProfileCustomValidator{
			logger: mgr.GetLogger().WithName("landlockprofile_validator"),
		}).
		WithDefaulter(&LandLockProfileCustomDefaulter{
			logger: mgr.GetLogger().WithName("landlockprofile_validator"),
		}).
		Complete()
	if err != nil {
		return fmt.Errorf("failed to setup LandlockProfile webhook: %w", err)
	}
	return nil
}

// +kubebuilder:webhook:path=/mutate-podlock-kubewarden-io-v1alpha1-landlockprofile,mutating=true,failurePolicy=fail,sideEffects=None,groups=podlock.kubewarden.io,resources=landlockprofiles,verbs=create;update,versions=v1alpha1,name=mlandlockprofile.podlock.kubewarden.io,admissionReviewVersions=v1

type LandLockProfileCustomDefaulter struct {
	logger logr.Logger
}

var _ webhook.CustomDefaulter = &LandLockProfileCustomDefaulter{}

// Default implements admission.CustomDefaulter.
func (d *LandLockProfileCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	profile, ok := obj.(*v1alpha1.LandlockProfile)
	if !ok {
		return fmt.Errorf("expected a Registry object but got %T", obj)
	}

	d.logger.Info("Defaulting LandlockProfile", "name", profile.GetName())

	// TODO: add finalizer to ensure a profile cannot be deleted while in use by a Pod

	return nil
}

// +kubebuilder:webhook:path=/validate-podlock-kubewarden-io-v1alpha1-landlockprofile,mutating=false,failurePolicy=fail,sideEffects=None,groups=podlock.kubewarden.io,resources=landlockprofiles,verbs=create;update,versions=v1alpha1,name=vlandlockprofile.landlock.kubewarden.io,admissionReviewVersions=v1

type LandlockProfileCustomValidator struct {
	logger logr.Logger
}

var _ webhook.CustomValidator = &LandlockProfileCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type LandlockProfile.
func (v *LandlockProfileCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	profile, ok := obj.(*v1alpha1.LandlockProfile)
	if !ok {
		return nil, fmt.Errorf("expected a LandlockProfile object but got %T", obj)
	}
	v.logger.Info("Validation for LandlockProfile upon creation", "name", profile.GetName())

	// TODO: add validation logic
	// - ensure binary paths are absolute and do not container traversals
	// - ensure no overlapping paths between different access levels

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type LandlockProfile.
func (v *LandlockProfileCustomValidator) ValidateUpdate(_ context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	profile, ok := newObj.(*v1alpha1.LandlockProfile)
	if !ok {
		return nil, fmt.Errorf("expected a LandlockProfile object for the newObj but got %T", newObj)
	}
	v.logger.Info("Validation for LandlockProfile upon update", "name", profile.GetName())

	// TODO: add validation logic

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type LandlockProfile.
func (v *LandlockProfileCustomValidator) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	profile, ok := obj.(*v1alpha1.LandlockProfile)
	if !ok {
		return nil, fmt.Errorf("expected a LandlockProfile object but got %T", obj)
	}
	v.logger.Info("Validation for LandlockProfile upon deletion", "name", profile.GetName())

	return nil, nil
}
