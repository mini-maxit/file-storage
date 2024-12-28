package server

import (
	"net/http"
	"strings"

	"github.com/mini-maxit/file-storage/internal/api/services"
	"github.com/sirupsen/logrus"
)

type Server struct {
	mux http.Handler
}

func (s *Server) Run(addr string) error {
	logrus.Infof("Server is running on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

// -----
// Handler functions
// -----

// listBucketsHandler -> GET /buckets
func listBucketsHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: implement logic (list all buckets)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("GET /buckets\n"))
}

// createBucketHandler -> POST /buckets
func createBucketHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: implement logic (create a new bucket)
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write([]byte("POST /buckets\n"))
}

// getBucketHandler -> GET /buckets/{bucketName}
func getBucketHandler(w http.ResponseWriter, r *http.Request, bucketName string) {
	// TODO: implement logic (get bucket info or list objects)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("GET /buckets/" + bucketName + "\n"))
}

// deleteBucketHandler -> DELETE /buckets/{bucketName}
func deleteBucketHandler(w http.ResponseWriter, r *http.Request, bucketName string) {
	// TODO: implement logic (delete the specified bucket)
	// 204 is the correct success status for a delete with no content
	w.WriteHeader(http.StatusNoContent)
}

// uploadMultipleHandler -> POST /buckets/{bucketName}/upload-multiple
func uploadMultipleHandler(w http.ResponseWriter, r *http.Request, bucketName string) {
	// TODO: implement logic (upload multiple files under the same prefix)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("POST /buckets/" + bucketName + "/upload-multiple\n"))
}

// getObjectHandler -> GET /buckets/{bucketName}/{objectKey}
func getObjectHandler(w http.ResponseWriter, r *http.Request, bucketName, objectKey string) {
	// TODO: implement logic (download or get metadata about an object)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("GET /buckets/" + bucketName + "/" + objectKey + "\n"))
}

// putObjectHandler -> PUT /buckets/{bucketName}/{objectKey}
func putObjectHandler(w http.ResponseWriter, r *http.Request, bucketName, objectKey string) {
	// TODO: implement logic (upload or update an object)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("PUT /buckets/" + bucketName + "/" + objectKey + "\n"))
}

// deleteObjectHandler -> DELETE /buckets/{bucketName}/{objectKey}
func deleteObjectHandler(w http.ResponseWriter, r *http.Request, bucketName, objectKey string) {
	// TODO: implement logic (delete an object from the bucket)
	w.WriteHeader(http.StatusNoContent)
}

// -----
// End of handler functions
// -----

// NewServer sets up the routes and returns the Server object
func NewServer(ts *services.TaskService) *Server {
	mux := http.NewServeMux()

	// /buckets
	mux.HandleFunc("/buckets", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			listBucketsHandler(w, r)
		case http.MethodPost:
			createBucketHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// /buckets/*
	mux.HandleFunc("/buckets/", func(w http.ResponseWriter, r *http.Request) {
		// Possible routes:
		//   GET /buckets/{bucketName}
		//   DELETE /buckets/{bucketName}
		//   POST /buckets/{bucketName}/upload-multiple
		//   GET/PUT/DELETE /buckets/{bucketName}/{objectKey}
		path := strings.TrimPrefix(r.URL.Path, "/buckets/")
		if path == "" {
			http.Error(w, "Bucket name is required", http.StatusBadRequest)
			return
		}

		// Check if path includes "/upload-multiple" or another slash
		parts := strings.SplitN(path, "/", 2)
		bucketName := parts[0]

		if len(parts) == 1 {
			// "/buckets/{bucketName}" (no second slash)
			switch r.Method {
			case http.MethodGet:
				getBucketHandler(w, r, bucketName)
			case http.MethodDelete:
				deleteBucketHandler(w, r, bucketName)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		secondPart := parts[1]

		if secondPart == "upload-multiple" && r.Method == http.MethodPost {
			uploadMultipleHandler(w, r, bucketName)
			return
		}

		objectKey := secondPart
		switch r.Method {
		case http.MethodGet:
			getObjectHandler(w, r, bucketName, objectKey)
		case http.MethodPut:
			putObjectHandler(w, r, bucketName, objectKey)
		case http.MethodDelete:
			deleteObjectHandler(w, r, bucketName, objectKey)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	return &Server{mux: mux}
}
