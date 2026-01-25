# File Storage

This is a file storage server for the MAXIT project.

## Features

- **S3-like API**: Bucket and object management
- **Signed URLs**: Time-limited, secure access to PDF files
- **Simple deployment**: Single binary with filesystem storage
- **HMAC-SHA256 signatures**: Cryptographically secure URL signing

## Build

Prerequisites:

- Docker

To build docker image for local usage run the following command:

```bash
docker build -t maxit/file-storage .
```

## Usage

### Prerequisites:

- **Go**: Ensure you have Go installed on your machine (version 1.23.2).

To set up and run the File Storage API, follow these steps:

1. **Clone the Repository**:
   ```bash
    git clone https://github.com/mini-maxit/file-storage.git
    cd file-storage
   ```
2. **Install Go Packages**: Ensure all necessary Go packages are installed by running:
   ```bash
    go mod tidy
   ```
3. **Environment Configuration**: Copy the .env.dist file to .env:
   ```bash
    cp .env.dist .env
   ```
   Update the `.env` file with the necessary environment variables.
   
   **Important**: Set `SIGNING_SECRET` for signed URL support:
   ```bash
   SIGNING_SECRET=your-secret-key-here
   ```
   
4. **Run the Application**: To run the application, you can use the prepared `Makefile`.
   jut run:
   ```bash
   make
   ```

## Features

### Signed URLs for Secure File Access

The file storage service supports signed, time-limited URLs for secure PDF access. See [SIGNED_URLS.md](./SIGNED_URLS.md) for detailed documentation.

Quick example:
```go
storage, _ := filestorage.NewFileStorage(filestorage.FileStorageConfig{
    URL: "https://storage.example.com",
})

// Generate a URL valid for 1 hour
signedURL, _ := storage.GetSignedFileURL("bucket", "file.pdf", 1*time.Hour, "secret")
```

## Endpoints

OpenAPI 3.0 specification: [api.raml](./api.raml)

### Error Structure

When an error occurs, the response is returned in JSON format with the following structure:

```json
{
  "reason": "A brief explanation of the error",
  "details": "A more detailed description of the error",
  "context": {
    "key": "value",
    "key2": "value2"
  }
}
```

**Field Descriptions**:

- reason: A high-level message describing the cause of the error, such as "Failed to process task" or "Submission not found."
- details: A more specific message or description of the error, often based on the underlying issue (e.g., "Invalid task parameters").
- context: An optional field containing additional context information about the error. This might include values like taskID, userID, submissionNumber, or other key-value pairs that provide insight into the specific conditions under which the error occurred. This field is included when relevant context is available.
