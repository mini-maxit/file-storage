package main

import (
	"github.com/mini-maxit/file-storage/internal/api/services"
	"github.com/mini-maxit/file-storage/internal/logger"
	"github.com/mini-maxit/file-storage/pkg/urlsigner"
	"os"

	"github.com/joho/godotenv"
	"github.com/mini-maxit/file-storage/internal/api/http/initialization"
	"github.com/mini-maxit/file-storage/internal/api/http/server"
	"github.com/mini-maxit/file-storage/internal/config"
	"github.com/sirupsen/logrus"
)

func main() {
	if _, ok := os.LookupEnv("DEBUG"); ok {
		err := godotenv.Load("././.env")
		if err != nil {
			logrus.Fatalf("could not load .env file. %s", err)
		}
	}

	_config := config.NewConfig()
	init := initialization.NewInitialization(_config)
	err := init.InitializeRootDirectory()
	if err != nil {
		logrus.Fatalf("failed to initialize root directory: %v", err)
	}

	fileService := services.NewFileService(_config)

	// Create URL signer with the configured secret
	signer := urlsigner.NewURLSigner(_config.SigningSecret)

	logger.InitializeLogger()
	log := logger.NewNamedLogger("server")

	addr := ":" + _config.Port
	_server := server.NewServer(fileService, signer, _config.InternalAPIKey, log)
	err = _server.Run(addr)
	if err != nil {
		logrus.Fatalf("server stopped: %v", err)
	}
}
