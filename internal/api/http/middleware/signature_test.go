package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mini-maxit/file-storage/pkg/urlsigner"
	"go.uber.org/zap"
)

func TestSignatureValidationMiddleware_ValidSignature(t *testing.T) {
	// Create a test signer
	signer := urlsigner.NewURLSigner("test-secret")
	
	// Create a test logger
	logger, _ := zap.NewDevelopment()
	sugaredLogger := logger.Sugar()

	// Create a simple handler that returns 200 OK
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap the handler with the signature validation middleware
	middleware := SignatureValidationMiddleware(nextHandler, signer, sugaredLogger)

	// Sign a URL
	path := "/buckets/test-bucket/test-file.pdf"
	signedURL, err := signer.SignURL(path, 1*time.Hour)
	if err != nil {
		t.Fatalf("Failed to sign URL: %v", err)
	}

	// Create a request with the signed URL
	req := httptest.NewRequest(http.MethodGet, signedURL, nil)
	w := httptest.NewRecorder()

	// Execute the middleware
	middleware.ServeHTTP(w, req)

	// Check that the request was successful
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestSignatureValidationMiddleware_ExpiredSignature(t *testing.T) {
	// Create a test signer
	signer := urlsigner.NewURLSigner("test-secret")
	
	// Create a test logger
	logger, _ := zap.NewDevelopment()
	sugaredLogger := logger.Sugar()

	// Create a simple handler that should not be reached
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called for expired signature")
		w.WriteHeader(http.StatusOK)
	})

	// Wrap the handler with the signature validation middleware
	middleware := SignatureValidationMiddleware(nextHandler, signer, sugaredLogger)

	// Sign a URL with negative TTL (already expired)
	path := "/buckets/test-bucket/test-file.pdf"
	signedURL, err := signer.SignURL(path, -1*time.Hour)
	if err != nil {
		t.Fatalf("Failed to sign URL: %v", err)
	}

	// Create a request with the expired signed URL
	req := httptest.NewRequest(http.MethodGet, signedURL, nil)
	w := httptest.NewRecorder()

	// Execute the middleware
	middleware.ServeHTTP(w, req)

	// Check that the request was forbidden
	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestSignatureValidationMiddleware_InvalidSignature(t *testing.T) {
	// Create a test signer
	signer := urlsigner.NewURLSigner("test-secret")
	
	// Create a test logger
	logger, _ := zap.NewDevelopment()
	sugaredLogger := logger.Sugar()

	// Create a simple handler that should not be reached
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called for invalid signature")
		w.WriteHeader(http.StatusOK)
	})

	// Wrap the handler with the signature validation middleware
	middleware := SignatureValidationMiddleware(nextHandler, signer, sugaredLogger)

	// Create a URL with invalid signature
	path := "/buckets/test-bucket/test-file.pdf"
	expires := fmt.Sprintf("%d", time.Now().Add(1*time.Hour).Unix())
	invalidSignedURL := fmt.Sprintf("%s?expires=%s&signature=invalid-signature", path, expires)

	// Create a request with the invalid signed URL
	req := httptest.NewRequest(http.MethodGet, invalidSignedURL, nil)
	w := httptest.NewRecorder()

	// Execute the middleware
	middleware.ServeHTTP(w, req)

	// Check that the request was forbidden
	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestSignatureValidationMiddleware_MissingSignature(t *testing.T) {
	// Create a test signer
	signer := urlsigner.NewURLSigner("test-secret")
	
	// Create a test logger
	logger, _ := zap.NewDevelopment()
	sugaredLogger := logger.Sugar()

	// Create a simple handler that should not be reached for PDF files without signature
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called for PDF without signature")
		w.WriteHeader(http.StatusOK)
	})

	// Wrap the handler with the signature validation middleware
	middleware := SignatureValidationMiddleware(nextHandler, signer, sugaredLogger)

	// Create a request without signature for a PDF file
	path := "/buckets/test-bucket/test-file.pdf"
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()

	// Execute the middleware
	middleware.ServeHTTP(w, req)

	// Check that the request was forbidden
	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestSignatureValidationMiddleware_MissingSignatureNonPDF(t *testing.T) {
	// Create a test signer
	signer := urlsigner.NewURLSigner("test-secret")
	
	// Create a test logger
	logger, _ := zap.NewDevelopment()
	sugaredLogger := logger.Sugar()

	// Create a simple handler that returns 200 OK
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap the handler with the signature validation middleware
	middleware := SignatureValidationMiddleware(nextHandler, signer, sugaredLogger)

	// Create a request without signature for a non-PDF file
	path := "/buckets/test-bucket/test-file.txt"
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()

	// Execute the middleware
	middleware.ServeHTTP(w, req)

	// Check that the request was successful (no signature required for non-PDF files)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for non-PDF file, got %d", w.Code)
	}
}

func TestSignatureValidationMiddleware_MetadataOnlyRequest(t *testing.T) {
	// Create a test signer
	signer := urlsigner.NewURLSigner("test-secret")
	
	// Create a test logger
	logger, _ := zap.NewDevelopment()
	sugaredLogger := logger.Sugar()

	// Create a simple handler that returns 200 OK
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap the handler with the signature validation middleware
	middleware := SignatureValidationMiddleware(nextHandler, signer, sugaredLogger)

	// Create a metadata-only request (no signature required)
	path := "/buckets/test-bucket/test-file.pdf?metadataOnly=true"
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()

	// Execute the middleware
	middleware.ServeHTTP(w, req)

	// Check that the request was successful (no signature required for metadata-only)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for metadata-only request, got %d", w.Code)
	}
}

func TestSignatureValidationMiddleware_NonGetRequest(t *testing.T) {
	// Create a test signer
	signer := urlsigner.NewURLSigner("test-secret")
	
	// Create a test logger
	logger, _ := zap.NewDevelopment()
	sugaredLogger := logger.Sugar()

	// Create a simple handler that returns 200 OK
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap the handler with the signature validation middleware
	middleware := SignatureValidationMiddleware(nextHandler, signer, sugaredLogger)

	// Create a PUT request (signature validation only applies to GET)
	path := "/buckets/test-bucket/test-file.pdf"
	req := httptest.NewRequest(http.MethodPut, path, nil)
	w := httptest.NewRecorder()

	// Execute the middleware
	middleware.ServeHTTP(w, req)

	// Check that the request was successful (no signature required for non-GET)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for PUT request, got %d", w.Code)
	}
}

func TestSignatureValidationMiddleware_NonObjectEndpoint(t *testing.T) {
	// Create a test signer
	signer := urlsigner.NewURLSigner("test-secret")
	
	// Create a test logger
	logger, _ := zap.NewDevelopment()
	sugaredLogger := logger.Sugar()

	// Create a simple handler that returns 200 OK
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap the handler with the signature validation middleware
	middleware := SignatureValidationMiddleware(nextHandler, signer, sugaredLogger)

	// Create a request to a non-object endpoint (e.g., bucket listing)
	path := "/buckets/test-bucket"
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()

	// Execute the middleware
	middleware.ServeHTTP(w, req)

	// Check that the request was successful (no signature required for bucket endpoints)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for bucket endpoint, got %d", w.Code)
	}
}

func TestIsObjectEndpoint(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/buckets/test-bucket/test-file.pdf", true},
		{"/buckets/test-bucket/folder/test-file.pdf", true},
		{"/buckets/test-bucket", false},
		{"/buckets", false},
		{"/buckets/test-bucket/upload-multiple", false},
		{"/buckets/test-bucket/remove-multiple", false},
		{"/other/path", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isObjectEndpoint(tt.path)
			if result != tt.expected {
				t.Errorf("isObjectEndpoint(%s) = %v, expected %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestIsPDFFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/buckets/test-bucket/test-file.pdf", true},
		{"/buckets/test-bucket/test-file.PDF", true},
		{"/buckets/test-bucket/folder/document.pdf", true},
		{"/buckets/test-bucket/test-file.txt", false},
		{"/buckets/test-bucket/image.jpg", false},
		{"/buckets/test-bucket/archive.zip", false},
		{"/buckets/test-bucket/pdffile", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isPDFFile(tt.path)
			if result != tt.expected {
				t.Errorf("isPDFFile(%s) = %v, expected %v", tt.path, result, tt.expected)
			}
		})
	}
}
