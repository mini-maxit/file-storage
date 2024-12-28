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

			// Get the objects (files), number of objects, and total size
			objects, numberOfObjects, totalSize := loadBucketObjects(bucketPath)

			// Add the bucket to the buckets map
			buckets[bucketName] = entities.Bucket{
				Name:            bucketName,
				CreationDate:    getFolderCreationTime(bucketPath),
				NumberOfObjects: numberOfObjects,
				Size:            totalSize,
				Objects:         objects,
			}
		}
	}

	return &FileService{
		buckets:       buckets,
		RootDirectory: rootDir,
	}
}

// loadBucketObjects loads all files (objects) in a bucket directory
func loadBucketObjects(bucketPath string) (map[string]entities.Object, int, int) {
	objects := make(map[string]entities.Object)
	var totalSize int
	var numberOfObjects int

	err := filepath.Walk(bucketPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Only process files, ignore directories
		if !info.IsDir() {
			// Calculate the relative key within the bucket
			relativeKey, err := filepath.Rel(bucketPath, path)
			if err != nil {
				return err
			}

			// Determine the file type (based on extension)
			fileType := filepath.Ext(path)

			// Create an Object and add it to the map
			objects[relativeKey] = entities.Object{
				Key:          relativeKey,
				Size:         int(info.Size()),
				LastModified: info.ModTime(),
				Type:         fileType,
			}

			// Update totals
			numberOfObjects++
			totalSize += int(info.Size())
		}
		return nil
	})

	if err != nil {
		panic("failed to load objects for bucket " + bucketPath + ": " + err.Error())
	}

	return objects, numberOfObjects, totalSize
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

func (fs *FileService) DeleteBucket(bucketName string) error {
	// Delete the bucket directory from the file system
	bucketPath := filepath.Join(fs.RootDirectory, "buckets", bucketName)
	if err := os.RemoveAll(bucketPath); err != nil {
		return errors.New("failed to delete bucket directory: " + err.Error())
	}

	// Delete the bucket from the in-memory map
	delete(fs.buckets, bucketName)

	return nil
}

// getFolderCreationTime retrieves the creation time of a folder (approximation using mod time)
func getFolderCreationTime(folderPath string) time.Time {
	info, err := os.Stat(folderPath)
	if err != nil {
		return time.Now()
	}
	return info.ModTime()
}
