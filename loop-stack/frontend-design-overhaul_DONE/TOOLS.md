# Discovered Tools
## Status
REUSED FROM GLOBAL (cached 2026-08-07) + REFRESHED for frontend-design-overhaul loop (resource-scout, 2026-08-07). All verified live.

## Confirmed Local Tools
| Tool | Version | Invocation example |
|---|---|---|
| Go | go1.26.0 windows/amd64 | `go build ./...` / `go test ./...` (module `play-music`, go 1.26) |
| Node | v24.19.0 (npm 12.0.2) | `node -v` |
| Git | 2.55.0.windows.3 | `git status`, `git diff --stat`, `git worktree add --detach ...` |
| curl | curl 8.21.0 (Windows, Schannel, libcurl/8.21.0) | `curl.exe -s -o NUL -w "%{http_code}" --max-time 5 http://localhost:4533/` → 200 |

IMPORTANT: `curl` in PowerShell is an alias for `Invoke-WebRequest`. Always invoke the real binary as `curl.exe`.

## MCP Servers & Browser Automation
- **playwright** MCP — configured in `C:\Users\hensi\.config\opencode\opencode.jsonc` (local, `npx --yes @playwright/mcp@latest`). VERIFIED AVAILABLE in this session: playwright_browser_navigate / snapshot / find / click / type / evaluate / console_messages / network_requests / resize / take_screenshot / wait_for all callable. Server: http://localhost:4533/ title "Play Music", 0 console errors (prior loop).
- **vision** subagent (`openrouter/qwen/qwen3.7-flash`) — **ORCHESTRATOR-ONLY**. Confirmed by prior loop: NOT callable inside loop subagent contexts (executor sessions lack the task tool and image input). Loop subagents MUST use programmatic checks instead (getComputedStyle, element-overlap scans, window.__player.getState()) and leave screenshots for human/orchestrator vision follow-up.
- **gh CLI**: ABSENT (confirmed). Map PRs→branches via `git ls-remote origin refs/pull/<n>/head` if ever needed.
- Project root `opencode.json`: agents loop-triage (primary), implementer, verifier only; NO MCP servers there.

## Design-Loop Specific Verification Notes
- **computed-style verification = the visual-verification workhorse for this loop**: `playwright_browser_evaluate` with `getComputedStyle` (element → backgroundColor/color/fontSize/display/gap/padding/margin/borderRadius, `element.scrollWidth <= document.documentElement.clientWidth` for horizontal overflow, `getBoundingClientRect` overlap checks). Run these instead of screenshots/vision.
- **Screenshot output dir**: `.playwright-mcp/` (playwright server CWD) — always pass an explicit filename (e.g. `.playwright-mcp/loop-name-shot.png` or under loop dir) so vision handoff/human review finds the right file.
- **Server serves HEAD, not working tree**: assets are go:embed-embedded at build time. Verified today: served style.css (35607 B, CRLF-embedded) matches `git show HEAD:web/assets/style.css` (33579 B LF) — same logical content. Working tree `web/assets/style.css` is git-CLEAN (uncommitted changes exist only in internal/server/helpers.go, internal/store/common.go, internal/store/users.go, web/assets/player.js). Design changes are NOT visible in browser QA until human applies diff + rebuilds + restarts server. Verify served-vs-HEAD with: first-60-chars compare + byte length (LF→CRLF conversion adds ~1 byte/line, so compare CONTENT, not byte count).
- **QA hook**: `window.__player = { getState: () => state, audio }` at web/assets/player.js:324 — assert player state via `playwright_browser_evaluate`.
- **Throwaway-client QA recipe** (if login flow is touched): public `POST /api/store/register {"phone":"<throwaway>"}` → token → `localStorage.setItem('pm_token', token)` → reload; re-call with `categoryIds:[<id>]` to grant categories; delete the user afterwards.
- **LF-diff capture**: PowerShell `>` writes UTF-16LE — never redirect git output with PS redirection; use `cmd /c "git diff ... > file"` for LF diffs.

## Resource Usage Guide (quick reference)
- `playwright_browser_navigate` url=http://localhost:4533/ → load app
- `playwright_browser_snapshot` / `playwright_browser_find` text="..." → element refs
- `playwright_browser_evaluate` function=() => { const el=document.querySelector('...'); return getComputedStyle(el).color } → computed-style verification (workhorse)
- `playwright_browser_click` target=<ref> element="desc" (or `playwright_browser_type`/`fill_form`)
- `playwright_browser_console_messages` level=error → JS errors (must be 0)
- `playwright_browser_network_requests` static=false / `playwright_browser_network_request` index=<n> part=response-body
- `playwright_browser_take_screenshot` scale=css filename=.playwright-mcp/<loop>-<what>.png → explicit filename for handoff
- `playwright_browser_resize` width=<px> height=<px> → responsive QA (<768px, desktop)
- `playwright_browser_wait_for` text=... / time=<s> → sync waits
- Vision: orchestrator-only → `task vision: "analyze <path>: <question>"` (NEVER from loop subagents)
- Liveness: `curl.exe -s -o NUL -w "%{http_code}" --max-time 5 http://localhost:4533/` → 200
- Served CSS vs HEAD: `curl.exe -s http://localhost:4533/style.css` vs `git show HEAD:web/assets/style.css` — compare first 60 chars, not byte count
- Go build (after human applies diff): `go build ./...` (workdir=project root)
- Git: `git status --porcelain`, `git log --oneline -10`, `git ls-files -v -- <path>` (assume-unchanged flag)

## Newly Discovered Resources (Online — Unconfirmed Local)
(empty — researcher-only section)
