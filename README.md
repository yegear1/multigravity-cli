![Multigravity](assets/multigravity-logo.jpg)

# Multigravity

**Run multiple Antigravity (and `agy`) IDE profiles simultaneously — each with its own accounts, extensions, settings, and AI conversations.**

No more logging in and out. Launch as many profiles as you need, all at once.

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

- **Interactive TUI:** Run `multigravity` without arguments in an interactive terminal for a quick-select menu with live status indicators.
- **Antigravity 2.0 (`agy`) Ready:** Seamlessly detects both `antigravity` and `agy` binaries.
- **Visual Window Theming:** Assign distinctive window/workbench colors per profile (`--color` or `multigravity color`) so you never confuse work and personal windows.
- **Dev Dotfiles Symlinked:** Host `.gitconfig` and `~/.ssh` keys are linked into isolated profiles by default, ensuring Git commits and SSH authentication work out of the box (with `--isolated-dotfiles` opt-out).
- **Graceful Lifecycle Management:** `stop` and `restart` profiles cleanly, with concurrency locks preventing accidental deletion or renaming while a profile is running.
- **Cache Cleaning & Lean Backups:** `multigravity clean` reclaims gigabytes of volatile Electron/Chromium cache, and `export` strips caches automatically for fast, lightweight archives.
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
| `multigravity ai quota [name]` | Alias for `multigravity quota` |
| `multigravity prime [name] [opt]` | Automatically prime weekly token cycles upon reset (dual bucket, jitter, cron/systemd) |
| `multigravity ai prime [name] [opt]` | Alias for `multigravity prime` |

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

### Utilities

| Command | Description |
|---------|-------------|
| `multigravity serve [--port <p>] [--host <h>]` | Start local HTTP REST & SSE streaming server (`127.0.0.1:8989`) |
| `multigravity stats` | Show disk usage per profile |
| `multigravity doctor` | Diagnose environment setup, paths, and binary detection |
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

