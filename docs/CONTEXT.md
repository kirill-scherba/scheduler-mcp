# scheduler-mcp — CONTEXT

*Last updated: 2026-08-11*

## Current state

scheduler-mcp is **production-working** and powers the first real service —
**Mail Butler** (automatic Gmail triage for Kirill). Mail Butler runs every
hour, classifies mail, posts digests to Discord `#mail-butler` + opencode chat,
and saves important facts to memory.

## Key decisions

1. **Persistence in shared memory, not files** (2026-08-07).
   Tasks live in `memory/scheduler/tasks` via the shared Go client
   `memory-store-mcp/client`. Rationale (Kirill): tasks are data — they must
   live in the memory store, visible from any host, not in a local JSON file.
   «Если memory-store лежит — жизни нет; агент без памяти — голая LLM».
2. **Shared Go client** (2026-08-07). Created `memory-store-mcp/client`
   (nested module, stdlib only) — `Save/Get/List/Delete/Search`, `MEMORY_URL`
   env, default `http://127.0.0.1:7708/mcp`. Tagged `client/v0.3.0`, used by
   scheduler-mcp and hh-mcp without local replace.
3. **Resources need remote proxy** (2026-08-07). mcp-gateway disables
   resources for `local` servers (`WithResourceCapabilities(false,false)`).
   scheduler-mcp therefore runs as its own HTTP service (`:7713`) and is
   registered in gateway as a `remote` proxy. Only then do `@Scheduled Tasks`
   and `@Scheduler Stats` appear in the `@` menu.
4. **Human-readable resources** (2026-08-08). `scheduler://tasks` returns a
   clean text list (not raw JSON) so Kirill can read tasks from the `@` menu.
5. **Session cleanup** (2026-08-08). Executor closes the agent session
   (`DELETE /session/{id}`) after each run — recurring tasks do not
   accumulate sessions in opencode-serve.
6. **Recurring reschedule** (2026-08-07). `every` tasks reschedule
   themselves after a successful run: `when += period`, status back to
   `pending`. `runs` counts executions, `last_run` stores the last time.

## Recent changes

- 2026-08-08 — Mail Butler task created (`c8356637`, every 1h, night rule);
  docs/mail-butler-task.md holds the full action prompt.
- 2026-08-08 — SKILL updated (`~/.config/opencode/skills/scheduler-mcp/` +
  copy in `matrica/config/opencode/skills/scheduler-mcp/`).
- 2026-08-08 — `scheduler://tasks` resource switched to human-readable text.
- 2026-08-08 — client v0.3.0 released on GitHub; projects dropped local
  `replace`.

## Open questions / notes

- Natural-language `when` («tomorrow», «in 30 minutes») is NOT implemented —
  agents must convert to ISO 8601 themselves.
- Night rule for daytime recurring tasks is handled in the task prompt
  («если ночь 22–8 — завершись»), not in the scheduler itself. A scheduler
  level «hours» window is a possible future feature.
- Go 1.26.6 security release expected 2026-08-11 — update projects after
  release (see `memory/todo/go-1-26-6-security-update`).
