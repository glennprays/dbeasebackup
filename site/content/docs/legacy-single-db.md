---
title: Legacy single-DB mode
description: Back up one database with flat PG_* environment variables instead of a targets file.
---

Before targets files existed, DBEaseBackup was configured with flat environment variables for one database. That still works: when `TARGETS_FILE` is unset and `PG_HOST` is set, the process builds a single `dump` target named `default` from those variables.

This is also the only configuration that the `v2.2.0` image and older understand.

## Example

```bash title=".env"
PG_HOST=db.internal
PG_PORT=5432
PG_USER=backup
PG_PASSWORD=change-me
PG_DATABASE=appdb
CRON_SCHEDULE="0 2 * * *"
BACKUP_DIR=/tmp/dbeasebackup
BACKUP_RETENTION_DAYS=30

STORAGE_TYPE=google-drive
GOOGLE_DRIVE_FOLDER_ID=1AbCdEfGhIjKlMnOpQrStUvWxYz
```

## How the variables map

| Variable | Target field | Notes |
| --- | --- | --- |
| | `name: default` | Fixed. Files are named `default_dump_<UTC timestamp>.dump`. |
| | `mode: dump` | Legacy mode cannot run `basebackup`. |
| `PG_HOST` | `host` | Required. |
| `PG_PORT` | `port` | Default `5432`. |
| `PG_USER` | `user` | Required. |
| `PG_PASSWORD` | `password` | |
| `PG_DATABASE` | `database` | Required. |
| `PG_SSLMODE` | `sslmode` | Validated but not applied; use `PGSSLMODE`. |
| `CRON_SCHEDULE` | `schedule` | Required. |
| `BACKUP_TIMEOUT` | `timeout` | Default `30m`. |
| `BACKUP_RETENTION_DAYS` | `retention_days` | Default `30`. |
| `BACKUP_VERIFY` | `verify` | Default `true`. |
| `BACKUP_DIR` | `backup_dir` | Default `backups/postgres`. |
| `GOOGLE_DRIVE_FOLDER_ID` | `storage.folder_id` | |

Retries use the default of 2. Startup fails with a clear message when `PG_HOST` is set but `PG_USER` or `CRON_SCHEDULE` is missing, and when neither `TARGETS_FILE` nor `PG_HOST` is set.

## Cleanup schedule

In legacy mode, retention cleanup runs on `CLEANUP_CRON_SCHEDULE` if set, otherwise on `CRON_SCHEDULE`, so cleanup and backup start at the same minute. With a targets file the fallback is `0 3 * * *` instead.

## Moving to a targets file

1. Write a `targets.yaml` with one target that copies your current values. Keep `name: default` if you want retention to keep managing the files that already exist, because cleanup only looks at files starting with `<name>_`.
2. Move the password into `password_env`.
3. Set `TARGETS_FILE` and remove the `PG_*`, `CRON_SCHEDULE` and `BACKUP_*` variables.
4. Restart. The startup log lists each configured target.
