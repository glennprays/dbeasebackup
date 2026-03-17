<p align="center">
  <img src="./img/logo.png" alt="Logo" width="160"/>

  <h1 align="center">DBEaseBackup</h1>
  <p align="center">A tool designed to automate database backups to cloud storage on a scheduled basis.</p>
  <p align="center">
    <a href="https://github.com/glennprays/dbeasebackup/actions/workflows/build-docker-image.yml"><img src="https://github.com/glennprays/dbeasebackup/actions/workflows/build-docker-image.yml/badge.svg" alt="Build Docker Image Status"></a>
    <a href="https://hub.docker.com/r/glennprays/dbeasebackup"><img src="https://img.shields.io/docker/v/glennprays/dbeasebackup?label=Docker&color=blue" alt="Docker Image"></a>
    <img src="https://img.shields.io/badge/License-MIT-green.svg" alt="License">
  </p>
</p>

## Table of Contents

- [Features](#features)
- [Quick Start](#quick-start)
- [Installation](#installation)
  - [Docker (Recommended)](#docker-recommended)
  - [Binary](#binary)
  - [From Source](#from-source)
- [Configuration](#configuration)
  - [Environment Variables](#environment-variables)
  - [Cron Schedule Examples](#cron-schedule-examples)
- [Storage Backends](#storage-backends)
  - [Google Drive Setup](#google-drive-setup)
  - [S3 Setup](#s3-setup)
- [Usage](#usage)
- [Backup Management](#backup-management)
- [Troubleshooting](#troubleshooting)
- [Security Considerations](#security-considerations)
- [Contributing](#contributing)
- [License](#license)

## Features

- **Automated Scheduled Backups** - Run database backups on configurable cron schedules
- **Multiple Database Support** - PostgreSQL (MySQL and others planned)
- **Multiple Storage Backends** - Google Drive and S3-compatible storage
- **Docker-First Design** - Easy deployment with Docker and docker-compose
- **Configurable Timeouts** - Prevent hanging backups with adjustable timeout settings
- **Automatic Connection Cleanup** - Handles stale database connections gracefully
- **Structured Logging** - JSON and text output formats with trace IDs
- **Timezone Support** - Run backups in any timezone

## Quick Start

Get started in 3 steps with Docker:

```bash
# 1. Copy the example configuration
cp example/.env.example .env

# 2. Edit .env with your database credentials and schedule
# Required: PG_HOST, PG_USER, PG_PASSWORD, PG_DATABASE, CRON_SCHEDULE
# For Google Drive: GOOGLE_DRIVE_FOLDER_ID

# 3. Run with Docker
docker run -d \
  --name dbeasebackup \
  --env-file .env \
  -v $(pwd)/service-account-key.json:/service-account-key.json \
  -p 8080:8080 \
  --network host \
  glennprays/dbeasebackup:latest
```

# 4. Verify health check
curl http://localhost:8080/health

For detailed setup instructions, see the [example directory](./example).

## Installation

### Docker (Recommended)

Pull and run the official image:

```bash
docker pull glennprays/dbeasebackup:latest
```

Or use docker-compose (recommended for production):

```bash
# Copy example files
cp -r example/* ./

# Configure your environment
cp .env.example .env
# Edit .env with your settings

# Start the service
docker compose up -d
```

### Binary

Download the latest release for your platform from the [releases page](https://github.com/glennprays/dbeasebackup/releases).

```bash
# Make it executable
chmod +x dbeasebackup

# Run with environment variables
export PG_HOST=localhost
export PG_USER=postgres
export PG_PASSWORD=your_password
export PG_DATABASE=your_database
export CRON_SCHEDULE="0 2 * * *"
export GOOGLE_DRIVE_FOLDER_ID=your_folder_id

./dbeasebackup
```

### From Source

Build from source (requires Go 1.25+):

```bash
# Clone the repository
git clone https://github.com/glennprays/dbeasebackup.git
cd dbeasebackup

# Install dependencies
go mod tidy

# Build
make build

# Or build for specific platforms
make build-linux   # Linux amd64
make build-darwin  # macOS

# Run
./dbeasebackup
```

## Configuration

### Environment Variables

DBEaseBackup is configured entirely through environment variables:

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| **Application** |
| `ENV` | Application environment (`local`, `development`, `production`) | `production` | No |
| **Database** |
| `PG_HOST` | PostgreSQL host address | - | Yes |
| `PG_PORT` | PostgreSQL port | `5432` | Yes |
| `PG_USER` | PostgreSQL username | - | Yes |
| `PG_PASSWORD` | PostgreSQL password | - | Yes |
| `PG_DATABASE` | Database name to backup | - | Yes |
| **Backup** |
| `BACKUP_PROVIDER` | Backup provider type | `postgres` | No |
| `BACKUP_DIR` | Local directory for backup files | `backups/postgres` | No |
| `BACKUP_TIMEOUT` | Timeout for backup operations (e.g., `30m`, `1h`, `2h30m`) | `30m` | No |
| **Scheduler** |
| `CRON_SCHEDULE` | Cron expression for backup schedule | - | Yes |
| `SCHEDULER_TIMEZONE` | Timezone for cron jobs | `UTC` | No |
| **Storage** |
| `STORAGE_TYPE` | Storage backend (`google-drive` or `s3`) | `google-drive` | No |
| **Google Drive** |
| `GOOGLE_DRIVE_FOLDER_ID` | Target Google Drive folder ID | - | Yes* |
| `GOOGLE_DRIVE_KEY_FILE` | Path to service account JSON key | `service-account-key.json` | No |
| **S3** |
| `S3_BUCKET` | S3 bucket name | - | Yes* |
| `S3_REGION` | S3 region | `us-east-1` | No |
| `S3_ACCESS_KEY_ID` | S3 access key ID | - | Yes* |
| `S3_SECRET_ACCESS_KEY` | S3 secret access key | - | Yes* |
| `S3_ENDPOINT` | Custom S3 endpoint (for S3-compatible storage) | - | No |
| **Logging** |
| `LOG_LEVEL` | Log level (`debug`, `info`, `warn`, `error`) | `info` | No |
| `LOG_FORMAT` | Log format (`json` or `text`) | `text` | No |
| **Health Check** |
| `HTTP_PORT` | Health check endpoint port (set to `0` to disable) | `8080` | No |

*Required when using that storage type

For detailed configuration guidance, see [docs/CONFIGURATION.md](./docs/CONFIGURATION.md).

### Cron Schedule Examples

| Schedule | Cron Expression | Description |
|----------|-----------------|-------------|
| Every hour | `0 * * * *` | Backup at the top of every hour |
| Every 6 hours | `0 */6 * * *` | Backup at midnight, 6am, noon, 6pm |
| Daily at 2am | `0 2 * * *` | Backup once daily at 2:00 AM |
| Daily at midnight | `0 0 * * *` | Backup once daily at midnight |
| Twice daily | `0 2,14 * * *` | Backup at 2:00 AM and 2:00 PM |
| Weekly (Sunday 2am) | `0 2 * * 0` | Backup every Sunday at 2:00 AM |
| Monthly (1st at 2am) | `0 2 1 * *` | Backup on the 1st of every month at 2:00 AM |

## Storage Backends

### Google Drive Setup

1. **Create a Google Service Account**
   - Go to [Google Cloud Console](https://console.cloud.google.com/)
   - Create a service account and download the JSON key
   - Rename to `service-account-key.json`

2. **Configure Google Drive Folder**
   - Create a folder in Google Drive for backups
   - Share the folder with the service account email (found in the JSON key)
   - Copy the folder ID from the URL: `https://drive.google.com/drive/folders/<FOLDER_ID>`

3. **Set Environment Variables**
   ```bash
   STORAGE_TYPE=google-drive
   GOOGLE_DRIVE_FOLDER_ID=your_folder_id_here
   GOOGLE_DRIVE_KEY_FILE=service-account-key.json
   ```

For detailed instructions, see [example/README.md](./example/README.md).

### S3 Setup

1. **Configure S3 Credentials**
   ```bash
   STORAGE_TYPE=s3
   S3_BUCKET=your-backup-bucket
   S3_REGION=us-east-1
   S3_ACCESS_KEY_ID=your_access_key
   S3_SECRET_ACCESS_KEY=your_secret_key
   ```

2. **For S3-Compatible Storage** (MinIO, DigitalOcean Spaces, etc.)
   ```bash
   S3_ENDPOINT=https://your-s3-compatible-endpoint
   ```

3. **Recommended S3 Bucket Settings**
   - Enable versioning for backup history
   - Configure lifecycle policies for automatic cleanup
   - Enable encryption at rest

## Usage

### Running with Docker

```bash
# Using docker run
docker run -d \
  --name dbeasebackup \
  --env-file .env \
  -v $(pwd)/service-account-key.json:/service-account-key.json \
  --network host \
  glennprays/dbeasebackup:latest

# Using docker-compose
docker compose up -d

# View logs
docker logs -f dbeasebackup
```

### Running as Binary

```bash
# Set required environment variables
export $(cat .env | xargs)

# Run
./dbeasebackup
```

### Health Checks

The application provides HTTP endpoints for health monitoring and Kubernetes integration:

| Endpoint | Purpose | Behavior |
|----------|---------|----------|
| `GET /health` | Comprehensive health check | Checks database and storage, returns 200 if healthy |
| `GET /readyz` | Kubernetes readiness probe | Returns `{"ready": true}` if all systems healthy |
| `GET /livez` | Kubernetes liveness probe | Returns `{"alive": true}` if server process is running |

**Docker port mapping:**
```bash
# Map port 8080 to access health checks from outside container
docker run -p 8080:8080 glennprays/dbeasebackup:latest
```

**Kubernetes probes example:**
```yaml
livenessProbe:
  httpGet:
    path: /livez
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /readyz
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5
```

**Testing health endpoint:**
```bash
curl http://localhost:8080/health
curl http://localhost:8080/readyz
curl http://localhost:8080/livez
```

**Note:** The application validates configuration at startup and will exit with an error if required variables are missing or invalid.

## Backup Management

### How Backups Work

1. Scheduler triggers backup at the configured cron schedule
2. `pg_dump` creates a tar-format backup file
3. Backup metadata is recorded in the `database_backups` table
4. Backup file is uploaded to configured storage (Google Drive or S3)
5. Local backup file is deleted after successful upload

### Backup File Format

- **Format**: tar archive (pg_dump tar format)
- **Naming**: `<database>_<timestamp>.tar`
- **Location**: Uploaded to cloud storage, local copy deleted after upload

### Restoring from Backup

```bash
# Download backup from storage (Google Drive or S3)

# Restore using pg_restore
pg_restore -h localhost -U postgres -d restored_database backup_file.tar
```

For detailed backup management guidance, see [docs/BACKUP-MANAGEMENT.md](./docs/BACKUP-MANAGEMENT.md).

## Troubleshooting

### Common Issues

**Database Connection Issues**
- Verify `PG_HOST`, `PG_PORT`, `PG_USER`, `PG_PASSWORD` are correct
- Ensure PostgreSQL accepts connections from the container (use `--network host`)
- Check if `pg_dump` is installed: `pg_dump --version`

**Backup Hanging/Timeout**
- Increase `BACKUP_TIMEOUT` for large databases (e.g., `1h`, `2h30m`)
- The application automatically cleans up stale connections
- Check PostgreSQL for long-running queries: `SELECT * FROM pg_stat_activity`

**Google Drive Upload Failures**
- Verify the service account JSON key is valid
- Ensure the folder is shared with the service account email
- Check `GOOGLE_DRIVE_FOLDER_ID` is correct

**S3 Upload Failures**
- Verify S3 credentials have write permissions
- Check bucket exists and is in the correct region
- For custom endpoints, ensure `S3_ENDPOINT` is set correctly

### Viewing Logs

Enable debug logging for troubleshooting:

```bash
LOG_LEVEL=debug
LOG_FORMAT=json  # For easier parsing
```

For comprehensive troubleshooting guidance, see [docs/TROUBLESHOOTING.md](./docs/TROUBLESHOOTING.md).

## Security Considerations

### Credential Management

- Never commit `.env` files or service account keys to version control
- Use secrets management (Docker secrets, Kubernetes secrets, HashiCorp Vault)
- Rotate credentials regularly

### Database Access

- Create a dedicated backup user with minimal privileges:
  ```sql
  CREATE USER backup_user WITH PASSWORD 'secure_password';
  GRANT CONNECT ON DATABASE your_database TO backup_user;
  GRANT USAGE ON SCHEMA public TO backup_user;
  GRANT SELECT ON ALL TABLES IN SCHEMA public TO backup_user;
  ```

### Network Security

- Use `--network host` only when necessary; prefer Docker networks
- Enable SSL for PostgreSQL connections in production

For comprehensive security guidance, see [docs/SECURITY.md](./docs/SECURITY.md).

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](./CONTRIBUTING.md) for:

- Development setup instructions
- Code style guidelines
- Pull request process
- How to add new storage providers or database support

## Supported Databases

- PostgreSQL (fully supported)
- MySQL (planned)
- MongoDB (planned)

## Supported Storage

- Google Drive
- S3 and S3-compatible storage (MinIO, DigitalOcean Spaces, etc.)
- Azure Blob Storage (planned)
- Backblaze B2 (planned)

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
