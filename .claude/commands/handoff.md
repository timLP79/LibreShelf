Save the current session's state for handoff to a fresh session. Tim prefers explicit handoff over auto-compact -- compaction's heuristic summarization degrades performance vs. starting fresh with crisp bd notes.

Execute this protocol:

1. **Identify the active bd issue(s).** Run `bd list --status=in_progress` and `bd ready -n 3`. If no issue is in_progress and we've been working on something untracked, ASK before filing one -- don't create an issue silently.

2. **Update the active bd issue(s) via `bd update <id> --notes`** with a handoff brief that includes:
   - Current branch + latest commit SHA (`git log -1 --oneline`)
   - One-line "where I am" summary
   - What's done so far this session (commits or pending edits)
   - What's next when work resumes
   - Any open design questions that paused work
   - Pointers to specific files / line numbers if mid-edit
   - Anything the next session would otherwise have to rediscover

3. **Reconcile uncommitted changes** via `git status`:
   - If they're a logical chunk: commit with a normal message
   - If they're mid-edit (broken / WIP): commit with `wip: handoff -- <brief>` and note in the bd notes that the working tree is broken at this commit
   - If they're trivial scratch (e.g. `.beads/issues.jsonl` only): include in the bd-update commit
   - Do NOT discard uncommitted work without asking

4. **Push state:**
   ```
   git pull --rebase
   git push
   git status   # must show "up to date with origin"
   ```

5. **Print a one-line summary** to the user:
   `Handoff saved to <bd_id> at <commit>. Resume on next session: \`bd show <bd_id>\`.`

After this, the user typically runs `/clear` and starts fresh. Do NOT run `/clear` yourself -- that is the user's call.

Why this exists: the project's auto-compact warning fires at ~70% context (configured in `.claude/hooks/context-warn.sh`). When it does, run this protocol rather than letting the harness auto-compact -- the bd-notes-based handoff produces cleaner next-session starts than the compact algorithm's heuristic summary.
