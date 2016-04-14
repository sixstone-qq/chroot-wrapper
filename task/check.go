package task

// This module package is using magic numbers to check the type of the incoming buffer

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"io"
	"os"
)

const (
	compressHeader = 3
	archiveHeader  = 262
)

func isGzip(buf []byte) bool {
	return len(buf) > compressHeader-1 &&
		buf[0] == 0x1F && buf[1] == 0x8B && buf[2] == 0x8
}

func isTar(buf []byte) bool {
	return len(buf) > archiveHeader-1 &&
		buf[257] == 0x75 && buf[258] == 0x73 &&
		buf[259] == 0x74 && buf[260] == 0x61 &&
		buf[261] == 0x72
}

// ValidImage checks if a given file is a valid image. It currently
// supports: tar and gzip tar files.
//
// It returns a bool indicating if the image is compressed and the
// error if happens.
func ValidImage(srcFileName string) (bool, error) {
	src, err := os.OpenFile(srcFileName, os.O_RDONLY, 0444)
	if err != nil {
		return false, err
	}
	defer src.Close()
	gzipped, err := checkCompress(src)
	if err != nil {
		return false, err
	}
	defer gzipped.(io.Closer).Close()
	supported, err := checkArchive(gzipped)
	if !supported {
		return gzipped != src, errors.New("Unknown archive")
	}
	return gzipped != src, nil
}

// checkCompress checks if the file is compressed and returned the
// compress reader in that case
// It returns the compressed reader or the same reader if it is not compressed.
func checkCompress(src io.ReadSeeker) (out io.Reader, err error) {
	buf := make([]byte, compressHeader)
	n_read, err := src.Read(buf)
	if n_read > 0 {
		// Rewind
		if _, err = src.Seek(0, 0); err != nil {
			return nil, err
		}
	}
	if isGzip(buf) {
		out, err = gzip.NewReader(src)
		if err != nil {
			return nil, err
		}
	} else {
		out = src
	}
	return out, nil
}

// checkArchive checks if a file is a supported archive type
func checkArchive(src io.Reader) (supported bool, err error) {
	// Do not use magic numbers by now
	tr := tar.NewReader(src)
	_, err = tr.Next()
	return err == nil, err
}
