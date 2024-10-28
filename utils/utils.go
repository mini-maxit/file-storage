package utils

import (
	"io"
	"log"
	"os"
)

// FileSize returns size of file
func FileSize(file *os.File) int64 {
	info, err := file.Stat()
	if err != nil {
		return 0
	}
	return info.Size()
}

// CloseIO tries to close any io.Closer and logs an error if one occurs.
func CloseIO(c io.Closer) {
	if err := c.Close(); err != nil {
		log.Printf("Error closing file: %v", err)
	}
}

// RemoveDirectory tries to remove any directory from given path and logs an error if one occurs.
func RemoveDirectory(path string) {
	if err := os.RemoveAll(path); err != nil {
		log.Printf("Error removing directory: %v", err)
	}
}
