package v1alpha1

import (
	"fmt"
	"path/filepath"

	"github.com/flavio/podlock/api/v1alpha1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func (v *LandlockProfileCustomValidator) validateBinaryPath(path string, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if !filepath.IsAbs(path) {
		allErrs = append(allErrs, field.Invalid(fldPath, path, "path must be absolute"))
	}

	if filepath.Clean(path) != path {
		allErrs = append(allErrs, field.Invalid(fldPath, path, "path contains traversals or is not clean"))
	}

	return allErrs
}

func (v *LandlockProfileCustomValidator) validateReadOnlyPaths(profile v1alpha1.Profile, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	for i, path := range profile.ReadOnly {
		allErrs = append(allErrs, v.validateBinaryPath(path, fldPath.Child("readOnly").Index(i))...)
	}

	return allErrs
}

func (v *LandlockProfileCustomValidator) validateReadWritePaths(profile v1alpha1.Profile, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	for i, path := range profile.ReadWrite {
		allErrs = append(allErrs, v.validateBinaryPath(path, fldPath.Child("readWrite").Index(i))...)
	}

	return allErrs
}

func (v *LandlockProfileCustomValidator) validateReadExecPaths(profile v1alpha1.Profile, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	for i, path := range profile.ReadExec {
		allErrs = append(allErrs, v.validateBinaryPath(path, fldPath.Child("readExec").Index(i))...)
	}

	return allErrs
}

func (v *LandlockProfileCustomValidator) validateReadWriteExecPaths(profile v1alpha1.Profile, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	for i, path := range profile.ReadWriteExec {
		allErrs = append(allErrs, v.validateBinaryPath(path, fldPath.Child("readWriteExec").Index(i))...)
	}

	return allErrs
}

func (v *LandlockProfileCustomValidator) validateNoOverlappingPaths(profile v1alpha1.Profile, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	readOnly := sets.New(profile.ReadOnly...)
	readWrite := sets.New(profile.ReadWrite...)
	readExec := sets.New(profile.ReadExec...)
	readWriteExec := sets.New(profile.ReadWriteExec...)

	// Check for overlaps between different access levels
	overlaps := []struct {
		field1 string
		set1   sets.Set[string]
		field2 string
		set2   sets.Set[string]
	}{
		{"readOnly", readOnly, "readWrite", readWrite},
		{"readOnly", readOnly, "readExec", readExec},
		{"readOnly", readOnly, "readWriteExec", readWriteExec},
		{"readWrite", readWrite, "readExec", readExec},
		{"readWrite", readWrite, "readWriteExec", readWriteExec},
		{"readExec", readExec, "readWriteExec", readWriteExec},
	}

	for _, overlap := range overlaps {
		intersection := overlap.set1.Intersection(overlap.set2)
		if intersection.Len() > 0 {
			allErrs = append(allErrs, field.Invalid(
				fldPath.Child(overlap.field1),
				sets.List(overlap.set1),
				fmt.Sprintf("overlapping paths with %s: %v", overlap.field2, sets.List(intersection)),
			))
		}
	}

	return allErrs
}
