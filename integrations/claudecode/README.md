# Claude Code SessionStart hook

`refrain-session-hook.sh` is the injection half of the memory system for
Claude Code: a SessionStart hook that fetches
`GET <REFRAIN_URL>/digest?expert=<slug>` and prints it. Claude Code adds
a SessionStart hook's stdout to the session context, so every session
opens already knowing the expert's profile, goals, recent decisions,
mastery state, and last session summary. The MCP tools at `/mcp` are the
other half (deep reads and writes during the session).

Any refrain outage fails open — the session starts without memory rather
than erroring.

## Wiring

Register the hook in `~/.claude/settings.json` (global) or
`.claude/settings.json` (per project — the better fit, since the expert
is usually project-scoped):

```json
{
  "hooks": {
    "SessionStart": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "REFRAIN_EXPERT=etude-x4 /path/to/refrain/integrations/claudecode/refrain-session-hook.sh"
          }
        ]
      }
    ]
  }
}
```

Env vars, set on the command as above:

- `REFRAIN_EXPERT`: the expert slug to fetch the digest for. Unset =
  silent no-op, so a shared settings file degrades cleanly on machines
  that do not run refrain.
- `REFRAIN_URL`: base URL of the refrain service. Default
  `http://127.0.0.1:8092` (the deploy port); set it when refrain runs on
  another host or port.

Requires `curl` on `PATH` and the script to be executable
(`chmod +x refrain-session-hook.sh`).

For the write side, also register refrain's MCP tools so the agent can
record decisions, notes, and session logs during the session:

```bash
claude mcp add --transport http refrain http://127.0.0.1:8092/mcp
```
