# Open WebUI digest filter

`refrain_digest_filter.py` is the injection half of the memory system
for Open WebUI: an inlet filter that fetches
`GET <REFRAIN_URL>/digest?expert=<slug>` at chat start and prepends the
digest as system context. The MCP tools at `/mcp` are the other half
(deep reads + writes). Any refrain outage fails open — the chat
proceeds without memory rather than erroring.

## Install

1. Admin Panel -> Functions -> "+" -> paste `refrain_digest_filter.py`
   -> save.
2. Enable the filter on each expert workspace model (model editor ->
   Filters).
3. Set the valves:
   - `REFRAIN_URL`: base URL of your refrain service (default
     `http://refrain:8092` works when refrain is reachable by container
     name; use its LAN IP otherwise).
   - `MODEL_EXPERTS`: JSON map of workspace-model id -> refrain expert
     slug. Empty by default — nothing is injected until you fill it.

Example mapping:

```json
{
  "chess-expert": "chess",
  "woodworking-expert": "woodworking"
}
```

Models absent from the mapping pass through untouched.
