# Multi-platform entry point for Multigravity (Windows/PowerShell)
$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path

# 1. Prefer compiled Go binary if present in bin\
$BinPath = Join-Path $ScriptDir "bin\multigravity.exe"
if (Test-Path $BinPath) {
    & $BinPath @args
    exit $LASTEXITCODE
}

# 2. If Go toolchain is available, compile or run directly
$GoCmd = Get-Command go -ErrorAction SilentlyContinue
if ($GoCmd) {
    $BinDir = Join-Path $ScriptDir "bin"
    if (!(Test-Path $BinDir)) {
        New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
    }
    Push-Location $ScriptDir
    try {
        & go build -o $BinPath ./cmd/multigravity 2>$null
        if ($LASTEXITCODE -eq 0 -and (Test-Path $BinPath)) {
            & $BinPath @args
            exit $LASTEXITCODE
        }
        & go run ./cmd/multigravity @args
        exit $LASTEXITCODE
    } finally {
        Pop-Location
    }
}

Write-Error "Error: multigravity binary not found in $ScriptDir\bin and Go toolchain is not available to build it."
Write-Host "Please install Go (1.23+) or build the binary with: make build" -ForegroundColor Red
exit 1
