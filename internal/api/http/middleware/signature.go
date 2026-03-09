package middleware

import (
	"net/http"
	"strings"

	"github.com/mini-maxit/file-storage/pkg/urlsigner"
	"go.uber.org/zap"
)

// SignatureValidationMiddleware validates signed URLs for GET requests to object endpoints
// and enforces that non-GET (write) operations carry the internal API key.
func SignatureValidationMiddleware(next http.Handler, signer *urlsigner.URLSigner, internalAPIKey string, log *zap.SugaredLogger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && isObjectEndpoint(r.URL.Path) {
			// Internal services bypass signature enforcement entirely.
			if internalAPIKey != "" && r.Header.Get("X-Internal-Key") == internalAPIKey {
				next.ServeHTTP(w, r)
				return
			}

			query := r.URL.Query()

			// Metadata-only requests are internal-only; require the internal key.
			if strings.ToLower(query.Get("metadataOnly")) == "true" {
				if internalAPIKey == "" || r.Header.Get("X-Internal-Key") != internalAPIKey {
					log.Warnf("Unauthorized metadata access to %s", r.URL.Path)
					http.Error(w, "Forbidden", http.StatusForbidden)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			expires := query.Get("expires")
			signature := query.Get("signature")

			if expires != "" || signature != "" {
				err := signer.ValidateSignedURL(r.URL.Path, expires, signature)
				if err != nil {
					log.Warnf("Signature validation failed for %s: %v", r.URL.Path, err)
					http.Error(w, "Forbidden: "+err.Error(), http.StatusForbidden)
					return
				}
				log.Debugf("Signature validated successfully for %s", r.URL.Path)
			} else {
				log.Warnf("Missing signature for file access: %s", r.URL.Path)
				http.Error(w, "Forbidden: signature required for file access", http.StatusForbidden)
				return
			}
		} else if r.Method == http.MethodGet && isBucketEndpoint(r.URL.Path) {
			// Bucket listing/info endpoints are internal-only.
			if internalAPIKey == "" || r.Header.Get("X-Internal-Key") != internalAPIKey {
				log.Warnf("Unauthorized access to bucket endpoint %s", r.URL.Path)
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
		} else if r.Method != http.MethodGet {
			// All write operations require the internal API key.
			if internalAPIKey == "" || r.Header.Get("X-Internal-Key") != internalAPIKey {
				log.Warnf("Unauthorized write attempt to %s %s", r.Method, r.URL.Path)
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// isBucketEndpoint checks if the path targets the /buckets namespace (listing, info).
func isBucketEndpoint(path string) bool {
	return strings.HasPrefix(path, "/buckets")
}

// isObjectEndpoint checks if the path is an object endpoint.
// Pattern: /buckets/{bucketName}/{objectKey}
func isObjectEndpoint(path string) bool {
	path = strings.TrimPrefix(path, "/")
	parts := strings.SplitN(path, "/", 3)

	// Must have at least 3 parts: buckets, bucketName, objectKey
	// and the first part must be "buckets"
	if len(parts) >= 3 && parts[0] == "buckets" {
		// Exclude special write endpoints; those are handled by write protection above.
		if parts[2] != "upload-multiple" && parts[2] != "remove-multiple" {
			return true
		}
	}

	return false
}
