package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/mini-maxit/file-storage/internal/api/http/middleware"
	"github.com/mini-maxit/file-storage/internal/api/services"
	"github.com/mini-maxit/file-storage/internal/entities"
	"github.com/mini-maxit/file-storage/internal/logger"
	"go.uber.org/zap"
	"mime/multipart"
)

// Server represents our HTTP server.
type Server struct {
	mux    http.Handler
	logger *zap.SugaredLogger
}

// Run starts the HTTP server.
func (s *Server) Run(addr string) error {
	s.logger.Infof("Server is running on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

// -----
// Handler functions
// -----

// listBucketsHandler -> GET /buckets
func listBucketsHandler(fs *services.FileService, w http.ResponseWriter, log *zap.SugaredLogger) {
	log.Info("Listing all buckets")
	buckets := fs.GetAllBuckets()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(buckets); err != nil {
		log.Errorf("Failed to encode buckets response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// createBucketHandler -> POST /buckets
func createBucketHandler(fs *services.FileService, w http.ResponseWriter, r *http.Request, log *zap.SugaredLogger) {
	var request struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Name == "" {
		log.Warn("Invalid request body for bucket creation")
		http.Error(w, "Invalid request body. 'name' is required.", http.StatusBadRequest)
		return
	}
	bucketName := request.Name

	// Check if the bucket already exists
	if _, err := fs.GetBucket(bucketName); err == nil {
		log.Warnf("Bucket %s already exists", bucketName)
		http.Error(w, "Bucket already exists.", http.StatusConflict)
		return
	}

	newBucket := entities.Bucket{
		Name:         bucketName,
		CreationDate: time.Now(),
	}

	if err := fs.CreateBucket(newBucket); err != nil {
		log.Errorf("Failed to create bucket %s: %v", bucketName, err)
		http.Error(w, "Failed to create bucket.", http.StatusInternalServerError)
		return
	}

	log.Infof("Bucket %s created successfully", bucketName)
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(newBucket); err != nil {
		log.Errorf("Failed to encode new bucket response: %v", err)
	}
}

// getBucketHandler -> GET /buckets/{bucketName}
func getBucketHandler(fs *services.FileService, w http.ResponseWriter, r *http.Request, bucketName string, log *zap.SugaredLogger) {
	log.Infof("Getting bucket info for %s", bucketName)
	bucket, err := fs.GetBucket(bucketName)
	if err != nil {
		log.Warnf("Bucket %s not found", bucketName)
		http.Error(w, "Bucket not found", http.StatusNotFound)
		return
	}

	query := r.URL.Query()
	listObjects := query.Get("listObjects") == "true"
	prefix := query.Get("prefix")

	if listObjects {
		filteredObjects := filterObjects(bucket.Objects, prefix)
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

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Errorf("Failed to encode bucket objects response: %v", err)
		}
	} else {
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

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(partialBucket); err != nil {
			log.Errorf("Failed to encode bucket response: %v", err)
		}
	}
}

// deleteBucketHandler -> DELETE /buckets/{bucketName}
func deleteBucketHandler(fs *services.FileService, w http.ResponseWriter, bucketName string, log *zap.SugaredLogger) {
	log.Infof("Deleting bucket %s", bucketName)
	bucket, err := fs.GetBucket(bucketName)
	if err != nil {
		log.Warnf("Bucket %s not found", bucketName)
		http.Error(w, "Bucket not found", http.StatusNotFound)
		return
	}

	if bucket.NumberOfObjects > 0 {
		log.Warnf("Bucket %s is not empty", bucketName)
		http.Error(w, "Bucket is not empty", http.StatusBadRequest)
		return
	}

	if err := fs.DeleteBucket(bucketName); err != nil {
		log.Errorf("Failed to delete bucket %s: %v", bucketName, err)
		http.Error(w, "Failed to delete bucket: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Infof("Bucket %s deleted successfully", bucketName)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("Bucket deleted successfully"))
}

// uploadMultipleHandler -> POST /buckets/{bucketName}/upload-multiple?prefix=<prefix>
func uploadMultipleHandler(fs *services.FileService, w http.ResponseWriter, r *http.Request, bucketName string, log *zap.SugaredLogger) {
	log.Infof("Uploading multiple files to bucket %s", bucketName)
	prefix := r.URL.Query().Get("prefix")
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix = prefix + "/"
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		log.Errorf("Failed to parse multipart form for bucket %s: %v", bucketName, err)
		http.Error(w, "Failed to parse multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}

	fileHeaders := r.MultipartForm.File["files"]
	if len(fileHeaders) == 0 {
		log.Warn("No files provided for upload")
		http.Error(w, "No files provided", http.StatusBadRequest)
		return
	}

	uploadedFiles := make([]string, 0, len(fileHeaders))
	for _, fh := range fileHeaders {
		file, err := fh.Open()
		if err != nil {
			log.Errorf("Failed to open file %s: %v", fh.Filename, err)
			http.Error(w, "Failed to open uploaded file: "+err.Error(), http.StatusInternalServerError)
			return
		}

		func(f multipart.File, filename string) {
			defer func(f multipart.File) {
				if err := f.Close(); err != nil {
					log.Errorf("Failed to close file %s: %v", filename, err)
				}
			}(f)
			objectKey := prefix + filename
			if err := fs.AddOrUpdateObject(bucketName, objectKey, f); err != nil {
				log.Errorf("Failed to upload file %s to bucket %s: %v", filename, bucketName, err)
				http.Error(w, "Failed to upload file "+filename+": "+err.Error(), http.StatusInternalServerError)
				return
			}
			log.Infof("Uploaded file %s as object %s", filename, objectKey)
			uploadedFiles = append(uploadedFiles, objectKey)
		}(file, fh.Filename)
	}

	response := struct {
		Message string   `json:"message"`
		Files   []string `json:"files"`
	}{
		Message: "Files uploaded successfully",
		Files:   uploadedFiles,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Errorf("Failed to encode upload response: %v", err)
	}
}

// removeMultipleHandler -> DELETE /buckets/{bucketName}/remove-multiple?prefix=<prefix>
func removeMultipleHandler(fs *services.FileService, w http.ResponseWriter, r *http.Request, bucketName string, log *zap.SugaredLogger) {
	log.Infof("Removing multiple objects from bucket %s", bucketName)
	prefix := r.URL.Query().Get("prefix")
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix = prefix + "/"
	}

	removedObjects, err := fs.RemoveObjects(bucketName, prefix)
	if err != nil {
		log.Errorf("Failed to remove objects with prefix '%s' from bucket %s: %v", prefix, bucketName, err)
		http.Error(w, "Failed to remove objects: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := struct {
		Message        string            `json:"message"`
		RemovedObjects []entities.Object `json:"removedObjects"`
	}{
		Message:        "Objects removed successfully",
		RemovedObjects: removedObjects,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Errorf("Failed to encode remove objects response: %v", err)
	}
}

// getObjectHandler -> GET /buckets/{bucketName}/{objectKey}
func getObjectHandler(fs *services.FileService, w http.ResponseWriter, r *http.Request, bucketName, objectKey string, log *zap.SugaredLogger) {
	log.Infof("Getting object %s from bucket %s", objectKey, bucketName)
	query := r.URL.Query()
	metadataOnly := strings.ToLower(query.Get("metadataOnly")) == "true"

	obj, err := fs.GetObject(bucketName, objectKey)
	if err != nil {
		log.Warnf("Object %s not found in bucket %s", objectKey, bucketName)
		http.Error(w, "Object not found", http.StatusNotFound)
		return
	}

	if metadataOnly {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(obj); err != nil {
			log.Errorf("Failed to encode object metadata: %v", err)
		}
		return
	}

	objectPath, err := fs.GetObjectFilePath(bucketName, objectKey)
	if err != nil {
		log.Warnf("Object %s not found on disk in bucket %s", objectKey, bucketName)
		http.Error(w, "Object not found on disk", http.StatusNotFound)
		return
	}

	log.Infof("Serving file %s from bucket %s", objectKey, bucketName)
	http.ServeFile(w, r, objectPath)
}

// putObjectHandler -> PUT /buckets/{bucketName}/{objectKey}
func putObjectHandler(fs *services.FileService, w http.ResponseWriter, r *http.Request, bucketName, objectKey string, log *zap.SugaredLogger) {
	log.Infof("Uploading/updating object %s in bucket %s", objectKey, bucketName)
	_, err := fs.GetBucket(bucketName)
	if err != nil {
		log.Warnf("Bucket %s not found", bucketName)
		http.Error(w, "Bucket not found", http.StatusNotFound)
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		log.Errorf("Failed to parse form data for object %s: %v", objectKey, err)
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		log.Warn("File not provided in form data")
		http.Error(w, "File not provided", http.StatusBadRequest)
		return
	}
	defer func(file multipart.File) {
		if err := file.Close(); err != nil {
			log.Errorf("Failed to close file for object %s: %v", objectKey, err)
		}
	}(file)

	if err := fs.AddOrUpdateObject(bucketName, objectKey, file); err != nil {
		log.Errorf("Failed to update object %s in bucket %s: %v", objectKey, bucketName, err)
		http.Error(w, "Failed to update bucket metadata", http.StatusInternalServerError)
		return
	}

	log.Infof("Object %s in bucket %s uploaded/updated successfully", objectKey, bucketName)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("Object uploaded or updated successfully"))
}

// deleteObjectHandler -> DELETE /buckets/{bucketName}/{objectKey}
func deleteObjectHandler(fs *services.FileService, w http.ResponseWriter, bucketName, objectKey string, log *zap.SugaredLogger) {
	log.Infof("Deleting object %s from bucket %s", objectKey, bucketName)
	_, err := fs.GetBucket(bucketName)
	if err != nil {
		log.Warnf("Bucket %s not found", bucketName)
		http.Error(w, "Bucket not found", http.StatusNotFound)
		return
	}

	_, err = fs.GetObject(bucketName, objectKey)
	if err != nil {
		log.Warnf("Object %s not found in bucket %s", objectKey, bucketName)
		http.Error(w, "Object not found", http.StatusNotFound)
		return
	}

	if err := fs.RemoveObject(bucketName, objectKey); err != nil {
		log.Errorf("Failed to delete object %s from bucket %s: %v", objectKey, bucketName, err)
		http.Error(w, "Failed to delete object", http.StatusInternalServerError)
		return
	}

	log.Infof("Object %s from bucket %s deleted successfully", objectKey, bucketName)
	w.WriteHeader(http.StatusNoContent)
}

// NewServer sets up the routes, wraps the mux with HTTP logging middleware,
// and returns the Server object.
func NewServer(fs *services.FileService, appLog *zap.SugaredLogger) *Server {
	// Create the base mux for our file storage API endpoints.
	mux := http.NewServeMux()

	// /buckets route
	mux.HandleFunc("/buckets", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			listBucketsHandler(fs, w, appLog)
		case http.MethodPost:
			createBucketHandler(fs, w, r, appLog)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// /buckets/* routes for bucket-specific and object-specific operations.
	mux.HandleFunc("/buckets/", func(w http.ResponseWriter, r *http.Request) {
		// Expected routes:
		//   GET /buckets/{bucketName}
		//   DELETE /buckets/{bucketName}
		//   POST /buckets/{bucketName}/upload-multiple
		//   DELETE /buckets/{bucketName}/remove-multiple?prefix=<prefix>
		//   GET/PUT/DELETE /buckets/{bucketName}/{objectKey}
		path := strings.TrimPrefix(r.URL.Path, "/buckets/")
		if path == "" {
			http.Error(w, "Bucket name is required", http.StatusBadRequest)
			return
		}

		parts := strings.SplitN(path, "/", 2)
		bucketName := parts[0]

		if len(parts) == 1 {
			switch r.Method {
			case http.MethodGet:
				getBucketHandler(fs, w, r, bucketName, appLog)
			case http.MethodDelete:
				deleteBucketHandler(fs, w, bucketName, appLog)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		secondPart := parts[1]
		if secondPart == "upload-multiple" && r.Method == http.MethodPost {
			uploadMultipleHandler(fs, w, r, bucketName, appLog)
			return
		}
		if secondPart == "remove-multiple" && r.Method == http.MethodDelete {
			removeMultipleHandler(fs, w, r, bucketName, appLog)
			return
		}

		// Otherwise treat as an object key.
		objectKey := secondPart
		switch r.Method {
		case http.MethodGet:
			getObjectHandler(fs, w, r, bucketName, objectKey, appLog)
		case http.MethodPut:
			putObjectHandler(fs, w, r, bucketName, objectKey, appLog)
		case http.MethodDelete:
			deleteObjectHandler(fs, w, bucketName, objectKey, appLog)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Retrieve an HTTP-specific logger.
	httpLog := logger.NewHttpLogger()

	// Wrap our mux with HTTP logging middleware.
	loggedMux := middleware.LoggingMiddleware(mux, httpLog)

	return &Server{
		mux:    loggedMux,
		logger: appLog,
	}
}

// filterObjects filters objects based on the prefix.
func filterObjects(objects map[string]entities.Object, prefix string) []entities.Object {
	var filtered []entities.Object
	for _, obj := range objects {
		if prefix == "" || strings.HasPrefix(obj.Key, prefix) {
			filtered = append(filtered, obj)
		}
	}
	return filtered
}
