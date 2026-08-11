# scheduler-mcp

**Universal scheduled tasks for AI agents.** One-off and recurring tasks that
run at a specific time via opencode sessions. Not just email — everything:
reminders, follow-ups, daily reports, mail triage.

## Features

- **One-off and recurring tasks** — `every` reschedules a task after each
  successful run (`when` advances by the period, task stays pending).
- **Persistence in shared memory** — tasks live in `memory/scheduler/tasks`
  via [memory-store-mcp/client](https://github.com/kirill-scherba/memory-store-mcp/tree/main/client).
  They survive scheduler restarts.
- **Agent execution** — when a task is due, the scheduler spawns an opencode
  session (default agent `baron`) with the task prompt and waits for the reply.
- **Notifications** — on completion/failure Kirill gets an inline notification
  in his opencode chat (notifications.ts plugin, HTTP `:7799/notify`).
- **Session cleanup** — the agent session is closed after each run
  (DELETE `/session/{id}`), so recurring tasks do not accumulate sessions.
- **Human-friendly `@` resources** — `@Scheduled Tasks` and `@Scheduler Stats`
  in the opencode `@` menu show tasks without writing a query.
- **Stats** — `scheduler_stats`: totals by status, total runs, avg duration.

## Install / Run

```bash
go build -o scheduler-mcp .
# stdio mode (for MCP clients)
./scheduler-mcp serve
# HTTP mode (StreamableHTTP, for mcp-gateway remote proxy)
./scheduler-mcp serve --http 127.0.0.1:7713
```

Systemd user service (`~/.config/systemd/user/scheduler-mcp.service`):

```ini
[Service]
Type=simple
ExecStart=%h/go/src/github.com/kirill-scherba/scheduler-mcp/scheduler-mcp serve --http 127.0.0.1:7713
Restart=on-failure
```

In mcp-gateway config it is registered as a **remote proxy** (resources only
work for remote, not local servers):

```json
"scheduler-mcp": { "type": "remote", "url": "http://127.0.0.1:7713/mcp" }
```

## Tools

| Tool | Purpose |
|------|---------|
| `scheduler_create(when, action, [author, agent, every, expired])` | Create a task |
| `scheduler_list([status])` | List tasks |
| `scheduler_cancel(id)` | Cancel a task |
| `scheduler_update(id, [when, action, every])` | Reschedule / change a task |
| `scheduler_stats()` | Execution statistics |

Always pass an **absolute ISO 8601 time** in `when`
(e.g. `2026-08-04T10:00:00Z`); the natural-language parser is not implemented
yet.

## Resources (`@` menu)

| Resource | What it shows |
|----------|---------------|
| `@ Scheduled Tasks` | Human-readable task list: id, action, when, every, status, author, runs, last_run, result |
| `@ Scheduler Stats` | Totals by status, total runs |

## Live example — Mail Butler

The first real service built on scheduler-mcp: every hour it triages Kirill's
Gmail (`kiriushka@gmail.com`), classifies messages (CRITICAL / JOB / NEWS /
ROUTINE / TRASH), posts a digest to Discord `#mail-butler` and to the opencode
chat, and saves important facts to memory. Full prompt:
`docs/mail-butler-task.md`.

## Related

- [memory-store-mcp/client](https://github.com/kirill-scherba/memory-store-mcp/tree/main/client) —
  shared Go client for the memory store (v0.3.0).
- [opencode-client](https://github.com/kirill-scherba/opencode-client) — Go
  client for opencode-serve (session creation, messaging).
- [mail-mcp](https://github.com/kirill-scherba/mail-mcp) — email server used
  by Mail Butler (Gmail multi-account).

## Docs

- `docs/DESIGN.md` — architecture
- `docs/CONTEXT.md` — current state and decisions
- `docs/STATUS.md` — progress, milestones, known issues
