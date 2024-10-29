package taskutils

import (
	"errors"
	"fmt"
	"github.com/mini-maxit/file-storage/utils"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/mini-maxit/file-storage/internal/config"
)

type TaskUtils struct {
	Config *config.Config
}

// NewTaskUtils creates a new instance of TaskUtils with the provided configuration.
func NewTaskUtils(cfg *config.Config) *TaskUtils {
	return &TaskUtils{
		Config: cfg,
	}
}

// BackupDirectory creates a backup of an existing directory in a temporary location.
func (tu *TaskUtils) BackupDirectory(taskDir string) (string, error) {
	backupDir, err := os.MkdirTemp("", "task_backup_*")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary backup directory: %v", err)
	}

	err = tu.CopyDir(taskDir, backupDir)
	if err != nil {
		return "", fmt.Errorf("failed to copy directory for backup: %v", err)
	}

	return backupDir, nil
}

// RestoreDirectory restores a backup directory to the original task directory.
func (tu *TaskUtils) RestoreDirectory(backupDir, taskDir string) error {
	err := os.RemoveAll(taskDir)
	if err != nil {
		return fmt.Errorf("failed to remove incomplete task directory for restoration: %v", err)
	}

	err = tu.CopyDir(backupDir, taskDir)
	if err != nil {
		return fmt.Errorf("failed to restore task directory from backup: %v", err)
	}

	return nil
}

// CopyDir copies the contents of one directory to another.
func (tu *TaskUtils) CopyDir(srcDir, destDir string) error {
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

		return tu.CopyFile(path, destPath)
	})
}

// CopyFile copies a file from src to dst.
func (tu *TaskUtils) CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer utils.CloseIO(sourceFile)

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer utils.CloseIO(destFile)

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

// CreateDirectoryStructure creates the required directory structure for a task.
func (tu *TaskUtils) CreateDirectoryStructure(srcDir, inputDir, outputDir string) error {
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

// ValidateFiles checks if input and output files have names in the correct format ({number}.in or {number}.out),
// ensures each file has a unique number, and validates that there is an equal count of input and output files.
// Also ensures there is a single description file with a .pdf extension.
func (tu *TaskUtils) ValidateFiles(files map[string][]byte) error {
	inputFiles := make(map[int]bool)
	outputFiles := make(map[int]bool)
	hasDescription := false

	// Define regex patterns to match "{number}.in" or "{number}.out"
	inputPattern := regexp.MustCompile(`^(\d+)\.in$`)
	outputPattern := regexp.MustCompile(`^(\d+)\.out$`)

	for fileName := range files {
		baseName := filepath.Base(fileName)

		// Validate input files
		if strings.HasPrefix(fileName, "src/input/") {
			matches := inputPattern.FindStringSubmatch(baseName)
			if matches == nil {
				return fmt.Errorf("input file %s does not match the required format {number}.in", baseName)
			}

			number := matches[1]
			num, err := strconv.Atoi(number)
			if err != nil {
				return fmt.Errorf("invalid input file number in %s: %v", baseName, err)
			}

			if inputFiles[num] {
				return fmt.Errorf("duplicate input file number %d found", num)
			}
			inputFiles[num] = true

		} else if strings.HasPrefix(fileName, "src/output/") { // Validate output files
			matches := outputPattern.FindStringSubmatch(baseName)
			if matches == nil {
				return fmt.Errorf("output file %s does not match the required format {number}.out", baseName)
			}

			number := matches[1]
			num, err := strconv.Atoi(number)
			if err != nil {
				return fmt.Errorf("invalid output file number in %s: %v", baseName, err)
			}

			if outputFiles[num] {
				return fmt.Errorf("duplicate output file number %d found", num)
			}
			outputFiles[num] = true

		} else if baseName == "description.pdf" || strings.HasPrefix(baseName, "description.") { // Validate description file
			if filepath.Ext(fileName) != ".pdf" {
				return errors.New("description must have a .pdf extension")
			}
			hasDescription = true

		} else { // Unrecognized file path
			return fmt.Errorf("unrecognized file path %s", fileName)
		}
	}

	// Ensure equal counts of input and output files
	if len(inputFiles) != len(outputFiles) {
		return errors.New("the number of input files must match the number of output files")
	}

	// Ensure a description file is provided
	if !hasDescription {
		return errors.New("a description file (description.pdf) is required")
	}

	// Check if the input and output file numbers match and are sequential from 1 to n
	for i := 1; i <= len(inputFiles); i++ {
		if !inputFiles[i] || !outputFiles[i] {
			return fmt.Errorf("input and output files must have matching numbers from 1 to %d", len(inputFiles))
		}
	}

	return nil
}

// SaveFiles saves input and output files in their respective directories using their original names and extensions.
func (tu *TaskUtils) SaveFiles(inputDir, outputDir string, files map[string][]byte) error {
	for fileName, fileContent := range files {
		var targetDir string

		if strings.HasPrefix(fileName, "src/input/") {
			targetDir = inputDir
		} else if strings.HasPrefix(fileName, "src/output/") {
			targetDir = outputDir
		} else {
			continue // Ignore files outside of input/output directories
		}

		// Save the file with its original name and extension
		targetFilePath := filepath.Join(targetDir, filepath.Base(fileName))
		if err := os.WriteFile(targetFilePath, fileContent, 0644); err != nil {
			return fmt.Errorf("failed to save file %s: %v", fileName, err)
		}
	}

	return nil
}

// GetNextSubmissionNumber determines the next submission number for a user by counting existing submissions.
func (tu *TaskUtils) GetNextSubmissionNumber(userDir string) (int, error) {
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

// IsAllowedFileExtension checks if the given file extension is in the allowed list from the configuration.
func (tu *TaskUtils) IsAllowedFileExtension(extension string) bool {
	for _, allowedExtension := range tu.Config.AllowedFileTypes {
		if extension == allowedExtension {
			return true
		}
	}
	return false
}

// SaveCompileErrorFile saves the compile-error.err file in the output directory
func (tu *TaskUtils) SaveCompileErrorFile(outputDir string, fileContent []byte) error {
	filePath := filepath.Join(outputDir, "compile-error.err")
	if err := os.WriteFile(filePath, fileContent, 0644); err != nil {
		return fmt.Errorf("failed to save compile-error.err: %v", err)
	}
	return nil
}
