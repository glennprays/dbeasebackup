<p align="center">
  <img src="../img/logo.png" alt="Logo" width="200"/>

  <h1 align="center">DBEaseBackup</h1>
</p>

## Get Started

DBEaseBackup automates database backups to cloud storage using scheduled cron jobs.

 This guide will help you set up DBEaseBackup quickly with either Google Drive or S3 storage.

### Quick Links

- [Google Drive Setup](#google-drive-setup) - Recommended for Google Drive users
- [S3 Setup](#s3-setup) - Alternative storage option
- [Docker Compose Examples](#docker-compose-examples) - Ready-to-use configurations

---

---

## Google Drive Setup

### Google Service Account

To utilize Google Drive for storage, you need to set up a **Google Service Account Key** (JSON version):

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Navigate to **IAM & Admin > Service Accounts**
3. Create a new service account or use an existing one)
4. Click on the service account, then click **Keys** tab
5. Click **Add Key** > **Create new key**
6. Select **JSON** as the key type
7. Click **Create** and download the JSON file
8. Rename the downloaded file to `service-account-key.json`

### Google Drive Folder

Obtain the ID of the Google Drive folder and configure it for the Google Service Account:

1. Create a target folder in Google Drive for your backups
2. Share the folder with the service account:
   - Open the `service-account-key.json` file
   - Find the `client_email` value
   - Right-click on the Google Drive folder
   - Click **Share**
   - Paste the `client_email` address
   - Grant **Editor** permissions
   - Click **Send**
3. Copy the folder ID:
   - Open the Google Drive folder in your browser
   - The URL will look like: `https://drive.google.com/drive/folders/FOLDER_ID_HERE`
   - Copy only the `FOLDER_ID_HERE` part

 your `GOOGLE_DRIVE_FOLDER_ID` environment variable

### Environment Variables

Copy the `.env.example` file and configure your settings:

```bash
cp .env.example .env
```

Edit `.env` with your configuration:

```bash
# Required
PG_HOST=your_postgres_host
PG_PORT=5432
PG_USER=postgres
PG_PASSWORD=your_password
PG_DATABASE=your_database
CRON_SCHEDULE="0 2 * * *"  # Daily at 2am
GOOGLE_DRIVE_FOLDER_ID=your_folder_id_here

```

### Running DBEaseBackup

After configuring all steps above, run DBEaseBackup with Docker Compose:

```bash
docker compose up -d
```

---

## S3 Setup

S3 is an alternative storage backend that works with AWS S3, MinIO, DigitalOcean Spaces, Backblaze B2, or any S3-compatible service.

### Configuration

Set environment variables in `.env`:

```bash
# Required
STORAGE_TYPE=s3
S3_BUCKET=your-bucket-name
S3_REGION=us-east-1
S3_ACCESS_KEY_ID=your-access-key-id
S3_SECRET_ACCESS_KEY=your-secret-access-key

CRON_SCHEDULE="0 2 * * *"
PG_HOST=your_postgres_host
PG_PORT=5432
PG_USER=postgres
PG_PASSWORD=your_password
PG_DATABASE=your_database
```

For S3-compatible services (MinIO, DigitalOcean Spaces, etc.), also set the endpoint:

```bash
S3_ENDPOINT=https://your-s3-endpoint
```

### Docker Compose

Use the `docker-compose.s3.yml` file:

```bash
# Copy S3 compose file
cp docker-compose.s3.yml docker-compose.yml

# Copy environment file
cp .env.example .env

# Edit .env with your settings
docker compose up -d
```

---

## Docker Compose Examples

### Google Drive

```yaml
services:
  db-auto-backup:
    image: glennprays/dbeasebackup:latest
    container_name: dbeasebackup-container
    env_file:
      - .env
    volumes:
      - ./service-account-key.json:/service-account-key.json
    network_mode: host
```

### S3

```yaml
services:
  db-auto-backup:
    image: glennprays/dbeasebackup:latest
    container_name: dbeasebackup-container
    environment:
      - STORAGE_TYPE=s3
      - S3_BUCKET=your-bucket
      - S3_REGION=us-east-1
      - S3_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID}
      - S3_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY}
      - PG_HOST=postgres
      - PG_PORT=5432
      - PG_USER=postgres
      - PG_PASSWORD=example
      - PG_DATABASE=mydb
      - CRON_SCHEDULE=0 2 * * *
    network_mode: host
```

---

## Troubleshooting

### Common Docker Issues

**Container exits immediately**
- Check logs for configuration errors: `docker logs dbeasebackup-container`
- Verify all required environment variables are set

**Can't connect to database**
- Ensure PostgreSQL is accessible from the container
- Use `network_mode: host` if PostgreSQL is on the same machine
- Or use a Docker network with the PostgreSQL container

**Permission denied on service-account-key.json**
- Verify the file exists: `ls -la service-account-key.json`
- Check file permissions

**Schedule not running**
- Check cron expression format in `CRON_SCHEDULE`
- Verify `SCHEDULER_TIMEZONE` is valid

---

## Additional Resources

- [Main README](../README.md) - Full documentation
- [docs/CONFIGURATION.md](../docs/CONFIGURATION.md) - Detailed configuration reference
- [docs/TROUBLESHOOTING.md](../docs/TROUBLESHOOTING.md) - Troubleshooting guide
- [docs/SECURITY.md](../docs/SECURITY.md) - Security best practices
