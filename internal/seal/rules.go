package seal

import (
	"context"
	"log/slog"
	"os"

	"github.com/landlock-lsm/go-landlock/landlock"

	podlockv1alpha1 "github.com/flavio/podlock/api/v1alpha1"
	ll "github.com/landlock-lsm/go-landlock/landlock/syscall"
)

const (
	accessFileR   landlock.AccessFSSet = ll.AccessFSReadFile
	accessFileRX  landlock.AccessFSSet = ll.AccessFSExecute | ll.AccessFSReadFile
	accessFileRW  landlock.AccessFSSet = accessFileR | ll.AccessFSWriteFile | ll.AccessFSTruncate
	accessFileRWX landlock.AccessFSSet = accessFileRX | accessFileRW

	accessDirR   landlock.AccessFSSet = ll.AccessFSReadDir | accessFileR
	accessDirRX  landlock.AccessFSSet = accessFileRX | accessDirR
	accessDirRW  landlock.AccessFSSet = accessDirR | accessFileRW | ll.AccessFSRemoveDir | ll.AccessFSRemoveFile | ll.AccessFSMakeChar | ll.AccessFSMakeDir | ll.AccessFSMakeReg | ll.AccessFSMakeSock | ll.AccessFSMakeFifo | ll.AccessFSMakeBlock | ll.AccessFSMakeSym | ll.AccessFSRefer
	accessDirRWX landlock.AccessFSSet = accessDirRX | accessDirRW
)

func ProfileToLandlockRules(profile *podlockv1alpha1.Profile, logger *slog.Logger) []landlock.Rule {
	var rules []landlock.Rule

	rules = append(rules, processPaths(profile.ReadOnly, accessDirR, accessFileR, logger)...)
	rules = append(rules, processPaths(profile.ReadWrite, accessDirRW, accessFileRW, logger)...)
	rules = append(rules, processPaths(profile.ReadExec, accessDirRX, accessFileRX, logger)...)
	rules = append(rules, processPaths(profile.ReadWriteExec, accessDirRWX, accessFileRWX, logger)...)

	return rules
}

// LinkedLibsFunc defines a function type for discovering linked libraries of a binary.
type LinkedLibsFunc func(ctx context.Context, binaryPath string, logger *slog.Logger) ([]string, error)

// RulesForBinaryToRun generates Landlock rules for the given binary.
// When addLinkedLibraries is true, it also includes rules for the binary's linked libraries.
// The linked libraries are discovered using the provided discoverLinkedLibsFn function
// passed as an argument.
func RulesForBinaryToRun(
	ctx context.Context,
	binaryPath string,
	addLinkedLibraries bool,
	logger *slog.Logger,
	discoverLinkedLibsFn LinkedLibsFunc,
) ([]landlock.Rule, error) {
	files := []string{binaryPath}

	if addLinkedLibraries {
		linkedLibs, err := discoverLinkedLibsFn(ctx, binaryPath, logger)
		if err != nil {
			return nil, err
		}

		logger.DebugContext(ctx, "Detected linked libraries", slog.Any("libraries", linkedLibs))

		files = append(files, linkedLibs...)
	}

	return processPaths(files, accessDirRX, accessFileRX, logger), nil
}

func processPaths(
	paths []string,
	dirAccessMode landlock.AccessFSSet,
	fileAccessMode landlock.AccessFSSet,
	logger *slog.Logger,
) []landlock.Rule {
	var rules []landlock.Rule
	var files []string
	var dirs []string

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			logger.Warn("unable to stat entry", "path", path, "error", err)
			continue
		}
		if info.IsDir() {
			dirs = append(dirs, path)
		} else {
			files = append(files, path)
		}
	}

	if len(files) > 0 {
		rules = append(rules, landlock.PathAccess(
			fileAccessMode,
			files...,
		))
	}

	if len(dirs) > 0 {
		rules = append(rules, landlock.PathAccess(
			dirAccessMode,
			dirs...,
		))
	}

	return rules
}

func DebugRules(rules []landlock.Rule, logger slog.Logger) {
	for _, r := range rules {
		if fsr, ok := r.(landlock.FSRule); ok {
			logger.Debug("Landlock rule", slog.String("rule", fsr.String()))
		} else {
			logger.Error("Landlock rule (not FSRule)", slog.Any("rule", r))
		}
	}
}
