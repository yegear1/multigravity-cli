# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **Autonomous Task Dispatcher (`internal/dispatch`):** Added `multigravity dispatch` (alias `dp`) to coordinate agent executions with profile isolation, ephemeral Git worktrees, structured diff parsing, and real-time log capture (`run`, `list`, `status`, `logs`, `diff`, `dashboard`, `cancel`, `delete`, `prune`).
- **Interactive Web Diff Visualizer & Task Dashboard:** Embedded HTML5 web UI (`/ui/tasks` and `/ui/tasks/:id/diff`) packaged directly into the Go binary (`//go:embed`) providing unified diff view, additions/deletions statistics, file tree navigation, and live SSE event updates.
- **PTY Terminal Multiplexer (`internal/agent`):** Added `multigravity agent` (alias `ag`) with real pseudo-terminal support (`creack/pty`) for interactive CLI tools (Claude Code, Aider, OpenCode), raw mode terminal attachment, and dedicated environment construction.
- **Ephemeral Git Worktrees (`internal/worktree`):** Added `multigravity worktree` (alias `wt`) to provision isolated branches and working trees per task in `.multigravity/worktrees/`, automatically excluded from host Git tracking via `.git/info/exclude`.
- **OpenAI & Anthropic Compatible AI Gateway (`internal/gateway`):** Added local `/v1/chat/completions` and `/v1/messages` HTTP streaming endpoints in `multigravity serve`, translating requests to CloudCode upstream streaming with support for Gemini and 3P Claude/GPT aliases.
- **Multi-Account Router with Auto-Failover:** Built-in multi-profile load balancing with `smart`, `round-robin`, `priority`, and `sticky` strategies, automatically placing profiles in cooldown and failing over upon HTTP 429 or 403 quota exhaustion without dropping client streams.
- **Workspace & Active Repository Detection (`internal/workspace`):** Added `multigravity workspace` (alias `ws`) to map project directories, active branches, and identify which profile owns a directory (`list`, `active`, `current`, `show`).
- **Proactive Quota Heuristic & 5-Hour Renewal:** Added mathematical classification between sliding 5-hour and weekly quota windows (`Window5h` vs `WindowWeekly`) and `--warm-5h` flag in `multigravity prime` to trigger early rolling renewal.
- **Auth-Only Lean Profiles (`--auth-only` / `--shared`):** Support for lightweight ~2 MB profiles that symlink host extensions and editor settings while keeping account credentials strictly isolated.
- **REST API Lifecycle & Sharing Mutations:** Added full profile mutation endpoints (`POST /api/v1/profiles`, `DELETE`, `launch`, `stop`, `restart`, `rename`) and dynamic resource sharing toggles (`/api/v1/profiles/:name/sharing/:resource`) with real-time SSE broadcasts.
- **Embedded Desktop Icons:** Packaged `.ico`, `.icns`, and `.png` icons into the binary via `//go:embed` for zero-configuration desktop shortcut generation.

## [2.0.0] - 2026-09-24

### Added
- **Native Go Architecture:** Completely re-engineered multigravity in Go using the Cobra CLI framework, delivering near-instant execution speed and self-contained static binaries across Linux, macOS, and Windows.
- **Zero Runtime Dependencies:** Eliminated external runtime dependencies (including Python 3) for permission injection, JSON manipulation, and quota telemetry.
- **Interactive TUI Menu:** Terminal-friendly menu when launched without arguments, featuring live profile status indicators (`● running` / `○ idle`), color preview, quick numeric launcher, and graceful non-interactive fallback.
- **Environment Diagnostics (`doctor`):** Added `multigravity doctor` to run comprehensive pre-flight health checks covering IDE executable detection, permissions, PATH integrity, and profile directory health.
- **Shell Autocompletion:** Added dynamic completions for Bash, Zsh, Fish, and PowerShell (`multigravity completion`) with contextual profile discovery.
- **Cross-Platform Desktop Integration:** Native desktop launchers generated automatically for Linux (`.desktop` with POSIX wrapper), macOS (`.app` bundles), and Windows (`.lnk` shortcuts).
- **Graceful Lifecycle Management:** Robust `stop` and `restart` commands with SQLite flush protection, process timeouts, and concurrency locks preventing accidental operations while profiles are active.
- **AI Chat & Memory Operations:** Granular management (`ai list`, `ai export`, `ai import`, `ai sync`) with non-destructive merge, Zip Slip defense, and zero token leakage.
- **AI Quota & Multi-Bucket Priming:** Telemetry (`quota`) and background priming engine (`prime`) with multi-bucket support (`gemini-weekly`, `gemini-5h`, `3p-weekly`, `3p-5h`), external prompt catalog, and schedulers for cron, systemd user timers, and Windows Task Scheduler.
- **Smart Entrypoints & Legacy Fallback:** Root launcher scripts with 3-tier execution (compiled binary -> auto-compile -> legacy scripts in `legacy/`).
- **Modernized Installation:** Updated `install.sh` and `install.ps1` with automated architecture detection (`amd64`, `arm64`) and direct GitHub release binary distribution.

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
