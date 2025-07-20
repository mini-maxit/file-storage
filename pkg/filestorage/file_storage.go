package filestorage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mini-maxit/file-storage/pkg/filestorage/entities"
)

type FileStorage interface {
	// Bucket operations
	GetBuckets() ([]entities.Bucket, error)
	CreateBucket(bucketName string) error

	// File operations
	GetBucket(bucketName string) (*entities.Bucket, error)
	DeleteBucket(bucketName string) error
	UploadMultipleFiles(bucketName string, directoryPrefix string, files []*os.File) error
	DeleteMultipleFiles(bucketName string, directoryPrefix string) error

	GetFile(bucketName string, objectKey string) ([]byte, error)
	GetFileMetadata(bucketName string, objectKey string) (*entities.Object, error)
	UploadFile(bucketName string, objectKey string, file *os.File) error
	DeleteFile(bucketName string, objectKey string) error
}

type FileStorageConfig struct {
	URL     string
	Version string // Currently not used, but can be used for versioning the storage API
}

type fileStorage struct {
	config FileStorageConfig
}

func NewFileStorage(config FileStorageConfig) (FileStorage, error) {
	if config.URL == "" {
		slog.Error("File storage URL is not set")
		return nil, errors.New("file storage URL is not set")
	}

	config.URL = strings.TrimSuffix(config.URL, "/") // Ensure URL does not end with a slash
	return &fileStorage{
		config: config,
	}, nil
}

func (fs *fileStorage) GetBuckets() ([]entities.Bucket, error) {
	apiPrefix := "/buckets"
	apiURL := fs.config.URL + apiPrefix

	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(apiURL)
	if err != nil {
		slog.Error("Error fetching buckets", "error", err)
		return nil, &ErrClient{
			Message: "failed to fetch buckets",
			Err:     err,
			Context: map[string]any{"url": apiURL},
		}
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		slog.Debug("Failed to fetch buckets", "status", resp.Status)
		msg, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("Error reading response body", "error", err)
			return nil, &ErrClient{
				Message: "failed to fetch buckets and read response body",
				Err:     err,
				Context: map[string]any{"status_code": resp.StatusCode},
			}
		}
		return nil, &ErrAPI{
			statusCode: resp.StatusCode,
			message:    string(msg),
		}
	}

	var buckets []entities.Bucket
	if err := json.NewDecoder(resp.Body).Decode(&buckets); err != nil {
		slog.Error("Error decoding buckets response", "error", err)
		return nil, &ErrClient{
			Message: "failed to decode buckets response",
			Err:     err,
			Context: map[string]any{"response_type": "buckets_list"},
		}
	}

	return buckets, nil
}

func (fs *fileStorage) CreateBucket(bucketName string) error {
	apiPrefix := "/buckets"
	apiURL := fs.config.URL + apiPrefix

	client := &http.Client{Timeout: 10 * time.Second}

	jsonBody := struct {
		Name string `json:"name"`
	}{
		Name: bucketName,
	}
	bytesBody, err := json.Marshal(jsonBody)
	if err != nil {
		slog.Error("Error marshalling bucket creation request", "error", err)
		return &ErrClient{
			Message: "failed to marshal bucket creation request",
			Err:     err,
			Context: map[string]any{"bucket_name": bucketName},
		}
	}
	body := bytes.NewReader(bytesBody)

	resp, err := client.Post(apiURL, "application/json", body)
	if err != nil {
		slog.Error("Error creating bucket", "error", err)
		return &ErrClient{
			Message: "failed to create bucket",
			Err:     err,
			Context: map[string]any{"bucket_name": bucketName, "url": apiURL},
		}
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		slog.Debug("Failed to create bucket", "status", resp.Status)
		msg, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("Error reading response body", "error", err)
			return &ErrClient{
				Message: "failed to create bucket and read response body",
				Err:     err,
				Context: map[string]any{"bucket_name": bucketName, "status_code": resp.StatusCode},
			}
		}
		return &ErrAPI{
			statusCode: resp.StatusCode,
			message:    string(msg),
		}
	}
	return nil
}

func (fs *fileStorage) GetBucket(bucketName string) (*entities.Bucket, error) {
	apiPrefix := fmt.Sprintf("/buckets/%s", bucketName)
	apiURL := fs.config.URL + apiPrefix

	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(apiURL)
	if err != nil {
		slog.Error("Error fetching bucket", "error", err)
		return nil, &ErrClient{
			Message: "failed to fetch bucket",
			Err:     err,
			Context: map[string]any{"bucket_name": bucketName, "url": apiURL},
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("Error reading response body", "error", err)
			return nil, &ErrClient{
				Message: "failed to fetch bucket and read response body",
				Err:     err,
				Context: map[string]any{"bucket_name": bucketName, "status_code": resp.StatusCode},
			}
		}
		return nil, &ErrAPI{
			statusCode: resp.StatusCode,
			message:    string(msg),
		}
	}

	var bucket entities.Bucket
	if err := json.NewDecoder(resp.Body).Decode(&bucket); err != nil {
		slog.Error("Error decoding bucket response", "error", err)
		return nil, &ErrClient{
			Message: "failed to decode bucket response",
			Err:     err,
			Context: map[string]any{"bucket_name": bucketName, "response_type": "bucket"},
		}
	}

	return &bucket, nil
}

func (fs *fileStorage) DeleteBucket(bucketName string) error {
	apiPrefix := fmt.Sprintf("/buckets/%s", bucketName)
	apiURL := fs.config.URL + apiPrefix

	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest(http.MethodDelete, apiURL, nil)
	if err != nil {
		slog.Error("Error creating delete request", "error", err)
		return &ErrClient{
			Message: "failed to create new delete request",
			Err:     err,
			Context: map[string]any{"bucket_name": bucketName, "method": "DELETE"},
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Error deleting bucket", "error", err)
		return &ErrClient{
			Message: "failed to delete bucket",
			Err:     err,
			Context: map[string]any{"bucket_name": bucketName, "url": apiURL},
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Debug("Failed to delete bucket", "status", resp.Status)
		msg, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("Error reading response body", "error", err)
			return &ErrClient{
				Message: "failed to delete bucket and read response body",
				Err:     err,
				Context: map[string]any{"bucket_name": bucketName, "status_code": resp.StatusCode},
			}
		}
		return &ErrAPI{
			statusCode: resp.StatusCode,
			message:    string(msg),
		}
	}

	return nil
}

func (fs *fileStorage) UploadMultipleFiles(bucketName string, directoryPrefix string, files []*os.File) error {
	if len(files) == 0 {
		return errors.New("no files provided for upload")
	}

	apiPrefix := fmt.Sprintf("/buckets/%s/upload-multiple?prefix=%s", bucketName, directoryPrefix)
	apiURL := fs.config.URL + apiPrefix

	client := &http.Client{Timeout: 30 * time.Second} // Longer timeout for multiple files

	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)

	var objectKeys []string
	maxFileSize := 10 << 20 // 10MB per file

	// Add each file to the form
	for i, file := range files {
		fileInfo, err := file.Stat()
		if err != nil {
			slog.Error("Could not stat file", "error", err, "file_index", i)
			return &ErrClient{
				Message: "could not stat file",
				Err:     err,
				Context: map[string]any{"file_index": i, "filename": file.Name(), "bucket_name": bucketName},
			}
		}

		if fileInfo.Size() > int64(maxFileSize) {
			return fmt.Errorf("file '%s' is larger than %d. got=%d. ", fileInfo.Name(), maxFileSize, fileInfo.Size())
		}

		objectKeys = append(objectKeys, fileInfo.Name())

		// Create form file
		formFile, err := writer.CreateFormFile("files", fileInfo.Name())
		if err != nil {
			slog.Error("Error creating form file", "error", err, "filename", fileInfo.Name())
			return &ErrClient{
				Message: "error creating form file",
				Err:     err,
				Context: map[string]any{"filename": fileInfo.Name(), "bucket_name": bucketName},
			}
		}

		// Reset file pointer to beginning
		file.Seek(0, io.SeekStart)

		_, err = io.Copy(formFile, file)
		if err != nil {
			slog.Error("Error copying file to form data", "error", err, "filename", fileInfo.Name())
			return &ErrClient{
				Message: "error copying file to form data",
				Err:     err,
				Context: map[string]any{"filename": fileInfo.Name(), "bucket_name": bucketName},
			}
		}
	}

	writer.Close()

	// Create the request
	req, err := http.NewRequest(http.MethodPost, apiURL, &buffer)
	if err != nil {
		slog.Error("Error creating upload request", "error", err)
		return &ErrClient{
			Message: "failed to create new upload request",
			Err:     err,
			Context: map[string]any{"bucket_name": bucketName, "directory_prefix": directoryPrefix, "file_count": len(files)},
		}
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Error uploading multiple files", "error", err)
		return &ErrClient{
			Message: "failed to upload multiple files",
			Err:     err,
			Context: map[string]any{"bucket_name": bucketName, "directory_prefix": directoryPrefix, "url": apiURL},
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Error("Failed to upload multiple files", "status", resp.Status)
		msg, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("Error reading response body", "error", err)
			return &ErrClient{
				Message: "failed to upload multiple files and read response body",
				Err:     err,
				Context: map[string]any{"bucket_name": bucketName, "status_code": resp.StatusCode},
			}
		}
		return &ErrAPI{
			statusCode: resp.StatusCode,
			message:    string(msg),
		}
	}

	return nil
}

func (fs *fileStorage) DeleteMultipleFiles(bucketName string, directoryPrefix string) error {
	apiPrefix := fmt.Sprintf("/buckets/%s/remove-multiple?prefix=%s", bucketName, directoryPrefix)
	apiURL := fs.config.URL + apiPrefix

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodDelete, apiURL, nil)
	if err != nil {
		slog.Error("Error creating delete request for multiple files", "error", err)
		return &ErrClient{
			Message: "failed to create new delete request for multiple files",
			Err:     err,
			Context: map[string]any{"bucket_name": bucketName, "directory_prefix": directoryPrefix, "method": "DELETE"},
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Error removing multiple files", "error", err)
		return &ErrClient{
			Message: "failed to remove multiple files",
			Err:     err,
			Context: map[string]any{"bucket_name": bucketName, "directory_prefix": directoryPrefix, "url": apiURL},
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Debug("Failed to remove multiple files", "status", resp.Status)
		msg, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("Error reading response body", "error", err)
			return &ErrClient{
				Message: "failed to remove multiple files and read response body",
				Err:     err,
				Context: map[string]any{"bucket_name": bucketName, "status_code": resp.StatusCode},
			}
		}
		return &ErrAPI{
			statusCode: resp.StatusCode,
			message:    string(msg),
		}
	}

	return nil
}

func (fs *fileStorage) GetFile(bucketName string, objectKey string) ([]byte, error) {
	apiPrefix := fmt.Sprintf("/buckets/%s/%s?metadataOnly=false", bucketName, objectKey)
	apiURL := fs.config.URL + apiPrefix

	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(apiURL)
	if err != nil {
		slog.Error("Error fetching file metadata", "error", err)
		return nil, &ErrClient{
			Message: "failed to fetch file",
			Err:     err,
			Context: map[string]any{"bucket_name": bucketName, "object_key": objectKey, "url": apiURL},
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Debug("Failed to fetch file metadata", "status", resp.Status)
		msg, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("Error reading response body", "error", err)
			return nil, &ErrClient{
				Message: "failed to fetch file and read response body",
				Err:     err,
				Context: map[string]any{"bucket_name": bucketName, "object_key": objectKey, "status_code": resp.StatusCode},
			}
		}
		return nil, &ErrAPI{
			statusCode: resp.StatusCode,
			message:    string(msg),
		}
	}

	fileContent, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("Error reading file content", "error", err)
		return nil, &ErrClient{
			Message: "failed to read file content",
			Err:     err,
			Context: map[string]any{"bucket_name": bucketName, "object_key": objectKey},
		}
	}
	return fileContent, nil
}

func (fs *fileStorage) GetFileMetadata(bucketName string, objectKey string) (*entities.Object, error) {
	apiPrefix := fmt.Sprintf("/buckets/%s/%s?metadataOnly=true", bucketName, objectKey)
	apiURL := fs.config.URL + apiPrefix

	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(apiURL)
	if err != nil {
		slog.Error("Error fetching file metadata", "error", err)
		return nil, &ErrClient{
			Message: "failed to fetch file metadata",
			Err:     err,
			Context: map[string]any{"bucket_name": bucketName, "object_key": objectKey, "url": apiURL},
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Error("Failed to fetch file metadata", "status", resp.Status)
		msg, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("Error reading response body", "error", err)
			return nil, &ErrClient{
				Message: "failed to fetch file metadata and read response body",
				Err:     err,
				Context: map[string]any{"bucket_name": bucketName, "object_key": objectKey, "status_code": resp.StatusCode},
			}
		}
		return nil, &ErrAPI{
			statusCode: resp.StatusCode,
			message:    string(msg),
		}
	}

	metadata := &entities.Object{}
	if err := json.NewDecoder(resp.Body).Decode(metadata); err != nil {
		slog.Error("Error decoding file metadata response", "error", err)
		return nil, &ErrClient{
			Message: "failed to decode file metadata response",
			Err:     err,
			Context: map[string]any{"bucket_name": bucketName, "object_key": objectKey, "response_type": "metadata"},
		}
	}

	return metadata, nil
}

func (fs *fileStorage) UploadFile(bucketName string, objectKey string, file *os.File) error {
	apiPrefix := fmt.Sprintf("/buckets/%s/%s", bucketName, objectKey)
	apiURL := fs.config.URL + apiPrefix
	// verify that file is not larger than 10MB
	fileInfo, err := file.Stat()
	if err != nil {
		slog.Error("Could not stat file", "error", err)
		return &ErrClient{
			Message: "could not stat file",
			Err:     err,
			Context: map[string]any{"bucket_name": bucketName, "object_key": objectKey},
		}
	}
	maxFileSize := 10 << 20
	if fileInfo.Size() > int64(maxFileSize) {
		return fmt.Errorf("file %s is larger than %d. got=%d", fileInfo.Name(), maxFileSize, fileInfo.Size())
	}

	client := &http.Client{Timeout: 10 * time.Second}
	file.Seek(0, io.SeekStart) // Ensure the file pointer is at the start
	buffer, writer, err := createFormWithFile(fileInfo, file)
	if err != nil {
		slog.Error("Could not create form for file upload", "error", err)
		return &ErrClient{
			Message: "could not create form for file upload",
			Err:     err,
			Context: map[string]any{"bucket_name": bucketName, "object_key": objectKey, "filename": fileInfo.Name()},
		}
	}

	header := http.Header{}
	header.Set("Content-Type", writer.FormDataContentType())
	parsedURL, err := url.Parse(apiURL)
	if err != nil {
		slog.Error("Error parsing API URL", "error", err)
		return &ErrClient{
			Message: "failed to parse API URL",
			Err:     err,
			Context: map[string]any{"url": apiURL, "bucket_name": bucketName, "object_key": objectKey},
		}
	}

	req := &http.Request{
		Method: http.MethodPut,
		URL:    parsedURL,
		Header: header,
		Body:   io.NopCloser(buffer),
	}

	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Error making request to filestorage", "error", err)
		return &ErrClient{
			Message: "failed to upload file",
			Err:     err,
			Context: map[string]any{"bucket_name": bucketName, "object_key": objectKey, "url": apiURL},
		}
	}

	if resp.StatusCode != http.StatusOK {
		slog.Error("Failed to upload file", "status", resp.Status)
		msg, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("Error reading response body", "error", err)
			return &ErrClient{
				Message: "failed to upload file and read response body",
				Err:     err,
				Context: map[string]any{"bucket_name": bucketName, "object_key": objectKey, "status_code": resp.StatusCode},
			}
		}
		return &ErrAPI{
			statusCode: resp.StatusCode,
			message:    string(msg),
		}
	}
	return nil
}

func createFormWithFile(fileInfo os.FileInfo, file *os.File) (*bytes.Buffer, *multipart.Writer, error) {
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	defer writer.Close()
	formFile, err := writer.CreateFormFile("file", fileInfo.Name())
	if err != nil {
		slog.Error("Error creating form file", "error", err)
		return nil, nil, fmt.Errorf("failed to create formFile for file %s", fileInfo.Name())
	}

	_, err = io.Copy(formFile, file)
	if err != nil {
		slog.Error("Error copying file to form data", "error", err)
		return nil, nil, fmt.Errorf("failed to copy file %s to form data: %w", fileInfo.Name(), err)
	}
	return &buffer, writer, nil
}

func (fs *fileStorage) DeleteFile(bucketName string, objectKey string) error {
	apiPrefix := fmt.Sprintf("/buckets/%s/%s", bucketName, objectKey)
	apiURL := fs.config.URL + apiPrefix

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodDelete, apiURL, nil)
	if err != nil {
		slog.Error("Error creating delete request for file", "error", err)
		return &ErrClient{
			Message: "failed to create new delete request for file",
			Err:     err,
			Context: map[string]any{"bucket_name": bucketName, "object_key": objectKey, "method": "DELETE"},
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Error deleting file", "error", err)
		return &ErrClient{
			Message: "failed to delete file",
			Err:     err,
			Context: map[string]any{"bucket_name": bucketName, "object_key": objectKey, "url": apiURL},
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		slog.Error("Failed to delete file", "status", resp.Status)
		msg, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("Error reading response body", "error", err)
			return &ErrClient{
				Message: "failed to delete file and read response body",
				Err:     err,
				Context: map[string]any{"bucket_name": bucketName, "object_key": objectKey, "status_code": resp.StatusCode},
			}
		}
		return &ErrAPI{
			statusCode: resp.StatusCode,
			message:    string(msg),
		}
	}

	return nil
}
