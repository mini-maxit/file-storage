package services

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/mini-maxit/file-storage/internal/config"
	"github.com/mini-maxit/file-storage/internal/entities"
)

type FileService struct {
	buckets       map[string]entities.Bucket
	RootDirectory string
}

func NewFileService(cfg *config.Config) *FileService {
	rootDir := cfg.RootDirectory
	buckets := make(map[string]entities.Bucket)

	// Define the /buckets path
	bucketsDir := filepath.Join(rootDir, "buckets")

	// Check if the directory exists
	if _, err := os.Stat(bucketsDir); os.IsNotExist(err) {
		// If the directory doesn't exist, create it
		err := os.MkdirAll(bucketsDir, 0755)
		if err != nil {
			panic("failed to create /buckets directory: " + err.Error())
		}
	}

	// Scan the /buckets directory for folders
	files, err := os.ReadDir(bucketsDir)
	if err != nil {
		panic("failed to scan /buckets directory: " + err.Error())
	}

	for _, file := range files {
		if file.IsDir() {
			bucketName := file.Name()
			bucketPath := filepath.Join(bucketsDir, bucketName)

			// Get the number of objects (files) and total size
			numberOfObjects, totalSize := calculateBucketStats(bucketPath)

			// Add the bucket to the buckets map
			buckets[bucketName] = entities.Bucket{
				Name:            bucketName,
				CreationDate:    getFolderCreationTime(bucketPath),
				NumberOfObjects: numberOfObjects,
				Size:            totalSize,
			}
		}
	}

	return &FileService{
		buckets:       buckets,
		RootDirectory: rootDir,
	}
}

// GetBucket retrieves a bucket by name
func (fs *FileService) GetBucket(bucketName string) (entities.Bucket, error) {
	if bucket, ok := fs.buckets[bucketName]; ok {
		return bucket, nil
	}
	return entities.Bucket{}, errors.New("bucket not found")
}

// CreateBucket creates a new bucket
func (fs *FileService) CreateBucket(bucket entities.Bucket) error {
	if _, exists := fs.buckets[bucket.Name]; exists {
		return errors.New("bucket already exists")
	}

	// Create the bucket directory in the filesystem
	bucketPath := filepath.Join(fs.RootDirectory, "buckets", bucket.Name)
	err := os.MkdirAll(bucketPath, 0755)
	if err != nil {
		return errors.New("failed to create bucket directory: " + err.Error())
	}

	// Add the bucket to the in-memory map
	fs.buckets[bucket.Name] = bucket
	return nil
}

// GetAllBuckets retrieves all buckets
func (fs *FileService) GetAllBuckets() []entities.Bucket {
	bucketList := make([]entities.Bucket, 0, len(fs.buckets))
	for _, bucket := range fs.buckets {
		bucketList = append(bucketList, bucket)
	}
	return bucketList
}

// getFolderCreationTime retrieves the creation time of a folder (approximation using mod time)
func getFolderCreationTime(folderPath string) time.Time {
	info, err := os.Stat(folderPath)
	if err != nil {
		return time.Now() // Default to now if we can't get the creation time
	}
	return info.ModTime()
}

// calculateBucketStats scans the directory to calculate the number of files and total size
func calculateBucketStats(bucketPath string) (int, int) {
	var numberOfObjects int
	var totalSize int

	err := filepath.Walk(bucketPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Only count files, not directories
		if !info.IsDir() {
			numberOfObjects++
			totalSize += int(info.Size())
		}
		return nil
	})

	if err != nil {
		panic("failed to calculate bucket stats for " + bucketPath + ": " + err.Error())
	}

	return numberOfObjects, totalSize
}
