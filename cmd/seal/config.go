package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	podlockv1alpha1 "github.com/flavio/podlock/api/v1alpha1"
)

type LogFormat string

const (
	// LogFormatText represents plain text log format
	LogFormatJSON LogFormat = "json"

	// LogFormatText represents plain text log format
	LogFormatText LogFormat = "text"
)

// LogFormatFlag implements flag.Value for LogFormat
type LogFormatFlag LogFormat

func (f *LogFormatFlag) String() string {
	return string(*f)
}

func (f *LogFormatFlag) Set(value string) error {
	switch value {
	case string(LogFormatJSON), string(LogFormatText):
		*f = LogFormatFlag(value)
		return nil
	case "":
		*f = LogFormatFlag(LogFormatJSON)
		return nil
	default:
		return fmt.Errorf("invalid log format: %s", value)
	}
}

type config struct {
	addLinkedLibraries bool
	profilePath        string
	binary             string
	binaryToRun        string
	binaryArgs         []string
	logLevel           string
	logFormat          LogFormat
	roPaths            []string
	rxPaths            []string
	rwPaths            []string
	rwxPaths           []string
}

// buildProfile builds the podlock profile based on the config.
func (c *config) buildProfile() (*podlockv1alpha1.Profile, error) {
	if c.profilePath != "" {
		return profileFromPath(c.profilePath, c.binary)
	}

	return &podlockv1alpha1.Profile{
		ReadOnly:      c.roPaths,
		ReadExec:      c.rxPaths,
		ReadWrite:     c.rwPaths,
		ReadWriteExec: c.rwxPaths,
	}, nil
}

// profileFromPath reads the profile file at the given path and returns
// the profile for the specified binary.
func profileFromPath(path, binary string) (*podlockv1alpha1.Profile, error) {
	profilesByBinary := &podlockv1alpha1.ProfileByBinary{}

	// Read and unmarshal the JSON profile
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open profile '%s': %w", path, err)
	}
	defer file.Close()

	if err := json.NewDecoder(file).Decode(profilesByBinary); err != nil {
		return nil, fmt.Errorf("cannot unmarshal contents of profile file '%s': %w", path, err)
	}

	profile, found := (*profilesByBinary)[binary]
	if !found {
		return nil, fmt.Errorf("cannot find profile for '%s'", binary)
	}

	return &profile, nil
}

// LogValue implements slog.LogValuer for config
func (c *config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Bool("addLinkedLibraries", c.addLinkedLibraries),
		slog.String("profilePath", c.profilePath),
		slog.String("binary", c.binary),
		slog.String("binaryToRun", c.binaryToRun),
		slog.Any("binaryArgs", c.binaryArgs),
		slog.String("logLevel", c.logLevel),
		slog.String("logFormat", string(c.logFormat)),
		slog.Any("roPaths", c.roPaths),
		slog.Any("rxPaths", c.rxPaths),
		slog.Any("rwPaths", c.rwPaths),
		slog.Any("rwxPaths", c.rwxPaths),
	)
}
