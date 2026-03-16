# Stage 1: Build the Go application
FROM golang:1.25-alpine AS builder

# Set the current working directory inside the container
WORKDIR /app

# Copy the go.mod and go.sum files to download dependencies
COPY go.mod go.sum ./

# Download all the dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build arguments for multi-arch (automatically provided by Buildx)
ARG TARGETOS
ARG TARGETARCH

# Build the Go application with optimizations (strip debug symbols, remove paths)
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /app/dbeasebackup ./cmd/dbeasebackup

# Stage 2: Create the final minimal image
FROM alpine:3.20

# Install only the PostgreSQL client (includes pg_dump)
# ca-certificates is needed for TLS connections to Google Drive/S3
RUN apk --no-cache add postgresql-client ca-certificates

# Copy the compiled Go binary from the build stage
COPY --from=builder /app/dbeasebackup /dbeasebackup

# Set the working directory
WORKDIR /

# Set the environment variable to production
ENV ENV=production

# Command to run the Go application
CMD ["/dbeasebackup"]
