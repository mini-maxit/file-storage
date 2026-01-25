package middleware

import (
	"net/http"
	"strings"

	"github.com/mini-maxit/file-storage/pkg/urlsigner"
	"go.uber.org/zap"
)

// SignatureValidationMiddleware validates signed URLs for GET requests to object endpoints
func SignatureValidationMiddleware(next http.Handler, signer *urlsigner.URLSigner, log *zap.SugaredLogger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only validate GET requests to object endpoints (not metadata-only requests)
		// Pattern: /buckets/{bucketName}/{objectKey}
		if r.Method == http.MethodGet && isObjectEndpoint(r.URL.Path) {
			query := r.URL.Query()
			
			// Check if this is a metadata-only request (no signature required)
			metadataOnly := strings.ToLower(query.Get("metadataOnly")) == "true"
			
			// Check if signature parameters are present
			expires := query.Get("expires")
			signature := query.Get("signature")
			
			// If either expires or signature is present, validate both
			if expires != "" || signature != "" {
				// Validate the signature
				err := signer.ValidateSignedURL(r.URL.Path, expires, signature)
				if err != nil {
					log.Warnf("Signature validation failed for %s: %v", r.URL.Path, err)
					http.Error(w, "Forbidden: "+err.Error(), http.StatusForbidden)
					return
				}
				log.Debugf("Signature validated successfully for %s", r.URL.Path)
			} else if !metadataOnly {
				// If no signature is provided and it's not metadata-only, require signature
				log.Warnf("Missing signature for file access: %s", r.URL.Path)
				http.Error(w, "Forbidden: signature required for file access", http.StatusForbidden)
				return
			}
		}

		// Continue with the next handler
		next.ServeHTTP(w, r)
	})
}

// isObjectEndpoint checks if the path is an object endpoint
// Pattern: /buckets/{bucketName}/{objectKey}
func isObjectEndpoint(path string) bool {
	// Remove leading slash
	path = strings.TrimPrefix(path, "/")
	
	// Split by /
	parts := strings.SplitN(path, "/", 3)
	
	// Must have at least 3 parts: buckets, bucketName, objectKey
	// and the first part must be "buckets"
	if len(parts) >= 3 && parts[0] == "buckets" {
		// Check that it's not a special endpoint like upload-multiple or remove-multiple
		if parts[2] != "upload-multiple" && parts[2] != "remove-multiple" {
			return true
		}
	}
	
	return false
}
