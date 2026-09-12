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
$BASE = if ($env:MULTIGRAVITY_HOME) { $env:MULTIGRAVITY_HOME } else { "$REAL_USERPROFILE\AntigravityProfiles" }
$VERSION = "1.3.0"

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
        $settingsFile = "$profileDir\AppData\Roaming\Antigravity\User\settings.json"
        if (Test-Path $settingsFile) {
            try {
                $obj = (Get-Content -Raw -Path $settingsFile -Encoding UTF8) | ConvertFrom-Json
                $curr = $obj.'workbench.colorCustomizations'.'titleBar.activeBackground'
                if ($curr) {
                    Write-Host "Profile '$profileName' color: $curr"
                    return
                }
            } catch {}
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
    Write-Host "  ai list <name>              List AI conversations in a profile"
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

    Write-Host "Launching Antigravity profile '$PROFILE'"
    
    # Preserve real user profile in env and launch Antigravity with isolated USERPROFILE
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
    $color             = ""
    $i = 0
    while ($i -lt $extraArgs.Count) {
        switch ($extraArgs[$i]) {
            "--shared"            { $shared = $true }
            "--from"              { $i++; if ($i -lt $extraArgs.Count) { $fromTpl = $extraArgs[$i] } }
            "--isolated-dotfiles" { $isolatedDotfiles = $true }
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

    if ($fromTpl) {
        $tplPath = "$(Get-TemplatesDir)\$fromTpl"
        if (!(Test-Path $tplPath)) {
            Write-Error "Error: template '$fromTpl' not found. Run: multigravity template list"
            exit 1
        }
        Write-Host "Creating profile '$name' from template '$fromTpl'..."
        Copy-Item -Path "$tplPath\*" -Destination $profileDir -Recurse -Force
        if (!$isolatedDotfiles) { Link-DevDotfiles $profileDir }
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

    $confirm = Read-Host "Delete profile '$PROFILE' and all its data? [y/N]"
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
    `$opts = @('new', 'color', 'stop', 'restart', 'clean', 'list', 'status', 'rename', 'delete', 'clone', 'template', 'export', 'import', 'ai', 'update', 'doctor', 'stats', 'completion', 'version', 'help')
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
            $_.Name -like "*token*" -or $_.Name -like "*oauth*" -or $_.Name -like "*auth*" -or $_.Name -eq "installation_id"
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
            $_.Name -like "*token*" -or $_.Name -like "*oauth*" -or $_.Name -like "*auth*" -or $_.Name -eq "installation_id"
        }
        foreach ($s in $sensitive) {
            Remove-Item -Path $s.FullName -Force -ErrorAction SilentlyContinue
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

function Invoke-AiCmd {
    param($sub, $arg1, $arg2)
    switch ($sub) {
        "export" { Invoke-AiExportProfile $arg1 $arg2 }
        "import" { Invoke-AiImportProfile $arg1 $arg2 }
        "list"   { Invoke-AiListProfile $arg1 }
        default  {
            Write-Error "Error: usage: multigravity ai <export|import|list> [args...]"
            exit 1
        }
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

        $settingsFile = "$($p.FullName)\AppData\Roaming\Antigravity\User\settings.json"
        $colorLabel = ""
        if (Test-Path $settingsFile) {
            try {
                $raw = Get-Content $settingsFile -Raw | ConvertFrom-Json
                $c = $raw.'workbench.colorCustomizations'.'titleBar.activeBackground'
                if ($c) { $colorLabel = "[$c]" }
            } catch {}
        }

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
