package taskutils

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

// ValidateFiles checks if the number of input and output files matches and if all files have a .txt extension.
func (tu *TaskUtils) ValidateFiles(files map[string][]byte) error {
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

// SaveFiles saves input and output files to their respective directories in the {number}.in.txt and {number}.out.txt format.
func (tu *TaskUtils) SaveFiles(inputDir, outputDir string, files map[string][]byte) error {
	inputCount := 1
	outputCount := 1

	for fileName, fileContent := range files {
		if strings.HasPrefix(fileName, "src/input/") {
			newFileName := fmt.Sprintf("%d.in.txt", inputCount)
			if err := os.WriteFile(filepath.Join(inputDir, newFileName), fileContent, 0644); err != nil {
				return fmt.Errorf("failed to save input file %s: %v", fileName, err)
			}
			inputCount++
		}
	}

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
