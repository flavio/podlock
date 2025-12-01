package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	podlockv1alpha1 "github.com/flavio/podlock/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogFormatFlag_Set(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantValue LogFormatFlag
		wantErr   bool
	}{
		{
			name:      "set json",
			input:     "json",
			wantValue: LogFormatFlag(LogFormatJSON),
			wantErr:   false,
		},
		{
			name:      "set text",
			input:     "text",
			wantValue: LogFormatFlag(LogFormatText),
			wantErr:   false,
		},
		{
			name:      "invalid value",
			input:     "xml",
			wantValue: "",
			wantErr:   true,
		},
		{
			name:      "empty value",
			input:     "",
			wantValue: LogFormatFlag(LogFormatJSON),
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var f LogFormatFlag
			err := f.Set(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantValue, f)
			}
		})
	}
}

func TestProfileFromPath(t *testing.T) {
	tmpDir := t.TempDir()
	profileFile := filepath.Join(tmpDir, "profile.json")

	profiles := podlockv1alpha1.ProfileByBinary{
		"/bin/ls": podlockv1alpha1.Profile{
			ReadOnly:      []string{"/etc"},
			ReadExec:      []string{"/usr/bin"},
			ReadWrite:     []string{"/tmp"},
			ReadWriteExec: []string{"/var"},
		},
		"/bin/cat": podlockv1alpha1.Profile{
			ReadOnly: []string{"/foo"},
		},
	}

	profileData, err := json.Marshal(profiles)
	require.NoError(t, err)
	err = os.WriteFile(profileFile, profileData, 0o644)
	require.NoError(t, err)

	tests := []struct {
		name    string
		path    string
		binary  string
		want    *podlockv1alpha1.Profile
		wantErr bool
	}{
		{
			name:   "existing binary",
			path:   profileFile,
			binary: "/bin/ls",
			want: &podlockv1alpha1.Profile{
				ReadOnly:      []string{"/etc"},
				ReadExec:      []string{"/usr/bin"},
				ReadWrite:     []string{"/tmp"},
				ReadWriteExec: []string{"/var"},
			},
			wantErr: false,
		},
		{
			name:    "missing binary",
			path:    profileFile,
			binary:  "/bin/bash",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "missing file",
			path:    filepath.Join(tmpDir, "doesnotexist.json"),
			binary:  "/bin/ls",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := profileFromPath(tt.path, tt.binary)
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
