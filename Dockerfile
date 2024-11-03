FROM golang:1.23

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV APP_PORT 8888

CMD [ "go", "run", "cmd/app/main.go" ]
