package initialization

import (
	"fmt"
	"os"

	"github.com/mini-maxit/file-storage/internal/config"
)

// Initialization is a struct that holds the application configuration.
// It provides methods for initializing necessary components based on the configuration.
type Initialization struct {
	config *config.Config
}

// NewInitialization creates a new Initialization instance using the provided configuration.
func NewInitialization(cfg *config.Config) *Initialization {
	return &Initialization{
		config: cfg,
	}
}

// InitializeRootDirectory checks if the root directory exists and creates it if it doesn't.
// If the directory creation fails, it returns an error.
func (i *Initialization) InitializeRootDirectory() error {
	rootDir := i.config.RootDirectory
	// Check if the directory exists
	if _, err := os.Stat(rootDir); os.IsNotExist(err) {
		// Directory doesn't exist, attempt to create it
		err := os.MkdirAll(rootDir, os.ModePerm)
		if err != nil {
			// Return an error if directory creation fails
			return fmt.Errorf("failed to create root directory %s: %v", rootDir, err)
		}
	}
	return nil
}
