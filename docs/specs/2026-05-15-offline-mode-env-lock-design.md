# Offline Mode Precedence Flip: env-var as deployment lock

Status: Design approved 2026-05-15. Implementation plan to follow.
Owner: Tim Palacios
Related bd issue: cs408-go-stack-85j
Supersedes (in part): DEC-033's precedence subsection. The rest of DEC-033 (env var name, settings key, why-not-auto-detect) still applies.

## Context

A0 shipped DEC-033 with the precedence rule "settings table row wins over the
`LIBRESHELF_OFFLINE` env var." During manual test of PR #78, Tim hit a UX trap:
the admin Settings page writes every known setting on every POST (treating
absent checkboxes as `off`), so a Save-while-exploring with the offline_mode
checkbox unchecked persisted `offline_mode=false` to the DB. On the next
restart with `LIBRESHELF_OFFLINE=true` set, the DB row silently overrode the
env var. OL Lookup returned 200 instead of 503. No log line, no warning.

For the documented audience (prisons, secure facilities, air-gapped deployments)
this is operationally dangerous. An operator who declared the deployment
offline via env var on Day 1 has no protection against a Day-2 admin
inadvertently saving a contradicting DB state on Day 0 of a prior session.

This spec flips the precedence: `LIBRESHELF_OFFLINE=true` becomes a deployment
lock. When set, it overrides the DB. The admin Settings UI shows the toggle
as locked. The handler skips writing the offline_mode row while locked.

## Goals

- `LIBRESHELF_OFFLINE=true` at startup means offline, period. No DB row can
  override it. No accidental Save can flip it. Restart-only mechanism to
  unlock.
- When the env var is not set (or not `true`), the runtime DB-based toggle
  works exactly as A0 documented: admin flips it from the Settings page, the
  setting persists across restarts.
- The admin UI shows the lock state explicitly so an operator never has to
  guess why the toggle does not respond.
- Other settings (currently `staff_can_import_patrons`) are unaffected by
  the offline lock.

## Non-goals

- Locking online mode (`LIBRESHELF_OFFLINE=false`). The asymmetric lock is
  intentional: nobody needs to declare "lock this deployment online" as a
  deployment-time choice. Only `=true` locks. Other values (including
  `false`, `1`, `yes`, unset) leave the DB in control.
- A `/health` or `/status` JSON endpoint exposing the lock state externally.
  Out of scope; the admin UI and startup log are sufficient surfaces.
- Auto-deleting the offline_mode DB row when the env var locks. The lock
  ignores the DB row; the row stays intact so that unsetting the env var
  cleanly restores the prior runtime intent.
- A symmetric "DB locks env var" mechanism. Not needed.
- Migration tooling. The precedence change is read-side only; no schema
  change, no data migration.

## Design decisions captured

| Decision | Value | Rationale |
|----------|-------|-----------|
| Env var role | Deployment lock when `=true` | Matches restricted-network operator mental model |
| Other env var values | No lock; DB controls | YAGNI on the "lock online" case |
| Locked UI | Greyed-out toggle, ON, with explanatory note | Most informative; consistent with the existing form-switch pattern |
| Locked POST behavior | Skip writing offline_mode entirely | Least destructive; preserves DB intent for the unlocked-later case |
| Visibility | Startup log + admin UI note | Sufficient surfaces; no new endpoints |
| Strictness on env var values | Only `"true"` (case-insensitive) locks | Matches existing `strings.EqualFold` parsing in init() |

## Architecture

### Predicate

`network.go` simplifies. The two-arg testable inner (`isExternalAllowedFn(dm,
offlineFromEnv)`) is replaced. New shape:

```go
// IsExternalAllowed returns true when external HTTP calls are permitted
// for this deployment. When LIBRESHELF_OFFLINE=true is set at startup,
// the env var acts as a deployment lock: it overrides any DB row.
// Otherwise the offline_mode row in the settings table controls; default
// false (external HTTP allowed).
func IsExternalAllowed(dm *DatabaseManager) bool {
    if offlineEnvDefault {
        return false
    }
    return isExternalAllowedFromDB(dm)
}

// isExternalAllowedFromDB reads the offline_mode settings row. Used by
// IsExternalAllowed when the env var is not locking, and by tests that
// want to exercise the DB-path branches without manipulating the env-var
// package var.
func isExternalAllowedFromDB(dm *DatabaseManager) bool {
    v, err := dm.GetSetting("offline_mode")
    if err != nil {
        log.Printf("IsExternalAllowed: GetSetting failed, defaulting to allow: %v", err)
        return true
    }
    if v == "" {
        return true
    }
    return !strings.EqualFold(v, "true")
}

// IsOfflineEnvLocked returns true when the LIBRESHELF_OFFLINE env var
// is locking offline mode (set to "true" at startup). The admin
// Settings page uses this to render the toggle as locked.
func IsOfflineEnvLocked() bool {
    return offlineEnvDefault
}
```

DB-error policy change: the previous logic returned `!offlineFromEnv` on a
DB fault (fail-closed if env says offline, fail-open if env says online).
The new logic always returns `true` (fail-open / allow external HTTP) when
the DB faults AND the env var is not locking. The env-var lock is checked
first, so if `offlineEnvDefault` is true, the lock fires before the DB read
even happens. This is simpler and safer: a transient DB fault cannot
accidentally flip a previously-offline deployment to online while the env
var is locking, because the env-var check runs first.

### Startup log

In `main.go`, after `NewDatabaseManager` returns but before the seed-cover
backfill, log a single line when locked:

```go
if IsOfflineEnvLocked() {
    log.Printf("LIBRESHELF_OFFLINE=true is locking offline mode; runtime DB setting will be ignored until the env var is unset.")
}
```

One emission per process lifetime, at startup. Not in `init()` (no logger
configured yet there).

### Handler changes

`HandleSettings` (GET):

```go
"Settings": gin.H{
    "StaffCanImportPatrons": dm.GetSettingBool("staff_can_import_patrons", false),
    "OfflineMode":           dm.GetSettingBool("offline_mode", false),
    "OfflineModeLocked":     IsOfflineEnvLocked(),
},
```

The previous `GetSettingBool("offline_mode", offlineEnvDefault)` is replaced
with a `false` default. When the env var is locking, the template branches
on `OfflineModeLocked` and shows `checked disabled` regardless of the DB
value. When the env var is not locking, the template shows the DB value.

`HandleSettingsPost`:

```go
type setting struct {
    key       string
    field     string
    skipWrite bool
}
for _, s := range []setting{
    {"staff_can_import_patrons", "staff_can_import_patrons", false},
    {"offline_mode", "offline_mode", IsOfflineEnvLocked()},
} {
    if s.skipWrite {
        continue
    }
    value := "false"
    if c.PostForm(s.field) == "on" {
        value = "true"
    }
    if err := dm.SetSetting(s.key, value, user.ID); err != nil {
        // existing error handling
    }
}
```

A disabled checkbox does not submit a form value, so under normal browser
behavior the form will not include `offline_mode` while locked. The
`skipWrite` flag is a belt-and-suspenders defense against a crafted POST
that includes a contradicting value.

### Template changes

`templates/admin_settings.html`, the "External API access" card body
becomes:

```html
<div class="form-check form-switch">
    <input class="form-check-input" type="checkbox" role="switch"
           id="offline_mode" name="offline_mode"
           {{if .Settings.OfflineModeLocked}}checked disabled{{else if .Settings.OfflineMode}}checked{{end}}>
    <label class="form-check-label" for="offline_mode">
        <strong>Offline mode (disable external API calls)</strong>
    </label>
    <div class="form-text small">
        {{if .Settings.OfflineModeLocked}}
            Locked by the <code>LIBRESHELF_OFFLINE</code> env var.
            Clear the env var and restart the server to edit this
            setting from the UI.
        {{else}}
            When enabled, LibreShelf will not contact Open Library
            (or future external metadata sources) for cover art
            or book information. Book entry continues to work
            using whatever information staff type in manually.
            Use this for deployments without internet access or
            where outbound HTTP is policy-restricted.
        {{end}}
    </div>
</div>
```

The visible label, role, and id stay constant so screen readers and the
existing CSS form-switch styling work consistently. Only `checked`,
`disabled`, and the help copy branch.

## Testable seam

`offlineEnvDefault` is a package var captured in `init()` from
`os.Getenv("LIBRESHELF_OFFLINE")`. Tests cannot easily flip this after
init runs. Solution: a small test-only helper in a new file (or appended
to `network_test.go` if small enough):

```go
// withOfflineEnvDefault swaps offlineEnvDefault for the duration of the
// test function and restores the prior value on Cleanup. Use this in
// tests that need to exercise the env-locked branches without invoking
// os.Setenv (which would not retroactively change the package var).
func withOfflineEnvDefault(t *testing.T, locked bool) {
    t.Helper()
    prior := offlineEnvDefault
    offlineEnvDefault = locked
    t.Cleanup(func() { offlineEnvDefault = prior })
}
```

Tests call `withOfflineEnvDefault(t, true)` at the top of any env-locked
test. The helper restores cleanly even when tests panic or fail.

## Tests

### Predicate tests (modify existing)

- `TestIsExternalAllowed_EnvDefaultTrue_NoSettingsRow` -- stays. Asserts
  env=true with no DB row returns false (external NOT allowed). The
  predicate's behavior here is identical under both the old and new
  precedence rules.
- `TestIsExternalAllowed_EnvDefaultOffline_NoSettingsRow` -- rename to
  `TestIsExternalAllowed_EnvDefaultFalse_NoSettingsRow` and keep behavior:
  env=false with no DB row returns true (allowed). Identical.
- `TestIsExternalAllowed_SettingsOverridesEnvToOffline` -- stays.
  env=false, DB=true (offline), result=false (NOT allowed). DB still wins
  when env is not locking.
- `TestIsExternalAllowed_SettingsOverridesEnvToOnline` -- this is the
  test that flips. Rename to
  `TestIsExternalAllowed_EnvLockBeatsDBOnline` and flip the expected
  result: env=true (locking), DB=false (online), result=false (NOT
  allowed). Documents the new precedence contract explicitly.
- `TestIsExternalAllowed_DBErrorFallsBackToEnvDefault` -- rename to
  `TestIsExternalAllowed_DBErrorWhenUnlockedFallsBackToAllow` and update.
  The new fallback is "allow" (true) when env is not locking. Keep the
  symmetric `TestIsExternalAllowed_DBErrorWhenLockedStaysBlocked` as a
  new test: env=true with broken DM still returns false (lock fires
  before the DB read).

### New predicate tests

- `TestIsOfflineEnvLocked_ReturnsEnvDefault` -- with the helper set to
  true, `IsOfflineEnvLocked()` returns true; with helper set to false,
  returns false. One-line sanity check on the exported accessor.

### New handler tests

- `TestSettingsPageGET_RendersLockedToggleWhenEnvLocked` -- call
  `withOfflineEnvDefault(t, true)`, GET /admin/settings, assert response
  body contains both `disabled` and the lock copy (e.g. search for the
  string `LIBRESHELF_OFFLINE`).
- `TestSettingsPagePOST_SkipsOfflineModeWhenEnvLocked` -- call
  `withOfflineEnvDefault(t, true)`. POST with `offline_mode=on` in the
  form (simulating a crafted submission). Assert `dm.GetSetting("offline_mode")`
  returns empty (no row written). Assert `staff_can_import_patrons` still
  writes correctly in the same POST (use `staff_can_import_patrons=on`
  and assert the row is true).

### Existing tests unchanged

- `TestSettingsPagePOST_FlipsOfflineModeOn` and
  `TestSettingsPagePOST_FlipsOfflineModeOff` -- both run with
  `offlineEnvDefault=false` (the test default), so the lock never fires.
  Existing behavior preserved.
- `TestSettingsPagePOST_BothTogglesWrittenEveryPOST` -- same reasoning.
- `TestSettingsPageGET_RendersOfflineToggle` -- still passes; the toggle
  is rendered in both locked and unlocked states.
- All handler 503 tests -- unchanged.
- All seed-backfill tests -- unchanged (those use SetSetting to flip the
  DB row, not the env var).

## Documentation

### DEC-034 (new)

New entry in `DECISIONS.md` appended after DEC-033. Records the
precedence flip with a Date line, a Decision paragraph, a Why paragraph
linking to cs408-go-stack-85j and the manual-test incident, a list of
behavior changes (predicate, handler, template), and a Related line
pointing to DEC-033 and this spec.

DEC-033 itself gets a one-line callout near the top of its precedence
paragraph: "**Superseded in part by DEC-034 (2026-05-15):** the precedence
rule was flipped. See DEC-034 for current behavior." The rest of DEC-033
(env var name, settings key, why-not-auto-detect) stays accurate and is
not modified.

### deployment.md

The `LIBRESHELF_OFFLINE` subsection is rewritten. Old text:

> Set to true to declare this deployment offline at startup. When on,
> LibreShelf does not attempt any outbound HTTP for Open Library lookup,
> seed-cover backfill, or future external metadata sources. Admin can
> also flip this at runtime via Settings; the runtime setting wins over
> the env var.

New text:

> Set to `true` to lock this deployment in offline mode. When set,
> LibreShelf does not attempt any outbound HTTP for Open Library lookup,
> seed-cover backfill, or future external metadata sources. The admin
> Settings UI shows the offline-mode toggle as locked and disabled; it
> cannot be edited until the env var is unset and the server is
> restarted.
>
> When the env var is not set (or set to any value other than `true`),
> the admin Settings UI is the source of truth for offline mode. Admin
> can flip the setting at runtime; the DB row persists across restarts.
>
> Use the env-var lock for deployments where outbound internet access is
> unavailable or policy-restricted (prisons, secure facilities,
> air-gapped networks). Use the runtime UI toggle for deployments that
> need temporary offline mode (network maintenance windows, etc).

## Implementation phasing

Single PR. No splitting. The change is internally cohesive: predicate +
handler + template + tests + docs all describe one precedence rule
change.

Commit shape (TDD-ordered):

1. Rewrite predicate tests for the new precedence + add new tests for
   env-lock-beats-DB and DB-error-when-locked.
2. Rewrite predicate (`network.go`) to match the new tests.
3. Add new handler tests for the locked GET + locked POST behavior.
4. Update handler + template to match the new tests.
5. Add the startup log line in `main.go`.
6. Update DEC-033 with the superseded-in-part callout. Add DEC-034.
   Update deployment.md.

Branch name: `feat/offline-mode-env-lock` (or similar; locked-in at
plan-writing time).

## Open follow-ups (out of scope here)

- `cs408-go-stack-di7` (SaveCoverFromURL gate) still needs to be resolved
  before Subproject A (Google Books) merges. Independent from this
  precedence work.
- The cross-toggle non-interference test that was added in PR #78 (commit
  77f77fb) still passes unchanged under the new precedence. The test's
  documentation comment about "every POST is 'set state from form,' not
  'flip whatever's in the form'" remains accurate for unlocked behavior
  on every setting and for locked behavior on every setting other than
  offline_mode. The comment does not need updating; the locked-skip
  behavior is documented in the new locked-POST test.

## References

- DEC-033 (`DECISIONS.md`) -- the original A0 precedence decision being
  superseded in part.
- bd issue `cs408-go-stack-85j` -- the bug report and four-option sketch
  this brainstorm narrowed down.
- bd issue `cs408-go-stack-ahq` -- the parent A0 work that shipped
  DEC-033 and surfaced this issue during manual test.
- `docs/specs/2026-05-15-google-books-fallback-design.md` -- the A0+A
  design spec; this precedence change does not alter Subproject A's
  design, only its predicate consumption.
- A0 PR #78 manual-test session log -- where the precedence trap was
  first observed (OL Lookup returned 200 despite `LIBRESHELF_OFFLINE=true`).
