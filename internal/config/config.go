// Package config provides configuration management for the application.
// It handles loading environment variables and setting default values.
package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds the configuration values needed by the application.
// It includes:
//   - PublicServerPort: the port on which the public server will run (defaults to "8888").
//   - InternalServerPort: the port on which the internal server will run (defaults to "8081").
//   - RootDirectory: the directory where tasks/files will be stored (defaults to "tasks/").
//   - SigningSecret: the secret key used for signing URLs (HMAC). Must match clients.
type Config struct {
	PublicServerPort   string
	InternalServerPort string
	RootDirectory      string
	SigningSecret      string
	MaxSignTTLSeconds  int64
}

// NewConfig loads the application's configuration from environment variables or sets defaults
// if environment variables are not available.
func NewConfig() *Config {
	// Load environment variables from the .env file
	err := godotenv.Load(".env")
	if err != nil {
		log.Printf("No .env file found or error loading it: %v", err)
	}

	port := os.Getenv("PUBLIC_SERVER_PORT")
	if port == "" {
		port = "8888"
	}

	internalPort := os.Getenv("INTERNAL_SERVER_PORT")
	if internalPort == "" {
		internalPort = "8081"
	}

	rootDirectory := os.Getenv("ROOT_DIRECTORY")
	if rootDirectory == "" {
		rootDirectory = "file-storage-media"
	}

	signingSecret := os.Getenv("SIGNING_SECRET")
	if signingSecret == "" {
		log.Fatal("SIGNING_SECRET environment variable is not set. Refusing to start. Set a strong random secret in your .env file.")
	}

	maxSignTTLSeconds := os.Getenv("MAX_SIGN_TTL_SECONDS")
	if maxSignTTLSeconds == "" {
		maxSignTTLSeconds = "3600"
	}
	maxSignTTLParsed, err := strconv.ParseInt(maxSignTTLSeconds, 10, 64)
	if err != nil || maxSignTTLParsed <= 0 {
		log.Fatal("MAX_SIGN_TTL_SECONDS must be a positive integer (seconds)")
	}

	return &Config{
		PublicServerPort:   port,
		InternalServerPort: internalPort,
		RootDirectory:      rootDirectory,
		SigningSecret:      signingSecret,
		MaxSignTTLSeconds:  maxSignTTLParsed,
	}
}
