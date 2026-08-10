# File Storage

This is a file storage server for the MAXIT project.

## Features

- **S3-like API**: Bucket and object management
- **Signed URLs**: Time-limited, secure access to files via HMAC-SHA256 signatures
- **Simple deployment**: Single binary with filesystem storage
- **Two-server topology**: public (signed GET/HEAD only) + internal (writes, private)

## Server topology

The service runs two HTTP servers:

- **Public** (`PUBLIC_SERVER_PORT`, default `8888`): signed GET/HEAD of objects only. All else → 403/405.
- **Internal** (`INTERNAL_SERVER_PORT`, default `8081`): all writes, bucket management, metadata, `/sign`. No auth — **network isolation only, never host-publish this port**.

See [SIGNED_URLS.md](./SIGNED_URLS.md) for the full signed-URL contract and deployment guidance.

### Signed URLs for Secure File Access

The file storage service supports signed, time-limited URLs for secure file access. See [SIGNED_URLS.md](./SIGNED_URLS.md) for detailed documentation.

Quick example:
```go
storage, _ := filestorage.NewFileStorage(filestorage.FileStorageConfig{
    URL: "http://file-storage:8081", // internal server, server-side only
})

// Signed path relative to the public server (caller prepends its public base URL)
signedPath, _ := storage.GetSignedFilePath("bucket", "file.pdf", 1*time.Hour)
```

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
   
   Available variables (see [SIGNED_URLS.md](./SIGNED_URLS.md) for details):
   - `PUBLIC_SERVER_PORT` — public server port (default `8888`)
   - `INTERNAL_SERVER_PORT` — internal server port (default `8081`; **never host-publish**)
   - `ROOT_DIRECTORY` — data directory (default `file-storage-media`)
   - `SIGNING_SECRET` — HMAC key, required
   - `MAX_SIGN_TTL_SECONDS` — max `/sign` TTL (default `3600`)
   
4. **Run the Application**: To run the application, you can use the prepared `Makefile`.
   just run:
   ```bash
   make
   ```
   Both servers start: public on `PUBLIC_SERVER_PORT`, internal on `INTERNAL_SERVER_PORT`.

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
