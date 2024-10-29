package utils

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"os"
	"strings"
	"testing"
)

// setupTestFiles creates sample .zip and .tar.gz files for testing
func setupTestFiles() error {
	if err := os.MkdirAll("testdata", 0755); err != nil {
		return err
	}

	// Create sample zip file
	if err := createSampleZip("testdata/test.zip"); err != nil {
		return fmt.Errorf("failed to create sample zip: %w", err)
	}

	// Create sample tar.gz file
	if err := createSampleTarGz("testdata/test.tar.gz"); err != nil {
		return fmt.Errorf("failed to create sample tar.gz: %w", err)
	}

	return nil
}

// createSampleZip creates a sample zip archive with a few test files
func createSampleZip(filePath string) error {
	outFile, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer CloseIO(outFile)

	zipWriter := zip.NewWriter(outFile)
	defer CloseIO(zipWriter)

	files := []struct {
		Name, Body string
	}{
		{"file1.txt", "This is file1"},
		{"file2.txt", "This is file2"},
	}

	for _, file := range files {
		f, err := zipWriter.Create(file.Name)
		if err != nil {
			return err
		}
		_, err = f.Write([]byte(file.Body))
		if err != nil {
			return err
		}
	}

	return nil
}

// createSampleTarGz creates a sample tar.gz archive with a few test files
func createSampleTarGz(filePath string) error {
	outFile, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer CloseIO(outFile)

	gzipWriter := gzip.NewWriter(outFile)
	defer CloseIO(gzipWriter)

	tarWriter := tar.NewWriter(gzipWriter)
	defer CloseIO(tarWriter)

	files := []struct {
		Name, Body string
	}{
		{"file1.txt", "This is file1"},
		{"file2.txt", "This is file2"},
	}

	for _, file := range files {
		hdr := &tar.Header{
			Name: file.Name,
			Mode: 0600,
			Size: int64(len(file.Body)),
		}
		if err := tarWriter.WriteHeader(hdr); err != nil {
			return err
		}
		if _, err := tarWriter.Write([]byte(file.Body)); err != nil {
			return err
		}
	}

	return nil
}

func TestDecompressArchive(t *testing.T) {
	// Setup test files
	if err := setupTestFiles(); err != nil {
		t.Fatalf("failed to set up test files: %v", err)
	}
	defer func() {
		err := os.RemoveAll("testdata")
		if err != nil {

		}
	}() // Cleanup test files after tests

	tests := []struct {
		name        string
		archivePath string
		newPath     string
		expectedErr string
	}{
		{
			name:        "Valid ZIP Archive",
			archivePath: "testdata/test.zip",
			newPath:     "testdata/output_zip",
			expectedErr: "",
		},
		{
			name:        "Valid TAR.GZ Archive",
			archivePath: "testdata/test.tar.gz",
			newPath:     "testdata/output_tar_gz",
			expectedErr: "",
		},
		{
			name:        "Unsupported File Type",
			archivePath: "testdata/test.txt",
			newPath:     "testdata/output_unsupported",
			expectedErr: "unsupported archive type",
		},
		{
			name:        "Non-existent Archive Path",
			archivePath: "testdata/nonexistent.zip",
			newPath:     "testdata/output_nonexistent",
			expectedErr: "failed to uncompress directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up the output directory if it already exists
			if _, err := os.Stat(tt.newPath); err == nil {
				err := os.RemoveAll(tt.newPath)
				if err != nil {
					return
				}
			}

			// Run the DecompressArchive function
			err := DecompressArchive(tt.archivePath, tt.newPath)

			// Check if an error was expected
			if tt.expectedErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.expectedErr) {
					t.Errorf("expected error containing '%s', got '%v'", tt.expectedErr, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				// Additional checks if there was no error
				if _, err := os.Stat(tt.newPath); os.IsNotExist(err) {
					t.Errorf("expected output directory '%s' to exist, but it does not", tt.newPath)
				}
			}

			// Clean up test output directory after each test
			err = os.RemoveAll(tt.newPath)
			if err != nil {
				return
			}
		})
	}
}
