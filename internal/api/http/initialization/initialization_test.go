package initialization

import (
	"os"
	"testing"

	"github.com/mini-maxit/file-storage/internal/config"
	"github.com/stretchr/testify/assert"
)

func setupTempDir(t *testing.T) (string, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "testrootdir")
	if err != nil {
		t.Fatalf("unable to create temp directory: %v", err)
	}

	return dir, func() {
		err := os.RemoveAll(dir)
		if err != nil {
			return
		}
	}
}

// TestInitializeRootDirectoryExists tests the case where the root directory already exists.
func TestInitializeRootDirectoryExists(t *testing.T) {
	// Set up a temporary directory to act as the root directory
	existingDir, cleanup := setupTempDir(t)
	defer cleanup()

	// Create a mock configuration with the existing directory
	mockConfig := &config.Config{
		RootDirectory: existingDir,
	}

	init := NewInitialization(mockConfig)

	// Call InitializeRootDirectory and ensure no error is returned
	err := init.InitializeRootDirectory()
	assert.NoError(t, err, "expected no error when root directory already exists")
}

// TestInitializeRootDirectoryCreate tests the case where the root directory doesn't exist and is created.
func TestInitializeRootDirectoryCreate(t *testing.T) {
	// Set up a temporary directory as the base for a non-existent subdirectory
	baseDir, cleanup := setupTempDir(t)
	defer cleanup()

	// Define a path to a subdirectory that doesn't exist yet
	nonExistentDir := baseDir + "/newtasks"

	// Create a mock configuration with the non-existent directory
	mockConfig := &config.Config{
		RootDirectory: nonExistentDir,
	}

	init := NewInitialization(mockConfig)

	// Call InitializeRootDirectory and ensure no error is returned
	err := init.InitializeRootDirectory()
	assert.NoError(t, err, "expected no error when creating a non-existent root directory")

	// Ensure that the directory was actually created
	_, err = os.Stat(nonExistentDir)
	assert.False(t, os.IsNotExist(err), "expected directory to be created")
}

// TestInitializeRootDirectoryCreateFail tests the case where the directory creation fails.
func TestInitializeRootDirectoryCreateFail(t *testing.T) {
	// Use an invalid directory path that will cause failure (e.g., a read-only directory or invalid path)
	invalidDir := "/invalid/dir"

	// Create a mock configuration with the invalid directory path
	mockConfig := &config.Config{
		RootDirectory: invalidDir,
	}

	init := NewInitialization(mockConfig)

	// Call InitializeRootDirectory and expect an error due to failure in creating the directory
	err := init.InitializeRootDirectory()
	assert.Error(t, err, "expected an error when failing to create the directory")
}
