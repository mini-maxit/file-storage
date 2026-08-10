FROM golang:1.23

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Public server port (signed GET/HEAD only, host-publish this one)
ENV PUBLIC_SERVER_PORT 8888
# Internal server port (writes + /sign). NEVER host-publish this port.
ENV INTERNAL_SERVER_PORT 8081

# Required at runtime:
#   SIGNING_SECRET       - HMAC key (required, startup fails without it)
# Optional:
#   ROOT_DIRECTORY       - data directory (default file-storage-media)
#   MAX_SIGN_TTL_SECONDS - max /sign TTL (default 3600)

CMD [ "go", "run", "cmd/app/main.go" ]
