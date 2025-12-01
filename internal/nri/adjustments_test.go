package nri

import (
	"testing"

	"github.com/containerd/nri/pkg/api"
	podlockv1alpha1 "github.com/flavio/podlock/api/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func TestCreateContainerAdjustment(t *testing.T) {
	tests := []struct {
		name            string
		podID           string
		containerName   string
		profileByBinary podlockv1alpha1.ProfileByBinary
		logLevel        string
		expectMounts    []api.Mount
		expectEnv       map[string]string
		expectHooks     []*api.Hook
	}{
		{
			name:          "single binary",
			podID:         "pod1",
			containerName: "cont1",
			profileByBinary: podlockv1alpha1.ProfileByBinary{
				"/bin/ls": {},
			},
			logLevel: "debug",
			expectMounts: []api.Mount{
				{
					Destination: SealBinaryPathContainer(),
					Source:      SealBinaryPathHost,
					Options:     []string{"rprivate", "rbind", "ro"},
					Type:        "bind",
				},
				{
					Destination: ContainerProfilePathInsideContainer(),
					Source:      landlockProfilePathOnHost("pod1", "cont1"),
					Options:     []string{"rprivate", "rbind", "ro"},
					Type:        "bind",
				},
				{
					Destination: SwappedBinaryPathInsideContainer("/bin/ls"),
					Source:      swappedBinaryPathOnHost("pod1", "cont1", "/bin/ls"),
					Options:     []string{"rprivate", "rbind", "ro"},
					Type:        "bind",
				},
			},
			expectEnv: map[string]string{
				"SEAL_LOG_LEVEL": "debug",
			},
			expectHooks: []*api.Hook{
				{
					Path: SwapOciHookBinaryPathHost,
					Args: []string{"swap-oci-hook", "-target", "/bin/ls", "-backup", SwappedBinaryPathInsideContainer("/bin/ls")},
				},
			},
		},
		{
			name:          "multiple binaries",
			podID:         "pod2",
			containerName: "cont2",
			profileByBinary: podlockv1alpha1.ProfileByBinary{
				"/bin/ls":  {},
				"/bin/cat": {},
			},
			logLevel: "info",
			expectMounts: []api.Mount{
				{
					Destination: SealBinaryPathContainer(),
					Source:      SealBinaryPathHost,
					Options:     []string{"rprivate", "rbind", "ro"},
					Type:        "bind",
				},
				{
					Destination: ContainerProfilePathInsideContainer(),
					Source:      landlockProfilePathOnHost("pod2", "cont2"),
					Options:     []string{"rprivate", "rbind", "ro"},
					Type:        "bind",
				},
				{
					Destination: SwappedBinaryPathInsideContainer("/bin/ls"),
					Source:      swappedBinaryPathOnHost("pod2", "cont2", "/bin/ls"),
					Options:     []string{"rprivate", "rbind", "ro"},
					Type:        "bind",
				},
				{
					Destination: SwappedBinaryPathInsideContainer("/bin/cat"),
					Source:      swappedBinaryPathOnHost("pod2", "cont2", "/bin/cat"),
					Options:     []string{"rprivate", "rbind", "ro"},
					Type:        "bind",
				},
			},
			expectEnv: map[string]string{
				"SEAL_LOG_LEVEL": "info",
			},
			expectHooks: []*api.Hook{
				{
					Path: SwapOciHookBinaryPathHost,
					Args: []string{"swap-oci-hook", "-target", "/bin/ls", "-backup", SwappedBinaryPathInsideContainer("/bin/ls")},
				},
				{
					Path: SwapOciHookBinaryPathHost,
					Args: []string{"swap-oci-hook", "-target", "/bin/cat", "-backup", SwappedBinaryPathInsideContainer("/bin/cat")},
				},
			},
		},
		{
			name:            "no binaries",
			podID:           "pod3",
			containerName:   "cont3",
			profileByBinary: podlockv1alpha1.ProfileByBinary{},
			logLevel:        "warn",
			expectMounts: []api.Mount{
				{
					Destination: SealBinaryPathContainer(),
					Source:      SealBinaryPathHost,
					Options:     []string{"rprivate", "rbind", "ro"},
					Type:        "bind",
				},
				{
					Destination: ContainerProfilePathInsideContainer(),
					Source:      landlockProfilePathOnHost("pod3", "cont3"),
					Options:     []string{"rprivate", "rbind", "ro"},
					Type:        "bind",
				},
			},
			expectEnv: map[string]string{
				"SEAL_LOG_LEVEL": "warn",
			},
			expectHooks: []*api.Hook{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adj := createContainerAdjustment(tt.podID, tt.containerName, tt.profileByBinary, tt.logLevel)
			assert.NotNil(t, adj)

			// Check mounts (order doesn't matter)
			for i := range tt.expectMounts {
				want := &tt.expectMounts[i]
				assert.Truef(t, containsMountWithFields(adj.GetMounts(), want), "expected mount %+v not found in actual mounts", want)
			}

			// Check env
			gotEnv := map[string]string{}
			for _, env := range adj.GetEnv() {
				gotEnv[env.GetKey()] = env.GetValue()
			}
			assert.Equal(t, tt.expectEnv, gotEnv)

			// Check hooks (order doesn't matter)
			if adj.GetHooks() != nil {
				assert.ElementsMatch(t, tt.expectHooks, adj.GetHooks().Hooks().GetCreateContainer())
			} else {
				assert.Empty(t, tt.expectHooks)
			}
		})
	}
}

func containsMountWithFields(actual []*api.Mount, expected *api.Mount) bool {
	for _, act := range actual {
		match := true
		if expected.GetDestination() != "" && act.GetDestination() != expected.GetDestination() {
			match = false
		}
		if expected.GetSource() != "" && act.GetSource() != expected.GetSource() {
			match = false
		}
		if len(expected.GetOptions()) > 0 && !assert.ObjectsAreEqualValues(expected.GetOptions(), act.GetOptions()) {
			match = false
		}
		if expected.GetType() != "" && act.GetType() != expected.GetType() {
			match = false
		}
		if match {
			return true
		}
	}
	return false
}
