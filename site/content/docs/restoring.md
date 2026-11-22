---
title: Restoring
description: Download a backup and restore it with pg_restore or as a data directory. Practise this before you need it.
---

DBEaseBackup does not restore for you. Restores use the standard PostgreSQL tools, which keeps the procedure the same whether DBEaseBackup is running or not. Test it on a scratch server on a regular schedule.

## Find the file

Files are named `<target>_<mode>_<YYYYMMDDTHHMMSSZ>.<ext>` in UTC, so sorting by name sorts by time.

```bash title="S3"
aws s3 ls s3://my-db-backups/backups/postgres/ | grep app-db_
aws s3 cp s3://my-db-backups/backups/postgres/app-db_dump_20261122T020000Z.dump .
```

For S3-compatible services add `--endpoint-url`. For Google Drive, open the target's folder and download the file, or use a tool such as `rclone` with the same service account.

## Restore a dump

A `.dump` file is a `pg_dump` custom-format archive for one database.

```bash
# 1. Inspect it
pg_restore --list app-db_dump_20261122T020000Z.dump | head

# 2. Create an empty database
createdb -h localhost -U postgres appdb_restore

# 3. Restore, 4 jobs in parallel
pg_restore -h localhost -U postgres -d appdb_restore -j 4 \
  --no-owner app-db_dump_20261122T020000Z.dump
```

Useful variations:

| Goal | Command |
| --- | --- |
| Recreate the database under its original name | `pg_restore -d postgres --create file.dump` |
| Replace objects in an existing database | `pg_restore -d appdb --clean --if-exists file.dump` |
| One table only | `pg_restore -d appdb -t orders file.dump` |
| Inspect as SQL | `pg_restore -f restore.sql file.dump` |

Roles are not in a dump. Create them first, or restore with `--no-owner --no-privileges` and grant access afterwards. Use a `pg_restore` with the same or a newer major version than the one that made the dump (the image ships 16).

Check the result:

```sql
SELECT relname, n_live_tup FROM pg_stat_user_tables ORDER BY n_live_tup DESC LIMIT 10;
```

## Restore a basebackup

A `.tar.gz` file is a complete data directory, including `backup_manifest` and the WAL needed to start. It restores the whole cluster to the moment the copy finished, on the same PostgreSQL major version.

```bash
# 1. Stop PostgreSQL on the target host
sudo systemctl stop postgresql

# 2. Move the old data directory aside (never extract over it)
sudo mv /var/lib/postgresql/16/main /var/lib/postgresql/16/main.old

# 3. Extract into an empty directory owned by postgres, mode 0700
sudo install -d -o postgres -g postgres -m 0700 /var/lib/postgresql/16/main
sudo tar -xzf prod-cluster_basebackup_20261122T040000Z.tar.gz \
  -C /var/lib/postgresql/16/main
sudo chown -R postgres:postgres /var/lib/postgresql/16/main

# 4. Optional: verify again on the restore host
sudo -u postgres pg_verifybackup /var/lib/postgresql/16/main

# 5. Start PostgreSQL
sudo systemctl start postgresql
```

Paths and service names vary by distribution. Configuration files that live inside the data directory (`postgresql.conf`, `pg_hba.conf` on some installs) come back from the backup; review them before starting a restored copy on a different host.

## Practise

A backup you have never restored is a hope, not a backup. A simple routine:

1. Monthly, pick the latest file of each target.
2. Restore it on a scratch server with the commands above.
3. Run a few queries your application depends on.
4. Write down how long it took. That is your real recovery time.
