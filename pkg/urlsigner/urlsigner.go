package urlsigner

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// URLSigner handles signing and validating URLs with HMAC signatures
type URLSigner struct {
	secret []byte
}

// NewURLSigner creates a new URLSigner with the given secret
func NewURLSigner(secret string) *URLSigner {
	return &URLSigner{
		secret: []byte(secret),
	}
}

// SignURL generates a signed URL with expiration
// path: the request path (e.g., "/buckets/mybucket/myfile.pdf")
// ttl: time-to-live duration for the URL
func (s *URLSigner) SignURL(path string, ttl time.Duration) (string, error) {
	if path == "" {
		return "", errors.New("path cannot be empty")
	}

	// Calculate expiration timestamp
	expiresAt := time.Now().Add(ttl).Unix()

	// Create the string to sign: "path:expiration"
	stringToSign := fmt.Sprintf("%s:%d", path, expiresAt)

	// Generate HMAC signature
	signature := s.generateSignature(stringToSign)

	// Build query parameters
	values := url.Values{}
	values.Set("expires", strconv.FormatInt(expiresAt, 10))
	values.Set("signature", signature)

	// Return path with query parameters
	if len(values) > 0 {
		return fmt.Sprintf("%s?%s", path, values.Encode()), nil
	}
	return path, nil
}

// ValidateSignedURL validates a signed URL
// path: the request path without query parameters
// expiresStr: the expires query parameter value
// signature: the signature query parameter value
func (s *URLSigner) ValidateSignedURL(path string, expiresStr string, signature string) error {
	if path == "" {
		return errors.New("path cannot be empty")
	}

	if expiresStr == "" {
		return errors.New("missing expires parameter")
	}

	if signature == "" {
		return errors.New("missing signature parameter")
	}

	// Parse expiration timestamp
	expiresAt, err := strconv.ParseInt(expiresStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid expires parameter: %w", err)
	}

	// Check if URL has expired
	if time.Now().Unix() > expiresAt {
		return errors.New("URL has expired")
	}

	// Recreate the string that was signed
	stringToSign := fmt.Sprintf("%s:%d", path, expiresAt)

	// Generate expected signature
	expectedSignature := s.generateSignature(stringToSign)

	// Compare signatures using constant-time comparison to prevent timing attacks
	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return errors.New("invalid signature")
	}

	return nil
}

// generateSignature creates an HMAC-SHA256 signature and returns it as base64 URL-encoded string
func (s *URLSigner) generateSignature(data string) string {
	h := hmac.New(sha256.New, s.secret)
	h.Write([]byte(data))
	return base64.URLEncoding.EncodeToString(h.Sum(nil))
}
