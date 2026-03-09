package middleware

import (
	"net/http"
	"strings"

	"github.com/mini-maxit/file-storage/pkg/urlsigner"
	"go.uber.org/zap"
)

// SignatureValidationMiddleware validates signed URLs for GET requests to object endpoints.
// All write operations are denied (should use internal server).
func SignatureValidationMiddleware(next http.Handler, signer *urlsigner.URLSigner, log *zap.SugaredLogger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && isObjectEndpoint(r.URL.Path) {
			query := r.URL.Query()

			// Metadata-only requests are internal-only.
			if strings.ToLower(query.Get("metadataOnly")) == "true" {
				log.Warnf("Unauthorized metadata-only access to %s", r.URL.Path)
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			expires := query.Get("expires")
			signature := query.Get("signature")

			if expires != "" && signature != "" {
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
			// Bucket listing/info endpoints require signature.
			log.Warnf("Unauthorized access to bucket endpoint %s", r.URL.Path)
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		} else if r.Method != http.MethodGet {
			// All write operations are denied on public server.
			log.Warnf("Write operation %s on public server: %s", r.Method, r.URL.Path)
			http.Error(w, "Method not allowed. Use internal server for write operations.", http.StatusMethodNotAllowed)
			return
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
