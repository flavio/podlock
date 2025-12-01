package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/flavio/podlock/internal/nri"
	"github.com/flavio/podlock/internal/seal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveBinaryPath(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "mybin")
	if err := os.WriteFile(testFile, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatalf("failed to create test binary: %v", err)
	}

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", tmpDir+":"+origPath)
	t.Chdir(tmpDir)

	tests := []struct {
		name     string
		input    string
		wantPath string
		wantErr  bool
	}{
		{
			name:     "absolute path",
			input:    "/usr/local/bin/mybin",
			wantPath: "/usr/local/bin/mybin",
			wantErr:  false,
		},
		{
			name:     "relative path",
			input:    "./mybin",
			wantPath: filepath.Join(tmpDir, "mybin"),
			wantErr:  false,
		},
		{
			name:     "in PATH",
			input:    "mybin",
			wantPath: filepath.Join(tmpDir, "mybin"),
			wantErr:  false,
		},
		{
			name:    "not found",
			input:   "doesnotexist",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveBinaryPath(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("resolveBinaryPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.wantPath {
				t.Errorf("resolveBinaryPath() = %v, want %v", got, tt.wantPath)
			}
		})
	}
}

func TestNativeMode(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantCfg   *config
		wantError bool
	}{
		{
			name: "only binary and args",
			args: []string{"--", "/bin/echo", "hello", "world"},
			wantCfg: &config{
				binary:     "/bin/echo",
				binaryArgs: []string{"hello", "world"},
				rxPaths:    []string{},
				roPaths:    []string{},
				rwPaths:    []string{},
				rwxPaths:   []string{},
			},
			wantError: false,
		},
		{
			name: "with flags and binary",
			args: []string{"-ro", "/etc", "--", "/bin/ls", "-l", "/tmp"},
			wantCfg: &config{
				binary:     "/bin/ls",
				binaryArgs: []string{"-l", "/tmp"},
				rxPaths:    []string{},
				roPaths:    []string{"/etc"},
				rwPaths:    []string{},
				rwxPaths:   []string{},
			},
			wantError: false,
		},
		{
			name:      "missing binary",
			args:      []string{"-ro", "/etc"},
			wantCfg:   nil,
			wantError: true,
		},
		{
			name: "repeated flags",
			args: []string{"-ro", "/etc", "-rw", "/tmp", "-ro", "/var", "--", "/bin/cat", "/etc/passwd"},
			wantCfg: &config{
				binary:     "/bin/cat",
				binaryArgs: []string{"/etc/passwd"},
				rxPaths:    []string{},
				roPaths:    []string{"/etc", "/var"},
				rwPaths:    []string{"/tmp"},
				rwxPaths:   []string{},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := nativeMode(tt.args)
			if tt.wantError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantCfg.binary, cfg.binary)
			assert.ElementsMatch(t, tt.wantCfg.binaryArgs, cfg.binaryArgs)
			assert.ElementsMatch(t, tt.wantCfg.roPaths, cfg.roPaths)
			assert.ElementsMatch(t, tt.wantCfg.rwPaths, cfg.rwPaths)
		})
	}
}

func TestWrapperMoode(t *testing.T) {
	tests := []struct {
		name              string
		envProfile        string
		envLogLevel       string
		envLdd            string
		binary            string
		binaryArgs        []string
		wantProfilePath   string
		wantLogLevel      string
		wantAddLinkedLibs bool
	}{
		{
			name:              "no envs set",
			envProfile:        "",
			envLogLevel:       "",
			envLdd:            "",
			binary:            "/bin/ls",
			binaryArgs:        []string{"-l"},
			wantProfilePath:   nri.ContainerProfilePathInsideContainer(),
			wantLogLevel:      "INFO",
			wantAddLinkedLibs: false,
		},
		{
			name:              "all envs set",
			envProfile:        "/tmp/profile.yaml",
			envLogLevel:       "DEBUG",
			envLdd:            "1",
			binary:            "/bin/echo",
			binaryArgs:        []string{"hello"},
			wantProfilePath:   "/tmp/profile.yaml",
			wantLogLevel:      "DEBUG",
			wantAddLinkedLibs: true,
		},
		{
			name:              "only profile set",
			envProfile:        "/etc/profile.yaml",
			envLogLevel:       "",
			envLdd:            "",
			binary:            "/bin/cat",
			binaryArgs:        []string{"/etc/passwd"},
			wantProfilePath:   "/etc/profile.yaml",
			wantLogLevel:      "INFO",
			wantAddLinkedLibs: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(seal.ProfileEnvVar, tt.envProfile)
			t.Setenv(seal.LogLevelEnvVar, tt.envLogLevel)
			t.Setenv(seal.AddLinkedLibrariesEnvVar, tt.envLdd)

			cfg, err := wrapperMoode(tt.binary, tt.binaryArgs)
			require.NoError(t, err)
			assert.Equal(t, tt.wantProfilePath, cfg.profilePath)
			assert.Equal(t, tt.wantLogLevel, cfg.logLevel)
			assert.Equal(t, tt.wantAddLinkedLibs, cfg.addLinkedLibraries)
			assert.Equal(t, tt.binary, cfg.binary)
			assert.Equal(t, tt.binaryArgs, cfg.binaryArgs)
		})
	}
}
