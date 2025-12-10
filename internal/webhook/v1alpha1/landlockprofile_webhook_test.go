package v1alpha1

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/flavio/podlock/api/v1alpha1"
)

func TestLandlockProfileCustomValidator_ValidateCreate(t *testing.T) {
	validator := &LandlockProfileCustomValidator{
		logger: logr.Discard(),
	}

	tests := []struct {
		name    string
		profile *v1alpha1.LandlockProfile
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid profile",
			profile: &v1alpha1.LandlockProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-profile",
					Namespace: "default",
				},
				Spec: v1alpha1.LandlockProfileSpec{
					ProfilesByContainer: map[string]v1alpha1.ProfileByBinary{
						"nginx": {
							"/usr/sbin/nginx": {
								ReadOnly: []string{"/etc/nginx"},
								ReadExec: []string{"/lib", "/lib64"},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "relative binary path",
			profile: &v1alpha1.LandlockProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-profile",
					Namespace: "default",
				},
				Spec: v1alpha1.LandlockProfileSpec{
					ProfilesByContainer: map[string]v1alpha1.ProfileByBinary{
						"nginx": {
							"usr/sbin/nginx": {
								ReadOnly: []string{"/etc/nginx"},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "path must be absolute",
		},
		{
			name: "binary path with traversal",
			profile: &v1alpha1.LandlockProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-profile",
					Namespace: "default",
				},
				Spec: v1alpha1.LandlockProfileSpec{
					ProfilesByContainer: map[string]v1alpha1.ProfileByBinary{
						"nginx": {
							"/usr/../bin/nginx": {
								ReadOnly: []string{"/etc/nginx"},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "path contains traversals or is not clean",
		},
		{
			name: "invalid path in readOnly",
			profile: &v1alpha1.LandlockProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-profile",
					Namespace: "default",
				},
				Spec: v1alpha1.LandlockProfileSpec{
					ProfilesByContainer: map[string]v1alpha1.ProfileByBinary{
						"nginx": {
							"/usr/sbin/nginx": {
								ReadOnly: []string{"etc/nginx"},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "path must be absolute",
		},
		{
			name: "invalid path in readWrite",
			profile: &v1alpha1.LandlockProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-profile",
					Namespace: "default",
				},
				Spec: v1alpha1.LandlockProfileSpec{
					ProfilesByContainer: map[string]v1alpha1.ProfileByBinary{
						"nginx": {
							"/usr/sbin/nginx": {
								ReadWrite: []string{"tmp"},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "path must be absolute",
		},
		{
			name: "invalid path in readExec",
			profile: &v1alpha1.LandlockProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-profile",
					Namespace: "default",
				},
				Spec: v1alpha1.LandlockProfileSpec{
					ProfilesByContainer: map[string]v1alpha1.ProfileByBinary{
						"app": {
							"/usr/bin/app": {
								ReadExec: []string{"lib"},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "path must be absolute",
		},
		{
			name: "invalid path in readWriteExec",
			profile: &v1alpha1.LandlockProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-profile",
					Namespace: "default",
				},
				Spec: v1alpha1.LandlockProfileSpec{
					ProfilesByContainer: map[string]v1alpha1.ProfileByBinary{
						"app": {
							"/usr/bin/app": {
								ReadWriteExec: []string{"opt"},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "path must be absolute",
		},
		{
			name: "path with traversal in readOnly",
			profile: &v1alpha1.LandlockProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-profile",
					Namespace: "default",
				},
				Spec: v1alpha1.LandlockProfileSpec{
					ProfilesByContainer: map[string]v1alpha1.ProfileByBinary{
						"nginx": {
							"/usr/sbin/nginx": {
								ReadOnly: []string{"/etc/../nginx"},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "path contains traversals or is not clean",
		},
		{
			name: "overlapping paths - readOnly and readWrite",
			profile: &v1alpha1.LandlockProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-profile",
					Namespace: "default",
				},
				Spec: v1alpha1.LandlockProfileSpec{
					ProfilesByContainer: map[string]v1alpha1.ProfileByBinary{
						"nginx": {
							"/usr/sbin/nginx": {
								ReadOnly:  []string{"/etc", "/tmp"},
								ReadWrite: []string{"/tmp"},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "overlapping paths with readWrite",
		},
		{
			name: "overlapping paths - readExec and readWriteExec",
			profile: &v1alpha1.LandlockProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-profile",
					Namespace: "default",
				},
				Spec: v1alpha1.LandlockProfileSpec{
					ProfilesByContainer: map[string]v1alpha1.ProfileByBinary{
						"app": {
							"/usr/bin/app": {
								ReadExec:      []string{"/etc", "/lib"},
								ReadWriteExec: []string{"/lib"},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "overlapping paths with readWriteExec",
		},
		{
			name: "multiple overlaps",
			profile: &v1alpha1.LandlockProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-profile",
					Namespace: "default",
				},
				Spec: v1alpha1.LandlockProfileSpec{
					ProfilesByContainer: map[string]v1alpha1.ProfileByBinary{
						"app": {
							"/usr/bin/app": {
								ReadOnly:      []string{"/etc", "/var"},
								ReadWrite:     []string{"/etc"},
								ReadExec:      []string{"/var"},
								ReadWriteExec: []string{"/opt"},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "overlapping paths",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validator.ValidateCreate(context.Background(), tt.profile)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestLandlockProfileCustomValidator_ValidateUpdate(t *testing.T) {
	validator := &LandlockProfileCustomValidator{
		logger: logr.Discard(),
	}

	tests := []struct {
		name       string
		oldProfile *v1alpha1.LandlockProfile
		newProfile *v1alpha1.LandlockProfile
		wantErr    bool
		errMsg     string
	}{
		{
			name: "valid update",
			oldProfile: &v1alpha1.LandlockProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-profile",
					Namespace: "default",
				},
				Spec: v1alpha1.LandlockProfileSpec{
					ProfilesByContainer: map[string]v1alpha1.ProfileByBinary{
						"nginx": {
							"/usr/sbin/nginx": {
								ReadOnly: []string{"/etc/nginx"},
							},
						},
					},
				},
			},
			newProfile: &v1alpha1.LandlockProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-profile",
					Namespace: "default",
				},
				Spec: v1alpha1.LandlockProfileSpec{
					ProfilesByContainer: map[string]v1alpha1.ProfileByBinary{
						"nginx": {
							"/usr/sbin/nginx": {
								ReadOnly: []string{"/etc/nginx", "/usr/share/nginx"},
								ReadExec: []string{"/lib", "/lib64"},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "update with invalid binary path",
			oldProfile: &v1alpha1.LandlockProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-profile",
					Namespace: "default",
				},
				Spec: v1alpha1.LandlockProfileSpec{
					ProfilesByContainer: map[string]v1alpha1.ProfileByBinary{
						"nginx": {
							"/usr/sbin/nginx": {
								ReadOnly: []string{"/etc/nginx"},
							},
						},
					},
				},
			},
			newProfile: &v1alpha1.LandlockProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-profile",
					Namespace: "default",
				},
				Spec: v1alpha1.LandlockProfileSpec{
					ProfilesByContainer: map[string]v1alpha1.ProfileByBinary{
						"nginx": {
							"usr/sbin/nginx": {
								ReadOnly: []string{"/etc/nginx"},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "path must be absolute",
		},
		{
			name: "update with invalid path in readWrite",
			oldProfile: &v1alpha1.LandlockProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-profile",
					Namespace: "default",
				},
				Spec: v1alpha1.LandlockProfileSpec{
					ProfilesByContainer: map[string]v1alpha1.ProfileByBinary{
						"nginx": {
							"/usr/sbin/nginx": {
								ReadOnly: []string{"/etc/nginx"},
							},
						},
					},
				},
			},
			newProfile: &v1alpha1.LandlockProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-profile",
					Namespace: "default",
				},
				Spec: v1alpha1.LandlockProfileSpec{
					ProfilesByContainer: map[string]v1alpha1.ProfileByBinary{
						"nginx": {
							"/usr/sbin/nginx": {
								ReadWrite: []string{"tmp"},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "path must be absolute",
		},
		{
			name: "update with overlapping paths",
			oldProfile: &v1alpha1.LandlockProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-profile",
					Namespace: "default",
				},
				Spec: v1alpha1.LandlockProfileSpec{
					ProfilesByContainer: map[string]v1alpha1.ProfileByBinary{
						"nginx": {
							"/usr/sbin/nginx": {
								ReadOnly: []string{"/etc/nginx"},
							},
						},
					},
				},
			},
			newProfile: &v1alpha1.LandlockProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-profile",
					Namespace: "default",
				},
				Spec: v1alpha1.LandlockProfileSpec{
					ProfilesByContainer: map[string]v1alpha1.ProfileByBinary{
						"nginx": {
							"/usr/sbin/nginx": {
								ReadOnly:  []string{"/etc", "/tmp"},
								ReadWrite: []string{"/tmp"},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "overlapping paths with readWrite",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validator.ValidateUpdate(context.Background(), tt.oldProfile, tt.newProfile)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestLandlockProfileCustomValidator_ValidateDelete(t *testing.T) {
	validator := &LandlockProfileCustomValidator{
		logger: logr.Discard(),
	}

	profile := &v1alpha1.LandlockProfile{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-profile",
			Namespace: "default",
		},
		Spec: v1alpha1.LandlockProfileSpec{
			ProfilesByContainer: map[string]v1alpha1.ProfileByBinary{
				"nginx": {
					"/usr/sbin/nginx": {
						ReadOnly: []string{"/etc/nginx"},
					},
				},
			},
		},
	}

	_, err := validator.ValidateDelete(context.Background(), profile)
	require.NoError(t, err)
}
