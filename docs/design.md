# Design decisions

Recall is the personal-memory layer for domain-expert agents. It stores
the one class of data that cannot be rebuilt from any source: the
user's own authored intent — goals, plans, decisions-with-why,
preferences, session history. World knowledge, capability code, and
live state all live elsewhere and are rebuildable; this layer is not.
That asymmetry drives every choice below.

**Substrate: markdown files in a git repository, not database rows.**
Memory must be human-legible and human-editable — a trust property no
database gives. The user can open goals.md in an editor, fix it, and
the service keeps working (external edits are checkpointed as their own
commits). Git history IS decision history: every write is a commit with
a meaningful message, so the audit trail and revertibility come for
free, and the whole corpus is portable with `git clone`.

**No vector index.** Memory holds only the user's own writing, so
corpora stay small — a few markdown files per expert. At that scale,
whole-file reads and grep-style line search beat an embedding stack
outright, and dropping the index removes the re-ingest problem
entirely: human edits need no watcher or re-embedding pass, because
every read hits disk. If memory ever outgrows this, the search
implementation is one function; the contract wouldn't change.

**Single writer.** All mutations pass through one store under a mutex
and become git commits. That is what makes concurrent clients (a phone
chat, a web UI, a coding agent) safe against one working tree without
locks leaking into the protocol. The write surface is deliberately
append-or-replace only — no delete tool exists; git revert is the
undo.

**Session context lives outside git** (`.session/`, gitignored, 24h
TTL). "Tonight I'm prepping for the tournament" is scratch state, not
memory — committing it would bury the history in noise, and stale
scratch must expire rather than leak into tomorrow. Session SUMMARIES
graduate into memory via the append-only session log.

**Two mechanisms, both required.** The digest endpoint exists for
harness-side INJECTION at chat start (profile, goals, recent
decisions, last session — capped to a few hundred tokens); the MCP
tools serve deep recall and writes during the conversation. Tools
alone fail because a model cannot search for what it doesn't know it
forgot; injection alone fails because nothing new would ever be
recorded.
