param(
  [string]$ApiBaseUrl = "http://42.193.145.61:8080",
  [int]$Port = 5173
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$webDir = Join-Path $repoRoot "web"

if (-not (Test-Path (Join-Path $webDir "package.json"))) {
  Write-Host "[ERROR] web/package.json not found." -ForegroundColor Red
  exit 1
}

if (-not (Get-Command npm -ErrorAction SilentlyContinue)) {
  Write-Host "[ERROR] npm not found. Please install Node.js first." -ForegroundColor Red
  exit 1
}

$env:VITE_API_BASE_URL = $ApiBaseUrl

Write-Host "[INFO] OnlyTun Web dev server" -ForegroundColor Cyan
Write-Host "[INFO] API: $ApiBaseUrl" -ForegroundColor Cyan
Write-Host "[INFO] URL: http://127.0.0.1:$Port" -ForegroundColor Cyan
Write-Host "[INFO] Press Ctrl+C to stop." -ForegroundColor Yellow

Start-Process "http://127.0.0.1:$Port"

Set-Location $webDir
npm run dev -- --host 127.0.0.1 --port $Port
