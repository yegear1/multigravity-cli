---
name: multigravity
description: Manages isolated Antigravity profiles, workspace isolation, telemetry, quota priming, and credentials sharing.
---

# Multigravity (`multigravity`)

Canonical specification for managing Google Antigravity IDE isolated profiles, telemetry inspection, quota priming, and credential/configuration sharing.

## 1. Scope & Blast Radius

- **MUTABLE PATHS:**
  - Profile storage directories: `$MULTIGRAVITY_HOME/<profile>/` (default: `~/AntigravityProfiles/<profile>/`).
  - Sentinel files inside profiles: `.isolated_mcp`, `.isolated_skills`, `.isolated_config`, `.isolated_gh`, `.isolated_dotfiles`.
  - Profile UI settings: `$MULTIGRAVITY_HOME/<profile>/User/settings.json`.
  - AI conversation export/import archives (`*.tar.gz`).
  - Global AI skills/config links when managing host customizations.
- **IMMUTABLE PATHS:**
  - Host dotfiles: `$REAL_HOME/.gitconfig`, `$REAL_HOME/.ssh/`, `$REAL_HOME/.config/gh/` (read/linked, never truncated or removed).
  - Antigravity IDE application binaries (`/usr/share/antigravity`, `/Applications/Antigravity.app`, etc.).
  - Active profile socket descriptors and process locks.
  - Project source repositories outside `$MULTIGRAVITY_HOME`.
- **FORBIDDEN ACTIONS:**
  - NEVER execute destructive profile actions (`delete`, `clean --all`) without verifying whether the profile is running (`multigravity status`).
  - NEVER remove or bypass `$HOME` redirection without preserving essential symlinks (`.gitconfig`, `.ssh`, `.config/gh`).
  - NEVER use profile names that violate the naming schema `^[a-zA-Z0-9][a-zA-Z0-9-]*$`.
  - NEVER commit plain-text tokens or credentials into exported AI conversation archives (`ai export` automatically sanitizes auth tokens).
  - NEVER run `ai prime` or `prime --force` blindly if weekly quota is depleted (< 5%) or if active human interaction has advanced the reset timer.

## 2. When to Use (Triggers)

The agent MUST activate this skill when:
- Operating in a multi-profile environment or needing to discover the active Antigravity profile context.
- Inspecting AI token limits, quota consumption, or reset schedules across Gemini weekly, Gemini 5h, 3P weekly, or 3P 5h buckets.
- Automating or validating quota prime cycles (`multigravity prime` or `multigravity ai prime`).
- Provisioning a new isolated developer workspace/profile (full or lightweight auth-only with shared host extensions via `--auth-only` / `--shared`) with dedicated GitHub CLI credentials, MCP servers, or custom theme colors.
- Sharing or isolating configuration components (`mcp`, `skills`, `config.json`, `gh`) between profiles.
- Backing up, exporting, restoring, or synchronizing AI chat histories and brains (`ai export`, `ai sync`, `ai import`).
- Diagnosing environment health, missing dependencies, or launch blockers (`multigravity doctor`).

The agent MUST NOT activate this skill when:
- Executing standard git operations within a project repository (use git or `github-releases`).
- Querying application logs or container infrastructure (activate `victorialogs-troubleshooting` or `homelab-inspector`).
- Managing non-Antigravity IDEs or standard system packages unrelated to Multigravity.

## 3. Required Tools & Prerequisites

- **Tools:** `multigravity` (executable in `$PATH` or invoked via `./multigravity`), `bash` (4.0+), `python3` (for telemetry and JSON queries), optional `jq`.
- **Pre-Conditions:**
  - Multigravity is installed and accessible (`command -v multigravity` or check `~/.local/bin/multigravity` / `/usr/local/bin/multigravity`).
  - Antigravity IDE executable is present or defined via `MULTIGRAVITY_APP`.
  - Profile directories adhere to `$MULTIGRAVITY_HOME` (default: `$HOME/AntigravityProfiles`).

## 4. Operational Procedure

### Step 1: Pre-Flight & Profile Identification
1. Detect current profile and runtime environment:
   ```bash
   multigravity doctor
   multigravity status
   ```
2. Verify active profile directory from `$HOME` or `$MULTIGRAVITY_HOME`:
   ```bash
   echo "Active Profile Directory: $HOME"
   multigravity list
   ```

### Step 2: Telemetry & Quota Inspection
1. Inspect AI quota balances and reset timestamps across all buckets:
   ```bash
   multigravity quota
   ```
2. Check if a priming prompt is pending for weekly or 5-hour cycles:
   ```bash
   multigravity prime --status
   multigravity prime <profile> --check --5h
   ```
3. If priming is required and quota has reset, execute prime with jitter protection:
   ```bash
   multigravity prime <profile> --include-5h
   ```

### Step 3: Managing Isolation Boundaries (MCP, Skills, Config, GitHub CLI)
1. Check isolation status for a target profile:
   ```bash
   multigravity mcp status <profile>
   multigravity skills status <profile>
   multigravity config status <profile>
   multigravity gh status <profile>
   ```
2. Isolate or share a specific component:
   ```bash
   # Isolate GitHub credentials for a client profile
   multigravity gh isolate <profile>

   # Re-share global skills with a profile
   multigravity skills share <profile>

   # Seed default read-only permissions (git, posix, npm, pnpm, uv) in config.json
   multigravity config seed <profile|--all|--host>
   ```
3. Verify presence or absence of sentinel files:
   - `.isolated_mcp`: MCP servers isolated
   - `.isolated_skills`: Global skills and plugins isolated
   - `.isolated_config`: `config.json` and AI permissions isolated (seeded with default read-only commands)
   - `.isolated_gh`: GitHub CLI credentials (`~/.config/gh`) isolated
   - `.isolated_dotfiles`: Host `.gitconfig` and `.ssh` isolated
   - `.auth_only` / `.shared`: Auth-only profile sharing host extensions and editor settings (~2 MB)

### Step 4: AI Conversation History & Brain Syncing
1. List available conversations in a profile:
   ```bash
   multigravity ai list <profile>
   ```
2. Export conversations (token-sanitized archive):
   ```bash
   multigravity ai export <profile> /path/to/archive.tar.gz
   ```
3. Synchronize conversation brains directly between profiles without copying credentials:
   ```bash
   multigravity ai sync <src_profile> <dest_profile>
   ```

### Step 5: Profile Maintenance & Lifecycle
1. Safely stop or restart a profile:
   ```bash
   multigravity stop <profile>
   multigravity restart <profile>
   ```
2. Reclaim disk space by clearing caches:
   ```bash
   multigravity clean <profile>
   ```

### Step 6: Machine-Readable Telemetry & Local HTTP API (Aggregator / UI)
1. Query machine-readable JSON contracts from CLI:
   ```bash
   # Profiles inventory and running states
   multigravity list --json

   # Quota telemetry and window reset timers
   multigravity quota [profile] --json

   # Storage stats and extension counts
   multigravity stats --json

   # Environment health and diagnostic checks
   multigravity doctor --json

   # AI conversations inventory
   multigravity ai list <profile> --json

   # Component sharing status
   multigravity mcp status <profile> --json
   ```
2. Start the local HTTP REST API server for background aggregators or UIs:
   ```bash
   # Bind to default 127.0.0.1:8989
   multigravity serve

   # Custom port or interface
   multigravity serve --port 9090 --host 127.0.0.1
   ```
3. Available Local API Endpoints:
   | Method | Endpoint | Description |
   | :--- | :--- | :--- |
   | `GET` | `/health` / `/api/v1/health` | Health check, version, and server uptime |
   | `GET` | `/api/v1/doctor` | Comprehensive system diagnostic report |
   | `GET` | `/api/v1/profiles` | List of all profiles with running state, color, and PIDs |
   | `GET` | `/api/v1/profiles/{name}` | Detailed information for a single profile |
   | `GET` | `/api/v1/profiles/{name}/stats` | Storage size and extension count for a single profile |
   | `GET` | `/api/v1/stats` | Aggregated storage usage across all profiles |
   | `GET` | `/api/v1/profiles/{name}/sharing` | MCP, skills, config, and GitHub CLI sharing status |
   | `GET` | `/api/v1/profiles/{name}/conversations` | AI conversation list and artifact counts |
   | `GET` | `/api/v1/quota` | Active Language Server quota metrics across all running profiles |
   | `GET` | `/api/v1/quota/{profile}` | Active quota metrics filtered by profile |
   | `POST` | `/api/v1/profiles/{name}/stop` | Gracefully stop profile processes (`{"force": false}`) |
   | `POST` | `/api/v1/profiles/{name}/clean` | Clean volatile caches for a profile |
   | `GET` | `/events` / `/api/v1/events` | Real-time Server-Sent Events (SSE) stream (`init`, `profiles`, `action`, `ping`) |

## 5. Canonical Examples

### Real-Time Streaming and Ingesting in Aggregators or Scripts
```bash
# Listen to real-time events push (SSE) without polling
curl -N -s http://127.0.0.1:8989/api/v1/events

# Fetch running profiles with PIDs
curl -s http://127.0.0.1:8989/api/v1/profiles | jq '.data[] | select(.is_running == true)'

# Inspect live quota fractions via CLI or HTTP
curl -s http://127.0.0.1:8989/api/v1/quota | jq '.data[].buckets'
```

### Checking Quota Before Dispatching Large Agentic Tasks
```bash
# Check quota status to ensure models (gemini, claude, gpt) are not exhausted
multigravity quota

# If reset occurred, verify if auto-prime is needed
multigravity prime --check
```

### Creating an Isolated Client Profile
```bash
# Create client-acme profile with isolated GitHub CLI and isolated config grants
multigravity new client-acme \
  --isolated-gh \
  --isolated-config \
  --color "#0d9488"

# Verify isolation status
multigravity gh status client-acme
multigravity config status client-acme
```

### Freeing Disk Space Across Inactive Profiles
```bash
# Verify which profiles are not running before cleaning
multigravity status

# Clean caches for a specific profile
multigravity clean old-experiment
```

## 6. Contrast Pairs

```bash
# BAD: Blindly deleting or modifying a profile while processes are still running
rm -rf ~/AntigravityProfiles/work
multigravity delete work --force

# GOOD: Checking profile status and stopping gracefully before deletion
multigravity status
multigravity stop work
multigravity delete work
```

```bash
# BAD: Hardcoding paths to host ~/.gitconfig inside an isolated profile script
cat ~/.gitconfig  # Can fail or read wrong identity if profile has isolated dotfiles

# GOOD: Respecting the profile's isolation contract
git config --list --show-origin
multigravity gh status <profile>
```

```bash
# BAD: Forcing prime when weekly quota balance is depleted (< 5%)
multigravity prime --force  # Wastes API calls and may trigger provider rate limits

# GOOD: Checking quota balance and reset eligibility first
multigravity prime --check --5h
```

## 7. Verification Checklist

- [ ] Target profile name verified against `^[a-zA-Z0-9][a-zA-Z0-9-]*$`.
- [ ] Running state checked via `multigravity status` before any destructive or stop action.
- [ ] Isolation sentinel files checked or updated correctly (`.isolated_mcp`, `.isolated_skills`, `.isolated_config`, `.isolated_gh`).
- [ ] Quota checks distinguish between the 4 buckets (`gemini-weekly`, `gemini-5h`, `3p-weekly`, `3p-5h`).
- [ ] AI exports and synchronizations asserted to be token-sanitized.
- [ ] Zero unverified changes to host `$REAL_HOME` dotfiles or credentials.
