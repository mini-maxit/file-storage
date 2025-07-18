package filestorage_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mini-maxit/file-storage/pkg/filestorage"
	"github.com/mini-maxit/file-storage/pkg/filestorage/entities"
)

func TestGetBuckets(t *testing.T) {
	expectedBuckets := []entities.Bucket{
		{
			Name:            "test-bucket",
			CreationDate:    time.Date(2023, time.October, 1, 0, 0, 0, 0, time.UTC),
			NumberOfObjects: 10,
			Size:            1024,
		},
		{
			Name:            "another-bucket",
			CreationDate:    time.Date(2023, time.December, 2, 0, 0, 0, 0, time.UTC),
			NumberOfObjects: 5,
			Size:            512,
		},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/buckets" || r.Method != http.MethodGet {
			t.Errorf("Expected GET request to /buckets, got %s to %s", r.Method, r.URL.Path)
			return
		}
		w.WriteHeader(200)
		bytes, err := json.Marshal(expectedBuckets)
		if err != nil {
			t.Fatalf("Failed to marshal expected buckets: %v", err)
		}
		w.Write(bytes)
	}))
	defer server.Close()
	config := filestorage.FileStorageConfig{
		URL: server.URL,
	}
	storage, err := filestorage.NewFileStorage(config)
	if err != nil {
		t.Fatalf("Failed to create file storage: %v", err)
	}

	buckets, err := storage.GetBuckets()
	if err != nil {
		t.Fatalf("Failed to get buckets: %v", err)
	}

	if len(buckets) == 0 {
		t.Error("Expected at least one bucket, got none")
	}
}

func TestCreateBucket(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/buckets" || r.Method != http.MethodPost {
			t.Errorf("Expected POST request to /buckets, got %s %s", r.Method, r.URL.Path)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	config := filestorage.FileStorageConfig{
		URL: server.URL,
	}
	storage, err := filestorage.NewFileStorage(config)
	if err != nil {
		t.Fatalf("Failed to create file storage: %v", err)
	}

	err = storage.CreateBucket("test-bucket")
	if err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}
}

func TestGetBucket(t *testing.T) {
	expectedBucket := entities.Bucket{
		Name:            "test-bucket",
		CreationDate:    time.Date(2023, time.October, 1, 0, 0, 0, 0, time.UTC),
		NumberOfObjects: 10,
		Size:            1024,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/buckets/test-bucket" || r.Method != http.MethodGet {
			t.Errorf("Expected GET request to /buckets/test-bucket, got %s %s", r.Method, r.URL.Path)
			return
		}
		w.WriteHeader(http.StatusOK)
		bytes, err := json.Marshal(expectedBucket)
		if err != nil {
			t.Fatalf("Failed to marshal expected bucket: %v", err)
		}
		w.Write(bytes)
	}))
	defer server.Close()

	config := filestorage.FileStorageConfig{
		URL: server.URL,
	}
	storage, err := filestorage.NewFileStorage(config)
	if err != nil {
		t.Fatalf("Failed to create file storage: %v", err)
	}

	bucket, err := storage.GetBucket("test-bucket")
	if err != nil {
		t.Fatalf("Failed to get bucket: %v", err)
	}

	if !reflect.DeepEqual(*bucket, expectedBucket) {
		t.Errorf("Expected bucket %v, got %v", expectedBucket, *bucket)
	}
}

func TestDeleteBucket(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/buckets/test-bucket" || r.Method != http.MethodDelete {
			t.Errorf("Expected DELETE request to /buckets/test-bucket, got %s %s", r.Method, r.URL.Path)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := filestorage.FileStorageConfig{
		URL: server.URL,
	}
	storage, err := filestorage.NewFileStorage(config)
	if err != nil {
		t.Fatalf("Failed to create file storage: %v", err)
	}

	err = storage.DeleteBucket("test-bucket")
	if err != nil {
		t.Fatalf("Failed to delete bucket: %v", err)
	}
}

func TestGetFileMetadata(t *testing.T) {
	expectedMetadata := entities.Object{
		Key:          "test-file",
		Size:         2048,
		LastModified: time.Date(2023, time.October, 1, 0, 0, 0, 0, time.UTC),
		Type:         "",
	}
	bucketName := "test-bucket"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != fmt.Sprintf("/buckets/%s/%s", bucketName, expectedMetadata.Key) || r.Method != http.MethodGet {
			t.Errorf("Expected GET request to /buckets/test-bucket/objects/test-file, got %s %s", r.Method, r.URL.Path)
			return
		}
		w.WriteHeader(http.StatusOK)
		bytes, err := json.Marshal(expectedMetadata)
		if err != nil {
			t.Fatalf("Failed to marshal file metadata: %v", err)
		}
		w.Write(bytes)
	}))
	defer server.Close()

	config := filestorage.FileStorageConfig{
		URL: server.URL,
	}
	storage, err := filestorage.NewFileStorage(config)
	if err != nil {
		t.Fatalf("Failed to create file storage: %v", err)
	}

	metadata, err := storage.GetFileMetadata(bucketName, expectedMetadata.Key)
	if err != nil {
		t.Fatalf("Failed to get file metadata: %v", err)
	}
	if !reflect.DeepEqual(*metadata, expectedMetadata) {
		t.Errorf("Expected file metadata %v, got %v", expectedMetadata, *metadata)
	}
}

func TestGetFile(t *testing.T) {
	expectedFile := []byte("This is a test file content")
	filename := "test-file"
	bucketName := "test-bucket"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RequestURI() != fmt.Sprintf("/buckets/%s/%s?metadataOnly=false", bucketName, filename) || r.Method != http.MethodGet {
			t.Errorf("Expected GET request to /buckets/test-bucket/test-file, got %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(expectedFile)
	}))
	defer server.Close()

	config := filestorage.FileStorageConfig{
		URL: server.URL,
	}

	storage, err := filestorage.NewFileStorage(config)
	if err != nil {
		t.Fatalf("Failed to create file storage: %v", err)
	}

	fileContent, err := storage.GetFile(bucketName, filename)
	if err != nil {
		t.Fatalf("Failed to get file: %v", err)
	}

	if !reflect.DeepEqual(fileContent, expectedFile) {
		t.Errorf("Expected file content %s, got %s", expectedFile, fileContent)
	}
}

func TestDeleteFile(t *testing.T) {
	bucketName := "test-bucket"
	filename := "test-file"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != fmt.Sprintf("/buckets/%s/%s", bucketName, filename) || r.Method != http.MethodDelete {
			t.Errorf("Expected DELETE request to /buckets/test-bucket/test-file, got %s %s", r.Method, r.URL.Path)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := filestorage.FileStorageConfig{
		URL: server.URL,
	}
	storage, err := filestorage.NewFileStorage(config)
	if err != nil {
		t.Fatalf("Failed to create file storage: %v", err)
	}

	err = storage.DeleteFile(bucketName, filename)
	if err != nil {
		t.Fatalf("Failed to delete file: %v", err)
	}
}

func TestUploadFile(t *testing.T) {
	fileContent := []byte("This is a test file content")
	filename := "test-file"
	file, err := os.CreateTemp(os.TempDir(), "")
	if err != nil {
		t.Fatalf("Failed to create temporary file: %v", err)
	}
	_, err = file.Write(fileContent)
	if err != nil {
		t.Fatalf("Failed to write to temporary file: %v", err)
	}
	bucketName := "test-bucket"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != fmt.Sprintf("/buckets/%s/%s", bucketName, filename) || r.Method != http.MethodPut {
			t.Errorf("Expected PUT request to /buckets/%s/%s, got %s %s", bucketName, filename, r.Method, r.URL.Path)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := filestorage.FileStorageConfig{
		URL: server.URL,
	}
	storage, err := filestorage.NewFileStorage(config)
	if err != nil {
		t.Fatalf("Failed to create file storage: %v", err)
	}

	err = storage.UploadFile(bucketName, filename, file)
	if err != nil {
		t.Fatalf("Failed to upload file: %v", err)
	}
}

func TestUploadMultipleFiles(t *testing.T) {
	directoryPrefix := "uploads/documents"
	bucketName := "test-bucket"

	// Create test files
	file1, err := os.CreateTemp(os.TempDir(), "test1*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file 1: %v", err)
	}
	defer os.Remove(file1.Name())
	file1.WriteString("Content of file 1")
	file1.Close()

	file2, err := os.CreateTemp(os.TempDir(), "test2*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file 2: %v", err)
	}
	defer os.Remove(file2.Name())
	file2.WriteString("Content of file 2")
	file2.Close()

	// Reopen files for reading
	file1, err = os.Open(file1.Name())
	if err != nil {
		t.Fatalf("Failed to reopen file 1: %v", err)
	}
	defer file1.Close()

	file2, err = os.Open(file2.Name())
	if err != nil {
		t.Fatalf("Failed to reopen file 2: %v", err)
	}
	defer file2.Close()

	files := []*os.File{file1, file2}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != fmt.Sprintf("/buckets/%s/upload-multiple", bucketName) || r.Method != http.MethodPost {
			t.Errorf("Expected POST request to /buckets/%s/upload-multiple, got %s %s", bucketName, r.Method, r.URL.Path)
			return
		}

		// Verify content type is multipart/form-data
		contentType := r.Header.Get("Content-Type")
		if !strings.Contains(contentType, "multipart/form-data") {
			t.Errorf("Expected multipart/form-data content type, got %s", contentType)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := filestorage.FileStorageConfig{
		URL: server.URL,
	}

	storage, err := filestorage.NewFileStorage(config)
	if err != nil {
		t.Fatalf("Failed to create file storage: %v", err)
	}

	err = storage.UploadMultipleFiles(bucketName, directoryPrefix, files)
	if err != nil {
		t.Fatalf("Failed to upload multiple files: %v", err)
	}
}

func TestUploadMultipleFilesWithoutPrefix(t *testing.T) {
	bucketName := "test-bucket"

	// Create test files
	file1, err := os.CreateTemp(os.TempDir(), "test1*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file 1: %v", err)
	}
	defer os.Remove(file1.Name())
	file1.WriteString("Content of file 1")
	file1.Close()

	file2, err := os.CreateTemp(os.TempDir(), "test2*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file 2: %v", err)
	}
	defer os.Remove(file2.Name())
	file2.WriteString("Content of file 2")
	file2.Close()

	// Reopen files for reading
	file1, err = os.Open(file1.Name())
	if err != nil {
		t.Fatalf("Failed to reopen file 1: %v", err)
	}
	defer file1.Close()

	file2, err = os.Open(file2.Name())
	if err != nil {
		t.Fatalf("Failed to reopen file 2: %v", err)
	}
	defer file2.Close()

	files := []*os.File{file1, file2}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != fmt.Sprintf("/buckets/%s/upload-multiple", bucketName) || r.Method != http.MethodPost {
			t.Errorf("Expected POST request to /buckets/%s/upload-multiple, got %s %s", bucketName, r.Method, r.URL.Path)
			return
		}

		// Verify content type is multipart/form-data
		contentType := r.Header.Get("Content-Type")
		if !strings.Contains(contentType, "multipart/form-data") {
			t.Errorf("Expected multipart/form-data content type, got %s", contentType)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := filestorage.FileStorageConfig{
		URL: server.URL,
	}

	storage, err := filestorage.NewFileStorage(config)
	if err != nil {
		t.Fatalf("Failed to create file storage: %v", err)
	}

	err = storage.UploadMultipleFiles(bucketName, "", files)
	if err != nil {
		t.Fatalf("Failed to upload multiple files: %v", err)
	}
}

func TestUploadMultipleFilesEmpty(t *testing.T) {
	bucketName := "test-bucket"
	files := []*os.File{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("Expected no request for empty file list, got %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	config := filestorage.FileStorageConfig{
		URL: server.URL,
	}

	storage, err := filestorage.NewFileStorage(config)
	if err != nil {
		t.Fatalf("Failed to create file storage: %v", err)
	}

	err = storage.UploadMultipleFiles(bucketName, "prefix", files)
	if err != nil {
		t.Fatalf("Failed to upload empty file list: %v", err)
	}

}

func TestDeleteMultipleFiles(t *testing.T) {
	directoryPrefix := "dir1/dir2/dir3-files"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RequestURI() != fmt.Sprintf("/buckets/test-bucket/remove-multiple?prefix=%s", directoryPrefix) || r.Method != http.MethodDelete {
			t.Errorf("Expected DELETE request to /buckets/test-bucket/remove-multiple?prefix=%s, got %s %s", directoryPrefix, r.Method, r.URL.Path)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := filestorage.FileStorageConfig{
		URL: server.URL,
	}

	storage, err := filestorage.NewFileStorage(config)
	if err != nil {
		t.Fatalf("Failed to create file storage: %v", err)
	}
	err = storage.DeleteMultipleFiles("test-bucket", directoryPrefix)
	if err != nil {
		t.Fatalf("Failed to delete multiple files: %v", err)
	}
}
