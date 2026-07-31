# recall

Personal memory for domain-expert agents: git-backed markdown memory
per expert (profile, goals, decisions-with-why, session logs, topic
notes), exposed two ways — MCP tools at `/mcp` for in-conversation
reads and writes, and an injection digest at `/digest` for harness
hooks (an Open WebUI inlet filter, a Claude Code session hook) to
prepend at chat start.

## Why

An agent that advises the same user across sessions needs to remember
what the user decided and WHY — the one class of data that cannot be
rebuilt from any source. Recall stores exactly that, and nothing else:

- **Markdown in a git repo**, not database rows: human-legible,
  human-editable, and git history is the audit trail.
- **No vector index**: memory holds only the user's own writing and
  stays small; whole-file reads and grep beat an embedding stack at
  this scale.
- **Single writer, append-or-replace only**: every write is a git
  commit; no delete tool exists — `git revert` is the undo.
- **Two mechanisms**: the digest is injected at chat start (a model
  cannot search for what it doesn't know it forgot); the tools serve
  deep recall and writes during the conversation.

The full rationale is in [docs/design.md](docs/design.md).

## Quickstart

```bash
go build -o bin/recall ./cmd/recall
RECALL_ROOT=$HOME/recall-memories TARGET_ENV=local ./bin/recall
curl http://localhost:8092/healthz
```

The memory root is created (and `git init`-ed) on first start. Config
is env vars: `RECALL_ROOT` (default `/data/memories`), `PORT` (default
`8092`), `TARGET_ENV` (`local` keeps text logs; anything else switches
to JSON). Requires the `git` binary on `PATH`.

For a container deployment see [deploy/](deploy/README.md); for the
Open WebUI injection filter see
[integrations/openwebui/](integrations/openwebui/README.md).

## API

### HTTP

| Endpoint | Purpose |
|---|---|
| `/mcp` | MCP Streamable HTTP: the tools below |
| `GET /digest?expert=<slug>` | Injection digest (`text/markdown`): session context, profile, goals, last 5 decisions, last session log entry — capped to a few hundred tokens |
| `GET /healthz` | Liveness |

### MCP tools

| Tool | Purpose |
|---|---|
| `list_experts` | Experts with their memory files |
| `read_memory` | Read one file in full (they are small — prefer this over search) |
| `search_memory` | Grep an expert's files for lines matching any query term |
| `record_decision` | Append a dated decision; the why is mandatory |
| `record_note` | Append to (or create) a topic note |
| `update_goals` | Replace goals.md wholesale |
| `update_profile` | Replace profile.md wholesale |
| `append_session_log` | Append a dated session summary |
| `set_session_context` | Ephemeral scratch context, 24h TTL, outside git |
| `create_expert` | Scaffold a new expert's memory space |

Per-expert layout: `profile.md`, `goals.md`, `decisions.md`, `log.md`,
`notes/<topic>.md`.

**Security posture**: both endpoints are unauthenticated by design and
must only be reachable on a trusted private network. See the warning
in [deploy/README.md](deploy/README.md).

## Development

```bash
make build     # bin/recall
make test      # go test -race ./...
make vet
make lint      # golangci-lint
make fmt-check
```
