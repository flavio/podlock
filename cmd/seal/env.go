package main

import (
	"os"
	"strings"

	"github.com/flavio/podlock/internal/seal"
)

// sealedProcessEnv returns a copy of strings representing the environment,
// in the form "key=value".
//
// The returned slice does not contain the environment variables used to configure
// seal itself.
func sealedProcessEnv() []string {
	var filtered []string
	for _, kv := range os.Environ() {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		if strings.HasPrefix(key, seal.SealEnvVarPrefix) {
			continue
		}
		filtered = append(filtered, kv)
	}
	return filtered
}
