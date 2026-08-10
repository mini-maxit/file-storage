# Signed URLs & server topology

file-storage runs **two HTTP servers** with strictly separated roles. This separation is the security model — do not blur it.

## Two servers

| Server | Default port | Role | Auth |
|---|---|---|---|
| **Public** | `8888` (`PUBLIC_SERVER_PORT`) | Signed GET / HEAD of objects only. All else → 403/405. | HMAC-SHA256 signature in URL |
| **Internal** | `8081` (`INTERNAL_SERVER_PORT`) | All writes (PUT/DELETE/multipart), bucket management, metadata, `/sign`. | None — network isolation only |

- The **internal server must never be host-published**. Backend and worker reach it as separate containers over the docker network (`http://file-storage:8081`). If you bind an internal port to the host you expose an unauthenticated API that can read, write and delete objects and mint signed URLs.
- The **public server is the only one reachable by browsers**, and only through signed URLs (e.g. via an nginx `/files/` prefix).

## Signed URL format

Signatures are generated server-side by the internal server and validated by the public server.

Signed path:

```
/buckets/{bucket}/{key}?expires={unixSeconds}&signature={base64url}
```

- `stringToSign = "/buckets/{bucket}/{key}:{expires}"`
- HMAC-SHA256 keyed by `SIGNING_SECRET`, signature base64 URL-encoded.
- Validation rejects expired URLs (`now >= expires`). See `pkg/urlsigner/urlsigner.go`.

## Getting a signed URL

Call the internal server's `/sign` endpoint (server-side only):

```
GET /sign?bucket={bucket}&key={key}&ttl={seconds}
```

- `bucket`, `key` — required. The object must exist (404 otherwise).
- `ttl` — optional, default `300`s, capped by `MAX_SIGN_TTL_SECONDS` (default `3600`, 400 if exceeded).
- Response: `{"signedPath": "/buckets/{bucket}/{key}?expires=...&signature=..."}`.

The caller prepends its public base URL (e.g. `https://example.com/files`) to `signedPath`.

## SDK usage

```go
storage, _ := filestorage.NewFileStorage(filestorage.FileStorageConfig{
    URL: "http://file-storage:8081", // internal server, server-side only
})

// signed path relative to the public server, e.g. /buckets/maxit/task/1/description.pdf?expires=...&signature=...
signedPath, _ := storage.GetSignedFilePath("maxit", "task/1/description.pdf", 5*time.Minute)

// raw unsigned URL — internal only, must never be returned to clients
storage.GetInternalFileURL("maxit", "task/1/description.pdf")
```

## Configuration

| Env | Default | Required | Notes |
|---|---|---|---|
| `PUBLIC_SERVER_PORT` | `8888` | | Public server port |
| `INTERNAL_SERVER_PORT` | `8081` | | Internal server port — do not host-publish |
| `ROOT_DIRECTORY` | `file-storage-media` | | Data directory |
| `SIGNING_SECRET` | | **yes** | HMAC key; startup fails if unset |
| `MAX_SIGN_TTL_SECONDS` | `3600` | | Max `ttl` accepted by `/sign` |

## Deployment checklist

1. Publish only `PUBLIC_SERVER_PORT` to the host (for nginx/browser signed access).
2. Never publish `INTERNAL_SERVER_PORT`; internal consumers connect over the docker network.
3. `SIGNING_SECRET` must be set and consistent across public+internal servers.
4. Keep `MAX_SIGN_TTL_SECONDS` bounded; minted URLs expire.
