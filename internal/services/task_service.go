package services

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mini-maxit/file-storage/internal/config"
)

// TaskService handles operations related to task management.
type TaskService struct {
	config *config.Config
}

// NewTaskService creates a new instance of TaskService with the provided configuration.
func NewTaskService(cfg *config.Config) *TaskService {
	return &TaskService{
		config: cfg,
	}
}

// CreateTaskDirectory creates a directory structure for a specific task.
// It creates a directory named `task{task_id}` containing the `src/`, `input/`, and `output/` folders.
// If the directory already exists, it backs it up, attempts to create a new one, and restores it on failure.
func (ts *TaskService) CreateTaskDirectory(taskID int, files map[string][]byte, overwrite bool) error {
	// Define the task directory path based on the task ID
	taskDir := filepath.Join(ts.config.RootDirectory, fmt.Sprintf("task%d", taskID))
	srcDir := filepath.Join(taskDir, "src")
	inputDir := filepath.Join(srcDir, "input")
	outputDir := filepath.Join(srcDir, "output")
	descriptionFile := filepath.Join(srcDir, "description.pdf")

	var backupDir string

	// Check if the task directory already exists
	if _, err := os.Stat(taskDir); err == nil {
		// Task directory already exists, handle backup and overwrite
		if overwrite {
			// Backup the existing directory to a temporary location
			backupDir, err = ts.backupDirectory(taskDir)
			if err != nil {
				return fmt.Errorf("failed to backup existing directory: %v", err)
			}

			// Remove the existing directory to prepare for the new structure
			err = os.RemoveAll(taskDir)
			if err != nil {
				// Clean up and return error if removal fails
				restoreError := ts.restoreDirectory(backupDir, taskDir)
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
	if err := ts.createDirectoryStructure(srcDir, inputDir, outputDir); err != nil {
		// Restore the previous state if directory creation fails
		restoreError := ts.restoreDirectory(backupDir, taskDir)
		if restoreError != nil {
			return fmt.Errorf("failed to restore existing directory: %v \n restoring because: %v", restoreError, err)
		}
		return err
	}

	// Validate the number of input and output files
	if err := ts.validateFiles(files); err != nil {
		// Restore the previous state if validation fails
		restoreError := ts.restoreDirectory(backupDir, taskDir)
		if restoreError != nil {
			return fmt.Errorf("failed to restore existing directory: %v \n restoring because: %v", restoreError, err)
		}
		return err
	}

	// Create the description.pdf file
	if err := os.WriteFile(descriptionFile, files["src/description.pdf"], 0644); err != nil {
		// Restore the previous state if writing description fails
		restoreError := ts.restoreDirectory(backupDir, taskDir)
		if restoreError != nil {
			return fmt.Errorf("failed to restore existing directory: %v \n restoring because: %v", restoreError, err)
		}
		return fmt.Errorf("failed to create description.pdf: %v", err)
	}

	// Save input and output files
	if err := ts.saveFiles(inputDir, outputDir, files); err != nil {
		// Restore the previous state if saving files fails
		restoreError := ts.restoreDirectory(backupDir, taskDir)
		if restoreError != nil {
			return fmt.Errorf("failed to restore existing directory: %v \n restoring because: %v", restoreError, err)
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
	taskDir := filepath.Join(ts.config.RootDirectory, fmt.Sprintf("task%d", taskID))
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

	if !ts.isAllowedFileExtension(fileExtension) {
		return fmt.Errorf("file extension '%s' is not allowed", fileExtension)
	}

	// Get the next submission number by counting existing submission directories
	submissionNumber, err := ts.getNextSubmissionNumber(userDir)
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
// under the user's specific submission directory.
func (ts *TaskService) StoreUserOutputs(taskID int, userID int, submissionNumber int, outputFiles map[string][]byte) error {
	// Define paths for the task, user, and specific submission directories
	taskDir := filepath.Join(ts.config.RootDirectory, fmt.Sprintf("task%d", taskID))
	expectedOutputDir := filepath.Join(taskDir, "src", "output")
	userSubmissionDir := filepath.Join(taskDir, "submissions", fmt.Sprintf("user%d", userID), fmt.Sprintf("submission%d", submissionNumber))
	outputDir := filepath.Join(userSubmissionDir, "output")

	// Count expected output files in the task's src/output directory
	expectedFiles, err := os.ReadDir(expectedOutputDir)
	if err != nil {
		return fmt.Errorf("failed to read expected output directory for task %d: %v", taskID, err)
	}

	// Check if the specific user submission directory exists
	if _, err := os.Stat(userSubmissionDir); os.IsNotExist(err) {
		return fmt.Errorf("submission directory does not exist for task %d, user %d, submission %d", taskID, userID, submissionNumber)
	}

	// Check if output files already exist in the output directory
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

	// Validate if there's only one file named "compile-error.err"
	if len(outputFiles) == 1 {
		for fileName := range outputFiles {
			if fileName == "compile-error.err" {
				// Save the compile-error.err file directly in the output directory
				return ts.saveCompileErrorFile(outputDir, outputFiles[fileName])
			}
		}
	}

	// Count the number of expected output files
	expectedOutputCount := 0
	for _, file := range expectedFiles {
		if filepath.Ext(file.Name()) == ".txt" {
			expectedOutputCount++
		}
	}

	// Check if the number of provided output files matches the expected count
	if len(outputFiles) != expectedOutputCount {
		return fmt.Errorf("number of output files does not match the expected number (%d) for task %d", expectedOutputCount, taskID)
	}

	// Save the output files in the {number}.out.txt format
	outputCount := 1
	for fileName, fileContent := range outputFiles {
		if filepath.Ext(fileName) != ".txt" {
			return fmt.Errorf("only .txt files or 'compile-error.err' are allowed in output files")
		}
		newFileName := fmt.Sprintf("%d.out.txt", outputCount)
		if err := os.WriteFile(filepath.Join(outputDir, newFileName), fileContent, 0644); err != nil {
			return fmt.Errorf("failed to save output file %s: %v", fileName, err)
		}
		outputCount++
	}

	return nil
}

// GetTaskFiles retrieves all files (description, input, and output) for a given task and returns them in a .tar.gz file.
// This function is useful for fetching the entire task content, preserving the folder structure.
func (ts *TaskService) GetTaskFiles(taskID int) (string, error) {
	// Define paths for the task and src directories
	taskDir := filepath.Join(ts.config.RootDirectory, fmt.Sprintf("task%d", taskID))
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
	defer tarFile.Close()

	// Initialize gzip writer
	gzipWriter := gzip.NewWriter(tarFile)
	defer gzipWriter.Close()

	// Initialize tar writer
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

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
		defer file.Close()

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
	submissionDir := filepath.Join(ts.config.RootDirectory, fmt.Sprintf("task%d", taskID), "submissions", fmt.Sprintf("user%d", userID), fmt.Sprintf("submission%d", submissionNum))

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
	taskDir := filepath.Join(ts.config.RootDirectory, fmt.Sprintf("task%d", taskID))
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
	inputFilePath := filepath.Join(inputDir, fmt.Sprintf("%d.in.txt", inputOutputID))
	outputFilePath := filepath.Join(outputDir, fmt.Sprintf("%d.out.txt", inputOutputID))

	// Ensure the input and output files exist
	if _, err := os.Stat(inputFilePath); os.IsNotExist(err) {
		return "", fmt.Errorf("input file %d.in.txt does not exist for task %d", inputOutputID, taskID)
	}
	if _, err := os.Stat(outputFilePath); os.IsNotExist(err) {
		return "", fmt.Errorf("output file %d.out.txt does not exist for task %d", inputOutputID, taskID)
	}

	// Create a temporary .tar.gz file
	tarFilePath := filepath.Join(os.TempDir(), fmt.Sprintf("task%d_inputOutput%d.tar.gz", taskID, inputOutputID))
	tarFile, err := os.Create(tarFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to create temporary tar file: %v", err)
	}
	defer tarFile.Close()

	// Initialize gzip writer
	gzipWriter := gzip.NewWriter(tarFile)
	defer gzipWriter.Close()

	// Initialize tar writer
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	// Add input and output files to the TAR archive with only the base filename
	for _, filePath := range []string{inputFilePath, outputFilePath} {
		// Open the file to read content
		file, err := os.Open(filePath)
		if err != nil {
			return "", fmt.Errorf("failed to open file %s: %v", filePath, err)
		}
		defer file.Close()

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
	}

	// Return the path to the created TAR.GZ file
	return tarFilePath, nil
}

// DeleteTask deletes the directory of a specific task, including all associated files and submissions.
func (ts *TaskService) DeleteTask(taskID int) error {
	// Construct the task directory path
	taskDir := filepath.Join(ts.config.RootDirectory, fmt.Sprintf("task%d", taskID))

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
	taskDir := filepath.Join(ts.config.RootDirectory, fmt.Sprintf("task%d", taskID))
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
	defer tarFile.Close()

	// Initialize gzip and tar writers
	gzipWriter := gzip.NewWriter(tarFile)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	// Function to add files to the archive with specified path
	addFileToTar := func(filePath, tarPath string) error {
		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("failed to open file %s: %v", filePath, err)
		}
		defer file.Close()

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
	inputFiles, err := filepath.Glob(filepath.Join(inputDir, "*.in.txt"))
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
	outputFiles, err := filepath.Glob(filepath.Join(outputDir, "*.out.txt"))
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

// backupDirectory creates a backup of an existing directory in a temporary location.
func (ts *TaskService) backupDirectory(taskDir string) (string, error) {
	backupDir, err := os.MkdirTemp("", "task_backup_*")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary backup directory: %v", err)
	}

	err = copyDir(taskDir, backupDir)
	if err != nil {
		return "", fmt.Errorf("failed to copy directory for backup: %v", err)
	}

	return backupDir, nil
}

// restoreDirectory restores a backup directory to the original task directory.
func (ts *TaskService) restoreDirectory(backupDir, taskDir string) error {
	err := os.RemoveAll(taskDir)
	if err != nil {
		return fmt.Errorf("failed to remove incomplete task directory for restoration: %v", err)
	}

	err = copyDir(backupDir, taskDir)
	if err != nil {
		return fmt.Errorf("failed to restore task directory from backup: %v", err)
	}

	return nil
}

// copyDir copies the contents of one directory to another.
func copyDir(srcDir, destDir string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(destDir, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		return copyFile(path, destPath)
	})
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	// Ensure permissions match
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, info.Mode())
}

// createDirectoryStructure creates the required directory structure for a task.
func (ts *TaskService) createDirectoryStructure(srcDir, inputDir, outputDir string) error {
	// Create src, input, and output directories
	if err := os.MkdirAll(srcDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create src directory: %v", err)
	}
	if err := os.MkdirAll(inputDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create input directory: %v", err)
	}
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}
	return nil
}

// validateFiles checks if the number of input and output files matches and if all files have a .txt extension.
func (ts *TaskService) validateFiles(files map[string][]byte) error {
	var inputFiles, outputFiles []string

	for fileName := range files {
		if strings.HasPrefix(fileName, "src/input/") {
			if filepath.Ext(fileName) != ".txt" {
				return errors.New("only .txt files are allowed for input files")
			}
			inputFiles = append(inputFiles, fileName)
		} else if strings.HasPrefix(fileName, "src/output/") {
			if filepath.Ext(fileName) != ".txt" {
				return errors.New("only .txt files are allowed for output files")
			}
			outputFiles = append(outputFiles, fileName)
		} else {
			if filepath.Ext(fileName) != ".pdf" {
				return errors.New("description must have a .pdf extension")
			}
		}
	}

	if len(inputFiles) != len(outputFiles) {
		return errors.New("the number of input files must match the number of output files")
	}

	return nil
}

// saveFiles saves input and output files to their respective directories in the {number}.in.txt and {number}.out.txt format.
func (ts *TaskService) saveFiles(inputDir, outputDir string, files map[string][]byte) error {
	inputCount := 1
	outputCount := 1

	// Save input files with the {number}.in.txt format
	for fileName, fileContent := range files {
		if strings.HasPrefix(fileName, "src/input/") {
			newFileName := fmt.Sprintf("%d.in.txt", inputCount)
			if err := os.WriteFile(filepath.Join(inputDir, newFileName), fileContent, 0644); err != nil {
				return fmt.Errorf("failed to save input file %s: %v", fileName, err)
			}
			inputCount++
		}
	}

	// Save output files with the {number}.out.txt format
	for fileName, fileContent := range files {
		if strings.HasPrefix(fileName, "src/output/") {
			newFileName := fmt.Sprintf("%d.out.txt", outputCount)
			if err := os.WriteFile(filepath.Join(outputDir, newFileName), fileContent, 0644); err != nil {
				return fmt.Errorf("failed to save output file %s: %v", fileName, err)
			}
			outputCount++
		}
	}

	return nil
}

// getNextSubmissionNumber determines the next submission number for a user by counting existing submissions.
func (ts *TaskService) getNextSubmissionNumber(userDir string) (int, error) {
	entries, err := os.ReadDir(userDir)
	if err != nil {
		return 0, fmt.Errorf("failed to read user directory: %v", err)
	}

	submissionCount := 0
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "submission") {
			submissionCount++
		}
	}

	return submissionCount + 1, nil
}

// isAllowedFileExtension checks if the given file extension is in the allowed list from the configuration.
func (ts *TaskService) isAllowedFileExtension(extension string) bool {
	for _, allowedExtension := range ts.config.AllowedFileTypes {
		if extension == allowedExtension {
			return true
		}
	}
	return false
}

// saveCompileErrorFile saves the compile-error.err file in the output directory
func (ts *TaskService) saveCompileErrorFile(outputDir string, fileContent []byte) error {
	filePath := filepath.Join(outputDir, "compile-error.err")
	if err := os.WriteFile(filePath, fileContent, 0644); err != nil {
		return fmt.Errorf("failed to save compile-error.err: %v", err)
	}
	return nil
}
