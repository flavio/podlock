package seal

import (
	"context"
	"debug/elf"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	FileTypeUnknown = "unknown"
	FileTypeELF     = "elf"
	FileTypeScript  = "script"
)

// FirstGlobber is an interface for finding the first matching file from a set of glob patterns.
type FirstGlobber struct {
	// searchPathPrefix is an optional prefix to prepend to each pattern before globbing.
	// This is useful for testing with a custom filesystem layout.
	searchPathPrefix string
}

// Find returns the first file that matches any of the provided glob patterns.
func (f *FirstGlobber) Find(patterns []string, logger *slog.Logger) string {
	// Prepend searchPathPrefix if set, this is useful for testing with a custom FS layout
	if f.searchPathPrefix != "" {
		for i, pattern := range patterns {
			patterns[i] = filepath.Join(f.searchPathPrefix, pattern)
		}
	}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err == nil && len(matches) > 0 {
			return matches[0]
		}
		if err != nil {
			logger.Warn("error during globbing", slog.String("pattern", pattern), slog.Any("error", err))
		}
	}
	return ""
}

// DiscoverLinkedLibraries discovers the shared libraries linked to the given binary.
// It relies on the system's dynamic linker/loader to list the linked libraries.
func DiscoverLinkedLibraries(ctx context.Context, binaryPath string, logger *slog.Logger) ([]string, error) {
	logger.DebugContext(ctx, "Discovering linked libraries", slog.String("binary", binaryPath))

	fileType, err := detectFileType(binaryPath)
	if err != nil {
		return nil, fmt.Errorf("could not detect file type for binary '%s': %w", binaryPath, err)
	}

	switch fileType {
	case FileTypeELF:
		// Proceed
	case FileTypeScript:
		logger.InfoContext(ctx, "binary is a script, skipping linked libraries discovery",
			slog.String("binary", binaryPath))
		return nil, nil
	default:
		logger.InfoContext(ctx, "binary file type is unknown, skipping linked libraries discovery",
			slog.String("binary", binaryPath))
		return nil, nil
	}

	elfFile, err := elf.Open(binaryPath)
	if err != nil {
		return nil, fmt.Errorf("could not open ELF binary '%s': %w", binaryPath, err)
	}
	defer elfFile.Close()

	importedLibs, err := elfFile.ImportedLibraries()
	if err != nil {
		return nil, fmt.Errorf("could not get imported libraries for binary '%s': %w", binaryPath, err)
	}

	if len(importedLibs) == 0 {
		logger.DebugContext(ctx, "binary is statically linked, no linked libraries found",
			slog.String("binary", binaryPath))
		return nil, nil
	}

	globber := &FirstGlobber{}
	ldSolver := findLdSoBin(elfFile.Class, globber, logger)
	var cmd *exec.Cmd
	if strings.Contains(ldSolver, "musl") {
		// Trick musl loader into ldd mode by setting argv[0] to "ldd"
		cmd = exec.CommandContext(ctx, ldSolver, binaryPath)
		cmd.Args[0] = "ldd"
	} else {
		cmd = exec.CommandContext(ctx, ldSolver, "--list", binaryPath)
	}
	logger.DebugContext(ctx, "Running loader to discover linked libraries",
		slog.String("loader", ldSolver),
		slog.Any("args", cmd.Args),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to run %s: %w\noutput: %s", ldSolver, err, string(output))
	}

	return parseLoaderOutput(string(output)), nil
}

// findLdSoBin returns the path to the dynamic linker/loader based on the ELF class.
// It tries common paths and patterns to locate the appropriate loader.
// If none is found, it returns an empty string.
func findLdSoBin(elfClass elf.Class, globber *FirstGlobber, logger *slog.Logger) string {
	var patterns []string

	switch elfClass {
	case elf.ELFCLASS32:
		patterns = append(patterns, "/lib32/ld-*.so.*")
	case elf.ELFCLASS64:
		patterns = append(patterns, "/lib64/ld-*.so.*")
	case elf.ELFCLASSNONE:
		logger.Info("unknown ELF class", slog.Int("elfClass", int(elfClass)))
	}

	// Add generic pattern as fallback. This catch the muls loader as well
	// since it follows the `/lib/ld-musl-*.so.*` pattern.
	patterns = append(patterns, "/lib/ld-*.so.*")

	return globber.Find(patterns, logger)
}

// parseLoaderOutput parses the output of the dynamic loader to extract
// the paths of linked libraries.
func parseLoaderOutput(output string) []string {
	var libs []string
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Look for lines with =>
		if idx := strings.Index(line, "=>"); idx != -1 {
			// Format: <libname> => <fullpath> (<address>)
			rest := strings.TrimSpace(line[idx+2:])
			parts := strings.Fields(rest)
			if len(parts) > 0 && strings.HasPrefix(parts[0], "/") {
				libs = append(libs, parts[0])
			}
		} else if fields := strings.Fields(line); len(fields) > 0 && strings.HasPrefix(fields[0], "/") {
			// Format: /lib/ld-musl-x86_64.so.1 (0x...)
			libs = append(libs, fields[0])
		}
	}
	return libs
}

// detectFileType reads the magic numbers of the file to determine its type.
// Currently it can identify ELF binaries and shebang scripts.
func detectFileType(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return FileTypeUnknown, fmt.Errorf("could not open file '%s': %w", path, err)
	}
	defer f.Close()

	magic := make([]byte, 4)
	n, err := f.Read(magic)
	if err != nil || n < 2 {
		return FileTypeUnknown, fmt.Errorf("could not read magic bytes from file '%s': %w", path, err)
	}

	switch {
	case n >= 4 && magic[0] == 0x7F && magic[1] == 'E' && magic[2] == 'L' && magic[3] == 'F':
		return FileTypeELF, nil
	case magic[0] == '#' && magic[1] == '!':
		return FileTypeScript, nil
	default:
		return FileTypeUnknown, nil
	}
}
