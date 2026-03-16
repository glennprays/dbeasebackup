# Contributing to DBEaseBackup

Thank you for your interest in contributing to DBEaseBackup! This document provides guidelines and instructions for contributing.

## Table of Contents

- [Development Setup](#development-setup)
- [Development Workflow](#development-workflow)
- [Code Style](#code-style)
- [Project Architecture](#project-architecture)
- [Adding New Providers](#adding-new-providers)
- [Pull Request Process](#pull-request-process)
- [Commit Message Format](#commit-message-format)

## Development Setup

### Prerequisites

- **Go 1.25+** - [Install Go](https://golang.org/doc/install)
- **PostgreSQL Client** - Required for `pg_dump` command
  - macOS: `brew install postgresql`
  - Ubuntu/Debian: `sudo apt-get install postgresql-client`
  - Windows: [Download PostgreSQL](https://www.postgresql.org/download/windows/)
- **Docker** - For containerized testing
- **golangci-lint** - For linting (optional but recommended)
  - macOS: `brew install golangci-lint`
  - Linux: `curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin`
- **Make** - For using Makefile commands

### Getting Started

1. **Fork and Clone**
   ```bash
   # Fork the repository on GitHub, then:
   git clone https://github.com/YOUR_USERNAME/dbeasebackup.git
   cd dbeasebackup
   ```

2. **Install Dependencies**
   ```bash
   go mod tidy
   ```

3. **Set Up Local Environment**
   ```bash
   # Copy example configuration
   cp .env.example .env

   # Edit .env with your local database credentials
   ```

4. **Verify Setup**
   ```bash
   # Run tests
   make test

   # Run linter
   make lint
   ```

## Development Workflow

### Building

```bash
# Build for current platform
make build

# Build for Linux (for Docker)
make build-linux

# Build for macOS
make build-darwin

# Build all packages
make build-all
```

### Testing

```bash
# Run all tests
make test

# Run tests with verbose output
make test-verbose

# Run tests with coverage
make test-coverage

# Run tests with race detection
make test-race
```

### Linting

```bash
# Run go vet
make vet

# Run golangci-lint
make lint

# Format code
make fmt
```

### Running Locally

```bash
# Run the application
make run

# Or build and run manually
make build
./dbeasebackup
```

### Docker Development

```bash
# Build Docker image
make docker-build

# Build and run in Docker
make docker-run
```

### Quick Quality Check

```bash
# Run fmt, vet, and test in one command
make check
```

## Code Style

### Go Conventions

- Follow [Effective Go](https://golang.org/doc/effective_go) guidelines
- Use `gofmt` for formatting (run `make fmt`)
- Follow [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)

### General Guidelines

- **Keep functions small and focused** - Each function should do one thing well
- **Use meaningful names** - Variable and function names should be self-documenting
- **Add comments for exported types and functions** - Follow Go's documentation conventions
- **Handle errors explicitly** - Don't ignore errors; handle or propagate them
- **Use interfaces for abstraction** - See the `Database` and `Storage` interfaces

### Formatting

Always format your code before committing:

```bash
make fmt
```

### Logging

The project uses `github.com/glennprays/log` for structured logging. Every log entry requires a `traceId`:

```go
// Correct usage
logger.Info(traceID, "Database backup created", nil,
    log.String("path", backupFileDir),
    log.Int("size", fileSize),
)

// Error logging
logger.Error(traceID, "Failed to upload backup", nil, log.Error(err))
```

## Project Architecture

DBEaseBackup follows clean architecture principles with clear separation of concerns:

```
cmd/dbeasebackup/main.go     # Entry point with dependency injection
├── config/                   # Configuration loading and validation
├── internal/backup/          # Core backup orchestration
└── pkg/                      # Reusable packages
    ├── database/             # Database interface and implementations
    ├── storage/              # Storage interface and implementations
    └── scheduler/            # Scheduler interface and implementations
```

### Key Interfaces

- **`Database`** (`pkg/database/database.go`) - Database connection and operations
- **`Storage`** (`pkg/storage/storage.go`) - Cloud storage operations
- **`Scheduler`** (`pkg/scheduler/scheduler.go`) - Job scheduling

For detailed architecture documentation, see [CLAUDE.md](./CLAUDE.md).

## Adding New Providers

### Adding a New Database Provider

1. **Implement the `Database` interface** in `pkg/database/`:

   ```go
   type Database interface {
       Connect(ctx context.Context) error
       Close() error
       Ping(ctx context.Context) error
       Exec(ctx context.Context, query string, args ...any) (sql.Result, error)
       Query(ctx context.Context, query string, args ...any) (*sql.Rows, error)
       QueryRow(ctx context.Context, query string, args ...any) *sql.Row
       GetDB() *sql.DB
   }
   ```

2. **Create the implementation file** (e.g., `pkg/database/mysql.go`)

3. **Add configuration variables** to `config/config.go`

4. **Update validation** in `config/config.go` to validate the new provider

5. **Add factory function** to create the appropriate database instance

6. **Write tests** for the new implementation

### Adding a New Storage Provider

1. **Implement the `Storage` interface** in `pkg/storage/`:

   ```go
   type Storage interface {
       Upload(ctx context.Context, file io.Reader, filename string, opts UploadOptions) error
       Download(ctx context.Context, filename string) (io.ReadCloser, error)
       Delete(ctx context.Context, filename string) error
       List(ctx context.Context, opts ListOptions) ([]FileInfo, error)
       Health(ctx context.Context) error
   }
   ```

2. **Create the implementation file** (e.g., `pkg/storage/azure-blob.go`)

3. **Add configuration variables** to `config/config.go`

4. **Update validation** in `config/config.go` to validate the new storage type

5. **Add factory function** to create the appropriate storage instance

6. **Write tests** for the new implementation

7. **Update documentation** (README.md, docs/, example/)

## Pull Request Process

### Before Submitting

1. **Create a feature branch**
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make your changes** following code style guidelines

3. **Run quality checks**
   ```bash
   make check  # Runs fmt, vet, and test
   make lint   # Run linter
   ```

4. **Update documentation** if needed
   - Update README.md for user-facing changes
   - Update CLAUDE.md for architecture changes
   - Add/update inline code comments

5. **Commit your changes** (see [Commit Message Format](#commit-message-format))

### Submitting

1. **Push to your fork**
   ```bash
   git push origin feature/your-feature-name
   ```

2. **Open a Pull Request** on GitHub
   - Provide a clear description of the changes
   - Reference any related issues
   - Ensure all CI checks pass

3. **Respond to code review feedback**

### After Merge

- Delete your feature branch
- Update your local main branch
   ```bash
   git checkout main
   git pull upstream main
   ```

## Commit Message Format

We follow [Conventional Commits](https://www.conventionalcommits.org/) specification:

### Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Types

- `feat` - New feature
- `fix` - Bug fix
- `docs` - Documentation changes
- `style` - Code style changes (formatting, semicolons, etc.)
- `refactor` - Code refactoring
- `test` - Adding or updating tests
- `chore` - Maintenance tasks

### Examples

```bash
# Feature
feat: add MySQL database support

# Bug fix
fix: resolve pg_dump hanging issue in Docker

# Documentation
docs: update README with S3 configuration

# Refactoring
refactor: simplify backup service error handling

# With scope
feat(storage): add Azure Blob Storage support
fix(config): validate S3 endpoint format
```

### Breaking Changes

For breaking changes, add `BREAKING CHANGE:` in the footer:

```
feat(api): change Storage interface Upload method signature

BREAKING CHANGE: Upload method now requires UploadOptions parameter
```

## Questions or Issues?

- Open an issue for bugs or feature requests
- Start a discussion for questions or ideas

Thank you for contributing to DBEaseBackup!
