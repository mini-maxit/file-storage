package services

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mini-maxit/file-storage/internal/config"
	errors "github.com/mini-maxit/file-storage/pkg/filestorage"
	"github.com/mini-maxit/file-storage/pkg/filestorage/entities"
)

type FileService struct {
	BucketsDirectory string
}

func NewFileService(cfg *config.Config) *FileService {
	rootDir := cfg.RootDirectory

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

	return &FileService{
		BucketsDirectory: bucketsDir,
	}
}

// DONE?
// returns unfiltered errors from os package
func scanBucketsDirectory(bucketsDir string) (map[string]*entities.Bucket, error) {
	buckets := make(map[string]*entities.Bucket)

	// Scan the /buckets directory for folders
	files, err := os.ReadDir(bucketsDir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() {
			bucketName := file.Name()
			bucketPath := filepath.Join(bucketsDir, bucketName)

			// Get the objects (files), number of objects, and total size
			objects, numberOfObjects, err := loadBucketObjects(bucketPath)
			if err != nil {
				return nil, err
			}
			// Add the bucket to the buckets map as a pointer
			buckets[bucketName] = &entities.Bucket{
				Name:            bucketName,
				NumberOfObjects: numberOfObjects,
				Objects:         objects,
			}
		}
	}

	return buckets, nil
}

// loadBucketObjects loads all files (objects) in a bucket directory.
func loadBucketObjects(bucketPath string) (map[string]entities.Object, int, error) {
	objects := make(map[string]entities.Object)
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
				Key:  relativeKey,
				Type: fileType,
			}

			// Update totals.
			numberOfObjects++
		}
		return nil
	})

	return objects, numberOfObjects, err
}

// DONE
// GetBucket retrieves a bucket by name, returning a pointer.
func (fs *FileService) GetBucket(bucketName string, listObjects bool, prefix string) (*entities.Bucket, error) {
	buckets, err := scanBucketsDirectory(fs.BucketsDirectory)
	if err != nil {
		return nil, err
	}
	bucket, ok := buckets[bucketName]
	if !ok {
		return nil, errors.ErrBucketNotFound
	}

	if listObjects {
		filteredObjects := filterObjects(bucket.Objects, prefix)
		return &entities.Bucket{
			Name:            bucket.Name,
			NumberOfObjects: len(filteredObjects),
			Objects:         filteredObjects,
		}, nil
	} else {
		return &entities.Bucket{
			Name:            bucket.Name,
			NumberOfObjects: bucket.NumberOfObjects,
			Objects:         nil,
		}, nil
	}
}

// DONE
// CreateBucket creates a new bucket.
func (fs *FileService) CreateBucket(bucketName string) error {
	buckets, err := scanBucketsDirectory(fs.BucketsDirectory)
	if err != nil {
		return err
	}

	if _, exists := buckets[bucketName]; exists {
		return errors.ErrBucketAlreadyExists
	}

	// Create the bucket directory in the filesystem.
	bucketPath := filepath.Join(fs.BucketsDirectory, bucketName)
	err = os.MkdirAll(bucketPath, 0755)
	if err != nil {
		return err
	}

	return nil
}

// GetAllBucketsMetadata retrieves all buckets' metadata (without the objects).
func (fs *FileService) GetAllBucketsMetadata() ([]*entities.Bucket, error) {
	buckets, err := scanBucketsDirectory(fs.BucketsDirectory)
	if err != nil {
		return nil, err
	}
	bucketList := make([]*entities.Bucket, 0, len(buckets))
	for _, bucket := range buckets {
		// Create a shallow copy of bucket metadata without the objects.
		metadata := entities.Bucket{
			Name:            bucket.Name,
			NumberOfObjects: bucket.NumberOfObjects,
			// Omit the Objects field.
		}
		bucketList = append(bucketList, &metadata)
	}
	return bucketList, nil
}

// DONE
// DeleteBucket deletes a bucket.
func (fs *FileService) DeleteBucket(bucketName string) error {
	buckets, err := scanBucketsDirectory(fs.BucketsDirectory)
	if err != nil {
		return err
	}

	bucket, ok := buckets[bucketName]
	if !ok {
		return errors.ErrBucketNotFound
	}

	if bucket.NumberOfObjects > 0 {
		return errors.ErrBucketNotEmpty
	}

	// Delete the bucket directory from the file system.
	bucketPath := filepath.Join(fs.BucketsDirectory, bucketName)
	if err := os.RemoveAll(bucketPath); err != nil {
		return err
	}

	return nil
}

// DONE
// AddOrUpdateObject adds or updates (if exists) an object in a bucket.
func (fs *FileService) AddOrUpdateObject(bucketName string, objectKey string, file io.Reader) error {
	buckets, err := scanBucketsDirectory(fs.BucketsDirectory)
	if err != nil {
		return err
	}

	_, ok := buckets[bucketName]
	if !ok {
		return errors.ErrBucketNotFound
	}

	// Create the directory for the object if it doesn't exist.
	objectPath := filepath.Join(fs.BucketsDirectory, bucketName, objectKey)
	objectDir := filepath.Dir(objectPath)
	if err := os.MkdirAll(objectDir, 0755); err != nil {
		return err
	}

	// Open the destination file for writing (replace if it exists).
	destFile, err := os.Create(objectPath)
	if err != nil {
		return err
	}

	defer destFile.Close()

	// Copy the uploaded file to the destination file.
	if _, err := io.Copy(destFile, file); err != nil {
		return err
	}

	return nil
}

// GetObject retrieves the object's metadata from the in-memory map.
func (fs *FileService) GetObject(bucketName, objectKey string) (*entities.Object, error) {

	buckets, err := scanBucketsDirectory(fs.BucketsDirectory)
	if err != nil {
		return nil, err
	}

	bucket, ok := buckets[bucketName]
	if !ok {
		return nil, errors.ErrBucketNotFound
	}
	obj, exists := bucket.Objects[objectKey]
	if !exists {
		return nil, errors.ErrObjectNotFound
	}

	// Return the address of the object copy.
	return &obj, nil
}

// DONE
// GetObjectFilePath returns the absolute path of the object file on disk.
func (fs *FileService) GetObjectFilePath(bucketName, objectKey string) (string, error) {
	// Build the path.
	objectPath := filepath.Join(fs.BucketsDirectory, bucketName, objectKey)

	// Check if file exists.
	info, err := os.Stat(objectPath)
	if os.IsNotExist(err) || info.IsDir() {
		return "", errors.ErrObjectNotFound
	} else if err != nil {
		return "", err
	}

	return objectPath, nil
}

// Done
// RemoveObject deletes an object file from disk.
func (fs *FileService) RemoveObject(bucketName, objectKey string) error {
	buckets, err := scanBucketsDirectory(fs.BucketsDirectory)
	if err != nil {
		return err
	}

	bucket, ok := buckets[bucketName]
	if !ok {
		return errors.ErrBucketNotFound
	}

	_, exists := bucket.Objects[objectKey]
	if !exists {
		return errors.ErrObjectNotFound
	}

	// Construct the file path on disk.
	objectPath := filepath.Join(fs.BucketsDirectory, bucketName, objectKey)
	if err := os.Remove(objectPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

// DONE
// RemoveObjects deletes all objects in the specified bucket whose keys start with the given prefix.
func (fs *FileService) RemoveObjects(bucketName, prefix string) error {
	buckets, err := scanBucketsDirectory(fs.BucketsDirectory)
	if err != nil {
		return err
	}
	bucket, ok := buckets[bucketName]
	if !ok {
		return errors.ErrBucketNotFound
	}

	// Iterate over a copy of the keys to avoid modifying the map during iteration.
	for key, _ := range bucket.Objects {
		if strings.HasPrefix(key, prefix) {
			// Construct the absolute path of the object file.
			objectPath := filepath.Join(fs.BucketsDirectory, bucketName, key)

			// Remove the file from disk.
			if err := os.Remove(objectPath); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}

	return nil
}

// filterObjects filters objects based on the prefix.
func filterObjects(objects map[string]entities.Object, prefix string) map[string]entities.Object {
	filtered := make(map[string]entities.Object)
	if objects == nil {
		return filtered
	}
	for key, obj := range objects {
		if prefix == "" || strings.HasPrefix(obj.Key, prefix) {
			filtered[key] = obj
		}
	}
	return filtered
}
