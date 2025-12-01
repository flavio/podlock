package main

import (
	"os"
	"testing"

	"github.com/flavio/podlock/internal/seal"
	"github.com/stretchr/testify/assert"
)

func TestSealedProcessEnv(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want map[string]string
	}{
		{
			name: "filters seal env vars",
			env: map[string]string{
				"FOO":               "bar",
				"BAR":               "baz",
				seal.ProfileEnvVar:  "/tmp/profile",
				seal.LogLevelEnvVar: "debug",
			},
			want: map[string]string{
				"FOO": "bar",
				"BAR": "baz",
			},
		},
		{
			name: "keeps non-seal env vars",
			env: map[string]string{
				"HELLO": "world",
				"PATH":  "/usr/bin",
			},
			want: map[string]string{
				"HELLO": "world",
				"PATH":  "/usr/bin",
			},
		},
		{
			name: "all seal env vars",
			env: map[string]string{
				seal.ProfileEnvVar:            "/tmp/profile",
				seal.LogLevelEnvVar:           "debug",
				seal.SealEnvVarPrefix + "FOO": "bar",
				seal.SealEnvVarPrefix + "BAR": "baz",
			},
			want: map[string]string{},
		},
	}

	origEnv := os.Environ()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear and set environment
			os.Clearenv()
			for k, v := range tt.env {
				os.Setenv(k, v) //nolint:usetesting // we need more control than t.Setenv here
			}

			gotSlice := sealedProcessEnv()
			got := map[string]string{}
			for _, kv := range gotSlice {
				parts := splitOnce(kv, "=")
				if len(parts) == 2 {
					got[parts[0]] = parts[1]
				}
			}
			assert.Equal(t, tt.want, got)
		})
	}
	// Restore original environment
	os.Clearenv()
	for _, kv := range origEnv {
		parts := splitOnce(kv, "=")
		if len(parts) == 2 {
			//nolint:usetesting // we need more control than t.Setenv here
			os.Setenv(parts[0], parts[1])
		}
	}
}

// splitOnce splits a string into two parts at the first "=".
func splitOnce(s, sep string) []string {
	i := -1
	for idx := range s {
		if s[idx:idx+1] == sep {
			i = idx
			break
		}
	}
	if i == -1 {
		return []string{s}
	}
	return []string{s[:i], s[i+1:]}
}
