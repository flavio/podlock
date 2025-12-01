package nri

import (
	"log/slog"

	ll "github.com/landlock-lsm/go-landlock/landlock/syscall"
)

// DetectLandlockVersion checks the Landlock ABI version supported by the kernel.
// It returns 0 if Landlock is not supported.
func DetectLandlockVersion(logger *slog.Logger) int {
	version, err := ll.LandlockGetABIVersion()
	if err != nil {
		logger.Warn("Failed to get Landlock ABI version", slog.Any("error", err))

		// That means Landlock is not supported
		return 0
	}

	logger.Info("Detected Landlock ABI version", slog.Int("version", version))

	return version
}
