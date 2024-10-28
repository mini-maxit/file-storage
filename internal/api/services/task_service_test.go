package services

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"github.com/mini-maxit/file-storage/internal/api/taskutils"
	"github.com/mini-maxit/file-storage/utils"
	"io"
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

	tu := taskutils.NewTaskUtils(mockConfig)
	ts := NewTaskService(mockConfig, tu)

	// Define mock files for input/output testing
	files := map[string][]byte{
		"src/description.pdf": []byte("Task description content"),
		"src/input/1.in":      []byte("Input file 1 content"),
		"src/output/1.out":    []byte("Output file 1 content"),
		"src/input/2.in":      []byte("Input file 2 content"),
		"src/output/2.out":    []byte("Output file 2 content"),
	}

	// Subtest for creating a new task directory
	t.Run("should create a new task directory", func(t *testing.T) {
		err := ts.CreateTaskDirectory(1, files, false)
		assert.NoError(t, err, "expected no error when creating a new task directory")

		// Verify the directory structure and files exist
		taskDir := filepath.Join(ts.taskDirectory, "task1")
		srcDir := filepath.Join(taskDir, "src")
		assert.DirExists(t, srcDir, "src directory should exist")
		assert.DirExists(t, filepath.Join(srcDir, "input"), "input directory should exist")
		assert.DirExists(t, filepath.Join(srcDir, "output"), "output directory should exist")

		// Verify description.pdf exists
		descriptionFile := filepath.Join(srcDir, "description.pdf")
		assert.FileExists(t, descriptionFile, "description.pdf should exist")

		// Verify input and output files are named correctly
		inputFile1 := filepath.Join(srcDir, "input", "1.in")
		outputFile1 := filepath.Join(srcDir, "output", "1.out")
		inputFile2 := filepath.Join(srcDir, "input", "2.in")
		outputFile2 := filepath.Join(srcDir, "output", "2.out")

		assert.FileExists(t, inputFile1, "1.in input file should exist")
		assert.FileExists(t, outputFile1, "1.out output file should exist")
		assert.FileExists(t, inputFile2, "2.in input file should exist")
		assert.FileExists(t, outputFile2, "2.out output file should exist")
	})

	// Subtest for overwriting an existing task directory
	t.Run("should overwrite an existing task directory", func(t *testing.T) {
		// Modify the files for overwrite
		files["src/description.pdf"] = []byte("New task description content")
		files["src/input/1.in"] = []byte("New input content")
		files["src/output/1.out"] = []byte("New output content")

		// Attempt to overwrite the directory
		err := ts.CreateTaskDirectory(1, files, true)
		assert.NoError(t, err, "expected no error when overwriting the task directory")

		// Verify the files have been overwritten
		descriptionFile := filepath.Join(ts.taskDirectory, "task1", "src", "description.pdf")
		content, checkErr := os.ReadFile(descriptionFile)
		assert.NoError(t, checkErr, "expected no error reading description.pdf")
		assert.Equal(t, "New task description content", string(content), "description.pdf content should be overwritten")

		inputFile := filepath.Join(ts.taskDirectory, "task1", "src", "input", "1.in")
		outputFile := filepath.Join(ts.taskDirectory, "task1", "src", "output", "1.out")
		assert.FileExists(t, inputFile, "1.in should exist")
		assert.FileExists(t, outputFile, "1.out should exist")
	})

	// Subtest for when task directory exists and overwrite is not allowed
	t.Run("should return an error when directory exists and overwrite is false", func(t *testing.T) {
		// Attempt to create the directory again without overwrite
		err := ts.CreateTaskDirectory(1, files, false)
		assert.ErrorIs(t, err, ErrDirectoryAlreadyExists, "expected ErrDirectoryAlreadyExists error")
	})

	// Subtest for mismatched input/output files
	t.Run("should return an error when input and output files are mismatched", func(t *testing.T) {
		// Mock files with mismatched input and output files
		mismatchedFiles := map[string][]byte{
			"src/description.pdf": []byte("Task description content"),
			"src/input/1.in":      []byte("Input file 1 content"),
			// Missing output file, mismatching the number of input files
		}

		// Attempt to create the directory
		err := ts.CreateTaskDirectory(1, mismatchedFiles, true)
		assert.ErrorIs(t, err, ErrFailedValidateFiles, "expected ErrFailedValidateFiles error due to mismatched input/output files")
	})

	// Subtest for files with invalid naming format
	t.Run("should return an error when files do not follow {number}.in or {number}.out format", func(t *testing.T) {
		invalidNamingFiles := map[string][]byte{
			"src/description.pdf": []byte("Task description content"),
			"src/input/file1.in":  []byte("Input file with incorrect name"),
			"src/output/1.output": []byte("Output file with incorrect name"),
		}

		err := ts.CreateTaskDirectory(2, invalidNamingFiles, false)
		assert.ErrorIs(t, err, ErrFailedValidateFiles, "expected ErrFailedValidateFiles due to invalid naming format")
	})

	// Subtest to check if an error is returned for non-pdf description files
	t.Run("should return an error when description file is not a .pdf file", func(t *testing.T) {
		invalidFiles := map[string][]byte{
			"src/description.exe": []byte("Task description content"),
			"src/input/1.in":      []byte("Input file 1 content"),
			"src/output/1.out":    []byte("Output file 1 content"),
		}

		// Attempt to create the directory with invalid file formats
		err := ts.CreateTaskDirectory(3, invalidFiles, false)
		assert.ErrorIs(t, err, ErrFailedValidateFiles, "expected ErrFailedValidateFiles when description is not a .pdf file")
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

	tu := taskutils.NewTaskUtils(mockConfig)
	ts := NewTaskService(mockConfig, tu)

	// Define mock task files for input/output testing to create a valid task
	taskFiles := map[string][]byte{
		"src/description.pdf": []byte("Task description content"),
		"src/input/1.in":      []byte("Input file 1 content"),
		"src/output/1.out":    []byte("Output file 1 content"),
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
		userDir := filepath.Join(ts.taskDirectory, "task1", "submissions", "user1")
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

		content, checkErr := os.ReadFile(solutionFile)
		assert.NoError(t, checkErr, "expected no error when reading solution.c file")
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
		submissionDir := filepath.Join(ts.taskDirectory, "task1", "submissions", "user1", "submission2")
		assert.DirExists(t, submissionDir, "submission2 directory should exist")

		// Verify that the output directory exists
		outputDir := filepath.Join(submissionDir, "output")
		assert.DirExists(t, outputDir, "output directory should exist")

		// Verify that the solution file exists and contains the correct content
		solutionFile := filepath.Join(submissionDir, "solution.c")
		assert.FileExists(t, solutionFile, "solution.c file should exist")

		content, checkErr := os.ReadFile(solutionFile)
		assert.NoError(t, checkErr, "expected no error when reading solution.c file")
		assert.Equal(t, string(userFileContent), string(content), "solution.c content should match")
	})

	// Subtest for ensuring invalid file extensions return an error
	t.Run("should return an error for unsupported file extensions", func(t *testing.T) {
		userFileContent := []byte("#include <stdio.h>\nint main() { return 0; }")
		fileName := "solution.java" // Unsupported file extension

		// Attempt to create a submission with an unsupported file extension
		err := ts.CreateUserSubmission(1, 2, userFileContent, fileName)
		assert.ErrorIs(t, err, ErrFileExtensionNotAllowed, "expected ErrFileExtensionNotAllowed error for unsupported file extension")
	})

	// Subtest for creating submissions for multiple users
	t.Run("should create submissions for multiple users", func(t *testing.T) {
		userFileContent := []byte("int main() { return 42; }")
		fileName := "solution.c"

		// Create a submission for user 2
		err := ts.CreateUserSubmission(1, 2, userFileContent, fileName)
		assert.NoError(t, err, "expected no error when creating submission for user 2")

		// Verify the user directory exists
		userDir := filepath.Join(ts.taskDirectory, "task1", "submissions", "user2")
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

		content, checkErr := os.ReadFile(solutionFile)
		assert.NoError(t, checkErr, "expected no error when reading solution.c file")
		assert.Equal(t, string(userFileContent), string(content), "solution.c content should match")
	})

	// Subtest for trying to submit a file to a non-existent task
	t.Run("should return an error when trying to submit to a non-existent task", func(t *testing.T) {
		userFileContent := []byte("int main() { return 404; }")
		fileName := "solution.c"

		// Simulate the task directory not being created (taskID 999)
		err := ts.CreateUserSubmission(999, 1, userFileContent, fileName)
		assert.ErrorIs(t, err, ErrInvalidTaskID, "expected ErrInvalidTaskID error when trying to submit to a non-existent task")
	})
}

func TestStoreUserOutputs(t *testing.T) {
	rootDir, cleanup := createTempRootDir(t)
	defer cleanup()

	// Create a mock configuration with the temporary root directory
	mockConfig := &config.Config{
		RootDirectory: rootDir,
	}

	tu := taskutils.NewTaskUtils(mockConfig)
	ts := NewTaskService(mockConfig, tu)

	// Helper function to create a specific user submission directory
	createUserSubmissionDir := func(taskID, userID, submissionNumber int) {
		userSubmissionDir := filepath.Join(ts.taskDirectory, fmt.Sprintf("task%d", taskID), "submissions", fmt.Sprintf("user%d", userID), fmt.Sprintf("submission%d", submissionNumber), "output")
		err := os.MkdirAll(userSubmissionDir, os.ModePerm)
		assert.NoError(t, err, "failed to create user submission directory")
	}

	// Helper function to create the task's expected output directory with some .out files
	createExpectedOutputFiles := func(taskID int, count int) {
		expectedOutputDir := filepath.Join(ts.taskDirectory, fmt.Sprintf("task%d", taskID), "src", "output")
		err := os.MkdirAll(expectedOutputDir, os.ModePerm)
		assert.NoError(t, err, "failed to create expected output directory")

		for i := 1; i <= count; i++ {
			filePath := filepath.Join(expectedOutputDir, fmt.Sprintf("%d.out", i))
			err := os.WriteFile(filePath, []byte(fmt.Sprintf("Expected output %d", i)), 0644)
			assert.NoError(t, err, "failed to create expected output file %d", i)
		}
	}

	// Subtest for storing valid output files
	t.Run("should store valid output files in {number}.out format", func(t *testing.T) {
		taskID := 1
		userID := 1
		submissionNumber := 1

		// Set up expected output files
		createExpectedOutputFiles(taskID, 2)

		// Create the user submission directory for the task
		createUserSubmissionDir(taskID, userID, submissionNumber)

		// Output files to store
		outputFiles := map[string][]byte{
			"1.out": []byte("Output 1 content"),
			"2.out": []byte("Output 2 content"),
		}

		// Store output files
		err := ts.StoreUserOutputs(taskID, userID, submissionNumber, outputFiles)
		assert.NoError(t, err, "expected no error when storing valid output files")

		// Verify files are stored correctly
		outputDir := filepath.Join(ts.taskDirectory, "task1", "submissions", "user1", "submission1", "output")
		assert.FileExists(t, filepath.Join(outputDir, "1.out"), "First output file should exist as 1.out")
		assert.FileExists(t, filepath.Join(outputDir, "2.out"), "Second output file should exist as 2.out")
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
		outputDir := filepath.Join(ts.taskDirectory, "task2", "submissions", "user1", "submission1", "output")
		assert.FileExists(t, filepath.Join(outputDir, "compile-error.err"), "Compile-error file should exist")
	})

	// Subtest for error when trying to store non-.txt files
	t.Run("should return an error when trying to store wrong format of output files", func(t *testing.T) {
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
		assert.ErrorIs(t, err, ErrInvalidOutputFileFormat, "expected ErrInvalidOutputFileFormat when storing files with the wrong format")
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
		assert.ErrorIs(t, err, ErrOutputFileMismatch, "expected ErrOutputFileMismatch error when the number of outputs does not match task's expected outputs")
	})
}

func TestGetTaskFiles(t *testing.T) {
	rootDir, cleanup := createTempRootDir(t)
	defer cleanup()

	// Mock configuration
	mockConfig := &config.Config{RootDirectory: rootDir}
	tu := taskutils.NewTaskUtils(mockConfig)
	ts := NewTaskService(mockConfig, tu)

	// Helper function to set up a sample task directory structure with files for testing
	createSampleTaskDir := func(taskID int) {
		taskDir := filepath.Join(ts.taskDirectory, fmt.Sprintf("task%d", taskID), "src")
		inputDir := filepath.Join(taskDir, "input")
		outputDir := filepath.Join(taskDir, "output")

		// Create task directories and sample files
		err := os.MkdirAll(inputDir, os.ModePerm)
		assert.NoError(t, err, "failed to create input directory")

		err = os.MkdirAll(outputDir, os.ModePerm)
		assert.NoError(t, err, "failed to create output directory")

		err = os.WriteFile(filepath.Join(taskDir, "description.pdf"), []byte("Task description content"), 0644)
		assert.NoError(t, err, "failed to create description file")

		err = os.WriteFile(filepath.Join(inputDir, "1.in"), []byte("Input file 1 content"), 0644)
		assert.NoError(t, err, "failed to create input file")

		err = os.WriteFile(filepath.Join(outputDir, "1.out"), []byte("Output file 1 content"), 0644)
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
		tarFile, checkErr := os.Open(tarFilePath)
		assert.NoError(t, checkErr, "failed to open created .tar.gz file")
		defer utils.CloseIO(tarFile)

		// Initialize gzip and tar readers
		gzipReader, checkErr := gzip.NewReader(tarFile)
		assert.NoError(t, checkErr, "failed to create gzip reader")
		defer utils.CloseIO(gzipReader)

		tarReader := tar.NewReader(gzipReader)
		filesFound := map[string]bool{
			"task1Files/src/description.pdf": false,
			"task1Files/src/input/1.in":      false,
			"task1Files/src/output/1.out":    false,
		}

		for {
			header, checkErr := tarReader.Next()
			if checkErr != nil {
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
		assert.ErrorIs(t, err, ErrTaskSrcDirDoesNotExist, "expected ErrTaskSrcDirDoesNotExist when src directory is missing")
		assert.Empty(t, tarFilePath, "expected no tar file to be created when src directory is missing")
	})
}

func TestGetUserSubmission(t *testing.T) {
	// Create a temporary root directory for tests
	rootDir, cleanup := createTempRootDir(t)
	defer cleanup()

	// Create a mock configuration with the temporary root directory
	mockConfig := &config.Config{
		RootDirectory: rootDir,
	}

	// Initialize the TaskService with the mock configuration
	tu := taskutils.NewTaskUtils(mockConfig)
	ts := NewTaskService(mockConfig, tu)

	// Helper function to set up a submission directory and add a program file
	createSubmission := func(taskID, userID, submissionNum int, fileName, fileContent string) error {
		submissionDir := filepath.Join(ts.taskDirectory, fmt.Sprintf("task%d", taskID), "submissions", fmt.Sprintf("user%d", userID), fmt.Sprintf("submission%d", submissionNum))
		err := os.MkdirAll(submissionDir, os.ModePerm)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(submissionDir, fileName), []byte(fileContent), 0644)
	}

	// Subtest: Retrieve a single valid program file
	t.Run("should retrieve the program file for a valid submission", func(t *testing.T) {
		taskID := 1
		userID := 1
		submissionNum := 1
		programFileName := "solution.c"
		programContent := "int main() { return 0; }"

		// Set up a valid submission directory
		err := createSubmission(taskID, userID, submissionNum, programFileName, programContent)
		assert.NoError(t, err, "expected no error in creating submission directory")

		// Retrieve the submission
		content, _, err := ts.GetUserSubmission(taskID, userID, submissionNum)
		assert.NoError(t, err, "expected no error when retrieving the program file")
		assert.Equal(t, programContent, string(content), "program content should match")
	})

	// Subtest: Error when submission directory does not exist
	t.Run("should return an error if submission directory does not exist", func(t *testing.T) {
		taskID := 2
		userID := 1
		submissionNum := 1

		// Attempt to retrieve a submission from a non-existent directory
		_, _, err := ts.GetUserSubmission(taskID, userID, submissionNum)
		assert.ErrorIs(t, err, ErrSubmissionDirDoesNotExist, "expected ErrSubmissionDirDoesNotExist when submission directory does not exist")
	})

	// Subtest: Error when no program file exists in the submission directory
	t.Run("should return an error if no program file is found", func(t *testing.T) {
		taskID := 3
		userID := 1
		submissionNum := 1

		// Create an empty submission directory without a program file
		submissionDir := filepath.Join(ts.taskDirectory, "task3", "submissions", "user1", "submission1")
		err := os.MkdirAll(submissionDir, os.ModePerm)
		assert.NoError(t, err, "expected no error in creating empty submission directory")

		// Attempt to retrieve a program file from the empty directory
		_, _, err = ts.GetUserSubmission(taskID, userID, submissionNum)
		assert.ErrorIs(t, err, ErrNoProgramFileFound, "expected ErrNoProgramFileFound when no program file is found")
	})

	// Subtest: Error when multiple program files exist in the submission directory
	t.Run("should return an error if multiple program files are found", func(t *testing.T) {
		taskID := 4
		userID := 1
		submissionNum := 1
		programContent := "int main() { return 0; }"

		// Set up a submission directory with multiple program files
		err := createSubmission(taskID, userID, submissionNum, "solution1.c", programContent)
		assert.NoError(t, err, "expected no error in creating first program file")
		err = createSubmission(taskID, userID, submissionNum, "solution2.c", programContent)
		assert.NoError(t, err, "expected no error in creating second program file")

		// Attempt to retrieve the program file
		_, _, err = ts.GetUserSubmission(taskID, userID, submissionNum)
		assert.ErrorIs(t, err, ErrMultipleProgramFilesFound, "expected ErrMultipleProgramFilesFound when multiple program files are found")
	})
}

func TestGetInputOutput(t *testing.T) {
	// Set up a temporary root directory
	rootDir, cleanup := createTempRootDir(t)
	defer cleanup()

	// Initialize TaskService with the mock configuration
	mockConfig := &config.Config{
		RootDirectory: rootDir,
	}
	tu := taskutils.NewTaskUtils(mockConfig)
	ts := NewTaskService(mockConfig, tu)

	// Helper function to create input and output files for a task
	createInputOutputFiles := func(taskID, inputOutputID int) error {
		taskDir := filepath.Join(ts.taskDirectory, fmt.Sprintf("task%d", taskID), "src")
		inputDir := filepath.Join(taskDir, "input")
		outputDir := filepath.Join(taskDir, "output")

		// Create input and output directories and files
		err := os.MkdirAll(inputDir, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to create input directory: %v", err)
		}
		err = os.MkdirAll(outputDir, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to create output directory: %v", err)
		}

		// Create specific input and output files
		inputFilePath := filepath.Join(inputDir, fmt.Sprintf("%d.in", inputOutputID))
		outputFilePath := filepath.Join(outputDir, fmt.Sprintf("%d.out", inputOutputID))
		err = os.WriteFile(inputFilePath, []byte("Test input content"), 0644)
		if err != nil {
			return fmt.Errorf("failed to create input file: %v", err)
		}
		err = os.WriteFile(outputFilePath, []byte("Test output content"), 0644)
		if err != nil {
			return fmt.Errorf("failed to create output file: %v", err)
		}

		return nil
	}

	// Subtest for successfully retrieving input and output files
	t.Run("should retrieve specified input and output files in a tar.gz format", func(t *testing.T) {
		taskID := 1
		inputOutputID := 1

		// Set up task files
		err := createInputOutputFiles(taskID, inputOutputID)
		assert.NoError(t, err, "expected no error in creating input and output files")

		// Call GetInputOutput and verify result
		tarFilePath, err := ts.GetInputOutput(taskID, inputOutputID)
		assert.NoError(t, err, "expected no error retrieving input/output files")
		assert.FileExists(t, tarFilePath, "tar.gz file should be created")

		// Expected files in the tar.gz archive
		expectedFiles := map[string]string{
			fmt.Sprintf("%d.in", inputOutputID):  "Test input content",
			fmt.Sprintf("%d.out", inputOutputID): "Test output content",
		}

		// Validate the archive contents
		validateTarContents(t, tarFilePath, expectedFiles)
	})

	// Subtest for handling missing input file
	t.Run("should return an error if input file is missing", func(t *testing.T) {
		taskID := 2
		inputOutputID := 1

		// Only create the output file
		err := os.MkdirAll(filepath.Join(ts.taskDirectory, fmt.Sprintf("task%d/src/output", taskID)), os.ModePerm)
		assert.NoError(t, err)
		err = os.MkdirAll(filepath.Join(ts.taskDirectory, fmt.Sprintf("task%d/src/input", taskID)), os.ModePerm)
		assert.NoError(t, err)
		err = os.WriteFile(filepath.Join(ts.taskDirectory, fmt.Sprintf("task%d/src/output", taskID), fmt.Sprintf("%d.out", inputOutputID)), []byte("Output content"), 0644)
		assert.NoError(t, err)

		// Try to retrieve input/output and expect an error
		_, err = ts.GetInputOutput(taskID, inputOutputID)
		assert.ErrorIs(t, err, ErrInputFileDoesNotExist, "expected ErrInputFileDoesNotExist when input file is missing")
	})

	// Subtest for handling missing output file
	t.Run("should return an error if output file is missing", func(t *testing.T) {
		taskID := 3
		inputOutputID := 1

		// Only create the input file
		err := os.MkdirAll(filepath.Join(ts.taskDirectory, fmt.Sprintf("task%d/src/output", taskID)), os.ModePerm)
		assert.NoError(t, err)
		err = os.MkdirAll(filepath.Join(ts.taskDirectory, fmt.Sprintf("task%d/src/input", taskID)), os.ModePerm)
		assert.NoError(t, err)
		err = os.WriteFile(filepath.Join(ts.taskDirectory, fmt.Sprintf("task%d/src/input", taskID), fmt.Sprintf("%d.in", inputOutputID)), []byte("Input content"), 0644)
		assert.NoError(t, err)

		// Try to retrieve input/output and expect an error
		_, err = ts.GetInputOutput(taskID, inputOutputID)
		assert.ErrorIs(t, err, ErrOutputFileDoesNotExist, "expected ErrOutputFileDoesNotExist when output file is missing")
	})
}

func TestDeleteTask(t *testing.T) {
	// Set up a temporary root directory
	rootDir, cleanup := createTempRootDir(t)
	defer cleanup()

	// Initialize TaskService with the mock configuration
	mockConfig := &config.Config{
		RootDirectory: rootDir,
	}
	tu := taskutils.NewTaskUtils(mockConfig)
	ts := NewTaskService(mockConfig, tu)

	// Helper function to create a task directory with sample files
	createTaskDir := func(taskID int) error {
		taskDir := filepath.Join(ts.taskDirectory, fmt.Sprintf("task%d", taskID))
		err := os.MkdirAll(taskDir, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to create task directory: %v", err)
		}
		// Create a sample file within the task directory
		filePath := filepath.Join(taskDir, "sample.txt")
		err = os.WriteFile(filePath, []byte("sample content"), 0644)
		if err != nil {
			return fmt.Errorf("failed to create sample file: %v", err)
		}
		return nil
	}

	// Test for successful deletion of task directory
	t.Run("should delete the task directory and all its contents", func(t *testing.T) {
		taskID := 1
		err := createTaskDir(taskID)
		assert.NoError(t, err, "expected no error creating task directory")

		// Call DeleteTask and verify result
		err = ts.DeleteTask(taskID)
		assert.NoError(t, err, "expected no error deleting task directory")

		// Verify that the task directory no longer exists
		taskDir := filepath.Join(ts.taskDirectory, fmt.Sprintf("task%d", taskID))
		_, err = os.Stat(taskDir)
		assert.True(t, os.IsNotExist(err), "task directory should be deleted")
	})

	// Test for handling non-existent directory
	t.Run("should return an error if the task directory does not exist", func(t *testing.T) {
		taskID := 2

		// Call DeleteTask on a non-existent directory
		err := ts.DeleteTask(taskID)
		assert.ErrorIs(t, err, ErrInvalidTaskID, "expected ErrInvalidTaskID when the task directory does not exist")
	})

	// Test for handling directory deletion failure due to permissions
	t.Run("should return an error if directory cannot be deleted", func(t *testing.T) {
		taskID := 3
		err := createTaskDir(taskID)
		assert.NoError(t, err, "expected no error creating task directory")

		// Make the directory read-only to simulate a deletion failure
		taskDir := filepath.Join(ts.taskDirectory, fmt.Sprintf("task%d", taskID))
		err = os.Chmod(taskDir, 0444) // read-only permissions
		assert.NoError(t, err, "expected no error changing task directory permissions")

		// Attempt to delete and check for an error
		err = ts.DeleteTask(taskID)
		assert.ErrorIs(t, err, ErrFailedDeleteTaskDirectory, "expected ErrFailedDeleteTaskDirectory when directory deletion fails due to permissions")

		// Restore permissions to allow cleanup
		_ = os.Chmod(taskDir, 0755)
	})
}

func TestGetUserSolutionPackage(t *testing.T) {
	// Set up a temporary root directory
	rootDir, cleanup := createTempRootDir(t)
	defer cleanup()

	// Initialize TaskService with a mock configuration
	mockConfig := &config.Config{
		RootDirectory: rootDir,
	}
	tu := taskutils.NewTaskUtils(mockConfig)
	ts := NewTaskService(mockConfig, tu)

	// Helper function to create input, output, and solution files for a task and user submission
	createTaskFiles := func(taskID, userID, submissionNum int) error {
		taskDir := filepath.Join(ts.taskDirectory, fmt.Sprintf("task%d", taskID))
		inputDir := filepath.Join(taskDir, "src", "input")
		outputDir := filepath.Join(taskDir, "src", "output")
		solutionDir := filepath.Join(taskDir, "submissions", fmt.Sprintf("user%d", userID), fmt.Sprintf("submission%d", submissionNum))

		// Create directories
		if err := os.MkdirAll(inputDir, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create input directory: %v", err)
		}
		if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create output directory: %v", err)
		}
		if err := os.MkdirAll(solutionDir, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create solution directory: %v", err)
		}

		// Create sample input files
		for i := 1; i <= 3; i++ {
			inputFile := filepath.Join(inputDir, fmt.Sprintf("%d.in", i))
			if err := os.WriteFile(inputFile, []byte(fmt.Sprintf("input content %d", i)), 0644); err != nil {
				return fmt.Errorf("failed to create input file %d: %v", i, err)
			}
		}

		// Create sample output files
		for i := 1; i <= 3; i++ {
			outputFile := filepath.Join(outputDir, fmt.Sprintf("%d.out", i))
			if err := os.WriteFile(outputFile, []byte(fmt.Sprintf("output content %d", i)), 0644); err != nil {
				return fmt.Errorf("failed to create output file %d: %v", i, err)
			}
		}

		// Create sample solution file with arbitrary extension
		solutionFile := filepath.Join(solutionDir, "solution.c")
		if err := os.WriteFile(solutionFile, []byte("solution content"), 0644); err != nil {
			return fmt.Errorf("failed to create solution file: %v", err)
		}

		return nil
	}

	// Subtest for successful archive creation
	t.Run("should create a tar.gz archive with inputs, outputs, and solution", func(t *testing.T) {
		taskID := 1
		userID := 1
		submissionNum := 1

		// Set up files
		err := createTaskFiles(taskID, userID, submissionNum)
		assert.NoError(t, err, "expected no error in creating task files")

		// Call GetUserSolutionPackage and verify result
		tarFilePath, err := ts.GetUserSolutionPackage(taskID, userID, submissionNum)
		assert.NoError(t, err, "expected no error fetching user solution package")
		assert.FileExists(t, tarFilePath, "tar.gz file should be created")

		// Expected files in the tar.gz archive
		expectedFiles := map[string]string{
			"Task/inputs/1.in":   "input content 1",
			"Task/inputs/2.in":   "input content 2",
			"Task/inputs/3.in":   "input content 3",
			"Task/outputs/1.out": "output content 1",
			"Task/outputs/2.out": "output content 2",
			"Task/outputs/3.out": "output content 3",
			"Task/solution.c":    "solution content",
		}

		// Validate the archive contents
		validateTarContents(t, tarFilePath, expectedFiles)
	})

	// Subtest for missing input directory
	t.Run("should return an error if input directory is missing", func(t *testing.T) {
		taskID := 2
		userID := 1
		submissionNum := 1

		// Set up files without the input directory
		taskDir := filepath.Join(ts.taskDirectory, fmt.Sprintf("task%d", taskID))
		outputDir := filepath.Join(taskDir, "src", "output")
		solutionDir := filepath.Join(taskDir, "submissions", fmt.Sprintf("user%d", userID), fmt.Sprintf("submission%d", submissionNum))
		err := os.MkdirAll(outputDir, os.ModePerm)
		assert.NoError(t, err)
		err = os.MkdirAll(solutionDir, os.ModePerm)
		assert.NoError(t, err)

		_, err = ts.GetUserSolutionPackage(taskID, userID, submissionNum)
		assert.ErrorIs(t, err, ErrInputDirectoryDoesNotExist, "expected ErrInputDirectoryDoesNotExist for missing input directory")
	})

	// Subtest for missing output directory
	t.Run("should return an error if output directory is missing", func(t *testing.T) {
		taskID := 3
		userID := 1
		submissionNum := 1

		// Set up files without the output directory
		taskDir := filepath.Join(ts.taskDirectory, fmt.Sprintf("task%d", taskID))
		inputDir := filepath.Join(taskDir, "src", "input")
		solutionDir := filepath.Join(taskDir, "submissions", fmt.Sprintf("user%d", userID), fmt.Sprintf("submission%d", submissionNum))
		err := os.MkdirAll(inputDir, os.ModePerm)
		assert.NoError(t, err)
		err = os.MkdirAll(solutionDir, os.ModePerm)
		assert.NoError(t, err)

		_, err = ts.GetUserSolutionPackage(taskID, userID, submissionNum)
		assert.ErrorIs(t, err, ErrOutputDirectoryDoesNotExist, "expected ErrOutputDirectoryDoesNotExist for missing output directory")
	})

	// Subtest for missing solution file
	t.Run("should return an error if solution file is missing", func(t *testing.T) {
		taskID := 4
		userID := 1
		submissionNum := 1

		// Set up files without the solution file
		taskDir := filepath.Join(ts.taskDirectory, fmt.Sprintf("task%d", taskID))
		inputDir := filepath.Join(taskDir, "src", "input")
		outputDir := filepath.Join(taskDir, "src", "output")
		err := os.MkdirAll(inputDir, os.ModePerm)
		assert.NoError(t, err)
		err = os.MkdirAll(outputDir, os.ModePerm)
		assert.NoError(t, err)

		_, err = ts.GetUserSolutionPackage(taskID, userID, submissionNum)
		assert.ErrorIs(t, err, ErrSolutionFileDoesNotExist, "expected ErrSolutionFileDoesNotExist for missing solution file")
	})
}

// Helper function to validate the tar.gz contents
func validateTarContents(t *testing.T, tarFilePath string, expectedFiles map[string]string) {
	tarFile, err := os.Open(tarFilePath)
	assert.NoError(t, err, "expected no error opening tar.gz file")
	defer utils.CloseIO(tarFile)

	gzipReader, err := gzip.NewReader(tarFile)
	assert.NoError(t, err, "expected no error creating gzip reader")
	defer utils.CloseIO(gzipReader)

	tarReader := tar.NewReader(gzipReader)
	foundFiles := make(map[string]string)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		assert.NoError(t, err, "expected no error reading tar header")

		content, err := io.ReadAll(tarReader)
		assert.NoError(t, err, "expected no error reading file content")
		foundFiles[header.Name] = string(content)
	}

	// Check that each expected file is present and has the correct content
	for path, expectedContent := range expectedFiles {
		assert.Equal(t, expectedContent, foundFiles[path], fmt.Sprintf("file content for %s should match expected", path))
	}
}
