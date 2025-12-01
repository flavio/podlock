package constants

const (
	// LandlockVersionNodeLabelKey is the node label key
	// used to store the Landlock ABI version supported by the node kernel.
	LandlockVersionNodeLabelKey = "podlock.kubewarden.io/landlock-version"

	// PodProfileLabel is the label used by pods to enable PodLock NRI plugin.
	PodProfileLabel = "podlock.kubewarden.io/profile"
)
