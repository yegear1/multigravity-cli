![Multigravity](assets/multigravity-logo.jpg)

# Multigravity

**The Agentic Development Platform & Multi-Account AI Gateway for Google Antigravity (and `agy`).**

Orchestrate autonomous coding agents, pool and failover AI quotas automatically across Google accounts, manage isolated IDE profiles, and execute tasks in ephemeral Git worktrees.

**English** | [Português](README.pt-br.md)

[![GitHub repository](https://img.shields.io/badge/GitHub-Repository-blue?logo=github)](https://github.com/yegear1/multigravity-cli)
[![GitHub profile](https://img.shields.io/badge/GitHub-Profile-lightgrey?logo=github)](https://github.com/yegear1)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey)](#install)
[![Showcase & Interactive Onboarding](https://img.shields.io/badge/Showcase-Live%20Demo-6366f1?logo=googlechrome&logoColor=white)](https://yegear1.github.io/multigravity-cli/)

> 🌐 **Interactive Showcase & Onboarding:** Check out our visual landing page with a live interactive terminal simulator, OS step-by-step guides, and disk footprint calculator at **[yegear1.github.io/multigravity-cli](https://yegear1.github.io/multigravity-cli/)**.

---

## Install

**macOS / Linux**

```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/yegear1/multigravity-cli/main/install.sh)"
```

**Windows** — open PowerShell and run:

```powershell
irm https://raw.githubusercontent.com/yegear1/multigravity-cli/main/install.ps1 | iex
```

---

## Quick Start

```bash
# Launch interactive menu (TUI) to inspect, select, or create profiles
multigravity

# Create profiles with custom visual themes
multigravity new work --color blue
multigravity new personal --color emerald

# Launch a profile
multigravity work

# Pass arguments straight through to Antigravity / agy
multigravity work .
multigravity work path/to/project
multigravity work --new-window
```

Each profile gets an automatic clickable desktop launcher:

| Platform | Location |
|----------|----------|
| macOS    | `~/Applications/Multigravity <name>.app` |
| Windows  | Start Menu → Programs |
| Linux    | `~/.local/share/applications/multigravity-<name>.desktop` |

---

## Key Features

- **Multi-Account AI Gateway:** Local OpenAI-compatible (`/v1/chat/completions`) and Anthropic-compatible (`/v1/messages`) endpoints with transparent auto-failover (HTTP 429/403) and smart multi-account load balancing.
- **Autonomous Agent Orchestrator:** Dispatch coding agents (Claude Code, Aider, OpenCode) with interactive PTY terminal multiplexing, profile environment isolation, and live log streaming (`multigravity dispatch`).
- **Ephemeral Git Worktrees:** Provision isolated branches and working directories per agent task in `.multigravity/worktrees/`, automatically excluded from host Git tracking (`multigravity worktree`).
- **Embedded Web Diff Visualizer:** Built-in web dashboard (`/ui/tasks`) and side-by-side/unified diff viewer (`/ui/tasks/:id/diff`) packaged directly into the Go binary with zero external dependencies.
- **Workspace & Repository Intelligence:** Auto-detects mapped projects, active Git branches, and running profile associations (`multigravity workspace`).
- **Interactive TUI:** Run `multigravity` without arguments in an interactive terminal for a quick-select menu with live status indicators.
- **Antigravity 2.0 (`agy`) Ready:** Seamlessly detects both `antigravity` and `agy` binaries across Linux, macOS, and Windows.
- **Auth-Only Lean Profiles:** Create profiles in seconds with shared host extensions and settings for just ~2 MB disk footprint (`--auth-only` / `--shared`).
- **Visual Window Theming:** Assign distinctive window/workbench colors per profile (`--color` or `multigravity color`) so you never confuse work and personal windows.
- **Dev Dotfiles Symlinked:** Host `.gitconfig` and `~/.ssh` keys are linked into isolated profiles by default, ensuring Git commits and SSH authentication work out of the box (with `--isolated-dotfiles` opt-out).
- **Graceful Lifecycle Management:** `stop` and `restart` profiles cleanly, with concurrency locks preventing accidental deletion or renaming while a profile is running.
- **Granular AI Session Migration:** `multigravity ai export` / `import` allows moving AI conversation databases and brain artifacts between profiles or machines with zero exposure of OAuth tokens or credentials.

---

## Commands

### Profile Management

| Command | Description |
|---------|-------------|
| `multigravity` | Open interactive profile selector menu (TUI) |
| `multigravity new <name>` | Create a new full profile |
| `multigravity new <name> --auth-only` | Create an auth-only profile (~2 MB: shared host extensions & settings, isolated accounts; alias: `--shared`) |
| `multigravity new <name> --from <template>` | Create a profile from a saved template |
| `multigravity new <name> --color <color>` | Create a profile with a custom window color theme |
| `multigravity new <name> --isolated-dotfiles` | Do not link host `.gitconfig` or `.ssh` into profile |
| `multigravity new <name> --isolated-mcp` | Do not share host Model Context Protocol (MCP) servers |
| `multigravity new <name> --isolated-skills` | Do not share host global skills and plugins |
| `multigravity new <name> --isolated-config` | Do not share host config.json and AI permission grants |
| `multigravity new <name> --isolated-gh` | Do not share host GitHub CLI credentials |
| `multigravity <name> [args...]` | Launch a profile (passes arguments to the IDE) |
| `multigravity stop <name> [--force]` | Gracefully stop a running profile (or force kill) |
| `multigravity restart <name>` | Restart a running profile |
| `multigravity color <name> [color]` | Set, view, or remove profile window color theme |
| `multigravity clean <name\|--all>` | Delete volatile Electron/Chromium caches to free disk space |
| `multigravity list [--json]` | List all profiles (or JSON format) |
| `multigravity status [name] [--json]` | Show running state, type, last used, and size per profile (or JSON format) |
| `multigravity clone <src> <dest>` | Copy an existing profile |
| `multigravity rename <old> <new>` | Rename a profile (blocked if currently running) |
| `multigravity delete <name>` | Delete a profile and all its data (blocked if currently running) |
| `multigravity login <profile>` | Sign in with Google OAuth2 PKCE; the credential stays in that profile's vault |
| `multigravity login status <profile> [--json]` | Show whether the profile vault holds a credential, without printing tokens |
| `multigravity login logout <profile>` | Delete the credential stored in the profile vault |

### AI Sessions & Chats

| Command | Description |
|---------|-------------|
| `multigravity ai list <name>` | List AI conversation titles and artifact counts in a profile |
| `multigravity ai export <name> [path]` | Export AI chats and brain data (sanitized of OAuth tokens & keys) |
| `multigravity ai import <archive> <name>` | Import AI chats into an existing profile non-destructively |
| `multigravity ai sync <src> <dest>` | Synchronize AI conversations directly between two local profiles |
| `multigravity mcp status <name>` | Check MCP server configuration sharing status |
| `multigravity mcp share <name>` | Share host MCP servers (`~/.gemini/config/mcp_config.json`) |
| `multigravity mcp isolate <name>` | Isolate profile with a private copy of MCP configuration |
| `multigravity skills status <name>` | Check skills & plugins configuration sharing status |
| `multigravity skills share <name>` | Share host skills & plugins (`~/.gemini/config/skills`, `plugins`) |
| `multigravity skills isolate <name>` | Isolate profile with a private copy of skills & plugins |
| `multigravity config status <name>` | Check config.json and permission grants sharing status |
| `multigravity config share <name>` | Share host config.json and permissions (`~/.gemini/config/config.json`) |
| `multigravity config isolate <name>` | Isolate profile with a private copy of config.json |
| `multigravity config seed [name\|--all\|--host]` | Seed default read-only permissions (git, posix, npm, pnpm, uv) in config.json |
| `multigravity gh status <name>` | Check GitHub CLI credentials sharing status |
| `multigravity gh share <name>` | Share host GitHub CLI credentials (`~/.config/gh` or `%APPDATA%\GitHub CLI`) |
| `multigravity gh isolate <name>` | Isolate profile with a private copy of GitHub CLI credentials |
| `multigravity quota [name]` | Show live AI token limits, usage percentage, and countdown until reset |
| `multigravity alerts [name]` | Report critical quota, recent quota drops, and reaped headless processes (`--json`) |
| `multigravity quota history [name]` | Show the stored quota and token time series (`--since 24h`, `--source`, `--json`) |
| `multigravity ai quota [name]` | Alias for `multigravity quota` |
| `multigravity prime [name] [opt]` | Automatically prime weekly token cycles upon reset (dual bucket, jitter, cron/systemd) |
| `multigravity ai prime [name] [opt]` | Alias for `multigravity prime` |

### Autonomous Agents & Task Dispatch

| Command | Description |
|---------|-------------|
| `multigravity dispatch run <cmd> [flags]` | Dispatch an autonomous agent task with profile isolation and optional worktree (alias: `dp run`) |
| `multigravity dispatch list [--json]` | List dispatched tasks with status, duration, and metadata (alias: `dp list`) |
| `multigravity dispatch status <task-id> [--json]` | Show detailed execution state and manifest of a task |
| `multigravity dispatch logs <task-id> [-f\|--tail N]` | Stream or inspect real-time execution logs |
| `multigravity dispatch diff <task-id> [--web\|--structured\|--json]` | Inspect task Git diff in terminal, structured table, JSON, or open web visualizer |
| `multigravity dispatch dashboard [--json]` | Executive summary dashboard of running, completed, and failed tasks |
| `multigravity dispatch cancel <task-id> [--force]` | Cancel a running task gracefully (or force kill) |
| `multigravity dispatch delete <task-id> [--worktree]` | Delete task record and optionally clean its worktree |
| `multigravity dispatch prune [--max-age <dur>]` | Prune finished tasks older than retention threshold |
| `multigravity agent run <profile> [--] <cmd>` | Run an interactive CLI agent (Claude Code, Aider, OpenCode) in a dedicated PTY (alias: `ag run`) |
| `multigravity agent list [--json]` | List active PTY agent sessions |
| `multigravity agent attach <id>` | Attach terminal directly to a running PTY agent session |
| `multigravity agent stop <id>` | Stop a PTY agent session gracefully |
| `multigravity exec [profile\|--all] "<prompt>" [--json]` | Fan out one prompt across a profile or every profile via the existing headless runner, with a worker pool and an aggregated JSON report |

### Ephemeral Git Worktrees

| Command | Description |
|---------|-------------|
| `multigravity worktree list [--repo <path>] [--json]` | List active git worktrees tracked by Multigravity (alias: `wt list`) |
| `multigravity worktree create <name> [--branch <b>] [--json]` | Create an ephemeral worktree in `.multigravity/worktrees/` |
| `multigravity worktree status <name> [--json]` | Check git status, modified and untracked files inside worktree |
| `multigravity worktree diff <name> [--stat] [--json]` | Show diff generated within worktree compared to base branch |
| `multigravity worktree remove <name> [--force]` | Clean up and remove an ephemeral worktree |
| `multigravity worktree prune` | Prune stale or orphaned worktree records |

### Workspaces & Repositories

| Command | Description |
|---------|-------------|
| `multigravity workspace list [--active] [--json]` | List mapped workspaces and Git status across profiles (alias: `ws list`) |
| `multigravity workspace active [--json]` | List workspaces currently open in running IDE instances |
| `multigravity workspace current [--json]` | Detect which profile and workspace own the current working directory (alias: `ws here`) |
| `multigravity workspace show <profile> <ws> [--json]` | Show detailed repository status, active branch, and policies |

### Templates

| Command | Description |
|---------|-------------|
| `multigravity template save <profile> <name>` | Save a profile as a reusable template |
| `multigravity template list` | List saved templates |
| `multigravity template delete <name>` | Remove a template |

### Backup & Transfer

| Command | Description |
|---------|-------------|
| `multigravity export <name> [path] [--include-cache]` | Archive a profile to `.tar.gz` (`.zip` on Windows), lean by default |
| `multigravity import <archive> [name]` | Restore a profile from an archive |

### Server & AI Gateway

| Command | Description |
|---------|-------------|
| `multigravity serve [--port <p>] [--host <h>]` | Start local HTTP daemon with REST API, SSE streaming, AI Gateway (`/v1/chat/completions`, `/v1/messages`), and Web UI (`/ui/tasks`) |
| `multigravity stats [--json]` | Show disk usage per profile |
| `multigravity doctor [--json]` | Diagnose environment setup, paths, and binary detection |
| `multigravity update` | Update Multigravity to the latest version |
| `multigravity completion` | Set up shell tab-completion |
| `multigravity version` | Show multigravity version |
| `multigravity help` | Show help message |

---

## Visual Window Theming

Differentiate your workspaces instantly with custom window accent colors:

```bash
# Supported color names: blue, green, emerald, red, purple, orange, cyan, pink, indigo, slate, etc.
multigravity color work blue
multigravity color personal emerald

# Or use any custom hex code
multigravity color client-x "#8b5cf6"

# Remove color customizations
multigravity color work --reset
```

---

## AI Conversation Migration & Sync

Migrate or sync Gemini/Antigravity chat history and brain knowledge between profiles securely:

```bash
# List conversations in a profile
multigravity ai list work

# Export AI chats (automatically strips sensitive OAuth tokens and credentials)
multigravity ai export work ./work-chats.tar.gz

# Import into another profile without overwriting existing conversations
multigravity ai import ./work-chats.tar.gz personal

# Direct profile-to-profile sync without creating intermediate archive files
multigravity ai sync work personal
```

---

## MCP Server Sharing (Model Context Protocol)

By default, all profiles link to host Model Context Protocol configurations (`~/.gemini/config/mcp_config.json` and cached schemas `~/.gemini/antigravity/mcp`), granting immediate access to local MCP servers across all profiles without manual setup:

```bash
# Check MCP sharing status for a profile
multigravity mcp status work

# Create a profile isolated from host MCP servers
multigravity new client-x --isolated-mcp

# Switch an existing profile between shared and isolated modes
multigravity mcp isolate work
multigravity mcp share work
```

---

## Skills & Plugins Sharing

By default, all profiles link to the host system's global custom skills (`~/.gemini/config/skills`) and plugins (`~/.gemini/config/plugins`), giving all profiles immediate access to custom workflows and plugin capabilities:

```bash
# Check skills & plugins sharing status for a profile
multigravity skills status work

# Create a profile with isolated skills & plugins
multigravity new client-x --isolated-skills

# Switch an existing profile between shared and isolated modes
multigravity skills isolate work
multigravity skills share work
```

---

## Config & Permissions Sharing (`config.json`)

By default, all profiles link to the host system's configuration (`~/.gemini/config/config.json`), sharing global permission grants and UI preferences in real-time across windows without duplicating approvals.

In addition, multigravity **automatically seeds default read-only permissions** in `"Always allow"` (`.userSettings.globalPermissionGrants.allow`) for both sandboxed (`command(...)`) and unsandboxed (`unsandboxed(...)`) execution. This includes:
- **Git:** `git status`, `git log`, `git diff`, `git show`, `git branch`, `git tag`, `git remote`, etc.
- **POSIX & System:** `ls`, `cat`, `head`, `tail`, `grep`, `rg`, `find`, `which`, `stat`, `df`, `ps`, etc.
- **Dev Tooling & Linters:** `npm test`, `npm run lint/check`, `pnpm test/lint`, `uv run pytest/ruff/pyright/mypy`, `ruff check`, `eslint`, `tsc --noEmit`, etc.

```bash
# Check config & permissions sharing status for a profile
multigravity config status work

# Seed or refresh default read-only permissions across all profiles
multigravity config seed --all

# Create a profile with isolated config and permissions (seeded automatically)
multigravity new client-x --isolated-config

# Switch an existing profile between shared and isolated modes
multigravity config isolate work
multigravity config share work
```

---

## AI Token Limits & Quota Telemetry (`multigravity quota`)

Check live AI token consumption, usage percentage, remaining fraction, and exact countdown until limits reset across running Antigravity profiles:

```bash
# Check quota for all active profiles
multigravity quota

# Check quota for a specific profile
multigravity quota work
multigravity ai quota work

# Time series of remaining quota and tokens (gateway, headless, and live snapshots)
multigravity quota history work --since 24h --json

# Critical quota (<= 5% remaining), a drop of at least 5 points since the previous snapshot, and reaped headless processes
multigravity alerts
multigravity alerts work --json
```

`alerts` reads the quota history. The 5% line is the same guard used before a 5-hour warm-up. A headless state file whose process has already exited is removed by the existing headless reap and reported once as an orphan.

---

## Multi-Account AI Gateway & Auto-Failover

Multigravity includes a local AI Gateway in `multigravity serve` that exposes OpenAI-compatible (`/v1/chat/completions`) and Anthropic-compatible (`/v1/messages`) endpoints, backed by your isolated Antigravity/Google accounts.

### Key Capabilities
- **Multi-Account Pooling & Auto-Failover:** If an active profile hits rate limits (HTTP 429 or 403 quota exhaustion), the gateway automatically places it in cooldown and seamlessly fails over to the next healthy profile without dropping the client stream.
- **Pluggable Routing Strategies:**
  - `smart` (default): Prioritizes profiles with higher remaining quota fractions, penalizes error rates, and balances load dynamically.
  - `round-robin`: Rotates requests sequentially through healthy profiles.
  - `priority`: Uses declared profile priority order.
  - `sticky`: Keeps requests on the current profile until a rate limit occurs.
- **Model Aliases:** Supports native Gemini models (`gemini-2.5-pro`, `gemini-2.5-flash`, `gemini-3.5-flash`) and transparent 3P aliases (`claude-3-7-sonnet`, `claude-3-5-sonnet`, `claude-opus`, `gpt-4o`).
- **Zero Credential Custody:** Requests use the local Antigravity Language Server tokens; no raw Google passwords or secrets are ever persisted.

### Usage Examples

```bash
# 1. Start the Multigravity Gateway daemon
multigravity serve

# 2. Query via OpenAI Chat Completions endpoint (streaming supported)
curl -s -N http://127.0.0.1:8989/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemini-2.5-pro",
    "messages": [{"role": "user", "content": "Explain Git worktrees in one sentence."}],
    "stream": true
  }'

# 3. Query via Anthropic Messages endpoint
curl -s http://127.0.0.1:8989/v1/messages \
  -H "Content-Type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-3-7-sonnet",
    "messages": [{"role": "user", "content": "Hello Claude!"}],
    "max_tokens": 100
  }'

# 4. Point Claude Code CLI to Multigravity Gateway
export ANTHROPIC_BASE_URL="http://127.0.0.1:8989"
claude

# 5. Point Aider / OpenCode / Continue to Multigravity Gateway
export OPENAI_BASE_URL="http://127.0.0.1:8989/v1"
aider --model gpt-4o
```

---

## Autonomous Agent Orchestration & Task Dispatcher

Multigravity can coordinate autonomous coding agents (Claude Code, Aider, OpenCode, Agy) in background tasks or interactive pseudo-terminals (PTY), isolated in dedicated profiles and ephemeral Git worktrees.

```
┌─────────────────────────────────────────────────────────────┐
│                    multigravity dispatch                    │
│                                                             │
│   ┌────────────────┐   ┌────────────────┐   ┌───────────┐   │
│   │ Profile Auth   │   │ Ephemeral Git  │   │ PTY TTY   │   │
│   │ & Quota Pool   │ + │ Worktree       │ + │ Terminal  │   │
│   │ (~/Antigravity)│   │ (.multigravity)│   │ Multiplex │   │
│   └────────────────┘   └────────────────┘   └───────────┘   │
└──────────────────────────────┬──────────────────────────────┘
                               │
            ┌──────────────────┴──────────────────┐
            ▼                                     ▼
   Terminal Stream & Logs                Web Diff Visualizer
  (multigravity dispatch logs)         (http://localhost:8989/ui/tasks)
```

### Dispatching Tasks with Ephemeral Git Worktrees

```bash
# Dispatch a task to run in a fresh ephemeral Git worktree
multigravity dispatch run --profile work --worktree --prompt "Refactor user authentication in auth.go"

# Stream task execution logs in real time
multigravity dispatch logs <task-id> -f

# Review the structured Git diff generated by the agent
multigravity dispatch diff <task-id> --structured

# Open the interactive visual web diff viewer in your browser
multigravity dispatch diff <task-id> --web
# Or visit the embedded dashboard directly: http://127.0.0.1:8989/ui/tasks
```

### Interactive PTY Sessions

For agent CLI tools that require interactive terminal prompts (`[y/n]`, tool approvals, ANSI styling):

```bash
# Run Claude Code or Aider inside an isolated PTY session
multigravity agent run work -- claude

# List and attach to active agent PTY sessions
multigravity agent list
multigravity agent attach <session-id>
```

---

## Workspace & Active Repository Intelligence

Track which projects belong to which Antigravity profile, detect open workspaces, and find your profile context without leaving the terminal:

```bash
# Show which profile and workspace own the current directory
multigravity workspace current

# List all mapped workspaces across profiles with Git status (branch, clean/dirty)
multigravity workspace list

# List only workspaces open in currently running IDE instances
multigravity workspace active

# Show details of a specific workspace
multigravity workspace show work backend-api
```

---

## Auth-Only Profiles (`--auth-only` / `--shared`)

Full profiles are completely isolated environments — separate extensions, settings, caches, and accounts. That's the default.

**Auth-Only profiles** take a lightweight approach: they symlink `extensions/` and core editor configurations (`settings.json`, `keybindings.json`, `snippets`) directly from your host Antigravity installation, isolating **strictly the account/auth layer and AI session state**. This yields a footprint of **~2 MB** per profile instead of the typical **~500 MB+** of a full installation, while still providing 100% independent Google login sessions and token quotas.

### Full vs. Auth-Only Comparison Matrix

| Feature / Aspect | Full Profile (Default) | Auth-Only Profile (`--auth-only` / `--shared`) |
| :--- | :--- | :--- |
| **Command** | `multigravity new <name>` | `multigravity new <name> --auth-only` *(or `--shared`)* |
| **Initial Disk Footprint** | **~500 MB** *(grows with duplicate extensions & caches)* | **~2 MB** *(lean, near-zero disk usage)* |
| **Antigravity / Gemini Accounts** | **Isolated** *(independent OAuth tokens & Google logins)* | **Isolated** *(independent OAuth tokens & Google logins)* |
| **AI Quotas & Token Limits** | **Independent** *(tracked separately per Google account)* | **Independent** *(tracked separately per Google account)* |
| **AI History & Brain Chats** | **Isolated** *(manipulated via `ai export/import/sync`)* | **Isolated** *(manipulated via `ai export/import/sync`)* |
| **IDE Extensions** | **Isolated** *(requires installing extensions per profile)* | **Shared via symlink** *(instantly mirrors host extensions)* |
| **Editor Settings & Keybindings** | **Isolated** *(`User/settings.json`, keybindings, snippets)* | **Shared via symlink** *(mirrors host preferences)* |
| **Window Theming (`--color`)** | **Supported** *(custom title bar & accent color)* | **Supported** *(safely uncouples `settings.json` non-destructively)* |
| **Dev Dotfiles (`.gitconfig`, `.ssh`)** | **Symlinked by default** *(opt-out with `--isolated-dotfiles`)* | **Symlinked by default** *(opt-out with `--isolated-dotfiles`)* |
| **GitHub CLI (`gh`) & Config** | **Symlinked by default** *(opt-out with `--isolated-gh`)* | **Symlinked by default** *(opt-out with `--isolated-gh`)* |
| **Best For** | Divergent stacks (Work vs Personal, different linters/themes) | Quota rotation across Google accounts, secondary logins with shared toolchains |

### Which Profile Type Should I Choose?

- **Choose Full Profile** if:
  - You want completely different sets of VS Code / Antigravity extensions for different projects (e.g. Go backend vs Flutter mobile vs Python ML).
  - You need custom editor keybindings or settings that shouldn't affect your daily driver.
  - You want strict separation between corporate and personal environments.

- **Choose Auth-Only Profile** if:
  - You primarily need **multiple Google accounts** to cycle or expand your weekly and 5-hour Gemini AI quotas.
  - You want to keep your familiar themes, extensions, snippets, and keybindings without reinstalling them.
  - You want fast profile creation with negligible disk footprint (~2 MB vs ~500 MB).

```bash
# Create an auth-only profile (alias: --shared)
multigravity new alt-account --auth-only

# Create an auth-only profile with a distinct color theme
multigravity new alt-account --auth-only --color purple
```

## Headless sign-in (`multigravity login`)

`multigravity login <profile>` signs a profile in with Google OAuth2 PKCE. It listens on an ephemeral `127.0.0.1` port, opens the browser (or prints the URL with `--no-browser`), and writes the refresh token only inside that profile:

- `.gemini/antigravity-cli/antigravity-oauth-token`
- `.gemini/jetski-standalone-oauth-token`

The host keyring is not modified. When that vault file exists, profile launches set `GEMINI_FORCE_FILE_STORAGE=true` so `agy` reads the profile file instead of the shared OS keyring. Email and display name are stored beside the vault in `account.json`, which contains no tokens. `login status --json` reports identity and expiry only.

```bash
multigravity login work
multigravity login status work --json
multigravity login logout work
```

---

## Parallel headless prompts (`multigravity exec`)

`multigravity exec` runs one prompt through the existing headless runner (`agy`, or the language-server cascade when `agy` is not on `PATH`). Each profile keeps its own `HOME` and credential vault. A worker pool caps how many runs are in flight. The command does not start an agent engine beside `dispatch` or the AI gateway.

```bash
multigravity exec work "Summarize the TODOs in this repository" --json
multigravity exec --all "Summarize the TODOs in this repository" --workers 4 --json
```

The JSON report aggregates `results`, `succeeded`, `failed`, and `total_tokens`. If any profile fails, the process exits non-zero after printing the report. The same contract is available as `POST /api/v1/exec` with `{"all": true, "prompt": "...", "workers": 4}`.

---

## Templates

Save a configured profile as a template, then spin up new profiles from it instantly:

```bash
# Save your ideal setup as a template
multigravity template save work base

# Create new profiles from it
multigravity new project-a --from base
multigravity new project-b --from base

# See what templates you have
multigravity template list
```

## AI Agent Skill Integration

Multigravity includes an official AI Agent Skill ([`skills/multigravity/SKILL.md`](skills/multigravity/SKILL.md)) following the `agent-skills` standard, enabling AI assistants (such as Antigravity and Cursor) to autonomously and safely inspect quotas, automate prime cycles, manage profile isolation boundaries, and synchronize AI chat brains.

To install or sync the skill into your local AI discovery directories (`~/.gemini/config/skills` and `~/.cursor/skills`):

```bash
./scripts/install-agent-skills.sh
```

---

## Shell Completion

Enable tab-completion for commands and profile names:

```bash
multigravity completion
```

Follow the instructions to add it to your `.zshrc`, `.bashrc`, or PowerShell `$PROFILE`.

---

## Uninstall

**macOS / Linux**

```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/yegear1/multigravity-cli/main/uninstall.sh)"
```

**Windows**

```powershell
irm https://raw.githubusercontent.com/yegear1/multigravity-cli/main/uninstall.ps1 | iex
```

You'll be asked whether to remove your profile data — nothing is deleted without confirmation.

---

## Profile Name Rules

Letters, numbers, and hyphens only. Must start with a letter or number.

```
✅  work   client-a   test1
❌  -name  my_profile
```

---

## License

The modifications, enhancements, and new features in this fork are licensed under the [MIT License](LICENSE).  
The original base codebase remains the copyright of Sujit Agarwal and original contributors under GitHub's Terms of Service.

---

## Credits & Acknowledgments

- **Original Author & Creator:** [Sujit Agarwal](https://github.com/sujitagarwal)
- **Windows Support:** [Samin Yeasar](https://github.com/Solez-ai)
- **Linux Support:** [Md Rayyan Nawaz](https://github.com/therayyanawaz)
- **Community Inspiration (Multigravity Pro):** [Pulkit](https://github.com/Pulkit7070) (pioneering `--auth-only` profile semantics and onboarding walkthroughs in [Pulkit7070/multigravity-pro](https://github.com/Pulkit7070/multigravity-pro))
- **Enhanced Fork & Maintainer:** [yegear1](https://github.com/yegear1)

