# Starts the Play Music Go server if it is not already listening on :4533.
# Used as webServer.command in playwright.config.js — with
# reuseExistingServer: true, Playwright only runs this when the server is
# down. The server process itself is started detached by m1-start.ps1.

$ErrorActionPreference = 'Stop'
$RepoRoot = Split-Path -Parent $PSScriptRoot
$url = 'http://localhost:4533/'

function Test-Health {
  try {
    $r = Invoke-WebRequest -Uri $url -UseBasicParsing -TimeoutSec 3
    return $r.StatusCode -eq 200
  } catch {
    return $false
  }
}

if (Test-Health) {
  exit 0
}

$start = Join-Path $PSScriptRoot '..\teamwork-state\fix-bugs-design-20260808\m1-start.ps1'
if (-not (Test-Path -LiteralPath $start)) {
  $start = Join-Path $RepoRoot 'm1-start.ps1'
}
if (-not (Test-Path -LiteralPath $start)) {
  Write-Error 'm1-start.ps1 not found — cannot boot the server.'
  exit 1
}
& $start -RepoRoot $RepoRoot

$deadline = (Get-Date).AddSeconds(45)
while ((Get-Date) -lt $deadline) {
  if (Test-Health) { exit 0 }
  Start-Sleep -Milliseconds 800
}
Write-Error 'Server did not become healthy on :4533.'
exit 1
