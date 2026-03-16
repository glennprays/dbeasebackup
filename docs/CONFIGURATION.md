# Configuration Reference

This document provides comprehensive configuration details for DBEaseBackup.

## Table of Contents

- [Environment Variables](#environment-variables)
- [Cron Schedule Examples](#cron-schedule-examples)
- [Timezone Configuration](#timezone-configuration)
- [Docker Configuration](#docker-configuration)
- [Configuration Validation](#configuration-validation)

## Environment Variables

### Complete Variable Reference

| Variable | Type | Default | Required | Description |
|----------|------|---------|----------|-------------|
| **Application** |
| `ENV` | string | `production` | No | Application environment. Values: `local`, `development`, `production`. In `local` or `development` mode, `.env` file is loaded automatically. |
| **Database Configuration** |
| `PG_HOST` | string | - | Yes | PostgreSQL server hostname or IP address. For Docker, use `host.docker.internal` to connect to host's localhost. |
| `PG_PORT` | string | `5432` | Yes | PostgreSQL server port. Standard port is 5432. |
| `PG_USER` | string | - | Yes | PostgreSQL username for authentication. Recommend using a dedicated backup user with minimal privileges. |
| `PG_PASSWORD` | string | - | Yes | PostgreSQL password. Handle securely - use secrets management in production. |
| `PG_DATABASE` | string | - | Yes | Name of the database to backup. |
| **Backup Configuration** |
| `BACKUP_PROVIDER` | string | `postgres` | No | Backup provider type. Currently only `postgres` is supported. |
| `BACKUP_DIR` | string | `backups/postgres` | No | Local directory for temporary backup files. Files are deleted after successful upload. |
| `BACKUP_TIMEOUT` | string | `30m` | No | Timeout for backup operations. Go duration format: `30m`, `1h`, `2h30m`. Increase for large databases. |
| **Scheduler Configuration** |
| `CRON_SCHEDULE` | string | - | Yes | Cron expression for backup schedule. Standard 5-field format. |
| `SCHEDULER_TIMEZONE` | string | `UTC` | No | Timezone for cron job execution. Use IANA timezone names. |
| **Storage Configuration** |
| `STORAGE_TYPE` | string | `google-drive` | No | Storage backend type. Values: `google-drive`, `s3`. |
| **Google Drive Configuration** |
| `GOOGLE_DRIVE_FOLDER_ID` | string | - | Conditional* | Google Drive folder ID for storing backups. Required when `STORAGE_TYPE=google-drive`. |
| `GOOGLE_DRIVE_KEY_FILE` | string | `service-account-key.json` | No | Path to Google Service Account JSON key file. Mounted in Docker container. |
| **S3 Configuration** |
| `S3_BUCKET` | string | - | Conditional* | S3 bucket name for storing backups. Required when `STORAGE_TYPE=s3`. |
| `S3_REGION` | string | `us-east-1` | No | AWS region for the S3 bucket. |
| `S3_ACCESS_KEY_ID` | string | - | Conditional* | AWS access key ID. Required when `STORAGE_TYPE=s3`. |
| `S3_SECRET_ACCESS_KEY` | string | - | Conditional* | AWS secret access key. Required when `STORAGE_TYPE=s3`. |
| `S3_ENDPOINT` | string | - | No | Custom S3 endpoint URL. Use for S3-compatible storage (MinIO, DigitalOcean Spaces, etc.). |
| **Logging Configuration** |
| `LOG_LEVEL` | string | `info` | No | Log verbosity level. Values: `debug`, `info`, `warn`, `error`. |
| `LOG_FORMAT` | string | `text` | No | Log output format. Values: `json`, `text`. Use `json` for production. |

*Conditional: Required when using the corresponding storage type.

### Variable Details

#### BACKUP_TIMEOUT Format

Go duration format examples:
- `30m` - 30 minutes
- `1h` - 1 hour
- `2h30m` - 2 hours 30 minutes
- `90m` - 90 minutes
- `1h30m15s` - 1 hour 30 minutes 15 seconds

Recommendations:
- Small databases (< 1GB): `30m`
- Medium databases (1-10GB): `1h`
- Large databases (> 10GB): `2h` or more

#### ENV Variable Behavior

| ENV Value | .env File | Use Case |
|-----------|-----------|----------|
| `local` | Loaded | Local development |
| `development` | Loaded | Development/testing environments |
| `production` | Not loaded | Production deployment |

## Cron Schedule Examples

### Cron Expression Format

```
┌───────────── minute (0 - 59)
│ ┌───────────── hour (0 - 23)
│ │ ┌───────────── day of month (1 - 31)
│ │ │ ┌───────────── month (1 - 12)
│ │ │ │ ┌───────────── day of week (0 - 6) (Sunday to Saturday)
│ │ │ │ │
* * * * *
```

### Common Schedules

| Schedule | Expression | Description |
|----------|------------|-------------|
| Every minute | `* * * * *` | Testing/debugging only |
| Every hour | `0 * * * *` | Hourly backups at minute 0 |
| Every 2 hours | `0 */2 * * *` | 12am, 2am, 4am, etc. |
| Every 6 hours | `0 */6 * * *` | 12am, 6am, 12pm, 6pm |
| Every 12 hours | `0 */12 * * *` | 12am and 12pm |
| Daily at midnight | `0 0 * * *` | Once daily at 00:00 |
| Daily at 2am | `0 2 * * *` | Once daily at 02:00 |
| Daily at 6am | `0 6 * * *` | Once daily at 06:00 |
| Twice daily (2am, 2pm) | `0 2,14 * * *` | Every 12 hours offset |
| Every 4 hours (starting at 2am) | `0 2,6,10,14,18,22 * * *` | 2am, 6am, 10am, 2pm, 6pm, 10pm |
| Weekly on Sunday | `0 2 * * 0` | Every Sunday at 02:00 |
| Weekly on Monday | `0 2 * * 1` | Every Monday at 02:00 |
| Weekly on Saturday | `0 2 * * 6` | Every Saturday at 02:00 |
| Bi-weekly (1st and 15th) | `0 2 1,15 * *` | 1st and 15th at 02:00 |
| Monthly (1st) | `0 2 1 * *` | 1st of every month at 02:00 |
| Quarterly | `0 2 1 1,4,7,10 *` | Jan 1, Apr 1, Jul 1, Oct 1 |

### Timezone Considerations

When using non-UTC timezones:

```bash
# Set timezone
SCHEDULER_TIMEZONE=America/New_York

# Backup at 2am Eastern Time
CRON_SCHEDULE="0 2 * * *"
# This runs at 2am ET (7am UTC during EST, 6am UTC during EDT)
```

### Cron Schedule Validation

Common mistakes:

| Incorrect | Correct | Issue |
|-----------|---------|-------|
| `0 24 * * *` | `0 0 * * *` | Hour must be 0-23 |
| `60 * * * *` | `0 * * * *` | Minute must be 0-59 |
| `0 2 * *` | `0 2 * * *` | Missing day of week field |
| `0 2 * * * *` | `0 2 * * *` | Too many fields (6 instead of 5) |

## Timezone Configuration

### Common Timezones

| Timezone | IANA Name | UTC Offset |
|----------|-----------|------------|
| UTC | `UTC` | +0 |
| US Eastern | `America/New_York` | -5/-4 |
| US Central | `America/Chicago` | -6/-5 |
| US Mountain | `America/Denver` | -7/-6 |
| US Pacific | `America/Los_Angeles` | -8/-7 |
| UK | `Europe/London` | +0/+1 |
| Central Europe | `Europe/Paris` | +1/+2 |
| India | `Asia/Kolkata` | +5:30 |
| Singapore | `Asia/Singapore` | +8 |
| Japan | `Asia/Tokyo` | +9 |
| Australia Sydney | `Australia/Sydney` | +10/+11 |

### Setting Timezone

```bash
# UTC (default)
SCHEDULER_TIMEZONE=UTC

# US Eastern
SCHEDULER_TIMEZONE=America/New_York

# Europe/Berlin
SCHEDULER_TIMEZONE=Europe/Berlin
```

### Timezone in Docker

```yaml
# docker-compose.yml
services:
  dbeasebackup:
    environment:
      - SCHEDULER_TIMEZONE=America/New_York
    # Or set container timezone
    environment:
      - TZ=America/New_York
```

## Docker Configuration

### Volume Mounts

```yaml
volumes:
  # Google Drive service account key
  - ./service-account-key.json:/service-account-key.json:ro

  # Custom backup directory (optional)
  - ./backups:/backups

  # Read-only configuration
  - ./config:/config:ro
```

### Network Configuration

**Option 1: Host Network (Simple)**

Use when database is on localhost:

```yaml
services:
  dbeasebackup:
    network_mode: host
```

**Option 2: Docker Network (Recommended)**

Use for production with proper network isolation:

```yaml
services:
  dbeasebackup:
    networks:
      - backend
  networks:
    backend:
      driver: bridge
```

**Option 3: Connect to External Network**

```yaml
services:
  dbeasebackup:
    networks:
      - database_network
networks:
  database_network:
    external: true
```

### Environment Configuration

**Method 1: env_file**

```yaml
services:
  dbeasebackup:
    env_file:
      - .env
```

**Method 2: Direct Environment Variables**

```yaml
services:
  dbeasebackup:
    environment:
      - PG_HOST=postgres
      - PG_PORT=5432
      - PG_USER=${PG_USER}
      - PG_PASSWORD=${PG_PASSWORD}
```

**Method 3: Docker Secrets (Production)**

```yaml
services:
  dbeasebackup:
    secrets:
      - pg_password
secrets:
  pg_password:
    external: true
```

### Resource Limits

```yaml
services:
  dbeasebackup:
    deploy:
      resources:
        limits:
          cpus: '0.5'
          memory: 512M
        reservations:
          cpus: '0.25'
          memory: 256M
```

### Complete Docker Compose Example

```yaml
version: '3.8'

services:
  dbeasebackup:
    image: glennprays/dbeasebackup:latest
    container_name: dbeasebackup
    restart: unless-stopped
    env_file:
      - .env
    volumes:
      - ./service-account-key.json:/service-account-key.json:ro
    networks:
      - backend
    deploy:
      resources:
        limits:
          memory: 512M

networks:
  backend:
    driver: bridge
```

## Configuration Validation

### Automatic Validation

DBEaseBackup validates configuration at startup:

1. **Required variables** - Ensures all required variables are set
2. **Valid values** - Validates enum values (LOG_LEVEL, STORAGE_TYPE, etc.)
3. **Conditional requirements** - Validates storage-specific variables

### Validation Errors

Common validation errors:

```
Error: PG_HOST environment variable is required
Error: CRON_SCHEDULE environment variable is required
Error: invalid LOG_LEVEL: verbose (valid: debug, info, warn, error)
Error: invalid STORAGE_TYPE: dropbox (valid: google-drive, s3)
Error: GOOGLE_DRIVE_FOLDER_ID environment variable is required when STORAGE_TYPE=google-drive
Error: S3_BUCKET environment variable is required when STORAGE_TYPE=s3
```

### Manual Validation

Test your configuration:

```bash
# Start the container and check logs
docker compose up

# Look for validation errors
# If validation passes, you'll see:
# "Starting backup service"
# "Scheduler started"
```

### Debugging Configuration

Enable debug logging to see configuration values:

```bash
LOG_LEVEL=debug
```

This will log (with sensitive values masked):
- Loaded configuration
- Database connection details
- Storage configuration
- Schedule configuration

## Configuration Best Practices

### Development

```bash
ENV=development
LOG_LEVEL=debug
LOG_FORMAT=text
CRON_SCHEDULE="*/5 * * * *"  # Every 5 minutes for testing
```

### Production

```bash
ENV=production
LOG_LEVEL=info
LOG_FORMAT=json
CRON_SCHEDULE="0 2 * * *"  # Daily at 2am
SCHEDULER_TIMEZONE=UTC
```

### Security

1. **Never commit** `.env` files
2. **Use secrets management** in production
3. **Rotate credentials** regularly
4. **Use dedicated** database users with minimal privileges
5. **Restrict** network access

### Performance

1. **Adjust** `BACKUP_TIMEOUT` based on database size
2. **Schedule** backups during low-traffic periods
3. **Monitor** resource usage during backups
4. **Use** appropriate storage class (S3 Standard vs. Glacier)
