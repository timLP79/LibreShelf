# Agent Instructions

LibreShelf project agent guide. See `CLAUDE.md` for full project context, collaboration rules,
and checkpoint scope. This file exists for cross-tool consumption (Cursor, Aider, Windsurf, etc.)
and for bd-injected workflow blocks.

This project uses **bd** (beads) for issue tracking and memory. Run `bd prime` for full workflow
context.

## Common Commands

```bash
bd ready                 # Find available work
bd show <id>             # View issue details
bd update <id> --claim   # Claim work
bd close <id>            # Complete work
bd remember "<insight>"  # Persist cross-session knowledge
bd memories              # List remembered insights
bd dolt pull             # Pull beads data from remote (session start)
bd dolt push             # Push beads data to remote
```

## Non-Interactive Shell Commands

**ALWAYS use non-interactive flags** with file operations to avoid hanging on confirmation prompts.

Shell commands like `cp`, `mv`, and `rm` may be aliased to include `-i` (interactive) mode on some systems, causing the agent to hang indefinitely waiting for y/n input.

**Use these forms instead:**
```bash
# Force overwrite without prompting
cp -f source dest           # NOT: cp source dest
mv -f source dest           # NOT: mv source dest
rm -f file                  # NOT: rm file

# For recursive operations
rm -rf directory            # NOT: rm -r directory
cp -rf source dest          # NOT: cp -r source dest
```

**Other commands that may prompt:**
- `scp` - use `-o BatchMode=yes` for non-interactive
- `ssh` - use `-o BatchMode=yes` to fail instead of prompting
- `apt-get` - use `-y` flag
- `brew` - use `HOMEBREW_NO_AUTO_UPDATE=1` env var

<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:ca08a54f -->
## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime` to see full workflow context and commands.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work
bd close <id>         # Complete work
```

### Rules

- Use `bd` for ALL task tracking — do NOT use TodoWrite, TaskCreate, or markdown TODO lists
- Run `bd prime` for detailed command reference and session close protocol
- Use `bd remember` for persistent knowledge — do NOT use MEMORY.md files

## Session Completion

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd dolt push
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
<!-- END BEADS INTEGRATION -->

---

## Project-Specific Overrides

This project uses feature branches with no upstream remote (e.g. `cp6-loans`). The generic
"MUST git push" mandate inside the beads integration block above does NOT apply here. The
correct close-out on this project is:

1. `go build ./...` and `go test ./...` must pass
2. `bd close <id>` for finished work
3. `bd dolt pull` to pull any beads updates from main
4. `git add <files>` and `git commit` on the feature branch
5. Merge to main locally when the branch is ready. No `git push` required.

Full project workflow, checkpoint scope, and collaboration rules live in `CLAUDE.md`.

## Testing Requirements

A beads issue that modifies Go code is NOT done until test coverage exists. When creating an
issue that will touch handlers, DB methods, middleware, or business logic, include tests in
acceptance criteria:

```bash
bd create --title="..." --description="..." --type=feature \
  --acceptance="- Feature behavior implemented
- Unit tests cover happy path and documented error cases
- go test ./... passes"
```

**Test-exempt changes** (closeable without new tests):

- Pure documentation (DECISIONS.md, CLAUDE.md, docs/, README.md)
- Template / CSS / JS (frontend-only, visually verified)
- Schema edits in `createSchema()` -- exercised by handler tests
- Seed data or flash map entries

If Go code lands without tests and isn't exempt, reopen the issue or create a follow-up for
the missing coverage.

## Security Review Requirements

A beads issue that touches handlers, DB methods, middleware, auth/session logic, templates
rendering user-controlled data, or anything related to credentials or permissions is NOT
done until the `security-review` skill has been run on the branch diff and any findings have
been addressed (or explicitly waived in the issue notes).

```
# After implementation + tests, before bd close
Skill: security-review
```

**Review-exempt changes** (closeable without a security pass):

- Pure documentation (DECISIONS.md, CLAUDE.md, docs/, README.md)
- Beads-only updates (.beads/issues.jsonl, notes, descriptions)
- Test-only changes
- Trivial styling (CSS color tweaks, template padding fixes that don't touch user-rendered data)

If a non-exempt change closes without a security pass, reopen the issue or file a follow-up
specifically for the missed review.

## Memory and Task Tracking (HARD RULE)

- **Memory:** `bd remember` only. Do NOT write to the per-device auto-memory system at
  `/home/tim/.claude/projects/.../memory/`. That directory must remain empty.
- **Tasks:** `bd` issues only. Do NOT use `TodoWrite`, `TaskCreate`, or markdown TODO lists.
- **GitHub Issues / Projects:** NOT used as a parallel tracker. Do not open issues on GitHub
  for design notes or internal task tracking. If a GitHub issue arrives via an external
  channel (e.g. customer bug report on a public repo), triage it into bd via `bd create`,
  then close the GitHub issue with a pointer to the bd ID. The GitHub Issues feature stays
  enabled as an inbox; bd is the canonical tracker.

See the Persistence and Memory section of `CLAUDE.md` for the full reasoning.

## Go Tutor Mode

Per `CLAUDE.md`: Tim writes all Go source. Agents do NOT use Write/Edit on `.go` files except
for the documented exceptions -- SQL schema in `createSchema()`, repetitive data entry (seed
data, struct literals, flash map entries), and test files. For HTML templates, CSS, and JS,
agents edit directly.
