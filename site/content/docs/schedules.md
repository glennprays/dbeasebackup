---
title: Schedules and timezones
description: Cron syntax for backup and cleanup schedules, and how SCHEDULER_TIMEZONE applies.
---

Each target's `schedule` decides when it backs up. One global `CLEANUP_CRON_SCHEDULE` decides when retention cleanup runs for every target. All expressions are evaluated in `SCHEDULER_TIMEZONE` (default `UTC`).

## Syntax

Schedules use the standard five-field cron format, parsed by [robfig/cron](https://github.com/robfig/cron):

```text
┌ minute (0-59)
│ ┌ hour (0-23)
│ │ ┌ day of month (1-31)
│ │ │ ┌ month (1-12)
│ │ │ │ ┌ day of week (0-6, Sunday = 0)
│ │ │ │ │
0 2 * * *
```

There is no seconds field. Descriptors such as `@daily`, `@hourly` and `@every 6h` are accepted too.

| Schedule | Expression |
| --- | --- |
| Every hour | `0 * * * *` |
| Every 6 hours | `0 */6 * * *` |
| Daily at 02:00 | `0 2 * * *` |
| Twice a day, 02:00 and 14:00 | `0 2,14 * * *` |
| Weekdays at 01:30 | `30 1 * * 1-5` |
| Sundays at 04:00 | `0 4 * * 0` |
| First of the month at 02:00 | `0 2 1 * *` |

Quote expressions in YAML (`schedule: "0 2 * * *"`), because a value that starts with `*` or `@` is otherwise not a plain string.

## Timezone

```bash
SCHEDULER_TIMEZONE=Asia/Jakarta
```

Use an IANA zone name. An unknown name stops startup with `invalid timezone`. Around daylight-saving changes, a run scheduled inside the skipped or repeated hour may be skipped or run twice; schedule backups outside 01:00–03:00 local time in zones with DST, or keep the default `UTC`.

The timezone affects only when jobs fire. Filename timestamps are always UTC.

## Spreading targets out

Runs of the same target never overlap, and at most `MAX_PARALLEL_BACKUPS` targets run at once; the rest wait for a slot. Staggering schedules (`0 2 * * *`, `20 2 * * *`, ...) keeps load on the database server and the local disk predictable. See [Concurrency and retries](/docs/concurrency-and-retries/).

## Cleanup schedule

`CLEANUP_CRON_SCHEDULE` defaults to `0 3 * * *`. One cleanup job is registered per target, all on this schedule. In [legacy mode](/docs/legacy-single-db/) the fallback is `CRON_SCHEDULE`. See [Retention](/docs/retention/).

## Testing a schedule

There is no command to run a backup immediately. To test, set a short schedule such as `*/5 * * * *` on one target, restart, watch the logs for `Starting scheduled job`, then restore the real schedule.
