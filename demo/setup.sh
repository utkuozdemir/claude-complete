#!/usr/bin/env bash
# Creates the home directory the demo is recorded in: a project with a few sessions, some of them named,
# and the MCP servers and models of a made-up account. The real ~/.claude never ends up in the recording.
set -euo pipefail

home="$1"
project="$home/work/acme-api"
transcripts="$home/.claude/projects/$(printf '%s' "$project" | sed 's/[^a-zA-Z0-9]/-/g')"

rm -rf "$home"
mkdir -p "$project" "$transcripts"

# id, custom title (empty for an unnamed session), AI title, linked pull request (0 for none), newest first
sessions=(
  "4f9c2a1e-8b7d-4c3a-9e5f-1a2b3c4d5e6f|fix-login-redirect|Fix the login redirect loop after SSO|142"
  "7d3e9b2c-1f4a-4e8d-b6c5-2d3e4f5a6b7c||Add retries to the webhook sender|157"
  "a1b2c3d4-5e6f-4a7b-8c9d-0e1f2a3b4c5d|release-v2.4|Write the release notes for 2.4|0"
  "c9d8e7f6-a5b4-4c3d-8e2f-1a0b9c8d7e6f||Find out why the dashboard query is slow|0"
  "e2f3a4b5-c6d7-4e8f-9a0b-1c2d3e4f5a6b||Move the config loader to env variables|161"
  "f6e5d4c3-b2a1-4f0e-9d8c-7b6a5f4e3d2c|flaky-ci|Track down the flaky integration test|0"
)

age=0
for entry in "${sessions[@]}"; do
  IFS='|' read -r id title ai pr <<<"$entry"
  file="$transcripts/$id.jsonl"

  {
    printf '{"type":"user","sessionId":"%s","message":{"role":"user","content":"%s"}}\n' "$id" "$ai"
    printf '{"type":"ai-title","aiTitle":"%s","sessionId":"%s"}\n' "$ai" "$id"
    [ -n "$title" ] && printf '{"type":"custom-title","customTitle":"%s","sessionId":"%s"}\n' "$title" "$id"
    [ "$pr" != 0 ] && printf '{"type":"pr-link","prNumber":%s,"prRepository":"acme/api","sessionId":"%s"}\n' "$pr" "$id"
    true
  } >"$file"

  touch -d "@$(($(date +%s) - age * 3600))" "$file"
  age=$((age + 5))
done

cat >"$home/.claude.json" <<JSON
{
  "mcpServers": {
    "github": {"type": "http", "url": "https://api.githubcopilot.com/mcp/"},
    "sentry": {"type": "http", "url": "https://mcp.sentry.dev/mcp"},
    "postgres": {"command": "npx", "args": ["@modelcontextprotocol/server-postgres"]}
  },
  "additionalModelOptionsCache": [
    {"value": "claude-opus-5-5[1m]", "description": "Opus 5.5 with 1M context"},
    {"value": "claude-sonnet-5-5[1m]", "description": "Sonnet 5.5 with 1M context"}
  ]
}
JSON

# the first completion after an update reads the help of every command, which is not what the demo is about
HOME="$home" XDG_CACHE_HOME="$home/.cache" claude-complete _carapace export claude "" >/dev/null
