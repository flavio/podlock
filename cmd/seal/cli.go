package main

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/lmittmann/tint"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/flavio/podlock/internal/cmdutil"
	"github.com/flavio/podlock/internal/nri"
	"github.com/flavio/podlock/internal/seal"
)

// StringSetFlag is a flag.Value implementation for sets of strings.
type StringSetFlag struct {
	Values sets.Set[string]
}

func (ssf *StringSetFlag) String() string {
	if ssf.Values == nil {
		return ""
	}
	return fmt.Sprintf("%v", ssf.Values.UnsortedList())
}

func (ssf *StringSetFlag) Set(value string) error {
	if ssf.Values == nil {
		ssf.Values = sets.New[string]()
	}
	ssf.Values.Insert(value)
	return nil
}

// setupLogger initializes the logger based on the provided log level.
func setupLogger(logLevel string, logFormat LogFormat) *slog.Logger {
	slogLevel, err := cmdutil.ParseLogLevel(logLevel)
	if err != nil {
		//nolint:sloglint // Use the global logger since the logger is not yet initialized
		slog.Error(
			"error initializing the logger",
			"error",
			err,
		)
		os.Exit(1)
	}

	switch logFormat {
	case LogFormatText:
		slogHandler := tint.NewHandler(os.Stdout,
			&tint.Options{
				Level:      slog.LevelDebug,
				TimeFormat: time.Kitchen,
			})
		return slog.New(slogHandler).With("component", "seal")
	case LogFormatJSON:
		fallthrough
	default:
		opts := slog.HandlerOptions{
			Level: slogLevel,
		}

		slogHandler := slog.NewJSONHandler(os.Stdout, &opts)
		return slog.New(slogHandler).With("component", "seal")
	}
}

// parseFlags parses command-line flags and determines whether to run in
// native 'seal' mode or wrapper mode.
// It returns the parsed configuration.
func parseFlags() (*config, error) {
	if filepath.Base(os.Args[0]) == "seal" {
		return nativeMode(os.Args[1:])
	}

	binary, err := filepath.Abs(os.Args[0])
	if err != nil {
		return nil, fmt.Errorf("could not determine absolute path of binary '%s': %w", os.Args[0], err)
	}
	binaryArgs := os.Args[1:]

	return wrapperMoode(binary, binaryArgs)
}

// wrapperMoode is used when `seal` is invoked with a different name,
// e.g., via a symlink or hardlink.
func wrapperMoode(binary string, binaryArgs []string) (*config, error) {
	profilePath := os.Getenv(seal.ProfileEnvVar)
	if profilePath == "" {
		profilePath = nri.ContainerProfilePathInsideContainer()
	}

	logLevel := os.Getenv(seal.LogLevelEnvVar)
	if logLevel == "" {
		logLevel = slog.LevelInfo.String()
	}

	logFormat := os.Getenv(seal.LogFormatEnvVar)
	if logFormat == "" {
		logFormat = string(LogFormatJSON)
	}

	addLinkedLibraries := os.Getenv(seal.AddLinkedLibrariesEnvVar) != ""

	binaryToRun := nri.SwappedBinaryPathInsideContainer(binary)

	return &config{
		profilePath:        profilePath,
		logLevel:           logLevel,
		logFormat:          LogFormat(logFormat),
		binary:             binary,
		binaryToRun:        binaryToRun,
		binaryArgs:         binaryArgs,
		addLinkedLibraries: addLinkedLibraries,
	}, nil
}

// nativeMode is used when `seal` is invoked directly.
//
//nolint:funlen // The function is long because it includes also the inline docs.
func nativeMode(args []string) (*config, error) {
	var (
		profilePath        string
		logLevel           string
		logFormat          LogFormat
		roFlag             StringSetFlag
		rxFlag             StringSetFlag
		rwFlag             StringSetFlag
		rwxFlag            StringSetFlag
		binary             string
		binaryArgs         []string
		addLinkedLibraries bool
	)

	// Distinguish between flag arguments of `seal` and the binary (plus its args)
	// to run by looking for the `--` separator.
	sepIdx := -1
	for i, arg := range args {
		if arg == "--" {
			sepIdx = i
			break
		}
	}

	var flagArgs []string
	if sepIdx >= 0 {
		flagArgs = args[:sepIdx]
		if len(args) > sepIdx+1 {
			binary = args[sepIdx+1]
			binaryArgs = args[sepIdx+2:]
		}
	} else {
		flagArgs = args
	}

	flagSet := flag.NewFlagSet("seal", flag.ContinueOnError)
	flagSet.Usage = func() {
		fmt.Fprintf(flagSet.Output(), `Usage: seal [options] -- <binary> [args...]

Options:
`)
		flagSet.PrintDefaults()
		fmt.Fprintf(flagSet.Output(), `
The -- separator is required to distinguish between options for 'seal' and the binary to execute.
Everything after -- is treated as the binary to run and its arguments.

Example:
  seal -ro /etc -rw /tmp -- cp -r /etc/default /tmp/default
`)
	}
	flagSet.Var(&roFlag, "ro", "read-only paths")
	flagSet.Var(&rxFlag, "rx", "read-exec path")
	flagSet.Var(&rwFlag, "rw", "read-write paths")
	flagSet.Var(&rwxFlag, "rwx", "read-write-exec paths")
	flagSet.StringVar(&logLevel, "log-level", "info", "log level: debug, info, warn, error")
	flagSet.Var((*LogFormatFlag)(&logFormat), "log-format", "Log format: json or text.")
	flagSet.BoolVar(&addLinkedLibraries, "ldd", false, "Automatically add linked libraries of the target binary to the profile.")

	if err := flagSet.Parse(flagArgs); err != nil {
		return nil, fmt.Errorf("could not parse flags: %w", err)
	}

	roPaths := roFlag.Values.UnsortedList()
	rxPaths := rxFlag.Values.UnsortedList()
	rwPaths := rwFlag.Values.UnsortedList()
	rwxPaths := rwxFlag.Values.UnsortedList()

	// Override with environment variables if set
	if profilePathEnv := os.Getenv(seal.ProfileEnvVar); profilePathEnv != "" {
		profilePath = profilePathEnv
	}
	if logLevelEnv := os.Getenv(seal.LogLevelEnvVar); logLevelEnv != "" {
		logLevel = logLevelEnv
	}
	if addLinkedLibrariesEnv := os.Getenv(seal.AddLinkedLibrariesEnvVar); addLinkedLibrariesEnv != "" {
		addLinkedLibraries = true
	}
	if logFormatEnv := os.Getenv(seal.LogFormatEnvVar); logFormatEnv != "" {
		logFormat = LogFormat(logFormatEnv)
	}

	if (len(roPaths) > 0 || len(rxPaths) > 0 || len(rwPaths) > 0 || len(rwxPaths) > 0) && profilePath != "" {
		return nil, errors.New("cannot use --profile together with --ro, --rx, --rw, or --rwx")
	}

	if binary == "" {
		return nil, errors.New("no binary specified to run; use -- to separate flags and binary")
	}

	binaryAbsolutePath, err := resolveBinaryPath(binary)
	if err != nil {
		return nil, err
	}

	return &config{
		profilePath:        profilePath,
		logLevel:           logLevel,
		logFormat:          logFormat,
		roPaths:            roPaths,
		rxPaths:            rxPaths,
		rwPaths:            rwPaths,
		rwxPaths:           rwxPaths,
		binary:             binaryAbsolutePath,
		binaryToRun:        binaryAbsolutePath,
		binaryArgs:         binaryArgs,
		addLinkedLibraries: addLinkedLibraries,
	}, nil
}

// resolveBinaryPath resolves the absolute path of the binary to run.
func resolveBinaryPath(binary string) (string, error) {
	// If the binary is just a name, look it up in PATH
	if filepath.Base(binary) == binary {
		binaryPath, err := exec.LookPath(binary)
		if err != nil {
			return "", fmt.Errorf("could not find binary in PATH: %w", err)
		}

		return binaryPath, nil
	}

	// If the binary is a local path, return its absolute path
	if filepath.IsLocal(binary) {
		absPath, err := filepath.Abs(binary)
		if err != nil {
			return "", fmt.Errorf("could not determine absolute path of binary '%s': %w", binary, err)
		}
		return absPath, nil
	}

	// Otherwise, return the cleaned path. We need to clean the path because
	// the profile file contains absolute paths.
	return filepath.Clean(binary), nil
}
