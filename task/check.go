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
	CompressHeader = 3
	ArchiveHeader  = 262
)

func isGzip(buf []byte) bool {
	return len(buf) > CompressHeader-1 &&
		buf[0] == 0x1F && buf[1] == 0x8B && buf[2] == 0x8
}

func isTar(buf []byte) bool {
	return len(buf) > ArchiveHeader-1 &&
		buf[257] == 0x75 && buf[258] == 0x73 &&
		buf[259] == 0x74 && buf[260] == 0x61 &&
		buf[261] == 0x72
}

// IsValidImage checks if a given file is a valid image
// Current supports: tar and gzip tar files
func IsValidImage(srcFileName string) error {
	src, err := os.OpenFile(srcFileName, os.O_RDONLY, 0444)
	if err != nil {
		return err
	}
	defer src.Close()
	gzipped, err := checkCompress(src)
	if err != nil {
		return err
	}
	supported, err := checkArchive(gzipped)
	if !supported {
		return errors.New("Unknown archive")
	}
	if err != io.EOF {
		return err
	}
	return nil
}

// checkCompress checks if the file is compressed and returned the
// compress reader in that case
// It returns the compressed reader or the same reader if it is not compressed.
func checkCompress(src io.ReadSeeker) (out io.Reader, err error) {
	buf := make([]byte, CompressHeader)
	n_read, err := src.Read(buf)
	if n_read > 0 {
		// Rewind
		if _, err = src.Seek(0, 0); err != nil {
			return nil, err
		}
	}
	if isGzip(buf) {
		out, err = gzip.NewReader(src)
		//defer out.Close()
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
