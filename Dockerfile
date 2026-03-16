# Stage 1: Build the Go application
FROM golang:1.22 AS builder
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
# Build the Go application
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /app/dbeasebackup ./cmd/dbeasebackup

# Stage 2: Create the final image
FROM alpine:latest
# Install the PostgreSQL client (which includes pg_dump)
RUN apk --no-cache add postgresql-client
# Copy the compiled Go binary from the build stage
COPY --from=builder /app/dbeasebackup /dbeasebackup
# Set the working directory
WORKDIR /
# Set the environment variable to production
ENV GO_ENV=production
# Command to run the Go application
CMD ["/dbeasebackup"]
