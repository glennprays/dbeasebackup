---
title: Quick start
description: Run DBEaseBackup in Docker with a targets file and get your first scheduled backup into S3 or Google Drive.
---

This guide runs the official image with one `dump` target. It takes about ten minutes. You need Docker, a reachable PostgreSQL server, and either an S3 bucket or a Google Drive folder shared with a service account.

> [!NOTE]
> `TARGETS_FILE` arrives in the first release after `v2.2.0`. On `v2.2.0` and older, use [Legacy single-DB mode](/docs/legacy-single-db/) instead.

## 1. Write a targets file

Create `targets.yaml`. Each entry is one backup target.

```yaml title="targets.yaml"
targets:
  - name: app-db                      # lower-case letters, digits, hyphens
    mode: dump
    host: db.internal
    port: "5432"
    user: backup
    password_env: APP_DB_PASSWORD     # read the password from this env var
    database: appdb
    schedule: "0 2 * * *"             # daily at 02:00 (SCHEDULER_TIMEZONE)
    retention_days: 30
    backup_dir: /tmp/dbeasebackup/app-db
```

> [!WARNING]
> Keep `backup_dir` under `/tmp` (or a mounted volume) when you use the official image. The image runs as the unprivileged user `appuser` (uid 100) with `/` as its working directory, so the default `backups/<name>` is not writable and every run fails with `permission denied`.

Every field is described in [Targets file](/docs/targets-file/).

## 2. Write an env file

Create `.env` for the global settings and secrets. Pick one storage backend.

```bash title=".env (S3)"
TARGETS_FILE=/config/targets.yaml
APP_DB_PASSWORD=change-me

STORAGE_TYPE=s3
S3_ENDPOINT=https://s3.us-east-1.amazonaws.com   # required; your MinIO/R2/etc. URL for other services
S3_BUCKET=my-db-backups
S3_REGION=us-east-1
S3_ACCESS_KEY_ID=AKIA...
S3_SECRET_ACCESS_KEY=...
```

```bash title=".env (Google Drive)"
TARGETS_FILE=/config/targets.yaml
APP_DB_PASSWORD=change-me

STORAGE_TYPE=google-drive
GOOGLE_DRIVE_KEY_FILE=/config/service-account-key.json
```

For Google Drive, also add the folder to the target, otherwise files land in the service account's own Drive where you cannot see them:

```yaml
    storage:
      folder_id: "1AbCdEfGhIjKlMnOpQrStUvWxYz"
```

[Google Drive](/docs/google-drive/) explains how to create the service account and share the folder.

## 3. Start the container

```bash
docker run -d --name dbeasebackup \
  --restart unless-stopped \
  --env-file .env \
  -v "$(pwd)/targets.yaml:/config/targets.yaml:ro" \
  -p 8080:8080 \
  glennprays/dbeasebackup:latest
```

For Google Drive, mount the key as well with `-v "$(pwd)/service-account-key.json:/config/service-account-key.json:ro"`. The file must be readable by uid 100 inside the container.

At startup the process validates every target and checks that `pg_dump` and `pg_restore` are on `PATH`. If anything is wrong it logs the reason and exits, so check the logs first:

```bash
docker logs dbeasebackup
```

## 4. Check health

```bash
curl -s localhost:8080/health
```

```json
{
  "healthy": true,
  "targets": [
    {
      "target": "app-db",
      "last_run": "0001-01-01T00:00:00Z",
      "last_success": "0001-01-01T00:00:00Z",
      "last_error": ""
    }
  ]
}
```

`healthy` reflects storage reachability. The zero timestamps mean the target has not run yet. See [Health checks](/docs/health-checks/).

## 5. Wait for the first run

There is no "run now" command; the first backup happens at the next matching cron time. To see a run quickly, temporarily set `schedule: "*/5 * * * *"` and restart the container. A successful run logs `Backup workflow completed successfully` and the file appears in storage as `app-db_dump_<UTC timestamp>.dump`.

## Next steps

- Add more targets, or a `basebackup` target for a whole cluster: [Backup modes](/docs/backup-modes/).
- Get a webhook on every finished run: [Webhook notifications](/docs/webhooks/).
- Practise a restore before you need one: [Restoring](/docs/restoring/).
