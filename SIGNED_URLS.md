# Signed URLs for Secure File Access

This file storage service supports signed, time-limited URLs for secure access to PDF files. This allows you to generate temporary URLs that expire after a specified duration, providing secure file delivery without requiring authentication at the storage level.

## Features

- **Time-limited access**: URLs expire after a configurable TTL (Time To Live)
- **Cryptographic signatures**: Uses HMAC-SHA256 to ensure URL integrity
- **Automatic validation**: Middleware validates signatures and expiration on every request
- **PDF-specific**: Currently enforces signed URLs only for PDF files
- **No authentication required**: Clients don't need credentials, just the signed URL

## Configuration

Set the `SIGNING_SECRET` environment variable in your `.env` file:

```bash
SIGNING_SECRET=your-secret-key-here
```

**Important**: Use a strong, randomly generated secret in production. The secret must be kept confidential and should be rotated periodically.

## Usage

### Generating Signed URLs (Client-side)

Use the `GetSignedFileURL` method to generate a signed URL:

```go
package main

import (
    "fmt"
    "time"
    "github.com/mini-maxit/file-storage/pkg/filestorage"
)

func main() {
    // Create file storage client
    config := filestorage.FileStorageConfig{
        URL: "https://storage.example.com",
    }
    storage, err := filestorage.NewFileStorage(config)
    if err != nil {
        panic(err)
    }

    // Generate a signed URL valid for 1 hour
    bucketName := "task-pdfs"
    objectKey := "task-123/description.pdf"
    ttl := 1 * time.Hour
    signingSecret := "your-secret-key-here" // Must match server secret

    signedURL, err := storage.GetSignedFileURL(bucketName, objectKey, ttl, signingSecret)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Signed URL: %s\n", signedURL)
    // Output: https://storage.example.com/buckets/task-pdfs/task-123/description.pdf?expires=1234567890&signature=...
}
```

### URL Structure

A signed URL includes:
- **Base path**: `/buckets/{bucketName}/{objectKey}`
- **expires**: Unix timestamp indicating when the URL expires
- **signature**: HMAC-SHA256 signature of the path and expiration

Example:
```
https://storage.example.com/buckets/task-pdfs/task-123/description.pdf?expires=1737804000&signature=abc123...
```

### Server-side Validation

The server automatically validates:
1. **Signature authenticity**: Ensures the URL hasn't been tampered with
2. **Expiration**: Rejects URLs past their expiration time
3. **Method**: Only GET requests are allowed with signed URLs

### Error Responses

| Scenario | HTTP Status | Response |
|----------|------------|----------|
| Valid signed URL | 200 OK | File content |
| Expired URL | 403 Forbidden | "Forbidden: URL has expired" |
| Invalid/tampered signature | 403 Forbidden | "Forbidden: invalid signature" |
| Missing signature (PDF) | 403 Forbidden | "Forbidden: signature required for PDF file access" |
| Metadata-only request | 200 OK | Metadata JSON (no signature required) |

## Security Considerations

1. **Secret Management**:
   - Store the signing secret securely (environment variables, secrets manager)
   - Never commit secrets to version control
   - Use different secrets for different environments
   - Rotate secrets periodically

2. **TTL Selection**:
   - Use the shortest TTL practical for your use case
   - For temporary downloads: 5-15 minutes
   - For email links: 1-24 hours
   - For public sharing: consider access control implications

3. **HTTPS Only**:
   - Always serve files over HTTPS to prevent URL interception
   - The signature protects against tampering, but HTTPS protects against eavesdropping

4. **Signature in Query Parameters**:
   - Allows CDN caching
   - Ensures URLs are self-contained and shareable
   - Note: Query parameters may appear in server logs

## File Type Restrictions

Currently, signature validation is enforced only for PDF files (`.pdf` extension). Other file types can be accessed without signatures, though signatures are still validated if provided.

To extend this to all files or specific file types, modify the `isPDFFile` function in `internal/api/http/middleware/signature.go`.

## Implementation Details

### URL Signing Algorithm

The signature is generated using HMAC-SHA256:

```
stringToSign = "{path}:{expiresTimestamp}"
signature = base64_url_encode(HMAC_SHA256(secret, stringToSign))
```

### Middleware Flow

1. Request arrives at server
2. Middleware checks if it's a GET request to an object endpoint
3. If signature parameters are present, validates them
4. If path is a PDF and no signature is present, rejects with 403
5. If validation passes, forwards to the handler
6. Handler serves the file

## Testing

Run the test suite to verify signed URL functionality:

```bash
# Test URL signer
go test ./pkg/urlsigner/

# Test middleware
go test ./internal/api/http/middleware/

# Test client library
go test ./pkg/filestorage/

# Run all tests
go test ./...
```

## Examples

### Example 1: Generate and Use a Signed URL

```go
// Server configuration
os.Setenv("SIGNING_SECRET", "my-secret-key")

// Client generates signed URL
storage, _ := filestorage.NewFileStorage(filestorage.FileStorageConfig{
    URL: "https://storage.example.com",
})

signedURL, _ := storage.GetSignedFileURL("docs", "manual.pdf", 15*time.Minute, "my-secret-key")

// User clicks the link and downloads the file
// URL is valid for 15 minutes
```

### Example 2: Backend Integration

```go
// In your backend service that manages tasks
func generateTaskPDFLink(taskID string) (string, error) {
    storage, err := filestorage.NewFileStorage(filestorage.FileStorageConfig{
        URL: os.Getenv("FILE_STORAGE_URL"),
    })
    if err != nil {
        return "", err
    }

    bucketName := "task-pdfs"
    objectKey := fmt.Sprintf("task-%s/description.pdf", taskID)
    ttl := 1 * time.Hour
    secret := os.Getenv("SIGNING_SECRET")

    return storage.GetSignedFileURL(bucketName, objectKey, ttl, secret)
}
```

## Troubleshooting

### "Forbidden: invalid signature"
- Ensure the signing secret matches on both client and server
- Check that the URL hasn't been modified after generation
- Verify the path is exactly as signed (including case)

### "Forbidden: URL has expired"
- The URL's TTL has passed
- Generate a new signed URL
- Consider increasing TTL if users need more time

### "Forbidden: signature required for PDF file access"
- PDF files require signed URLs
- Generate a signed URL before accessing the file
- For metadata access, use `?metadataOnly=true`

## Migration Notes

If you have existing clients accessing PDFs without signatures:
1. Deploy the server with signature validation
2. Update clients to generate signed URLs
3. Consider a grace period with optional signatures before enforcing
4. Monitor logs for signature validation failures

## API Compatibility

This feature maintains backward compatibility:
- Metadata requests (`?metadataOnly=true`) don't require signatures
- Non-PDF files don't require signatures (currently)
- Upload, delete, and bucket operations are unaffected
