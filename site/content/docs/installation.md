---
title: Installation
description: Image tags, the PostgreSQL client tools DBEaseBackup needs, and how to build the binary from source.
---

DBEaseBackup ships as a container image. You can also build a single static binary and run it anywhere the PostgreSQL client tools are installed.

## Docker image

The image is published to Docker Hub as `glennprays/dbeasebackup` for `linux/amd64` and `linux/arm64`.

| Tag | Meaning |
| --- | --- |
| `latest` | The most recent release |
| `vX.Y.Z` | A specific release |

Pin a version tag in production so an upgrade is a deliberate change. The current tags are listed on [Docker Hub](https://hub.docker.com/r/glennprays/dbeasebackup/tags).

> [!WARNING]
> These docs describe the current code. Multi-target configuration (`TARGETS_FILE`), `basebackup` mode, per-target retries and `MAX_PARALLEL_BACKUPS` are not in `v2.2.0`; they ship with the next release. Until then, the published image supports only the legacy single-database variables described in [Legacy single-DB mode](/docs/legacy-single-db/).

What the image contains:

- Alpine Linux 3.20 with `postgresql-client` (PostgreSQL 16 client tools), which includes `pg_dump`, `pg_restore`, `pg_basebackup` and `pg_verifybackup`.
- `ca-certificates` for TLS to Google and S3 endpoints.
- The binary at `/dbeasebackup`, run as the unprivileged user `appuser` (uid 100) with working directory `/`.
- `ENV=production`, so no `.env` file is read inside the container. Pass settings with `--env-file` or your orchestrator.
- Port `8080` exposed for the health endpoints.

> [!WARNING]
> Because the working directory `/` is owned by root, the default `backup_dir` (`backups/<name>`) cannot be created. Set `backup_dir` to a path under `/tmp`, or mount a volume that uid 100 can write to.

### Docker Compose

```yaml title="compose.yaml"
services:
  dbeasebackup:
    image: glennprays/dbeasebackup:latest   # pin a vX.Y.Z tag in production
    restart: unless-stopped
    env_file: .env
    volumes:
      - ./targets.yaml:/config/targets.yaml:ro
      # Google Drive only:
      # - ./service-account-key.json:/config/service-account-key.json:ro
    ports:
      - "8080:8080"
```

## Client tool versions

The client tools must match your servers:

- `pg_dump` refuses to dump a server whose major version is newer than its own. The image ships version 16, so for PostgreSQL 17 or later build your own image with a newer `postgresql-client` package.
- `basebackup` mode needs `pg_basebackup` and `pg_verifybackup`, which exist from PostgreSQL 13.

At startup each target checks its tools on `PATH` and the process exits if one is missing: `pg_dump` and `pg_restore` for `dump`, `pg_basebackup` and `pg_verifybackup` for `basebackup`.

## Build from source

Requires Go 1.26 or later.

```bash
git clone https://github.com/glennprays/dbeasebackup.git
cd dbeasebackup
make build          # writes ./dbeasebackup for your platform
make build-linux    # static linux/amd64 binary
```

Run it with your settings in the environment, or in a `.env` file in the working directory (read only when `ENV` is unset or `development`):

```bash
export TARGETS_FILE=./targets.yaml
export STORAGE_TYPE=s3
# ...remaining storage settings
./dbeasebackup
```

GitHub releases are tags only; there are no prebuilt binaries attached. Use the image or build from source.

## Upgrading

Configuration is read only at startup, so an upgrade is a restart with the new image. Nothing is migrated: the process keeps no state besides the files in storage.
