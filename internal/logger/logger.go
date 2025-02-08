package logger

import (
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	logPath     string
	httpLogPath string

	sugarLogger     *zap.SugaredLogger
	httpSugarLogger *zap.SugaredLogger
)

// init initializes the log file paths and ensures that the directories exist.
func init() {
	// Use the system temp directory.
	tempDir := os.TempDir()

	// Construct directories for service and HTTP logs.
	serviceLogDir := filepath.Join(tempDir, "file-storage", "logs", "services")
	httpLogDir := filepath.Join(tempDir, "file-storage", "logs", "http")

	// Create directories if they don't exist.
	if err := os.MkdirAll(serviceLogDir, 0755); err != nil {
		fmt.Printf("Error creating service log directory: %v\n", err)
	}
	if err := os.MkdirAll(httpLogDir, 0755); err != nil {
		fmt.Printf("Error creating HTTP log directory: %v\n", err)
	}

	// Set the full paths for the log files.
	logPath = filepath.Join(serviceLogDir, "log.txt")
	httpLogPath = filepath.Join(httpLogDir, "log.txt")
}

// InitializeLogger sets up Zap with a custom configuration and initializes the SugaredLogger.
func InitializeLogger() {
	// Configure log rotation with lumberjack for service logs.
	fileWriter := zapcore.AddSync(&lumberjack.Logger{
		Filename: logPath,
		MaxAge:   1, // days
		Compress: true,
	})

	// Configure log rotation with lumberjack for HTTP logs.
	httpFileWriter := zapcore.AddSync(&lumberjack.Logger{
		Filename: httpLogPath,
		MaxAge:   1, // days
		Compress: true,
	})

	// Standard output writer.
	consoleWriter := zapcore.AddSync(os.Stdout)

	// Encoder configuration for file logs (no colors).
	fileEncoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "source",
		MessageKey:     "msg",
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeLevel:    zapcore.CapitalLevelEncoder, // no colors for file output
		EncodeDuration: zapcore.StringDurationEncoder,
	}

	// Encoder configuration for console logs (with colors).
	consoleEncoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "source",
		MessageKey:     "msg",
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder, // colors enabled here
		EncodeDuration: zapcore.StringDurationEncoder,
	}

	// Create cores for file and console logging.
	fileCore := zapcore.NewCore(
		zapcore.NewConsoleEncoder(fileEncoderConfig),
		fileWriter,
		zap.InfoLevel,
	)

	consoleCore := zapcore.NewCore(
		zapcore.NewConsoleEncoder(consoleEncoderConfig),
		consoleWriter,
		zap.InfoLevel,
	)

	// Create a separate core for HTTP file logging.
	httpCore := zapcore.NewCore(
		zapcore.NewConsoleEncoder(fileEncoderConfig),
		httpFileWriter,
		zap.InfoLevel,
	)

	// Combine file and console cores for general logging.
	combinedCore := zapcore.NewTee(fileCore, consoleCore)

	// Initialize the general logger with caller information and stacktrace for errors.
	logger := zap.New(combinedCore, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	sugarLogger = logger.Sugar()

	// Initialize the HTTP logger (file-only).
	httpLogger := zap.New(httpCore, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	httpSugarLogger = httpLogger.Sugar()

	// Log the paths to the log files to the console.
	sugarLogger.Infof("Service logs are being saved to: %s", logPath)
	sugarLogger.Infof("HTTP logs are being saved to: %s", httpLogPath)
}

// NewHttpLogger returns the named HTTP logger.
func NewHttpLogger() *zap.SugaredLogger {
	if httpSugarLogger == nil {
		InitializeLogger()
	}
	return httpSugarLogger.Named("http")
}

// NewNamedLogger creates a new named SugaredLogger for a given service.
func NewNamedLogger(serviceName string) *zap.SugaredLogger {
	if sugarLogger == nil {
		InitializeLogger()
	}
	return sugarLogger.Named(serviceName)
}
