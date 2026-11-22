---
title: Health checks and Kubernetes
description: The /health, /readyz and /livez endpoints, what each one checks, and probe settings for Docker and Kubernetes.
---

An HTTP server on `HTTP_PORT` (default `8080`) serves three JSON endpoints. They need no authentication, so keep the port on an internal network.

| Endpoint | Checks | 200 when | Otherwise |
| --- | --- | --- | --- |
| `/livez` | Nothing; the process answers | Always | |
| `/readyz` | Storage reachable and server started | Both true | 503 |
| `/health` | Storage reachable; includes per-target status | Storage reachable | 503 |

The database is not checked by any endpoint, and a failed backup does not change the status code. A single bad run should not restart the pod or flap probes; alert on [webhooks](/docs/webhooks/) or the `targets` array instead.

## /health

```bash
curl -s localhost:8080/health
```

```json
{
  "healthy": true,
  "targets": [
    {
      "target": "app-db",
      "last_run": "2026-11-22T02:01:12.53Z",
      "last_success": "2026-11-22T02:01:12.53Z",
      "last_error": ""
    },
    {
      "target": "prod-cluster",
      "last_run": "2026-11-22T04:12:40.01Z",
      "last_success": "2026-11-15T04:10:02.77Z",
      "last_error": "base backup verification failed: pg_verifybackup failed: exit status 1 (...)"
    }
  ]
}
```

When storage is unreachable the response is `503` with the reason in `issues`:

```json
{
  "healthy": false,
  "issues": ["storage: storage health check failed: S3 bucket my-db-backups does not exist"],
  "targets": []
}
```

Field notes:

- `last_run` is updated after every attempt, including retries. `last_success` only after a successful one. `last_error` is cleared by the next success.
- Before a target's first run both timestamps are `0001-01-01T00:00:00Z`.
- Statuses live in memory and reset on restart.
- The storage check costs one API call per request: S3 `HeadBucket`, or a one-file listing on Google Drive. Probe every 10 s or slower.

A stale `last_success` is the most useful alert. For a daily target, alert when it is older than about 26 hours.

## /readyz and /livez

```bash
curl -s localhost:8080/readyz   # {"ready":true}  or 503 {"ready":false}
curl -s localhost:8080/livez    # {"alive":true}
```

`/readyz` turns false as soon as shutdown begins.

## Kubernetes

```yaml title="deployment.yaml (excerpt)"
containers:
  - name: dbeasebackup
    image: glennprays/dbeasebackup:latest
    ports:
      - containerPort: 8080
    livenessProbe:
      httpGet: { path: /livez, port: 8080 }
      initialDelaySeconds: 10
      periodSeconds: 30
    readinessProbe:
      httpGet: { path: /readyz, port: 8080 }
      initialDelaySeconds: 5
      periodSeconds: 30
```

Run a single replica. Several replicas would each run every schedule and upload duplicate backups.

## Docker

The image includes BusyBox `wget`, but no `curl`:

```yaml title="compose.yaml (excerpt)"
healthcheck:
  test: ["CMD", "wget", "-qO-", "http://127.0.0.1:8080/livez"]
  interval: 30s
  timeout: 5s
  retries: 3
```

## Port

`HTTP_PORT` changes the port. The server cannot be turned off: `HTTP_PORT=0` makes it listen on a random free port.
