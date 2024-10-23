package main

import (
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

	addr := ":" + _config.Port
	_server := server.NewServer(init)
	err = _server.Run(addr)
	if err != nil {
		logrus.Fatalf("server stopped: %v", err)
	}
}
