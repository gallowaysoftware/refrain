# Deploying recall

One container, one volume. The container image needs the `git` binary
(every write is a commit), which the Dockerfile already installs.

## Bring-up

1. `cp .env.example .env` and replace every `REPLACE-` value:
   `RECALL_IPV4` (a free LAN IP on the macvlan network) and
   `RECALL_DATA` (host path for the memory root).
2. `docker compose up -d --build`
3. Sanity check — from another LAN box, not the docker host (macvlan
   isolation blocks host-to-container traffic):

   ```bash
   curl http://<RECALL_IPV4>:8092/healthz
   ```

The reference compose file assumes a pre-existing macvlan network named
`br0` so sibling containers and LAN/VPN clients reach recall directly by
IP. If that isn't your pattern, replace the `networks` block with a
bridge network and publish port 8092.

On Unraid, add these labels to the service for a WebUI link and shell:

```yaml
    labels:
      net.unraid.docker.webui: "http://${RECALL_IPV4}:8092/healthz"
      net.unraid.docker.shell: "sh"
```

## Endpoints (:8092)

| Endpoint | Purpose |
|---|---|
| `/mcp` | MCP tools (Streamable HTTP): reads and writes during a conversation |
| `GET /digest?expert=<slug>` | Compact markdown digest (`text/markdown`) for injection at chat start |
| `GET /healthz` | Liveness |

## The two-mechanism contract

Memory reaches the model through both endpoints, deliberately:

- **Injection** (`/digest`): a harness-side hook — the Open WebUI inlet
  filter in `integrations/openwebui/`, a Claude Code session hook —
  fetches the digest at chat start and prepends it as system context.
  This is what makes memory reliable: a model cannot search for what it
  doesn't know it forgot.
- **Tools** (`/mcp`): deep recall (read whole files, grep) and all
  writes during the conversation.

Wire up both. Injection without tools means nothing new is ever
remembered; tools without injection means the model starts every chat
amnesiac. See `docs/design.md` for the full rationale.

## Security posture — read this

**recall serves both endpoints unauthenticated, including write tools.**
That is a deployment CHOICE this reference config makes: it assumes a
trusted, private network (LAN/VPN only, no port forwarding, no reverse
proxy exposing it beyond the segment). Anyone who can reach port 8092
can read and mutate every expert's memory.

The mitigations are structural rather than authn: every write is a git
commit (fully auditable, trivially revertible), paths are
slug-validated, and the store is append-or-replace only — no delete
tool exists. If your network is not trusted, put an authenticating
reverse proxy in front before exposing it, or don't deploy this.

## Backups

`RECALL_DATA` is a plain git repository of markdown — the only
irreplaceable data in the stack. Back it up like one: copy it, or
`git remote add` a private remote and push. The service needs no
change either way.
