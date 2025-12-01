package nri

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	podlockv1alpha1 "github.com/flavio/podlock/api/v1alpha1"
)

// fileSha256 returns the SHA256 checksum of the given file.
func fileSha256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file '%s': %w", path, err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("failed to compute SHA256 of file '%s': %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// CopyFileIfDifferent copies the file from src to dst, but only if the SHA256
// checksum differs.
func CopyFileIfDifferent(src, dst string, logger *slog.Logger) error {
	dstDir := filepath.Dir(dst)

	srcSum, err := fileSha256(src)
	if err != nil {
		return fmt.Errorf(
			"failed to compute SHA256 of source file '%s': %w", src, err)
	}
	dstSum, err := fileSha256(dst)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf(
			"failed to compute SHA256 of destination file '%s': %w", dst, err)
	}

	if srcSum == dstSum {
		logger.Debug(
			"destination file is up to date, skipping copy",
			slog.String("path", dst))
		return nil
	}

	if err = os.MkdirAll(dstDir, 0o750); err != nil {
		return fmt.Errorf("failed to create %s directory: %w", dstDir, err)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	//nolint:gosec // these are executable binaries
	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf(
			"failed to create destination file '%s': %w",
			dst, err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file to destination: %w", err)
	}

	logger.Debug(
		"copied file to destination",
		slog.String("source", src),
		slog.String("destination", dst))

	return nil
}

// SwappedBinaryPathInsideContainer returns the path where PodLock will store the swapped binary
// for the given original binary path.
//
// For example, the original binary "/usr/bin/curl" will be swapped to
// "/.podlock/swapped-binaries/usr/bin/curl" inside the container.
func SwappedBinaryPathInsideContainer(originalBinaryPath string) string {
	return filepath.Join(
		PodLockContainerSwappedBinariesDir,
		originalBinaryPath,
	)
}

// swappedBinaryPathOnHost returns the path on the host where the swapped binary
// will be stored for the given pod ID, container ID, and original binary path.
//
// For example, for pod ID "pod123", container name "ctr1", and original binary
// "/usr/bin/curl", the swapped binary path on the host will be
// "/var/run/podlock/pod123/ctr1/swapped-binaries/usr/bin/curl".
func swappedBinaryPathOnHost(podID, containerName, originalBinaryPath string) string {
	return filepath.Join(
		PodLockVarRunDir,
		podID,
		containerName,
		"swapped-binaries",
		originalBinaryPath,
	)
}

// reserveSwappedBinaries creates empty files on the host to reserve the paths
// where the swapped binaries will be stored.
func (p *Plugin) reserveSwappedBinaries(
	podID,
	containerName string,
	profileByBinary podlockv1alpha1.ProfileByBinary,
) error {
	for binary := range profileByBinary {
		swappedBin := swappedBinaryPathOnHost(podID, containerName, binary)
		p.Logger.Debug(
			"reserving swapped binary path",
			slog.String("pod ID", podID),
			slog.String("container name", containerName),
			slog.String("binary", binary),
			slog.String("swapped binary path", swappedBin),
		)

		// Create the directory for the swapped binary
		if err := os.MkdirAll(
			filepath.Dir(swappedBin),
			0o750,
		); err != nil {
			return fmt.Errorf(
				"failed to create runtime dir for swapped binary '%s': %w",
				swappedBin,
				err,
			)
		}

		// Now create an empty file to reserve the path
		//nolint:gosec // we need to set the exec permissions for binaries that we're about to mount
		f, err := os.OpenFile(
			swappedBin,
			os.O_CREATE,
			0o755,
		)
		if err != nil {
			return fmt.Errorf(
				"failed to create empty file for swapped binary '%s': %w",
				swappedBin,
				err,
			)
		}
		if err = f.Close(); err != nil {
			return fmt.Errorf(
				"failed to close file for swapped binary '%s': %w",
				swappedBin,
				err,
			)
		}
	}

	return nil
}

func landlockProfilePathOnHost(podID, containerName string) string {
	return filepath.Join(
		PodLockVarRunDir,
		podID,
		containerName,
		ContainerProfileName,
	)
}

func (p *Plugin) writeLandlockProfileToHostFilesystem(
	podID,
	containerName string,
	profileByBinary podlockv1alpha1.ProfileByBinary,
) error {
	landlockProfilePath := landlockProfilePathOnHost(podID, containerName)

	p.Logger.Debug(
		"writing landlock profile to host filesystem",
		slog.String("pod ID", podID),
		slog.String("container name", containerName),
		slog.String("landlock profile path", landlockProfilePath),
	)

	// Create the directory for the landlock profile
	if err := os.MkdirAll(
		filepath.Dir(landlockProfilePath),
		0o750,
	); err != nil {
		return fmt.Errorf(
			"failed to create runtime dir for landlock profile '%s': %w",
			landlockProfilePath,
			err,
		)
	}

	// Write the landlock profile to the host filesystem
	//nolint:gosec // landlock profile file must be readable by any user inside the container
	f, err := os.OpenFile(
		landlockProfilePath,
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		0o644,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to create landlock profile file '%s': %w",
			landlockProfilePath,
			err,
		)
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(profileByBinary); err != nil {
		return fmt.Errorf(
			"failed to write landlock profile JSON to file '%s': %w",
			landlockProfilePath,
			err,
		)
	}

	return nil
}
