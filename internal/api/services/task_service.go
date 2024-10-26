package services

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"github.com/mini-maxit/file-storage/internal/api/taskutils"
	"github.com/mini-maxit/file-storage/utils"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/mini-maxit/file-storage/internal/config"
)

// TaskService handles operations related to task management.
type TaskService struct {
	config        *config.Config
	tu            *taskutils.TaskUtils
	taskDirectory string
}

// NewTaskService creates a new instance of TaskService with the provided configuration.
func NewTaskService(cfg *config.Config, tu *taskutils.TaskUtils) *TaskService {
	return &TaskService{
		config:        cfg,
		tu:            tu,
		taskDirectory: filepath.Join(cfg.RootDirectory, "tasks"),
	}
}

// CreateTaskDirectory creates a directory structure for a specific task.
// It creates a directory named `task{task_id}` containing the `src/`, `input/`, and `output/` folders.
// If the directory already exists, it backs it up, attempts to create a new one, and restores it on failure.
func (ts *TaskService) CreateTaskDirectory(taskID int, files map[string][]byte, overwrite bool) error {
	// Define the task directory path based on the task ID
	taskDir := filepath.Join(ts.taskDirectory, fmt.Sprintf("task%d", taskID))
	srcDir := filepath.Join(taskDir, "src")
	inputDir := filepath.Join(srcDir, "input")
	outputDir := filepath.Join(srcDir, "output")
	descriptionFile := filepath.Join(srcDir, "description.pdf")

	var backupDir string
	shouldRestore := false

	// Check if the task directory already exists
	if _, err := os.Stat(taskDir); err == nil {
		// Task directory already exists, handle backup and overwrite
		if overwrite {
			// Backup the existing directory to a temporary location
			backupDir, err = ts.tu.BackupDirectory(taskDir)
			shouldRestore = true
			if err != nil {
				return fmt.Errorf("failed to backup existing directory: %v", err)
			}

			// Remove the existing directory to prepare for the new structure
			err = os.RemoveAll(taskDir)
			if err != nil {
				// Clean up and return error if removal fails
				restoreError := ts.tu.RestoreDirectory(backupDir, taskDir)
				if restoreError != nil {
					return fmt.Errorf("failed to restore existing directory: %v \n restoring because: %v", restoreError, err)
				}
				return fmt.Errorf("failed to remove existing directory: %v", err)
			}
		} else {
			// If overwrite flag is set to false, return an error
			return errors.New("the task directory already exists, overwrite not allowed")
		}
	}

	// Create the required directory structure
	if err := ts.tu.CreateDirectoryStructure(srcDir, inputDir, outputDir); err != nil {
		// Restore the previous state if directory creation fails
		if shouldRestore {
			restoreError := ts.tu.RestoreDirectory(backupDir, taskDir)
			if restoreError != nil {
				return fmt.Errorf("failed to restore existing directory: %v \n restoring because: %v", restoreError, err)
			}
		}
		return err
	}

	// Validate the number of input and output files
	if err := ts.tu.ValidateFiles(files); err != nil {
		// Restore the previous state if validation fails
		if shouldRestore {
			restoreError := ts.tu.RestoreDirectory(backupDir, taskDir)
			if restoreError != nil {
				return fmt.Errorf("failed to restore existing directory: %v \n restoring because: %v", restoreError, err)
			}
		}
		return err
	}

	// Create the description.pdf file
	if err := os.WriteFile(descriptionFile, files["src/description.pdf"], 0644); err != nil {
		// Restore the previous state if writing description fails
		if shouldRestore {
			restoreError := ts.tu.RestoreDirectory(backupDir, taskDir)
			if restoreError != nil {
				return fmt.Errorf("failed to restore existing directory: %v \n restoring because: %v", restoreError, err)
			}
		}
		return fmt.Errorf("failed to create description.pdf: %v", err)
	}

	// Save input and output files
	if err := ts.tu.SaveFiles(inputDir, outputDir, files); err != nil {
		// Restore the previous state if saving files fails
		if shouldRestore {
			restoreError := ts.tu.RestoreDirectory(backupDir, taskDir)
			if restoreError != nil {
				return fmt.Errorf("failed to restore existing directory: %v \n restoring because: %v", restoreError, err)
			}
		}
		return err
	}

	// Remove the backup directory after successful operation
	if backupDir != "" {
		err := os.RemoveAll(backupDir)
		if err != nil {
			return fmt.Errorf("failed to remove backup directory: %v", err)
		}
	}

	return nil
}

// CreateUserSubmission creates a new submission directory for a user's task submission.
// It creates a directory `submissions/user{user_id}/submission{n}/`, where n is an incrementing submission number.
// It places the user's submission file (e.g., solution.{ext}) inside the submission folder
// and creates an empty `output/` folder for the generated output files.
func (ts *TaskService) CreateUserSubmission(taskID int, userID int, userFile []byte, fileName string) error {
	// Define paths
	taskDir := filepath.Join(ts.taskDirectory, fmt.Sprintf("task%d", taskID))
	submissionsDir := filepath.Join(taskDir, "submissions")
	userDir := filepath.Join(submissionsDir, fmt.Sprintf("user%d", userID))

	// Check whether task directory exists
	if _, err := os.Stat(taskDir); os.IsNotExist(err) {
		return errors.New("invalid taskID: task directory does not exist")
	}

	// Ensure the submissions directory exists
	if _, err := os.Stat(submissionsDir); os.IsNotExist(err) {
		err := os.MkdirAll(submissionsDir, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to create submissions directory: %v", err)
		}
	}

	// Ensure the user directory exists
	if _, err := os.Stat(userDir); os.IsNotExist(err) {
		err := os.MkdirAll(userDir, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to create user directory for user%d: %v", userID, err)
		}
	}

	// Get the file extension and validate it
	fileExtension := strings.ToLower(filepath.Ext(fileName))
	if fileExtension == "" {
		return fmt.Errorf("file has no extension")
	}

	if !ts.tu.IsAllowedFileExtension(fileExtension) {
		return fmt.Errorf("file extension '%s' is not allowed", fileExtension)
	}

	// Get the next submission number by counting existing submission directories
	submissionNumber, err := ts.tu.GetNextSubmissionNumber(userDir)
	if err != nil {
		return fmt.Errorf("failed to get next submission number: %v", err)
	}

	// Define the submission directory path
	submissionDir := filepath.Join(userDir, fmt.Sprintf("submission%d", submissionNumber))
	outputDir := filepath.Join(submissionDir, "output")

	// Create the submission directory and the empty output directory
	err = os.MkdirAll(outputDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create submission or output directory: %v", err)
	}

	// Save the user's file in the submission directory with the correct extension
	userFilePath := filepath.Join(submissionDir, "solution"+fileExtension)
	if err := os.WriteFile(userFilePath, userFile, 0644); err != nil {
		return fmt.Errorf("failed to save user file: %v", err)
	}

	return nil
}

// StoreUserOutputs saves output files generated by the user's program inside the appropriate output/ folder
// under the user's specific submission directory, validating format and matching the task's expected output files.
func (ts *TaskService) StoreUserOutputs(taskID int, userID int, submissionNumber int, outputFiles map[string][]byte) error {
	// Define paths for the task, user, and specific submission directories
	taskDir := filepath.Join(ts.taskDirectory, fmt.Sprintf("task%d", taskID))
	expectedOutputDir := filepath.Join(taskDir, "src", "output")
	userSubmissionDir := filepath.Join(taskDir, "submissions", fmt.Sprintf("user%d", userID), fmt.Sprintf("submission%d", submissionNumber))
	outputDir := filepath.Join(userSubmissionDir, "output")

	// Read expected output files from the task's src/output directory
	expectedFiles, err := os.ReadDir(expectedOutputDir)
	if err != nil {
		return fmt.Errorf("failed to read expected output directory for task %d: %v", taskID, err)
	}

	// Ensure user submission directory exists
	if _, err := os.Stat(userSubmissionDir); os.IsNotExist(err) {
		return fmt.Errorf("submission directory does not exist for task %d, user %d, submission %d", taskID, userID, submissionNumber)
	}

	// Verify if the output directory already has files
	if _, err := os.Stat(outputDir); err == nil {
		entries, err := os.ReadDir(outputDir)
		if err != nil {
			return fmt.Errorf("failed to read output directory: %v", err)
		}
		if len(entries) > 0 {
			return fmt.Errorf("output directory already contains files for task %d, user %d, submission %d", taskID, userID, submissionNumber)
		}
	} else if os.IsNotExist(err) {
		// Create the output directory if it doesn't exist
		if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create output directory: %v", err)
		}
	} else {
		return fmt.Errorf("failed to access output directory: %v", err)
	}

	// If there's only one file named "compile-error.err", save it and return
	if len(outputFiles) == 1 {
		for fileName := range outputFiles {
			if fileName == "compile-error.err" {
				return ts.tu.SaveCompileErrorFile(outputDir, outputFiles[fileName])
			}
		}
	}

	// Map expected output numbers from the task's output directory
	expectedOutputCount := 0
	expectedNumbers := make(map[int]bool)
	for _, file := range expectedFiles {
		if matches := regexp.MustCompile(`^(\d+)\.out$`).FindStringSubmatch(file.Name()); matches != nil {
			num, _ := strconv.Atoi(matches[1])
			expectedNumbers[num] = true
			expectedOutputCount++
		}
	}

	// Verify the count of provided output files matches the expected count
	if len(outputFiles) != expectedOutputCount {
		return fmt.Errorf("number of output files does not match the expected number (%d) for task %d", expectedOutputCount, taskID)
	}

	// Track user-provided output numbers to avoid duplicates
	userOutputNumbers := make(map[int]bool)

	// Save output files in the original name with the {number}.out format
	for fileName, fileContent := range outputFiles {
		baseName := filepath.Base(fileName)
		matches := regexp.MustCompile(`^(\d+)\.out$`).FindStringSubmatch(baseName)
		if matches == nil {
			return fmt.Errorf("output file %s does not match the required format {number}.out", baseName)
		}

		num, err := strconv.Atoi(matches[1])
		if err != nil {
			return fmt.Errorf("invalid output file number in %s: %v", baseName, err)
		}

		// Ensure there are no duplicate numbers among the user files
		if userOutputNumbers[num] {
			return fmt.Errorf("duplicate output file number %d found in user submission", num)
		}
		userOutputNumbers[num] = true

		// Ensure the output file number matches expected output files
		if !expectedNumbers[num] {
			return fmt.Errorf("unexpected output file number %d provided", num)
		}

		// Save the output file in the output directory
		if err := os.WriteFile(filepath.Join(outputDir, baseName), fileContent, 0644); err != nil {
			return fmt.Errorf("failed to save output file %s: %v", fileName, err)
		}
	}

	return nil
}

// GetTaskFiles retrieves all files (description, input, and output) for a given task and returns them in a .tar.gz file.
// This function is useful for fetching the entire task content, preserving the folder structure.
func (ts *TaskService) GetTaskFiles(taskID int) (string, error) {
	// Define paths for the task and src directories
	taskDir := filepath.Join(ts.taskDirectory, fmt.Sprintf("task%d", taskID))
	srcDir := filepath.Join(taskDir, "src")

	// Check if the src directory exists
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return "", fmt.Errorf("task src directory does not exist for task %d", taskID)
	}

	// Create a temporary file for the TAR.GZ archive
	tarFilePath := filepath.Join(os.TempDir(), fmt.Sprintf("task%dFiles.tar.gz", taskID))
	tarFile, err := os.Create(tarFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to create temporary tar file: %v", err)
	}
	defer utils.CloseIO(tarFile)

	// Initialize gzip writer
	gzipWriter := gzip.NewWriter(tarFile)
	defer utils.CloseIO(gzipWriter)

	// Initialize tar writer
	tarWriter := tar.NewWriter(gzipWriter)
	defer utils.CloseIO(tarWriter)

	// Walk through the src directory and add files to the TAR archive
	err = filepath.Walk(srcDir, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Determine relative path for maintaining directory structure
		relPath, err := filepath.Rel(filepath.Dir(srcDir), filePath) // root folder for src
		if err != nil {
			return fmt.Errorf("failed to determine relative path for file %s: %v", filePath, err)
		}

		// Set up the TAR header
		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return fmt.Errorf("failed to create tar header for file %s: %v", filePath, err)
		}
		header.Name = filepath.Join(fmt.Sprintf("task%dFiles", taskID), relPath)

		// Write the header
		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write tar header for file %s: %v", filePath, err)
		}

		// If it's a directory, skip writing the content
		if info.IsDir() {
			return nil
		}

		// Write the file content
		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("failed to open file %s: %v", filePath, err)
		}
		defer utils.CloseIO(file)

		if _, err := io.Copy(tarWriter, file); err != nil {
			return fmt.Errorf("failed to write file %s to tar: %v", filePath, err)
		}

		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to add files to tar: %v", err)
	}

	// Return the path to the created TAR.GZ file
	return tarFilePath, nil
}

// GetUserSubmission fetches the specific submission file for a user in a given task.
func (ts *TaskService) GetUserSubmission(taskID int, userID int, submissionNum int) ([]byte, string, error) {
	// Define the path to the specific submission directory
	submissionDir := filepath.Join(ts.taskDirectory, fmt.Sprintf("task%d", taskID), "submissions", fmt.Sprintf("user%d", userID), fmt.Sprintf("submission%d", submissionNum))

	// Check if the submission directory exists
	if _, err := os.Stat(submissionDir); os.IsNotExist(err) {
		return nil, "", fmt.Errorf("submission directory does not exist for task %d, user %d, submission %d", taskID, userID, submissionNum)
	}

	// Read files in the submission directory to locate the program file
	files, err := os.ReadDir(submissionDir)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read submission directory: %v", err)
	}

	// Find the single program file in the directory
	var programFile string
	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), "solution") {
			if programFile != "" {
				return nil, "", fmt.Errorf("multiple program files found in submission %d for user %d in task %d", submissionNum, userID, taskID)
			}
			programFile = file.Name()
		}
	}

	// Check if a program file was found
	if programFile == "" {
		return nil, "", fmt.Errorf("no program file found in submission %d for user %d in task %d", submissionNum, userID, taskID)
	}

	// Read the content of the program file
	programFilePath := filepath.Join(submissionDir, programFile)
	fileContent, err := os.ReadFile(programFilePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read program file %s: %v", programFile, err)
	}

	return fileContent, programFile, nil
}

// GetInputOutput retrieves the specific input and output files for a given task and returns them in a .tar.gz archive.
// This is useful for accessing specific input/output pairs based on their ID.
func (ts *TaskService) GetInputOutput(taskID int, inputOutputID int) (string, error) {
	// Define paths for the task and the specific input/output directories
	taskDir := filepath.Join(ts.taskDirectory, fmt.Sprintf("task%d", taskID))
	inputDir := filepath.Join(taskDir, "src", "input")
	outputDir := filepath.Join(taskDir, "src", "output")

	// Check if the task's input and output directories exist
	if _, err := os.Stat(inputDir); os.IsNotExist(err) {
		return "", fmt.Errorf("input directory does not exist for task %d", taskID)
	}
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		return "", fmt.Errorf("output directory does not exist for task %d", taskID)
	}

	// Locate specific input and output files based on inputOutputID
	inputFilePath := filepath.Join(inputDir, fmt.Sprintf("%d.in", inputOutputID))
	outputFilePath := filepath.Join(outputDir, fmt.Sprintf("%d.out", inputOutputID))

	// Ensure the input and output files exist
	if _, err := os.Stat(inputFilePath); os.IsNotExist(err) {
		return "", fmt.Errorf("input file %d.in does not exist for task %d", inputOutputID, taskID)
	}
	if _, err := os.Stat(outputFilePath); os.IsNotExist(err) {
		return "", fmt.Errorf("output file %d.out does not exist for task %d", inputOutputID, taskID)
	}

	// Create a temporary .tar.gz file
	tarFilePath := filepath.Join(os.TempDir(), fmt.Sprintf("task%d_inputOutput%d.tar.gz", taskID, inputOutputID))
	tarFile, err := os.Create(tarFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to create temporary tar file: %v", err)
	}
	defer utils.CloseIO(tarFile)

	// Initialize gzip writer
	gzipWriter := gzip.NewWriter(tarFile)
	defer utils.CloseIO(gzipWriter)

	// Initialize tar writer
	tarWriter := tar.NewWriter(gzipWriter)
	defer utils.CloseIO(tarWriter)

	// Add input and output files to the TAR archive with only the base filename
	for _, filePath := range []string{inputFilePath, outputFilePath} {
		// Open the file to read content
		file, err := os.Open(filePath)
		if err != nil {
			return "", fmt.Errorf("failed to open file %s: %v", filePath, err)
		}

		// Gather file info and set up the TAR header
		info, err := file.Stat()
		if err != nil {
			return "", fmt.Errorf("failed to get file info for %s: %v", filePath, err)
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return "", fmt.Errorf("failed to create tar header for file %s: %v", filePath, err)
		}
		// Use only the base filename for header.Name to avoid folder structure
		header.Name = info.Name()

		// Write the header and file content to the TAR archive
		if err := tarWriter.WriteHeader(header); err != nil {
			return "", fmt.Errorf("failed to write tar header for file %s: %v", filePath, err)
		}
		if _, err := io.Copy(tarWriter, file); err != nil {
			return "", fmt.Errorf("failed to write file %s to tar: %v", filePath, err)
		}

		utils.CloseIO(file)
	}

	// Return the path to the created TAR.GZ file
	return tarFilePath, nil
}

// DeleteTask deletes the directory of a specific task, including all associated files and submissions.
func (ts *TaskService) DeleteTask(taskID int) error {
	// Construct the task directory path
	taskDir := filepath.Join(ts.taskDirectory, fmt.Sprintf("task%d", taskID))

	// Check if the task directory exists
	_, err := os.Stat(taskDir)
	if os.IsNotExist(err) {
		return fmt.Errorf("no directory exists for task %d", taskID)
	} else if err != nil {
		return fmt.Errorf("failed to access task directory for task %d: %v", taskID, err)
	}

	// Attempt to remove the task directory and all its contents
	err = os.RemoveAll(taskDir)
	if err != nil {
		return fmt.Errorf("failed to delete task directory for task %d: %v", taskID, err)
	}

	return nil
}

// GetUserSolutionPackage fetches the specific package for a given task, user, and submission number,
// organizing it in a structured .tar.gz archive containing inputs, outputs, and the solution file.
func (ts *TaskService) GetUserSolutionPackage(taskID, userID, submissionNum int) (string, error) {
	// Define paths for the task directories and files
	taskDir := filepath.Join(ts.taskDirectory, fmt.Sprintf("task%d", taskID))
	inputDir := filepath.Join(taskDir, "src", "input")
	outputDir := filepath.Join(taskDir, "src", "output")
	solutionPattern := filepath.Join(taskDir, "submissions", fmt.Sprintf("user%d", userID), fmt.Sprintf("submission%d", submissionNum), "solution.*")

	// Check if the input and output directories exist
	if _, err := os.Stat(inputDir); os.IsNotExist(err) {
		return "", fmt.Errorf("input directory does not exist for task %d", taskID)
	}
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		return "", fmt.Errorf("output directory does not exist for task %d", taskID)
	}

	// Find the solution file with any extension
	solutionFiles, err := filepath.Glob(solutionPattern)
	if err != nil {
		return "", fmt.Errorf("failed to search for solution file: %v", err)
	}
	if len(solutionFiles) == 0 {
		return "", fmt.Errorf("solution file does not exist for user %d, submission %d of task %d", userID, submissionNum, taskID)
	}
	if len(solutionFiles) > 1 {
		return "", fmt.Errorf("multiple solution files found for user %d, submission %d of task %d", userID, submissionNum, taskID)
	}
	solutionFile := solutionFiles[0]

	// Create a temporary .tar.gz file to store the package
	tarFilePath := filepath.Join(os.TempDir(), fmt.Sprintf("task%d_user%d_submission%d_package.tar.gz", taskID, userID, submissionNum))
	tarFile, err := os.Create(tarFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to create temporary tar file: %v", err)
	}
	defer utils.CloseIO(tarFile)

	// Initialize gzip and tar writers
	gzipWriter := gzip.NewWriter(tarFile)
	defer utils.CloseIO(gzipWriter)

	tarWriter := tar.NewWriter(gzipWriter)
	defer utils.CloseIO(tarWriter)

	// Function to add files to the archive with specified path
	addFileToTar := func(filePath, tarPath string) error {
		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("failed to open file %s: %v", filePath, err)
		}
		defer utils.CloseIO(file)

		info, err := file.Stat()
		if err != nil {
			return fmt.Errorf("failed to get file info for %s: %v", filePath, err)
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("failed to create tar header for file %s: %v", filePath, err)
		}

		header.Name = tarPath // Use provided tarPath for directory structure in archive

		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write tar header for file %s: %v", filePath, err)
		}

		if _, err := io.Copy(tarWriter, file); err != nil {
			return fmt.Errorf("failed to write file %s to tar: %v", filePath, err)
		}

		return nil
	}

	// Add input files to the "inputs/" folder in the tar
	inputFiles, err := filepath.Glob(filepath.Join(inputDir, "*.in"))
	if err != nil {
		return "", fmt.Errorf("failed to read input files: %v", err)
	}
	for _, filePath := range inputFiles {
		fileName := filepath.Base(filePath)
		err := addFileToTar(filePath, filepath.Join("Task", "inputs", fileName))
		if err != nil {
			return "", err
		}
	}

	// Add output files to the "outputs/" folder in the tar
	outputFiles, err := filepath.Glob(filepath.Join(outputDir, "*.out"))
	if err != nil {
		return "", fmt.Errorf("failed to read output files: %v", err)
	}
	for _, filePath := range outputFiles {
		fileName := filepath.Base(filePath)
		err := addFileToTar(filePath, filepath.Join("Task", "outputs", fileName))
		if err != nil {
			return "", err
		}
	}

	// Add the solution file to the tar, preserving its original extension
	err = addFileToTar(solutionFile, filepath.Join("Task", filepath.Base(solutionFile)))
	if err != nil {
		return "", err
	}

	// Return the path to the created .tar.gz file
	return tarFilePath, nil
}
