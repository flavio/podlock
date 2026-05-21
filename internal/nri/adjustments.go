package nri

import (
	"github.com/containerd/nri/pkg/api"

	podlockv1alpha1 "github.com/flavio/podlock/api/v1alpha1"
	"github.com/flavio/podlock/internal/seal"
)

const (
	mountTypeBind   = "bind"
	mountOptionBind = "rbind"
	mountOptionPriv = "rprivate"
	hookArgBackup   = "-backup"
	hookArgTarget   = "-target"
	swapOciHookCmd  = "swap-oci-hook"
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
		Options:     []string{mountOptionPriv, mountOptionBind, "ro"},
		Type:        mountTypeBind,
	})

	// inject the profile
	adjustment.AddMount(&api.Mount{
		Destination: ContainerProfilePathInsideContainer(),
		Source:      landlockProfilePathOnHost(podID, containerName),
		Options:     []string{mountOptionPriv, mountOptionBind, "ro"},
		Type:        mountTypeBind,
	})

	// inject swap-oci-hook hooks for each binary

	createContainerHooks := []*api.Hook{}

	for binary := range profileByBinary {
		swappedBinOnHost := swappedBinaryPathOnHost(podID, containerName, binary)
		swappedBinInsideContainer := SwappedBinaryPathInsideContainer(binary)

		adjustment.AddMount(&api.Mount{
			Destination: swappedBinInsideContainer,
			Source:      swappedBinOnHost,
			Options:     []string{mountOptionPriv, mountOptionBind, "ro"},
			Type:        mountTypeBind,
		})

		hook := &api.Hook{
			Path: SwapOciHookBinaryPathHost,
			Args: []string{swapOciHookCmd, hookArgTarget, binary, hookArgBackup, swappedBinInsideContainer},
		}

		createContainerHooks = append(createContainerHooks, hook)
	}
	adjustment.AddHooks(&api.Hooks{
		CreateContainer: createContainerHooks,
	})

	adjustment.AddEnv(seal.LogLevelEnvVar, logLevel)

	return adjustment
}
