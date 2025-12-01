package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"syscall"

	"github.com/landlock-lsm/go-landlock/landlock"

	"github.com/flavio/podlock/internal/seal"
)

func main() {
	cfg, err := parseFlags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing flags: %v\n", err)
		os.Exit(1)
	}

	logger := setupLogger(cfg.logLevel, cfg.logFormat)
	logger.Debug("Starting seal command", slog.Any("config", cfg))

	profile, err := cfg.buildProfile()
	if err != nil {
		logger.Error("Could not build profile", slog.Any("error", err))
		os.Exit(1)
	}

	// Build rules defined inside of the profile
	rules := seal.ProfileToLandlockRules(profile, logger)

	ctx := context.Background()

	// Build rules for the binary to run
	binaryRules, err := seal.RulesForBinaryToRun(ctx, cfg.binaryToRun, cfg.addLinkedLibraries, logger, seal.DiscoverLinkedLibraries)
	if err != nil {
		logger.Error("Could not build Landlock rules for the binary to run", slog.Any("error", err))
		os.Exit(1)
	}
	rules = append(rules, binaryRules...)
	seal.DebugRules(rules, *logger)

	// Apply Landlock rules
	if err = landlock.V3.RestrictPaths(rules...); err != nil {
		logger.Error("Could not enable Landlock", slog.Any("error", err))
		os.Exit(1)
	}
	logger.Info("landlock profile applied")

	newEnv := sealedProcessEnv()
	args := append([]string{cfg.binary}, cfg.binaryArgs...)

	logger.Debug("About to start sealed process",
		slog.String("binary", cfg.binary),
		slog.Any("args", cfg.binaryArgs),
		slog.Any("env", newEnv),
	)

	//nolint: gosec // We really need to pass all the args we got from the user to Exec
	err = syscall.Exec(cfg.binaryToRun, args, newEnv)
	if err != nil {
		logger.Error("Could not execve the target binary",
			slog.Any("error", err),
			slog.String("binary", cfg.binary),
			slog.Any("args", args),
			slog.Any("env", newEnv),
		)
		os.Exit(1)
	}

	// This point should never be reached because syscall.Exec replaces the current process.
	panic("execve: unexpected return")
}
