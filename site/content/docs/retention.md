---
title: Retention
description: How the cleanup job finds and deletes old backups, using storage itself as the source of truth.
---

Each target keeps its backups for `retention_days` (default `30`). A cleanup job per target deletes older files. There is no catalog database: the job lists storage and reads the age of each file from its name.

## How cleanup works

On `CLEANUP_CRON_SCHEDULE` (default `0 3 * * *`), for each target:

1. If `retention_days` is `0`, stop. The target keeps everything.
2. List storage for files whose name starts with `<name>_`:
   - Google Drive: inside `storage.folder_id` when set, otherwise across the service account's files.
   - S3: under `S3_PREFIX/` in the bucket.
3. Parse the UTC timestamp from each name, for example `20261122T020000Z` in `app-db_dump_20261122T020000Z.dump`.
4. Delete every file whose timestamp is older than now minus `retention_days` days.

Details:

- Age comes from the **filename**, not the storage object's modification time. Copying files between buckets keeps their age.
- Files whose name does not parse are skipped with a warning. Renamed files are therefore kept forever.
- A file already gone is logged and skipped. Other delete errors are logged and the job moves on to the next file.
- Cleanup does not send webhooks. Its results are in the logs (`Backup cleanup completed` with `deleted_count` and `skipped_count`).
- Cleanup runs even when the latest backup failed. With a short retention and a long run of failures you can end up with no backups; alert on failed runs (see [Webhook notifications](/docs/webhooks/)).

## Choosing a window

| Schedule | Suggested `retention_days` | Copies kept |
| --- | --- | --- |
| Hourly dumps | 2 | About 48 |
| Daily dumps | 30 | About 30 |
| Weekly basebackup | 28–56 | 4–8 |

Keep at least a few copies older than your slowest way of noticing data loss.

## Storage-side lifecycle rules

You can combine retention with bucket lifecycle rules, for example moving objects older than 30 days to a cheaper storage class. Set `retention_days` longer than any rule that deletes objects, or set it to `0` and let the bucket expire objects.

## Limits to know

> [!WARNING]
> **Google Drive lists one page.** The listing reads a single page of results (the Drive API default of 100 files) and does not paginate. A folder with more than 100 matching files may leave older ones behind. Use one folder per target and a retention window that keeps the count under 100.

> [!WARNING]
> **Google Drive does not match by exact prefix.** The listing query is `name contains '<name>_'`, and Drive decides which names match by splitting them into terms. Cleanup then trusts any file whose name ends in a valid timestamp, so in a shared folder one target's cleanup could pick up another target's files. Give each target its own `storage.folder_id`.

> [!NOTE]
> **S3 prefix format.** Write `S3_PREFIX` without a trailing slash (`backups/postgres`, not `backups/postgres/`). Uploads normalise the path but the cleanup listing does not, so a trailing slash makes cleanup find nothing.

Deletes on Google Drive are permanent: the API call bypasses the trash.
