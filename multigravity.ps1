<#
.SYNOPSIS
Run multiple Antigravity IDE profiles at the same time.
#>

param (
    [Parameter(Position = 0, Mandatory = $false)]
    [string]$cmd,
    
    [Parameter(Position = 1, Mandatory = $false)]
    [string]$arg1,

    [Parameter(Position = 2, Mandatory = $false)]
    [string]$arg2,

    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$ForwardArgs
)

$REAL_USERPROFILE = if ($env:REAL_USERPROFILE) { $env:REAL_USERPROFILE } else { $env:USERPROFILE }
if ($REAL_USERPROFILE -like "*\AntigravityProfiles\*") {
    $parent = Split-Path -Parent $REAL_USERPROFILE
    if ((Split-Path -Leaf $parent) -eq "AntigravityProfiles") {
        $REAL_USERPROFILE = Split-Path -Parent $parent
    }
}
$BASE = if ($env:MULTIGRAVITY_HOME) { $env:MULTIGRAVITY_HOME } else { "$REAL_USERPROFILE\AntigravityProfiles" }
$VERSION = "1.5.0"

function Find-Antigravity {
    $override = if ($env:MULTIGRAVITY_APP) { $env:MULTIGRAVITY_APP } else { $env:AGY_APP }
    if ($override -and (Test-Path $override)) { return $override }

    $paths = @(
        "$env:LOCALAPPDATA\Programs\Antigravity\Antigravity.exe",
        "$env:LOCALAPPDATA\Programs\antigravity\antigravity.exe",
        "$env:PROGRAMFILES\Antigravity\Antigravity.exe",
        "${env:ProgramFiles(x86)}\Antigravity\Antigravity.exe",
        "$env:LOCALAPPDATA\Programs\agy\agy.exe",
        "$env:PROGRAMFILES\agy\agy.exe",
        "$REAL_USERPROFILE\scoop\apps\antigravity\current\antigravity.exe",
        "$REAL_USERPROFILE\scoop\apps\agy\current\agy.exe"
    )
    foreach ($p in $paths) {
        if (Test-Path $p) { return $p }
    }
    
    # Try to find in PATH
    $exeCommand = Get-Command antigravity.exe -ErrorAction SilentlyContinue
    if ($exeCommand) { return $exeCommand.Source }

    $agyCommand = Get-Command agy.exe -ErrorAction SilentlyContinue
    if ($agyCommand) { return $agyCommand.Source }
    
    return $null
}

function Find-LanguageServer {
    $app = Find-Antigravity
    if ($app) {
        $dir = Split-Path -Parent $app
        $cand = Join-Path $dir "resources\bin\language_server.exe"
        if (Test-Path $cand) { return $cand }
    }
    $candidates = @(
        "$env:LOCALAPPDATA\Programs\Antigravity\resources\bin\language_server.exe",
        "$env:LOCALAPPDATA\Programs\antigravity\resources\bin\language_server.exe",
        "$env:PROGRAMFILES\Antigravity\resources\bin\language_server.exe",
        "$env:LOCALAPPDATA\Programs\agy\resources\bin\language_server.exe",
        "$env:PROGRAMFILES\agy\resources\bin\language_server.exe"
    )
    foreach ($c in $candidates) {
        if (Test-Path $c) { return $c }
    }
    return $null
}

$APP = Find-Antigravity

function Get-TemplatesDir {
    return "$BASE\.templates"
}

function Get-SystemDataDir {
    return "$env:APPDATA\Antigravity"
}

function Get-SystemExtensionsDir {
    return "$REAL_USERPROFILE\.antigravity\extensions"
}

function Test-SharedProfile {
    param($name)
    return Test-Path "$BASE\$name\.shared"
}

function Link-DevDotfiles {
    param($profileDir)
    if (Test-Path "$profileDir\.isolated_dotfiles") { return }

    $realUser = if ($env:REAL_USERPROFILE) { $env:REAL_USERPROFILE } else { $REAL_USERPROFILE }
    if ([string]::IsNullOrEmpty($realUser)) { $realUser = $env:USERPROFILE }

    $gitConfig = "$realUser\.gitconfig"
    $sshDir    = "$realUser\.ssh"

    if ((Test-Path $gitConfig) -and !(Test-Path "$profileDir\.gitconfig")) {
        New-Item -ItemType SymbolicLink -Path "$profileDir\.gitconfig" -Target $gitConfig -ErrorAction SilentlyContinue | Out-Null
    }
    if ((Test-Path $sshDir) -and !(Test-Path "$profileDir\.ssh")) {
        New-Item -ItemType Junction -Path "$profileDir\.ssh" -Target $sshDir -ErrorAction SilentlyContinue | Out-Null
    }
    $gitCreds = "$realUser\.git-credentials"
    if ((Test-Path $gitCreds) -and !(Test-Path "$profileDir\.git-credentials")) {
        New-Item -ItemType SymbolicLink -Path "$profileDir\.git-credentials" -Target $gitCreds -ErrorAction SilentlyContinue | Out-Null
    }
}

function Link-McpConfig {
    param($profileDir)
    if (Test-Path "$profileDir\.isolated_mcp") { return }

    $realUser = if ($env:REAL_USERPROFILE) { $env:REAL_USERPROFILE } else { $REAL_USERPROFILE }
    if ([string]::IsNullOrEmpty($realUser)) { $realUser = $env:USERPROFILE }

    $hostMcpConfig = "$realUser\.gemini\config\mcp_config.json"
    $targetConfigDir = "$profileDir\.gemini\config"
    $targetMcpConfig = "$targetConfigDir\mcp_config.json"

    # 1. Link mcp_config.json
    if ((Test-Path $hostMcpConfig) -and !(Test-Path $targetMcpConfig)) {
        if (!(Test-Path $targetConfigDir)) {
            New-Item -ItemType Directory -Force -Path $targetConfigDir | Out-Null
        }
        New-Item -ItemType SymbolicLink -Path $targetMcpConfig -Target $hostMcpConfig -ErrorAction SilentlyContinue | Out-Null
    }

    # 2. Link schemas directory ~/.gemini/antigravity/mcp
    $hostMcpSchemas = "$realUser\.gemini\antigravity\mcp"
    $targetAntigravityDir = "$profileDir\.gemini\antigravity"
    $targetMcpSchemas = "$targetAntigravityDir\mcp"

    if ((Test-Path $hostMcpSchemas) -and !(Test-Path $targetMcpSchemas)) {
        if (!(Test-Path $targetAntigravityDir)) {
            New-Item -ItemType Directory -Force -Path $targetAntigravityDir | Out-Null
        }
        New-Item -ItemType Junction -Path $targetMcpSchemas -Target $hostMcpSchemas -ErrorAction SilentlyContinue | Out-Null
    }
}

function Link-SkillsConfig {
    param($profileDir)
    if (Test-Path "$profileDir\.isolated_skills") { return }

    $realUser = if ($env:REAL_USERPROFILE) { $env:REAL_USERPROFILE } else { $REAL_USERPROFILE }
    if ([string]::IsNullOrEmpty($realUser)) { $realUser = $env:USERPROFILE }

    $targetConfigDir = "$profileDir\.gemini\config"

    # 1. Link skills directory ~/.gemini/config/skills
    $hostSkills = "$realUser\.gemini\config\skills"
    $targetSkills = "$targetConfigDir\skills"

    if ((Test-Path $hostSkills) -and !(Test-Path $targetSkills)) {
        if (!(Test-Path $targetConfigDir)) {
            New-Item -ItemType Directory -Force -Path $targetConfigDir | Out-Null
        }
        New-Item -ItemType Junction -Path $targetSkills -Target $hostSkills -ErrorAction SilentlyContinue | Out-Null
    }

    # 2. Link plugins directory ~/.gemini/config/plugins
    $hostPlugins = "$realUser\.gemini\config\plugins"
    $targetPlugins = "$targetConfigDir\plugins"

    if ((Test-Path $hostPlugins) -and !(Test-Path $targetPlugins)) {
        if (!(Test-Path $targetConfigDir)) {
            New-Item -ItemType Directory -Force -Path $targetConfigDir | Out-Null
        }
        New-Item -ItemType Junction -Path $targetPlugins -Target $hostPlugins -ErrorAction SilentlyContinue | Out-Null
    }
}

function Link-UserConfig {
    param($profileDir)
    if (Test-Path "$profileDir\.isolated_config") { return }

    $realUser = if ($env:REAL_USERPROFILE) { $env:REAL_USERPROFILE } else { $REAL_USERPROFILE }
    if ([string]::IsNullOrEmpty($realUser)) { $realUser = $env:USERPROFILE }

    $hostConfig = "$realUser\.gemini\config\config.json"
    $targetConfigDir = "$profileDir\.gemini\config"
    $targetConfig = "$targetConfigDir\config.json"

    if ((Test-Path $hostConfig) -and !(Test-Path $targetConfig)) {
        if (!(Test-Path $targetConfigDir)) {
            New-Item -ItemType Directory -Force -Path $targetConfigDir | Out-Null
        }
        New-Item -ItemType SymbolicLink -Path $targetConfig -Target $hostConfig -ErrorAction SilentlyContinue | Out-Null
    }
}

function Link-GhConfig {
    param($profileDir)
    if ((Test-Path "$profileDir\.isolated_dotfiles") -or (Test-Path "$profileDir\.isolated_gh")) { return }

    $realUser = if ($env:REAL_USERPROFILE) { $env:REAL_USERPROFILE } else { $REAL_USERPROFILE }
    if ([string]::IsNullOrEmpty($realUser)) { $realUser = $env:USERPROFILE }

    # GitHub CLI stores auth in %APPDATA%\GitHub CLI on Windows
    $hostGhAppdata = "$realUser\AppData\Roaming\GitHub CLI"
    $targetAppdataDir = "$profileDir\AppData\Roaming"
    $targetGhAppdata = "$targetAppdataDir\GitHub CLI"

    if ((Test-Path $hostGhAppdata) -and !(Test-Path $targetGhAppdata)) {
        if (!(Test-Path $targetAppdataDir)) {
            New-Item -ItemType Directory -Force -Path $targetAppdataDir | Out-Null
        }
        New-Item -ItemType Junction -Path $targetGhAppdata -Target $hostGhAppdata -ErrorAction SilentlyContinue | Out-Null
    }

    # Also link ~/.config/gh if present (common in cross-platform/WSL setups)
    $hostGhConfig = "$realUser\.config\gh"
    $targetConfigDir = "$profileDir\.config"
    $targetGhConfig = "$targetConfigDir\gh"
    if ((Test-Path $hostGhConfig) -and !(Test-Path $targetGhConfig)) {
        if (!(Test-Path $targetConfigDir)) {
            New-Item -ItemType Directory -Force -Path $targetConfigDir | Out-Null
        }
        New-Item -ItemType Junction -Path $targetGhConfig -Target $hostGhConfig -ErrorAction SilentlyContinue | Out-Null
    }
}

function Resolve-ProfileColor {
    param([string]$colorName)
    $inputStr = $colorName.Trim().ToLower()

    switch ($inputStr) {
        "blue"                { return @{ Primary = "#1e40af"; Secondary = "#172554" } }
        "navy"                { return @{ Primary = "#1e3a8a"; Secondary = "#0f172a" } }
        "green"               { return @{ Primary = "#166534"; Secondary = "#14532d" } }
        "emerald"             { return @{ Primary = "#065f46"; Secondary = "#064e3b" } }
        "teal"                { return @{ Primary = "#115e59"; Secondary = "#134e4a" } }
        "cyan"                { return @{ Primary = "#155e75"; Secondary = "#164e63" } }
        "red"                 { return @{ Primary = "#991b1b"; Secondary = "#7f1d1d" } }
        "rose"                { return @{ Primary = "#9f1239"; Secondary = "#881337" } }
        "purple"              { return @{ Primary = "#6b21a8"; Secondary = "#581c87" } }
        "violet"              { return @{ Primary = "#5b21b6"; Secondary = "#4c1d95" } }
        "indigo"              { return @{ Primary = "#3730a3"; Secondary = "#312e81" } }
        "pink"                { return @{ Primary = "#9d174d"; Secondary = "#831843" } }
        "orange"              { return @{ Primary = "#9a3412"; Secondary = "#7c2d12" } }
        { $_ -in @("amber", "yellow") } { return @{ Primary = "#854d0e"; Secondary = "#713f12" } }
        { $_ -in @("slate", "gray", "grey") } { return @{ Primary = "#334155"; Secondary = "#1e293b" } }
        default {
            $hex = $inputStr.TrimStart("#")
            if ($hex -match "^[0-9a-fA-F]{6}$") {
                return @{ Primary = "#$hex"; Secondary = "#$hex" }
            }
            return $null
        }
    }
}

function Set-ProfileColor {
    param([string]$profileName, [string]$colorArg)

    $profileDir = "$BASE\$profileName"
    if (!(Test-Path $profileDir)) {
        Write-Error "Error: profile '$profileName' does not exist"
        exit 1
    }

    $userDir = "$profileDir\AppData\Roaming\Antigravity\User"
    New-Item -ItemType Directory -Force -Path $userDir | Out-Null
    $settingsFile = "$userDir\settings.json"

    # If settings.json is a symlink, uncouple it safely
    $item = Get-Item -Path $settingsFile -ErrorAction SilentlyContinue
    if ($item -and ($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint)) {
        $content = Get-Content -Raw -Path $settingsFile -ErrorAction SilentlyContinue
        Remove-Item -Force -Path $settingsFile
        if ($content) { Set-Content -Path $settingsFile -Value $content -Encoding UTF8 }
    }

    $settingsObj = [ordered]@{}
    if (Test-Path $settingsFile) {
        try {
            $jsonRaw = Get-Content -Raw -Path $settingsFile -Encoding UTF8
            if ($jsonRaw.Trim()) {
                $settingsObj = $jsonRaw | ConvertFrom-Json -AsHashtable
            }
        } catch {
            $settingsObj = [ordered]@{}
        }
    }

    if ($colorArg -eq "--clear") {
        if ($settingsObj.ContainsKey("workbench.colorCustomizations")) {
            $colors = $settingsObj["workbench.colorCustomizations"]
            $keysToRemove = @(
                "titleBar.activeBackground", "titleBar.activeForeground",
                "titleBar.inactiveBackground", "titleBar.inactiveForeground",
                "activityBar.background", "activityBar.foreground",
                "activityBar.inactiveForeground", "statusBar.background",
                "statusBar.foreground"
            )
            foreach ($k in $keysToRemove) {
                if ($colors.ContainsKey($k)) { $colors.Remove($k) }
            }
            if ($colors.Count -eq 0) {
                $settingsObj.Remove("workbench.colorCustomizations")
            }
        }
        $settingsObj | ConvertTo-Json -Depth 10 | Set-Content -Path $settingsFile -Encoding UTF8
        Write-Host "Cleared color customizations for profile '$profileName'."
        return
    }

    $res = Resolve-ProfileColor $colorArg
    if (!$res) {
        Write-Error "Error: invalid color '$colorArg'. Use a recognized name (blue, green, red, purple, orange, cyan, pink, emerald, indigo, slate) or hex '#RRGGBB'."
        exit 1
    }

    if (!$settingsObj.ContainsKey("workbench.colorCustomizations")) {
        $settingsObj["workbench.colorCustomizations"] = [ordered]@{}
    }
    $colors = $settingsObj["workbench.colorCustomizations"]
    $colors["titleBar.activeBackground"]   = $res.Primary
    $colors["titleBar.activeForeground"]   = "#ffffff"
    $colors["titleBar.inactiveBackground"] = $res.Secondary
    $colors["titleBar.inactiveForeground"] = "#d1d5db"
    $colors["activityBar.background"]      = $res.Secondary
    $colors["activityBar.foreground"]      = "#ffffff"
    $colors["activityBar.inactiveForeground"] = "#9ca3af"
    $colors["statusBar.background"]        = $res.Primary
    $colors["statusBar.foreground"]        = "#ffffff"

    $settingsObj | ConvertTo-Json -Depth 10 | Set-Content -Path $settingsFile -Encoding UTF8
    Write-Host "Set theme color for profile '$profileName' to $colorArg."
}

function Get-ProfileColor {
    param([string]$profileName)
    if ([string]::IsNullOrWhiteSpace($profileName)) { return "" }
    $profileDir = "$BASE\$profileName"
    if (!(Test-Path $profileDir)) { return "" }
    $settingsFile = "$profileDir\AppData\Roaming\Antigravity\User\settings.json"
    if (Test-Path $settingsFile) {
        try {
            $obj = (Get-Content -Raw -Path $settingsFile -Encoding UTF8) | ConvertFrom-Json
            $c = $obj.'workbench.colorCustomizations'.'titleBar.activeBackground'
            if ($c) { return [string]$c }
        } catch {}
    }
    return ""
}

function Invoke-ColorProfile {
    param([string]$profileName, [string]$colorArg)

    if ([string]::IsNullOrWhiteSpace($profileName)) {
        Write-Error "Error: usage: multigravity color <profile> [color|--clear]"
        exit 1
    }
    Validate-Name $profileName

    $profileDir = "$BASE\$profileName"
    if (!(Test-Path $profileDir)) {
        Write-Error "Error: profile '$profileName' does not exist"
        exit 1
    }

    if ([string]::IsNullOrWhiteSpace($colorArg)) {
        $curr = Get-ProfileColor $profileName
        if ($curr) {
            Write-Host "Profile '$profileName' color: $curr"
            return
        }
        Write-Host "Profile '$profileName' has no custom color set."
        return
    }

    Set-ProfileColor $profileName $colorArg
}

function Write-Usage {
    Write-Host "Usage: multigravity <command> [args]"
    Write-Host ""
    Write-Host "Commands:"
    Write-Host "  new <name> [options]        Create a new profile + Start Menu shortcut"
    Write-Host "      --shared                Share extensions & settings; isolate only accounts"
    Write-Host "      --from <template>        Seed from a saved template"
    Write-Host "      --isolated-dotfiles     Do not link user .gitconfig/.ssh into profile"
    Write-Host "      --isolated-mcp          Do not share system MCP server configurations"
    Write-Host "      --isolated-skills       Do not share system skills and plugins"
    Write-Host "      --isolated-config       Do not share system config.json and AI permissions"
    Write-Host "      --isolated-gh           Do not share system GitHub CLI credentials"
    Write-Host "      --color <color>         Set UI theme color (e.g. blue, green, red, '#1e3a8a')"
    Write-Host "  color <name> [color|--clear] View or change window theme color"
    Write-Host "  stop <name> [--force]       Stop a running profile gracefully"
    Write-Host "  restart <name> [args...]    Restart a profile"
    Write-Host "  clean <name|--all>          Clean profile caches to free up disk space"
    Write-Host "  list                        List existing profiles"
    Write-Host "  status                      Show running state, type, and last-used per profile"
    Write-Host "  rename <old> <new>          Rename a profile (updates shortcut if present)"
    Write-Host "  delete <name> [--force]     Delete a profile and its data"
    Write-Host "  clone <src> <dest>          Copy an existing profile"
    Write-Host "  template save <profile> <name>   Save a profile as a reusable template"
    Write-Host "  template list               List saved templates"
    Write-Host "  template delete <name>      Remove a template"
    Write-Host "  export <name> [path] [--include-cache] Archive a profile to a .zip file"
    Write-Host "  import <archive> [name]     Restore a profile from a .zip archive"
    Write-Host "  ai export <name> [path]     Export AI conversations & brains (credentials sanitized)"
    Write-Host "  ai import <archive> <name>  Import AI conversations into an existing profile"
    Write-Host "  ai sync <src> <dest>        Synchronize AI conversations directly between two profiles"
    Write-Host "  ai list <name>              List AI conversations in a profile"
    Write-Host "  ai quota [name]             Show AI token limits, usage percentage, and reset time"
    Write-Host "  ai prime [name] [options]   Auto-prime weekly token cycle on reset with random jitter"
    Write-Host "  mcp <status|share|isolate> <name> Manage MCP server configuration sharing"
    Write-Host "  skills <status|share|isolate> <name> Manage skills and plugins configuration sharing"
    Write-Host "  config <status|share|isolate> <name> Manage config.json and permission grants sharing"
    Write-Host "  gh <status|share|isolate> <name> Manage GitHub CLI credentials sharing"
    Write-Host "  quota [name]                Show AI token limits, usage percentage, and reset time"
    Write-Host "  prime [name] [options]      Auto-prime weekly token cycle on reset with random jitter"
    Write-Host "      --check                 Check if prime is needed without executing"
    Write-Host "      --force                 Prime immediately regardless of quota status"
    Write-Host "      --status                Show priming state and active scheduled task"
    Write-Host "      --no-jitter             Skip random delay (prime immediately on reset)"
    Write-Host "      --jitter <mins>         Maximum jitter delay in minutes (default: 60)"
    Write-Host "      --install-task          Register automated Windows Scheduled Task"
    Write-Host "      --uninstall-task        Remove automated Windows Scheduled Task"
    Write-Host "  update                      Update multigravity to the latest version"
    Write-Host "  doctor                      Run a system diagnosis"
    Write-Host "  stats                       Show storage usage per profile"
    Write-Host "  completion                  Show setup instructions for shell completion"
    Write-Host "  version                     Show multigravity version"
    Write-Host "  <name>                      Launch Antigravity with the given profile"
    Write-Host "  help                        Show this help"
    Write-Host ""
    Write-Host "Profile names: alphanumeric and hyphens only (e.g. work, personal, test-1)"
    Write-Host ""
    Write-Host "Environment:"
    Write-Host "  MULTIGRAVITY_APP      Override the Antigravity app path or command"
    Write-Host "  AGY_APP               Alias for MULTIGRAVITY_APP"
    Write-Host "  MULTIGRAVITY_HOME     Override the profile storage directory"
}

function Validate-Name {
    param($name)
    if ([string]::IsNullOrWhiteSpace($name)) {
        Write-Error "Error: profile name required"
        exit 1
    }
    if ($name -notmatch "^[a-zA-Z0-9][a-zA-Z0-9-]*$") {
        Write-Error "Error: profile name must start with alphanumeric and contain only letters, numbers, or hyphens"
        exit 1
    }
}

function Invoke-CreateProfile {
    param($PROFILE)
    $PROFILE_DIR = "$BASE\$PROFILE"
    
    New-Item -ItemType Directory -Force -Path "$PROFILE_DIR\.antigravity\extensions" | Out-Null
    New-Item -ItemType Directory -Force -Path "$PROFILE_DIR\AppData\Roaming" | Out-Null
    New-Item -ItemType Directory -Force -Path "$PROFILE_DIR\AppData\Local" | Out-Null

    Link-DevDotfiles $PROFILE_DIR
    Link-McpConfig $PROFILE_DIR
    Link-SkillsConfig $PROFILE_DIR
    Link-UserConfig $PROFILE_DIR
    Link-GhConfig $PROFILE_DIR
}

function Invoke-CreateSharedProfile {
    param($name)
    $profileDir = "$BASE\$name"
    $sysData     = Get-SystemDataDir
    $sysExt      = Get-SystemExtensionsDir

    New-Item -ItemType Directory -Force -Path $profileDir | Out-Null
    New-Item -ItemType File      -Force -Path "$profileDir\.shared" | Out-Null

    # Isolated AppData so accounts don't bleed across profiles
    $userDataDir = "$profileDir\AppData\Roaming\Antigravity\User"
    New-Item -ItemType Directory -Force -Path $userDataDir | Out-Null
    New-Item -ItemType Directory -Force -Path "$profileDir\AppData\Local" | Out-Null

    # Symlink settings files from the system install so they stay in sync
    if (Test-Path "$sysData\User") {
        foreach ($f in @("settings.json", "keybindings.json", "snippets")) {
            $src  = "$sysData\User\$f"
            $dest = "$userDataDir\$f"
            if ((Test-Path $src) -and !(Test-Path $dest)) {
                New-Item -ItemType SymbolicLink -Path $dest -Target $src -ErrorAction SilentlyContinue | Out-Null
            }
        }
    }

    # Point extensions at the system folder instead of an empty private copy
    $extDir = "$profileDir\.antigravity\extensions"
    if (Test-Path $sysExt) {
        if (Test-Path $extDir) { Remove-Item $extDir -Force -ErrorAction SilentlyContinue }
        New-Item -ItemType Directory -Force -Path "$profileDir\.antigravity" | Out-Null
        New-Item -ItemType SymbolicLink -Path $extDir -Target $sysExt -ErrorAction SilentlyContinue | Out-Null
    } else {
        New-Item -ItemType Directory -Force -Path $extDir | Out-Null
    }

    Link-DevDotfiles $profileDir
    Link-McpConfig $profileDir
    Link-SkillsConfig $profileDir
    Link-UserConfig $profileDir
    Link-GhConfig $profileDir
}

function Invoke-LaunchProfile {
    param($PROFILE, $ArgsToForward)
    $PROFILE_DIR = "$BASE\$PROFILE"

    if (!(Test-Path $PROFILE_DIR)) {
        Write-Error "Error: profile '$PROFILE' does not exist. Run: multigravity new $PROFILE"
        exit 1
    }

    if ([string]::IsNullOrEmpty($APP) -or !(Test-Path $APP)) {
        Write-Error "Error: Antigravity.exe (or agy.exe) not found"
        exit 1
    }

    Link-DevDotfiles $PROFILE_DIR
    Link-McpConfig $PROFILE_DIR
    Link-SkillsConfig $PROFILE_DIR
    Link-UserConfig $PROFILE_DIR
    Link-GhConfig $PROFILE_DIR

    Write-Host "Launching Antigravity profile '$PROFILE'"
    
    # Preserve real user profile in env and launch Antigravity with isolated USERPROFILE
    $realUser = if ($env:REAL_USERPROFILE) { $env:REAL_USERPROFILE } else { $REAL_USERPROFILE }
    if ([string]::IsNullOrEmpty($realUser)) { $realUser = $env:USERPROFILE }

    $userBinDirs = @(
        "$realUser\.cargo\bin",
        "$realUser\.local\bin",
        "$realUser\AppData\Local\Programs\Python",
        "$realUser\AppData\Local\Microsoft\WinGet\Links"
    )
    foreach ($ub in $userBinDirs) {
        if ((Test-Path $ub) -and ($env:Path -notlike "*$ub*")) {
            $env:Path = "$ub;$env:Path"
        }
    }

    $env:REAL_USERPROFILE = $REAL_USERPROFILE
    $env:USERPROFILE = $PROFILE_DIR
    $env:APPDATA = "$PROFILE_DIR\AppData\Roaming"
    $env:LOCALAPPDATA = "$PROFILE_DIR\AppData\Local"
    
    if ($ArgsToForward) {
        Start-Process -FilePath $APP -ArgumentList $ArgsToForward
    }
    else {
        Start-Process -FilePath $APP
    }
}

function Invoke-ListProfiles {
    Write-Host "Existing profiles:"
    if (Test-Path $BASE) {
        $profiles = Get-ChildItem -Directory -Path $BASE | Where-Object { $_.PSIsContainer -and $_.Name -ne ".templates" }
        if ($profiles.Count -gt 0) {
            foreach ($p in $profiles) {
                Write-Host $p.Name
            }
        }
        elseif ($profiles -is [System.IO.DirectoryInfo]) {
            Write-Host $profiles.Name
        }
        else {
            Write-Host "(none)"
        }
    }
    else {
        Write-Host "(none)"
    }
}

function Invoke-CreateShortcut {
    param($PROFILE)
    $APP_NAME = "Multigravity $PROFILE"
    $SHORTCUT_PATH = "$env:APPDATA\Microsoft\Windows\Start Menu\Programs\$APP_NAME.lnk"
    
    $SCRIPT_PATH = $MyInvocation.MyCommand.Path
    # If script path is empty (e.g. running from prompt), try to find it
    if ([string]::IsNullOrEmpty($SCRIPT_PATH)) {
        $cmdObj = Get-Command multigravity -ErrorAction SilentlyContinue
        if ($cmdObj) { $SCRIPT_PATH = $cmdObj.Source }
    }
    
    $WshShell = New-Object -comObject WScript.Shell
    $Shortcut = $WshShell.CreateShortcut($SHORTCUT_PATH)
    $Shortcut.TargetPath = "powershell.exe"
    $Shortcut.Arguments = "-WindowStyle Hidden -ExecutionPolicy Bypass -Command `"& '$SCRIPT_PATH' $PROFILE`""
    if ($APP) {
        $Shortcut.IconLocation = "$APP, 0"
    }
    $Shortcut.Save()

    Write-Host "Shortcut created: $SHORTCUT_PATH"
}

function Invoke-NewProfile {
    param($name, [string[]]$extraArgs)

    $shared            = $false
    $fromTpl           = ""
    $isolatedDotfiles  = $false
    $isolatedMcp       = $false
    $isolatedSkills    = $false
    $isolatedConfig    = $false
    $isolatedGh        = $false
    $color             = ""
    $i = 0
    while ($i -lt $extraArgs.Count) {
        switch ($extraArgs[$i]) {
            "--shared"            { $shared = $true }
            "--from"              { $i++; if ($i -lt $extraArgs.Count) { $fromTpl = $extraArgs[$i] } }
            "--isolated-dotfiles" { $isolatedDotfiles = $true }
            "--isolated-mcp"      { $isolatedMcp = $true }
            "--isolated-skills"   { $isolatedSkills = $true }
            "--isolated-config"   { $isolatedConfig = $true }
            "--isolated-gh"       { $isolatedGh = $true }
            "--color"             { $i++; if ($i -lt $extraArgs.Count) { $color = $extraArgs[$i] } }
        }
        $i++
    }

    if ([string]::IsNullOrWhiteSpace($name)) {
        Write-Error "Error: profile name required"
        exit 1
    }

    Validate-Name $name

    $profileDir = "$BASE\$name"
    if (Test-Path $profileDir) {
        Write-Error "Error: profile '$name' already exists"
        exit 1
    }

    New-Item -ItemType Directory -Force -Path $BASE | Out-Null
    New-Item -ItemType Directory -Force -Path $profileDir | Out-Null

    if ($isolatedDotfiles) {
        New-Item -ItemType File -Force -Path "$profileDir\.isolated_dotfiles" | Out-Null
    }
    if ($isolatedMcp) {
        New-Item -ItemType File -Force -Path "$profileDir\.isolated_mcp" | Out-Null
    }
    if ($isolatedSkills) {
        New-Item -ItemType File -Force -Path "$profileDir\.isolated_skills" | Out-Null
    }
    if ($isolatedConfig) {
        New-Item -ItemType File -Force -Path "$profileDir\.isolated_config" | Out-Null
    }
    if ($isolatedGh) {
        New-Item -ItemType File -Force -Path "$profileDir\.isolated_gh" | Out-Null
    }

    if ($fromTpl) {
        $tplPath = "$(Get-TemplatesDir)\$fromTpl"
        if (!(Test-Path $tplPath)) {
            Write-Error "Error: template '$fromTpl' not found. Run: multigravity template list"
            exit 1
        }
        Write-Host "Creating profile '$name' from template '$fromTpl'..."
        Copy-Item -Path "$tplPath\*" -Destination $profileDir -Recurse -Force
        if (!$isolatedDotfiles) { Link-DevDotfiles $profileDir }
        if (!$isolatedMcp) { Link-McpConfig $profileDir }
        if (!$isolatedSkills) { Link-SkillsConfig $profileDir }
        if (!$isolatedConfig) { Link-UserConfig $profileDir }
        if (!$isolatedGh) { Link-GhConfig $profileDir }
    } elseif ($shared) {
        Invoke-CreateSharedProfile $name
    } else {
        Invoke-CreateProfile $name
    }

    if ($color) {
        Set-ProfileColor $name $color
    }

    Write-Host "Created profile '$name'"
    Invoke-CreateShortcut $name
}

function Get-ProfileProcesses {
    param($profileName)
    $matched = @()
    $procs = Get-Process -Name "Antigravity", "agy" -ErrorAction SilentlyContinue
    if ($procs) {
        foreach ($proc in $procs) {
            try {
                $cl = (Get-CimInstance Win32_Process -Filter "ProcessId = $($proc.Id)" -ErrorAction SilentlyContinue).CommandLine
                if ($cl -and ($cl -like "*$profileName*")) {
                    $matched += $proc
                }
            } catch {}
        }
    }
    return $matched
}

function Test-ProfileRunning {
    param($profileName)
    $procs = Get-ProfileProcesses $profileName
    return ($procs -and ($procs.Count -gt 0))
}

function Stop-Profile {
    param($profileName, [switch]$Force)
    Validate-Name $profileName

    $profileDir = "$BASE\$profileName"
    if (!(Test-Path $profileDir)) {
        Write-Error "Error: profile '$profileName' does not exist"
        exit 1
    }

    $procs = Get-ProfileProcesses $profileName
    if (!$procs -or ($procs.Count -eq 0)) {
        Write-Host "Profile '$profileName' is not running."
        return
    }

    Write-Host "Stopping profile '$profileName'..."

    if ($Force) {
        $procs | Stop-Process -Force -ErrorAction SilentlyContinue
        Write-Host "Profile '$profileName' forcefully stopped."
        return
    }

    foreach ($p in $procs) {
        try { $p.CloseMainWindow() | Out-Null } catch {}
    }

    $waited = 0
    while ($waited -lt 15) {
        Start-Sleep -Milliseconds 200
        $waited++
        $stillRunning = Get-ProfileProcesses $profileName
        if (!$stillRunning -or ($stillRunning.Count -eq 0)) {
            Write-Host "Profile '$profileName' stopped gracefully."
            return
        }
    }

    Write-Host "Terminating remaining processes for profile '$profileName'..."
    $still = Get-ProfileProcesses $profileName
    if ($still) {
        $still | Stop-Process -Force -ErrorAction SilentlyContinue
    }
    Write-Host "Profile '$profileName' stopped."
}

function Restart-Profile {
    param($profileName, [string[]]$forwardArgs)
    Validate-Name $profileName
    Stop-Profile $profileName
    Start-Sleep -Milliseconds 500
    Invoke-LaunchProfile $profileName $forwardArgs
}

function Invoke-DeleteProfile {
    param($PROFILE, [switch]$Force)
    Validate-Name $PROFILE

    $PROFILE_DIR = "$BASE\$PROFILE"
    if (!(Test-Path $PROFILE_DIR)) {
        Write-Error "Error: profile '$PROFILE' does not exist"
        exit 1
    }

    if (Test-ProfileRunning $PROFILE) {
        if ($Force) {
            Write-Host "Profile '$PROFILE' is running. Stopping it first (-Force)..."
            Stop-Profile $PROFILE -Force
        } else {
            Write-Error "Error: cannot delete profile '$PROFILE' because it is currently running. Stop it first with: multigravity stop $PROFILE (or use --force)"
            exit 1
        }
    }

    $confirm = if ($Force) { "y" } else { Read-Host "Delete profile '$PROFILE' and all its data? [y/N]" }
    if ($confirm -match "^[Yy]$") {
        try {
            Remove-Item -Recurse -Force $PROFILE_DIR -ErrorAction Stop
            
            $SHORTCUT_PATH = "$env:APPDATA\Microsoft\Windows\Start Menu\Programs\Multigravity $PROFILE.lnk"
            if (Test-Path $SHORTCUT_PATH) {
                Remove-Item -Force $SHORTCUT_PATH
                Write-Host "Removed shortcut: $SHORTCUT_PATH"
            }
            Write-Host "Deleted profile '$PROFILE'"
        } catch {
            Write-Error "Error: could not delete profile directory. Ensure Antigravity is closed and no files are in use."
            Write-Host "Details: $_"
        }
    }
    else {
        Write-Host "Aborted."
    }
}

function Invoke-RenameProfile {
    param($OLD, $NEW)
    Validate-Name $OLD
    Validate-Name $NEW

    $OLD_DIR = "$BASE\$OLD"
    $NEW_DIR = "$BASE\$NEW"

    if (!(Test-Path $OLD_DIR)) {
        Write-Error "Error: profile '$OLD' does not exist"
        exit 1
    }
    if (Test-Path $NEW_DIR) {
        Write-Error "Error: profile '$NEW' already exists"
        exit 1
    }

    if (Test-ProfileRunning $OLD) {
        Write-Error "Error: cannot rename profile '$OLD' because it is currently running. Stop it first with: multigravity stop $OLD"
        exit 1
    }

    Rename-Item -Path $OLD_DIR -NewName $NEW

    $OLD_SHORTCUT = "$env:APPDATA\Microsoft\Windows\Start Menu\Programs\Multigravity $OLD.lnk"
    if (Test-Path $OLD_SHORTCUT) {
        Remove-Item -Force $OLD_SHORTCUT
        Invoke-CreateShortcut $NEW
    }

    Write-Host "Renamed profile '$OLD' to '$NEW'"
}

function Invoke-CloneProfile {
    param($SRC, $DEST)
    Validate-Name $SRC
    Validate-Name $DEST

    $SRC_DIR = "$BASE\$SRC"
    $DEST_DIR = "$BASE\$DEST"

    if (!(Test-Path $SRC_DIR)) {
        Write-Error "Error: source profile '$SRC' does not exist"
        exit 1
    }
    if (Test-Path $DEST_DIR) {
        Write-Error "Error: destination profile '$DEST' already exists"
        exit 1
    }

    Write-Host "Cloning profile '$SRC' to '$DEST'..."
    Copy-Item -Path $SRC_DIR -Destination $DEST_DIR -Recurse
    Invoke-CreateShortcut $DEST

    Write-Host "Successfully cloned '$SRC' to '$DEST'"
}

function Get-FolderSize {
    param($Path)
    $size = (Get-ChildItem $Path -Recurse -ErrorAction SilentlyContinue | Measure-Object -Property Length -Sum).Sum
    if ($size -ge 1GB) { "{0:N2} GB" -f ($size / 1GB) }
    elseif ($size -ge 1MB) { "{0:N2} MB" -f ($size / 1MB) }
    elseif ($size -ge 1KB) { "{0:N2} KB" -f ($size / 1KB) }
    else { "$size B" }
}

function Clean-SingleProfile {
    param($name, [switch]$Silent)
    $pDir = "$BASE\$name"
    if (!(Test-Path $pDir)) { return }

    $before = Get-FolderSize $pDir

    $cacheDirs = @(
        "$pDir\AppData\Local\Antigravity\Cache",
        "$pDir\AppData\Local\Antigravity\Code Cache",
        "$pDir\AppData\Local\Antigravity\GPUCache",
        "$pDir\AppData\Local\Antigravity\DawnGraphiteCache",
        "$pDir\AppData\Local\Antigravity\DawnWebGPUCache",
        "$pDir\AppData\Local\Antigravity\Crashpad",
        "$pDir\AppData\Local\Temp",
        "$pDir\AppData\Roaming\Antigravity\logs",
        "$pDir\AppData\Roaming\Antigravity\CachedData",
        "$pDir\AppData\Roaming\Antigravity\CachedExtensions",
        "$pDir\AppData\Roaming\Antigravity\Service Worker\CacheStorage",
        "$pDir\AppData\Roaming\Antigravity\Service Worker\ScriptCache",
        "$pDir\.cache",
        "$pDir\.gemini\antigravity\crashes",
        "$pDir\.npm\_cacache"
    )

    foreach ($dir in $cacheDirs) {
        if (Test-Path $dir) {
            try {
                Remove-Item -Path $dir -Recurse -Force -ErrorAction SilentlyContinue
            } catch {}
        }
    }

    New-Item -ItemType Directory -Force -Path "$pDir\AppData\Local\Temp" -ErrorAction SilentlyContinue | Out-Null
    New-Item -ItemType Directory -Force -Path "$pDir\.cache" -ErrorAction SilentlyContinue | Out-Null

    $after = Get-FolderSize $pDir
    if (!$Silent) {
        Write-Host "Cleaned cache for '$name' ($before -> $after)"
    }
}

function Invoke-CleanProfile {
    param($target)
    if ([string]::IsNullOrWhiteSpace($target)) {
        Write-Error "Error: usage: multigravity clean <profile|--all>"
        exit 1
    }

    if ($target -eq "--all") {
        if (!(Test-Path $BASE)) {
            Write-Host "No profiles found."
            return
        }
        $profiles = Get-ChildItem -Directory -Path $BASE -ErrorAction SilentlyContinue | Where-Object { $_.Name -ne ".templates" }
        $count = 0
        foreach ($p in $profiles) {
            $name = $p.Name
            if (Test-ProfileRunning $name) {
                Write-Host "Skipping '$name': profile is currently running"
                continue
            }
            Clean-SingleProfile $name
            $count++
        }
        Write-Host "Cleaned $count profile(s)."
    } else {
        Validate-Name $target
        $pDir = "$BASE\$target"
        if (!(Test-Path $pDir)) {
            Write-Error "Error: profile '$target' does not exist"
            exit 1
        }

        if (Test-ProfileRunning $target) {
            Write-Error "Error: profile '$target' is currently running — stop it first (multigravity stop $target)"
            exit 1
        }

        Clean-SingleProfile $target
    }
}

function Invoke-ProfileStats {
    if (!(Test-Path $BASE)) {
        Write-Host "No profiles found."
        return
    }

    Write-Host "Profile Storage Usage:"
    Write-Host ("{0,-20} {1,-10} {2,-10}" -f "PROFILE", "SIZE", "EXTENSIONS")
    Write-Host ("{0,-20} {1,-10} {2,-10}" -f "-------", "----", "----------")

    $profiles = Get-ChildItem -Directory -Path $BASE | Where-Object { $_.Name -ne ".templates" }
    foreach ($p in $profiles) {
        $size = Get-FolderSize $p.FullName
        $extPath = Join-Path $p.FullName ".antigravity\extensions"
        $extCount = if (Test-Path $extPath) { (Get-ChildItem $extPath).Count } else { 0 }
        Write-Host ("{0,-20} {1,-10} {2,-10}" -f $p.Name, $size, $extCount)
    }

    Write-Host ""
    $total = Get-FolderSize $BASE
    Write-Host "Total usage: $total"
}

function Invoke-DoctorCli {
    $errors = 0
    $warnings = 0

    Write-Host "Checking multigravity environment..."

    # 1. Antigravity / Agy Installation
    if ($APP -and (Test-Path $APP)) {
        Write-Host "  [OK] Antigravity/Agy: Found at $APP"
    } else {
        Write-Host "  [FAIL] Antigravity/Agy: Not found. Ensure it is installed or set MULTIGRAVITY_APP or AGY_APP."
        $errors++
    }

    # 2. Path Check
    $cmdObj = Get-Command multigravity -ErrorAction SilentlyContinue
    if ($cmdObj) {
        Write-Host "  [OK] Global Binary: $($cmdObj.Source)"
    } else {
        Write-Host "  [WARN] Global Binary: Not found in PATH. Run install script or update PATH."
        $warnings++
    }

    # 3. Base Directory
    if (Test-Path $BASE) {
        # Check writability
        try {
            $testFile = Join-Path $BASE ".write-test"
            New-Item -ItemType File -Path $testFile -Force -ErrorAction Stop | Out-Null
            Remove-Item $testFile -Force
            Write-Host "  [OK] Profile storage: $BASE (writable)"
        } catch {
            Write-Host "  [FAIL] Profile storage: $BASE (NOT writable)"
            $errors++
        }
    } else {
        Write-Host "  [WARN] Profile storage: $BASE (Not yet created)"
    }

    Write-Host ""
    if ($errors -eq 0) {
        if ($warnings -eq 0) {
            Write-Host "Your environment looks perfect!"
        } else {
            Write-Host "Found $warnings warning(s). Multigravity should still work, but some features might be degraded."
        }
    } else {
        Write-Host "Found $errors error(s) and $warnings warning(s). Please fix the errors above."
    }
}

function Invoke-UpdateCli {
    $repo = if ($env:MULTIGRAVITY_REPO) { $env:MULTIGRAVITY_REPO } else { "yegear1/multigravity-cli" }
    $branch = if ($env:MULTIGRAVITY_BRANCH) { $env:MULTIGRAVITY_BRANCH } else { "main" }
    $script_url = "https://raw.githubusercontent.com/$repo/$branch/multigravity.ps1"
    $target = $MyInvocation.MyCommand.Path
    if ([string]::IsNullOrEmpty($target)) {
        $cmdObj = Get-Command multigravity -ErrorAction SilentlyContinue
        if ($cmdObj) { $target = $cmdObj.Source }
    }

    if ([string]::IsNullOrEmpty($target)) {
        Write-Error "Error: could not determine script path for update"
        exit 1
    }

    Write-Host "Updating multigravity from $script_url ..."
    try {
        $result = Invoke-WebRequest -Uri $script_url -UseBasicParsing -ErrorAction Stop
        [System.IO.File]::WriteAllText($target, $result.Content, [System.Text.Encoding]::UTF8)
        Write-Host "Successfully updated multigravity!"
    } catch {
        Write-Error "Error: failed to download update: $_"
        exit 1
    }
}

function Invoke-VersionCli {
    Write-Host "multigravity v$VERSION (fork: yegear1/multigravity-cli)"
}

function Invoke-HelpCompletion {
    Write-Host "To enable autocompletion in PowerShell, add the following to your `$PROFILE:"
    Write-Host ""
    Write-Host '  Invoke-Expression (& multigravity completion powershell)'
    Write-Host ""
    Write-Host "Then restart your terminal or run: . `$PROFILE"
}

function Invoke-GenerateCompletion {
    param($shell)
    if ($shell -eq "powershell") {
        @"
Register-ArgumentCompleter -Native -CommandName multigravity -ScriptBlock {
    param(`$wordToComplete, `$commandAst, `$cursorPosition)
    `$opts = @('new', 'color', 'stop', 'restart', 'clean', 'list', 'status', 'rename', 'delete', 'clone', 'template', 'export', 'import', 'ai', 'mcp', 'skills', 'config', 'gh', 'quota', 'prime', 'update', 'doctor', 'stats', 'completion', 'version', 'help')
    `$profiles = if (Test-Path '$BASE') { Get-ChildItem -Directory -Path '$BASE' | Select-Object -ExpandProperty Name } else { @() }
    (`$opts + `$profiles) | Where-Object { `$_ -like "`$wordToComplete*" } | ForEach-Object {
        [System.Management.Automation.CompletionResult]::new(`$_, `$_, 'ParameterValue', `$_)
    }
}
"@
    } else {
        Write-Host "Only 'powershell' completion is supported on Windows."
    }
}

function Invoke-TemplateCmd {
    param($sub, $a, $b)
    switch ($sub) {
        "save" {
            if ([string]::IsNullOrWhiteSpace($a) -or [string]::IsNullOrWhiteSpace($b)) {
                Write-Error "Error: usage: multigravity template save <profile> <name>"; exit 1
            }
            Validate-Name $a; Validate-Name $b
            $srcDir  = "$BASE\$a"
            $tplDir  = Get-TemplatesDir
            $tplPath = "$tplDir\$b"
            if (!(Test-Path $srcDir))  { Write-Error "Error: profile '$a' does not exist"; exit 1 }
            if (Test-Path $tplPath)    { Write-Error "Error: template '$b' already exists"; exit 1 }
            New-Item -ItemType Directory -Force -Path $tplDir | Out-Null
            Write-Host "Saving '$a' as template '$b'..."
            Copy-Item -Path $srcDir -Destination $tplPath -Recurse
            $marker = "$tplPath\.shared"
            if (Test-Path $marker) { Remove-Item $marker -Force }
            Write-Host "Saved template '$b'"
        }
        "list" {
            $tplDir = Get-TemplatesDir
            Write-Host "Templates:"
            if (!(Test-Path $tplDir)) { Write-Host "  (none)"; return }
            $items = Get-ChildItem -Directory -Path $tplDir -ErrorAction SilentlyContinue
            if ($items.Count -eq 0) { Write-Host "  (none)"; return }
            foreach ($t in $items) {
                Write-Host ("  {0,-20} {1}" -f $t.Name, (Get-FolderSize $t.FullName))
            }
        }
        "delete" {
            if ([string]::IsNullOrWhiteSpace($a)) { Write-Error "Error: template name required"; exit 1 }
            Validate-Name $a
            $tplPath = "$(Get-TemplatesDir)\$a"
            if (!(Test-Path $tplPath)) { Write-Error "Error: template '$a' does not exist"; exit 1 }
            Remove-Item -Recurse -Force $tplPath
            Write-Host "Deleted template '$a'"
        }
        default {
            Write-Error "Error: usage: multigravity template <save|list|delete>"; exit 1
        }
    }
}

function Invoke-StatusProfiles {
    if (!(Test-Path $BASE)) { Write-Host "No profiles found."; return }

    Write-Host ("{0,-18} {1,-10} {2,-12} {3,-20} {4}" -f "PROFILE", "RUNNING", "TYPE", "LAST USED", "SIZE")
    Write-Host ("{0,-18} {1,-10} {2,-12} {3,-20} {4}" -f "-------", "-------", "----", "---------", "----")

    $dirs = Get-ChildItem -Directory -Path $BASE -ErrorAction SilentlyContinue |
            Where-Object { $_.Name -ne ".templates" }

    foreach ($d in $dirs) {
        $running = if (Test-ProfileRunning $d.Name) { "yes" } else { "no" }

        $ptype    = if (Test-Path "$($d.FullName)\.shared") { "shared" } else { "full" }
        $lastUsed = $d.LastWriteTime.ToString("yyyy-MM-dd HH:mm")
        $size     = Get-FolderSize $d.FullName

        if ($running -eq "yes") {
            Write-Host ("{0,-18} " -f $d.Name) -NoNewline
            Write-Host ("{0,-10} " -f $running) -NoNewline -ForegroundColor Green
            Write-Host ("{0,-12} {1,-20} {2}" -f $ptype, $lastUsed, $size)
        } else {
            Write-Host ("{0,-18} {1,-10} {2,-12} {3,-20} {4}" -f $d.Name, $running, $ptype, $lastUsed, $size)
        }
    }
}

function Invoke-ExportProfile {
    param($name, $outPath, [switch]$IncludeCache)
    if ([string]::IsNullOrWhiteSpace($name)) { Write-Error "Error: profile name required"; exit 1 }
    Validate-Name $name

    $profileDir = "$BASE\$name"
    if (!(Test-Path $profileDir)) { Write-Error "Error: profile '$name' does not exist"; exit 1 }

    if ([string]::IsNullOrWhiteSpace($outPath)) { $outPath = ".\$name.zip" }

    Write-Host "Exporting '$name' to $outPath ..."

    if ($IncludeCache) {
        Compress-Archive -Path $profileDir -DestinationPath $outPath -Force
    } else {
        $tempStaging = Join-Path $env:TEMP "_mg_export_$(Get-Random)"
        try {
            $stageDir = Join-Path $tempStaging $name
            New-Item -ItemType Directory -Force -Path $stageDir | Out-Null
            Copy-Item -Path "$profileDir\*" -Destination $stageDir -Recurse -Force
            
            $cacheDirs = @(
                "$stageDir\AppData\Local\Antigravity\Cache",
                "$stageDir\AppData\Local\Antigravity\Code Cache",
                "$stageDir\AppData\Local\Antigravity\GPUCache",
                "$stageDir\AppData\Local\Antigravity\DawnGraphiteCache",
                "$stageDir\AppData\Local\Antigravity\DawnWebGPUCache",
                "$stageDir\AppData\Local\Antigravity\Crashpad",
                "$stageDir\AppData\Local\Temp",
                "$stageDir\AppData\Roaming\Antigravity\logs",
                "$stageDir\AppData\Roaming\Antigravity\CachedData",
                "$stageDir\AppData\Roaming\Antigravity\CachedExtensions",
                "$stageDir\AppData\Roaming\Antigravity\Service Worker\CacheStorage",
                "$stageDir\AppData\Roaming\Antigravity\Service Worker\ScriptCache",
                "$stageDir\.cache",
                "$stageDir\.gemini\antigravity\crashes",
                "$stageDir\.npm\_cacache"
            )
            foreach ($cd in $cacheDirs) {
                if (Test-Path $cd) {
                    Remove-Item -Path $cd -Recurse -Force -ErrorAction SilentlyContinue
                }
            }

            Compress-Archive -Path $stageDir -DestinationPath $outPath -Force
        } finally {
            if (Test-Path $tempStaging) {
                Remove-Item -Path $tempStaging -Recurse -Force -ErrorAction SilentlyContinue
            }
        }
    }
    Write-Host "Done."
}

function Invoke-ImportProfile {
    param($archivePath, $name)

    if ([string]::IsNullOrWhiteSpace($archivePath)) {
        Write-Error "Error: usage: multigravity import <archive.zip> [name]"; exit 1
    }
    if (!(Test-Path $archivePath)) {
        Write-Error "Error: file not found: $archivePath"; exit 1
    }

    if ([string]::IsNullOrWhiteSpace($name)) {
        $name = [System.IO.Path]::GetFileNameWithoutExtension($archivePath)
    }
    Validate-Name $name

    $dest = "$BASE\$name"
    if (Test-Path $dest) {
        Write-Error "Error: profile '$name' already exists — choose a different name or delete it first"
        exit 1
    }

    New-Item -ItemType Directory -Force -Path $BASE | Out-Null
    Write-Host "Importing as '$name'..."

    $tmp = "$BASE\_mg_import_$(Get-Random)"
    Expand-Archive -Path $archivePath -DestinationPath $tmp -Force

    $top = Get-ChildItem -Directory -Path $tmp
    if ($top.Count -eq 1) {
        Move-Item -Path $top[0].FullName -Destination $dest
        Remove-Item $tmp -Recurse -Force
    } else {
        Rename-Item -Path $tmp -NewName $name
    }

    Invoke-CreateShortcut $name
    Write-Host "Imported profile '$name'"
}

function Invoke-AiListProfile {
    param($name)
    if ([string]::IsNullOrWhiteSpace($name)) { Write-Error "Error: profile name required"; exit 1 }
    Validate-Name $name

    $pDir = "$BASE\$name"
    if (!(Test-Path $pDir)) { Write-Error "Error: profile '$name' does not exist"; exit 1 }

    $geminiDir = "$pDir\.gemini\antigravity"
    if (!(Test-Path "$geminiDir\conversations") -and !(Test-Path "$geminiDir\brain")) {
        Write-Host "Profile '$name' has no saved AI chats."
        return
    }

    Write-Host "AI Conversations in profile '$name':"
    Write-Host ("{0,-38} {1,-32} {2}" -f "CONVERSATION ID", "TITLE", "ARTIFACTS")
    Write-Host ("{0,-38} {1,-32} {2}" -f "---------------", "-----", "---------")

    $dbFiles = Get-ChildItem -Path "$geminiDir\conversations" -Filter "*.db" -ErrorAction SilentlyContinue
    if (!$dbFiles -or $dbFiles.Count -eq 0) {
        Write-Host "No conversations found."
        return
    }

    foreach ($db in $dbFiles) {
        $uuid = $db.BaseName
        $title = "(untitled conversation)"
        $annot = "$geminiDir\annotations\$uuid.pbtxt"
        if (Test-Path $annot) {
            $content = Get-Content $annot -Raw -ErrorAction SilentlyContinue
            if ($content -match 'title:\s*"([^"]+)"') {
                $title = $matches[1]
            }
        }
        if ($title.Length -gt 30) {
            $title = $title.Substring(0, 27) + "..."
        }

        $artCount = "none"
        $brainDir = "$geminiDir\brain\$uuid"
        if (Test-Path $brainDir) {
            $mds = Get-ChildItem -Path $brainDir -Filter "*.md" -ErrorAction SilentlyContinue
            if ($mds -and $mds.Count -gt 0) {
                $artCount = "{0} file(s)" -f $mds.Count
            }
        }

        Write-Host ("{0,-38} {1,-32} {2}" -f $uuid, $title, $artCount)
    }

    Write-Host ""
    Write-Host ("Total conversations: {0}" -f $dbFiles.Count)
}

function Invoke-AiExportProfile {
    param($name, $outPath)
    if ([string]::IsNullOrWhiteSpace($name)) { Write-Error "Error: profile name required"; exit 1 }
    Validate-Name $name

    $pDir = "$BASE\$name"
    if (!(Test-Path $pDir)) { Write-Error "Error: profile '$name' does not exist"; exit 1 }

    if (Test-ProfileRunning $name) {
        Write-Error "Error: profile '$name' is currently running — stop it first to ensure SQLite database flush (multigravity stop $name)"
        exit 1
    }

    $geminiDir = "$pDir\.gemini\antigravity"
    if (!(Test-Path $geminiDir)) {
        Write-Error "Error: profile '$name' has no AI data (.gemini\antigravity does not exist)"
        exit 1
    }

    if ([string]::IsNullOrWhiteSpace($outPath)) { $outPath = ".\$name-ai-chats.zip" }

    Write-Host "Exporting AI chats from '$name' to $outPath ..."

    $tempStaging = Join-Path $env:TEMP "_mg_aiexport_$(Get-Random)"
    try {
        New-Item -ItemType Directory -Force -Path $tempStaging | Out-Null
        $items = @("conversations", "brain", "annotations", "knowledge", "antigravity_state.pbtxt", "agyhub_summaries_proto.pb")
        $copiedCount = 0
        foreach ($item in $items) {
            $src = Join-Path $geminiDir $item
            if (Test-Path $src) {
                Copy-Item -Path $src -Destination $tempStaging -Recurse -Force
                $copiedCount++
            }
        }

        if ($copiedCount -eq 0) {
            Write-Error "Error: no AI conversation or brain data found in profile '$name'"
            exit 1
        }

        $sensitive = Get-ChildItem -Path $tempStaging -Recurse -File | Where-Object {
            $_.Name -like "*token*" -or $_.Name -like "*oauth*" -or $_.Name -like "*auth*" -or $_.Name -like "*credential*" -or $_.Name -eq "installation_id"
        }
        foreach ($s in $sensitive) {
            Remove-Item -Path $s.FullName -Force -ErrorAction SilentlyContinue
        }

        Compress-Archive -Path "$tempStaging\*" -DestinationPath $outPath -Force
        
        $convs = Get-ChildItem -Path "$geminiDir\conversations" -Filter "*.db" -ErrorAction SilentlyContinue
        $convCount = if ($convs) { $convs.Count } else { 0 }
        Write-Host "Successfully exported $convCount conversation(s) to $outPath (credentials sanitized)."
    } finally {
        if (Test-Path $tempStaging) {
            Remove-Item -Path $tempStaging -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}

function Invoke-AiImportProfile {
    param($archivePath, $name)
    if ([string]::IsNullOrWhiteSpace($archivePath) -or [string]::IsNullOrWhiteSpace($name)) {
        Write-Error "Error: usage: multigravity ai import <archive.zip> <profile>"; exit 1
    }
    if (!(Test-Path $archivePath)) {
        Write-Error "Error: file not found: $archivePath"; exit 1
    }
    Validate-Name $name

    $pDir = "$BASE\$name"
    if (!(Test-Path $pDir)) {
        Write-Error "Error: profile '$name' does not exist"; exit 1
    }

    if (Test-ProfileRunning $name) {
        Write-Error "Error: profile '$name' is currently running — stop it first (multigravity stop $name)"
        exit 1
    }

    $targetGemini = "$pDir\.gemini\antigravity"
    New-Item -ItemType Directory -Force -Path $targetGemini | Out-Null

    Write-Host "Importing AI chats into '$name'..."

    $tempStaging = Join-Path $env:TEMP "_mg_aiimport_$(Get-Random)"
    try {
        New-Item -ItemType Directory -Force -Path $tempStaging | Out-Null
        Expand-Archive -Path $archivePath -DestinationPath $tempStaging -Force

        $sensitive = Get-ChildItem -Path $tempStaging -Recurse -File | Where-Object {
            $_.Name -like "*token*" -or $_.Name -like "*oauth*" -or $_.Name -like "*auth*" -or $_.Name -like "*credential*" -or $_.Name -eq "installation_id"
        }
        foreach ($s in $sensitive) {
            Remove-Item -Path $s.FullName -Force -ErrorAction SilentlyContinue
        }

        if (Test-Path "$targetGemini\agyhub_summaries_proto.pb") {
            Remove-Item -Path "$tempStaging\agyhub_summaries_proto.pb" -Force -ErrorAction SilentlyContinue
        }
        if (Test-Path "$targetGemini\antigravity_state.pbtxt") {
            Remove-Item -Path "$tempStaging\antigravity_state.pbtxt" -Force -ErrorAction SilentlyContinue
        }

        Copy-Item -Path "$tempStaging\*" -Destination $targetGemini -Recurse -Force

        $convs = Get-ChildItem -Path "$targetGemini\conversations" -Filter "*.db" -ErrorAction SilentlyContinue
        $convCount = if ($convs) { $convs.Count } else { 0 }
        Write-Host "Successfully imported AI chats into profile '$name' ($convCount conversation(s) now available)."
    } finally {
        if (Test-Path $tempStaging) {
            Remove-Item -Path $tempStaging -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}

function Invoke-AiSyncProfile {
    param($src, $dest)
    if ([string]::IsNullOrWhiteSpace($src) -or [string]::IsNullOrWhiteSpace($dest)) {
        Write-Error "Error: usage: multigravity ai sync <source_profile> <target_profile>"; exit 1
    }
    Validate-Name $src
    Validate-Name $dest

    if ($src -eq $dest) {
        Write-Error "Error: source and target profiles cannot be the same"; exit 1
    }

    $srcDir = "$BASE\$src"
    $destDir = "$BASE\$dest"

    if (!(Test-Path $srcDir)) { Write-Error "Error: source profile '$src' does not exist"; exit 1 }
    if (!(Test-Path $destDir)) { Write-Error "Error: target profile '$dest' does not exist"; exit 1 }

    if (Test-ProfileRunning $src) {
        Write-Error "Error: source profile '$src' is currently running — stop it first (multigravity stop $src)"
        exit 1
    }
    if (Test-ProfileRunning $dest) {
        Write-Error "Error: target profile '$dest' is currently running — stop it first (multigravity stop $dest)"
        exit 1
    }

    $srcGemini = "$srcDir\.gemini\antigravity"
    $destGemini = "$destDir\.gemini\antigravity"

    if (!(Test-Path $srcGemini)) {
        Write-Error "Error: source profile '$src' has no AI data (.gemini\antigravity does not exist)"
        exit 1
    }

    foreach ($sub in @("conversations", "annotations", "brain", "knowledge")) {
        $subPath = Join-Path $destGemini $sub
        if (!(Test-Path $subPath)) { New-Item -ItemType Directory -Force -Path $subPath | Out-Null }
    }

    Write-Host "Syncing AI conversations from '$src' to '$dest'..."
    $synced = 0

    $srcConvs = Join-Path $srcGemini "conversations"
    if (Test-Path $srcConvs) {
        $dbFiles = Get-ChildItem -Path $srcConvs -Filter "*.db" -File -ErrorAction SilentlyContinue
        foreach ($db in $dbFiles) {
            $destDb = Join-Path "$destGemini\conversations" $db.Name
            $uuid = [System.IO.Path]::GetFileNameWithoutExtension($db.Name)

            if (!(Test-Path $destDb) -or ($db.LastWriteTime -gt (Get-Item $destDb).LastWriteTime)) {
                Copy-Item -Path $db.FullName -Destination $destDb -Force

                $srcAnnot = Join-Path "$srcGemini\annotations" "$uuid.pbtxt"
                if (Test-Path $srcAnnot) {
                    Copy-Item -Path $srcAnnot -Destination "$destGemini\annotations\$uuid.pbtxt" -Force
                }

                $srcBrain = Join-Path "$srcGemini\brain" $uuid
                if (Test-Path $srcBrain) {
                    $destBrain = Join-Path "$destGemini\brain" $uuid
                    if (!(Test-Path $destBrain)) { New-Item -ItemType Directory -Force -Path $destBrain | Out-Null }
                    Copy-Item -Path "$srcBrain\*" -Destination $destBrain -Recurse -Force -ErrorAction SilentlyContinue
                }

                $synced++
            }
        }
    }

    $srcKnowledge = Join-Path $srcGemini "knowledge"
    if (Test-Path $srcKnowledge) {
        $kFiles = Get-ChildItem -Path $srcKnowledge -ErrorAction SilentlyContinue
        foreach ($kf in $kFiles) {
            if ($kf.Name -like "*token*" -or $kf.Name -like "*oauth*" -or $kf.Name -like "*auth*" -or $kf.Name -like "*credential*") { continue }
            $destKf = Join-Path "$destGemini\knowledge" $kf.Name
            if (!(Test-Path $destKf) -or ($kf.LastWriteTime -gt (Get-Item $destKf).LastWriteTime)) {
                Copy-Item -Path $kf.FullName -Destination "$destGemini\knowledge" -Recurse -Force -ErrorAction SilentlyContinue
            }
        }
    }

    if (!(Test-Path "$destGemini\agyhub_summaries_proto.pb") -and (Test-Path "$srcGemini\agyhub_summaries_proto.pb")) {
        Copy-Item -Path "$srcGemini\agyhub_summaries_proto.pb" -Destination "$destGemini\agyhub_summaries_proto.pb" -Force
    }

    $totalConvs = (Get-ChildItem -Path "$destGemini\conversations" -Filter "*.db" -ErrorAction SilentlyContinue).Count
    Write-Host "Successfully synced $synced conversation(s) into '$dest' (total: $totalConvs available)."
}

function Invoke-AiCmd {
    param($sub, $arg1, $arg2)
    switch ($sub) {
        "export" { Invoke-AiExportProfile $arg1 $arg2 }
        "import" { Invoke-AiImportProfile $arg1 $arg2 }
        "sync"   { Invoke-AiSyncProfile $arg1 $arg2 }
        "list"   { Invoke-AiListProfile $arg1 }
        "quota"  { Invoke-QuotaCmd $arg1 }
        "prime"  { Invoke-PrimeCmd $arg1 $arg2 }
        default  {
            Write-Error "Error: usage: multigravity ai <export|import|sync|list|quota|prime> [args...]"
            exit 1
        }
    }
}

function Invoke-McpCmd {
    param($action, $profile)
    if ([string]::IsNullOrWhiteSpace($action) -or [string]::IsNullOrWhiteSpace($profile)) {
        Write-Error "Error: usage: multigravity mcp <status|share|isolate> <profile>"
        exit 1
    }
    Validate-Name $profile

    $profilePath = "$BASE\$profile"
    if (!(Test-Path $profilePath)) {
        Write-Error "Error: profile '$profile' does not exist"
        exit 1
    }

    $realUser = if ($env:REAL_USERPROFILE) { $env:REAL_USERPROFILE } else { $REAL_USERPROFILE }
    if ([string]::IsNullOrEmpty($realUser)) { $realUser = $env:USERPROFILE }

    $hostMcpConfig = "$realUser\.gemini\config\mcp_config.json"
    $targetMcpConfig = "$profilePath\.gemini\config\mcp_config.json"
    $hostMcpSchemas = "$realUser\.gemini\antigravity\mcp"
    $targetMcpSchemas = "$profilePath\.gemini\antigravity\mcp"

    switch ($action) {
        "status" {
            if (Test-Path "$profilePath\.isolated_mcp") {
                Write-Host "Profile '$profile' has isolated MCP servers (--isolated-mcp active)."
            } elseif (Test-Path $targetMcpConfig) {
                $item = Get-Item -Path $targetMcpConfig -ErrorAction SilentlyContinue
                if ($item -and ($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint)) {
                    $target = $item.Target
                    Write-Host "Profile '$profile' shares host MCP servers -> $target"
                } else {
                    Write-Host "Profile '$profile' has a standalone local mcp_config.json."
                }
            } else {
                Write-Host "Profile '$profile' has no MCP servers configured."
            }
        }
        "share" {
            if (Test-Path "$profilePath\.isolated_mcp") {
                Remove-Item -Force -Path "$profilePath\.isolated_mcp" -ErrorAction SilentlyContinue
            }
            $item = Get-Item -Path $targetMcpConfig -ErrorAction SilentlyContinue
            if ($item -and ($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint)) {
                Write-Host "Profile '$profile' is already sharing host MCP servers."
            } else {
                if (Test-Path $targetMcpConfig) {
                    Move-Item -Path $targetMcpConfig -Destination "$targetMcpConfig.bak" -Force
                    Write-Host "Backed up existing mcp_config.json to mcp_config.json.bak"
                }
                Link-McpConfig $profilePath
                Write-Host "Profile '$profile' is now sharing host MCP servers."
            }
        }
        "isolate" {
            New-Item -ItemType File -Force -Path "$profilePath\.isolated_mcp" | Out-Null
            $item = Get-Item -Path $targetMcpConfig -ErrorAction SilentlyContinue
            if ($item -and ($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint)) {
                Remove-Item -Force -Path $targetMcpConfig -ErrorAction SilentlyContinue
                if (Test-Path $hostMcpConfig) {
                    Copy-Item -Path $hostMcpConfig -Destination $targetMcpConfig -Force
                    Write-Host "Copied host MCP config to standalone file for '$profile'."
                }
            }
            $schemaItem = Get-Item -Path $targetMcpSchemas -ErrorAction SilentlyContinue
            if ($schemaItem -and ($schemaItem.Attributes -band [System.IO.FileAttributes]::ReparsePoint)) {
                Remove-Item -Force -Path $targetMcpSchemas -ErrorAction SilentlyContinue
                if (Test-Path $hostMcpSchemas) {
                    Copy-Item -Path $hostMcpSchemas -Destination $targetMcpSchemas -Recurse -Force
                }
            }
            Write-Host "Profile '$profile' is now isolated from host MCP updates."
        }
        default {
            Write-Error "Error: usage: multigravity mcp <status|share|isolate> <profile>"
            exit 1
        }
    }
}

function Invoke-SkillsCmd {
    param($action, $profile)
    if ([string]::IsNullOrWhiteSpace($action) -or [string]::IsNullOrWhiteSpace($profile)) {
        Write-Error "Error: usage: multigravity skills <status|share|isolate> <profile>"
        exit 1
    }
    Validate-Name $profile

    $profilePath = "$BASE\$profile"
    if (!(Test-Path $profilePath)) {
        Write-Error "Error: profile '$profile' does not exist"
        exit 1
    }

    $realUser = if ($env:REAL_USERPROFILE) { $env:REAL_USERPROFILE } else { $REAL_USERPROFILE }
    if ([string]::IsNullOrEmpty($realUser)) { $realUser = $env:USERPROFILE }

    $hostSkills = "$realUser\.gemini\config\skills"
    $hostPlugins = "$realUser\.gemini\config\plugins"
    $targetSkills = "$profilePath\.gemini\config\skills"
    $targetPlugins = "$profilePath\.gemini\config\plugins"

    switch ($action) {
        "status" {
            if (Test-Path "$profilePath\.isolated_skills") {
                Write-Host "Profile '$profile' has isolated skills/plugins (--isolated-skills active)."
            } elseif ((Test-Path $targetSkills) -or (Test-Path $targetPlugins)) {
                $sItem = Get-Item -Path $targetSkills -ErrorAction SilentlyContinue
                $pItem = Get-Item -Path $targetPlugins -ErrorAction SilentlyContinue
                $isShared = ($sItem -and ($sItem.Attributes -band [System.IO.FileAttributes]::ReparsePoint)) -or ($pItem -and ($pItem.Attributes -band [System.IO.FileAttributes]::ReparsePoint))
                if ($isShared) {
                    $sTarget = if ($sItem -and ($sItem.Attributes -band [System.IO.FileAttributes]::ReparsePoint)) { $sItem.Target } else { "(none)" }
                    $pTarget = if ($pItem -and ($pItem.Attributes -band [System.IO.FileAttributes]::ReparsePoint)) { $pItem.Target } else { "(none)" }
                    Write-Host "Profile '$profile' shares host skills -> $sTarget (plugins -> $pTarget)"
                } else {
                    Write-Host "Profile '$profile' has standalone local skills/plugins."
                }
            } else {
                Write-Host "Profile '$profile' has no custom skills or plugins configured."
            }
        }
        "share" {
            if (Test-Path "$profilePath\.isolated_skills") {
                Remove-Item -Force -Path "$profilePath\.isolated_skills" -ErrorAction SilentlyContinue
            }
            $sItem = Get-Item -Path $targetSkills -ErrorAction SilentlyContinue
            $pItem = Get-Item -Path $targetPlugins -ErrorAction SilentlyContinue
            $alreadyShared = ($sItem -and ($sItem.Attributes -band [System.IO.FileAttributes]::ReparsePoint)) -and ($pItem -and ($pItem.Attributes -band [System.IO.FileAttributes]::ReparsePoint))

            if ($alreadyShared) {
                Write-Host "Profile '$profile' is already sharing host skills and plugins."
            } else {
                if ((Test-Path $targetSkills) -and !($sItem -and ($sItem.Attributes -band [System.IO.FileAttributes]::ReparsePoint))) {
                    Move-Item -Path $targetSkills -Destination "$targetSkills.bak" -Force
                    Write-Host "Backed up existing skills directory to skills.bak"
                }
                if ((Test-Path $targetPlugins) -and !($pItem -and ($pItem.Attributes -band [System.IO.FileAttributes]::ReparsePoint))) {
                    Move-Item -Path $targetPlugins -Destination "$targetPlugins.bak" -Force
                    Write-Host "Backed up existing plugins directory to plugins.bak"
                }
                Link-SkillsConfig $profilePath
                Write-Host "Profile '$profile' is now sharing host skills and plugins."
            }
        }
        "isolate" {
            New-Item -ItemType File -Force -Path "$profilePath\.isolated_skills" | Out-Null
            $sItem = Get-Item -Path $targetSkills -ErrorAction SilentlyContinue
            if ($sItem -and ($sItem.Attributes -band [System.IO.FileAttributes]::ReparsePoint)) {
                Remove-Item -Force -Path $targetSkills -ErrorAction SilentlyContinue
                if (Test-Path $hostSkills) {
                    Copy-Item -Path $hostSkills -Destination $targetSkills -Recurse -Force
                    Write-Host "Copied host skills to standalone directory for '$profile'."
                }
            }
            $pItem = Get-Item -Path $targetPlugins -ErrorAction SilentlyContinue
            if ($pItem -and ($pItem.Attributes -band [System.IO.FileAttributes]::ReparsePoint)) {
                Remove-Item -Force -Path $targetPlugins -ErrorAction SilentlyContinue
                if (Test-Path $hostPlugins) {
                    Copy-Item -Path $hostPlugins -Destination $targetPlugins -Recurse -Force
                    Write-Host "Copied host plugins to standalone directory for '$profile'."
                }
            }
            Write-Host "Profile '$profile' is now isolated from host skills and plugins updates."
        }
        default {
            Write-Error "Error: usage: multigravity skills <status|share|isolate> <profile>"
            exit 1
        }
    }
}

function Invoke-ConfigCmd {
    param($action, $profile)

    if ([string]::IsNullOrEmpty($action) -or [string]::IsNullOrEmpty($profile)) {
        Write-Error "Error: usage: multigravity config <status|share|isolate> <profile>"
        exit 1
    }

    Test-ValidName $profile

    $profilePath = "$BASE\$profile"
    if (!(Test-Path $profilePath)) {
        Write-Error "Error: profile '$profile' does not exist"
        exit 1
    }

    $realUser = if ($env:REAL_USERPROFILE) { $env:REAL_USERPROFILE } else { $REAL_USERPROFILE }
    if ([string]::IsNullOrEmpty($realUser)) { $realUser = $env:USERPROFILE }

    $hostConfig = "$realUser\.gemini\config\config.json"
    $targetConfig = "$profilePath\.gemini\config\config.json"

    switch ($action) {
        "status" {
            if (Test-Path "$profilePath\.isolated_config") {
                Write-Host "Profile '$profile' has isolated configuration (--isolated-config active)."
            } elseif (Test-Path $targetConfig) {
                $cItem = Get-Item -Path $targetConfig -ErrorAction SilentlyContinue
                $isShared = $cItem -and ($cItem.Attributes -band [System.IO.FileAttributes]::ReparsePoint)
                if ($isShared) {
                    $cTarget = if ($cItem.Target) { $cItem.Target } else { "(symlink)" }
                    Write-Host "Profile '$profile' shares host config.json -> $cTarget"
                } else {
                    Write-Host "Profile '$profile' has standalone local config.json."
                }
            } else {
                Write-Host "Profile '$profile' has no config.json configured."
            }
        }
        "share" {
            if (Test-Path "$profilePath\.isolated_config") {
                Remove-Item -Force -Path "$profilePath\.isolated_config" -ErrorAction SilentlyContinue
            }
            $cItem = Get-Item -Path $targetConfig -ErrorAction SilentlyContinue
            $alreadyShared = $cItem -and ($cItem.Attributes -band [System.IO.FileAttributes]::ReparsePoint)

            if ($alreadyShared) {
                Write-Host "Profile '$profile' is already sharing host config.json."
            } else {
                if ((Test-Path $targetConfig) -and !($cItem -and ($cItem.Attributes -band [System.IO.FileAttributes]::ReparsePoint))) {
                    Move-Item -Path $targetConfig -Destination "$targetConfig.bak" -Force
                    Write-Host "Backed up existing config.json to config.json.bak"
                }
                Link-UserConfig $profilePath
                Write-Host "Profile '$profile' is now sharing host config.json."
            }
        }
        "isolate" {
            New-Item -ItemType File -Force -Path "$profilePath\.isolated_config" | Out-Null
            $cItem = Get-Item -Path $targetConfig -ErrorAction SilentlyContinue
            if ($cItem -and ($cItem.Attributes -band [System.IO.FileAttributes]::ReparsePoint)) {
                Remove-Item -Force -Path $targetConfig -ErrorAction SilentlyContinue
                if (Test-Path $hostConfig) {
                    Copy-Item -Path $hostConfig -Destination $targetConfig -Force
                    Write-Host "Copied host config.json to standalone file for '$profile'."
                }
            }
            Write-Host "Profile '$profile' is now isolated from host config updates."
        }
        default {
            Write-Error "Error: usage: multigravity config <status|share|isolate> <profile>"
            exit 1
        }
    }
}

function Invoke-GhCmd {
    param($action, $profile)

    if ([string]::IsNullOrEmpty($action) -or [string]::IsNullOrEmpty($profile)) {
        Write-Error "Error: usage: multigravity gh <status|share|isolate> <profile>"
        exit 1
    }

    Validate-Name $profile

    $profilePath = "$BASE\$profile"
    if (!(Test-Path $profilePath)) {
        Write-Error "Error: profile '$profile' does not exist"
        exit 1
    }

    $realUser = if ($env:REAL_USERPROFILE) { $env:REAL_USERPROFILE } else { $REAL_USERPROFILE }
    if ([string]::IsNullOrEmpty($realUser)) { $realUser = $env:USERPROFILE }

    $hostGh = "$realUser\AppData\Roaming\GitHub CLI"
    $targetGh = "$profilePath\AppData\Roaming\GitHub CLI"

    switch ($action) {
        "status" {
            if (Test-Path "$profilePath\.isolated_gh") {
                Write-Host "Profile '$profile' has isolated GitHub CLI credentials (--isolated-gh active)."
            } elseif (Test-Path "$profilePath\.isolated_dotfiles") {
                Write-Host "Profile '$profile' has isolated dotfiles (--isolated-dotfiles active)."
            } elseif (Test-Path $targetGh) {
                $ghItem = Get-Item -Path $targetGh -ErrorAction SilentlyContinue
                $isShared = $ghItem -and ($ghItem.Attributes -band [System.IO.FileAttributes]::ReparsePoint)
                if ($isShared) {
                    $ghTarget = if ($ghItem.Target) { $ghItem.Target } else { "(symlink/junction)" }
                    Write-Host "Profile '$profile' shares host GitHub CLI credentials -> $ghTarget"
                } else {
                    Write-Host "Profile '$profile' has standalone local GitHub CLI credentials."
                }
            } else {
                Write-Host "Profile '$profile' has no GitHub CLI credentials configured."
            }
        }
        "share" {
            if (Test-Path "$profilePath\.isolated_gh") {
                Remove-Item -Force -Path "$profilePath\.isolated_gh" -ErrorAction SilentlyContinue
            }
            $ghItem = Get-Item -Path $targetGh -ErrorAction SilentlyContinue
            $alreadyShared = $ghItem -and ($ghItem.Attributes -band [System.IO.FileAttributes]::ReparsePoint)

            if ($alreadyShared) {
                Write-Host "Profile '$profile' is already sharing host GitHub CLI credentials."
            } else {
                if ((Test-Path $targetGh) -and !($ghItem -and ($ghItem.Attributes -band [System.IO.FileAttributes]::ReparsePoint))) {
                    Move-Item -Path $targetGh -Destination "$targetGh.bak" -Force
                    Write-Host "Backed up existing GitHub CLI directory to 'GitHub CLI.bak'"
                }
                Link-GhConfig $profilePath
                Write-Host "Profile '$profile' is now sharing host GitHub CLI credentials."
            }
        }
        "isolate" {
            New-Item -ItemType File -Force -Path "$profilePath\.isolated_gh" | Out-Null
            $ghItem = Get-Item -Path $targetGh -ErrorAction SilentlyContinue
            if ($ghItem -and ($ghItem.Attributes -band [System.IO.FileAttributes]::ReparsePoint)) {
                Remove-Item -Force -Path $targetGh -ErrorAction SilentlyContinue
                if (Test-Path $hostGh) {
                    Copy-Item -Path $hostGh -Destination $targetGh -Recurse -Force
                    Write-Host "Copied host GitHub CLI credentials to standalone directory for '$profile'."
                }
            }
            Write-Host "Profile '$profile' is now isolated from host GitHub CLI credentials updates."
        }
        default {
            Write-Error "Error: usage: multigravity gh <status|share|isolate> <profile>"
            exit 1
        }
    }
}

function Invoke-QuotaCmd {
    param($targetProfile)

    if ($targetProfile) {
        Validate-Name $targetProfile
        $pDir = "$BASE\$targetProfile"
        if (!(Test-Path $pDir)) {
            Write-Error "Error: profile '$targetProfile' does not exist"
            exit 1
        }
    }

    $procs = Get-CimInstance Win32_Process -Filter "Name = 'language_server.exe'" -ErrorAction SilentlyContinue
    if (!$procs) {
        if ($targetProfile) {
            Write-Host "Profile '$targetProfile' is not running."
            Write-Host "To check quota and token consumption, launch it first: multigravity $targetProfile"
        } else {
            Write-Host "No active Antigravity profile or Language Server found in execution."
            Write-Host "Launch a profile to monitor quota: multigravity <name>"
        }
        return
    }

    $foundAny = $false
    foreach ($proc in $procs) {
        $cmdLine = $proc.CommandLine
        if ([string]::IsNullOrEmpty($cmdLine)) { continue }

        $csrfMatch = [regex]::Match($cmdLine, "--csrf_token\s+([a-f0-9-]+)")
        if (!$csrfMatch.Success) { continue }
        $csrf = $csrfMatch.Groups[1].Value

        $profileName = "host"
        $ppid = $proc.ParentProcessId
        if ($ppid) {
            $parent = Get-CimInstance Win32_Process -Filter "ProcessId = $ppid" -ErrorAction SilentlyContinue
            if ($parent -and $parent.CommandLine) {
                $m = [regex]::Match($parent.CommandLine, "AntigravityProfiles[\\/]([^\\/ ]+)")
                if ($m.Success) {
                    $profileName = $m.Groups[1].Value
                }
            }
        }

        if ($targetProfile -and ($profileName -ne $targetProfile)) { continue }

        $ports = @()
        $conns = Get-NetTCPConnection -OwningProcess $proc.ProcessId -State Listen -ErrorAction SilentlyContinue
        if ($conns) {
            foreach ($c in $conns) {
                $ports += $c.LocalPort
            }
        }

        $resData = $null
        $usedPort = 0
        foreach ($port in ($ports | Select-Object -Unique)) {
            $url = "https://127.0.0.1:$port/exa.language_server_pb.LanguageServerService/RetrieveUserQuotaSummary"
            try {
                $headers = @{
                    "Content-Type" = "application/json"
                    "X-Codeium-Csrf-Token" = $csrf
                }
                [System.Net.ServicePointManager]::ServerCertificateValidationCallback = { $true }
                $resp = Invoke-RestMethod -Uri $url -Method Post -Headers $headers -Body "{}" -TimeoutSec 3 -ErrorAction Stop
                if ($resp -and $resp.response) {
                    $resData = $resp.response
                    $usedPort = $port
                    break
                }
            } catch {}
        }

        if (!$resData) { continue }
        $foundAny = $true

        Write-Host ""
        Write-Host "Quota Status — Profile: $profileName (PID $($proc.ProcessId), Port $usedPort)"
        Write-Host "============================================================"
        
        $groups = $resData.groups
        if (!$groups) {
            Write-Host "  No quota information returned."
            continue
        }

        foreach ($g in $groups) {
            Write-Host ""
            Write-Host "• $($g.displayName) ($($g.description)):"
            foreach ($b in $g.buckets) {
                $remFrac = [double]$b.remainingFraction
                $remPct  = [math]::Round($remFrac * 100, 1)
                $usedPct = [math]::Round(100 - $remPct, 1)
                $timeLeftStr = ""
                if ($b.resetTime) {
                    try {
                        $rt = [DateTime]::Parse($b.resetTime).ToUniversalTime()
                        $now = [DateTime]::UtcNow
                        $diff = $rt - $now
                        if ($diff.TotalSeconds -gt 0) {
                            $hours = [math]::Floor($diff.TotalHours)
                            $mins = $diff.Minutes
                            $timeLeftStr = " | Resets in: $($hours)h $($mins)m"
                        } else {
                            $timeLeftStr = " | Quota refreshed!"
                        }
                    } catch {
                        $timeLeftStr = " | Reset: $($b.resetTime)"
                    }
                }

                $barLen = 20
                $filled = [int][math]::Round($barLen * ($remPct / 100.0))
                if ($filled -lt 0) { $filled = 0 }
                if ($filled -gt $barLen) { $filled = $barLen }
                $bar = "[" + ("=" * $filled) + (" " * ($barLen - $filled)) + "]"

                Write-Host "  - $($b.displayName):"
                Write-Host "    $bar Remaining: $remPct% | Used: $usedPct%$timeLeftStr"
                if ($b.description) {
                    Write-Host "    Details: $($b.description)"
                }
            }
        }
        Write-Host ""
    }

    if (!$foundAny) {
        if ($targetProfile) {
            Write-Host "Profile '$targetProfile' is not running."
            Write-Host "To check quota and token consumption, launch it first: multigravity $targetProfile"
        } else {
            Write-Host "No active Antigravity profile or Language Server found in execution."
            Write-Host "Launch a profile to monitor quota: multigravity <name>"
        }
    }
}

function Invoke-PrimeCmd {
    param(
        $targetProfile,
        [switch]$Force,
        [switch]$Check,
        [switch]$Status,
        [switch]$NoJitter,
        [int]$JitterMinutes = 60,
        [switch]$Include5h,
        [switch]$FiveHours,
        [switch]$InstallTask,
        [switch]$UninstallTask,
        [switch]$Quiet
    )

    if ($args) {
        foreach ($a in $args) {
            switch ($a) {
                "--5h"             { $Include5h = $true }
                "--include-5h"     { $Include5h = $true }
                "-Include5h"       { $Include5h = $true }
                "-FiveHours"       { $Include5h = $true }
                "--force"          { $Force = $true }
                "-Force"           { $Force = $true }
                "--check"          { $Check = $true }
                "-Check"           { $Check = $true }
                "--status"         { $Status = $true }
                "-Status"          { $Status = $true }
                "--no-jitter"      { $NoJitter = $true }
                "-NoJitter"        { $NoJitter = $true }
                "--quiet"          { $Quiet = $true }
                "-Quiet"           { $Quiet = $true }
                "--install-cron"   { $InstallTask = $true }
                "-InstallTask"     { $InstallTask = $true }
                "--uninstall-cron" { $UninstallTask = $true }
                "-UninstallTask"   { $UninstallTask = $true }
                default {
                    if (!$a.StartsWith("-") -and [string]::IsNullOrEmpty($targetProfile)) {
                        $targetProfile = $a
                    }
                }
            }
        }
    }

    if ([string]::IsNullOrEmpty($targetProfile)) {
        $profiles = @(Get-ChildItem -Directory -Path $BASE -ErrorAction SilentlyContinue | Where-Object { $_.Name -ne ".templates" })
        if ($profiles.Count -eq 1) {
            $targetProfile = $profiles[0].Name
        }
    }

    $taskName = "MultigravityPrime-$targetProfile"
    if ($InstallTask) {
        if ([string]::IsNullOrEmpty($targetProfile)) {
            Write-Error "Error: profile name is required for -InstallTask"
            exit 1
        }
        Validate-Name $targetProfile
        $psExe = (Get-Process -Id $PID).Path
        if (!$psExe) { $psExe = "powershell.exe" }
        $extraAction = if ($Include5h -or $FiveHours) { " --5h" } else { "" }
        $action = "-ExecutionPolicy Bypass -NoProfile -File `"$PSCommandPath`" prime $targetProfile$extraAction -Quiet"
        schtasks.exe /create /tn "$taskName" /tr "`"$psExe`" $action" /sc hourly /mo 1 /f | Out-Null
        if ($LASTEXITCODE -eq 0) {
            Write-Host "Installed hourly scheduled task '$taskName' for profile '$targetProfile'$(if ($extraAction) { ' (including 5-hour limits)' })."
            Write-Host "Schedule: Every hour"
        } else {
            Write-Error "Failed to create scheduled task '$taskName'."
        }
        return
    }

    if ($UninstallTask) {
        if ([string]::IsNullOrEmpty($targetProfile)) {
            Write-Error "Error: profile name is required for -UninstallTask"
            exit 1
        }
        Validate-Name $targetProfile
        schtasks.exe /delete /tn "$taskName" /f 2>$null | Out-Null
        Write-Host "Removed scheduled task '$taskName' for profile '$targetProfile'."
        return
    }

    if ($targetProfile) {
        Validate-Name $targetProfile
        $pDir = "$BASE\$targetProfile"
        if (!(Test-Path $pDir)) {
            Write-Error "Error: profile '$targetProfile' does not exist"
            exit 1
        }
    }

    $stateDir = "$env:LOCALAPPDATA\multigravity"
    if (!(Test-Path $stateDir)) {
        New-Item -ItemType Directory -Path $stateDir -Force | Out-Null
    }
    $stateFile = "$stateDir\prime_state.json"
    $state = @{ "profiles" = @{} }
    if (Test-Path $stateFile) {
        try {
            $raw = Get-Content -Raw -Path $stateFile -ErrorAction Stop
            $parsed = ConvertFrom-Json $raw
            if ($parsed.profiles) {
                foreach ($prop in $parsed.profiles.PSObject.Properties) {
                    $state["profiles"][$prop.Name] = @{
                        "last_primed_at"   = $prop.Value.last_primed_at
                        "last_reset_time"  = $prop.Value.last_reset_time
                        "last_cascade_id"  = $prop.Value.last_cascade_id
                        "last_model"       = $prop.Value.last_model
                        "target_prime_time"= $prop.Value.target_prime_time
                        "status"           = $prop.Value.status
                    }
                }
            }
        } catch {}
    }

    $procs = Get-CimInstance Win32_Process -Filter "Name = 'language_server.exe'" -ErrorAction SilentlyContinue
    $usedPort = 0
    $usedCsrf = $null
    $procPid = 0
    $headlessProc = $null

    if ($procs) {
        foreach ($proc in $procs) {
            $cmdLine = $proc.CommandLine
            if ([string]::IsNullOrEmpty($cmdLine)) { continue }

            $csrfMatch = [regex]::Match($cmdLine, "--csrf_token\s+([a-f0-9-]+)")
            if (!$csrfMatch.Success) { continue }
            $c = $csrfMatch.Groups[1].Value

            $profName = "host"
            $ppid = $proc.ParentProcessId
            if ($ppid) {
                $parent = Get-CimInstance Win32_Process -Filter "ProcessId = $ppid" -ErrorAction SilentlyContinue
                if ($parent -and $parent.CommandLine) {
                    $m = [regex]::Match($parent.CommandLine, "AntigravityProfiles[\\/]([^\\/ ]+)")
                    if ($m.Success) { $profName = $m.Groups[1].Value }
                }
            }

            if ($targetProfile -and ($profName -ne $targetProfile)) { continue }
            if (!$targetProfile -and $profName) { $targetProfile = $profName }

            $ports = @()
            $conns = Get-NetTCPConnection -OwningProcess $proc.ProcessId -State Listen -ErrorAction SilentlyContinue
            if ($conns) {
                foreach ($cn in $conns) { $ports += $cn.LocalPort }
            }

            foreach ($p in ($ports | Select-Object -Unique)) {
                $url = "https://127.0.0.1:$p/exa.language_server_pb.LanguageServerService/RetrieveUserQuotaSummary"
                try {
                    $headers = @{ "Content-Type" = "application/json"; "X-Codeium-Csrf-Token" = $c }
                    [System.Net.ServicePointManager]::ServerCertificateValidationCallback = { $true }
                    $resp = Invoke-RestMethod -Uri $url -Method Post -Headers $headers -Body "{}" -TimeoutSec 2 -ErrorAction Stop
                    if ($resp -and $resp.response) {
                        $usedPort = $p
                        $usedCsrf = $c
                        $procPid = $proc.ProcessId
                        break
                    }
                } catch {}
            }
            if ($usedPort -gt 0) { break }
        }
    }

    if ($usedPort -eq 0) {
        $lsBin = Find-LanguageServer
        if (!$lsBin) {
            if (!$Quiet) {
                Write-Error "Error: Language server executable not found and profile '$targetProfile' is not running."
            }
            exit 1
        }
        $pDir = if ($targetProfile) { "$BASE\$targetProfile" } else { $null }
        $pGemini = if ($pDir) { "$pDir\.gemini" } else { "$env:USERPROFILE\.gemini" }
        $tempCsrf = [guid]::NewGuid().ToString()
        $headlessArgs = @(
            "--standalone",
            "--headless=true",
            "--gemini_dir", $pGemini,
            "--app_data_dir", "antigravity",
            "--csrf_token", $tempCsrf,
            "--https_server_port", "0",
            "--http_server_port", "0",
            "--api_server_url", "https://generativelanguage.googleapis.com",
            "--cloud_code_endpoint", "https://daily-cloudcode-pa.googleapis.com"
        )
        $origProfile = $env:USERPROFILE
        try {
            if ($pDir) { $env:USERPROFILE = $pDir }
            $headlessProc = Start-Process -FilePath $lsBin -ArgumentList $headlessArgs -PassThru -WindowStyle Hidden
            for ($i = 0; $i -lt 30; $i++) {
                Start-Sleep -Milliseconds 100
                $conns = Get-NetTCPConnection -OwningProcess $headlessProc.Id -State Listen -ErrorAction SilentlyContinue
                if ($conns) {
                    foreach ($cn in $conns) {
                        $url = "https://127.0.0.1:$($cn.LocalPort)/exa.language_server_pb.LanguageServerService/RetrieveUserQuotaSummary"
                        try {
                            $headers = @{ "Content-Type" = "application/json"; "X-Codeium-Csrf-Token" = $tempCsrf }
                            [System.Net.ServicePointManager]::ServerCertificateValidationCallback = { $true }
                            $resp = Invoke-RestMethod -Uri $url -Method Post -Headers $headers -Body "{}" -TimeoutSec 1 -ErrorAction Stop
                            if ($resp -and $resp.response) {
                                $usedPort = $cn.LocalPort
                                $usedCsrf = $tempCsrf
                                $procPid = $headlessProc.Id
                                break
                            }
                        } catch {}
                    }
                }
                if ($usedPort -gt 0) { break }
            }
        } catch {
            if (!$Quiet) { Write-Error "Failed to start headless language server: $_" }
            exit 1
        } finally {
            $env:USERPROFILE = $origProfile
        }
    }

    if ($usedPort -eq 0) {
        if ($headlessProc) { Stop-Process -Id $headlessProc.Id -Force -ErrorAction SilentlyContinue }
        if (!$Quiet) { Write-Error "Could not connect to Language Server." }
        exit 1
    }

    $quotaResp = $null
    try {
        $headers = @{ "Content-Type" = "application/json"; "X-Codeium-Csrf-Token" = $usedCsrf }
        [System.Net.ServicePointManager]::ServerCertificateValidationCallback = { $true }
        $url = "https://127.0.0.1:$usedPort/exa.language_server_pb.LanguageServerService/RetrieveUserQuotaSummary"
        $quotaResp = Invoke-RestMethod -Uri $url -Method Post -Headers $headers -Body "{}" -TimeoutSec 5 -ErrorAction Stop
    } catch {
        if ($headlessProc) { Stop-Process -Id $headlessProc.Id -Force -ErrorAction SilentlyContinue }
        if (!$Quiet) { Write-Error "Failed to query quota summary: $_" }
        exit 1
    }

    $promptsFile = "$stateDir\prompts.json"
    $defaultPrompts = @(
        # English
        "ping",
        "Hello! Quick status check.",
        "Hi, are you ready?",
        "Good morning! Ready for today's tasks?",
        "Quick ping test, thanks!",
        "Hi there! How is everything running?",
        "Ready to assist today?",
        "Hello, just checking in.",
        "Quick connectivity check.",
        "Hi! All systems operational?",
        "Hello! Quick sanity check.",
        "Hi, checking in for a new session.",
        "Good day! Ready when you are.",
        "Quick check: system online?",
        "Hello! Confirming connection.",
        "Hi there, ready for coding?",
        "Ping test, please acknowledge.",
        "Good morning! Everything running smoothly?",
        "Hi! Quick hello before getting started.",
        "Testing connection, thanks!",
        # Portuguese
        "Olá! Tudo bem por aí?",
        "Oi! Teste rápido de status.",
        "Bom dia! Pronto para os trabalhos de hoje?",
        "Olá, tudo funcionando certinho?",
        "Oi, checagem rápida de conexão.",
        "Pronto para ajudar hoje?",
        "Olá! Sistema operacional?",
        "Checagem rápida de status, valeu!",
        "Oi, apenas confirmando conexão.",
        "Olá! Pronto para começar?",
        "Bom dia! Tudo certo por aqui?",
        "Olá, teste rápido de comunicação.",
        "Oi! Sistema ativo?",
        "Checagem de rotina, tudo ok?",
        "Olá, pronto para mais uma sessão?",
        "Oi, confirmando disponibilidade.",
        "Teste de ping, obrigado!",
        "Olá, tudo tranquilo?",
        "Bom dia, pronto para codar?",
        "Oi, verificação rápida do assistente."
    )

    $promptPool = @()
    if (Test-Path $promptsFile) {
        try {
            $pData = Get-Content -Path $promptsFile -Raw | ConvertFrom-Json
            if ($pData -and $pData.prompts) {
                $promptPool = @($pData.prompts)
            }
        } catch {}
    }
    if ($promptPool.Count -eq 0) {
        $promptPool = $defaultPrompts
        try {
            $pObj = @{ "prompts" = $defaultPrompts }
            Set-Content -Path $promptsFile -Value (ConvertTo-Json $pObj -Depth 5) -Force
        } catch {}
    }

    $bucketConfigs = @(
        @{
            "key" = "gemini"
            "bucket_id" = "gemini-weekly"
            "name" = "Gemini Models (Weekly)"
            "period" = "weekly"
            "parent_key" = $null
            "model" = "MODEL_PLACEHOLDER_M73"
            "model_label" = "gemini-3.6-flash-low"
        },
        @{
            "key" = "gemini_5h"
            "bucket_id" = "gemini-5h"
            "name" = "Gemini Models (5-Hour Window)"
            "period" = "5h"
            "parent_key" = "gemini"
            "model" = "MODEL_PLACEHOLDER_M73"
            "model_label" = "gemini-3.6-flash-low"
        },
        @{
            "key" = "3p"
            "bucket_id" = "3p-weekly"
            "name" = "Claude & GPT models (Weekly)"
            "period" = "weekly"
            "parent_key" = $null
            "model" = "MODEL_PLACEHOLDER_M35"
            "model_label" = "claude-sonnet-4-6"
        },
        @{
            "key" = "3p_5h"
            "bucket_id" = "3p-5h"
            "name" = "Claude & GPT models (5-Hour Window)"
            "period" = "5h"
            "parent_key" = "3p"
            "model" = "MODEL_PLACEHOLDER_M35"
            "model_label" = "claude-sonnet-4-6"
        }
    )

    $buckets = @{}
    if ($quotaResp -and $quotaResp.response -and $quotaResp.response.groups) {
        foreach ($g in $quotaResp.response.groups) {
            foreach ($b in $g.buckets) {
                if ($b.bucketId -in @("gemini-weekly", "3p-weekly", "gemini-5h", "3p-5h")) {
                    $buckets[$b.bucketId] = $b
                }
            }
        }
    }

    if ($buckets.Count -eq 0) {
        if ($headlessProc) { Stop-Process -Id $headlessProc.Id -Force -ErrorAction SilentlyContinue }
        if (!$Quiet) { Write-Error "No quota buckets found in telemetry." }
        exit 1
    }

    $now = [DateTime]::UtcNow
    $profKey = if ($targetProfile) { $targetProfile } else { "default" }
    if (!$state["profiles"].ContainsKey($profKey)) {
        $state["profiles"][$profKey] = @{}
    }
    $profState = $state["profiles"][$profKey]

    # Migrate legacy flat state to dual-bucket state if needed
    if ($profState.last_reset_time -and !$profState.gemini) {
        $legacyGemini = @{
            "last_primed_at" = $profState.last_primed_at
            "last_reset_time" = $profState.last_reset_time
            "last_cascade_id" = $profState.last_cascade_id
            "last_model" = if ($profState.last_model) { $profState.last_model } else { "gemini-3.6-flash-low" }
            "last_prompt" = $profState.last_prompt
            "target_prime_time" = $profState.target_prime_time
            "status" = if ($profState.status) { $profState.status } else { "success" }
        }
        $profState = @{
            "gemini" = $legacyGemini
            "3p" = @{}
        }
        $state["profiles"][$profKey] = $profState
    }

    if ($Status) {
        if ($headlessProc) { Stop-Process -Id $headlessProc.Id -Force -ErrorAction SilentlyContinue }
        
        $taskInstalled = $false
        $taskHas5h = $false
        $taskQuery = schtasks.exe /query /tn "MultigravityPrime-$profKey" /fo list 2>$null
        if ($LASTEXITCODE -eq 0) {
            $taskInstalled = $true
            if ($taskQuery -match "--5h") { $taskHas5h = $true }
        }

        Write-Host ""
        Write-Host "Prime Status — Profile: $profKey"
        Write-Host "============================================================"
        $serverMode = if ($headlessProc) { "Headless (Standby)" } else { "Active IDE Instance (PID $procPid, Port $usedPort)" }
        Write-Host "  Language Server:    $serverMode"

        foreach ($cfg in $bucketConfigs) {
            $b = $buckets[$cfg.bucket_id]
            $bState = if ($profState[$cfg.key]) { $profState[$cfg.key] } else { @{} }

            Write-Host ""
            Write-Host "  • $($cfg.name) ($($cfg.bucket_id)):"
            if (!$b) {
                Write-Host "    Status:           Not available in telemetry"
                continue
            }

            $remFrac = [double]$b.remainingFraction
            $resetTimeStr = $b.resetTime
            $timeLeftStr = ""
            $diffSec = 0
            if ($resetTimeStr) {
                try {
                    $rt = [DateTime]::Parse($resetTimeStr).ToUniversalTime()
                    $diff = $rt - $now
                    $diffSec = [int]$diff.TotalSeconds
                    if ($diffSec -gt 0) {
                        $hours = [math]::Floor($diff.TotalHours)
                        $mins = $diff.Minutes
                        $timeLeftStr = "$($hours)h $($mins)m"
                    } else {
                        $timeLeftStr = "Refreshed!"
                    }
                } catch {
                    $timeLeftStr = "$resetTimeStr"
                }
            }

            $lastPrimed = if ($bState.last_primed_at) { $bState.last_primed_at } else { "Never" }
            $lastModel = if ($bState.last_model) { $bState.last_model } else { "-" }
            $lastPrompt = if ($bState.last_prompt) { ", prompt: `"$($bState.last_prompt)`"" } else { "" }
            $lastReset = if ($bState.last_reset_time) { $bState.last_reset_time } else { "-" }
            $targetPrimeTime = $bState.target_prime_time

            $isRefreshed = ($remFrac -ge 0.999) -or ($diffSec -le 0)
            $alreadyPrimed = ($lastReset -eq $resetTimeStr) -and (!$isRefreshed)
            $cycleStatus = "Active (Countdown running)"
            if ($alreadyPrimed) {
                $cycleStatus = "Active ($($cfg.period) countdown is currently running)"
            } elseif ($isRefreshed) {
                if ($targetPrimeTime) {
                    $cycleStatus = "Pending Prime with Jitter (Scheduled at: $targetPrimeTime)"
                } else {
                    $cycleStatus = "Ready to Prime (Reset occurred / New cycle waiting to start)"
                }
            }

            Write-Host "    Limit Quota:      $([math]::Round($remFrac * 100, 1))% remaining | Resets in: $timeLeftStr"
            Write-Host "    Reset Target:     $resetTimeStr"
            Write-Host "    Last Primed:      $lastPrimed (model: $lastModel$lastPrompt)"
            Write-Host "    Cycle Status:     $cycleStatus"
        }

        Write-Host ""
        Write-Host "  Scheduled Watchdog:"
        $taskDesc = if ($taskInstalled -and $taskHas5h) { "Installed (Hourly, Weekly + 5h)" } elseif ($taskInstalled) { "Installed (Hourly, Weekly only)" } else { "Not installed" }
        Write-Host "    Scheduled Task:   $taskDesc"
        Write-Host "    Prompts Catalog:  $promptsFile ($($promptPool.Count) prompts loaded)"
        Write-Host ""
        return
    }

    # Execute Priming per bucket
    $usedPrompts = @()
    $primedProviders = @()
    try {
        foreach ($cfg in $bucketConfigs) {
            if ($cfg.period -eq "5h" -and !$Include5h -and !$FiveHours -and !$Force) {
                continue
            }

            $b = $buckets[$cfg.bucket_id]
            if (!$b) { continue }

            if ($cfg.parent_key) {
                $parentB = $buckets["$($cfg.parent_key)-weekly"]
                if ($parentB) {
                    $pRem = [double]$parentB.remainingFraction
                    if ($pRem -le 0.05 -and !$Force) {
                        if (!$Quiet) {
                            Write-Host "Skipping 5h prime for $($cfg.name): weekly quota is exhausted ($([math]::Round($pRem * 100, 1))% remaining)."
                        }
                        continue
                    }
                }
            }

            $remFrac = [double]$b.remainingFraction
            $resetTimeStr = $b.resetTime
            $timeLeftStr = ""
            $diffSec = 0
            if ($resetTimeStr) {
                try {
                    $rt = [DateTime]::Parse($resetTimeStr).ToUniversalTime()
                    $diff = $rt - $now
                    $diffSec = [int]$diff.TotalSeconds
                    if ($diffSec -gt 0) {
                        $hours = [math]::Floor($diff.TotalHours)
                        $mins = $diff.Minutes
                        $timeLeftStr = "$($hours)h $($mins)m"
                    } else {
                        $timeLeftStr = "Refreshed!"
                    }
                } catch {
                    $timeLeftStr = "$resetTimeStr"
                }
            }

            $isRefreshed = ($remFrac -ge 0.999) -or ($diffSec -le 0)
            if (!$profState.ContainsKey($cfg.key)) { $profState[$cfg.key] = @{} }
            $bState = $profState[$cfg.key]
            $lastReset = $bState.last_reset_time
            $alreadyPrimed = ($lastReset -eq $resetTimeStr) -and ($remFrac -lt 0.999)

            if (!$Force) {
                if ($alreadyPrimed) {
                    if (!$Quiet) {
                        Write-Host "Quota for '$profKey' [$($cfg.name)] is already primed and active for this cycle."
                        Write-Host "Current cycle resets in $timeLeftStr ($resetTimeStr)."
                    }
                    continue
                }

                if (!$isRefreshed) {
                    if (!$Quiet) {
                        Write-Host "Quota for '$profKey' [$($cfg.name)] has not reset yet ($([math]::Round($remFrac * 100, 1))% remaining, resets in $timeLeftStr)."
                    }
                    continue
                }

                if ($Check) {
                    Write-Host "Quota for '$profKey' [$($cfg.name)] is READY for priming (Reset occurred / New cycle waiting)."
                    continue
                }

                # Link if provider was already primed in this session
                if ($primedProviders -contains $cfg.model) {
                    $bState["last_primed_at"] = [DateTime]::UtcNow.ToString("o")
                    $bState["last_reset_time"] = $resetTimeStr
                    $bState["last_model"] = $cfg.model_label
                    $bState["last_prompt"] = "(Linked with $($cfg.parent_key) prime)"
                    $bState["target_prime_time"] = $null
                    $bState["status"] = "success"
                    $profState[$cfg.key] = $bState
                    $state["profiles"][$profKey] = $profState
                    Set-Content -Path $stateFile -Value (ConvertTo-Json $state -Depth 5) -Force
                    if (!$Quiet) {
                        Write-Host "✓ 5-Hour window for '$profKey' [$($cfg.name)] auto-linked with previous prime in this session."
                    }
                    continue
                }

                $bucketMaxJitter = if ($cfg.period -eq "5h") { [math]::Min($JitterMinutes, 15) } else { $JitterMinutes }
                if (!$NoJitter -and ($bucketMaxJitter -gt 0)) {
                    $targetStr = $bState.target_prime_time
                    $targetTime = $null
                    if ($targetStr) {
                        try { $targetTime = [DateTime]::Parse($targetStr).ToUniversalTime() } catch {}
                    }
                    if (!$targetTime -or ($targetTime -lt $now.AddHours(-2))) {
                        $jitterSec = Get-Random -Minimum 0 -Maximum ($bucketMaxJitter * 60)
                        $targetTime = $now.AddSeconds($jitterSec)
                        $bState["target_prime_time"] = $targetTime.ToString("o")
                        $profState[$cfg.key] = $bState
                        $state["profiles"][$profKey] = $profState
                        Set-Content -Path $stateFile -Value (ConvertTo-Json $state -Depth 5) -Force
                    }

                    if ($now -lt $targetTime) {
                        $waitS = [int]($targetTime - $now).TotalSeconds
                        if ($Quiet) {
                            continue
                        } else {
                            Write-Host "Anti-bot jitter for $($cfg.name): waiting $([math]::Floor($waitS / 60))m $($waitS % 60)s before priming..."
                            Start-Sleep -Seconds $waitS
                        }
                    }
                }
            }

            # Last-mile pre-prime check
            $manualActivity = $false
            try {
                $chkHeaders = @{ "Content-Type" = "application/json"; "X-Codeium-Csrf-Token" = $usedCsrf }
                $chkResp = Invoke-RestMethod -Uri "https://127.0.0.1:$usedPort/exa.language_server_pb.LanguageServerService/RetrieveUserQuotaSummary" -Method Post -Headers $chkHeaders -Body "{}" -TimeoutSec 3 -ErrorAction SilentlyContinue
                if ($chkResp -and $chkResp.response -and $chkResp.response.groups) {
                    foreach ($cg in $chkResp.response.groups) {
                        foreach ($cb in $cg.buckets) {
                            if ($cb.bucketId -eq $cfg.bucket_id) {
                                $freshRem = [double]$cb.remainingFraction
                                $freshRst = $cb.resetTime
                                if (!$Force -and ($freshRem -lt 0.999) -and (($freshRst -ne $resetTimeStr) -or ($freshRem -lt ($remFrac - 0.005)))) {
                                    $manualActivity = $true
                                    if (!$Quiet) {
                                        Write-Host "Aborting prime for $($cfg.name): manual user activity detected (quota now at $([math]::Round($freshRem * 100, 1))%)."
                                    }
                                    $bState["target_prime_time"] = $null
                                    Set-Content -Path $stateFile -Value (ConvertTo-Json $state -Depth 5) -Force
                                    break
                                }
                            }
                        }
                    }
                }
            } catch {}

            if ($manualActivity) { continue }

            # Pick prompt not yet used in this run
            $available = $promptPool | Where-Object { $usedPrompts -notcontains $_ }
            if (!$available -or $available.Count -eq 0) { $available = $promptPool }
            $selectedPrompt = $available | Get-Random
            $usedPrompts += $selectedPrompt

            $headers = @{ "Content-Type" = "application/json"; "X-Codeium-Csrf-Token" = $usedCsrf }
            [System.Net.ServicePointManager]::ServerCertificateValidationCallback = { $true }
            
            $startUrl = "https://127.0.0.1:$usedPort/exa.language_server_pb.LanguageServerService/StartCascade"
            $startBody = '{"source":"CORTEX_TRAJECTORY_SOURCE_CLI"}'
            $startResp = Invoke-RestMethod -Uri $startUrl -Method Post -Headers $headers -Body $startBody -TimeoutSec 10 -ErrorAction Stop
            $cascadeId = $startResp.cascadeId

            $msgUrl = "https://127.0.0.1:$usedPort/exa.language_server_pb.LanguageServerService/SendUserCascadeMessage"
            $msgBodyObj = @{
                "cascadeId" = $cascadeId
                "items" = @(@{ "text" = $selectedPrompt })
                "cascadeConfig" = @{
                    "plannerConfig" = @{
                        "requestedModel" = @{
                            "model" = $cfg.model
                        }
                    }
                }
            }
            $msgJson = ConvertTo-Json $msgBodyObj -Depth 5
            Invoke-RestMethod -Uri $msgUrl -Method Post -Headers $headers -Body $msgJson -TimeoutSec 10 -ErrorAction Stop | Out-Null

            # Brief grace delay to allow the LLM response stream to finish saving in SQLite
            Start-Sleep -Seconds 3

            $bState["last_primed_at"] = [DateTime]::UtcNow.ToString("o")
            $bState["last_reset_time"] = $resetTimeStr
            $bState["last_cascade_id"] = $cascadeId
            $bState["last_model"] = $cfg.model_label
            $bState["last_prompt"] = $selectedPrompt
            $bState["target_prime_time"] = $null
            $bState["status"] = "success"
            $profState[$cfg.key] = $bState
            $state["profiles"][$profKey] = $profState
            Set-Content -Path $stateFile -Value (ConvertTo-Json $state -Depth 5) -Force
            $primedProviders += $cfg.model

            if (!$Quiet) {
                Write-Host ""
                Write-Host "✓ Successfully primed quota for profile '$profKey' [$($cfg.name)]!" -ForegroundColor Green
                Write-Host "  Model: $($cfg.model_label) (minimum token cost)"
                Write-Host "  Prompt: `"$selectedPrompt`""
                Write-Host "  Cascade ID: $cascadeId"
                Write-Host "  Reset countdown has officially started!"
            } else {
                Write-Host "[$([DateTime]::UtcNow.ToString('yyyy-MM-dd HH:mm:ss UTC'))] Primed $profKey [$($cfg.name)] with $($cfg.model_label): `"$selectedPrompt`""
            }
        }
    } finally {
        if ($headlessProc) { Stop-Process -Id $headlessProc.Id -Force -ErrorAction SilentlyContinue }
    }
}

function Invoke-InteractiveMenu {
    if (!(Test-Path $BASE)) {
        Write-Host "No profiles found."
        Write-Host "Create your first profile with: multigravity new <name>"
        return
    }

    $profiles = @(Get-ChildItem -Directory -Path $BASE -ErrorAction SilentlyContinue | Where-Object { $_.Name -ne ".templates" })
    if ($profiles.Count -eq 0) {
        Write-Host "No profiles found."
        Write-Host "Create your first profile with: multigravity new <name>"
        return
    }

    Write-Host ""
    Write-Host "Multigravity — Select Profile:" -ForegroundColor Cyan
    Write-Host ""

    for ($i = 0; $i -lt $profiles.Count; $i++) {
        $p = $profiles[$i]
        $name = $p.Name
        $isRunning = Test-ProfileRunning $name
        $runStatus = if ($isRunning) { "● running" } else { "○ idle" }
        $ptype = if (Test-Path "$($p.FullName)\.shared") { "shared" } else { "isolated" }

        $colorLabel = ""
        $c = Get-ProfileColor $name
        if ($c) { $colorLabel = "[$c]" }

        $num = "[{0}]" -f ($i + 1)
        if ($isRunning) {
            Write-Host ("  {0,-5} {1,-18} " -f $num, $name) -NoNewline
            Write-Host ("{0,-10} " -f $runStatus) -ForegroundColor Green -NoNewline
            Write-Host ("({0}) {1}" -f $ptype, $colorLabel)
        } else {
            Write-Host ("  {0,-5} {1,-18} {2,-10} ({3}) {4}" -f $num, $name, $runStatus, $ptype, $colorLabel)
        }
    }

    Write-Host ""
    Write-Host "  [n]   Create new profile"
    Write-Host "  [q]   Quit"
    Write-Host ""

    $choice = (Read-Host ("Select [1-{0}, n, q]" -f $profiles.Count)).Trim()
    if ([string]::IsNullOrWhiteSpace($choice) -or $choice -eq "q" -or $choice -eq "Q") {
        return
    }

    if ($choice -eq "n" -or $choice -eq "N") {
        $newName = (Read-Host "Enter new profile name").Trim()
        if (![string]::IsNullOrWhiteSpace($newName)) {
            Invoke-NewProfile $newName
        }
        return
    }

    if ($choice -match "^\d+$") {
        $idx = [int]$choice - 1
        if ($idx -ge 0 -and $idx -lt $profiles.Count) {
            $selected = $profiles[$idx].Name
            Invoke-LaunchProfile $selected @()
            return
        }
    }

    $match = $profiles | Where-Object { $_.Name -eq $choice } | Select-Object -First 1
    if ($match) {
        Invoke-LaunchProfile $match.Name @()
        return
    }

    Write-Host "Invalid selection: $choice"
}

switch ($cmd) {
    "new" {
        $extra = @()
        if ($arg2)       { $extra += $arg2 }
        if ($ForwardArgs) { $extra += $ForwardArgs }
        Invoke-NewProfile $arg1 $extra
    }
    "color" {
        Invoke-ProfileColorCmd $arg1 $arg2
    }
    "stop" {
        $force = ($arg2 -eq "--force" -or $arg2 -eq "-f")
        Stop-Profile $arg1 -Force:$force
    }
    "restart" {
        $extra = @()
        if ($arg2)       { $extra += $arg2 }
        if ($ForwardArgs) { $extra += $ForwardArgs }
        Restart-Profile $arg1 $extra
    }
    "clean" {
        Invoke-CleanProfile $arg1
    }
    "list" {
        Invoke-ListProfiles
    }
    "status" {
        Invoke-StatusProfiles
    }
    "rename" {
        Invoke-RenameProfile $arg1 $arg2
    }
    "delete" {
        $force = ($arg2 -eq "--force" -or $arg2 -eq "-f")
        Invoke-DeleteProfile $arg1 -Force:$force
    }
    "clone" {
        Invoke-CloneProfile $arg1 $arg2
    }
    "template" {
        Invoke-TemplateCmd $arg1 $arg2 ($ForwardArgs | Select-Object -First 1)
    }
    "export" {
        $all = @()
        if ($arg2) { $all += $arg2 }
        if ($ForwardArgs) { $all += $ForwardArgs }
        $inc = ($all -contains "--include-cache")
        $pathArg = ($all | Where-Object { $_ -ne "--include-cache" } | Select-Object -First 1)
        Invoke-ExportProfile $arg1 $pathArg -IncludeCache:$inc
    }
    "import" {
        Invoke-ImportProfile $arg1 $arg2
    }
    "ai" {
        Invoke-AiCmd $arg1 $arg2 ($ForwardArgs | Select-Object -First 1)
    }
    "mcp" {
        Invoke-McpCmd $arg1 $arg2
    }
    "skills" {
        Invoke-SkillsCmd $arg1 $arg2
    }
    "config" {
        Invoke-ConfigCmd $arg1 $arg2
    }
    "gh" {
        Invoke-GhCmd $arg1 $arg2
    }
    "quota" {
        Invoke-QuotaCmd $arg1
    }
    "prime" {
        $extra = @()
        if ($arg2)       { $extra += $arg2 }
        if ($ForwardArgs) { $extra += $ForwardArgs }
        Invoke-PrimeCmd $arg1 $extra
    }
    "update" {
        Invoke-UpdateCli
    }
    "doctor" {
        Invoke-DoctorCli
    }
    "stats" {
        Invoke-ProfileStats
    }
    "completion" {
        if ($arg1) {
            Invoke-GenerateCompletion $arg1
        } else {
            Invoke-HelpCompletion
        }
    }
    { $_ -in @("version", "--version", "-v") } {
        Invoke-VersionCli
    }
    "help"   { Write-Usage }
    "--help" { Write-Usage }
    "-h"     { Write-Usage }
    "" {
        if ([System.Console]::IsInputRedirected -or [System.Console]::IsOutputRedirected) {
            Write-Usage
            exit 1
        } else {
            Invoke-InteractiveMenu
        }
    }
    default {
        $AllArgs = @()
        if ($arg1)       { $AllArgs += $arg1 }
        if ($arg2)       { $AllArgs += $arg2 }
        if ($ForwardArgs) { $AllArgs += $ForwardArgs }
        Invoke-LaunchProfile $cmd $AllArgs
    }
}
