package services

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
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
		"src/input/file1.txt":  []byte("Input file 1 content"),
		"src/output/file1.txt": []byte("Output file 1 content"),
		"src/input/file2.txt":  []byte("Input file 2 content"),
		"src/output/file2.txt": []byte("Output file 2 content"),
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

		// Verify input and output files are renamed correctly
		inputFile1 := filepath.Join(srcDir, "input", "1.in.txt")
		outputFile1 := filepath.Join(srcDir, "output", "1.out.txt")
		inputFile2 := filepath.Join(srcDir, "input", "2.in.txt")
		outputFile2 := filepath.Join(srcDir, "output", "2.out.txt")

		assert.FileExists(t, inputFile1, "1.in.txt input file should exist")
		assert.FileExists(t, outputFile1, "1.out.txt output file should exist")
		assert.FileExists(t, inputFile2, "2.in.txt input file should exist")
		assert.FileExists(t, outputFile2, "2.out.txt output file should exist")
	})

	// Subtest for overwriting an existing task directory
	t.Run("should overwrite an existing task directory", func(t *testing.T) {
		// Modify the files for overwrite
		files["src/description.pdf"] = []byte("New task description content")
		files["src/input/file1.txt"] = []byte("New input content")
		files["src/output/file1.txt"] = []byte("New output content")

		// Attempt to overwrite the directory
		err := ts.CreateTaskDirectory(1, files, true)
		assert.NoError(t, err, "expected no error when overwriting the task directory")

		// Verify the files have been overwritten and renamed correctly
		descriptionFile := filepath.Join(mockConfig.RootDirectory, "task1", "src", "description.pdf")
		content, err := os.ReadFile(descriptionFile)
		assert.NoError(t, err, "expected no error reading description.pdf")
		assert.Equal(t, "New task description content", string(content), "description.pdf content should be overwritten")

		inputFile := filepath.Join(mockConfig.RootDirectory, "task1", "src", "input", "1.in.txt")
		outputFile := filepath.Join(mockConfig.RootDirectory, "task1", "src", "output", "1.out.txt")
		assert.FileExists(t, inputFile, "1.in.txt should exist")
		assert.FileExists(t, outputFile, "1.out.txt should exist")
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
			"src/input/file1.txt": []byte("Input file 1 content"),
			// Missing output file, mismatching the number of input files
		}

		// Attempt to create the directory
		err := ts.CreateTaskDirectory(1, mismatchedFiles, false)
		assert.Error(t, err, "expected an error due to mismatched input/output files")
	})

	// Subtest to check if an error is returned for non-txt input/output files
	t.Run("should return an error when input/output files are not .txt files", func(t *testing.T) {
		invalidFiles := map[string][]byte{
			"src/input/1.in.pdf":   []byte("Input file in invalid format"),
			"src/output/1.out.pdf": []byte("Output file in invalid format"),
		}

		// Attempt to create the directory with invalid file formats
		err := ts.CreateTaskDirectory(3, invalidFiles, false)
		assert.Error(t, err, "expected an error when input or output file is not a .txt file")
		assert.Contains(t, err.Error(), "only .txt files are allowed", "error message should mention invalid file format")
	})

	// Subtest to check if an error is returned for non-pdf description files
	t.Run("should return an error when description file is not a .pdf file", func(t *testing.T) {
		invalidFiles := map[string][]byte{
			"src/description.exe":  []byte("Task description content"),
			"src/input/1.in.txt":   []byte("Input file 1 content"),
			"src/output/1.out.txt": []byte("Output file 1 content"),
		}

		// Attempt to create the directory with invalid file formats
		err := ts.CreateTaskDirectory(3, invalidFiles, false)
		assert.Error(t, err, "expected an error when description is not a .pdf file")
		assert.Contains(t, err.Error(), "description must have a .pdf extension", "error message should mention invalid file format")
	})
}

func TestCreateUserSubmission(t *testing.T) {
	rootDir, cleanup := createTempRootDir(t)
	defer cleanup()

	// Create a mock configuration with the temporary root directory
	mockConfig := &config.Config{
		RootDirectory:    rootDir,
		AllowedFileTypes: []string{".c", ".cpp", ".py"}, // Allowed extensions
	}

	ts := NewTaskService(mockConfig)

	// Define mock task files for input/output testing to create a valid task
	taskFiles := map[string][]byte{
		"src/description.pdf":  []byte("Task description content"),
		"src/input/1.in.txt":   []byte("Input file 1 content"),
		"src/output/1.out.txt": []byte("Output file 1 content"),
	}

	// Set up the task directory for task ID 1
	err := ts.CreateTaskDirectory(1, taskFiles, false)
	assert.NoError(t, err, "expected no error when creating the task directory for task 1")

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

func TestStoreUserOutputs(t *testing.T) {
	rootDir, cleanup := createTempRootDir(t)
	defer cleanup()

	// Create a mock configuration with the temporary root directory
	mockConfig := &config.Config{
		RootDirectory: rootDir,
	}

	ts := NewTaskService(mockConfig)

	// Helper function to create a specific user submission directory
	createUserSubmissionDir := func(taskID, userID, submissionNumber int) {
		userSubmissionDir := filepath.Join(mockConfig.RootDirectory, fmt.Sprintf("task%d", taskID), "submissions", fmt.Sprintf("user%d", userID), fmt.Sprintf("submission%d", submissionNumber), "output")
		err := os.MkdirAll(userSubmissionDir, os.ModePerm)
		assert.NoError(t, err, "failed to create user submission directory")
	}

	// Helper function to create the task's expected output directory with some .out.txt files
	createExpectedOutputFiles := func(taskID int, count int) {
		expectedOutputDir := filepath.Join(mockConfig.RootDirectory, fmt.Sprintf("task%d", taskID), "src", "output")
		err := os.MkdirAll(expectedOutputDir, os.ModePerm)
		assert.NoError(t, err, "failed to create expected output directory")

		for i := 1; i <= count; i++ {
			filePath := filepath.Join(expectedOutputDir, fmt.Sprintf("%d.out.txt", i))
			err := os.WriteFile(filePath, []byte(fmt.Sprintf("Expected output %d", i)), 0644)
			assert.NoError(t, err, "failed to create expected output file %d", i)
		}
	}

	// Subtest for storing valid output files
	t.Run("should store valid output files in {number}.out.txt format", func(t *testing.T) {
		taskID := 1
		userID := 1
		submissionNumber := 1

		// Set up expected output files
		createExpectedOutputFiles(taskID, 2)

		// Create the user submission directory for the task
		createUserSubmissionDir(taskID, userID, submissionNumber)

		// Output files to store
		outputFiles := map[string][]byte{
			"output1.txt": []byte("Output 1 content"),
			"output2.txt": []byte("Output 2 content"),
		}

		// Store output files
		err := ts.StoreUserOutputs(taskID, userID, submissionNumber, outputFiles)
		assert.NoError(t, err, "expected no error when storing valid output files")

		// Verify files are stored correctly
		outputDir := filepath.Join(mockConfig.RootDirectory, "task1", "submissions", "user1", "submission1", "output")
		assert.FileExists(t, filepath.Join(outputDir, "1.out.txt"), "First output file should exist as 1.out.txt")
		assert.FileExists(t, filepath.Join(outputDir, "2.out.txt"), "Second output file should exist as 2.out.txt")
	})

	// Subtest for handling compile-error.err
	t.Run("should store compile-error.err when it is the only file", func(t *testing.T) {
		taskID := 2
		userID := 1
		submissionNumber := 1

		// Set up expected output files
		createExpectedOutputFiles(taskID, 2)

		// Create the user submission directory for the task
		createUserSubmissionDir(taskID, userID, submissionNumber)

		// Compile error file
		outputFiles := map[string][]byte{
			"compile-error.err": []byte("Compilation error details"),
		}

		// Store compile error
		err := ts.StoreUserOutputs(taskID, userID, submissionNumber, outputFiles)
		assert.NoError(t, err, "expected no error when storing compile-error.err")

		// Verify compile-error.err exists
		outputDir := filepath.Join(mockConfig.RootDirectory, "task2", "submissions", "user1", "submission1", "output")
		assert.FileExists(t, filepath.Join(outputDir, "compile-error.err"), "Compile-error file should exist")
	})

	// Subtest for error when trying to store non-.txt files
	t.Run("should return an error when trying to store non-.txt output files", func(t *testing.T) {
		taskID := 3
		userID := 1
		submissionNumber := 1

		// Set up expected output files
		createExpectedOutputFiles(taskID, 1)

		// Create the user submission directory for the task
		createUserSubmissionDir(taskID, userID, submissionNumber)

		// Invalid output file (non-.txt)
		outputFiles := map[string][]byte{
			"output1.pdf": []byte("Invalid output format"),
		}

		// Attempt to store invalid output files
		err := ts.StoreUserOutputs(taskID, userID, submissionNumber, outputFiles)
		assert.Error(t, err, "expected an error when trying to store non-.txt files")
		assert.Contains(t, err.Error(), "only .txt files or 'compile-error.err' are allowed", "error message should mention invalid file format")
	})

	// Subtest for error when number of output files doesn't match the number of output files of a task
	t.Run("should return an error when number of outputs does not match task expected outputs", func(t *testing.T) {
		taskID := 6
		userID := 1
		submissionNumber := 1

		// Set up expected output files
		createExpectedOutputFiles(taskID, 2)

		// Create the user submission directory for the task
		createUserSubmissionDir(taskID, userID, submissionNumber)

		// Store only one output file (mismatched count with task's expected output count)
		outputFiles := map[string][]byte{
			"output1.txt": []byte("User output 1"),
		}

		// Attempt to store the output files and expect an error
		err := ts.StoreUserOutputs(taskID, userID, submissionNumber, outputFiles)
		assert.Error(t, err, "expected an error when number of user outputs does not match task's expected outputs")
		assert.Contains(t, err.Error(), "number of output files does not match the expected number", "error message should indicate output count mismatch")
	})
}

func TestGetTaskFiles(t *testing.T) {
	rootDir, cleanup := createTempRootDir(t)
	defer cleanup()

	// Mock configuration
	mockConfig := &config.Config{RootDirectory: rootDir}
	ts := NewTaskService(mockConfig)

	// Helper function to set up a sample task directory structure with files for testing
	createSampleTaskDir := func(taskID int) {
		taskDir := filepath.Join(rootDir, fmt.Sprintf("task%d", taskID), "src")
		inputDir := filepath.Join(taskDir, "input")
		outputDir := filepath.Join(taskDir, "output")

		// Create task directories and sample files
		err := os.MkdirAll(inputDir, os.ModePerm)
		assert.NoError(t, err, "failed to create input directory")

		err = os.MkdirAll(outputDir, os.ModePerm)
		assert.NoError(t, err, "failed to create output directory")

		err = os.WriteFile(filepath.Join(taskDir, "description.pdf"), []byte("Task description content"), 0644)
		assert.NoError(t, err, "failed to create description file")

		err = os.WriteFile(filepath.Join(inputDir, "1.in.txt"), []byte("Input file 1 content"), 0644)
		assert.NoError(t, err, "failed to create input file")

		err = os.WriteFile(filepath.Join(outputDir, "1.out.txt"), []byte("Output file 1 content"), 0644)
		assert.NoError(t, err, "failed to create output file")
	}

	// Subtest for successful .tar.gz creation
	t.Run("should create a .tar.gz with the expected structure and files", func(t *testing.T) {
		taskID := 1
		createSampleTaskDir(taskID)

		// Call the function to test
		tarFilePath, err := ts.GetTaskFiles(taskID)
		assert.NoError(t, err, "expected no error when creating task archive")
		assert.FileExists(t, tarFilePath, "expected the tar file to be created")

		// Open the created .tar.gz file and verify its contents
		tarFile, err := os.Open(tarFilePath)
		assert.NoError(t, err, "failed to open created .tar.gz file")
		defer tarFile.Close()

		// Initialize gzip and tar readers
		gzipReader, err := gzip.NewReader(tarFile)
		assert.NoError(t, err, "failed to create gzip reader")
		defer gzipReader.Close()

		tarReader := tar.NewReader(gzipReader)
		filesFound := map[string]bool{
			"task1Files/src/description.pdf":  false,
			"task1Files/src/input/1.in.txt":   false,
			"task1Files/src/output/1.out.txt": false,
		}

		for {
			header, err := tarReader.Next()
			if err != nil {
				break
			}
			if _, exists := filesFound[header.Name]; exists {
				filesFound[header.Name] = true
			}
		}

		// Verify all expected files were included
		for fileName, found := range filesFound {
			assert.True(t, found, "expected file %s to be present in the archive", fileName)
		}
	})

	// Subtest for error when src directory is missing
	t.Run("should return an error when src directory is missing", func(t *testing.T) {
		taskID := 2
		tarFilePath, err := ts.GetTaskFiles(taskID)
		assert.Error(t, err, "expected an error when src directory is missing")
		assert.Empty(t, tarFilePath, "expected no tar file to be created when src directory is missing")
	})
}
