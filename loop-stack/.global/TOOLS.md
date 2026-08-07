# Global Discovered Tools
## Status
CONFIRMED (resource-scout, full discovery run 2026-08-07, written to loop-stack/validate-play-music-v116/TOOLS.md)
REFRESHED for frontend-design-overhaul 2026-08-07: all entries re-verified live; design-loop notes added.

## Confirmed Local Tools
| Tool | Version | Invocation example |
|---|---|---|
| Go | go1.26.0 windows/amd64 | `go build ./...` / `go test ./...` (module `play-music`, go 1.26) |
| Node | v24.19.0 (npm 12.0.2) | `node -v` |
| Docker | 29.6.2 (build dfc4efb) | `docker ps`, `docker compose up` |
| Git | 2.55.0.windows.3 | `git status`, `git diff --stat`, `git worktree add --detach ...` |
| curl | curl 8.21.0 (Windows, Schannel) | `curl.exe -s -o NUL -w "%{http_code}" http://localhost:4533/` → 200 |

IMPORTANT: `curl` in PowerShell is an alias for `Invoke-WebRequest`. Always invoke the real binary as `curl.exe`.

## MCP Servers & Browser Automation
- **playwright** MCP — configured in `C:\Users\hensi\.config\opencode\opencode.jsonc` (local, `npx --yes @playwright/mcp@latest`, enabled). VERIFIED WORKING 2026-08-07 (and re-verified in the frontend-design-overhaul session): navigate/snapshot/find/click/type/evaluate/console/network/resize/take_screenshot all callable; server http://localhost:4533/ title "Play Music", 0 console errors.
- **vision** subagent — `openrouter/qwen/qwen3.7-flash` via OpenRouter (user opencode.jsonc). Analyze screenshots/OCR. Invoke: `task vision: "analyze <path>: <question>"`. Read-only. **ORCHESTRATOR-ONLY — confirmed NOT callable inside loop subagent contexts** (executor sessions lack the task tool and image input); loop subagents must use programmatic checks (getComputedStyle, overlap scans, window.__player.getState()) instead.
- Project root `opencode.json`: agents loop-triage (primary), implementer, verifier only; NO MCP servers there.
- **gh CLI**: ABSENT (confirmed).

## Environment Notes (verified 2026-08-07)
- **Server serves HEAD, not working tree**: assets are go:embed-embedded at build time. Served style.css (35607 B, CRLF) = `git show HEAD:web/assets/style.css` (33579 B LF) same content; working tree style.css git-CLEAN. Browser QA of design changes impossible until human applies diff + rebuilds + restarts.
- **computed-style verification**: `playwright_browser_evaluate` + getComputedStyle = visual-verification workhorse for design loops (colors, typography, spacing, overflow via scrollWidth, overlaps via getBoundingClientRect).
- **Screenshots** land in `.playwright-mcp/` (server CWD) — always pass explicit filename for vision/human handoff.
- **QA hook**: `window.__player` at web/assets/player.js:324; throwaway-client recipe via `POST /api/store/register` with `localStorage.setItem('pm_token', ...)`.
- **LF-diff capture**: never use PS redirection for git output (UTF-16LE); use `cmd /c "git diff ... > file"`.

## Resource Usage Guide (quick reference)
- `playwright_browser_navigate` url=http://localhost:4533/
- `playwright_browser_snapshot` / `playwright_browser_find` text="..." → element refs
- `playwright_browser_evaluate` function=() => { ... getComputedStyle(el).color ... } → computed-style verification
- `playwright_browser_click` target=<ref> element="desc" (or `playwright_browser_type`/`fill_form`)
- `playwright_browser_console_messages` level=error → JS errors (must be 0)
- `playwright_browser_network_requests` static=false / `playwright_browser_network_request` index=<n> part=response-body
- `playwright_browser_take_screenshot` scale=css filename=.playwright-mcp/<name>.png → explicit filename for vision
- `playwright_browser_resize` width=<px> height=<px> → responsive QA
- `playwright_browser_wait_for` text=... / time=<s> → sync waits
- Vision (orchestrator-only): `task vision: "OCR and analyze <screenshot>: ..."`
- Go: `go build ./...`, `go test ./...`, `go vet ./...` (workdir=project root)
- Liveness: `curl.exe -s -o NUL -w "%{http_code}" --max-time 5 http://localhost:4533/` → 200
- Git: `git status --porcelain`, `git log --oneline -10`, `git branch -a`, `git ls-files -v -- <path>`
- LF-diff: `cmd /c "git diff ... > file"`

## Newly Discovered Resources (Online — Unconfirmed Local)
(empty — researcher-only section)
