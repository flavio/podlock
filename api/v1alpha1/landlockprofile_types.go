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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// LandlockProfileFinalizer is added to LandlockProfile resources to ensure
	// they are not deleted while still in use by Pods.
	LandlockProfileFinalizer = "podlock.kubewarden.io/landlockprofile"
)

type ProfileByBinary map[string]Profile

// LandlockProfileSpec defines the desired state of LandlockProfile
type LandlockProfileSpec struct {
	// +optional
	ProfilesByContainer map[string]ProfileByBinary `json:"profilesByContainer,omitempty"`
}

type Profile struct {
	ReadOnly      []string `json:"readOnly,omitempty"`
	ReadWrite     []string `json:"readWrite,omitempty"`
	ReadExec      []string `json:"readExec,omitempty"`
	ReadWriteExec []string `json:"readWriteExec,omitempty"`
}

// LandlockProfileStatus defines the observed state of LandlockProfile.
type LandlockProfileStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the LandlockProfile resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// LandlockProfile is the Schema for the landlockprofiles API
type LandlockProfile struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of LandlockProfile
	// +required
	Spec LandlockProfileSpec `json:"spec"`

	// status defines the observed state of LandlockProfile
	// +optional
	Status LandlockProfileStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// LandlockProfileList contains a list of LandlockProfile
type LandlockProfileList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []LandlockProfile `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LandlockProfile{}, &LandlockProfileList{})
}
