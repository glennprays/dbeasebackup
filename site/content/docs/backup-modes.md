---
title: Backup modes
description: When to use a logical dump of one database and when to take a physical copy of the whole cluster.
---

Each target has a `mode`. `dump` makes a logical backup of one database with `pg_dump`. `basebackup` makes a physical copy of the whole cluster with `pg_basebackup`.

| | `dump` | `basebackup` |
| --- | --- | --- |
| Tool | `pg_dump -F c` | `pg_basebackup -Fp -Xs -c fast`, then `pg_verifybackup` |
| Scope | One database (`database` required) | Every database in the cluster (`database` must be empty) |
| File | `<name>_dump_<ts>.dump` (custom format, compressed) | `<name>_basebackup_<ts>.tar.gz` |
| Privileges | Read access to the database | Role with `REPLICATION` plus a `replication` line in `pg_hba.conf` |
| Restore into | Any server with the same or a newer major version | The same major version, as a whole data directory |
| Roles and other globals | Not included | Included |
| Checked | After upload, with `pg_restore --list` (warn only) | Before upload, with `pg_verifybackup` (fail closed) |
| Local disk | About the compressed dump size | About twice the cluster size at peak |

## dump

```yaml
- name: app-db
  mode: dump
  host: db.internal
  user: backup
  password_env: APP_DB_PASSWORD
  database: appdb
  schedule: "0 2 * * *"
```

The provider runs:

```bash
pg_dump -h <host> -p <port> -U <user> -d <database> -F c -f <backup_dir>/<name>_dump_<ts>.dump
```

The custom format (`-F c`) is compressed and lets `pg_restore` restore selected tables, run in parallel (`-j`) and list contents without restoring. If `pg_dump` fails, its last 64 KiB of stderr are included in the error, the log and the webhook.

A dump covers one database. Roles, role memberships and tablespace definitions are cluster-wide and are not in the file. Keep them separately, for example with a periodic `pg_dumpall --globals-only`, or use `basebackup`.

## basebackup

```yaml
- name: prod-cluster
  mode: basebackup
  host: db.internal
  user: replication
  password_env: REPL_PASSWORD
  schedule: "0 4 * * 0"
  timeout: 2h
```

One run has four stages, all inside the target's `timeout`:

1. `pg_basebackup -Fp -Xs -c fast` writes a plain data directory with its `backup_manifest`. `-Xs` streams the WAL needed for a consistent copy; `-c fast` asks the server for an immediate checkpoint.
2. `pg_verifybackup` checks every file against the manifest and checks the WAL. **If it fails, nothing is uploaded** and the attempt fails.
3. The directory is streamed into `<name>_basebackup_<ts>.tar.gz`.
4. The plain directory is removed and the tarball goes to upload.

Requirements on the server:

- A role with the `REPLICATION` attribute: `CREATE ROLE replication WITH REPLICATION LOGIN PASSWORD '...';`
- A `pg_hba.conf` line that allows it, for example `host replication replication 10.0.0.5/32 scram-sha-256`.
- `max_wal_senders` of at least 2, because WAL streaming opens a second connection.
- PostgreSQL 13 or later client tools (`pg_verifybackup` was added in 13), with the same major version as the server.

> [!WARNING]
> Peak local disk use is about twice the cluster size: the plain directory and the tarball exist together until the directory is removed. Size `backup_dir` for that.

> [!CAUTION]
> Plain format writes extra tablespaces to the same absolute paths they have on the server. Clusters that use tablespaces outside the data directory are not supported by this mode yet; use `dump` for those databases.

`basebackup` is a full copy each time. It is not incremental and does not archive WAL continuously, so it restores to the moment the copy finished, not to an arbitrary point in time.

## Choosing

- Use `dump` for single application databases, for moving data between major versions, and when you want to restore one table.
- Use `basebackup` to rebuild a whole server quickly, roles and all, on the same major version.
- Many teams run both: frequent dumps of the important databases and a weekly basebackup of the cluster.
