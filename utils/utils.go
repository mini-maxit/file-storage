package utils

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// FileSize returns size of file
func FileSize(file *os.File) int64 {
	info, err := file.Stat()
	if err != nil {
		return 0
	}
	return info.Size()
}

// CloseIO tries to close any io.Closer and logs an error if one occurs.
func CloseIO(c io.Closer) {
	if err := c.Close(); err != nil {
		log.Printf("Error closing file: %v", err)
	}
}

// RemoveDirectory tries to remove any directory from given path and logs an error if one occurs.
func RemoveDirectory(path string) {
	if err := os.RemoveAll(path); err != nil {
		log.Printf("Error removing directory: %v", err)
	}
}

// DecompressArchive decompresses archive (either .zip or .tar.gzip) to the given newPath
func DecompressArchive(archivePath string, newPath string) error {
	if strings.HasSuffix(archivePath, ".gz") {
		err := DecompressGzip(archivePath, newPath)
		if err != nil {
			return fmt.Errorf("failed to uncompress directory (gzip): %v", err)
		}
	} else if strings.HasSuffix(archivePath, ".zip") {
		err := DecompressZip(archivePath, newPath)
		if err != nil {
			return fmt.Errorf("failed to uncompress directory (zip): %v", err)
		}
	} else {
		return fmt.Errorf("unsupported archive type: %s", archivePath)
	}

	return nil
}

// DecompressGzip decompresses a Gzip archive from archivePath to a new directory in the newPath
func DecompressGzip(archivePath string, newPath string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer CloseIO(file)

	uncompressedStream, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer CloseIO(uncompressedStream)

	tarReader := tar.NewReader(uncompressedStream)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			dirPath := path.Join(newPath, header.Name)
			if err := os.MkdirAll(dirPath, 0755); err != nil {
				return err
			}

		case tar.TypeReg:
			filePath := path.Join(newPath, header.Name)
			if err := os.MkdirAll(path.Dir(filePath), 0755); err != nil {
				return err
			}

			outFile, err := os.Create(filePath)
			if err != nil {
				return err
			}
			defer CloseIO(outFile)

			if _, err := io.Copy(outFile, tarReader); err != nil {
				return err
			}

		default:
			return errors.New("unsupported file type")
		}
	}
	return nil
}

// DecompressZip decompresses a Gzip archive from archivePath to a new directory in the newPath
func DecompressZip(archivePath string, newPath string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer CloseIO(r)

	for _, f := range r.File {
		filePath := filepath.Join(newPath, f.Name)

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(filePath, 0755); err != nil {
				return err
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
				return err
			}

			inFile, err := f.Open()
			if err != nil {
				return err
			}
			defer CloseIO(inFile)

			outFile, err := os.Create(filePath)
			if err != nil {
				return err
			}
			defer CloseIO(outFile)

			if _, err := io.Copy(outFile, inFile); err != nil {
				return err
			}
		}
	}
	return nil
}