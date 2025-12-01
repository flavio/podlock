package seal

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/landlock-lsm/go-landlock/landlock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessPaths(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "testfile")
	testDir := filepath.Join(tmpDir, "testdir")
	testDir2 := filepath.Join(tmpDir, "testdir2")
	_ = os.WriteFile(testFile, []byte("data"), 0o644)
	_ = os.Mkdir(testDir, 0o755)
	_ = os.Mkdir(testDir2, 0o755)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	fileAccessMode := accessFileR
	dirAccessMode := accessDirR

	tests := []struct {
		name      string
		paths     []string
		wantRules []landlock.Rule
	}{
		{
			name:      "single file",
			paths:     []string{testFile},
			wantRules: []landlock.Rule{landlock.PathAccess(fileAccessMode, []string{testFile}...)},
		},
		{
			name:      "single dir",
			paths:     []string{testDir},
			wantRules: []landlock.Rule{landlock.PathAccess(dirAccessMode, []string{testDir}...)},
		},
		{
			name:      "multiple entries of same type",
			paths:     []string{testDir, testDir2},
			wantRules: []landlock.Rule{landlock.PathAccess(dirAccessMode, []string{testDir, testDir2}...)},
		},
		{
			name:  "file and dir",
			paths: []string{testFile, testDir},
			wantRules: []landlock.Rule{
				landlock.PathAccess(fileAccessMode, []string{testFile}...),
				landlock.PathAccess(dirAccessMode, []string{testDir}...),
			},
		},
		{
			name:      "nonexistent path",
			paths:     []string{filepath.Join(tmpDir, "doesnotexist")},
			wantRules: []landlock.Rule{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules := processPaths(tt.paths, dirAccessMode, fileAccessMode, logger)
			assert.ElementsMatch(t, tt.wantRules, rules)
		})
	}
}

func TestRulesForBinaryToRun(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	testBin := filepath.Join(tmpDir, "testbin")
	testLib := filepath.Join(tmpDir, "testlib.so")
	_ = os.WriteFile(testBin, []byte("binary"), 0o755)
	_ = os.WriteFile(testLib, []byte("library"), 0o644)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	tests := []struct {
		name               string
		binaryPath         string
		addLinkedLibraries bool
		mockLinkedLibs     []string
		wantRules          []landlock.Rule
	}{
		{
			name:               "library discovery disabled",
			binaryPath:         testBin,
			addLinkedLibraries: false,
			mockLinkedLibs:     nil,
			wantRules: []landlock.Rule{
				landlock.PathAccess(accessFileRX, testBin),
			},
		},
		{
			name:               "with linked libraries",
			binaryPath:         testBin,
			addLinkedLibraries: true,
			mockLinkedLibs:     []string{testLib},
			wantRules: []landlock.Rule{
				landlock.PathAccess(accessFileRX, testBin, testLib),
			},
		},
		{
			name:               "statically linked binary",
			binaryPath:         testBin,
			addLinkedLibraries: true,
			mockLinkedLibs:     []string{},
			wantRules: []landlock.Rule{
				landlock.PathAccess(accessFileRX, testBin),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDiscover := func(_ context.Context, _ string, _ *slog.Logger) ([]string, error) {
				return tt.mockLinkedLibs, nil
			}
			rules, err := RulesForBinaryToRun(ctx, tt.binaryPath, tt.addLinkedLibraries, logger, mockDiscover)
			require.NoError(t, err)
			assert.ElementsMatch(t, tt.wantRules, rules)
		})
	}
}
