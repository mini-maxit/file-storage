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

// TestCreateUserSubmission tests the CreateUserSubmission function using subtests to describe different scenarios.
func TestCreateUserSubmission(t *testing.T) {
	rootDir, cleanup := createTempRootDir(t)
	defer cleanup()

	// Create a mock configuration with the temporary root directory
	mockConfig := &config.Config{
		RootDirectory:    rootDir,
		AllowedFileTypes: []string{".c", ".cpp", ".py"}, // Allowed extensions
	}

	ts := NewTaskService(mockConfig)

	// Subtest for creating the first submission for a user
	t.Run("should create a new user submission directory", func(t *testing.T) {
		userFileContent := []byte("int main() { return 0; }")
		fileName := "solution.c"

		// Create the first submission for user 1 and task 1
		err := ts.CreateUserSubmission(1, 1, userFileContent, fileName)
		assert.NoError(t, err, "expected no error when creating the first user submission")

		// Verify the directory structure
		userDir := filepath.Join(mockConfig.RootDirectory, "task1", "submissions", "user1")
		assert.DirExists(t, userDir, "user directory should exist")

		// Verify submission directory exists
		submissionDir := filepath.Join(userDir, "submission1")
		assert.DirExists(t, submissionDir, "submission1 directory should exist")

		// Verify that the output directory exists
		outputDir := filepath.Join(submissionDir, "output")
		assert.DirExists(t, outputDir, "output directory should exist")

		// Verify that the solution file exists and contains the correct content
		solutionFile := filepath.Join(submissionDir, "solution.c")
		assert.FileExists(t, solutionFile, "solution.c file should exist")

		content, err := os.ReadFile(solutionFile)
		assert.NoError(t, err, "expected no error when reading solution.c file")
		assert.Equal(t, string(userFileContent), string(content), "solution.c content should match")
	})

	// Subtest for creating multiple submissions for the same user
	t.Run("should create multiple submissions for the same user with incrementing submission numbers", func(t *testing.T) {
		userFileContent := []byte("int main() { return 1; }")
		fileName := "solution.c"

		// Create a second submission for user 1 and task 1
		err := ts.CreateUserSubmission(1, 1, userFileContent, fileName)
		assert.NoError(t, err, "expected no error when creating the second user submission")

		// Verify submission directory exists
		submissionDir := filepath.Join(mockConfig.RootDirectory, "task1", "submissions", "user1", "submission2")
		assert.DirExists(t, submissionDir, "submission2 directory should exist")

		// Verify that the output directory exists
		outputDir := filepath.Join(submissionDir, "output")
		assert.DirExists(t, outputDir, "output directory should exist")

		// Verify that the solution file exists and contains the correct content
		solutionFile := filepath.Join(submissionDir, "solution.c")
		assert.FileExists(t, solutionFile, "solution.c file should exist")

		content, err := os.ReadFile(solutionFile)
		assert.NoError(t, err, "expected no error when reading solution.c file")
		assert.Equal(t, string(userFileContent), string(content), "solution.c content should match")
	})

	// Subtest for ensuring invalid file extensions return an error
	t.Run("should return an error for unsupported file extensions", func(t *testing.T) {
		userFileContent := []byte("#include <stdio.h>\nint main() { return 0; }")
		fileName := "solution.java" // Unsupported file extension

		// Attempt to create a submission with an unsupported file extension
		err := ts.CreateUserSubmission(1, 2, userFileContent, fileName)
		assert.Error(t, err, "expected an error when creating submission with unsupported file extension")
		assert.Contains(t, err.Error(), "file extension '.java' is not allowed", "error message should mention unsupported file extension")
	})

	// Subtest for creating submissions for multiple users
	t.Run("should create submissions for multiple users", func(t *testing.T) {
		userFileContent := []byte("int main() { return 42; }")
		fileName := "solution.c"

		// Create a submission for user 2
		err := ts.CreateUserSubmission(1, 2, userFileContent, fileName)
		assert.NoError(t, err, "expected no error when creating submission for user 2")

		// Verify the user directory exists
		userDir := filepath.Join(mockConfig.RootDirectory, "task1", "submissions", "user2")
		assert.DirExists(t, userDir, "user2 directory should exist")

		// Verify the first submission directory exists
		submissionDir := filepath.Join(userDir, "submission1")
		assert.DirExists(t, submissionDir, "submission1 directory should exist")

		// Verify that the output directory exists
		outputDir := filepath.Join(submissionDir, "output")
		assert.DirExists(t, outputDir, "output directory should exist")

		// Verify that the solution file exists and contains the correct content
		solutionFile := filepath.Join(submissionDir, "solution.c")
		assert.FileExists(t, solutionFile, "solution.c file should exist")

		content, err := os.ReadFile(solutionFile)
		assert.NoError(t, err, "expected no error when reading solution.c file")
		assert.Equal(t, string(userFileContent), string(content), "solution.c content should match")
	})

	// Subtest for trying to submit a file to a non-existent task
	t.Run("should return an error when trying to submit to a non-existent task", func(t *testing.T) {
		userFileContent := []byte("int main() { return 404; }")
		fileName := "solution.c"

		// Simulate the task directory not being created (taskID 999)
		err := ts.CreateUserSubmission(999, 1, userFileContent, fileName)
		assert.Error(t, err, "expected an error when trying to submit to a non-existent task")
		assert.Contains(t, err.Error(), "invalid taskID: task directory does not exist", "error message should indicate failure due to missing task directory")
	})
}
