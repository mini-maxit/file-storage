package server

import (
	"encoding/json"
	"github.com/mini-maxit/file-storage/internal/entities"
	"net/http"
	"strings"
	"time"

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
func listBucketsHandler(fs *services.FileService, w http.ResponseWriter, r *http.Request) {
	// Get all buckets from the FileService
	buckets := fs.GetAllBuckets()

	// Write the response as JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(buckets); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// createBucketHandler -> POST /buckets
func createBucketHandler(fs *services.FileService, w http.ResponseWriter, r *http.Request) {
	// Parse the request body to get the bucket name
	var request struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Name == "" {
		http.Error(w, "Invalid request body. 'name' is required.", http.StatusBadRequest)
		return
	}
	bucketName := request.Name

	// Check if the bucket already exists
	if _, err := fs.GetBucket(bucketName); err == nil {
		http.Error(w, "Bucket already exists.", http.StatusConflict)
		return
	}

	// Create the new bucket
	newBucket := entities.Bucket{
		Name:         bucketName,
		CreationDate: time.Now(),
	}

	if err := fs.CreateBucket(newBucket); err != nil {
		http.Error(w, "Failed to create bucket.", http.StatusInternalServerError)
		return
	}

	// Respond with success
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(newBucket)
}

// getBucketHandler -> GET /buckets/{bucketName}
func getBucketHandler(fs *services.FileService, w http.ResponseWriter, r *http.Request, bucketName string) {
	// Retrieve the bucket by name
	bucket, err := fs.GetBucket(bucketName)
	if err != nil {
		http.Error(w, "Bucket not found", http.StatusNotFound)
		return
	}

	// Parse query parameters
	query := r.URL.Query()
	listObjects := query.Get("listObjects") == "true"
	prefix := query.Get("prefix")

	if listObjects {
		// Filter objects by prefix, if provided
		filteredObjects := filterObjects(bucket.Objects, prefix)

		// Prepare the response
		response := struct {
			Name            string            `json:"name"`
			CreationDate    time.Time         `json:"creationDate"`
			NumberOfObjects int               `json:"numberOfObjects"`
			Size            int               `json:"size"`
			Objects         []entities.Object `json:"objects"`
		}{
			Name:            bucket.Name,
			CreationDate:    bucket.CreationDate,
			NumberOfObjects: len(filteredObjects),
			Size:            bucket.Size,
			Objects:         filteredObjects,
		}

		// Write the response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	} else {
		// Prepare a partial response excluding the Objects field
		partialBucket := struct {
			Name            string    `json:"name"`
			CreationDate    time.Time `json:"creationDate"`
			NumberOfObjects int       `json:"numberOfObjects"`
			Size            int       `json:"size"`
		}{
			Name:            bucket.Name,
			CreationDate:    bucket.CreationDate,
			NumberOfObjects: bucket.NumberOfObjects,
			Size:            bucket.Size,
		}

		// Write the response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(partialBucket)
	}
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
func NewServer(fs *services.FileService) *Server {
	mux := http.NewServeMux()

	// /buckets
	mux.HandleFunc("/buckets", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			listBucketsHandler(fs, w, r)
		case http.MethodPost:
			createBucketHandler(fs, w, r)
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
				getBucketHandler(fs, w, r, bucketName)
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

// filterObjects filters objects based on the prefix
func filterObjects(objects map[string]entities.Object, prefix string) []entities.Object {
	var filtered []entities.Object

	for _, obj := range objects {
		if prefix == "" || strings.HasPrefix(obj.Key, prefix) {
			filtered = append(filtered, obj)
		}
	}

	return filtered
}
