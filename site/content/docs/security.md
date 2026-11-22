---
title: Security
description: Least-privilege database roles, credential handling, TLS, storage hardening and what the health endpoint exposes.
---

A backup tool holds read access to all your data and write access to where copies live. Treat its credentials and its storage as production secrets.

## Database roles

Give each target a dedicated role with only what its mode needs.

### dump

On PostgreSQL 14 and later, the predefined role `pg_read_all_data` covers every table, view and sequence:

```sql
CREATE ROLE backup LOGIN PASSWORD '...';
GRANT CONNECT ON DATABASE appdb TO backup;
GRANT pg_read_all_data TO backup;
```

On older versions, grant read access per schema and keep it current for new tables:

```sql
GRANT USAGE ON SCHEMA public TO backup;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO backup;
GRANT SELECT ON ALL SEQUENCES IN SCHEMA public TO backup;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO backup;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON SEQUENCES TO backup;
```

Repeat for each schema. Missing grants make `pg_dump` fail with `permission denied`. Row-level security policies also apply to the backup role unless it has `BYPASSRLS`.

### basebackup

A physical copy needs a replication role, which can read every file of the cluster:

```sql
CREATE ROLE replication WITH REPLICATION LOGIN PASSWORD '...';
```

```text title="pg_hba.conf"
host  replication  replication  10.0.0.5/32  scram-sha-256
```

Allow it only from the host that runs DBEaseBackup.

## Credentials

- Put passwords in environment variables and reference them with `password_env`, not `password`.
- Inject the variables from your platform's secret store (Docker or Kubernetes secrets, Vault, a cloud secret manager) rather than committing `.env` files.
- Passwords reach `pg_dump` and `pg_basebackup` through `PGPASSWORD` in the child environment, never on the command line, so they do not show in `ps` output or debug logs.
- Mount the Google service account key read-only, readable by uid 100 only where possible, and rotate it by adding a new key before deleting the old one.
- Scope S3 keys to one bucket and prefix. The policy in [S3 and compatible](/docs/s3/) is the minimum.

## TLS

- **Database:** set `PGSSLMODE=verify-full` (with the server's CA available to libpq) on the process. The per-target `sslmode` field is validated but not passed to the tools yet, so it does not protect anything on its own.
- **S3:** use an `https://` `S3_ENDPOINT`. `http://` or a bare host sends keys and data in plain text.
- **Google Drive** and webhooks over `https://` use the system CA bundle shipped in the image.

## Storage hardening

- Make the bucket private and enable default encryption.
- Turn on versioning or object lock so that a leaked key cannot delete history; pair it with a lifecycle rule for old versions.
- DBEaseBackup does not encrypt files itself. If storage-side encryption is not enough, use a bucket with customer-managed keys (SSE-KMS), or encrypt the copies in a separate step.

## Health endpoint

`/health` needs no authentication and returns target names and the last error of each target. Errors can contain host names, database names and role names. Keep `HTTP_PORT` on an internal network or behind your orchestrator; do not publish it to the internet.

## Webhooks

Set `WEBHOOK_SECRET` and verify `X-Signature-256` on the receiving side; see [Webhook notifications](/docs/webhooks/). Without a secret anyone who knows the URL can send fake events.

## Container

The image runs as the non-root user `appuser` (uid 100). It also works with a read-only root filesystem if `/tmp` is writable:

```yaml title="compose.yaml (excerpt)"
read_only: true
tmpfs:
  - /tmp:size=4g        # size it for your largest backup
cap_drop: [ALL]
security_opt: ["no-new-privileges:true"]
```

## Reporting a vulnerability

Do not open a public issue. Use GitHub's private vulnerability reporting on the [repository](https://github.com/glennprays/dbeasebackup/security) and include steps to reproduce.
