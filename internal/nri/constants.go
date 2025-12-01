package nri

import "path/filepath"

const (

	// SealBinaryPathHost is the path where the seal binary is located on the host
	SealBinaryPathHost = "/opt/podlock/bin/seal"

	// SwapOciHookBinaryPathContainer is the path where the seal binary is located within the NRI container
	SwapOciHookBinaryPathContainer = "/swap-oci-hook"

	// SwapOciHookBinaryPathHost is the path where the seal binary is located on the host
	SwapOciHookBinaryPathHost = "/opt/podlock/bin/swap-oci-hook"

	// PodLockContainerDataDir is the directory inside the container where PodLock
	// stores its data
	PodLockContainerDataDir = "/.podlock/"

	// PodLockContainerSwappedBinariesDir is the directory inside the container
	// where PodLock stores the swapped binaries
	PodLockContainerSwappedBinariesDir = "/.podlock/swapped-binaries/"

	// PodLockVarRunDir is the directory where PodLock NRI plugin stores
	// runtime files. It's the same dir inside the container and on the host.
	PodLockVarRunDir = "/var/run/podlock/"

	// ContainerProfileName is the name of the file where the landlock profile
	// is storedr.
	ContainerProfileName = "profile.json"
)

// SealBinaryPathContainer returns the path where the seal binary is located
// within the container.
func SealBinaryPathContainer() string {
	return filepath.Join(PodLockContainerDataDir, "bin", "seal")
}

func ContainerProfilePathInsideContainer() string {
	return filepath.Join(PodLockContainerDataDir, ContainerProfileName)
}
