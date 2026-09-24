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

# 3. Fallback to legacy PowerShell implementation
$LegacyScript = Join-Path $ScriptDir "legacy\multigravity.ps1"
if (Test-Path $LegacyScript) {
    & $LegacyScript @args
    exit $LASTEXITCODE
}

Write-Error "Error: multigravity binary not found and neither Go toolchain nor legacy script is available."
exit 1
