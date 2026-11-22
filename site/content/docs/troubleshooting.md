---
title: Troubleshooting
description: Error messages you may see at startup, during a run and during cleanup, with their causes and fixes.
---

Start with the logs. Startup errors are printed before the process exits; run errors carry the target name and a `trace_id`.

```bash
docker logs dbeasebackup 2>&1 | tail -50
```

## Startup

| Message | Cause | Fix |
| --- | --- | --- |
| `Failed to resolve backup targets: no targets configured: provide TARGETS_FILE or legacy PG_* env vars` | Neither `TARGETS_FILE` nor `PG_HOST` is set | Set one of them. |
| `unable to read targets file ...` | Wrong path or the file is not mounted | Check the volume mount and `TARGETS_FILE`. |
| `target "x": name must match ...`, `schedule is required`, `database is required when mode=dump` | Invalid targets file | See [Targets file](/docs/targets-file/) for every rule. |
| `Failed to initialize logger: invalid config: environment must be one of: dev, development, staging, prod, production (got: local)` | `ENV=local` | Use `ENV=development`. |
| `pg_dump not found in PATH` / `pg_verifybackup not found in PATH` | Client tools missing (binary install) | Install PostgreSQL client tools; `pg_verifybackup` needs 13 or later. |
| `Failed to initialize storage` with `Endpoint:  does not follow ip address or domain name standards` | `S3_ENDPOINT` is empty | Set it, also for AWS. See [S3 and compatible](/docs/s3/). |
| `invalid timezone ...` | Unknown `SCHEDULER_TIMEZONE` | Use an IANA name such as `Europe/Berlin`. |
| `Failed to load configuration: invalid STORAGE_TYPE: ... (valid: google-drive, s3)` | Typo in `STORAGE_TYPE` | Use `google-drive` or `s3`. |

## Every run fails

| Message | Cause | Fix |
| --- | --- | --- |
| `unable to create backup directory: mkdir backups: permission denied` | The image's working directory `/` is not writable for uid 100 | Set `backup_dir` under `/tmp` or mount a writable volume. |
| `password authentication failed for user` | Wrong password, or `password_env` names a variable that is unset (then the literal `password` is used) | Check the variable inside the container: `docker exec dbeasebackup env`. |
| `no pg_hba.conf entry for host` | The server does not accept this host | Add a `host` line for the database (dump) or a `host replication` line (basebackup). |
| `aborting because of server version mismatch` | `pg_dump` is older than the server | Use client tools of the server's major version or newer. The image ships 16. |
| `permission denied for table ...` | The role cannot read everything | See [Security](/docs/security/) for grants. |
| `number of requested standby connections exceeds max_wal_senders` | basebackup needs a WAL sender slot | Raise `max_wal_senders` on the server. |
| `backup cancelled: context deadline exceeded` | The run took longer than `timeout` | Raise `timeout`, or schedule the run when the server is quieter. |
| `storageQuotaExceeded` | The Drive service account has no storage | Use a shared-drive folder or S3. See [Google Drive](/docs/google-drive/). |
| `File not found: <folder id>` | Folder not shared with the service account, wrong ID, or a shared drive | Share the folder with `client_email` as Editor and check the ID. |
| `The specified bucket does not exist` / `Access Denied` | Wrong bucket, region or key permissions | Check `S3_BUCKET`, `S3_REGION` and the policy. |

`LOG_LEVEL=debug` prints the exact command line of each backup (without the password), which you can run by hand from inside the container to see the raw error:

```bash
docker exec -it dbeasebackup sh
PGPASSWORD=... pg_dump -h db.internal -U backup -d appdb -F c -f /tmp/test.dump
```

## Runs are skipped

`Previous run still in progress, skipping` means the target's previous run, with its retries, had not finished when the schedule fired again. Widen the schedule, reduce `retries`, or raise `MAX_PARALLEL_BACKUPS` if runs are waiting for a pool slot. See [Concurrency and retries](/docs/concurrency-and-retries/).

## No run at all

- Check that the schedule means what you think in `SCHEDULER_TIMEZONE`. The startup log line `Jobs scheduled` shows each target's `backup_schedule`.
- Runs only happen on schedule. There is no run on startup.
- Several replicas of the container each run every schedule. Run exactly one.

## Verification warning

`Backup verification failed, but backup was uploaded successfully` means `pg_restore --list` could not read the uploaded dump. The file is kept and the run counts as successful. Download the file and run `pg_restore --list` on it yourself; if it fails there too, the dump is unusable. On S3, check that the target has no `storage.folder_id` (see [Verification](/docs/verification/)).

## Old backups are not deleted

- `retention_days: 0` keeps everything.
- The file was renamed, so its timestamp cannot be parsed (`Skipping file with unparseable backup name`).
- On S3: `S3_PREFIX` has a trailing slash, or the target sets `storage.folder_id`.
- On Google Drive: more than 100 matching files in the folder; cleanup reads only the first page.
- Cleanup runs on `CLEANUP_CRON_SCHEDULE` (default 03:00), not after each backup. Look for `Backup cleanup completed` in the logs.

## Webhooks do not arrive

Look for `Webhook delivery failed` or `Webhook endpoint returned non-success status` with the status code. There are no retries, so a receiver that was down during a run misses that event. See [Webhook notifications](/docs/webhooks/).

## /health returns 503

The body's `issues` array names the storage error. Backup failures never cause a 503; they show in each target's `last_error`.

## Still stuck

Open an issue on [GitHub](https://github.com/glennprays/dbeasebackup/issues) with the version, the log lines for one `trace_id` (remove secrets) and your targets file without passwords.
