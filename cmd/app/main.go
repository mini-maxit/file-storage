package main

import (
	"os"
	"time"

	"github.com/mini-maxit/file-storage/internal/api/services"
	"github.com/mini-maxit/file-storage/internal/logger"
	"github.com/mini-maxit/file-storage/pkg/urlsigner"

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

	signer := urlsigner.NewURLSigner(_config.SigningSecret)

	logger.InitializeLogger()
	public_log := logger.NewNamedLogger("public-server")

	publicAddr := ":" + _config.PublicServerPort
	publicServer := server.NewServer(fileService, signer, public_log)
	go func() {
		public_log.Infof("Starting public server on %s", publicAddr)
		if err := publicServer.Run(publicAddr); err != nil {
			logrus.Fatalf("public server stopped: %v", err)
		}
	}()

	internalAddr := ":" + _config.InternalServerPort
	internal_log := logger.NewNamedLogger("internal-server")
	internalServer := server.NewInternalServer(fileService, signer, time.Duration(_config.MaxSignTTLSeconds)*time.Second, internal_log)
	internal_log.Infof("Starting internal server on %s", internalAddr)
	if err := internalServer.Run(internalAddr); err != nil {
		logrus.Fatalf("internal server stopped: %v", err)
	}
}
