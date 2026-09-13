# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.5.0] - 2026-09-13

### Added
- **AI Quota & Token Telemetry:** `multigravity quota [profile]` and `ai quota` commands to inspect live token limits, usage fractions, and reset countdowns across sliding windows (`gemini-5h`, `gemini-weekly`, `3p-5h`, `3p-weekly`).
- **AI Weekly Quota Dual-Bucket Prime Automation:** `multigravity prime [profile]` command and background schedulers (cron, user systemd timers on Linux/macOS, Windows Scheduled Tasks) with random anti-bot jitter, external `prompts.json` pool, and dual-bucket targeting (`gemini-weekly` and `3p-weekly` for Claude Sonnet / GPT) to auto-prime weekly cycles upon reset.
- **Global Assistant Permissions & Config Sharing:** Shared `~/.gemini/config/config.json` across profiles by default (`multigravity config <status|share|isolate> <profile>`) allowing tool grants approved with "Always allow" to persist in real time across all profiles, with `--isolated-config` opt-out.
- **GitHub CLI & Git Credentials Synchronization:** Automatic symlink/junction of GitHub CLI authentication (`~/.config/gh` or `%APPDATA%\GitHub CLI`) and Git HTTPS credentials (`~/.git-credentials`) across profiles (`multigravity gh <status|share|isolate> <profile>`), with `--isolated-gh` opt-out.
- **Host User PATH Preservation:** Preserved and enriched host user binary directories (`~/.local/bin`, `~/.cargo/bin`, etc.) into profile subprocesses at launch, allowing CLI tools to work immediately in the integrated terminal without duplicating binaries.

## [1.4.1] - 2026-09-13

### Added
- **Skills & Plugins Sharing:** Shared host global custom skills (`~/.gemini/config/skills`) and plugins (`~/.gemini/config/plugins`) across profiles by default without credential leakage.
- **Skills Lifecycle Management:** `multigravity skills (status|share|isolate) <profile>` commands and `--isolated-skills` creation flag for full isolation or opt-in sharing.
- **MCP Server Sharing (Model Context Protocol):** Shared host Model Context Protocol configuration (`~/.gemini/config/mcp_config.json`) and cached schemas (`~/.gemini/antigravity/mcp`) across profiles by default without credential leakage.
- **MCP Lifecycle Management:** `multigravity mcp (status|share|isolate) <profile>` commands and `--isolated-mcp` creation flag for full profile MCP isolation or opt-in sharing.
- **Direct AI Conversation Sync:** `multigravity ai sync <src> <dest>` command to sync chat histories, annotations, and brain artifacts directly between two local profiles without temporary archive files.
- **Shell Autocompletion:** Updated Bash, Zsh, and PowerShell tab-completions for `skills` subcommands, `mcp` subcommands, and `ai sync`.

## [1.4.0] - 2026-09-12

### Added
- **Interactive TUI:** Quick-select menu when running `multigravity` without arguments in an interactive terminal, displaying live status indicators (`● running` / `○ idle`).
- **Antigravity 2.0 (`agy`) Support:** Native binary detection for `agy` alongside `antigravity`, with expanded search across portable paths (`~/apps/antigravity/antigravity`, `/opt/`, and Scoop).
- **Profile Window Theming:** Distinctive window and workbench accent colors per profile via `--color` and `multigravity color <name> [color]`.
- **Graceful Lifecycle Controls:** `stop` and `restart` commands with SQLite flush safety and active concurrency locks against accidental deletion or renaming.
- **Cache Cleaning & Lean Backups:** `multigravity clean` command to free disk space from volatile Electron caches, plus automated cache exclusion during `export`.
- **Granular AI Session Migration & Direct Sync:** `multigravity ai (list, export, import, sync)` for non-destructive chat migration, direct profile-to-profile sync, sidebar index preservation, and strict OAuth token/credential sanitization.
- **Developer Dotfiles Integration:** Automatic linking of host `.gitconfig` and `~/.ssh` into isolated profiles (with `--isolated-dotfiles` opt-out).
- **CLI Version Command:** Dedicated `version`, `-v`, and `--version` flags with autocompletion support for Bash, Zsh, and PowerShell.
- **Documentation:** Complete Brazilian Portuguese documentation in `README.pt-br.md` and MIT licensing clarification for fork contributions.
