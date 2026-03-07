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

	u, err := url.Parse(signedURL)
	if err != nil {
		t.Fatalf("Failed to parse signed URL: %v", err)
	}

	if u.Query().Get("expires") == "" {
		t.Error("Signed URL should contain expires parameter")
	}

	if u.Query().Get("signature") == "" {
		t.Error("Signed URL should contain signature parameter")
	}

	if u.Path != path {
		t.Errorf("Signed URL should have path %s, got %s", path, u.Path)
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

	u, err := url.Parse(signedURL)
	if err != nil {
		t.Fatalf("Failed to parse signed URL: %v", err)
	}

	expiresStr := u.Query().Get("expires")
	signature := u.Query().Get("signature")

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

	u, err := url.Parse(signedURL)
	if err != nil {
		t.Fatalf("Failed to parse signed URL: %v", err)
	}

	expiresStr := u.Query().Get("expires")
	signature := u.Query().Get("signature")

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

	u, err := url.Parse(signedURL)
	if err != nil {
		t.Fatalf("Failed to parse signed URL: %v", err)
	}

	expiresStr := u.Query().Get("expires")
	invalidSignature := "invalid-signature"

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

	u, err := url.Parse(signedURL)
	if err != nil {
		t.Fatalf("Failed to parse signed URL: %v", err)
	}

	expiresStr := u.Query().Get("expires")
	signature := u.Query().Get("signature")

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

	signedURL1, err := signer1.SignURL(path, ttl)
	if err != nil {
		t.Fatalf("signer1.SignURL failed: %v", err)
	}
	signedURL2, err := signer2.SignURL(path, ttl)
	if err != nil {
		t.Fatalf("signer2.SignURL failed: %v", err)
	}

	u1, err := url.Parse(signedURL1)
	if err != nil {
		t.Fatalf("failed to parse signedURL1: %v", err)
	}
	u2, err := url.Parse(signedURL2)
	if err != nil {
		t.Fatalf("failed to parse signedURL2: %v", err)
	}

	sig1 := u1.Query().Get("signature")
	sig2 := u2.Query().Get("signature")
	if sig1 == "" || sig2 == "" {
		t.Fatal("failed to extract signatures from signed URLs")
	}

	if sig1 == sig2 {
		t.Error("Different secrets should produce different signatures")
	}
}
