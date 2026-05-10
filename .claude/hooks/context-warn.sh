#!/usr/bin/env bash
# Stop hook: warns once per session when transcript reaches ~70% of the
# Opus 4.7 1M-token context window. The user prefers explicit /handoff
# over auto-compact; this fires the reminder at a useful threshold.
#
# Token estimate: bytes/12 (calibrated from a 6.5 MB transcript at 523k
# tokens per /context, ~12.4 chars/token in this JSONL format).

set -u

INPUT=$(cat)
SESSION_ID=$(printf '%s' "$INPUT" | jq -r '.session_id // empty')
[ -z "$SESSION_ID" ] && exit 0

TRANSCRIPT="$HOME/.claude/projects/-home-tim-projects-side-libre-shelf/${SESSION_ID}.jsonl"
[ ! -f "$TRANSCRIPT" ] && exit 0

FLAG="/tmp/claude-ctx-warned-${SESSION_ID}"
[ -f "$FLAG" ] && exit 0

BYTES=$(stat -c %s "$TRANSCRIPT")
TOKENS=$((BYTES / 12))
LIMIT=1000000
THRESHOLD=$((LIMIT * 70 / 100))

if [ "$TOKENS" -gt "$THRESHOLD" ]; then
  touch "$FLAG"
  PCT=$((TOKENS * 100 / LIMIT))
  TOKENS_K=$((TOKENS / 1000))
  printf '{"systemMessage": "Context at ~%d%% (~%dk / 1000k tokens estimated). Run /handoff to save state via bd notes + git, then start a fresh session before performance degrades."}\n' "$PCT" "$TOKENS_K"
fi
exit 0
