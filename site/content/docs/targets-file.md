---
title: Targets file
description: Every field of the YAML or JSON file that lists what to back up, with defaults and validation rules.
---

The targets file lists every database or cluster to back up. Point `TARGETS_FILE` at it. YAML and JSON both work; the format is chosen by the file extension (`.yaml`, `.yml` or `.json`).

```yaml title="targets.yaml"
targets:
  - name: app-db
    mode: dump
    host: db.internal
    port: "5432"
    user: backup
    password_env: APP_DB_PASSWORD
    database: appdb
    schedule: "0 2 * * *"
    timeout: 45m
    retention_days: 30
    verify: true
    retries: 2
    backup_dir: /tmp/dbeasebackup/app-db
    storage:
      folder_id: "1AbCdEfGhIjKlMnOpQrStUvWxYz"   # Google Drive only

  - name: prod-cluster
    mode: basebackup
    host: db.internal
    user: replication
    password_env: REPL_PASSWORD
    schedule: "0 4 * * 0"
    timeout: 2h
    retention_days: 7
    backup_dir: /tmp/dbeasebackup/prod-cluster
```

A complete example lives in the repository as [`targets.example.yaml`](https://github.com/glennprays/dbeasebackup/blob/main/targets.example.yaml).

## Fields

| Field | Required | Default | Description |
| --- | --- | --- | --- |
| `name` | yes | | Unique ID. Must match `^[a-z0-9][a-z0-9-]{0,62}$`. Used in filenames, logs, webhooks and retention. |
| `mode` | no | `dump` | `dump` (one database, `pg_dump`) or `basebackup` (whole cluster, `pg_basebackup`). See [Backup modes](/docs/backup-modes/). |
| `host` | yes | | Server host name or IP. |
| `port` | no | `5432` | Server port. Quote it in YAML to keep it a string. |
| `user` | yes | | Role to connect as. `basebackup` needs the `REPLICATION` attribute. |
| `password` | no | | Literal password. Prefer `password_env`. |
| `password_env` | no | | Name of an environment variable that holds the password. Wins over `password` when the variable is set and not empty. |
| `database` | for `dump` | | Database to dump. Must be empty for `basebackup`, which copies the whole cluster. |
| `sslmode` | no | `disable` | Validated (`disable`, `require`, `verify-ca`, `verify-full`) but not yet passed to the PostgreSQL tools. See the note below. |
| `schedule` | yes | | Cron expression for backups. See [Schedules and timezones](/docs/schedules/). |
| `timeout` | no | `30m` | Go duration, for example `45m` or `2h30m`. Must be positive. |
| `retention_days` | no | `30` | Delete backups older than this many days. `0` keeps everything. Negative values are rejected. |
| `verify` | no | `true` | `dump` only: download the uploaded file and check it with `pg_restore --list`. See [Verification](/docs/verification/). |
| `retries` | no | `2` | Extra attempts after a failed run. `0` disables retries; negative values count as `0`. |
| `backup_dir` | no | `backups/<name>` | Local staging directory. The file is deleted after upload. |
| `storage.folder_id` | no | | Google Drive folder ID for this target's files. |
| `storage.prefix` | no | | Parsed, but not applied yet. Use the global `S3_PREFIX`. |

> [!WARNING]
> `sslmode` is checked at startup but never reaches `pg_dump` or `pg_basebackup`, so libpq falls back to its own default (`prefer`). To require TLS today, set the standard libpq variable `PGSSLMODE=require` (or `verify-full`) on the DBEaseBackup process. The tools inherit its environment, so this applies to every target.

## What the timeout covers

- `dump`: the `pg_dump` command.
- `basebackup`: `pg_basebackup`, `pg_verifybackup` and packing the `.tar.gz`.

Upload and post-upload verification are not covered by `timeout`. When the timeout fires, the partial local file is removed and the attempt fails, which counts toward [retries](/docs/concurrency-and-retries/).

## Validation

The process validates the whole file at startup and exits with an error that names the target and field when something is wrong. It fails when:

- the file cannot be read or parsed, or contains no targets;
- two targets share a `name`, or a name does not match the pattern;
- `schedule` is missing or not a valid cron expression;
- `mode` is not `dump` or `basebackup`;
- `database` is missing for `dump`, or set for `basebackup`;
- `host` or `user` is missing;
- `sslmode` is not one of the four allowed values;
- `retention_days` is negative or `timeout` is not a positive duration;
- the PostgreSQL client tools for a target's mode are not on `PATH`.

## Passwords

Keep passwords out of the file. With `password_env: APP_DB_PASSWORD`, set `APP_DB_PASSWORD` in the container environment (for example from a Docker or Kubernetes secret). The password is handed to the PostgreSQL tools as `PGPASSWORD` in their environment, never on the command line.

## Storage placement

Storage credentials are global (see [Environment variables](/docs/environment-variables/)); placement is per target:

- **Google Drive:** `storage.folder_id` is the parent folder for the target's files. Without it, files go to the service account's own Drive. See [Google Drive](/docs/google-drive/).
- **S3:** all targets share the bucket and the global `S3_PREFIX`. Leave `storage.folder_id` empty for S3 targets; see [S3 and compatible](/docs/s3/) for why.

## Filenames

Each run uploads one file:

| Mode | Filename |
| --- | --- |
| `dump` | `<name>_dump_<YYYYMMDDTHHMMSSZ>.dump` |
| `basebackup` | `<name>_basebackup_<YYYYMMDDTHHMMSSZ>.tar.gz` |

The timestamp is UTC, for example `app-db_dump_20261122T020000Z.dump`. [Retention](/docs/retention/) relies on this format, so do not rename uploaded files.
