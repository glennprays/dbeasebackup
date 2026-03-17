# DBeaseBackup Makefile
# Project: github.com/glennprays/dbeasebackup
# Binary: dbeasebackup

BINARY_NAME=dbeasebackup
MAIN_PATH=./cmd/dbeasebackup
GO=go

# Default target
.DEFAULT_GOAL := help

## build: Build the binary
build:
	$(GO) build -o $(BINARY_NAME) $(MAIN_PATH)

## build-linux: Build for Linux (amd64)
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -o $(BINARY_NAME) $(MAIN_PATH)

## build-darwin: Build for macOS
build-darwin:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build -o $(BINARY_NAME) $(MAIN_PATH)

## build-all: Build all packages
build-all:
	$(GO) build -v ./...

## test: Run all tests
test:
	$(GO) test -cover ./...

## test-verbose: Run tests with verbose output
test-verbose:
	$(GO) test -v ./...

## test-coverage: Run tests with coverage
test-coverage:
	$(GO) test -cover ./...

## test-race: Run tests with race detection
test-race:
	$(GO) test -race ./...

## fmt: Format code
fmt:
	$(GO) fmt ./...

## vet: Run go vet
vet:
	$(GO) vet ./...

## lint: Run golangci-lint
lint:
	golangci-lint run

## tidy: Tidy dependencies
tidy:
	$(GO) mod tidy

## check: Run fmt, vet, and test
check: fmt vet test

## run: Run the application
run:
	$(GO) run $(MAIN_PATH)

## docker-build: Build Docker image
docker-build:
	docker build -t $(BINARY_NAME) .

## docker-run: Build and run in Docker
docker-run: docker-build
	docker run -it --rm $(BINARY_NAME)

## clean: Remove build artifacts
clean:
	rm -f $(BINARY_NAME)
	$(GO) clean

## help: Show this help
help:
	@echo "DBeaseBackup - Database Backup Tool"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':'
