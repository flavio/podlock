package v1alpha1

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/flavio/podlock/api/v1alpha1"
)

// SetupRegistryWebhookWithManager registers the webhook for Registry in the manager.
func SetupRegistryWebhookWithManager(mgr ctrl.Manager) error {
	err := ctrl.NewWebhookManagedBy(mgr, &v1alpha1.LandlockProfile{}).
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

var _ admission.Defaulter[*v1alpha1.LandlockProfile] = &LandLockProfileCustomDefaulter{}

// Default implements admission.Defaulter.
func (d *LandLockProfileCustomDefaulter) Default(_ context.Context, profile *v1alpha1.LandlockProfile) error {
	// Check if the profile is being deleted
	if !profile.DeletionTimestamp.IsZero() {
		// No need to default a deleting object
		return nil
	}

	d.logger.Info("Defaulting LandlockProfile", "name", profile.GetName())

	// Add finalizer to ensure a profile cannot be deleted while in use by a Pod
	if !controllerutil.ContainsFinalizer(profile, v1alpha1.LandlockProfileFinalizer) {
		controllerutil.AddFinalizer(profile, v1alpha1.LandlockProfileFinalizer)
		d.logger.Info("Added finalizer to LandlockProfile", "name", profile.GetName())
	}

	return nil
}

// +kubebuilder:webhook:path=/validate-podlock-kubewarden-io-v1alpha1-landlockprofile,mutating=false,failurePolicy=fail,sideEffects=None,groups=podlock.kubewarden.io,resources=landlockprofiles,verbs=create;update,versions=v1alpha1,name=vlandlockprofile.landlock.kubewarden.io,admissionReviewVersions=v1

type LandlockProfileCustomValidator struct {
	logger logr.Logger
}

var _ admission.Validator[*v1alpha1.LandlockProfile] = &LandlockProfileCustomValidator{}

// ValidateCreate implements admission.Validator so a webhook will be registered for the type LandlockProfile.
func (v *LandlockProfileCustomValidator) ValidateCreate(_ context.Context, profile *v1alpha1.LandlockProfile) (admission.Warnings, error) {
	v.logger.Info("Validation for LandlockProfile upon creation", "name", profile.GetName())

	allErrs := v.validateProfile(profile)

	if len(allErrs) > 0 {
		return nil, apierrors.NewInvalid(
			v1alpha1.GroupVersion.WithKind("LandlockProfile").GroupKind(),
			profile.Name,
			allErrs,
		)
	}

	return nil, nil
}

// ValidateUpdate implements admission.Validator so a webhook will be registered for the type LandlockProfile.
func (v *LandlockProfileCustomValidator) ValidateUpdate(_ context.Context, _, newObj *v1alpha1.LandlockProfile) (admission.Warnings, error) {
	profile := newObj
	v.logger.Info("Validation for LandlockProfile upon update", "name", profile.GetName())

	allErrs := v.validateProfile(profile)

	if len(allErrs) > 0 {
		return nil, apierrors.NewInvalid(
			v1alpha1.GroupVersion.WithKind("LandlockProfile").GroupKind(),
			profile.Name,
			allErrs,
		)
	}

	return nil, nil
}

func (v *LandlockProfileCustomValidator) validateProfile(profile *v1alpha1.LandlockProfile) field.ErrorList {
	var allErrs field.ErrorList

	specPath := field.NewPath("spec", "profilesByContainer")
	for containerName, profileByBinary := range profile.Spec.ProfilesByContainer {
		containerPath := specPath.Key(containerName)
		for binaryPath, binProfile := range profileByBinary {
			binaryPathField := containerPath.Key(binaryPath)

			allErrs = append(allErrs, v.validateBinaryPath(binaryPath, binaryPathField)...)
			allErrs = append(allErrs, v.validateNoOverlappingPaths(binProfile, binaryPathField)...)
			allErrs = append(allErrs, v.validateReadOnlyPaths(binProfile, binaryPathField)...)
			allErrs = append(allErrs, v.validateReadWritePaths(binProfile, binaryPathField)...)
			allErrs = append(allErrs, v.validateReadExecPaths(binProfile, binaryPathField)...)
			allErrs = append(allErrs, v.validateReadWriteExecPaths(binProfile, binaryPathField)...)
		}
	}

	return allErrs
}

// ValidateDelete implements admission.Validator so a webhook will be registered for the type LandlockProfile.
func (v *LandlockProfileCustomValidator) ValidateDelete(_ context.Context, profile *v1alpha1.LandlockProfile) (admission.Warnings, error) {
	v.logger.Info("Validation for LandlockProfile upon deletion", "name", profile.GetName())

	return nil, nil
}
