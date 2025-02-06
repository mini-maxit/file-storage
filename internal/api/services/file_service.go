package services

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/mini-maxit/file-storage/internal/config"
	"github.com/mini-maxit/file-storage/internal/entities"
)

type FileService struct {
	buckets       map[string]*entities.Bucket
	RootDirectory string
}

func NewFileService(cfg *config.Config) *FileService {
	rootDir := cfg.RootDirectory
	buckets := make(map[string]*entities.Bucket)

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

			// Add the bucket to the buckets map as a pointer
			buckets[bucketName] = &entities.Bucket{
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

// loadBucketObjects loads all files (objects) in a bucket directory.
func loadBucketObjects(bucketPath string) (map[string]entities.Object, int, int) {
	objects := make(map[string]entities.Object)
	var totalSize int
	var numberOfObjects int

	err := filepath.Walk(bucketPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Only process files; ignore directories.
		if !info.IsDir() {
			// Calculate the relative key within the bucket.
			relativeKey, err := filepath.Rel(bucketPath, path)
			if err != nil {
				return err
			}

			// Determine the file type (based on extension).
			fileType := filepath.Ext(path)

			// Create an Object and add it to the map.
			objects[relativeKey] = entities.Object{
				Key:          relativeKey,
				Size:         int(info.Size()),
				LastModified: info.ModTime(),
				Type:         fileType,
			}

			// Update totals.
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

// GetBucket retrieves a bucket by name, returning a pointer.
func (fs *FileService) GetBucket(bucketName string) (*entities.Bucket, error) {
	if bucket, ok := fs.buckets[bucketName]; ok {
		return bucket, nil
	}
	return nil, errors.New("bucket not found")
}

// CreateBucket creates a new bucket.
func (fs *FileService) CreateBucket(bucket entities.Bucket) error {
	if _, exists := fs.buckets[bucket.Name]; exists {
		return errors.New("bucket already exists")
	}

	// Create the bucket directory in the filesystem.
	bucketPath := filepath.Join(fs.RootDirectory, "buckets", bucket.Name)
	err := os.MkdirAll(bucketPath, 0755)
	if err != nil {
		return errors.New("failed to create bucket directory: " + err.Error())
	}

	// Ensure the Objects map is initialized.
	if bucket.Objects == nil {
		bucket.Objects = make(map[string]entities.Object)
	}

	// Add the bucket pointer to the in-memory map.
	fs.buckets[bucket.Name] = &bucket
	return nil
}

// GetAllBuckets retrieves all buckets' metadata (without the objects).
func (fs *FileService) GetAllBuckets() []*entities.Bucket {
	bucketList := make([]*entities.Bucket, 0, len(fs.buckets))
	for _, bucket := range fs.buckets {
		// Create a shallow copy of bucket metadata without the objects.
		metadata := entities.Bucket{
			Name:            bucket.Name,
			CreationDate:    bucket.CreationDate,
			NumberOfObjects: bucket.NumberOfObjects,
			Size:            bucket.Size,
			// Omit the Objects field.
		}
		bucketList = append(bucketList, &metadata)
	}
	return bucketList
}

// DeleteBucket deletes a bucket.
func (fs *FileService) DeleteBucket(bucketName string) error {
	// Delete the bucket directory from the file system.
	bucketPath := filepath.Join(fs.RootDirectory, "buckets", bucketName)
	if err := os.RemoveAll(bucketPath); err != nil {
		return errors.New("failed to delete bucket directory: " + err.Error())
	}

	// Delete the bucket from the in-memory map.
	delete(fs.buckets, bucketName)
	return nil
}

// AddOrUpdateObject adds or updates (if exists) an object in a bucket.
func (fs *FileService) AddOrUpdateObject(bucketName string, objectKey string, file io.Reader) error {
	bucket, ok := fs.buckets[bucketName]
	if !ok {
		return errors.New("bucket not found")
	}

	// Create the directory for the object if it doesn't exist.
	objectPath := filepath.Join(fs.RootDirectory, "buckets", bucketName, objectKey)
	objectDir := filepath.Dir(objectPath)
	if err := os.MkdirAll(objectDir, 0755); err != nil {
		return errors.New("failed to create object directory: " + err.Error())
	}

	// Open the destination file for writing (replace if it exists).
	destFile, err := os.Create(objectPath)
	if err != nil {
		return errors.New("failed to create object file: " + err.Error())
	}
	defer func() {
		if cerr := destFile.Close(); cerr != nil {
			panic("failed to close object file: " + cerr.Error())
		}
	}()

	// Copy the uploaded file to the destination file.
	if _, err := io.Copy(destFile, file); err != nil {
		return errors.New("failed to copy object file: " + err.Error())
	}

	// Get file info.
	fileInfo, err := os.Stat(objectPath)
	if err != nil {
		return errors.New("failed to stat object file: " + err.Error())
	}

	size := fileInfo.Size()
	lastModified := fileInfo.ModTime()

	// If the object already exists, adjust the bucket size.
	if existingObject, exists := bucket.Objects[objectKey]; exists {
		bucket.Size -= existingObject.Size
	}

	// Add or update the object in the map.
	bucket.Objects[objectKey] = entities.Object{
		Key:          objectKey,
		Size:         int(size),
		LastModified: lastModified,
		Type:         filepath.Ext(objectKey),
	}

	// Update bucket metadata.
	bucket.NumberOfObjects = len(bucket.Objects)
	bucket.Size += int(size)

	return nil
}

// GetObject retrieves the object's metadata from the in-memory map.
func (fs *FileService) GetObject(bucketName, objectKey string) (*entities.Object, error) {
	bucket, ok := fs.buckets[bucketName]
	if !ok {
		return nil, errors.New("bucket not found")
	}

	obj, exists := bucket.Objects[objectKey]
	if !exists {
		return nil, errors.New("object not found")
	}

	// Return the address of the object copy.
	return &obj, nil
}

// GetObjectFilePath returns the absolute path of the object file on disk.
func (fs *FileService) GetObjectFilePath(bucketName, objectKey string) (string, error) {
	// Build the path.
	objectPath := filepath.Join(fs.RootDirectory, "buckets", bucketName, objectKey)

	// Check if file exists.
	info, err := os.Stat(objectPath)
	if os.IsNotExist(err) || info.IsDir() {
		return "", errors.New("object file not found or is a directory")
	} else if err != nil {
		return "", err
	}

	return objectPath, nil
}

// RemoveObject deletes an object file from disk and updates in-memory metadata.
func (fs *FileService) RemoveObject(bucketName, objectKey string) error {
	bucket, ok := fs.buckets[bucketName]
	if !ok {
		return errors.New("bucket not found")
	}

	obj, exists := bucket.Objects[objectKey]
	if !exists {
		return errors.New("object not found")
	}

	// Construct the file path on disk.
	objectPath := filepath.Join(fs.RootDirectory, "buckets", bucketName, objectKey)
	if err := os.Remove(objectPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	// Update bucket metadata.
	bucket.Size -= obj.Size
	delete(bucket.Objects, objectKey)
	bucket.NumberOfObjects = len(bucket.Objects)
	return nil
}

// getFolderCreationTime retrieves the creation time of a folder (approximation using mod time).
func getFolderCreationTime(folderPath string) time.Time {
	info, err := os.Stat(folderPath)
	if err != nil {
		return time.Now()
	}
	return info.ModTime()
}
