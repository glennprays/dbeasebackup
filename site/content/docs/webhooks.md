---
title: Webhook notifications
description: The JSON payload sent after every backup run, how to verify its HMAC signature, and delivery behaviour.
---

Set `WEBHOOK_URL` and DBEaseBackup sends one `POST` when a backup run finishes, after all retries, whether it succeeded or failed.

```bash title=".env"
WEBHOOK_URL=https://hooks.example.com/dbeasebackup
WEBHOOK_SECRET=a-long-random-string
WEBHOOK_TIMEOUT=10s
```

## Payload

```json title="failure"
{
  "event_type": "backup",
  "status": "failure",
  "target": "app-db",
  "duration": "2m41.512s",
  "provider": "",
  "storage_type": "",
  "timestamp": "2026-11-22T02:02:41.51Z",
  "trace_id": "8a8f4c1e-2c43-4f0e-9c55-0f4c7b1d9e21",
  "error": "unable to backup database: exit status 1 (pg_dump: pg_dump: error: connection to server at \"db.internal\" (10.0.0.12), port 5432 failed: FATAL:  password authentication failed for user \"backup\")"
}
```

| Field | Type | Notes |
| --- | --- | --- |
| `event_type` | string | Always `backup`. Retention cleanup sends no webhooks. |
| `status` | string | `success` or `failure`. |
| `target` | string | Target `name`. |
| `duration` | string | Go duration for the whole run, including retries and backoff waits. |
| `provider` | string | Currently always empty. |
| `storage_type` | string | Currently always empty. |
| `timestamp` | string | RFC 3339 time the run finished, in the process timezone (UTC in the image). |
| `trace_id` | string | UUID of the run. Search the logs for it. |
| `error` | string | Last attempt's error. Omitted on success. |

The payload does not include the filename or size yet. Use `trace_id` to find them in the logs (`Backup workflow completed successfully` carries `filename` and `size`).

Headers:

```text
Content-Type: application/json
User-Agent: DBEaseBackup-Webhook/1.0
X-Signature-256: sha256=<hex HMAC-SHA256 of the raw body, keyed with WEBHOOK_SECRET>
```

`X-Signature-256` is present only when `WEBHOOK_SECRET` is set.

## Verify the signature

Compute HMAC-SHA256 over the **raw request body** with your secret and compare it to the header in constant time. Parse the JSON only after the check passes.

```go title="Go"
func valid(body []byte, header, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(want), []byte(header))
}
```

```python title="Python"
import hashlib, hmac

def valid(body: bytes, header: str, secret: str) -> bool:
    want = "sha256=" + hmac.new(secret.encode(), body, hashlib.sha256).hexdigest()
    return hmac.compare_digest(want, header)
```

```javascript title="Node.js"
const crypto = require("node:crypto");

function valid(body, header, secret) {
  const want = "sha256=" + crypto.createHmac("sha256", secret).update(body).digest("hex");
  const a = Buffer.from(want), b = Buffer.from(header || "");
  return a.length === b.length && crypto.timingSafeEqual(a, b);
}
```

The payload has no nonce, so add your own replay protection if it matters: reject requests whose `timestamp` is more than a few minutes old, or deduplicate on `trace_id`.

## Delivery

- One attempt per run, with `WEBHOOK_TIMEOUT` (default `10s`). There are no retries.
- A response of `300` or above, or a network error, is logged at warn level (`Webhook endpoint returned non-success status`, `Webhook delivery failed`) and does not affect the backup.
- No webhook is sent when a run is skipped because the previous run of the same target is still going, or when shutdown cancels a run that is waiting for a pool slot.

Because delivery is best effort, also alert on a stale `last_success` from [/health](/docs/health-checks/) so a lost webhook cannot hide a failing target.

## Examples

Slack and similar chat tools expect their own JSON shape. Put a small relay in between that verifies the signature and posts a message such as `app-db backup failed: <error>`.
