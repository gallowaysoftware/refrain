# Design decisions

Refrain is the personal-memory layer for domain-expert agents. It stores
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
tools serve deep memory reads and writes during the conversation. Tools
alone fail because a model cannot search for what it doesn't know it
forgot; injection alone fails because nothing new would ever be
recorded.


## Search ergonomics (2026-07-31)

Three refinements, adapted from ideas in tobi/qmd (a local markdown
search engine) without taking the dependency — it runs its own
in-process GGUF models, which would mean a second model runtime and a
second embedding universe beside the fleet's existing one.

- **Hits carry their file's description.** A matching line lifted out of
  a long note is ambiguous alone; every `search_memory` hit now includes
  the containing file's H1 plus first prose line. Derived, never
  authored — no frontmatter to maintain, no way for it to drift.
- **Reads can be windowed.** `read_memory` takes `from_line`/`max_lines`
  and reports `total_lines` plus `partial`, so a long note can be read
  in pieces and the caller can tell whether it has the whole thing.
- **`read_many` reads a glob in one call** with a per-file line cap.
  "Show me all the notes" was N round trips; now it is one, and a wide
  pattern stays affordable.

None of this adds an index. The substrate is still markdown in git, and
search is still grep — appropriate while memory holds only the user's
own writing. The trigger for revisiting: searches that miss content the
user knows is there (paraphrase misses), or a memory directory growing
past a megabyte or two. The fleet-native answer then is an optional
index built against the *same* embedding service the knowledge base
uses, not a second one.
