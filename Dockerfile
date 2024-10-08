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
# Build the Go application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/main ./main.go

# Stage 2: Create the final image
FROM alpine:latest
# Install the PostgreSQL client (which includes pg_dump)
RUN apk --no-cache add postgresql-client
# Copy the compiled Go binary from the build stage
COPY --from=builder /app/main /main
# Set the working directory
WORKDIR /
# Set the environment variable to production
ENV GO_ENV=production
# Command to run the Go application
CMD ["/main"]
