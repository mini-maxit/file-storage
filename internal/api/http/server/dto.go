package server

import "github.com/mini-maxit/file-storage/pkg/filestorage/entities"

// ---- REQUESTS ----

// CreateBucketRequest represents payload for creating a bucket.
type CreateBucketRequest struct {
	Name string `json:"name"`
}

// ---- RESPONSES ----
type GetBucketResponse struct {
	Name            string                     `json:"name"`
	NumberOfObjects int                        `json:"numberOfObjects"`
	Objects         map[string]entities.Object `json:"objects"`
}