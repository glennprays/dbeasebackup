<p align="center">
  <img src="./img/logo.png" alt="Logo" width="160"/>

  <h1 align="center">DBEaseBackup</h1>
  <p align="center">Scheduled database backups to Google Drive or S3, verified and pruned by retention.</p>
  <p align="center">
    <a href="https://dbease.glennprays.com/"><img src="https://img.shields.io/badge/website-dbease.glennprays.com-b4441c" alt="Website"></a>
    <a href="https://dbease.glennprays.com/docs/"><img src="https://img.shields.io/badge/docs-read-b4441c" alt="Documentation"></a>
    <a href="https://github.com/glennprays/dbeasebackup/actions/workflows/build-docker-image.yml"><img src="https://github.com/glennprays/dbeasebackup/actions/workflows/build-docker-image.yml/badge.svg" alt="Build Docker Image Status"></a>
    <a href="https://hub.docker.com/r/glennprays/dbeasebackup"><img src="https://img.shields.io/docker/v/glennprays/dbeasebackup?label=Docker&color=blue" alt="Docker Image"></a>
    <img src="https://img.shields.io/badge/License-MIT-green.svg" alt="License">
  </p>
  <p align="center">
    <a href="https://dbease.glennprays.com/">Website</a> ·
    <a href="https://dbease.glennprays.com/docs/">Documentation</a> ·
    <a href="https://dbease.glennprays.com/docs/quick-start/">Quick start</a> ·
    <a href="https://hub.docker.com/r/glennprays/dbeasebackup">Docker Hub</a>
  </p>
</p>

DBEaseBackup is a single Go process that backs up each configured database target on its own cron schedule, verifies the backup, uploads it to Google Drive or an S3-compatible bucket, and deletes copies older than the target's retention window. It keeps no state of its own and exposes `/health`, `/readyz` and `/livez` for your orchestrator.

## Features

- **Many targets, one process.** A `targets.yaml` lists every database with its own connection, cron schedule, mode, retention, retries and storage folder. Secrets stay in environment variables.
- **Two backup modes.** `dump` runs `pg_dump -F c` for one database. `basebackup` runs `pg_basebackup` for the whole cluster and checks it with `pg_verifybackup` before upload.
- **Verified backups.** Base backups that fail verification are never uploaded. Dumps are downloaded again after upload and read with `pg_restore --list`.
- **Google Drive or S3.** Google Drive through a service account, or any S3-compatible service (AWS S3, MinIO, Cloudflare R2, Backblaze B2, DigitalOcean Spaces).
- **Retention without a catalog.** Cleanup reads the UTC timestamp in each filename and deletes files older than `retention_days`.
- **Retries and limits.** Failed runs retry with backoff, overlapping runs of one target are skipped, and `MAX_PARALLEL_BACKUPS` caps concurrent backups.
- **Easy to monitor.** `/health`, `/readyz` and `/livez` endpoints, an HMAC-signed webhook after each run, and JSON logs with trace IDs.

## Supported engines and storage

| | Today | Planned |
| --- | --- | --- |
| Databases | PostgreSQL (client tools 16 in the image) | More engines |
| Storage | Google Drive, S3 and S3-compatible | |

Read the full documentation at **https://dbease.glennprays.com/docs/**.

## Quick start

Write a targets file with one target per database:

```yaml
# targets.yaml
targets:
  - name: app-db
    mode: dump
    host: db.internal
    port: "5432"
    user: backup
    password_env: APP_DB_PASSWORD
    database: appdb
    schedule: "0 2 * * *"
    retention_days: 30
    backup_dir: /tmp/dbeasebackup/app-db
```

Put global settings and secrets in `.env`:

```bash
TARGETS_FILE=/config/targets.yaml
APP_DB_PASSWORD=change-me

STORAGE_TYPE=s3
S3_ENDPOINT=https://s3.us-east-1.amazonaws.com
S3_BUCKET=my-db-backups
S3_REGION=us-east-1
S3_ACCESS_KEY_ID=AKIA...
S3_SECRET_ACCESS_KEY=...
```

Start the container and check health:

```bash
docker run -d --name dbeasebackup \
  --restart unless-stopped \
  --env-file .env \
  -v "$(pwd)/targets.yaml:/config/targets.yaml:ro" \
  -p 8080:8080 \
  glennprays/dbeasebackup:latest

curl -s localhost:8080/health
```

> [!NOTE]
> `TARGETS_FILE`, `basebackup` mode and retries need `v2.3.0` or newer. On `v2.2.0` and older, use the [legacy single-database variables](https://dbease.glennprays.com/docs/legacy-single-db/). Upgrading from `v2.2.0`? Read [Upgrading](https://dbease.glennprays.com/docs/installation/#from-v220-or-older) first.

The full walkthrough, including Google Drive, is in the [quick start](https://dbease.glennprays.com/docs/quick-start/).

## Documentation

- [Introduction](https://dbease.glennprays.com/docs/introduction/): how a run works and what is out of scope
- [Targets file](https://dbease.glennprays.com/docs/targets-file/) and [environment variables](https://dbease.glennprays.com/docs/environment-variables/)
- [Backup modes](https://dbease.glennprays.com/docs/backup-modes/), [verification](https://dbease.glennprays.com/docs/verification/), [retention](https://dbease.glennprays.com/docs/retention/) and [restoring](https://dbease.glennprays.com/docs/restoring/)
- [Google Drive](https://dbease.glennprays.com/docs/google-drive/) and [S3](https://dbease.glennprays.com/docs/s3/) storage
- [Health checks](https://dbease.glennprays.com/docs/health-checks/), [webhooks](https://dbease.glennprays.com/docs/webhooks/) and [logging](https://dbease.glennprays.com/docs/logging/)
- [Security](https://dbease.glennprays.com/docs/security/) and [troubleshooting](https://dbease.glennprays.com/docs/troubleshooting/)

The website and docs are built from [`site/`](./site) and deployed to https://dbease.glennprays.com/ by [GitHub Actions](./.github/workflows/pages.yml). Docs pages live in [`site/content/docs`](./site/content/docs). Preview locally with `make site-serve` (http://localhost:4000).

## Build from source

Requires Go 1.26 or newer, plus the PostgreSQL client tools on `PATH` at runtime.

```bash
git clone https://github.com/glennprays/dbeasebackup.git
cd dbeasebackup
make build
./dbeasebackup
```

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md) for development setup, code style and the pull request process.

## License

MIT. See [LICENSE](LICENSE).
