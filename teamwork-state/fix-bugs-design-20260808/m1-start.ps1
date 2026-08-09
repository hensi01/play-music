# M1 worker: inicia o play-music.exe com env do .env (leitura runtime, sem embutir credenciais)
param(
    [string]$RepoRoot = "C:\Users\hensi\Downloads\MARKETING-ARQUIVOS\MARKETING - ARQUIVOS\programacao\gits\play-music"
)
$ErrorActionPreference = 'Stop'
$envFile = Join-Path $RepoRoot '.env'
Get-Content $envFile | Where-Object { $_ -match '^\s*[A-Za-z_][A-Za-z0-9_]*=' -and $_ -notmatch '^\s*#' } | ForEach-Object {
    $parts = $_ -split '=', 2
    $name = $parts[0].Trim()
    $val = $parts[1].Trim()
    if ($val -match '^".*"$') { $val = $val.Substring(1, $val.Length - 2) }
    [Environment]::SetEnvironmentVariable($name, $val, 'Process')
}
Set-Location -LiteralPath $RepoRoot
$out = Join-Path $RepoRoot 'm1-srv.log'
$err = Join-Path $RepoRoot 'm1-srv-err.log'
$p = Start-Process -FilePath (Join-Path $RepoRoot 'play-music.exe') -RedirectStandardOutput $out -RedirectStandardError $err -PassThru -WindowStyle Hidden
$p.Id | Out-File -FilePath (Join-Path $RepoRoot 'm1-srv.pid') -Encoding ascii
