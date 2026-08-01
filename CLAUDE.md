# refrain

Personal-memory service for domain-expert agents: git-backed markdown
memory per expert, exposed as MCP tools at `/mcp` plus an injection
digest at `/digest` for harness hooks. No vector index by design
(memory corpora are small; grep + whole-file reads); every write is a
git commit; session context is the one non-git piece (ephemeral, 24h
TTL). Design rationale: `docs/design.md`.

## Layout

| Path | Purpose |
| ---- | ------- |
| `cmd/refrain` | Entry point: env config, mux, logging middleware, graceful shutdown |
| `internal/store` | Single-writer store over the git-backed memory root; all mutations commit |
| `internal/mcpserver` | MCP tools (Streamable HTTP) + the plain-HTTP digest handler |
| `deploy/` | Reference compose stack (macvlan) + Dockerfile |
| `integrations/openwebui/` | Inlet filter that injects the digest at chat start |
| `integrations/claudecode/` | SessionStart hook that injects the digest into Claude Code sessions |

## Conventions

- `log/slog` only; JSON handler outside `TARGET_ENV=local`.
- stdlib `net/http` + closure middleware; the `statusWriter` implements
  `http.Flusher` deliberately (MCP Streamable HTTP uses SSE).
- Errors: `fmt.Errorf("lowercase operation: %w", err)`; exported
  sentinel errors (`store.ErrNotFound`) where handlers map them.
- `mcpserver` depends on a package-private narrow interface over the
  store with a `var _ memoryStore = (*store.Store)(nil)` guard.
- Tests: stdlib table-driven, no testify. The store tests run against
  the real git binary — the git interaction IS the logic — and skip
  when git is absent.
- Comments explain WHY, not WHAT. No emojis anywhere.

## Inner loop

```bash
go build ./...
go vet ./...
go test -race ./...
gofmt -l .          # CI fails if this prints anything
golangci-lint run   # CI gates this too — vet alone is NOT enough
```
