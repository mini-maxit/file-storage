FROM golang:1.27.3 AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /file-storage ./cmd/app

FROM gcr.io/distroless/static-debian13

COPY --from=build /file-storage /file-storage

# Public server port (signed GET/HEAD only, host-publish this one)
ENV PUBLIC_SERVER_PORT 8888
# Internal server port (writes + /sign). NEVER host-publish this port.
ENV INTERNAL_SERVER_PORT 8081

# Required at runtime:
#   SIGNING_SECRET       - HMAC key (required, startup fails without it)
# Optional:
#   ROOT_DIRECTORY       - data directory (default file-storage-media)
#   MAX_SIGN_TTL_SECONDS - max /sign TTL (default 3600)

CMD [ "/file-storage" ]
