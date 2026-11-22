---
title: Verification
description: What DBEaseBackup checks for each mode, when the check runs, and what a failed check means.
---

Both modes check the backup, at different points and with different consequences.

| Mode | When | Check | On failure |
| --- | --- | --- | --- |
| `dump` | After upload | Download the file from storage and run `pg_restore --list` on it | Warning in the log; the run still counts as a success |
| `basebackup` | Before upload | `pg_verifybackup` on the plain directory | The attempt fails, nothing is uploaded, retries apply |

## dump: read back from storage

When `verify` is `true` (the default), after a successful upload the service downloads the same file from storage and streams it into `pg_restore --list`. Nothing is written to disk. The check proves that:

- the object in storage is readable with the configured credentials;
- it is a valid `pg_dump` custom-format archive with a readable table of contents.

It does not restore any data, so it cannot catch every problem. A test restore is still the only full proof; see [Restoring](/docs/restoring/).

A failed check logs `Backup verification failed, but backup was uploaded successfully` at warn level with the `pg_restore` output. The file stays in storage, the run is reported as `success` in [webhooks](/docs/webhooks/) and `/health`, and no retry happens. Watch for that message in your log pipeline.

Set `verify: false` on a target to skip the download, for example for very large dumps on metered storage. `BACKUP_VERIFY=false` does the same in [legacy mode](/docs/legacy-single-db/).

> [!NOTE]
> On S3 the check downloads `S3_PREFIX/<file>`. A target with `storage.folder_id` set on S3 uploads to a different key, so the check cannot find the file. Leave `folder_id` empty for S3 targets.

## basebackup: verify before upload

`pg_verifybackup` compares every file in the copied data directory with the checksums in `backup_manifest` and checks that the required WAL is present and parseable. If it fails, the directory is removed, nothing is uploaded and the attempt returns an error that includes the tool's stderr. The usual [retries](/docs/concurrency-and-retries/) then apply.

`verify: false` has no effect on `basebackup`: the pre-upload check always runs.
