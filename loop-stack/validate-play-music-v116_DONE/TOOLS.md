# Discovered Tools
## Status
CONFIRMED (resource-scout, full discovery run 2026-08-07)

Goal context: validate R1-R3 acceptance criteria, player play/pause desync in web/assets/player.js, Dependabot PRs, v1.16.0 features. App is a Go web app ("Play Music") running live on http://localhost:4533/ (HTTP 200 confirmed). Browser QA via Playwright MCP is functional.

## Confirmed Local Tools
| Tool | Version | Invocation example |
|---|---|---|
| Go | go1.26.0 windows/amd64 | `go build ./...` / `go test ./...` (module `play-music`, go 1.26) |
| Node | v24.19.0 (npm 12.0.2) | `node -v` |
| Docker | 29.6.2 (build dfc4efb) | `docker ps`, `docker compose up` (docker-compose.yml exists in project root) |
| Git | 2.55.0.windows.3 | `git status`, `git diff --stat`, `git worktree add ...` |
| curl | curl 8.21.0 (Windows, Schannel) | `curl.exe -s -o NUL -w "%{http_code}" http://localhost:4533/` |

IMPORTANT: `curl` in PowerShell is an alias for `Invoke-WebRequest`. Always invoke the real binary as `curl.exe`, or PS will mangle flags.

## MCP Servers & Browser Automation
- **playwright** MCP — configured in `C:\Users\hensi\.config\opencode\opencode.jsonc` (`"mcp": { "playwright": { "type": "local", "command": ["npx", "--yes", "@playwright/mcp@latest"], "enabled": true } }`). VERIFIED WORKING this session: navigated to http://localhost:4533/, page title "Play Music", PWA install prompt rendered, 0 console errors.
- **vision** subagent — `openrouter/qwen/qwen3.7-flash` via OpenRouter, configured in user opencode.jsonc. Analyze screenshots/OCR. Invoke via task tool: `task vision: "analyze <screenshot path>: <question>"`. Read-only: no edit/bash.
- Project root `opencode.json` has NO MCP servers — only agents loop-triage (primary), implementer, verifier. Agent roster in `.opencode/agents/`: agent-factory, auditor, executor, knowledge-sources, memory-keeper, planner, researcher, resource-scout, verifier.
- **gh CLI**: ABSENT (confirmed `gh` not found).

## Server Health
- `http://localhost:4533/` → HTTP 200, 1541 bytes, `text/html; charset=utf-8`, title "Play Music". LIVE.

## Resource Usage Guide
- Playwright browser QA (primary tool for R1-R3 + player QA):
  - `playwright_browser_navigate` url=http://localhost:4533/ → load app
  - `playwright_browser_snapshot` → accessibility tree (refs for all elements)
  - `playwright_browser_find` text="..." → locate element refs by text
  - `playwright_browser_click` target=<ref> element="description" → click (player play/pause, buttons)
  - `playwright_browser_type` target=<ref> text="..." → fill inputs
  - `playwright_browser_fill_form` → multi-field fill
  - `playwright_browser_console_messages` level=error → JS errors (must be 0)
  - `playwright_browser_network_requests` static=false → API/network activity
  - `playwright_browser_network_request` index=<n> part=response-body → inspect API payloads
  - `playwright_browser_take_screenshot` scale=css filename=loop-stack/validate-play-music-v116/shot.png → screenshot for vision
  - `playwright_browser_wait_for` text="..." / time=<s> → sync waits (use before asserting player state)
  - `playwright_browser_evaluate` function=() => document.querySelector(...).className → DOM state checks (player class toggles)
- Vision analysis: `task vision: "OCR and analyze loop-stack/validate-play-music-v116/shot.png — describe player UI state and any errors"`
- Go checks: `go test ./...` (workdir=project root), `go vet ./...`
- Server liveness: `curl.exe -s -o NUL -w "%{http_code}" --max-time 5 http://localhost:4533/` → expect 200
- Git state: `git status --porcelain`, `git log --oneline -10`, `git branch -a` (for Dependabot PR branches)

## Newly Discovered Resources (Online — Unconfirmed Local)
(empty — researcher-only section)

## Not Relevant / Notes
- No Python/pyproject detected (Go-only project tooling: go.mod + docker-compose.yml, no package.json/Makefile).
- Playwright screenshots/snapshots land in `.playwright-mcp/` inside the workdir by default; pass explicit filenames under the loop dir for vision handoff.
