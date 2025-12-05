package seal

import (
	"context"
	"debug/elf"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseLoaderOutput(t *testing.T) {
	muslOutput := `
        /lib/ld-musl-x86_64.so.1 (0x7f1c03cbe000)
        libcurl.so.4 => /usr/lib/libcurl.so.4 (0x7f1c03bd0000)
        libz.so.1 => /usr/lib/libz.so.1 (0x7f1c03bb5000)
        libc.musl-x86_64.so.1 => /lib/ld-musl-x86_64.so.1 (0x7f1c03cbe000)
        libcares.so.2 => /usr/lib/libcares.so.2 (0x7f1c03b7a000)
        libnghttp2.so.14 => /usr/lib/libnghttp2.so.14 (0x7f1c03b56000)
        libidn2.so.0 => /usr/lib/libidn2.so.0 (0x7f1c03b25000)
        libpsl.so.5 => /usr/lib/libpsl.so.5 (0x7f1c03b11000)
        libssl.so.3 => /usr/lib/libssl.so.3 (0x7f1c03a4e000)
        libcrypto.so.3 => /usr/lib/libcrypto.so.3 (0x7f1c03602000)
        libzstd.so.1 => /usr/lib/libzstd.so.1 (0x7f1c03552000)
        libbrotlidec.so.1 => /usr/lib/libbrotlidec.so.1 (0x7f1c03543000)
        libunistring.so.5 => /usr/lib/libunistring.so.5 (0x7f1c0339e000)
        libbrotlicommon.so.1 => /usr/lib/libbrotlicommon.so.1 (0x7f1c0337b000)
	`

	ldSoOutput := `
        linux-vdso.so.1 (0x00007f9b4e199000)
        libselinux.so.1 => /lib64/libselinux.so.1 (0x00007f9b4e12b000)
        libacl.so.1 => /lib64/libacl.so.1 (0x00007f9b4e122000)
        libattr.so.1 => /lib64/libattr.so.1 (0x00007f9b4e11a000)
        libc.so.6 => /lib64/libc.so.6 (0x00007f9b4df1e000)
        libpcre2-8.so.0 => /lib64/libpcre2-8.so.0 (0x00007f9b4de66000)
        /lib64/ld-linux-x86-64.so.2 (0x00007f9b4e19b000)
	`

	tests := []struct {
		name   string
		input  string
		expect []string
	}{
		{
			name:  "musl",
			input: muslOutput,
			expect: []string{
				"/lib/ld-musl-x86_64.so.1",
				"/usr/lib/libcurl.so.4",
				"/usr/lib/libz.so.1",
				"/lib/ld-musl-x86_64.so.1",
				"/usr/lib/libcares.so.2",
				"/usr/lib/libnghttp2.so.14",
				"/usr/lib/libidn2.so.0",
				"/usr/lib/libpsl.so.5",
				"/usr/lib/libssl.so.3",
				"/usr/lib/libcrypto.so.3",
				"/usr/lib/libzstd.so.1",
				"/usr/lib/libbrotlidec.so.1",
				"/usr/lib/libunistring.so.5",
				"/usr/lib/libbrotlicommon.so.1",
			},
		},
		{
			name:  "ld-so",
			input: ldSoOutput,
			expect: []string{
				"/lib64/libselinux.so.1",
				"/lib64/libacl.so.1",
				"/lib64/libattr.so.1",
				"/lib64/libc.so.6",
				"/lib64/libpcre2-8.so.0",
				"/lib64/ld-linux-x86-64.so.2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLoaderOutput(tt.input)
			sort.Strings(got)
			sort.Strings(tt.expect)
			assert.Equal(t, tt.expect, got)
		})
	}
}

func TestDiscoverLinkedLibrariesDynamicallyLinkedBin(t *testing.T) {
	// This test relies on the presence of the 'cp' binary in the system PATH.
	// It also assumes it's a dynamically linked binary, which is the case also
	// systems with musl libc.

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	binaryPath, err := exec.LookPath("cp")
	require.NoError(t, err)

	libs, err := DiscoverLinkedLibraries(context.Background(), binaryPath, logger)
	require.NoError(t, err)
	assert.NotEmpty(t, libs, "expected to find linked libraries for 'cp' binary")
}

func TestDiscoverLinkedLibrariesStaticallyLinkedBin(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	binaryPath := "../../bin/seal"

	// Check if the binary exists
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		// Try to build the binary
		cmd := exec.Command("make", "-C", "../../", "seal")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		require.NoError(t, err, "failed to build seal binary with 'make seal'")
	}

	// Confirm the binary exists after build
	_, err := os.Stat(binaryPath)
	require.NoError(t, err, "seal binary not found after build")

	libs, err := DiscoverLinkedLibraries(context.Background(), binaryPath, logger)
	require.NoError(t, err)
	assert.Empty(t, libs, "expected no linked libraries")
}

func TestDiscoverLinkedLibrariesNonELFBin(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	tests := []struct {
		name   string
		binary string
	}{
		{
			name:   "script file",
			binary: "../../test/fixtures/script.sh",
		},
		{
			name:   "non-ELF binary",
			binary: "../../README.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := os.Stat(tt.binary); os.IsNotExist(err) {
				t.Skipf("file not found at %s", tt.binary)
			}

			libs, err := DiscoverLinkedLibraries(context.Background(), tt.binary, logger)
			require.NoError(t, err)
			assert.Empty(t, libs, "expected no linked libraries")
		})
	}
}

func TestDetectFileType(t *testing.T) {
	cpBin, err := exec.LookPath("cp")
	require.NoError(t, err)

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "ELF binary",
			path:     cpBin,
			expected: FileTypeELF,
		},
		{
			name:     "script file",
			path:     "../../test/fixtures/script.sh",
			expected: FileTypeScript,
		},
		{
			name:     "txt file",
			path:     "../../README.md",
			expected: FileTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := os.Stat(tt.path)
			require.NoError(t, err, "test file does not exist: %s", tt.path)

			ftype, err := detectFileType(tt.path)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, ftype)
		})
	}
}

func TestFindLdSoBin(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	tmpDir := t.TempDir()

	ldSo32 := "/lib32/ld-linux.so.2"
	ldSo64 := "/lib64/ld-linux-x86-64.so.2"
	ldSoMusl := "/lib/ld-musl-x86_64.so.1"

	// Create all needed files before running the tests
	allFiles := []string{ldSo32, ldSo64, ldSoMusl}
	for _, f := range allFiles {
		destPath := filepath.Join(tmpDir, f)
		require.NoError(t, os.MkdirAll(filepath.Dir(destPath), 0o755))
		file, err := os.Create(destPath)
		require.NoError(t, err, "failed to create file %s", destPath)
		file.Close()
	}

	tests := []struct {
		name               string
		elfClass           elf.Class
		ldSoBinariesOnDisk []string
		expect             string
	}{
		{
			name:               "glibc32",
			elfClass:           elf.ELFCLASS32,
			ldSoBinariesOnDisk: []string{ldSo32, ldSo64},
			expect:             ldSo32,
		},
		{
			name:               "glibc64",
			elfClass:           elf.ELFCLASS64,
			ldSoBinariesOnDisk: []string{ldSo32, ldSo64},
			expect:             ldSo64,
		},
		{
			name:               "musl",
			elfClass:           elf.Class(0),
			ldSoBinariesOnDisk: []string{ldSoMusl},
			expect:             ldSoMusl,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Remove files not needed for this test case
			for _, f := range allFiles {
				path := filepath.Join(tmpDir, f)
				shouldExist := false
				for _, needed := range tt.ldSoBinariesOnDisk {
					if f == needed {
						shouldExist = true
						break
					}
				}
				if !shouldExist {
					_ = os.Remove(path)
				} else {
					// Ensure file exists
					if _, err := os.Stat(path); os.IsNotExist(err) {
						file, err := os.Create(path)
						require.NoError(t, err)
						file.Close()
					}
				}
			}

			globber := &FirstGlobber{searchPathPrefix: tmpDir}
			got := findLdSoBin(tt.elfClass, globber, logger)
			assert.Equal(t, filepath.Join(tmpDir, tt.expect), got)
		})
	}
}

func TestFindLdGlibc(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	tmpDir := t.TempDir()

	ldSo32 := filepath.Join(tmpDir, "lib32", "ld-linux.so.2")
	ldSo64 := filepath.Join(tmpDir, "lib64", "ld-linux.so.2")

	for _, f := range []string{ldSo32, ldSo64} {
		require.NoError(t, os.MkdirAll(filepath.Dir(f), 0o755))
		file, err := os.Create(f)
		require.NoError(t, err, "failed to create file %s", f)
		file.Close()
	}

	tests := []struct {
		name     string
		elfClass elf.Class
		expect   string
	}{
		{
			name:     "glibc32",
			elfClass: elf.ELFCLASS32,
			expect:   ldSo32,
		},
		{
			name:     "glibc64",
			elfClass: elf.ELFCLASS64,
			expect:   ldSo64,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			globber := &FirstGlobber{searchPathPrefix: tmpDir}
			got := findLdSoBin(tt.elfClass, globber, logger)
			assert.Equal(t, tt.expect, got)
		})
	}
}

func TestFindLdSoBinMusl(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	tmpDir := t.TempDir()

	ldSoMusl := filepath.Join(tmpDir, "/lib", "ld-musl-x86_64.so.1")

	require.NoError(t, os.MkdirAll(filepath.Dir(ldSoMusl), 0o755))
	file, err := os.Create(ldSoMusl)
	require.NoError(t, err, "failed to create file %s", ldSoMusl)
	file.Close()

	tests := []struct {
		name     string
		elfClass elf.Class
		expect   string
	}{
		{
			name:     "musl32",
			elfClass: elf.ELFCLASS32,
			expect:   ldSoMusl,
		},
		{
			name:     "musl64",
			elfClass: elf.ELFCLASS64,
			expect:   ldSoMusl,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			globber := &FirstGlobber{searchPathPrefix: tmpDir}
			got := findLdSoBin(tt.elfClass, globber, logger)
			assert.Equal(t, tt.expect, got)
		})
	}
}
