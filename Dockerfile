# syntax=docker/dockerfile:1

FROM golang:1.22 AS builder
WORKDIR /app

# Only download modules first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Now copy the rest of the source
COPY . .

# Build a static Linux binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /app/bin/server ./main.go

# Use a minimal base with CA certificates for HTTPS
FROM gcr.io/distroless/base-debian12
WORKDIR /

COPY --from=builder /app/bin/server /server

ENV PORT=8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/server"]


