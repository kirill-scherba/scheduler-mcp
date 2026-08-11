# scheduler-mcp — STATUS

*Last updated: 2026-08-11*

## Milestones

| Date | Milestone |
|------|-----------|
| 2026-08-03 | Skeleton: 4 MCP tools, in-memory store, 10s ticker (`41744eb`) |
| 2026-08-04 | Executor via opencode-client + notify to chat (`2399943`); opencode-client v0.3.0 (`7868c1d`) |
| 2026-08-07 | Persistence via memory-store client, `every` reschedule, `scheduler_stats`, `scheduler://tasks` + `scheduler://stats` resources, HTTP mode `:7713` (`0f155aa`) |
| 2026-08-08 | **First real service: Mail Butler** — hourly Gmail triage (CRITICAL/JOB/NEWS/ROUTINE/TRASH), Discord + opencode digests, memory rule. Saved domain scherba.ru, found Alfa-Bank collectors. |
| 2026-08-08 | Session cleanup (`CloseSessionID` after run); human-readable `@Scheduled Tasks`; SKILL + memory bank; client `v0.3.0` on GitHub (replace dropped) |

## What works

- [x] One-off and recurring (`every`) tasks with rescheduling
- [x] Persistence in memory (`memory/scheduler/tasks`) — survives restarts
- [x] Execution via opencode session (agent `baron` by default)
- [x] Completion/failure notifications to Kirill's chat
- [x] Session closed after each run
- [x] `scheduler_stats` — totals, runs, avg duration
- [x] `@Scheduled Tasks` / `@Scheduler Stats` resources (human-readable)
- [x] Bounded concurrency (3), per-task timeout (10 min)
- [x] Mail Butler live task (`c8356637`, every 1h, night rule)

## Known issues

- Natural-language `when` not implemented — agents must pass ISO 8601.
- Night window («hours») not a scheduler feature — handled in prompt.

## TODO / planned

- `scheduler_stats from/to` — history per period (events in
  `memory/scheduler/history/<date>/`). «Что делал scheduler за неделю».
- Persist result history for recurring tasks (currently last result only).
- Possible scheduler-level «hours» window for daytime recurring tasks.
- Translate remaining Go projects to shared memory-client
  (goal `goal/2026-08-07/1786093409973753876`: chat-machines,
  orchestrator-watchdog, go-prolog-mcp, web-search-mcp, ai-hub).
