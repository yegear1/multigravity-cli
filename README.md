![Multigravity](assets/multigravity-logo.jpg)

# Multigravity

**Run multiple Antigravity (and `agy`) IDE profiles simultaneously — each with its own accounts, extensions, settings, and AI conversations.**

No more logging in and out. Launch as many profiles as you need, all at once.

**English** | [Português](README.pt-br.md)

[![GitHub repository](https://img.shields.io/badge/GitHub-Repository-blue?logo=github)](https://github.com/yegear1/multigravity-cli)
[![GitHub profile](https://img.shields.io/badge/GitHub-Profile-lightgrey?logo=github)](https://github.com/yegear1)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey)](#install)

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
| `multigravity new <name> --shared` | Create a lightweight profile (shared extensions & settings, isolated accounts) |
| `multigravity new <name> --from <template>` | Create a profile from a saved template |
| `multigravity new <name> --color <color>` | Create a profile with a custom window color theme |
| `multigravity new <name> --isolated-dotfiles` | Do not link host `.gitconfig` or `.ssh` into profile |
| `multigravity <name> [args...]` | Launch a profile (passes arguments to the IDE) |
| `multigravity stop <name> [--force]` | Gracefully stop a running profile (or force kill) |
| `multigravity restart <name>` | Restart a running profile |
| `multigravity color <name> [color]` | Set, view, or remove profile window color theme |
| `multigravity clean <name\|--all>` | Delete volatile Electron/Chromium caches to free disk space |
| `multigravity list` | List all profiles |
| `multigravity status` | Show running state, type, last used, and size per profile |
| `multigravity clone <src> <dest>` | Copy an existing profile |
| `multigravity rename <old> <new>` | Rename a profile (blocked if currently running) |
| `multigravity delete <name>` | Delete a profile and all its data (blocked if currently running) |

### AI Sessions & Chats

| Command | Description |
|---------|-------------|
| `multigravity ai list <name>` | List AI conversation titles and artifact counts in a profile |
| `multigravity ai export <name> [path]` | Export AI chats and brain data (sanitized of OAuth tokens & keys) |
| `multigravity ai import <archive> <name>` | Import AI chats into an existing profile non-destructively |

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
| `multigravity stats` | Show disk usage per profile |
| `multigravity doctor` | Diagnose environment setup, paths, and binary detection |
| `multigravity update` | Update Multigravity to the latest version |
| `multigravity completion` | Set up shell tab-completion |
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

## AI Conversation Migration

Migrate Gemini/Antigravity chat history and brain knowledge between profiles or across machines securely:

```bash
# List conversations in a profile
multigravity ai list work

# Export AI chats (automatically strips sensitive OAuth tokens and credentials)
multigravity ai export work ./work-chats.tar.gz

# Import into another profile without overwriting existing conversations
multigravity ai import ./work-chats.tar.gz personal
```

---

## Shared Profiles

Full profiles are fully isolated — separate extensions, settings, and accounts. That's the default.

**Shared profiles** go lighter: they symlink extensions and settings from your main Antigravity install, isolating only the account/auth layer. Useful when you need a second account but don't want to duplicate gigabytes of extensions.

```bash
multigravity new client-x --shared
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

This project is licensed under the [MIT License](LICENSE).

---

## Credits & Acknowledgments

- **Original Author & Creator:** [Sujit Agarwal](https://github.com/sujitagarwal)
- **Windows Support:** [Samin Yeasar](https://github.com/Solez-ai)
- **Linux Support:** [Md Rayyan Nawaz](https://github.com/therayyanawaz)
- **Enhanced Fork & Maintainer:** [yegear1](https://github.com/yegear1)
