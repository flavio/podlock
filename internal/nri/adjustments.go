package nri

import (
	"github.com/containerd/nri/pkg/api"

	podlockv1alpha1 "github.com/flavio/podlock/api/v1alpha1"
	"github.com/flavio/podlock/internal/seal"
)

func createContainerAdjustment(
	podID, containerName string,
	profileByBinary podlockv1alpha1.ProfileByBinary,
	logLevel string,
) *api.ContainerAdjustment {
	adjustment := &api.ContainerAdjustment{}

	// inject seal binary
	adjustment.AddMount(&api.Mount{
		Destination: SealBinaryPathContainer(),
		Source:      SealBinaryPathHost,
		Options:     []string{"rprivate", "rbind", "ro"},
		Type:        "bind",
	})

	// inject the profile
	adjustment.AddMount(&api.Mount{
		Destination: ContainerProfilePathInsideContainer(),
		Source:      landlockProfilePathOnHost(podID, containerName),
		Options:     []string{"rprivate", "rbind", "ro"},
		Type:        "bind",
	})

	// inject swap-oci-hook hooks for each binary

	createContainerHooks := []*api.Hook{}

	for binary := range profileByBinary {
		swappedBinOnHost := swappedBinaryPathOnHost(podID, containerName, binary)
		swappedBinInsideContainer := SwappedBinaryPathInsideContainer(binary)

		adjustment.AddMount(&api.Mount{
			Destination: swappedBinInsideContainer,
			Source:      swappedBinOnHost,
			Options:     []string{"rprivate", "rbind", "ro"},
			Type:        "bind",
		})

		hook := &api.Hook{
			Path: SwapOciHookBinaryPathHost,
			Args: []string{"swap-oci-hook", "-target", binary, "-backup", swappedBinInsideContainer},
		}

		createContainerHooks = append(createContainerHooks, hook)
	}
	adjustment.AddHooks(&api.Hooks{
		CreateContainer: createContainerHooks,
	})

	adjustment.AddEnv(seal.LogLevelEnvVar, logLevel)

	return adjustment
}
