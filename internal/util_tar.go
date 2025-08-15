package internal

import (
	"archive/tar"
	"io"
)

func extractFileFromTar(r *tar.Reader) (*tar.Header, []byte, error) {
	hdr, err := r.Next()

	// Check if we've reached the end of the tar stream
	if err == io.EOF {
		return nil, nil, io.EOF
	}

	// Check for other errors
	if err != nil {
		return nil, nil, err
	}

	// Check if the header is a regular file
	if hdr.Typeflag != tar.TypeReg {
		return hdr, nil, nil
	}

	// Read the file contents
	var buf []byte
	buf, err = io.ReadAll(r)

	// Check for errors while reading the file contents
	if err != nil {
		return nil, nil, err
	}

	// Return the header and file contents
	return hdr, buf, nil
}
