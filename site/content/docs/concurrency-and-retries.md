---
title: Concurrency and retries
description: Overlap protection per target, the global parallelism limit, retry backoff, and shutdown behaviour.
---

Three rules decide what happens when schedules fire: runs of one target never overlap, a global pool limits how many targets run together, and failed runs retry with backoff.

## Per-target overlap

If a target's schedule fires while its previous run is still going (including that run's retries and backoff waits), the new trigger is **skipped**, not queued. The log shows `Previous run still in progress, skipping` and no webhook is sent. If you see this often, the schedule is shorter than the run takes; widen the interval or raise the target's `timeout` budget accordingly.

## Global pool

`MAX_PARALLEL_BACKUPS` (default `1`) is the number of targets that may back up at the same time. A run that finds the pool full **waits** for a slot; it runs late rather than being dropped. `0` is treated as `1`.

While a run waits, it holds its own overlap lock, so the next trigger of the same target is skipped as described above.

Raise the limit when targets live on different servers and the local disk has room for several backups at once. Remember that `basebackup` needs about twice the cluster size in `backup_dir`.

## Retries

A failed attempt is retried `retries` times (default `2`) with a growing wait between attempts:

| Attempt | Wait before it |
| --- | --- |
| 1 | |
| 2 | 30 s |
| 3 | 2 min |
| 4 and later | 2 min (cap) |

Each attempt is a full run: a new dump or base backup with a new timestamp in its filename. Any step can fail an attempt: the backup command, `pg_verifybackup`, the upload, or removing the local file. A failure after the upload succeeded (for example, the local file could not be deleted) means the retry uploads a second copy; both are kept until [retention](/docs/retention/) removes them.

One [webhook](/docs/webhooks/) is sent after the final attempt with its outcome. `/health` updates `last_run` after every attempt.

Set `retries: 0` for targets where a retry would do more harm than good, for example a large `basebackup` during business hours.

## Shutdown

On `SIGTERM` or `SIGINT` the process:

1. marks `/readyz` as not ready and stops the health server;
2. stops the scheduler, so no new runs start;
3. waits up to 30 seconds for running jobs to finish, then exits.

Running backup commands are not cancelled. If a job is still running after 30 seconds, the process exits anyway: in a container its `pg_dump` or `pg_basebackup` child dies with it, and the partial file stays in `backup_dir` until you delete it. Run as a plain binary, the child may keep running on its own. Plan restarts outside backup windows, and give the container enough grace for the 30-second wait: `docker stop -t 40`, `stop_grace_period: 40s` in Compose, or `terminationGracePeriodSeconds: 40` in Kubernetes. The defaults (10 s and 30 s) kill the process before its own wait ends.
