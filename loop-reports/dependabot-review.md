# Dependabot PR Review — Verdicts for #1 #2 #5 #6 #9 #19

Date: 2026-08-07
Base: `origin/master` @ `8df0521134829be7318ff409da5e8c6b1b821772` (feat v1.16.0)
Mode: report-only — no source changes, no commits, no pushes.

## Summary Table

| PR | Dependabot branch | Dependency / Target | In master? | merge-tree exit (1 = conflict) | Conflict files | Verdict |
|----|-------------------|---------------------|-----------|-------------------------------|----------------|---------|
| #1 | `dependabot/docker/crazy-max/osxcross-26.1-debian` | crazy-max/osxcross (Dockerfile build stage) | NO (Dockerfile rewritten, no osxcross stage) | 1 | Dockerfile (content) | **CLOSE** |
| #2 | `dependabot/go_modules/github.com/pressly/goose/v3-3.27.3` | pressly/goose/v3 v3.27.3 | NO (absent from go.mod/go.sum) | 1 | go.mod, go.sum (content) | **CLOSE** |
| #5 | `dependabot/go_modules/github.com/lestrrat-go/jwx/v3-3.2.0` | lestrrat-go/jwx/v3 v3.2.0 | NO (absent from go.mod/go.sum) | 1 | go.mod, go.sum (content) | **CLOSE** |
| #6 | `dependabot/go_modules/github.com/mattn/go-sqlite3-1.14.49` | mattn/go-sqlite3 v1.14.49 | NO (absent from go.mod/go.sum) | 1 | go.mod, go.sum (content) | **CLOSE** |
| #9 | `dependabot/github_actions/dot-github/workflows/actions/stale-11` | actions/stale 10→11 (`.github/workflows/stale.yml`) | NO (`.github/` deleted from master) | 1 | .github/workflows/stale.yml (modify/delete) | **CLOSE** |
| #19 | `dependabot/go_modules/github.com/gohugoio/hashstructure-1.0.0` | gohugoio/hashstructure v1.0.0 | NO (absent from go.mod/go.sum) | 1 | go.mod, go.sum (content) | **CLOSE** |

**Verdict: all 6 = CLOSE.**

## Root Cause (single, referenced per-PR above)

Commit `b2e353f8` (2026-08-05, ancestor of master) slimmed the module:

1. **go.mod slimming** — removed 4 Go deps: `pressly/goose/v3`, `lestrrat-go/jwx/v3`, `mattn/go-sqlite3`, `gohugoio/hashstructure`. Master's go.mod now requires only: dhowden/tag, golang-jwt/jwt/v5, jackc/pgx/v5, minio-go/v7, redis/go-redis/v9, robfig/cron/v3, golang.org/x/image (+ indirects).
2. **Dockerfile rewrite** — slim 2-stage alpine build (golang:1.26-alpine → alpine:3.21+ffmpeg); the osxcross stage is gone.
3. **`.github/` deletion** — no CI workflows remain on master (PR #9 is dead-on-arrival).

All 6 PRs are single-commit dependency bumps based on pre-slimming commits (`54103a00` for #2 #5 #6 #19; `af5e6277` for #1 #9). Their diffs reference the old fat go.mod / Dockerfile / workflows, so every one conflicts with the slimmed master and, more importantly, would resurrect dead dependencies or deleted files if merged.

## Per-PR Evidence

### #1 — osxcross Docker stage (CLOSE)
- `git merge-tree --write-tree origin/master origin/dependabot/docker/crazy-max/osxcross-26.1-debian` → exit 1
- Conflict: `Auto-merging Dockerfile / CONFLICT (content): Merge conflict in Dockerfile` (base + both sides diverge)
- Presence: `Select-String -Path Dockerfile -Pattern "osxcross"` → no match (exit 1). Master Dockerfile has no osxcross stage, no `--platform/xx` cross-compile.
- Closing this PR cannot add value: the stage it bumps no longer exists.

### #2 — pressly/goose/v3 v3.27.3 (CLOSE)
- `git merge-tree --write-tree origin/master origin/dependabot/go_modules/github.com/pressly/goose/v3-3.27.3` → exit 1
- Conflict: go.mod + go.sum (content) — `CONFLICT (content): Merge conflict in go.mod`
- Presence: `pressly/goose` → no match in go.mod; no goose lines in go.sum.
- Branch tip `8eeefda7` = single bump commit off `54103a00`.

### #5 — lestrrat-go/jwx/v3 v3.2.0 (CLOSE)
- `git merge-tree --write-tree origin/master origin/dependabot/go_modules/github.com/lestrrat-go/jwx/v3-3.2.0` → exit 1
- Conflict: go.mod + go.sum (content).
- Presence: `lestrrat-go/jwx` → no match in go.mod; no jwx lines in go.sum.

### #6 — mattn/go-sqlite3 v1.14.49 (CLOSE)
- `git merge-tree --write-tree origin/master origin/dependabot/go_modules/github.com/mattn/go-sqlite3-1.14.49` → exit 1
- Conflict: go.mod + go.sum (content).
- Presence: `mattn/go-sqlite3` → no match in go.mod; no go-sqlite3 lines in go.sum.

### #9 — actions/stale 10→11 (CLOSE)
- `git merge-tree --write-tree origin/master origin/dependabot/github_actions/dot-github/workflows/actions/stale-11` → exit 1
- Conflict: `CONFLICT (modify/delete): .github/workflows/stale.yml deleted in origin/master and modified in <branch>`
- Presence: `Test-Path .github` → **False**. The whole directory was deleted from master; there is nothing for this workflow to attach to.

### #19 — gohugoio/hashstructure v1.0.0 (CLOSE)
- `git merge-tree --write-tree origin/master origin/dependabot/go_modules/github.com/gohugoio/hashstructure-1.0.0` → exit 1
- Conflict: go.mod + go.sum (content).
- Presence: `gohugoio/hashstructure` → no match in go.mod; no hashstructure lines in go.sum.

## Re-verified Build / Test Evidence (2026-08-07, project root)

| Command | Result | Exit |
|---|---|---|
| `go mod verify` | `all modules verified` | 0 |
| `go mod tidy -diff` | 1 cosmetic delta only: `golang.org/x/crypto v0.31.0` indirect→direct (no version/removal changes) | 1 (diff found — expected) |
| `go build ./...` | clean (no output) | 0 |
| `go test ./...` | PASS — 15 packages; tests only in `internal/phone` + `internal/stream` (ok); rest no test files | 0 |

The tidy delta confirms the slimmed go.mod is complete and stable — removing the 4 dependabot deps left no dangling requirements, and nothing the PRs target is needed by master.

## Go.mod Evidence Table (master @ 8df05211)

**Present (7 direct requires):** github.com/dhowden/tag, github.com/golang-jwt/jwt/v5, github.com/jackc/pgx/v5, github.com/minio/minio-go/v7, github.com/redis/go-redis/v9, github.com/robfig/cron/v3, golang.org/x/image

**Absent (4 dependabot targets):** github.com/pressly/goose/v3 (#2), github.com/lestrrat-go/jwx/v3 (#5), github.com/mattn/go-sqlite3 (#6), github.com/gohugoio/hashstructure (#19)

## Recommendation

**Close all 6 PRs** (`gh pr close 1 2 5 6 9 19` — note: `gh` CLI is NOT installed locally, so the human must run this on their own machine or in the GitHub UI; all 6 are `state=open, base=master` per GitHub API).

Rationale:
- Merging any of them would resurrect dead code paths into the slimmed module or recreate a deleted CI pipeline.
- "Rebase" would produce a no-op or forced-conflict-resolution change with zero functional benefit.
- Future dependabot bumps (if/when new deps land on master) will rebase onto the slimmed master automatically and open fresh PRs — these 6 stale ones are noise.
- Optionally re-enable CI later (new `.github/workflows/` with go build/test on the slimmed module) — but that is a separate, deliberate decision, not something these PRs provide.
