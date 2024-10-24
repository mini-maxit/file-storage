// Package config provides configuration management for the application.
// It handles loading environment variables and setting default values.
package config

import (
	"github.com/joho/godotenv"
	"log"
	"os"
	"strings"
)

// Config holds the configuration values needed by the application.
// It includes:
//   - Port: the port on which the server will run (defaults to "8080").
//   - RootDirectory: the directory where tasks/files will be stored (defaults to "tasks/").
//   - AllowedFileTypes: a list of allowed file types for submissions (defaults to ".c, .cpp, .py").
type Config struct {
	Port             string
	RootDirectory    string
	AllowedFileTypes []string
}

// NewConfig loads the application's configuration from environment variables or sets defaults
// if environment variables are not available.
func NewConfig() *Config {
	// Load environment variables from the .env file
	err := godotenv.Load(".env")
	if err != nil {
		log.Printf("No .env file found or error loading it: %v", err)
	}

	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	rootDirectory := os.Getenv("ROOT_DIRECTORY")
	if rootDirectory == "" {
		rootDirectory = "tasks"
	}

	// Load allowed file types from environment or set default ones
	allowedFileTypesEnv := os.Getenv("ALLOWED_FILE_TYPES")
	allowedFileTypes := []string{".c", ".cpp", ".py"}
	if allowedFileTypesEnv != "" {
		// Split the environment variable string into a slice
		allowedFileTypes = strings.Split(allowedFileTypesEnv, ",")
		for i := range allowedFileTypes {
			allowedFileTypes[i] = strings.TrimSpace(allowedFileTypes[i])
		}
	}

	return &Config{
		Port:             port,
		RootDirectory:    rootDirectory,
		AllowedFileTypes: allowedFileTypes,
	}
}
