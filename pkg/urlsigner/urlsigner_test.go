package urlsigner

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestSignURL(t *testing.T) {
	signer := NewURLSigner("test-secret")
	path := "/buckets/test-bucket/test-file.pdf"
	ttl := 1 * time.Hour

	signedURL, err := signer.SignURL(path, ttl)
	if err != nil {
		t.Fatalf("Failed to sign URL: %v", err)
	}

	if signedURL == "" {
		t.Fatal("Signed URL should not be empty")
	}

	// Verify the signed URL contains expires and signature parameters
	if !strings.Contains(signedURL, "expires=") {
		t.Error("Signed URL should contain expires parameter")
	}

	if !strings.Contains(signedURL, "signature=") {
		t.Error("Signed URL should contain signature parameter")
	}

	if !strings.HasPrefix(signedURL, path) {
		t.Errorf("Signed URL should start with original path %s, got %s", path, signedURL)
	}
}

func TestSignURLEmptyPath(t *testing.T) {
	signer := NewURLSigner("test-secret")
	_, err := signer.SignURL("", 1*time.Hour)
	if err == nil {
		t.Error("Expected error for empty path")
	}
}

func TestValidateSignedURL_Valid(t *testing.T) {
	signer := NewURLSigner("test-secret")
	path := "/buckets/test-bucket/test-file.pdf"
	ttl := 1 * time.Hour

	signedURL, err := signer.SignURL(path, ttl)
	if err != nil {
		t.Fatalf("Failed to sign URL: %v", err)
	}

	// Parse the signed URL to extract query parameters
	parts := strings.Split(signedURL, "?")
	if len(parts) != 2 {
		t.Fatalf("Invalid signed URL format: %s", signedURL)
	}

	// Parse query parameters properly
	params, err := url.ParseQuery(parts[1])
	if err != nil {
		t.Fatalf("Failed to parse query parameters: %v", err)
	}

	expiresStr := params.Get("expires")
	signature := params.Get("signature")

	// Validate the signed URL
	err = signer.ValidateSignedURL(path, expiresStr, signature)
	if err != nil {
		t.Errorf("Failed to validate valid signed URL: %v", err)
	}
}

func TestValidateSignedURL_Expired(t *testing.T) {
	signer := NewURLSigner("test-secret")
	path := "/buckets/test-bucket/test-file.pdf"
	
	// Create a URL that expired 1 hour ago
	ttl := -1 * time.Hour

	signedURL, err := signer.SignURL(path, ttl)
	if err != nil {
		t.Fatalf("Failed to sign URL: %v", err)
	}

	// Parse the signed URL to extract query parameters
	parts := strings.Split(signedURL, "?")
	if len(parts) != 2 {
		t.Fatalf("Invalid signed URL format: %s", signedURL)
	}

	// Parse query parameters properly
	params, err := url.ParseQuery(parts[1])
	if err != nil {
		t.Fatalf("Failed to parse query parameters: %v", err)
	}

	expiresStr := params.Get("expires")
	signature := params.Get("signature")

	// Validate the signed URL - should fail due to expiration
	err = signer.ValidateSignedURL(path, expiresStr, signature)
	if err == nil {
		t.Error("Expected error for expired URL")
	}

	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("Expected 'expired' error, got: %v", err)
	}
}

func TestValidateSignedURL_InvalidSignature(t *testing.T) {
	signer := NewURLSigner("test-secret")
	path := "/buckets/test-bucket/test-file.pdf"
	ttl := 1 * time.Hour

	signedURL, err := signer.SignURL(path, ttl)
	if err != nil {
		t.Fatalf("Failed to sign URL: %v", err)
	}

	// Parse the signed URL to extract query parameters
	parts := strings.Split(signedURL, "?")
	if len(parts) != 2 {
		t.Fatalf("Invalid signed URL format: %s", signedURL)
	}

	// Parse query parameters properly
	params, err := url.ParseQuery(parts[1])
	if err != nil {
		t.Fatalf("Failed to parse query parameters: %v", err)
	}

	expiresStr := params.Get("expires")
	invalidSignature := "invalid-signature"

	// Validate with invalid signature
	err = signer.ValidateSignedURL(path, expiresStr, invalidSignature)
	if err == nil {
		t.Error("Expected error for invalid signature")
	}

	if !strings.Contains(err.Error(), "signature") {
		t.Errorf("Expected 'signature' error, got: %v", err)
	}
}

func TestValidateSignedURL_MissingExpires(t *testing.T) {
	signer := NewURLSigner("test-secret")
	path := "/buckets/test-bucket/test-file.pdf"

	err := signer.ValidateSignedURL(path, "", "some-signature")
	if err == nil {
		t.Error("Expected error for missing expires parameter")
	}
}

func TestValidateSignedURL_MissingSignature(t *testing.T) {
	signer := NewURLSigner("test-secret")
	path := "/buckets/test-bucket/test-file.pdf"

	err := signer.ValidateSignedURL(path, "1234567890", "")
	if err == nil {
		t.Error("Expected error for missing signature parameter")
	}
}

func TestValidateSignedURL_TamperedPath(t *testing.T) {
	signer := NewURLSigner("test-secret")
	path := "/buckets/test-bucket/test-file.pdf"
	ttl := 1 * time.Hour

	signedURL, err := signer.SignURL(path, ttl)
	if err != nil {
		t.Fatalf("Failed to sign URL: %v", err)
	}

	// Parse the signed URL to extract query parameters
	parts := strings.Split(signedURL, "?")
	if len(parts) != 2 {
		t.Fatalf("Invalid signed URL format: %s", signedURL)
	}

	// Parse query parameters properly
	params, err := url.ParseQuery(parts[1])
	if err != nil {
		t.Fatalf("Failed to parse query parameters: %v", err)
	}

	expiresStr := params.Get("expires")
	signature := params.Get("signature")

	// Validate with a different path (tampered)
	tamperedPath := "/buckets/test-bucket/different-file.pdf"
	err = signer.ValidateSignedURL(tamperedPath, expiresStr, signature)
	if err == nil {
		t.Error("Expected error for tampered path")
	}

	if !strings.Contains(err.Error(), "signature") {
		t.Errorf("Expected 'signature' error, got: %v", err)
	}
}

func TestDifferentSecretsProduceDifferentSignatures(t *testing.T) {
	path := "/buckets/test-bucket/test-file.pdf"
	ttl := 1 * time.Hour

	signer1 := NewURLSigner("secret1")
	signer2 := NewURLSigner("secret2")

	signedURL1, _ := signer1.SignURL(path, ttl)
	signedURL2, _ := signer2.SignURL(path, ttl)

	// Extract signatures
	parts1 := strings.Split(signedURL1, "signature=")
	parts2 := strings.Split(signedURL2, "signature=")

	if len(parts1) < 2 || len(parts2) < 2 {
		t.Fatal("Failed to extract signatures")
	}

	sig1 := parts1[1]
	sig2 := parts2[1]

	if sig1 == sig2 {
		t.Error("Different secrets should produce different signatures")
	}
}
