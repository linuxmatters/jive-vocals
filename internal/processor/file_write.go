package processor

import (
	"fmt"
	"os"
	"path/filepath"
)

var processorRename = os.Rename

// createSiblingTempPath creates a hidden, same-directory temp path whose basename
// includes the marker and ends in .tmp.flac; tests pin the exact naming pattern.
func createSiblingTempPath(targetPath, marker string) (string, error) {
	return createSiblingTempPathSuffix(targetPath, marker, ".tmp.flac")
}

// createSiblingStatsPath creates a hidden, same-directory temp path whose basename
// includes the marker and ends in .tmp.json, for per-graph loudnorm stats readback.
func createSiblingStatsPath(targetPath, marker string) (string, error) {
	return createSiblingTempPathSuffix(targetPath, marker, ".tmp.json")
}

// createSiblingTempPathSuffix creates a hidden, same-directory temp path whose
// basename is .<marker>-*<suffix>. The marker must not contain a path separator.
func createSiblingTempPathSuffix(targetPath, marker, suffix string) (string, error) {
	if marker == "" || filepath.Base(marker) != marker {
		return "", fmt.Errorf("invalid temp marker: %q", marker)
	}

	tempFile, err := os.CreateTemp(filepath.Dir(targetPath), "."+marker+"-*"+suffix)
	if err != nil {
		return "", fmt.Errorf("failed to create temporary output next to %s: %w", targetPath, err)
	}

	tempPath := tempFile.Name()
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("failed to close temporary output %s: %w", tempPath, err)
	}

	return tempPath, nil
}

// publishOutput moves a same-directory temp file to dst, atomically overwriting
// any existing destination (os.Rename replaces dst on the same filesystem), so a
// re-run replaces the prior output rather than failing.
func publishOutput(src, dst string) error {
	if err := processorRename(src, dst); err != nil {
		return fmt.Errorf("failed to publish output to %s: %w", dst, err)
	}

	return nil
}
