package main

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mini-maxit/file-storage/internal/api/http/server"
	"github.com/mini-maxit/file-storage/internal/api/services"
	"github.com/mini-maxit/file-storage/internal/config"
	"github.com/mini-maxit/file-storage/internal/logger"
	"github.com/mini-maxit/file-storage/pkg/urlsigner"
)

func TestSignedURLIntegration(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	testSecret := "test-signing-secret"

	// Create config
	cfg := &config.Config{
		Port:          "8080",
		RootDirectory: tempDir,
		SigningSecret: testSecret,
	}

	// Create file service
	fileService := services.NewFileService(cfg)

	// Create URL signer
	signer := urlsigner.NewURLSigner(testSecret)

	// Initialize logger
	logger.InitializeLogger()
	log := logger.NewNamedLogger("test")

	// Create server
	srv := server.NewServer(fileService, signer, "test-internal-key", log)

	// Create a test bucket
	createBucket(t, srv, "test-bucket")

	// Upload a PDF file
	uploadPDF(t, srv, "test-bucket", "test.pdf", []byte("PDF content"))

	// Test 1: Valid signed URL should work
	t.Run("ValidSignedURL", func(t *testing.T) {
		path := "/buckets/test-bucket/test.pdf"
		signedURL, err := signer.SignURL(path, 1*time.Hour)
		if err != nil {
			t.Fatalf("Failed to sign URL: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, signedURL, nil)
		w := httptest.NewRecorder()

		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d: %s", w.Code, w.Body.String())
		}

		body, _ := io.ReadAll(w.Body)
		if string(body) != "PDF content" {
			t.Errorf("Expected 'PDF content', got %s", string(body))
		}
	})

	// Test 2: Expired signed URL should fail
	t.Run("ExpiredSignedURL", func(t *testing.T) {
		path := "/buckets/test-bucket/test.pdf"
		signedURL, err := signer.SignURL(path, -1*time.Hour)
		if err != nil {
			t.Fatalf("Failed to sign URL: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, signedURL, nil)
		w := httptest.NewRecorder()

		srv.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403, got %d", w.Code)
		}
	})

	// Test 3: Invalid signature should fail
	t.Run("InvalidSignature", func(t *testing.T) {
		path := "/buckets/test-bucket/test.pdf"
		expires := time.Now().Add(1 * time.Hour).Unix()
		invalidURL := fmt.Sprintf("%s?expires=%d&signature=invalid", path, expires)

		req := httptest.NewRequest(http.MethodGet, invalidURL, nil)
		w := httptest.NewRecorder()

		srv.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403, got %d", w.Code)
		}
	})

	// Test 4: Missing signature for PDF should fail
	t.Run("MissingSignature", func(t *testing.T) {
		path := "/buckets/test-bucket/test.pdf"
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()

		srv.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403, got %d", w.Code)
		}
	})

	// Test 5: Metadata request requires internal key (no signature needed)
	t.Run("MetadataWithoutSignature", func(t *testing.T) {
		path := "/buckets/test-bucket/test.pdf?metadataOnly=true"
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Internal-Key", "test-internal-key")
		w := httptest.NewRecorder()

		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200 for metadata request with internal key, got %d: %s", w.Code, w.Body.String())
		}
	})

	// Test 6: All files require a signature — non-PDF files are no exception
	t.Run("NonPDFWithoutSignature", func(t *testing.T) {
		uploadFile(t, srv, "test-bucket", "test.txt", []byte("Text content"))

		path := "/buckets/test-bucket/test.txt"
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()

		srv.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403 for non-PDF file without signature, got %d: %s", w.Code, w.Body.String())
		}
	})
}

// Helper function to create a bucket
func createBucket(t *testing.T, srv http.Handler, bucketName string) {
	body := fmt.Sprintf(`{"name":"%s"}`, bucketName)
	req := httptest.NewRequest(http.MethodPost, "/buckets", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Key", "test-internal-key")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated && w.Code != http.StatusConflict {
		t.Fatalf("Failed to create bucket: %d - %s", w.Code, w.Body.String())
	}
}

// Helper function to upload a PDF
func uploadPDF(t *testing.T, srv http.Handler, bucketName, fileName string, content []byte) {
	uploadFile(t, srv, bucketName, fileName, content)
}

// Helper function to upload any file
func uploadFile(t *testing.T, srv http.Handler, bucketName, fileName string, content []byte) {
	var b bytes.Buffer
	writer := multipart.NewWriter(&b)

	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}

	_, err = part.Write(content)
	if err != nil {
		t.Fatalf("Failed to write file content: %v", err)
	}

	writer.Close()

	url := fmt.Sprintf("/buckets/%s/%s", bucketName, fileName)
	req := httptest.NewRequest(http.MethodPut, url, &b)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Internal-Key", "test-internal-key")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to upload file: %d - %s", w.Code, w.Body.String())
	}
}
