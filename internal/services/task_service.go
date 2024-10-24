package services

import (
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

// validateFiles checks if the number of input and output files matches.
func (ts *TaskService) validateFiles(files map[string][]byte) error {
	var inputFiles, outputFiles []string

	for fileName := range files {
		if filepath.Base(fileName) == "src/description.pdf" {
			continue
		}
		if strings.HasPrefix(fileName, "input/") {
			inputFiles = append(inputFiles, fileName)
		}
		if strings.HasPrefix(fileName, "output/") {
			outputFiles = append(outputFiles, fileName)
		}
	}

	if len(inputFiles) != len(outputFiles) {
		return errors.New("the number of input files must match the number of output files")
	}

	return nil
}

// saveFiles saves input and output files to their respective directories.
func (ts *TaskService) saveFiles(inputDir, outputDir string, files map[string][]byte) error {
	// Save input files
	for fileName, fileContent := range files {
		if strings.HasPrefix(fileName, "src/input/") {
			if err := os.WriteFile(filepath.Join(inputDir, filepath.Base(fileName)), fileContent, 0644); err != nil {
				return fmt.Errorf("failed to save input file %s: %v", fileName, err)
			}
		}
	}

	// Save output files
	for fileName, fileContent := range files {
		if strings.HasPrefix(fileName, "src/output/") {
			if err := os.WriteFile(filepath.Join(outputDir, filepath.Base(fileName)), fileContent, 0644); err != nil {
				return fmt.Errorf("failed to save output file %s: %v", fileName, err)
			}
		}
	}

	return nil
}