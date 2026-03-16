# Troubleshooting Guide

This guide helps you diagnose and resolve common issues with DBEaseBackup.

## Table of Contents

- [Common Issues](#common-issues)
  - [Database Connection Issues](#database-connection-issues)
  - [pg_dump Not Found](#pg_dump-not-found)
  - [Backup Hanging/Timeout](#backup-hangingtimeout)
  - [Google Drive Upload Failures](#google-drive-upload-failures)
  - [S3 Upload Failures](#s3-upload-failures)
  - [Cron Schedule Not Working](#cron-schedule-not-working)
- [Viewing Logs](#viewing-logs)
- [Health Checks](#health-checks)

## Common Issues

### Database Connection Issues

**Symptoms:**
- Application fails to start
- Error messages about connection refused or timeout
- "password authentication failed" errors

**Possible Causes & Solutions:**

1. **Incorrect credentials**
   ```bash
   # Verify your credentials by connecting manually
   psql -h $PG_HOST -p $PG_PORT -U $PG_USER -d $PG_DATABASE
   ```

2. **PostgreSQL not accepting connections**
   - Check `postgresql.conf` for `listen_addresses`
   - Check `pg_hba.conf` for allowed connection types
   - Restart PostgreSQL after changes

3. **Network connectivity issues**
   - Verify the host is reachable: `ping $PG_HOST`
   - Check if the port is open: `nc -zv $PG_HOST $PG_PORT`
   - For Docker, ensure `--network host` is used or proper network configuration

4. **Docker-specific issues**
   ```bash
   # Use host network to access localhost services
   docker run --network host ...

   # Or use host.docker.internal for Docker Desktop
   PG_HOST=host.docker.internal
   ```

### pg_dump Not Found

**Symptoms:**
- Error: "pg_dump: command not found"
- Backup fails immediately

**Solutions:**

Install PostgreSQL client tools:

**macOS:**
```bash
brew install postgresql
```

**Ubuntu/Debian:**
```bash
sudo apt-get update
sudo apt-get install postgresql-client
```

**CentOS/RHEL:**
```bash
sudo yum install postgresql
```

**Alpine Linux (Docker):**
```bash
apk add --no-cache postgresql-client
```

**Verify installation:**
```bash
pg_dump --version
```

### Backup Hanging/Timeout

**Symptoms:**
- Backup process hangs indefinitely
- No output or progress
- Container becomes unresponsive

**Causes & Solutions:**

1. **Large database exceeding timeout**
   ```bash
   # Increase timeout for large databases
   BACKUP_TIMEOUT=2h  # 2 hours
   BACKUP_TIMEOUT=4h30m  # 4 hours 30 minutes
   ```

2. **Stale database connections**

   DBEaseBackup automatically handles stale connections. If you're running an older version, upgrade to the latest version which includes automatic connection cleanup.

   To manually check for stale connections:
   ```sql
   SELECT * FROM pg_stat_activity
   WHERE state = 'idle'
   AND query_start < NOW() - INTERVAL '1 hour';
   ```

3. **Long-running queries blocking backup**
   ```sql
   -- Check for blocking queries
   SELECT pid, now() - pg_stat_activity.query_start AS duration, query, state
   FROM pg_stat_activity
   WHERE (now() - pg_stat_activity.query_start) > interval '5 minutes';

   -- Terminate if necessary (be careful!)
   SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE pid = <blocking_pid>;
   ```

4. **Insufficient disk space**
   ```bash
   # Check available disk space
   df -h

   # Ensure BACKUP_DIR has enough space for the backup
   ```

### Google Drive Upload Failures

**Symptoms:**
- "Unable to upload file" errors
- "Permission denied" errors
- "Folder not found" errors

**Solutions:**

1. **Invalid service account key**
   ```bash
   # Verify the JSON key file is valid
   cat service-account-key.json | jq .

   # Ensure it contains client_email and private_key fields
   ```

2. **Folder not shared with service account**
   - Open Google Drive folder
   - Click Share
   - Add the `client_email` from your service account JSON
   - Grant Editor permissions

3. **Incorrect folder ID**
   ```bash
   # Folder ID is in the URL
   # https://drive.google.com/drive/folders/FOLDER_ID_HERE
   #
   # Copy only the FOLDER_ID_HERE part
   ```

4. **API quota exceeded**
   - Check Google Cloud Console for API quotas
   - Google Drive API has daily limits
   - Consider reducing backup frequency

5. **Network issues**
   - Verify internet connectivity
   - Check if firewall blocks Google APIs

### S3 Upload Failures

**Symptoms:**
- "Access Denied" errors
- "Bucket not found" errors
- Upload timeouts

**Solutions:**

1. **Invalid credentials**
   ```bash
   # Test credentials with AWS CLI
   aws s3 ls --region $S3_REGION

   # Or test directly
   aws s3 ls s3://$S3_BUCKET
   ```

2. **Insufficient permissions**

   Ensure the IAM user/role has these permissions:
   ```json
   {
     "Effect": "Allow",
     "Action": [
       "s3:PutObject",
       "s3:GetObject",
       "s3:ListBucket",
       "s3:DeleteObject"
     ],
     "Resource": [
       "arn:aws:s3:::your-bucket",
       "arn:aws:s3:::your-bucket/*"
     ]
   }
   ```

3. **Incorrect region**
   ```bash
   # Verify bucket region
   aws s3api get-bucket-location --bucket $S3_BUCKET
   ```

4. **S3-compatible storage issues**
   ```bash
   # For MinIO, DigitalOcean Spaces, etc.
   # Ensure endpoint is correct
   S3_ENDPOINT=https://your-endpoint.com

   # Example for MinIO
   S3_ENDPOINT=http://minio:9000

   # Example for DigitalOcean Spaces
   S3_ENDPOINT=https://nyc3.digitaloceanspaces.com
   ```

5. **Bucket doesn't exist**
   ```bash
   # Create bucket if needed
   aws s3 mb s3://your-bucket --region $S3_REGION
   ```

### Cron Schedule Not Working

**Symptoms:**
- Backups don't run at expected times
- Backups run at wrong times

**Solutions:**

1. **Incorrect cron expression**

   Verify your cron expression:
   ```
   Minute Hour Day Month Weekday
     *     *    *     *      *

   Examples:
   "0 2 * * *"     - Daily at 2:00 AM
   "0 */6 * * *"   - Every 6 hours
   "30 3 * * 1"    - Every Monday at 3:30 AM
   ```

2. **Wrong timezone**
   ```bash
   # Set correct timezone
   SCHEDULER_TIMEZONE=America/New_York
   SCHEDULER_TIMEZONE=Europe/London
   SCHEDULER_TIMEZONE=Asia/Tokyo
   ```

3. **Application not running**
   ```bash
   # Check if container is running
   docker ps | grep dbeasebackup

   # Check logs
   docker logs dbeasebackup
   ```

## Viewing Logs

### Enable Debug Logging

For troubleshooting, enable debug mode:

```bash
LOG_LEVEL=debug
LOG_FORMAT=json  # Optional: easier to parse
```

### Docker Logs

```bash
# View recent logs
docker logs dbeasebackup

# Follow logs in real-time
docker logs -f dbeasebackup

# View last 100 lines
docker logs --tail 100 dbeasebackup
```

### Log Structure

Logs include trace IDs for tracking request flows:

```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "level": "info",
  "message": "Database backup created",
  "service": "dbeasebackup",
  "env": "production",
  "trace_id": "abc123",
  "metadata": {
    "path": "backups/postgres/mydb_20240115.tar",
    "size": 1048576
  }
}
```

### Common Log Messages

| Message | Meaning |
|---------|---------|
| "Starting backup service" | Application started successfully |
| "Scheduler started" | Cron scheduler is running |
| "Executing backup job" | Backup job triggered |
| "Database backup created" | pg_dump completed |
| "Backup uploaded successfully" | Upload to storage completed |
| "Connection refused" | Cannot connect to database |

## Health Checks

### Verify Database Connectivity

```bash
# From inside the container
docker exec dbeasebackup pg_isready -h $PG_HOST -p $PG_PORT

# Or use the application's ping
# Check logs for "Ping successful" messages
```

### Verify Storage Access

**Google Drive:**
```bash
# Test service account access
gcloud auth activate-service-account --key-file=service-account-key.json
```

**S3:**
```bash
# Test write access
echo "test" | aws s3 cp - s3://$S3_BUCKET/test.txt
aws s3 rm s3://$S3_BUCKET/test.txt
```

### Check Application Status

```bash
# Container running?
docker ps

# Exit codes
docker inspect dbeasebackup --format='{{.State.ExitCode}}'

# Resource usage
docker stats dbeasebackup
```

## Still Having Issues?

1. **Check the logs** with debug level enabled
2. **Verify all configuration** is correct
3. **Test components individually** (database, storage)
4. **Open an issue** on [GitHub](https://github.com/glennprays/dbeasebackup/issues) with:
   - Full error message
   - Relevant log output (with sensitive data removed)
   - Your configuration (with sensitive data removed)
   - Docker version and platform
