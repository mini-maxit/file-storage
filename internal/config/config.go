// Package config provides configuration management for the application.
// It handles loading environment variables and setting default values.
package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config holds the configuration values needed by the application.
// It includes:
//   - Port: the port on which the server will run (defaults to "8080").
//   - RootDirectory: the directory where tasks/files will be stored (defaults to "tasks/").
//   - SigningSecret: the secret key used for signing URLs (HMAC).
//   - InternalAPIKey: the key that internal services (backend, worker) must send to bypass
//     signature requirements and perform write operations.
type Config struct {
	Port           string
	RootDirectory  string
	SigningSecret  string
	InternalAPIKey string
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
		rootDirectory = "file-storage-media"
	}

	signingSecret := os.Getenv("SIGNING_SECRET")
	if signingSecret == "" {
		log.Fatal("SIGNING_SECRET environment variable is not set. Refusing to start. Set a strong random secret in your .env file.")
	}

	internalAPIKey := os.Getenv("INTERNAL_API_KEY")
	if internalAPIKey == "" {
		log.Fatal("INTERNAL_API_KEY environment variable is not set. Refusing to start. Set a strong random secret in your .env file.")
	}

	return &Config{
		Port:           port,
		RootDirectory:  rootDirectory,
		SigningSecret:  signingSecret,
		InternalAPIKey: internalAPIKey,
	}
}
