package services

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mini-maxit/file-storage/internal/config"
	"github.com/stretchr/testify/assert"
)

// Helper function to create temporary directories for testing
func createTempRootDir(t *testing.T) (string, func()) {
	t.Helper()

	// Create a temporary root directory for tasks
	tempDir, err := os.MkdirTemp("", "task_service_test")
	if err != nil {
		t.Fatalf("unable to create temp root directory: %v", err)
	}

	// Return cleanup function to remove the directory after the test
	return tempDir, func() {
		err := os.RemoveAll(tempDir)
		if err != nil {
			return
		}
	}
}

// TestCreateTaskDirectory tests the createTaskDirectory function using subtests to describe different scenarios.
func TestCreateTaskDirectory(t *testing.T) {
	rootDir, cleanup := createTempRootDir(t)
	defer cleanup()

	// Create a mock configuration with the temporary root directory
	mockConfig := &config.Config{
		RootDirectory: rootDir,
	}

	ts := NewTaskService(mockConfig)

	// Define mock files for input/output testing
	files := map[string][]byte{
		"src/description.pdf":  []byte("Task description content"),
		"src/input/1.in.txt":   []byte("Input file 1 content"),
		"src/output/1.out.txt": []byte("Output file 1 content"),
	}

	// Subtest for creating a new task directory
	t.Run("should create a new task directory", func(t *testing.T) {
		err := ts.CreateTaskDirectory(1, files, false)
		assert.NoError(t, err, "expected no error when creating a new task directory")

		// Verify the directory structure and files exist
		taskDir := filepath.Join(mockConfig.RootDirectory, "task1")
		srcDir := filepath.Join(taskDir, "src")
		assert.DirExists(t, srcDir, "src directory should exist")
		assert.DirExists(t, filepath.Join(srcDir, "input"), "input directory should exist")
		assert.DirExists(t, filepath.Join(srcDir, "output"), "output directory should exist")

		// Verify description.pdf exists
		descriptionFile := filepath.Join(srcDir, "description.pdf")
		assert.FileExists(t, descriptionFile, "description.pdf should exist")

		// Verify input and output files exist
		inputFile := filepath.Join(srcDir, "input", "1.in.txt")
		outputFile := filepath.Join(srcDir, "output", "1.out.txt")
		assert.FileExists(t, inputFile, "input file should exist")
		assert.FileExists(t, outputFile, "output file should exist")
	})

	// Subtest for overwriting an existing task directory
	t.Run("should overwrite an existing task directory", func(t *testing.T) {
		// Modify the files for overwrite
		files["src/description.pdf"] = []byte("New task description content")
		files["src/input/1.in.txt"] = []byte("New input content")
		files["src/output/1.out.txt"] = []byte("New output content")

		// Attempt to overwrite the directory
		err := ts.CreateTaskDirectory(1, files, true)
		assert.NoError(t, err, "expected no error when overwriting the task directory")

		// Verify the files have been overwritten
		descriptionFile := filepath.Join(mockConfig.RootDirectory, "task1", "src", "description.pdf")
		content, err := os.ReadFile(descriptionFile)
		assert.NoError(t, err, "expected no error reading description.pdf")
		assert.Equal(t, "New task description content", string(content), "description.pdf content should be overwritten")
	})

	// Subtest for when task directory exists and overwrite is not allowed
	t.Run("should return an error when directory exists and overwrite is false", func(t *testing.T) {
		// Attempt to create the directory again without overwrite
		err := ts.CreateTaskDirectory(1, files, false)
		assert.Error(t, err, "expected error when trying to create an existing task directory without overwrite")
	})

	// Subtest for mismatched input/output files
	t.Run("should return an error when input and output files are mismatched", func(t *testing.T) {
		// Mock files with mismatched input and output files
		mismatchedFiles := map[string][]byte{
			"src/description.pdf": []byte("Task description content"),
			"src/input/1.in.txt":  []byte("Input file 1 content"),
			// Missing output file, mismatching the number of input files
		}

		// Attempt to create the directory
		err := ts.CreateTaskDirectory(1, mismatchedFiles, false)
		assert.Error(t, err, "expected an error due to mismatched input/output files")
	})
}
