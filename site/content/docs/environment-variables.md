---
title: Environment variables
description: Global settings for storage, scheduling, logging, health checks and webhooks, with their defaults.
---

Global settings come from environment variables. Per-target settings (connection, schedule, mode, retention) live in the [targets file](/docs/targets-file/). Variables marked *legacy* are read only when `TARGETS_FILE` is unset; see [Legacy single-DB mode](/docs/legacy-single-db/).

## Application

| Variable | Default | Description |
| --- | --- | --- |
| `ENV` | `production` | `production` skips reading `.env`. When `ENV` is unset, `local` or `development`, a `.env` file in the working directory is loaded first. The logger accepts `dev`, `development`, `staging`, `prod` and `production`; `ENV=local` makes startup fail, so use `development` locally. |
| `TARGETS_FILE` | | Path to the targets file (YAML or JSON). When unset, the legacy variables define a single target named `default`. |
| `MAX_PARALLEL_BACKUPS` | `1` | How many targets may back up at the same time. `0` behaves like `1`; negative values are rejected. See [Concurrency and retries](/docs/concurrency-and-retries/). |

## Scheduling

| Variable | Default | Description |
| --- | --- | --- |
| `SCHEDULER_TIMEZONE` | `UTC` | IANA timezone for every cron expression, for example `Europe/Berlin`. An unknown zone stops startup. |
| `CLEANUP_CRON_SCHEDULE` | `0 3 * * *` | When the retention cleanup runs for every target. In legacy mode the fallback is `CRON_SCHEDULE` instead. |

## Storage

| Variable | Default | Description |
| --- | --- | --- |
| `STORAGE_TYPE` | `google-drive` | `google-drive` or `s3`. One backend serves all targets. |

### Google Drive

| Variable | Default | Description |
| --- | --- | --- |
| `GOOGLE_DRIVE_KEY_FILE` | `service-account-key.json` | Path to the service account JSON key, relative to the working directory (`/` in the image). |
| `GOOGLE_DRIVE_FOLDER_ID` | | *Legacy.* Folder for the `default` target. With a targets file, use `storage.folder_id` per target. |

### S3 and compatible

| Variable | Default | Description |
| --- | --- | --- |
| `S3_BUCKET` | | Bucket name. Must exist; `/health` reports an error otherwise. |
| `S3_REGION` | `us-east-1` | Bucket region. |
| `S3_ACCESS_KEY_ID` | | Access key. |
| `S3_SECRET_ACCESS_KEY` | | Secret key. |
| `S3_ENDPOINT` | | **Required**, also for AWS (`https://s3.<region>.amazonaws.com`). Empty makes startup fail. An `https://` prefix turns TLS on; `http://` or no scheme turns it off. See [S3 and compatible](/docs/s3/). |
| `S3_PREFIX` | | Key prefix for every object, for example `backups/postgres`. |

## Logging

| Variable | Default | Description |
| --- | --- | --- |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`. |
| `LOG_FORMAT` | `text` | Validated (`json` or `text`), but output is currently always JSON. See [Logging](/docs/logging/). |

## Health endpoint

| Variable | Default | Description |
| --- | --- | --- |
| `HTTP_PORT` | `8080` | Port for `/health`, `/readyz` and `/livez`. The server always starts; `0` binds a random free port rather than disabling it. |

## Webhooks

| Variable | Default | Description |
| --- | --- | --- |
| `WEBHOOK_URL` | | Endpoint that receives a POST after every finished backup run. Empty disables webhooks. |
| `WEBHOOK_SECRET` | | When set, each request carries `X-Signature-256: sha256=<HMAC-SHA256 of the body>`. |
| `WEBHOOK_TIMEOUT` | `10s` | Request timeout as a Go duration. Invalid values fall back to `10s`. |

See [Webhook notifications](/docs/webhooks/) for the payload.

## Legacy single-database

Read only when `TARGETS_FILE` is unset. Details in [Legacy single-DB mode](/docs/legacy-single-db/).

| Variable | Default | Description |
| --- | --- | --- |
| `PG_HOST` | | Server host. Setting it turns legacy mode on. |
| `PG_PORT` | `5432` | Server port. |
| `PG_USER` | | Role to connect as. Required. |
| `PG_PASSWORD` | | Password. |
| `PG_DATABASE` | | Database to dump. Required. |
| `PG_SSLMODE` | `disable` | Validated but not applied, like `sslmode`. Use `PGSSLMODE`. |
| `CRON_SCHEDULE` | | Backup schedule. Required. |
| `BACKUP_DIR` | `backups/postgres` | Local staging directory. |
| `BACKUP_TIMEOUT` | `30m` | Timeout for `pg_dump`. |
| `BACKUP_RETENTION_DAYS` | `30` | Retention in days; `0` keeps everything. |
| `BACKUP_VERIFY` | `true` | Verify the upload with `pg_restore --list`. |

## libpq variables

The PostgreSQL tools inherit the process environment, so standard libpq variables apply to every target. The useful one today is `PGSSLMODE` (for example `require` or `verify-full`), because `sslmode` is not yet passed through. DBEaseBackup sets `PGPASSWORD` itself for each command.

> [!NOTE]
> `BACKUP_PROVIDER` appears in older examples. Nothing reads it; the backup tool is chosen by each target's `mode`.
