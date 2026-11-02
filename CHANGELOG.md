# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [2.2.5] - 2026-11-02

### Added
- **Native Antigravity Subagent Orchestration Bridge:** Dispatches macro-routing plans using `agy` headless invocations or MCP delegation with automatic quota failover across secondary profiles.
- **Hybrid Workspace Modes for Zero-I/O Dispatch:** Added `worktree`, `mirror`, and `inherit` workspace execution modes to subtask plan requests. Read-only and analysis subtasks run directly in `inherit` mode to eliminate worktree checkout and filesystem overhead.
- **Dynamic Dispatch Roles & Read-Only Sandboxing:** Added specialized execution personas (`developer`, `researcher`, `tester`, `reviewer`) with read-only subtask guardrails (`ReadOnly: true`) that prevent unauthorized file mutations.
- **Cross-Profile Shared Cooldown & Throttling Pool:** Coordinated cross-profile cooldown tracking via `~/.multigravity/cooldown-pool.json` to prevent cascade 429 rate limit errors when distributing parallel tasks across accounts.
- **On-Demand Headless Agent Runner for Inactive Profiles:** Automatically initializes and spawns headless language server runners on inactive secondary profiles when assigned work in parallel wave dispatches.
- **Build Isolation & Compiler Cache Strategy for Concurrent Worktrees:** Isolates `CARGO_TARGET_DIR` per worktree and configures shared global compiler cache (`sccache`, `ccache`) across concurrent worktrees to avoid lock contention and rebuild overhead.
- **Cross-Profile Toolchain & PATH Discovery:** Discovers installed SDKs and toolchains across all Antigravity profiles and injects them into ephemeral execution paths.
- **MCP Auto-Routing & Synchronous Wait:** Added auto profile quota routing (`profile: "auto"`) and synchronous execution waiting (`wait: true`) to the MCP `dispatch_task` tool.

## [2.2.4] - 2026-11-02

### Added
- **On-Demand Secondary Profile Quota Probing (`--probe` / Wave Pre-Flight):** Added `--probe` flag to `multigravity dispatch wave` and background probing (`ProbeSecondaryProfiles`), triggering fast RPC telemetry calibration across secondary profiles to resolve idle unknown quota states prior to wave dispatch.
- **AST Outline Extraction for Subtask Pre-Injection:** Added AST outline parser (`ExtractASTOutline`) for Go and TypeScript/JavaScript files exceeding inline injection thresholds during subtask prompt generation, preserving structural type signatures and interfaces while significantly reducing context token consumption.
- **Compact Wave Execution Output Sanitization:** Sanitized compact JSON wave summaries (`ToCompact`) by suppressing empty diff payloads and truncating failed worker logs to the terminal 15 lines (omitted on success) to eliminate token waste in multi-agent pipelines.
- **Ephemeral Worktree Branch Pruning:** Automatically deletes ephemeral Git branches (`multigravity-plan-*`, `multigravity-task-*`) upon worktree apply (`ApplyWorktree`) and prune (`PruneWorktree`, `PruneAllWorktrees`), eliminating branch residue in host repositories.

### Changed
- **Componentized Server Handlers & Modular Dispatch CLI:** Modularized HTTP route handlers in `internal/server/` and split dispatch CLI subcommands in `internal/cmd/` into dedicated files without altering endpoint signatures or command line flags.

## [2.2.3] - 2026-11-01

### Added
- **Ephemeral Worktree Dependency Symlinking:** Automatically symlinks ignored build directories (`node_modules`, `target`, `.venv`, `.cargo`) from repository root into ephemeral Git worktrees, allowing fast-track zero-install verification in subtasks.
- **Run Artifact Persistence & Compact JSON:** Saves detailed execution run artifacts to `.multigravity/runs/<plan_id>.json` and defaults CLI plan output to compact machine-readable summaries, with `--full` flag for exhaustive diff/log payloads.
- **Verify-on-Timeout Recovery:** Automatically triggers and checks the subtask verification command (`Verify:`) on execution timeout when local changes exist, safely recovering tasks as `completed` if tests pass (exit 0).

### Fixed
- **Inline Read Injection Bounds:** Enforced protective line and size bounds on non-ranged `Read:` entries (capped at 150 lines and 5 KB) during prompt synthesis to prevent LLM context exhaustion.
- **Profile-Aware Concurrency Throttling:** Clamped active workers to `min(req.Workers, activeEligibleProfiles)` in plan dispatching to prevent worker contention and quota starvation on single-profile environments.
- **Wave Linting Model Fallback:** Injected default model fallback prior to wave linting, preventing un-modelled tasks from falsely failing `TierMicro` path and package limits.

## [2.2.2] - 2026-11-01

### Added
- **Multi-Wave Autonomous Dispatcher (`multigravity dispatch wave`):** Added automated wave orchestrator parsing `.agent/TASK.md` sequential and parallel wave specifications (`[PARALLEL: Wx]`, `[SEQ: Wx]`), executing plans across isolated ephemeral Git worktrees, handling verification retry loops, and automatically updating task completion logs. Added `--lint` validation flag.
- **Model-Aware Quota Routing & Inference Tiers:** Canonical tier classification (`TierMicro`, `TierStandard`, `TierReasoning`, `TierArchitecture`) mapped to live models (e.g. `gemini-3.8-flash-low/medium/high`). Static zero-token complexity classifier (`ClassifyTaskComplexity`), intelligent model escalation on failure, and model-aware quota bucket selection prioritizing secondary accounts.
- **Quota Cooldown Persistence & Calibration:** Persistent 429 rate limit cooldown tracking via `<profileDir>/.multigravity/quota-history.jsonl` with reset duration parsing (`ParseQuotaResetDuration`). Configurable safety quota floors (`DefaultSafetyQuotaThreshold`) and dynamic token calibration against live Cloud Code responses.
- **Installed Agent Auto-Discovery:** Dynamic agent CLI discovery (`DiscoverInstalledAgent`) across `claude`, `agy`, `codex`, `opencode`, and `aider` to avoid dispatching to missing runners.
- **Worktree Environment Seeding & Atomic Apply:** Ephemeral Git worktrees automatically inherit root repository Python virtual environments (`UV_PROJECT_ENVIRONMENT`, `VIRTUAL_ENV`, `PYTHONPATH`) and `NODE_PATH`. Added `--apply-atomic` and `multigravity dispatch apply` for topological merges with SHA base verification.

### Fixed
- **Runner Flag Normalization:** Fixed flag conflicts between `--model` and `--effort` for `agy` execution and rejected false-positive zero exit codes from empty Go test runs.
- **Profile Boundary Isolation:** Enforced per-profile Linux OAuth token file isolation by bypassing global D-Bus Secret Service keyrings in `language_server`, and fixed Google OAuth client secret resolution.
- **Task Cancellation & Timeout Cleanup:** Added cross-process PID termination fallback during plan cancellation and bounded diagnostic log tail ingestion on subtask failure.

## [2.2.1] - 2026-09-29

### Added
- **Explicit Dispatch Profile & Repository Target:** Added `--profile` (`-p`) and `--repo` (`-r`) flags to `multigravity dispatch run`, allowing scripted and orchestrated workflows to execute tasks under specific profiles and repositories without relying on interactive prompts.
- **Model Forwarding in Agent Command:** Added `--model` (`-m`) flag to `multigravity agent run` with automatic passthrough to `agy`, matching `exec` and headless runner capabilities.

### Fixed
- **Deterministic Quota Availability Flag:** Set `quota_known: false` for idle profiles or when live language server RPC telemetry is unavailable, eliminating misleading zero-percent quota reports and preventing premature exhaustion alerts.
- **Partial Execution Fan-out Resiliency:** `multigravity exec` now delivers partial aggregated reports instead of failing the entire operation when inactive auth-only profiles are skipped or individual worker timeouts occur.

## [2.2.0] - 2026-09-28

### Added
- **Native MCP Server (`multigravity mcp serve` & `mcp-server`):** Built-in Model Context Protocol (MCP) server supporting stdio and HTTP/SSE JSON-RPC 2.0 transports with 16+ tools spanning task dispatch, profile lifecycle, workspace mapping, live quota telemetry, proactive priming, and structured Git diff inspection.
- **Subtask Plan Aggregator (`multigravity dispatch plan` & `/api/v1/dispatch/plans`):** Concurrent batch execution of multi-agent plans across isolated ephemeral Git worktrees, synchronized under timeout control, with consolidated unified diff summaries, line metrics, and automated disjoint scope collision detection.
- **Multi-Agent Orchestration Skill (`skills/multigravity-orchestrator`):** Formal playbook for autonomous AI agents to decompose complex objectives into Directed Acyclic Graphs (DAGs), allocate profiles dynamically based on live quota fraction, and execute topological merges with zero merge conflicts.
- **Model Selection & Tiered Thinking:** Added `--model` (`-m`) flag in `multigravity exec`, `multigravity headless run`, and the REST API. Added support for tiered thinking levels (`LOW`, `MEDIUM`, `HIGH`) via `thinkingConfig` for Gemini 3.7 and 3.8 tiered models in the AI gateway, synced with live CloudCode agent catalog models.
- **Untracked File Diff Ingestion:** Task and worktree diff generation (`internal/worktree`) automatically incorporates newly created untracked files via `git diff --no-index` without mutating the Git index.
- **MCP Dispatch Arguments and Prompts:** `dispatch_task` accepts optional `args` and `prompt` parameters with automatic command resolution (`-p`) for `agy`.

### Fixed
- **Process Detection Prefix Collision:** Prevented false positive active profile status when profile names share prefixes (e.g., `yegear` matching `yegear2`) via strict boundary matching in `internal/profile`.
- **Pre-Flight Git Diagnostics:** Added Git installation and version diagnostics to `multigravity doctor`.
- **Headless Timeout Reporting:** Explicit context deadline exceeded message formatting for timeouts in `multigravity exec` and headless execution.
- **Task Deletion Force Flag:** Added `-f, --force` flag alias to `multigravity dispatch delete` to tear down associated Git worktrees in a single operation.
- **Language Server CSRF Redaction:** Strictly omitted the internal language server CSRF token from public JSON serialization in active servers, REST APIs, and MCP quota tools.

## [2.1.0] - 2026-09-26

### Added
- **MCP and Skills Catalog (`multigravity catalog`):** Read-only inventory of MCP servers, skills, and plugins, with the same sharing mode as `mcp status` and `skills status`. `GET /api/v1/catalog` returns the same report. Environment values, headers, arguments, and URL credentials stay out of the text and the JSON.
- **IDE Log Stream (`multigravity logs`):** Tail and follow the graphical IDE `main.log`, also exposed at `GET /api/v1/profiles/{name}/ide/logs`. Spawn lines redact `--csrf_token` and `--host_bridge_token`.
- **Profile Snapshots (`multigravity snapshot`):** Restore points for a profile and its AI conversations (`create`, `list`, `rollback`, `delete`). Archives live under `$MULTIGRAVITY_HOME/.snapshots/` and omit auth tokens and credential trees. Create and rollback refuse while the profile is running. `GET/POST /api/v1/profiles/{name}/snapshots` and `POST .../snapshots/{id}/rollback` expose the same contract.
- **Quota and Orphan Alerts (`multigravity alerts`):** Reports buckets at or below the existing 5% remaining-quota line, drops of at least 5 points between the two latest quota-history snapshots, and headless processes reaped by the background manager. `GET /api/v1/alerts` returns the same report, and `multigravity serve` pushes an `alerts` SSE event when the set changes.
- **Quota and Token Time Series:** `multigravity quota history` and `GET /api/v1/quota/history` store per-profile remaining-quota snapshots and token counts from the gateway and headless runs. Live `quota` reads stay instantaneous and append a point only when the fractions change.
- **Headless Google Sign-In (`multigravity login`):** Added OAuth2 PKCE login with an ephemeral `127.0.0.1` callback. Refresh tokens are stored only in the profile vault (`.gemini/antigravity-cli/antigravity-oauth-token` and `.gemini/jetski-standalone-oauth-token`), never in the host keyring. Profile launches that already have a vault set `GEMINI_FORCE_FILE_STORAGE=true`.
- **Parallel Headless Prompt Fan-out (`multigravity exec`):** Added `multigravity exec [profile|--all] "<prompt>"` and `POST /api/v1/exec`. The same prompt runs through the existing headless runner on each selected profile, with a worker pool and one aggregated JSON report. It does not start a second agent engine beside `dispatch` or the AI gateway.
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

### Fixed
- **Windows builds:** Terminal resize listening no longer references `SIGWINCH` on Windows, so the release binaries compile for `windows/amd64`.

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
