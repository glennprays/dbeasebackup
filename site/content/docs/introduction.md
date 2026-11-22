---
title: Introduction
description: What DBEaseBackup does, how one backup run works, and what it deliberately leaves out.
---

DBEaseBackup is a single Go process that backs up databases on a cron schedule and uploads each backup to Google Drive or an S3-compatible bucket. PostgreSQL is the supported engine today; support for more engines is planned. It runs as a container or a plain binary next to your database, keeps no state of its own, and exposes health endpoints for your orchestrator.

## What it does

- **Backs up many targets from one process.** Each target in the [targets file](/docs/targets-file/) has its own connection, schedule, mode, retention and storage placement.
- **Two backup modes.** `dump` runs `pg_dump -F c` against one database. `basebackup` runs `pg_basebackup` against a whole cluster and checks the result with `pg_verifybackup` before anything is uploaded. See [Backup modes](/docs/backup-modes/).
- **Verifies what it uploads.** Dump files are downloaded again after upload and parsed with `pg_restore --list`. See [Verification](/docs/verification/).
- **Deletes old backups for you.** A cleanup job (03:00 daily by default) lists storage, reads the timestamp in each filename and removes files older than `retention_days`. See [Retention](/docs/retention/).
- **Retries and limits concurrency.** Failed runs are retried with backoff, runs of the same target never overlap, and `MAX_PARALLEL_BACKUPS` caps how many targets run at once. See [Concurrency and retries](/docs/concurrency-and-retries/).
- **Reports outcomes.** One signed webhook per finished run, JSON logs with a trace ID per run, and `/health`, `/readyz` and `/livez` endpoints.

## How one run works

1. The scheduler fires the target's cron expression in `SCHEDULER_TIMEZONE`.
2. If the previous run of the same target is still going, this run is skipped with a warning.
3. The run waits for a free slot in the global pool (`MAX_PARALLEL_BACKUPS`).
4. The provider writes the backup to the target's local `backup_dir`:
   - `dump`: `pg_dump -F c` to `<target>_dump_<UTC timestamp>.dump`
   - `basebackup`: `pg_basebackup -Fp -Xs`, then `pg_verifybackup`, then a streamed `<target>_basebackup_<UTC timestamp>.tar.gz`
5. The file is uploaded to storage.
6. In `dump` mode, the uploaded file is downloaded and checked with `pg_restore --list`.
7. The local file is deleted.
8. On failure the run retries (30 s, then 2 min). When it finally succeeds or gives up, one webhook is sent.

Timestamps in filenames use the layout `20060102T150405Z`, for example `app-db_dump_20261122T020000Z.dump`.

## What it does not do

> [!NOTE]
> These limits are intentional. Knowing them up front helps you decide whether DBEaseBackup fits.

- **PostgreSQL only, for now.** Other engines are planned but not available in any release yet.
- **No point-in-time recovery.** There is no continuous WAL archiving. `basebackup` streams only the WAL needed to make that copy consistent.
- **No restore command.** Restores use the standard PostgreSQL tools. See [Restoring](/docs/restoring/).
- **No client-side encryption.** Files are uploaded as produced; rely on the storage provider's encryption at rest. See [Security](/docs/security/).
- **No catalog database.** Storage listing is the source of truth. Renaming uploaded files removes them from retention.
- **No "run now" trigger.** Runs happen only on schedule.

## Next steps

- Follow the [Quick start](/docs/quick-start/) to run your first scheduled backup.
- Read [Installation](/docs/installation/) for image tags, building from source and required client tools.
