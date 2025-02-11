// Package config provides configuration management for the application.
// It handles loading environment variables and setting default values.
package config

import (
	"github.com/joho/godotenv"
	"log"
	"os"
)

// Config holds the configuration values needed by the application.
// It includes:
//   - Port: the port on which the server will run (defaults to "8080").
//   - RootDirectory: the directory where tasks/files will be stored (defaults to "tasks/").
type Config struct {
	Port          string
	RootDirectory string
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
		rootDirectory = "root"
	}

	return &Config{
		Port:          port,
		RootDirectory: rootDirectory,
	}
}
