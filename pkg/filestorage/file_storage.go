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
		return nil, err
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		slog.Error("Failed to fetch buckets", "status", resp.Status)
		return nil, errors.New("failed to fetch buckets")
	}

	var buckets []entities.Bucket
	if err := json.NewDecoder(resp.Body).Decode(&buckets); err != nil {
		slog.Error("Error decoding buckets response", "error", err)
		return nil, err
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
		return err
	}
	body := bytes.NewReader(bytesBody)

	resp, err := client.Post(apiURL, "application/json", body)
	if err != nil {
		slog.Error("Error creating bucket", "error", err)
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		slog.Error("Failed to create bucket", "status", resp.Status)
		msg, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("Error reading response body", "error", err)
			return errors.New("failed to create bucket and could not read response body")
		}
		return fmt.Errorf("failed to create bucket %s", string(msg))
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
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		slog.Error("Failed to fetch bucket", "status", resp.Status)
		msg, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("Error reading response body", "error", err)
			return nil, errors.New("failed to create bucket and could not read response body")
		}
		return nil, fmt.Errorf("failed to fetch bucket: %s", string(msg))
	}

	var bucket entities.Bucket
	if err := json.NewDecoder(resp.Body).Decode(&bucket); err != nil {
		slog.Error("Error decoding bucket response", "error", err)
		return nil, err
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
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Error deleting bucket", "error", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Error("Failed to delete bucket", "status", resp.Status)
		msg, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("Error reading response body", "error", err)
			return errors.New("failed to delete bucket and could not read response body")
		}
		return fmt.Errorf("failed to delete bucket: %s", string(msg))
	}

	return nil
}

func (fs *fileStorage) UploadMultipleFiles(bucketName string, directoryPrefix string, files []*os.File) error {
	if len(files) == 0 {
		return nil
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
			return err
		}

		if fileInfo.Size() > int64(maxFileSize) {
			slog.Error("File is larger than maxFileSize", "file_size", fileInfo.Size(), "max_file_size", maxFileSize, "file_name", fileInfo.Name())
			return fmt.Errorf("file %s is larger than allowed maximum. %d>%d", fileInfo.Name(), fileInfo.Size(), maxFileSize)
		}

		objectKeys = append(objectKeys, fileInfo.Name())

		// Create form file
		formFile, err := writer.CreateFormFile("files", fileInfo.Name())
		if err != nil {
			slog.Error("Error creating form file", "error", err, "file_name", fileInfo.Name())
			return err
		}

		// Reset file pointer to beginning
		file.Seek(0, io.SeekStart)

		_, err = io.Copy(formFile, file)
		if err != nil {
			slog.Error("Error copying file to form data", "error", err, "file_name", fileInfo.Name())
			return err
		}
	}

	writer.Close()

	// Create the request
	req, err := http.NewRequest(http.MethodPost, apiURL, &buffer)
	if err != nil {
		slog.Error("Error creating upload request", "error", err)
		return err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Error uploading multiple files", "error", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		slog.Error("Failed to upload multiple files", "status", resp.Status)
		msg, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("Error reading response body", "error", err)
			return errors.New("failed to upload multiple files and could not read response body")
		}
		return fmt.Errorf("failed to upload multiple files: %s", string(msg))
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
		return err
	}
	slog.Info("Request URL", "url", req.URL.String())
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Error removing multiple files", "error", err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		slog.Error("Failed to remove multiple files", "status", resp.Status)
		msg, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("Error reading response body", "error", err)
			return errors.New("failed to remove multiple files and could not read response body")
		}
		return fmt.Errorf("failed to remove multiple files: %s", string(msg))
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
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Error("Failed to fetch file metadata", "status", resp.Status)
		msg, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("Error reading response body", "error", err)
			return nil, errors.New("failed to fetch file metadata and could not read response body")
		}
		return nil, fmt.Errorf("failed to fetch file metadata: %s", string(msg))
	}

	fileContent, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("Error reading file content", "error", err)
		return nil, err
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
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Error("Failed to fetch file metadata", "status", resp.Status)
		msg, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("Error reading response body", "error", err)
			return nil, errors.New("failed to fetch file metadata and could not read response body")
		}
		return nil, fmt.Errorf("failed to fetch file metadata: %s", string(msg))
	}

	metadata := &entities.Object{}
	if err := json.NewDecoder(resp.Body).Decode(metadata); err != nil {
		slog.Error("Error decoding file metadata response", "error", err)
		return nil, err
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
		return nil
	}
	maxFileSize := 10 << 20
	if fileInfo.Size() > int64(maxFileSize) {
		slog.Error("File is larger than maxFileSize", "file size", fileInfo.Size(), "max file size", maxFileSize)
		return fmt.Errorf("Passed file is larger than allowed maximum. %d>%d", fileInfo.Size(), maxFileSize)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	file.Seek(0, io.SeekStart) // Ensure the file pointer is at the start
	buffer, writer, err := createFormWithFile(fileInfo, file)
	if err != nil {
		slog.Error("Could not create form for file upload", "error", err)
		return err
	}

	header := http.Header{}
	header.Set("Content-Type", writer.FormDataContentType())
	parsedURL, err := url.Parse(apiURL)
	if err != nil {
		slog.Error("Error parsing API URL", "error", err)
		return nil
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
		return err
	}

	if resp.StatusCode != http.StatusOK {
		slog.Error("Failed to upload file", "status", resp.Status)
		msg, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("Error reading response body", "error", err)
			return errors.New("failed to upload file and could not read response body")
		}
		return fmt.Errorf("failed to upload file: %s", string(msg))
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
		return nil, nil, err
	}

	_, err = io.Copy(formFile, file)
	if err != nil {
		slog.Error("Error copying file to form data", "error", err)
		return nil, nil, err
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
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Error deleting file", "error", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Error("Failed to delete file", "status", resp.Status)
		msg, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("Error reading response body", "error", err)
			return errors.New("failed to delete file and could not read response body")
		}
		return fmt.Errorf("failed to delete file: %s", string(msg))
	}

	return nil
}
