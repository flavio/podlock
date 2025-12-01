package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"golang.org/x/sys/unix"

	"github.com/flavio/podlock/internal/nri"
)

// swap-oci-hook swaps the target file with the backup file using move_mount.
// It then replaces the target file with the `seal` binary that has already
// been injected into the container root filesystem by the podlock NRI plugin.
//
// The target file is backuped to the backup file location.
// In this way, the `seal` binary can run the original binary applying a
// Landlock profile.
//
// This hook is run at the OCI Prestart stage, before the container
// process is started.
// The working directory of the hook is set to be the container root filesystem
// by runC.
//
// The actual swap happens by doing an overmount of the target file with the
// `seal` binary using the move_mount syscall.
//
// Note, this program uses the move_mount syscall, which requires Linux kernel
// 5.2 or higher.
func main() {
	target := flag.String("target", "", "Path to the target file")
	backup := flag.String("backup", "", "Path to the backup file")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Validate flags
	if *target == "" || *backup == "" {
		fmt.Fprintf(os.Stderr, "usage: %s -target <target> -backup <backup>\n", os.Args[0])
		os.Exit(1)
	}

	// Change root to the container root filesystem, this is required to setup
	// all the overmounts correctly.
	if err := unix.Chroot("."); err != nil {
		logger.Error("failed to chroot to container root", slog.Any("error", err))
		os.Exit(1)
	}

	if err := performOverMounts(*target, *backup); err != nil {
		logger.Error("failed to perform swap", slog.Any("error", err))
		os.Exit(1)
	}
}

func performOverMounts(target, backup string) error {
	sealFd, err := unix.OpenTree(
		unix.AT_FDCWD,
		nri.SealBinaryPathContainer(),
		unix.OPEN_TREE_CLONE,
	)
	if err != nil {
		return fmt.Errorf("open_tree failed for seal binary: %w", err)
	}
	defer unix.Close(sealFd)

	targetFd, err := unix.OpenTree(
		unix.AT_FDCWD,
		target,
		unix.OPEN_TREE_CLONE,
	)
	if err != nil {
		return fmt.Errorf("open_tree failed for target: %w", err)
	}
	defer unix.Close(targetFd)

	// mount the target to the backup location
	err = unix.MoveMount(
		targetFd,
		"",
		unix.AT_FDCWD,
		backup,
		unix.MOVE_MOUNT_F_EMPTY_PATH,
	)
	if err != nil {
		return fmt.Errorf("move_mount failed for backup %w", err)
	}

	// mount the seal binary to the target location
	err = unix.MoveMount(
		sealFd,
		"",
		unix.AT_FDCWD,
		target,
		unix.MOVE_MOUNT_F_EMPTY_PATH,
	)
	if err != nil {
		return fmt.Errorf("move_mount failed for target: %w", err)
	}

	return nil
}
