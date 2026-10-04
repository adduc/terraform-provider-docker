// Package internal implements the core functionality for the terraform-provider-docker.
// It provides data sources for retrieving files and logs from Docker containers.
package internal

import (
	"archive/tar"
	"encoding/base64"
	"fmt"
	"io"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// File size limits
const (
	// MaxFileSize is the maximum size of a single file that can be extracted (10MB)
	MaxFileSize = 10 * 1024 * 1024
)

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

// fileContentBase64 returns the content_base64 attribute for a tar entry.
// Content is always base64-encoded, since Terraform strings must be valid
// UTF-8 and file contents may not be. It is null for entries that are not
// regular files.
func fileContentBase64(info *FileInfo) types.String {
	if info.Content == nil {
		return types.StringNull()
	}

	return types.StringValue(base64.StdEncoding.EncodeToString(info.Content))
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
