package services

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/mini-maxit/file-storage/internal/api/taskutils"
	"github.com/mini-maxit/file-storage/utils"

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
func (ts *TaskService) CreateTaskDirectory(taskID int, files map[string][]byte, overwrite bool) ServiceError {
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
		if !overwrite {
			// If overwrite flag is set to false, return an error
			return ErrDirectoryAlreadyExists
		}

		// Backup the existing directory to a temporary location
		backupDir, err = ts.tu.BackupDirectory(taskDir)
		shouldRestore = true
		if err != nil {
			return ErrFailedBackupDirectory
		}

		// Remove the existing directory to prepare for the new structure
		err = os.RemoveAll(taskDir)
		if err != nil {
			// Clean up and return error if removal fails
			restoreError := ts.tu.RestoreDirectory(backupDir, taskDir)
			if restoreError != nil {
				return ErrFailedRestoreDirectory
			}
			return ErrFailedRemoveDirectory
		}
	}

	// Create the required directory structure
	if err := ts.tu.CreateDirectoryStructure(srcDir, inputDir, outputDir); err != nil {
		// Restore the previous state if directory creation fails
		if shouldRestore {
			restoreError := ts.tu.RestoreDirectory(backupDir, taskDir)
			if restoreError != nil {
				return ErrFailedRestoreDirectory
			}
		}
		return ErrFailedCreateDirectory
	}

	// Validate the number of input and output files
	if err := ts.tu.ValidateFiles(files); err != nil {
		// Restore the previous state if validation fails
		if shouldRestore {
			restoreError := ts.tu.RestoreDirectory(backupDir, taskDir)
			if restoreError != nil {
				return ErrFailedRestoreDirectory
			}
		}
		return &InternalServerError{err.Error()}
	}

	// Create the description.pdf file
	if err := os.WriteFile(descriptionFile, files["src/description.pdf"], 0644); err != nil {
		// Restore the previous state if writing description fails
		if shouldRestore {
			restoreError := ts.tu.RestoreDirectory(backupDir, taskDir)
			if restoreError != nil {
				return ErrFailedRestoreDirectory
			}
		}
		return ErrFailedCreateDescription
	}

	// Save input and output files
	if err := ts.tu.SaveFiles(inputDir, outputDir, files); err != nil {
		// Restore the previous state if saving files fails
		if shouldRestore {
			restoreError := ts.tu.RestoreDirectory(backupDir, taskDir)
			if restoreError != nil {
				return ErrFailedRestoreDirectory
			}
		}
		return ErrFailedSaveFiles
	}

	// Remove the backup directory after successful operation
	if backupDir != "" {
		err := os.RemoveAll(backupDir)
		if err != nil {
			return ErrFailedRemoveDirectory
		}
	}

	return nil
}

// CreateUserSubmission creates a new submission directory for a user's task submission.
// It creates a directory `submissions/user{user_id}/submission{n}/`, where n is an incrementing submission number.
// It places the user's submission file (e.g., solution.{ext}) inside the submission folder
// and creates an empty `output/` folder for the generated output files.
// It returns the submission number and any ServiceError encountered.
func (ts *TaskService) CreateUserSubmission(taskID int, userID int, userFile []byte, fileName string) (int, ServiceError) {
	// Define paths
	taskDir := filepath.Join(ts.taskDirectory, fmt.Sprintf("task%d", taskID))
	submissionsDir := filepath.Join(taskDir, "submissions")
	userDir := filepath.Join(submissionsDir, fmt.Sprintf("user%d", userID))

	// Check whether task directory exists
	if _, err := os.Stat(taskDir); os.IsNotExist(err) {
		return 0, ErrInvalidTaskID
	}

	// Ensure the submissions directory exists
	if _, err := os.Stat(submissionsDir); os.IsNotExist(err) {
		err := os.MkdirAll(submissionsDir, os.ModePerm)
		if err != nil {
			return 0, ErrFailedCreateSubmissionDir
		}
	}

	// Ensure the user directory exists
	if _, err := os.Stat(userDir); os.IsNotExist(err) {
		err := os.MkdirAll(userDir, os.ModePerm)
		if err != nil {
			return 0, ErrFailedCreateDirectory
		}
	}

	// Get the file extension and validate it
	fileExtension := strings.ToLower(filepath.Ext(fileName))
	if fileExtension == "" {
		return 0, ErrFileHasNoExtension
	}

	if !ts.tu.IsAllowedFileExtension(fileExtension) {
		return 0, ErrFileExtensionNotAllowed
	}

	// Get the next submission number by counting existing submission directories
	submissionNumber, err := ts.tu.GetNextSubmissionNumber(userDir)
	if err != nil {
		return 0, ErrFailedGetSubmissionNumber
	}

	// Define the submission directory path
	submissionDir := filepath.Join(userDir, fmt.Sprintf("submission%d", submissionNumber))
	outputDir := filepath.Join(submissionDir, "output")

	// Create the submission directory and the empty output directory
	err = os.MkdirAll(outputDir, os.ModePerm)
	if err != nil {
		return 0, ErrFailedCreateSubmissionDir
	}

	// Save the user's file in the submission directory with the correct extension
	userFilePath := filepath.Join(submissionDir, "solution"+fileExtension)
	if err := os.WriteFile(userFilePath, userFile, 0644); err != nil {
		return 0, ErrFailedSaveUserFile
	}

	return submissionNumber, nil
}

// StoreUserOutputs saves output files generated by the user's program inside the appropriate output/ folder
// under the user's specific submission directory, validating format and matching the task's expected output files.
func (ts *TaskService) StoreUserOutputs(taskID int, userID int, submissionNumber int, outputFiles map[string][]byte) ServiceError {
	// Define paths for the task, user, and specific submission directories
	taskDir := filepath.Join(ts.taskDirectory, fmt.Sprintf("task%d", taskID))
	expectedOutputDir := filepath.Join(taskDir, "src", "output")
	userSubmissionDir := filepath.Join(taskDir, "submissions", fmt.Sprintf("user%d", userID), fmt.Sprintf("submission%d", submissionNumber))
	outputDir := filepath.Join(userSubmissionDir, "output")

	// Read expected output files from the task's src/output directory
	expectedFiles, err := os.ReadDir(expectedOutputDir)
	if err != nil {
		return ErrFailedGetInputOutputFile
	}

	// Ensure user submission directory exists
	if _, err := os.Stat(userSubmissionDir); os.IsNotExist(err) {
		return ErrSubmissionDirDoesNotExist
	}

	// Verify if the output directory already has files
	if _, err := os.Stat(outputDir); err == nil {
		entries, err := os.ReadDir(outputDir)
		if err != nil {
			return ErrFailedReadOutputDirectory
		}
		if len(entries) > 0 {
			return ErrOutputDirContainsFiles
		}
	} else if os.IsNotExist(err) {
		// Create the output directory if it doesn't exist
		if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
			return ErrFailedCreateDirectory
		}
	} else {
		return ErrFailedAccessOutputDirectory
	}

	// If there's only one file named "compile-error.err", save it and return
	if len(outputFiles) == 1 {
		for fileName := range outputFiles {
			if fileName == "compile-err.err" {
				err = ts.tu.SaveCompileErrorFile(outputDir, outputFiles[fileName])
				if err != nil {
					return ErrFailedToSaveCompileError
				}
				return nil
			}
		}
	}

	// Count the number of output files provided by the user
	outputFilesCount := 0
	re := regexp.MustCompile(`^(\d+)\.out$`)
	for fileName := range outputFiles {
		if matches := re.FindStringSubmatch(fileName); matches != nil {
			outputFilesCount++
		}
	}

	// Map expected output numbers from the task's output directory
	expectedOutputCount := 0
	expectedNumbers := make(map[int]bool)

	for _, file := range expectedFiles {
		if matches := re.FindStringSubmatch(file.Name()); matches != nil {
			num, _ := strconv.Atoi(matches[1])
			expectedNumbers[num] = true
			expectedOutputCount++
		}
	}

	// Verify the count of provided output files matches the expected count
	if outputFilesCount != expectedOutputCount {
		return ErrOutputFileMismatch
	}

	// Track user-provided output numbers to avoid duplicates
	userOutputNumbers := make(map[int]bool)
	stderrNumbers := make(map[int]bool)

	// Save output files in the original name with the {number}.out or {number}.err format
	for fileName, fileContent := range outputFiles {
		baseName := filepath.Base(fileName)
		outputMatches := regexp.MustCompile(`^(\d+)\.out$`).FindStringSubmatch(baseName)
		stderrMatches := regexp.MustCompile(`^(\d+)\.err$`).FindStringSubmatch(baseName)

		if outputMatches != nil {
			// Handle output files
			num, err := strconv.Atoi(outputMatches[1])
			if err != nil {
				return ErrInvalidOutputFileNumber
			}

			// Ensure there are no duplicate numbers among the user files
			if userOutputNumbers[num] {
				return ErrDuplicateOutputFileNumber
			}
			userOutputNumbers[num] = true

			// Ensure the output file number matches expected output files
			if !expectedNumbers[num] {
				return ErrUnexpectedOutputFileNumber
			}

			// Save the output file in the output directory
			if err := os.WriteFile(filepath.Join(outputDir, baseName), fileContent, 0644); err != nil {
				return ErrFailedSaveOutputFile
			}
		} else if stderrMatches != nil {
			// Handle stderr files
			num, err := strconv.Atoi(stderrMatches[1])
			if err != nil {
				return ErrInvalidStderrFileNumber
			}

			// Ensure there are no duplicate numbers among the stderr files
			if stderrNumbers[num] {
				return ErrDuplicateStderrFileNumber
			}

			// Save the stderr file in the output directory
			if err := os.WriteFile(filepath.Join(outputDir, baseName), fileContent, 0644); err != nil {
				return ErrFailedSaveStderrFile
			}
		} else {
			// Return error if file format is neither .out nor .err
			return ErrInvalidOutputFileFormat
		}
	}

	return nil
}

// GetTaskFiles retrieves all files (description, input, and output) for a given task and returns them in a .tar.gz file.
// This function is useful for fetching the entire task content, preserving the folder structure.
func (ts *TaskService) GetTaskFiles(taskID int) (string, ServiceError) {
	// Define paths for the task and src directories
	taskDir := filepath.Join(ts.taskDirectory, fmt.Sprintf("task%d", taskID))
	srcDir := filepath.Join(taskDir, "src")

	// Check if the src directory exists
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return "", ErrTaskSrcDirDoesNotExist
	}

	// Create a temporary file for the TAR.GZ archive
	tarFilePath := filepath.Join(os.TempDir(), fmt.Sprintf("task%dFiles.tar.gz", taskID))
	tarFile, err := os.Create(tarFilePath)
	if err != nil {
		return "", ErrFailedCreateTarFile
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
			return ErrFailedAccessFile
		}

		// Determine relative path for maintaining directory structure
		relPath, err := filepath.Rel(filepath.Dir(srcDir), filePath) // root folder for src
		if err != nil {
			return ErrFailedDetermineRelPath
		}

		// Set up the TAR header
		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return ErrFailedCreateTarHeader
		}
		header.Name = filepath.Join(fmt.Sprintf("task%dFiles", taskID), relPath)

		// Write the header
		if err := tarWriter.WriteHeader(header); err != nil {
			return ErrFailedWriteTarHeader
		}

		// If it's a directory, skip writing the content
		if info.IsDir() {
			return nil
		}

		// Write the file content
		file, err := os.Open(filePath)
		if err != nil {
			return ErrFailedOpenFile
		}
		defer utils.CloseIO(file)

		if _, err := io.Copy(tarWriter, file); err != nil {
			return ErrFailedWriteFileToTar
		}

		return nil
	})
	if err != nil {
		return "", ErrFailedAddFilesToTar
	}

	// Return the path to the created TAR.GZ file
	return tarFilePath, nil
}

// GetUserSubmission fetches the specific submission file for a user in a given task.
func (ts *TaskService) GetUserSubmission(taskID int, userID int, submissionNum int) ([]byte, string, ServiceError) {
	// Define the path to the specific submission directory
	submissionDir := filepath.Join(ts.taskDirectory, fmt.Sprintf("task%d", taskID), "submissions", fmt.Sprintf("user%d", userID), fmt.Sprintf("submission%d", submissionNum))

	// Check if the submission directory exists
	if _, err := os.Stat(submissionDir); os.IsNotExist(err) {
		return nil, "", ErrSubmissionDirDoesNotExist
	}

	// Read files in the submission directory to locate the program file
	files, err := os.ReadDir(submissionDir)
	if err != nil {
		return nil, "", ErrFailedReadSubmissionDirectory
	}

	// Find the single program file in the directory
	var programFile string
	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), "solution") {
			if programFile != "" {
				return nil, "", ErrMultipleProgramFilesFound
			}
			programFile = file.Name()
		}
	}

	// Check if a program file was found
	if programFile == "" {
		return nil, "", ErrNoProgramFileFound
	}

	// Read the content of the program file
	programFilePath := filepath.Join(submissionDir, programFile)
	fileContent, err := os.ReadFile(programFilePath)
	if err != nil {
		return nil, "", ErrFailedReadProgramFile
	}

	return fileContent, programFile, nil
}

// GetInputOutput retrieves the specific input and output files for a given task and returns them in a .tar.gz archive.
// This is useful for accessing specific input/output pairs based on their ID.
func (ts *TaskService) GetInputOutput(taskID int, inputOutputID int) (string, ServiceError) {
	// Define paths for the task and the specific input/output directories
	taskDir := filepath.Join(ts.taskDirectory, fmt.Sprintf("task%d", taskID))
	inputDir := filepath.Join(taskDir, "src", "input")
	outputDir := filepath.Join(taskDir, "src", "output")

	// Check if the task's input and output directories exist
	if _, err := os.Stat(inputDir); os.IsNotExist(err) {
		return "", ErrInputDirectoryDoesNotExist
	}
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		return "", ErrOutputDirectoryDoesNotExist
	}

	// Locate specific input and output files based on inputOutputID
	inputFilePath := filepath.Join(inputDir, fmt.Sprintf("%d.in", inputOutputID))
	outputFilePath := filepath.Join(outputDir, fmt.Sprintf("%d.out", inputOutputID))

	// Ensure the input and output files exist
	if _, err := os.Stat(inputFilePath); os.IsNotExist(err) {
		return "", ErrInputFileDoesNotExist
	}
	if _, err := os.Stat(outputFilePath); os.IsNotExist(err) {
		return "", ErrOutputFileDoesNotExist
	}

	// Create a temporary .tar.gz file
	tarFilePath := filepath.Join(os.TempDir(), fmt.Sprintf("task%d_inputOutput%d.tar.gz", taskID, inputOutputID))
	tarFile, err := os.Create(tarFilePath)
	if err != nil {
		return "", ErrFailedCreateTarFile
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
			return "", ErrFailedOpenFile
		}

		// Gather file info and set up the TAR header
		info, err := file.Stat()
		if err != nil {
			return "", ErrFailedGetFileInfo
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return "", ErrFailedCreateTarHeader
		}
		// Use only the base filename for header.Name to avoid folder structure
		header.Name = info.Name()

		// Write the header and file content to the TAR archive
		if err := tarWriter.WriteHeader(header); err != nil {
			return "", ErrFailedWriteTarHeader
		}
		if _, err := io.Copy(tarWriter, file); err != nil {
			return "", ErrFailedWriteFileToTar
		}

		utils.CloseIO(file)
	}

	// Return the path to the created TAR.GZ file
	return tarFilePath, nil
}

// DeleteTask deletes the directory of a specific task, including all associated files and submissions.
func (ts *TaskService) DeleteTask(taskID int) ServiceError {
	// Construct the task directory path
	taskDir := filepath.Join(ts.taskDirectory, fmt.Sprintf("task%d", taskID))

	// Check if the task directory exists
	_, err := os.Stat(taskDir)
	if os.IsNotExist(err) {
		return ErrInvalidTaskID
	} else if err != nil {
		return ErrFailedAccessTaskDirectory
	}

	// Attempt to remove the task directory and all its contents
	err = os.RemoveAll(taskDir)
	if err != nil {
		return ErrFailedDeleteTaskDirectory
	}

	return nil
}

// GetUserSolutionPackage fetches the specific package for a given task, user, and submission number,
// organizing it in a structured .tar.gz archive containing inputs, outputs, and the solution file.
func (ts *TaskService) GetUserSolutionPackage(taskID, userID, submissionNum int) (string, ServiceError) {
	// Define paths for the task directories and files
	taskDir := filepath.Join(ts.taskDirectory, fmt.Sprintf("task%d", taskID))
	inputDir := filepath.Join(taskDir, "src", "input")
	outputDir := filepath.Join(taskDir, "src", "output")
	solutionPattern := filepath.Join(taskDir, "submissions", fmt.Sprintf("user%d", userID), fmt.Sprintf("submission%d", submissionNum), "solution.*")

	// Check if the input and output directories exist
	if _, err := os.Stat(inputDir); os.IsNotExist(err) {
		return "", ErrInputDirectoryDoesNotExist
	}
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		return "", ErrOutputDirectoryDoesNotExist
	}

	// Find the solution file with any extension
	solutionFiles, err := filepath.Glob(solutionPattern)
	if err != nil {
		return "", ErrFailedSearchSolutionFile
	}
	if len(solutionFiles) == 0 {
		return "", ErrSolutionFileDoesNotExist
	}
	if len(solutionFiles) > 1 {
		return "", ErrMultipleSolutionFilesFound
	}
	solutionFile := solutionFiles[0]

	// Create a temporary .tar.gz file to store the package
	tarFilePath := filepath.Join(os.TempDir(), fmt.Sprintf("task%d_user%d_submission%d_package.tar.gz", taskID, userID, submissionNum))
	tarFile, err := os.Create(tarFilePath)
	if err != nil {
		return "", ErrFailedCreateTarFile
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
			return ErrFailedOpenFile
		}
		defer utils.CloseIO(file)

		info, err := file.Stat()
		if err != nil {
			return ErrFailedGetFileInfo
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return ErrFailedCreateTarHeader
		}

		header.Name = tarPath // Use provided tarPath for directory structure in archive

		if err := tarWriter.WriteHeader(header); err != nil {
			return ErrFailedWriteTarHeader
		}

		if _, err := io.Copy(tarWriter, file); err != nil {
			return ErrFailedWriteFileToTar
		}

		return nil
	}

	// Add input files to the "inputs/" folder in the tar
	inputFiles, err := filepath.Glob(filepath.Join(inputDir, "*.in"))
	if err != nil {
		return "", ErrFailedReadInputFiles
	}
	for _, filePath := range inputFiles {
		fileName := filepath.Base(filePath)
		err := addFileToTar(filePath, filepath.Join("Task", "inputs", fileName))
		if err != nil {
			return "", ErrFailedAddFilesToTar
		}
	}

	// Add output files to the "outputs/" folder in the tar
	outputFiles, err := filepath.Glob(filepath.Join(outputDir, "*.out"))
	if err != nil {
		return "", ErrFailedReadOutputFiles
	}
	for _, filePath := range outputFiles {
		fileName := filepath.Base(filePath)
		err := addFileToTar(filePath, filepath.Join("Task", "outputs", fileName))
		if err != nil {
			return "", ErrFailedAddFilesToTar
		}
	}

	// Add the solution file to the tar, preserving its original extension
	err = addFileToTar(solutionFile, filepath.Join("Task", filepath.Base(solutionFile)))
	if err != nil {
		return "", ErrFailedAddFilesToTar
	}

	// Return the path to the created .tar.gz file
	return tarFilePath, nil
}

// GetTaskDescription fetches the description file for a given task.
func (ts *TaskService) GetTaskDescription(taskID int) ([]byte, string, ServiceError) {
	// Define the path to the task description file
	descriptionFilePath := filepath.Join(ts.taskDirectory, fmt.Sprintf("task%d", taskID), "src", "description.pdf")

	// Check if the description file exists
	if _, err := os.Stat(descriptionFilePath); os.IsNotExist(err) {
		return nil, "", ErrDescriptionFileDoesNotExist
	}

	// Read the content of the description file
	fileContent, err := os.ReadFile(descriptionFilePath)
	if err != nil {
		return nil, "", ErrFailedReadDescriptionFile
	}

	return fileContent, "description.pdf", nil
}
