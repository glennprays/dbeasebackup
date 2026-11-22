---
title: Logging
description: The JSON log format, trace IDs per run, and the messages worth alerting on.
---

DBEaseBackup writes one JSON object per line to stdout. Every scheduled run gets its own trace ID, so you can follow one backup from start to finish across retries.

## Format

```json
{"level":"info","timestamp":"2026-11-22T02:00:00.004Z","message":"Starting backup workflow","service":"dbeasebackup","env":"production","trace_id":"8a8f4c1e-2c43-4f0e-9c55-0f4c7b1d9e21","metadata":null,"component":"backup-service","target":"app-db","provider":"postgres-backup"}
```

| Field | Notes |
| --- | --- |
| `timestamp` | ISO 8601 |
| `level` | `debug`, `info`, `warn` or `error` |
| `message` | Fixed text, safe to match on |
| `service` | Always `dbeasebackup` |
| `env` | Value of `ENV` |
| `trace_id` | UUID per scheduled run, or a fixed label such as `main-init` or `scheduler-init` for startup |
| `metadata` | Reserved, currently `null` |
| other keys | Context such as `target`, `component`, `filename`, `size`, `duration`, `error` |

`LOG_LEVEL` sets the minimum level (default `info`). `LOG_FORMAT` is validated but has no effect: output is always JSON. Use `jq` to read it in a terminal:

```bash
docker logs -f dbeasebackup | jq -r '"\(.timestamp) \(.level) \(.target // "-") \(.message)"'
```

## Following one run

```bash
docker logs dbeasebackup | jq -c 'select(.trace_id == "8a8f4c1e-2c43-4f0e-9c55-0f4c7b1d9e21")'
```

The [webhook](/docs/webhooks/) for a run carries the same `trace_id`.

## Messages worth knowing

| Level | Message | Meaning |
| --- | --- | --- |
| info | `Starting scheduled job` | A cron trigger fired (`job` is `backup:<name>` or `backup-cleanup:<name>`) |
| info | `Database dump completed` / `Base backup completed` | The local file is ready |
| info | `Backup uploaded to storage` | Upload finished |
| info | `Backup workflow completed successfully` | Run succeeded; has `filename`, `size`, `duration` |
| warn | `Previous run still in progress, skipping` | Overlap protection skipped a trigger |
| info | `Retrying backup after backoff` | A retry is about to start; has `attempt`, `backoff` |
| error | `Backup attempt failed` | One attempt failed; has `error`, `attempt`, `max_attempts` |
| error | `Job execution failed` | The run failed after all attempts |
| warn | `Backup verification failed, but backup was uploaded successfully` | `pg_restore --list` rejected the uploaded dump |
| warn | `Skipping file with unparseable backup name` | Retention found a file it cannot date |
| info | `Backup cleanup completed` | Retention finished; has `deleted_count`, `skipped_count` |
| warn | `Webhook delivery failed` | The webhook endpoint was unreachable |

Good alerts: any `error`, the verification warning, and `Previous run still in progress, skipping` showing up repeatedly (runs take longer than the schedule interval).

## Debug level

`LOG_LEVEL=debug` adds the exact `pg_dump` or `pg_basebackup` command line (`Executing pg_dump command`) and each health probe. Passwords never appear: they are passed in the `PGPASSWORD` environment variable, not as arguments.

## Collecting logs

Because output is line-delimited JSON on stdout, any collector that reads container logs works without a parser: Fluent Bit, Vector, Promtail or the Docker `json-file` driver.
