// Package internal implements the core functionality for the terraform-provider-docker.
// It provides data sources for retrieving files and logs from Docker containers,
// with security features like path sanitization and input validation.
package internal

import (
	"archive/tar"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
)

// File size limits
const (
	// MaxFileSize is the maximum size of a single file that can be extracted (10MB)
	MaxFileSize = 10 * 1024 * 1024
)

// formatError creates a standardized error message with context.
func formatError(operation, resource, details string, err error) string {
	if err != nil {
		return fmt.Sprintf("Failed to %s %s: %s (%v)", operation, resource, details, err)
	}
	return fmt.Sprintf("Failed to %s %s: %s", operation, resource, details)
}

// sanitizePath validates and cleans a file path to prevent path traversal attacks.
// It rejects paths containing ".." components and ensures the path is within bounds.
func sanitizePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	// Clean the path to resolve any . and .. elements
	cleaned := filepath.Clean(path)
	
	// Check for path traversal attempts
	if strings.Contains(cleaned, "..") || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("path traversal detected: %s", path)
	}
	
	// Ensure the path doesn't start with / to avoid absolute paths
	cleaned = strings.TrimPrefix(cleaned, "/")
	
	return cleaned, nil
}

// validateContainerName validates that a container name follows Docker naming conventions.
// Docker container names must match [a-zA-Z0-9][a-zA-Z0-9_.-]* and cannot be empty.
func validateContainerName(name string) error {
	if name == "" {
		return fmt.Errorf("container name cannot be empty")
	}
	
	// Docker container name validation: must start with alphanumeric, then can contain alphanumeric, underscore, period, dash
	matched, err := regexp.MatchString(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`, name)
	if err != nil {
		return fmt.Errorf("failed to validate container name: %w", err)
	}
	if !matched {
		return fmt.Errorf("invalid container name format (must start with letter/number, then can contain letters/numbers/underscore/period/dash): %s", name)
	}
	
	return nil
}

// extractFileFromTar extracts a single file entry from a tar reader.
// Returns FileInfo containing the header and content, or an error if extraction fails.
// Content will be nil for non-regular files. Files larger than MaxFileSize will be rejected.
func extractFileFromTar(r *tar.Reader) (*FileInfo, error) {
	hdr, err := r.Next()

	// Check if we've reached the end of the tar stream
	if err == io.EOF {
		return nil, io.EOF
	}

	// Check for other errors
	if err != nil {
		return nil, err
	}

	// Check if the header is a regular file
	if hdr.Typeflag != tar.TypeReg {
		return &FileInfo{Header: hdr, Content: nil}, nil
	}

	// Check file size before reading to prevent memory exhaustion
	if hdr.Size > MaxFileSize {
		return nil, fmt.Errorf("file too large: %d bytes exceeds maximum allowed size of %d bytes", hdr.Size, MaxFileSize)
	}

	// Read the file contents
	var buf []byte
	buf, err = io.ReadAll(r)

	// Check for errors while reading the file contents
	if err != nil {
		return nil, err
	}

	// Return the FileInfo with header and file contents
	return &FileInfo{Header: hdr, Content: buf}, nil
}

// FileInfo represents metadata and content extracted from a tar archive entry.
// It contains both the tar header information and the actual file content.
// Content will be nil for non-regular files (directories, symlinks, etc.).
type FileInfo struct {
	Header  *tar.Header // tar header containing file metadata
	Content []byte      // file content, nil for non-regular files
}

// extractAllFilesFromTar extracts all files from a tar reader into a map.
// Returns a map where keys are file names and values are FileInfo structs.
// Files larger than MaxFileSize will be rejected with an error.
func extractAllFilesFromTar(r *tar.Reader) (map[string]*FileInfo, error) {
	files := make(map[string]*FileInfo)

	for {
		fileInfo, err := extractFileFromTar(r)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		files[fileInfo.Header.Name] = fileInfo
	}

	return files, nil
}
