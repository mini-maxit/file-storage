package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
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

	// Create file service (shared between both servers)
	fileService := services.NewFileService(cfg)

	// Create URL signer
	signer := urlsigner.NewURLSigner(testSecret)

	// Initialize logger
	logger.InitializeLogger()
	log := logger.NewNamedLogger("test")

	// Public server: signed-URL validation, GET only
	publicSrv := server.NewServer(fileService, signer, log)
	// Internal server: no auth, all CRUD operations
	internalSrv := server.NewInternalServer(fileService, signer, log)

	// Seed data via internal server (no auth needed)
	createBucket(t, internalSrv, "test-bucket")
	uploadPDF(t, internalSrv, "test-bucket", "test.pdf", []byte("PDF content"))

	// Test 1: Valid signed URL should work on public server
	t.Run("ValidSignedURL", func(t *testing.T) {
		path := "/buckets/test-bucket/test.pdf"
		signedURL, err := signer.SignURL(path, 1*time.Hour)
		if err != nil {
			t.Fatalf("Failed to sign URL: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, signedURL, nil)
		w := httptest.NewRecorder()

		publicSrv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d: %s", w.Code, w.Body.String())
		}

		body, _ := io.ReadAll(w.Body)
		if string(body) != "PDF content" {
			t.Errorf("Expected 'PDF content', got %s", string(body))
		}
	})

	// Test 2: Expired signed URL should fail on public server
	t.Run("ExpiredSignedURL", func(t *testing.T) {
		path := "/buckets/test-bucket/test.pdf"
		signedURL, err := signer.SignURL(path, -1*time.Hour)
		if err != nil {
			t.Fatalf("Failed to sign URL: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, signedURL, nil)
		w := httptest.NewRecorder()

		publicSrv.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403, got %d", w.Code)
		}
	})

	// Test 3: Invalid signature should fail on public server
	t.Run("InvalidSignature", func(t *testing.T) {
		path := "/buckets/test-bucket/test.pdf"
		expires := time.Now().Add(1 * time.Hour).Unix()
		invalidURL := fmt.Sprintf("%s?expires=%d&signature=invalid", path, expires)

		req := httptest.NewRequest(http.MethodGet, invalidURL, nil)
		w := httptest.NewRecorder()

		publicSrv.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403, got %d", w.Code)
		}
	})

	// Test 4: Missing signature should fail on public server
	t.Run("MissingSignature", func(t *testing.T) {
		path := "/buckets/test-bucket/test.pdf"
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()

		publicSrv.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403, got %d", w.Code)
		}
	})

	// Test 5: Metadata request is internal-only — allowed on internal server, rejected on public server
	t.Run("MetadataViaInternalServer", func(t *testing.T) {
		path := "/buckets/test-bucket/test.pdf?metadataOnly=true"
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()

		internalSrv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200 for metadata request on internal server, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("MetadataRejectedOnPublicServer", func(t *testing.T) {
		path := "/buckets/test-bucket/test.pdf?metadataOnly=true"
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()

		publicSrv.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403 for metadata request on public server, got %d", w.Code)
		}
	})

	// Test 6: All files require a signature on public server — non-PDF files are no exception
	t.Run("NonPDFWithoutSignature", func(t *testing.T) {
		uploadFile(t, internalSrv, "test-bucket", "test.txt", []byte("Text content"))

		path := "/buckets/test-bucket/test.txt"
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()

		publicSrv.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403 for non-PDF file without signature, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestSignEndpoint(t *testing.T) {
	tempDir := t.TempDir()
	testSecret := "test-sign-endpoint-secret"

	cfg := &config.Config{
		Port:          "8080",
		RootDirectory: tempDir,
		SigningSecret: testSecret,
	}

	fileService := services.NewFileService(cfg)
	signer := urlsigner.NewURLSigner(testSecret)

	logger.InitializeLogger()
	log := logger.NewNamedLogger("test-sign")

	internalSrv := server.NewInternalServer(fileService, signer, log)

	// Create a bucket and object so the key exists
	createBucket(t, internalSrv, "sign-bucket")
	uploadFile(t, internalSrv, "sign-bucket", "doc.pdf", []byte("hello"))

	t.Run("ValidRequest", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/sign?bucket=sign-bucket&key=doc.pdf&ttl=600", nil)
		w := httptest.NewRecorder()
		internalSrv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var resp struct {
			SignedPath string `json:"signedPath"`
		}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		if !strings.HasPrefix(resp.SignedPath, "/buckets/sign-bucket/doc.pdf?") {
			t.Errorf("Expected signedPath to start with /buckets/sign-bucket/doc.pdf?, got %s", resp.SignedPath)
		}
		if !strings.Contains(resp.SignedPath, "expires=") || !strings.Contains(resp.SignedPath, "signature=") {
			t.Errorf("Expected signedPath to contain expires and signature params, got %s", resp.SignedPath)
		}
	})

	t.Run("DefaultTTL", func(t *testing.T) {
		// No ttl param — should use default 300s
		req := httptest.NewRequest(http.MethodGet, "/sign?bucket=sign-bucket&key=doc.pdf", nil)
		w := httptest.NewRecorder()
		internalSrv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var resp struct {
			SignedPath string `json:"signedPath"`
		}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		if resp.SignedPath == "" {
			t.Error("Expected non-empty signedPath")
		}
	})

	t.Run("MissingBucket", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/sign?key=doc.pdf&ttl=60", nil)
		w := httptest.NewRecorder()
		internalSrv.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400, got %d", w.Code)
		}
	})

	t.Run("MissingKey", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/sign?bucket=sign-bucket&ttl=60", nil)
		w := httptest.NewRecorder()
		internalSrv.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected 400, got %d", w.Code)
		}
	})

	t.Run("MethodNotAllowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/sign?bucket=sign-bucket&key=doc.pdf", nil)
		w := httptest.NewRecorder()
		internalSrv.ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected 405, got %d", w.Code)
		}
	})
}

// Helper function to create a bucket
func createBucket(t *testing.T, srv http.Handler, bucketName string) {
	body := fmt.Sprintf(`{"name":"%s"}`, bucketName)
	req := httptest.NewRequest(http.MethodPost, "/buckets", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
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
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to upload file: %d - %s", w.Code, w.Body.String())
	}
}
