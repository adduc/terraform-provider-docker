package internal

import (
	"archive/tar"
	"io"
)

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

type FileInfo struct {
	Header  *tar.Header
	Content []byte
}

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
