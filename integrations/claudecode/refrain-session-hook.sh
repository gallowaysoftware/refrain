#!/bin/sh
# refrain SessionStart hook for Claude Code — the INJECTION half of the
# memory system for CLI coding sessions (sibling of the Open WebUI inlet
# filter in integrations/openwebui). It fetches the expert's memory
# digest from refrain and prints it; Claude Code adds a SessionStart
# hook's stdout to the session context, so every session starts knowing
# the user's profile, goals, recent decisions, course mastery, and where
# the last session left off.
#
# Config is env vars, set on the hook command in settings.json:
#   REFRAIN_URL     base URL of the refrain service
#                   (default http://127.0.0.1:8092, the deploy port)
#   REFRAIN_EXPERT  expert slug to fetch the digest for
#                   (unset = silent no-op, so one settings.json can be
#                   shared across machines that track different domains)
#
# Fail-open by design: any outage, timeout, or misconfiguration exits 0
# with no output — a session without memory beats a session that fails
# to start.
set -u

[ "${REFRAIN_EXPERT:-}" ] || exit 0
url=${REFRAIN_URL:-http://127.0.0.1:8092}

digest=$(curl -fsS --max-time 3 "$url/digest?expert=$REFRAIN_EXPERT" 2>/dev/null) || exit 0
[ "$digest" ] || exit 0

printf '%s\n\n%s\n' \
  "The following is your persistent memory for this user in this domain, maintained via the refrain MCP tools:" \
  "$digest"
