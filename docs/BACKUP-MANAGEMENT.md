# Backup Management Guide

This guide explains how to manage, monitor, and restore database backups created by DBEaseBackup.

## Table of Contents

- [How Backups Work](#how-backups-work)
- [Backup File Format](#backup-file-format)
- [Listing Backups](#listing-backups)
- [Downloading Backups](#downloading-backups)
- [Restoring from Backup](#restoring-from-backup)
- [Backup Retention](#backup-retention)
- [Monitoring Backups](#monitoring-backups)

## How Backups Work

### Backup Workflow

DBEaseBackup follows this workflow for each scheduled backup:

```
1. Scheduler triggers backup at scheduled time
         ↓
2. Connect to PostgreSQL database
         ↓
3. Execute pg_dump to create tar-format backup
         ↓
4. Record backup metadata in database_backups table
         ↓
5. Upload backup file to cloud storage (Google Drive or S3)
         ↓
6. Delete local backup file after successful upload
         ↓
7. Log completion and cleanup
```

### Backup Types

DBEaseBackup creates **full database backups** using `pg_dump` in tar format. This includes:

- All tables and data
- Indexes
- Sequences
- Views
- Stored procedures
- Functions
- Triggers
- Schema definitions

### Automatic Features

- **Connection cleanup**: Stale database connections are automatically terminated before backup
- **Timeout handling**: Backups that exceed `BACKUP_TIMEOUT` are terminated
- **Local cleanup**: Temporary backup files are deleted after successful upload
- **Metadata tracking**: Backup information is stored in `database_backups` table

## Backup File Format

### File Format

- **Format**: tar archive (PostgreSQL tar format)
- **Compression**: None by default (can be added with external tools)
- **Compatibility**: Compatible with `pg_restore`

### File Naming Convention

```
<database_name>_<timestamp>.tar
```

Example:
```
production_db_2024-01-15T02-00-00Z.tar
```

### File Size Considerations

- Backup size is approximately equal to database size on disk
- For large databases, ensure sufficient disk space in `BACKUP_DIR`
- Consider increasing `BACKUP_TIMEOUT` for large databases

## Listing Backups

### Via Database Query

Backup metadata is stored in the `database_backups` table:

```sql
-- List all backups
SELECT * FROM database_backups ORDER BY created_at DESC;

-- List backups from last 7 days
SELECT
    id,
    database_name,
    file_path,
    file_size,
    status,
    created_at
FROM database_backups
WHERE created_at >= NOW() - INTERVAL '7 days'
ORDER BY created_at DESC;

-- Count backups by status
SELECT status, COUNT(*) FROM database_backups GROUP BY status;
```

### Via Google Drive

1. Open the Google Drive folder specified in `GOOGLE_DRIVE_FOLDER_ID`
2. Sort by "Last modified" to see recent backups
3. File names include the timestamp for easy identification

### Via S3/AWS CLI

```bash
# List all backups in bucket
aws s3 ls s3://your-backup-bucket/ --recursive

# List recent backups (last 7 days)
aws s3 ls s3://your-backup-bucket/ --recursive | grep "$(date +%Y-%m-%d)"

# Get backup count
aws s3 ls s3://your-backup-bucket/ --recursive | wc -l
```

### Via Google Drive API

```bash
# Using gcloud CLI (requires setup)
gcloud alpha drive files list --folder <FOLDER_ID>
```

## Downloading Backups

### From Google Drive

1. **Manual download**
   - Navigate to your backup folder in Google Drive
   - Right-click on the backup file
   - Select "Download"

2. **Using rclone** (recommended for automation)
   ```bash
   # Install rclone and configure Google Drive
   rclone copy gdrive:backup-folder/production_db_2024-01-15T02-00-00Z.tar ./
   ```

3. **Using Google Drive API**
   ```bash
   # Using gdown or similar tools
   gdown --folder <FOLDER_ID> --remaining-ok
   ```

### From S3

1. **AWS CLI**
   ```bash
   # Download specific backup
   aws s3 cp s3://your-backup-bucket/production_db_2024-01-15T02-00-00Z.tar ./

   # Download all backups
   aws s3 sync s3://your-backup-bucket/ ./backups/
   ```

2. **Presigned URLs** (for temporary access)
   ```bash
   # Generate a presigned URL (valid for 1 hour)
   aws s3 presign s3://your-backup-bucket/production_db_2024-01-15T02-00-00Z.tar --expires-in 3600
   ```

3. **S3 Console**
   - Navigate to bucket in AWS Console
   - Select backup file
   - Click "Download"

## Restoring from Backup

### Prerequisites

- Backup file downloaded from storage
- Target PostgreSQL database (can be empty or existing)
- Sufficient disk space for restoration

### Basic Restore

```bash
# Restore to a new database
pg_restore -h localhost -U postgres -d restored_database backup_file.tar

# Restore with verbose output
pg_restore -h localhost -U postgres -d restored_database -v backup_file.tar

# Restore with specific jobs (parallel restore)
pg_restore -h localhost -U postgres -d restored_database -j 4 backup_file.tar
```

### Complete Restoration Workflow

1. **Download the backup**
   ```bash
   # From S3
   aws s3 cp s3://your-backup-bucket/production_db_2024-01-15T02-00-00Z.tar ./

   # Or from Google Drive (manual download or rclone)
   ```

2. **Create target database**
   ```sql
   -- Connect to PostgreSQL
   psql -U postgres

   -- Create new database
   CREATE DATABASE restored_database;

   -- Exit psql
   \q
   ```

3. **Restore the backup**
   ```bash
   pg_restore -h localhost -U postgres -d restored_database production_db_2024-01-15T02-00-00Z.tar
   ```

4. **Verify restoration**
   ```bash
   # Connect to restored database
   psql -h localhost -U postgres -d restored_database

   # List tables
   \dt

   # Check row counts
   SELECT schemaname, relname, n_live_tup
   FROM pg_stat_user_tables
   ORDER BY n_live_tup DESC;
   ```

### Advanced Restore Options

```bash
# Restore only specific schemas
pg_restore -h localhost -U postgres -d restored_database -n public backup_file.tar

# Restore only specific tables
pg_restore -h localhost -U postgres -d restored_database -t users -t orders backup_file.tar

# Restore with clean (drop existing objects first)
pg_restore -h localhost -U postgres -d restored_database --clean backup_file.tar

# Restore with create database
pg_restore -h localhost -U postgres -d postgres --create backup_file.tar

# Generate SQL script instead of restoring
pg_restore -f restore.sql backup_file.tar
```

### Restore to Different Environment

```bash
# Restore to different server
pg_restore -h production-server -U postgres -d production_db backup_file.tar

# Restore with different owner
pg_restore -h localhost -U postgres -d restored_database --no-owner --no-privileges backup_file.tar
# Then reassign ownership as needed
```

### Troubleshooting Restore Issues

1. **Permission errors**
   ```bash
   # Use superuser account for restore
   pg_restore -h localhost -U postgres -d restored_database backup_file.tar
   ```

2. **Database exists errors**
   ```bash
   # Drop and recreate database
   DROP DATABASE IF EXISTS restored_database;
   CREATE DATABASE restored_database;
   ```

3. **Version incompatibility**
   - Backup format is compatible across PostgreSQL versions
   - For major version upgrades, test restore on a staging environment first

## Backup Retention

### Recommended Retention Policy

| Backup Frequency | Retention Period | Use Case |
|-----------------|------------------|----------|
| Hourly | 24-48 hours | Point-in-time recovery for recent changes |
| Daily | 30 days | Standard recovery window |
| Weekly | 3 months | Monthly reporting, audits |
| Monthly | 12 months | Compliance, long-term archives |

### Implementing Retention

**S3 Lifecycle Policies:**
```bash
# Create lifecycle policy
aws s3api put-bucket-lifecycle-configuration \
  --bucket your-backup-bucket \
  --lifecycle-configuration file://lifecycle.json
```

`lifecycle.json`:
```json
{
  "Rules": [
    {
      "ID": "DeleteOldBackups",
      "Status": "Enabled",
      "Filter": {},
      "Expiration": {
        "Days": 90
      }
    },
    {
      "ID": "ArchiveOldBackups",
      "Status": "Enabled",
      "Filter": {},
      "Transitions": [
        {
          "Days": 30,
          "StorageClass": "STANDARD_IA"
        },
        {
          "Days": 90,
          "StorageClass": "GLACIER"
        }
      ]
    }
  ]
}
```

**Google Drive:**
- Manual cleanup recommended
- Use Google Apps Script for automation
- Consider third-party tools for lifecycle management

### Manual Cleanup

```bash
# List and delete old backups from S3
aws s3 ls s3://your-backup-bucket/ --recursive | \
  awk '{print $4}' | \
  xargs -I {} aws s3 rm s3://your-backup-bucket/{}

# Or use lifecycle policies instead
```

## Monitoring Backups

### Log Monitoring

Enable debug logging for detailed monitoring:

```bash
LOG_LEVEL=info
LOG_FORMAT=json
```

Key log events to monitor:
- "Starting backup service" - Application started
- "Scheduler started" - Cron scheduler running
- "Executing backup job" - Backup triggered
- "Database backup created" - pg_dump successful
- "Backup uploaded successfully" - Upload complete
- "Backup job completed" - Full workflow complete

### Monitoring Commands

```bash
# Check recent backups via logs
docker logs dbeasebackup 2>&1 | grep "Backup job completed"

# Check for errors
docker logs dbeasebackup 2>&1 | grep -i error

# Count successful backups today
docker logs dbeasebackup 2>&1 | grep "$(date +%Y-%m-%d)" | grep "Backup uploaded" | wc -l
```

### Alerting Recommendations

Set up alerts for:

1. **Backup failures**
   - Monitor logs for "error" or "failed" messages
   - Alert on non-zero exit codes

2. **Missed schedules**
   - Alert if no backup completed within expected window
   - Example: If daily backup at 2 AM, alert if no backup by 3 AM

3. **Storage issues**
   - Alert on upload failures
   - Monitor storage quota

4. **Backup size anomalies**
   - Alert if backup size changes dramatically
   - Could indicate data loss or corruption

### Health Check Script

```bash
#!/bin/bash
# check-backups.sh - Verify recent backups exist

# Check for backups in last 25 hours (allows for daily backup)
BACKUP_COUNT=$(aws s3 ls s3://your-backup-bucket/ --recursive | \
  grep "$(date -d '1 day ago' +%Y-%m-%d)" | wc -l)

if [ "$BACKUP_COUNT" -eq 0 ]; then
  echo "WARNING: No backups found in the last 24 hours"
  exit 1
else
  echo "OK: $BACKUP_COUNT backup(s) found"
  exit 0
fi
```

### Integration with Monitoring Tools

**Prometheus metrics** (if implemented):
```
dbeasebackup_backups_total{status="success"}
dbeasebackup_backups_total{status="failure"}
dbeasebackup_backup_size_bytes
dbeasebackup_backup_duration_seconds
```

**Health check endpoint** (future enhancement):
```bash
curl http://localhost:8080/health
# Returns: {"status": "healthy", "last_backup": "2024-01-15T02:00:00Z"}
```
