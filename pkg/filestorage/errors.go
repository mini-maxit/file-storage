package filestorage

import (
	"errors"
	"fmt"
)

var (
	ErrBucketAlreadyExists    = errors.New("bucket already exists")
	ErrBucketNotFound         = errors.New("bucket not found")
	ErrFailedToCreateBucket   = errors.New("failed to create bucket")
	ErrFailedToScanBuckets    = errors.New("failed to scan buckets directory")
	ErrFailedToLoadObjects    = errors.New("failed to load objects for bucket")
	ErrBucketNotEmpty         = errors.New("bucket not empty")
	ErrFailedToEncodeResponse = errors.New("failed to encode response")
	ErrFailedToGetBucket      = errors.New("failed to get bucket")
	ErrObjectNotFound         = errors.New("object not found")
	ErrFailedToGetObject      = errors.New("failed to get object")
)

// ErrAPI represents an error returned by the API.
type ErrAPI struct {
	StatusCode int
	Message    string
}

func (e *ErrAPI) Error() string {
	return fmt.Sprintf("[HTTP %d] API error: %s", e.StatusCode, e.Message)
}

// ErrClient wraps an error that occurs on the client side,
// e.g., I/O failures, encoding bugs, request construction issues.
type ErrClient struct {
	Message string
	Err     error
	Context map[string]any
}

func (e *ErrClient) Error() string {
	contextStr := ""
	if e.Context != nil {
		contextStr = fmt.Sprintf("(context: %v)", e.Context)
	}
	return fmt.Sprintf("client error: %s: %v %s", e.Message, e.Err, contextStr)
}

func (e *ErrClient) Unwrap() error {
	return e.Err
}
