$ErrorActionPreference = "Stop"

$REPO = if ($env:MULTIGRAVITY_REPO) { $env:MULTIGRAVITY_REPO } else { "yegear1/multigravity-cli" }
$BRANCH = if ($env:MULTIGRAVITY_BRANCH) { $env:MULTIGRAVITY_BRANCH } else { "main" }
$RAW = "https://raw.githubusercontent.com/$REPO/$BRANCH"
$INSTALL_DIR = "$env:USERPROFILE\.local\bin"

function Write-Step ($message) {
    Write-Host "  -> $message"
}

function Abort ($message) {
    Write-Error "Error: $message"
    exit 1
}

Write-Host "Installing Multigravity to $INSTALL_DIR ..."

if (!(Test-Path $INSTALL_DIR)) {
    New-Item -ItemType Directory -Force -Path $INSTALL_DIR | Out-Null
}

$IN_PATH = $false
foreach ($path in ($env:PATH -split ';')) {
    if ($path.TrimEnd('\') -eq $INSTALL_DIR.TrimEnd('\')) {
        $IN_PATH = $true
        break
    }
}

if (!$IN_PATH) {
    Write-Step "Adding $INSTALL_DIR to user PATH..."
    $userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
    $newPath = if ($userPath) { "$userPath;$INSTALL_DIR" } else { "$INSTALL_DIR" }
    [Environment]::SetEnvironmentVariable("PATH", $newPath, "User")
    $env:PATH = "$env:PATH;$INSTALL_DIR"
    Write-Host "  Added to PATH! You may need to restart your terminal for changes to take effect."
    Write-Host ""
}

$installed = $false

# Option A: Build from local source if Go is available and inside repository
$GoCmd = Get-Command go -ErrorAction SilentlyContinue
if ((Test-Path ".\cmd\multigravity\main.go") -and $GoCmd) {
    Write-Step "Building binary from local source with Go..."
    & go build -o "$INSTALL_DIR\multigravity.exe" ./cmd/multigravity 2>$null
    if ($LASTEXITCODE -eq 0 -and (Test-Path "$INSTALL_DIR\multigravity.exe")) {
        $installed = $true
    }
}

# Option B: Download pre-compiled release binary
if (!$installed) {
    $assetName = "multigravity-windows-amd64.exe"
    $releaseUrl = "https://github.com/$REPO/releases/latest/download/$assetName"
    Write-Step "Downloading pre-compiled binary ($assetName)..."
    try {
        Invoke-WebRequest -Uri $releaseUrl -OutFile "$INSTALL_DIR\multigravity.exe" -UseBasicParsing -ErrorAction Stop
        $installed = $true
    } catch {
        # Release asset might not be available yet
    }
}

# Option C: Abort if neither local build nor release binary was installed
if (!$installed) {
    Abort "Pre-compiled binary unavailable for windows-$arch and Go toolchain not found. Please install Go (1.23+) or download a binary from https://github.com/$REPO/releases"
}

Write-Step "Creating wrapper script..."
$wrapper = @"
@echo off
"%~dp0multigravity.exe" %*
"@

[System.IO.File]::WriteAllText("$INSTALL_DIR\multigravity.cmd", $wrapper, [System.Text.Encoding]::ASCII)

Write-Host ""
Write-Host "✓ Multigravity installed successfully!"
Write-Host ""
Write-Host "Usage:"
Write-Host "  multigravity help"
Write-Host "  multigravity new <profile-name>"
Write-Host "  multigravity <profile-name>"
