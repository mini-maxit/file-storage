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
	config := config.NewConfig()
	init := initialization.NewInitialization(config)
	server := server.NewServer(init)
	err := server.Run(":8080")
	if err != nil {
		logrus.Fatalf("server stopped: %v", err)
	}

}
