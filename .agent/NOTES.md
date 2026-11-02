# NOTES.md — Active Decisions and Contracts for Legacy Codebase

> Stores WHY recent changes and technical discoveries were made.
> For untouchable constraints, consult [`.agent/INVARIANTS.md`](INVARIANTS.md).
> For cumulative architectural logs and design specs, consult [`docs/adr/decisions.md`](../docs/adr/decisions.md).

---

## How to Use this File (for the Agent)

1. **Read before planning any task.**
2. **Record a new entry when:**
   - An unusual behavior or platform pitfall is investigated/resolved.
   - A new data contract (schema/payload) is expanded backward-compatibly.
   - Conscious technical debt is incurred during a fix.
3. **Keep entries concise and prune once consolidated into `INVARIANTS.md` or `ARCHIVE.md`.**

---

## Active Technical Contracts & Decisions

- **Pre-Approved Tasks (`APPROVED`), Parallel Waves (`[PARALLEL: Wx]`) & `task-queue` Skill:** Tasks finalized during interactive planning use `Status: APPROVED` in `.agent/TASK.md` (with independent disjoint-file tasks tagged `[PARALLEL: Wx]` + `Paths:`), allowing fresh sessions (`proceed`) or isolated worker subagents and multi-account quota failover (`task-queue` skill on `proceed queue`) to transition directly to `RUNNING` without a redundant `PLANNING` pause, halting only when ambiguities, unapproved dependencies, or blockers require human intervention.
- **Ephemeral Worktree Subagent Execution Gotchas (`k-002`):** See [`.agent/knowledge/k-002-dispatch-plan-worktree-orphan-and-venv-isolation.md`](knowledge/k-002-dispatch-plan-worktree-orphan-and-venv-isolation.md) for root causes and approved fixes covering (1) `agy -p` 20k thinking token timeouts, `--model` vs `--effort` flag conflict rules (`gemini-3.8-flash-low/medium/high` already encode effort), and single-grep escape hatch + line-ranged `Read: file:start-end` pre-injection, (2) orphaned detached child processes surviving `ExecutePlan` deadline and cross-process `CancelTask`, (3) `.venv` editable-install (`.pth`) shadowing in ephemeral worktrees without `PYTHONPATH` precedence, (4) bounded `verification_output`/`log_tail` diagnostics in `SubtaskResult` (including on `empty_diff`) and rejecting Go `testing: warning: no tests to run` false-positives, (5) diffing worktrees strictly against `HEAD` (`git diff HEAD -- <paths>`) / immutable SHA `BaseCommit` rather than moving `main` during multi-task wave commits, and (6) model-aware bucket routing (`gemini`/`argon`/`3p`) with failed-profile retry exclusion and native `.agent/TASK.md` wave execution (`multigravity dispatch wave`).

### 2026-11-02 [Task 08.6] Hybrid Workspace Modes (Zero-I/O Read-Only Dispatch)

- **Context:** Previously, subtask dispatch in `dispatch plan` and mutating runs incurred mandatory ephemeral git worktree creation (`git worktree add`, branch creation, lock contention) even for read-only or research tasks where modifying the codebase was prohibited.
- **Active Contracts:**
  - `internal/dispatch/types.go`:
    - `WorkspaceMode` constants: `WorkspaceWorktree` ("worktree"), `WorkspaceMirror` ("mirror"), `WorkspaceInherit` ("inherit").
    - `Task` and `DispatchOptions`: expanded with `Workspace string json:"workspace,omitempty"`.
  - `internal/dispatch/manager.go`:
    - Workspace mode resolution: when unset, defaults to `WorkspaceInherit` for read-only / researcher tasks or when `NewWorktree` is false, and `WorkspaceWorktree` when `NewWorktree` is true.
    - `WorkspaceInherit`: executes directly in-place (`repoRoot` or `opts.Cwd`) without git worktree or branch overhead ("Zero-I/O"), enforcing `agent-guard` filesystem mutation guards (`Sandbox: true`) when in read-only / researcher persona.
    - `WorkspaceMirror`: creates lightweight mirrored workspace (`.multigravity/mirrors/<taskID>`) with symlinked repository entries and `.git` without `git worktree add` overhead; cleaned up on task deletion or prune.
    - `WorkspaceWorktree`: full git worktree creation with dedicated branch and build isolation.
  - `internal/dispatch/plan.go`:
    - `SubtaskSpec`, `PlanRequest`, and `SubtaskResult`: expanded with `Workspace string json:"workspace,omitempty"`.
    - `ValidatePlan`: validates plan and subtask workspace modes (`worktree`, `mirror`, `inherit`).
    - `ExecutePlan`: read-only subtasks auto-default to `inherit` (skipping worktree creation); exempt from `empty_diff` failures and atomic apply; cleans up mirrors on task completion.
  - `internal/dispatch/wave.go`:
    - `WaveTaskSpec` and `WaveOptions`: support `- Workspace:` in `.agent/TASK.md` and propagate across wave execution and retries.
    - `LintTaskWave`: validates declared workspace modes.
  - `internal/cmd/`:
    - Added `--workspace` flag to `multigravity dispatch run`, `multigravity dispatch plan`, and `multigravity dispatch wave`.

### 2026-11-02 [Task 08.5] Dynamic Dispatch Roles & Execution Sandboxing

- **Context:** Previously, task dispatch assumed full mutating developer privileges for all agent runners, lacking persona role boundaries (researcher, verifier) and strict read-only execution modes.
- **Active Contracts:**
  - `internal/agent/guard.go`:
    - Role constants: `RoleDeveloper` ("developer"), `RoleResearcher` ("researcher"), `RoleVerifier` ("verifier"), `RoleReviewer` ("reviewer"), `RoleAuditor` ("auditor").
    - `IsReadOnlyRole(role string) bool`: reports whether an execution role is inherently read-only (`researcher`, `reviewer`, `auditor`).
    - `GuardOptions` & `WrapWithAgentGuardWithOptions(command, args, opts)`: wraps agent commands with `--role <role>` and `--read-only` boundary flags.
    - `InterceptSubshellMutationWithRole(repoRoot, subshellCmd, role, readOnly)`: checks command pipeline, blocking mutating commands (`rm`, `touch`, `cp`, `mv`, `mkdir`, `rmdir`, `chmod`, `chown`, `truncate`, `tee`), sed in-place (`sed -i`), git mutating actions (`commit`, `push`, `reset`, `clean`, `restore`, `merge`, `rebase`, `apply`, `cherry-pick`, `tag`, `rm`, `mv`, `revert`, `checkout`), and file redirection (`>`, `>>`) in read-only / verifier modes, while safely allowing null sinks (`/dev/null`, `nul`, `&1`, `&2`) and test output logs.
  - `internal/dispatch/`:
    - `types.go`: added `ReadOnly bool json:"read_only,omitempty"` to `Task` and `DispatchOptions`.
    - `manager.go`: `Dispatch` activates sandboxing when `opts.ReadOnly` or non-default `opts.Role` is passed, wrapping commands via `WrapWithAgentGuardWithOptions`, and preserving flags during verification retries.
    - `plan.go`: `SubtaskSpec`, `PlanRequest`, and `SubtaskResult` accept and propagate `Role` and `ReadOnly`. `ExecutePlan` exempts read-only / researcher tasks from `empty_diff` audit failures and atomic git apply mutations.
    - `wave.go`: `WaveTaskSpec` and `WaveOptions` parse `- Role:` and `- Read-Only:` bullets from `.agent/TASK.md` and propagate them across wave subtask execution and cascade retries.
  - `internal/cmd/`:
    - Added `--role` and `--read-only` flags to `multigravity dispatch run`, `multigravity dispatch plan`, `multigravity dispatch wave`, and `multigravity agent-guard wrap`.

### 2026-11-02 [Task 08.4] Cross-Profile Shared Cooldown & Throttling Pool

- **Context:** Previously, rate-limit (HTTP 429) feedback was recorded only per-profile in local memory and single profile history. When a model tier or provider RPM/TPM limit was hit, secondary profiles repeatedly attempted the exact same exhausted model, wasting time and quota retries.
- **Active Contracts:**
  - `internal/quota/cooldown.go`:
    - `CooldownEntry` / `CooldownPool`: models active rate-limit and throttling state across targets (`profile:<name>`, `model:<model>`, `bucket:<bucket>`, `global`), persisted to `$MULTIGRAVITY_HOME/.multigravity/cooldown-pool.json` with advisory file locking (`lockFile`/`unlockFile`).
    - `RecordModelCooldown(model string, duration time.Duration, reason string)`: places a specific model under shared cooldown. Effort suffixes (`-low`, `-medium`, `-high`) match their base family name.
    - `RecordBucketCooldown(bucketGroup string, duration time.Duration, reason string)`: places model buckets (`argon`, `3p`, `gemini`) under shared cooldown.
    - `RecordProfileCooldown(profile string, duration time.Duration)`: synchronizes profile cooldown to both the shared cooldown pool and the profile's `quota-history.jsonl`.
    - `IsModelOnCooldown(model string) (bool, time.Time, string)`: verifies whether global, model, base family, or bucket cooldown is active.
    - `IsProfileOnCooldown(profile string) bool`: checks in-memory state, shared pool, and history.
    - `ResetSharedCooldowns()`: clears in-memory and disk shared cooldown state.
    - `ParseRetryAfterHeader(headerVal string) time.Duration`: parses standard HTTP `Retry-After` headers (integer seconds, duration strings, and RFC1123 HTTP dates).
  - `internal/quota/routing.go`:
    - `PreflightProfilesForModelFromCandidates`, `PreflightProfilesForModel`, and `SelectEligibleProfilesForModel`: abort immediately if `IsModelOnCooldown(model)` is true, preventing futile secondary profile attempts.
    - `SelectEligibleProfilesForModel`: filters candidate profiles ensuring `IsProfileOnCooldown(cand.Profile)` is false.
    - `ParseQuotaResetDuration`: supports both `Retry-After` header values and "Resets in ..." error strings.
  - `internal/gateway/client.go`:
    - `UpstreamHTTPError`: expanded with `RetryAfter time.Duration` and `Model string`.
    - `StreamGenerateContentWithConnect`: extracts `Retry-After` header and fallback body duration into `UpstreamHTTPError`.
    - `CheckModelCooldown(model string)` and `RecordUpstreamRateLimit(model, profile string, err error)`: expose fast-path cooldown verification and error feedback recording.
  - `internal/gateway/gateway.go` & `internal/gateway/anthropic.go`:
    - Auto-failover inspects `UpstreamHTTPError.RetryAfter` to set precise cooldown on `MarkRateLimited` and `RecordProfileCooldown`.

### 2026-11-02 [Task 08.3] On-Demand Headless Agent Runner for Inactive Profiles

- **Context:** Automated wave execution and multi-account quota routing previously failed or skipped secondary profiles when inactive (not running in the desktop UI), and invocations specifying `agy --headless` failed with unrecognized flag errors (`flags provided but not defined: -headless`).
- **Active Contracts:**
  - `internal/headless/manager.go`:
    - `EnsureRunning(profileName string, timeout time.Duration) (*InstanceInfo, error)`: verifies a background headless server is responsive, returning existing healthy instances or starting a new managed server with strict environment and credential isolation.
    - `IsProfileActive(profileName string) bool`: returns true if either the desktop IDE process (`profile.IsProfileRunning`) or a live headless server is active.
  - `internal/agent/runner.go`:
    - `EnsureProfileReady(profileName string) error`: on-demand awakens inactive profiles via `headless.EnsureRunning` when not active.
    - `NormalizeAgyArgs(args []string) []string`: strips `--headless` and `--headless=true` variants while preserving valid flags and resolving model/effort conflicts.
    - `RunHeadlessSubagent(ctx context.Context, opts HeadlessSubagentOptions) (*HeadlessSubagentResult, error)`: executes subagents using `agy` in print mode (`-p`) or cleanly falls back to headless Language Server Cascade RPC if `agy` is not installed, recording tokens and Bayesian calibrator heuristics.
  - `internal/dispatch/`:
    - `manager.go`: `normalizeAgyArgs` delegates to `agent.NormalizeAgyArgs`. `Dispatch` awakens inactive profiles when `opts.AutoSpawnHeadless` or `opts.Headless` is set.
    - `plan.go`: `ExecutePlan` sets `AutoSpawnHeadless: isAgy` on dispatched subtasks.
    - `wave.go`: `ExecuteTaskWave` ensures explicit and retry target profiles are awakened on-demand via `agent.EnsureProfileReady`.

### 2026-11-02 [Task 08.2] Build Isolation & Compiler Cache Strategy for Concurrent Worktrees

- **Context:** Parallel subtasks in ephemeral git worktrees previously experienced `.cargo-lock` file contention and intermediate build artifact corruption when multiple workers ran `cargo build` or `cargo test` concurrently against a shared symlinked `target` directory from the repository root.
- **Active Contracts:**
  - `internal/worktree/symlink.go` & `internal/worktree/manager.go`:
    - `DefaultAutoLinkPaths`: updated to `[]string{"node_modules", ".venv", ".cargo", ".cache", ".sccache"}`, excluding `target` to avoid lockfile contention.
    - `IsolatedBuildPaths`: `[]string{"target", ".pytest_cache", ".mypy_cache", ".turbo"}` tracks output directories that must remain isolated per worktree.
    - `IsolateWorktreeBuildArtifacts(repoRoot, targetDir string) error`: ensures worktrees for Cargo repositories create an isolated regular `target/` directory rather than symlinking to the repository root.
    - `LinkGlobalCompilerCache(repoRoot, targetDir string) error`: auto-symlinks `.cache` and `.sccache` from `repoRoot` or the host user cache into worktrees.
    - `CleanWorktreeIndexLock(worktreePath string)`: proactively removes any transient `index.lock` left in the worktree's gitdir before agent session launch.
  - `internal/dispatch/executor.go`:
    - `ConfigureWorktreeBuildEnv(envMap map[string]string, repoRoot, worktreePath string)`: injects worktree-isolated `CARGO_TARGET_DIR` (pointing to `<worktreePath>/target`), discovers `sccache` to set `RUSTC_WRAPPER` and `SCCACHE_DIR`, and preserves host `GOCACHE` and `GOMODCACHE`.
    - `ExecuteVerifyCommandWithEnv(dir, cmdStr string, env map[string]string) ([]byte, int, error)`: executes verification commands within the worktree with isolated build environment variables and Go test false-positive guard.
  - `internal/profile/env.go`:
    - `PopulateWorktreeProjectEnv`: auto-populates `CARGO_TARGET_DIR` and compiler cache (`ConfigureCompilerCache`, `FindSccacheBinary`, `GetGlobalCompilerCacheDir`) for Cargo worktrees alongside Python and Node environments.

### 2026-11-02 [Task 08.1] Cross-Profile Toolchain & PATH Discovery (`.isolated_toolchain` Sentinel & Doctor Audit)

- **Context:** Subagents and isolated profile environments previously lacked automated discovery of host dev compilers (Python via `uv`/`pyenv`/`conda`, Go SDKs in `/usr/local/go/bin` or `~/sdk/go*/bin`, ASDF/nodenv shims, Windows `Python*/Scripts`) and peer profile toolchain installations under `MULTIGRAVITY_HOME`.
- **Active Contracts:**
  - `internal/config/config.go`:
    - `SentinelIsolatedToolchain = ".isolated_toolchain"`: sentinel file opting a profile out of cross-profile toolchain discovery and sharing.
  - `internal/profile/env.go`:
    - `DiscoverPeerProfileToolchains(currentProfile string) []string`: scans peer profiles in `MULTIGRAVITY_HOME` (excluding `currentProfile` and profiles with `.isolated_toolchain`), returning verified `.cargo/bin`, `.local/bin`, `go/bin`, `.bun/bin`, `.local/share/pnpm`, and Windows Python dirs.
    - `BuildEnrichedPATHForProfile(basePath, hostHome, currentProfile string) string`: expands candidate scanning for Python (`uv`, `pyenv`, `asdf`, `conda`), Go (`/usr/local/go/bin`, `/opt/go/bin`, `~/sdk/go*/bin`), Node (`asdf`, `nodenv`, `nvm`, `/opt/homebrew/bin`), Rust (`~/.rustup/toolchains/*/bin`), and injects discovered peer toolchains while strictly enforcing native Linux paths preceding Windows mounts (`/mnt/c/...`).
    - `BuildEnrichedPATH(hostHome)` and `BuildEnrichedPATHWithBase(basePath, hostHome)` delegate backward-compatibly to `BuildEnrichedPATHForProfile`.
  - `internal/profile/sharing.go`:
    - `GetToolchainStatus`, `ToolchainStatus`, `ToolchainShare`, `ToolchainIsolate`: exposes toolchain sharing and isolation under resource `"toolchain"`, wired into `GetAllSharingStatus` (now reporting 6 resources) and `SetResourceSharing`.
  - `internal/doctor/doctor.go`:
    - Added `"Development Toolchains"` diagnostic check inspecting available compilers (`cargo`, `rustc`, `node`, `pnpm`, `go`, `python`) on the enriched PATH and reporting detected binaries.

### 2026-11-02 [Task 07.4] On-Demand Secondary Profile Quota Probing (`--probe` / Wave Pre-Flight)

- **Context:** Automated multi-account wave distribution previously failed to route tasks to inactive secondary profiles whose language servers were offline (`quota_known: false`), restricting worker parallelism to the current active profile.
- **Active Contracts:**
  - `internal/quota/routing.go`:
    - `ProbeInactiveProfiles(profiles []string) ([]string, error)`: discovers registered profiles lacking an active language server, boots background headless instances (`StartHeadlessServer`), records `.multigravity/headless.json` state, queries quota summaries, and returns the slice of awakened profile names.
    - `SetProbeTestHook(fn func(profileName string) (*ActiveServer, error)) func()`: thread-safe hook allowing hermetic testing of inactive profile probing without spawning OS processes.
  - `internal/dispatch/wave.go`:
    - `WaveOptions`: added `Probe bool json:"probe,omitempty"`. When true, executes `quota.ProbeInactiveProfiles(nil)` during wave pre-flight before planning and concurrency calculation.
  - `internal/cmd/dispatch_wave.go`:
    - `multigravity dispatch wave`: added `--probe` flag wired into `WaveOptions.Probe`.

### 2026-11-02 [Task 07.3] AST Outline Extraction for Large Files in Subtask Pre-Injection (Token Economy)

- **Context:** Non-ranged file pre-injections exceeding `maxNonRangedLines = 150` lines or `maxNonRangedBytes = 5120` bytes previously suffered arbitrary line truncation, cutting off struct fields, interfaces, and function signatures.
- **Active Contracts:**
  - `internal/dispatch/outline.go`:
    - `ExtractASTOutline(filePath string, content []byte) (string, error)`: parses Go source files using stdlib `go/parser` and `go/ast`, removes function/method bodies (`fn.Body = nil`) and comments, and formats a clean outline with `go/format.Node`. For TypeScript/JavaScript (`.ts`, `.tsx`, `.js`, `.jsx`), extracts top-level and exported interface/type/class/function declarations. Returns an error on unsupported file extensions or syntax parse failures.
  - `internal/dispatch/plan.go`:
    - `SynthesizeSubtaskPromptWithRepo`: when non-ranged files (`startLine == 0 && endLine == 0`) exceed `maxNonRangedLines` (150) or `maxNonRangedBytes` (5120), attempts `ExtractASTOutline`. On success, formats the snippet as `--- <entry> (AST Outline) ---`, capping the outline at `maxNonRangedBytes` while preserving semantic contracts within `maxBudget = 15360` (15 KB). Gracefully falls back to raw line truncation if AST outline extraction is unavailable or fails.

### 2026-11-02 [Tasks 07.1 & 07.2] Worktree Ephemeral Branch Cleanup on Apply/Prune & Compact JSON Wave Output Sanitization

- **Context:** Wave execution and worktree operations previously leaked ephemeral git branches (`multigravity/wave-*`, `multigravity/retry-*`, `multigravity/<task-id>`) in the host repository upon prune, completion, or failure. In addition, raw wave execution results dumped full prompts, raw diffs, verbose log tails, and empty structured diff blocks into machine-readable JSON outputs.
- **Active Contracts:**
  - `internal/worktree/`:
    - `manager.go`:
      - `RemoveWorktree`: automatically deletes local git branch with `GitBranchDelete(repoRoot, cleanBranch, true)` when `opts.DeleteBranch` is true OR when the branch starts with `multigravity/`.
      - `PruneWorktree(repoPath, id string) error`: prunes a specific worktree by ID and deletes any associated `multigravity/` branch.
      - `PruneAll(repoPath string) error` / `PruneWorktrees(repoPath string) error`: cleans missing worktree manifest records, executes `GitWorktreePrune`, and sweeps all local git branches with prefix `multigravity/` deleting any branch that is not currently checked out by an active worktree.
      - `CreateWorktree`: proactively removes any transient `.git/worktrees/<id>/index.lock` file left during worktree provisioning.
    - `git.go`:
      - `GitBranchDelete(repoRoot, branch string, force bool) error`: normalizes branch references by stripping `refs/heads/`.
      - `GitListBranches(repoRoot, prefix string) ([]string, error)`: lists local branch names matching prefix using `git for-each-ref`.
  - `internal/dispatch/`:
    - `plan.go`:
      - `SanitizeSubtaskCompact(st SubtaskResult) SubtaskResult`: returns a compact copy clearing `Prompt` and `Diff`, omitting `StructuredDiff` when 0 files were changed (`nil`), and truncating `LogTail` and `VerificationOutput` to at most 500 characters with `... (truncated) ...` header.
      - `PlanResult.ToCompact()`: maps all subtasks through `SanitizeSubtaskCompact` and clears `UnifiedSummary.CombinedDiff`.
      - `ExecutePlan`: prunes ephemeral worktrees and branches upon successful atomic commit (`req.ApplyAtomic`) or upon cancellation/failure.
    - `wave.go`:
      - `WaveExecutionResult.ToCompact()`: maps all tasks through `SanitizeSubtaskCompact`.
      - `ExecuteTaskWave`: cleans up all wave task worktrees and branches and executes `worktree.PruneWorktrees(repoRoot)` upon return.

### 2026-11-01 [Tasks 90.1 & 90.2] Componentization of Server Routes & Modularization of Dispatch CLI Subcommands

- **Context:** Parallel refactoring wave W1 componentized monolithic legacy files into focused domain-specific files while preserving exact API and CLI surface contracts with 100% test pass.
- **Active Contracts:**
  - `internal/server/`:
    - `routes.go`: retained router registration (`setupRoutes`), middleware (`corsMiddleware`), and response helpers (`writeJSON`, `writeError`).
    - `routes_profile.go`: profile CRUD, stats, share, prime, shortcuts, and exec endpoints.
    - `routes_dispatch.go`: dispatch run, wave, plan, tasks, worktree, agent sessions, and workspaces endpoints.
    - `routes_gateway.go`: gateway chat completions, messages, models, quota, history, and router endpoints.
    - `routes_system.go`: health, doctor, alerts, catalog, idelog, snapshot, and MCP endpoints.
  - `internal/cmd/`:
    - `dispatch.go`: root `dispatchCmd` command definition, global/shared flags, and subcommand registration.
    - `dispatch_run.go`: `runSubCmd` and execution flags (`--profile`, `--verify`, `--retry`, `--model`, etc.).
    - `dispatch_ops.go`: operational subcommands (`list`, `status`, `logs`, `diff`, `dashboard`, `cancel`, `delete`, `prune`).
    - `dispatch_plan.go`: `planSubCmd` (`multigravity dispatch plan`).
    - `dispatch_wave.go`: `waveSubCmd` (`multigravity dispatch wave`).
    - `dispatch_apply.go`: `applySubCmd` (`multigravity dispatch apply`).


### 2026-11-01 [Tasks 06.1–06.6] Orchestration Resiliency, Token Economy, Verify-on-Timeout, Profile-Aware Concurrency Throttling & InlineRead Bounds

- **Context:** Parallel plan execution and wave orchestration (`multigravity dispatch wave` / `ExecutePlan`) required compact artifact outputs, build cache dependency symlinks in ephemeral worktrees, verify-on-timeout recovery, default model fallback in wave linting, profile-aware concurrency throttling to prevent single-profile starvation, and line/size bounds on non-ranged `Read:` entries in pre-loaded context.
- **Active Contracts:**
  - `internal/dispatch/plan.go` (`[06.5]`, `6dbb07f`; `[06.6]`, `92663c3`):
    - `resolveEligibleProfilesForModel(model string) []string`: pre-flights profiles using safety threshold with fallback to `DefaultSafetyQuotaThreshold` and profile listing, excluding profiles on active cooldown.
    - `countActiveEligibleProfiles(subtasks []SubtaskSpec, defaultModel string) int`: calculates distinct active profile count, resolving auto profiles against eligible candidate pools.
    - `ExecutePlan`: clamps worker concurrency to `min(req.Workers, activeEligibleProfiles)` (floor of 1) and distributes auto subtasks round-robin across eligible profiles.
    - `SynthesizeSubtaskPromptWithRepo`: caps individual non-ranged `Read:` entries (`startLine == 0 && endLine == 0`) to `maxNonRangedLines = 150` lines and `maxNonRangedBytes = 5120` (5 KB), preventing large files from blowing out context budgets while preserving ranged micro-contracts (`file:start-end`) up to aggregate `maxBudget = 15360` (15 KB).

### 2026-11-01 [Tasks 05.13–05.27] Inference Tier Matrix, Complexity Classifier, Cascade Retry Escalation, Quota Pre-Flight (`1%` Floor & Persistent `quota-history.jsonl` Cooldowns), Installed-Runner Auto-Discovery, Failure Classification, Wave Linting, and Documentation Sync

- **Context:** Extended wave orchestration (`multigravity dispatch wave` / `ExecutePlan` / `multigravity route`) with a 4-tier model capability hierarchy, zero-token static task complexity classification, structured failure reason classification, failure-gated cascade retry escalation, quota pre-flight safety floors (`1%` stable floor, `15%` complex-task threshold) with persistent 429 cooldown tracking in `quota-history.jsonl`, installed-runner auto-discovery (`DiscoverInstalledAgent`), automated `.agent/TASK.md` log/promotion updates, sibling `*_test.go` pre-injection, micro-contract wave linting (`--lint`), and synchronized agent skills + bilingual READMEs.
- **Active Contracts:**
  - `internal/quota/tiers.go` & `internal/quota/classifier.go` (`[05.13.1]`, [`65bb810`](https://github.com/yegear1/multigravity-cli/commit/65bb810); `[05.13.2]`, [`407eec5`](https://github.com/yegear1/multigravity-cli/commit/407eec5)):
    - Canonical tiers (`TierMicro`, `TierStandard`, `TierReasoning`, `TierArchitecture`), `ClassifyModelTier`, `EscalateModel`, and `ClassifyTaskComplexity(spec TaskSpec) ComplexityResult`.
  - `internal/quota/routing.go` & `internal/quota/history.go` (`[05.15]`, [`8be6cae`](https://github.com/yegear1/multigravity-cli/commit/8be6cae); `[05.22]`, [`a7c815e`](https://github.com/yegear1/multigravity-cli/commit/a7c815e); `[05.26]`, [`cb2f996`](https://github.com/yegear1/multigravity-cli/commit/cb2f996)):
    - `CriticalQuotaThreshold = 0.01`, `DefaultSafetyQuotaThreshold = 0.01`, `SafetyThresholdForModel` (`0.15` for `TierReasoning`/`TierArchitecture` or `WeightHeavy`, `0.01` for `TierMicro`/`TierStandard`), `RecordProfileCooldown` / `IsProfileOnCooldown` (persisting `Sample.CooldownUntil` with `SourceCooldown = "cooldown"` in `<profileDir>/.multigravity/quota-history.jsonl` across CLI invocations), `ParseQuotaResetDuration` (extracting `"Resets in ..."`), and `PreflightProfilesForModel` wired into `ExecutePlan` and `ExecuteTaskWave` with fallback to `0.01` floor (while preserving `MinRemainingFraction = 0.05` for `prime` and `alert`).
  - `internal/dispatch/manager.go` (`[05.25]`, [`0d0985a`](https://github.com/yegear1/multigravity-cli/commit/0d0985a)):
    - `DiscoverInstalledAgent` probes `claude`, `agy`, `codex`, `opencode`, `aider` across `PATH` and `profile.BuildEnrichedPATH`, skipping uninstalled runners automatically when `agentType == ""` and `command == ""`.
  - `skills/` & `README*.md` (`[05.23]`, [`472433c`](https://github.com/yegear1/multigravity-cli/commit/472433c); `[05.24]`, [`dc116fd`](https://github.com/yegear1/multigravity-cli/commit/dc116fd); `[05.27]`, [`9d9855b`](https://github.com/yegear1/multigravity-cli/commit/9d9855b)):
    - Synchronized `skills/multigravity-orchestrator/SKILL.md`, `skills/multigravity/SKILL.md`, `README.md`, and `README.pt-br.md` with Epic 05 contracts.
  - `internal/dispatch/plan.go` (`[05.13.3a]`, [`f4b34d2`](https://github.com/yegear1/multigravity-cli/commit/f4b34d2); `[05.16]`, [`266bec2`](https://github.com/yegear1/multigravity-cli/commit/266bec2); `[05.20]`, [`9e27365`](https://github.com/yegear1/multigravity-cli/commit/9e27365); `[05.21]`, [`1524e9d`](https://github.com/yegear1/multigravity-cli/commit/1524e9d)):
    - `SynthesizeCascadeRetryPrompt` injects bounded failure diagnostics into retry prompts.
    - `SubtaskResult.FailureReason` (`ClassifySubtaskFailure`) classifies `timeout`, `rate_limited`, `empty_diff`, `verify_failed`, and `agent_error`; `SubtaskResult.TokensUsed` records estimated tokens.
    - `SynthesizeSubtaskPromptWithRepo` / `ExecutePlan` invokes `ClassifyTaskComplexity` when model is omitted, injects `--model <resolvedModel>` into `dOpts.Args`, includes `Verify:`, and pre-injects trailing lines of sibling `*_test.go` files within residual `maxBudget` (`15 KB`).
  - `internal/dispatch/wave.go` & `internal/cmd/dispatch.go` (`[05.13.3b]`, [`19248bb`](https://github.com/yegear1/multigravity-cli/commit/19248bb); `[05.14]`, [`eb5f857`](https://github.com/yegear1/multigravity-cli/commit/eb5f857); `[05.17]`, [`3492921`](https://github.com/yegear1/multigravity-cli/commit/3492921); `[05.19]`, [`3efaa40`](https://github.com/yegear1/multigravity-cli/commit/3efaa40); `[05.18]`, [`7637c47`](https://github.com/yegear1/multigravity-cli/commit/7637c47)):
    - `ParseTaskWave` strips governance metadata tags (`[APPROVED]`, `[PARALLEL: Wx]`, `[SEQ: Wx]`) from `WaveTaskSpec.Title`.
    - `ExecuteTaskWave` gates retry escalation on `res.FailureReason` (recording profile cooldown and keeping same model on `rate_limited`, escalating via `quota.EscalateModel` on `verify_failed`, and skipping retry on `empty_diff`/`timeout` with empty diagnostics).
    - `markTasksCompletedInFile` prepends completed tasks to `## Completed Tasks Log`, marks `## Active Task` acceptance criteria `[x]`, and promotes the next `[APPROVED]` backlog item.
    - `LintTaskWave` and `multigravity dispatch wave --lint` validate `TierMicro` path/package bounds, `Read:` entry counts ($\le 4$), and presence of `Verify:`.

### 2026-11-01 [Tasks 05.2–05.12] Ephemeral Worktree Project Env Seeding, Plan Timeout Subtask Termination, Route Tier Alignment, Immutable `BaseCommit` Diff Scoping, Model-Aware Quota Bucket Routing, Line-Ranged `Read:` Pre-Injection, and Native `dispatch wave`

- **Context:** Multi-account parallel wave execution (`task-queue` / `multigravity dispatch plan` / `multigravity dispatch wave`) required automatic `.venv`/`PYTHONPATH`/`NODE_PATH` inheritance inside ephemeral worktrees, guaranteed termination of timed-out child sessions, bounded failure diagnostics in `SubtaskResult` (including on `empty_diff`), alignment of `multigravity route` with `agy models` CLI identifiers, immutable SHA `BaseCommit` resolution + path-scoped diffs, model-aware bucket routing (`gemini`/`argon`/`3p`) with failed-profile retry exclusion, line-ranged `Read: file:start-end` pre-injection, and single-command `.agent/TASK.md` wave execution.
- **Active Contracts:**
  - `internal/profile/env.go` & `internal/agent/env.go` (`[05.2]`, [`b1e5ae5`](https://github.com/yegear1/multigravity-cli/commit/b1e5ae5)):
    - `PopulateWorktreeProjectEnv(envMap map[string]string, baseRepoPath, worktreePath string)` seeds `UV_PROJECT_ENVIRONMENT` and `VIRTUAL_ENV` from `<baseRepoPath>/.venv`, prepends `.venv/bin` (or `Scripts` on Windows) to `PATH`, and prepends `<worktreePath>/src` and `<worktreePath>` to `PYTHONPATH` so worktree modules shadow editable `.pth` links. Also seeds `NODE_PATH` and `<baseRepoPath>/node_modules/.bin` when present.
    - `BuildAgentEnv` invokes `PopulateWorktreeProjectEnv` automatically when running inside `.multigravity/worktrees/<id>`.
  - `internal/dispatch/plan.go`, `internal/dispatch/wave.go`, `internal/dispatch/manager.go` & `internal/cmd/dispatch.go` (`[05.3]`, [`2837a59`](https://github.com/yegear1/multigravity-cli/commit/2837a59); `[05.9]`, [`1714133`](https://github.com/yegear1/multigravity-cli/commit/1714133); `[05.10]`, [`2f53f12`](https://github.com/yegear1/multigravity-cli/commit/2f53f12); `[05.11]`, [`74d1e09`](https://github.com/yegear1/multigravity-cli/commit/74d1e09)):
    - `ExecutePlan` calls `m.CancelTask(repoRoot, task.ID, true)` when `planCtx` expires or `completedTask.Status == StatusRunning` after `WaitForTask`, and preserves any partial worktree diff.
    - `CancelTask` falls back to killing `task.PID` via `os.FindProcess` when `m.agentMgr` has no in-memory session (cross-process CLI invocation).
    - `ResolveAgentCommandWithCwd` normalizes conflicting `agy` `--model <m>-<tier>` and `--effort <e>` flags via `normalizeAgyArgs`, and `defaultExecuteVerifyCommand` treats Go `testing: warning: no tests to run` as verification failure (`exitCode = 1`).
    - `SynthesizeSubtaskPromptWithRepo` supports line-ranged `Read: file:start-end` pre-injection (`InlineRead`), defaults micro-contracts to `gemini-3.8-flash-low`, and includes the single-grep escape hatch guardrail.
    - `SubtaskResult` populates `LogTail` whenever `Status != StatusCompleted || strings.TrimSpace(Diff) == ""`, marks `empty_diff` tasks with `Paths` as `StatusAuditFailed`, and supports `ApplyAtomic` (`--apply-atomic`) and `OmitDiff` (`--omit-diff`).
    - `ParseTaskWave` and `ExecuteTaskWave` (`multigravity dispatch wave [--wave Wx] [--file .agent/TASK.md] [--timeout 4m] [--no-retry] [--json]`) parse `.agent/TASK.md` waves, execute subtasks atomically, retry once on `gemini-4-argon-medium` excluding the failed profile, and auto-update/commit `.agent/TASK.md`.
  - `internal/quota/routing.go` (`[05.4]`, [`8c50ba4`](https://github.com/yegear1/multigravity-cli/commit/8c50ba4); `[05.7]`, [`a842530`](https://github.com/yegear1/multigravity-cli/commit/a842530); `[05.8]`, [`4bc9bb9`](https://github.com/yegear1/multigravity-cli/commit/4bc9bb9)):
    - `ModelForWeight` maps `light` -> `gemini-3.8-flash-low`, `medium` -> `gemini-3.8-flash-medium`, `heavy` -> `gemini-3.8-flash-high`.
    - `EffortForWeight` maps `light` -> `low`, `medium` -> `medium`, `heavy` -> `high`, exposed as `RecommendedEffort` (`recommended_effort`) in `RouteRecommendation`.
    - `SelectEligibleProfiles`, `SelectEligibleProfilesFromCandidates`, `BucketGroupForModel`, `ScoreServerForBucket`, and `SelectEligibleProfilesForModel` score target model buckets (`gemini`, `argon`, `3p`), prevent false-positive `1.0` scores when a model family is absent, and prioritize non-excluded secondary profiles.
  - `internal/worktree/manager.go`, `internal/worktree/git.go`, `internal/worktree/types.go` (`[05.6]`, [`080fba6`](https://github.com/yegear1/multigravity-cli/commit/080fba6)):
    - `CreateWorktree` resolves `wt.BaseCommit` to the worktree's immutable initial `HEAD` commit SHA (`headCommit`) so advancing `main` during multi-task waves never shifts the diff baseline.
    - `DiffOptions.Paths` and `GitDiff(..., paths ...string)` scope both tracked diffs and untracked file diffs to the specified paths.
  - `skills/multigravity-orchestrator/SKILL.md` (`[05.5]`, [`d7c7ee8`](https://github.com/yegear1/multigravity-cli/commit/d7c7ee8); `[05.12]`, [`2b8e717`](https://github.com/yegear1/multigravity-cli/commit/2b8e717)):
    - Synchronized with `multigravity dispatch wave`, `Read: file:start-end` pre-injection, `gemini-3.8-flash-low` defaults, model-aware bucket routing, and bounded `SubtaskResult` diagnostics.


### 2026-10-31 [Task 05.1] Centralized PATH Enrichment, Host Toolchain Preservation, and Doctor Toolchain Bleed Detection

- **Context:** Isolated profile execution (`profile/launcher.go`, `headless/manager.go`, `agent/env.go`) duplicated logic for enriching `PATH` and did not discover modern JS runtime toolchains (`pnpm`, `fnm`, `volta`, `nvm`). On WSL2, unmanaged ordering allowed Windows mount paths (`/mnt/c/...`) to shadow Linux runtimes. In addition, redirecting `HOME` to `profileDir` caused Cargo, Rustup, and Go to default their toolchain directories to `<profileDir>/.cargo`, `<profileDir>/.rustup`, and `<profileDir>/go`, breaking compiler shims and triggering accidental toolchain downloads into profiles.
- **Active Contracts:**
  - `internal/profile/env.go`:
    - `BuildEnrichedPATH(hostHome string) string` / `BuildEnrichedPATHWithBase(basePath, hostHome string) string`: consolidates candidate scanning (`.local/bin`, `.cargo/bin`, `.bun/bin`, `go/bin`, `.local/share/pnpm`, `.fnm/current/bin`, `.volta/bin`, and `~/.nvm/versions/node/*/bin`).
    - Enforces stable partitioning on Unix/WSL2 where all native Linux system and user paths precede `/mnt/c/` Windows mounts.
    - `PopulateToolchainEnv(envMap map[string]string, hostHome string)`: exports `CARGO_HOME`, `RUSTUP_HOME`, and default `GOPATH` pointing to `hostHome` when unset.
  - Consumers: `launcher.go`, `headless/manager.go`, and `agent/env.go` delegate to `profile.BuildEnrichedPATH` and `profile.PopulateToolchainEnv`.
  - `internal/doctor/doctor.go`: audits profile directories under `MULTIGRAVITY_HOME` and emits a warning if orphaned `.cargo` or `.rustup` directories exist.

### 2026-10-31 [Task 00.6] Open Source Readiness & Upstream Gateway Error Port-Number Safety

- **Context:** Audited codebase for public open-source readiness and resolved an intermittent test failure in `TestGatewayUpstreamErrorDoesNotFailover`. The rate-limit detector `IsRateLimitOrQuotaExhausted` in `internal/gateway/client.go` was inspecting `err.Error()`, which embeds `e.Endpoint` (e.g. `http://127.0.0.1:42981` or `:40315`). When an ephemeral test server was assigned an OS port containing "429" or "403", 500 Internal Server Errors were falsely classified as rate limits or quota exhaustion, triggering unexpected failover.
- **Active Contracts:**
  - `internal/gateway/client.go`: `IsRateLimitOrQuotaExhausted` checks `upstreamErr.StatusCode` and matches error patterns strictly against `upstreamErr.Message` (the HTTP response body), isolating detection from the endpoint hostname/port.
  - Community Governance: Scaffolding added for `CODE_OF_CONDUCT.md` (Contributor Covenant 2.1), `CONTRIBUTING.md`, `SECURITY.md` (48h SLA), `.github/pull_request_template.md`, and `.github/ISSUE_TEMPLATE/` (`bug_report.md`, `feature_request.md`).
  - Git identity: `git config user.email` configured locally to `yegear1@users.noreply.github.com`.
  - Skill hygiene: Sanitized hardcoded local developer host paths (`/home/yegear/`) in `skills/multigravity-orchestrator/SKILL.md`.

### 2026-10-23 [Task 04.4] Per-Profile OAuth Token File Isolation & Linux D-Bus Keyring Bypass (`language_server`)

- **Context:** On Linux desktop sessions (`gnome-keyring-daemon` / `org.freedesktop.secrets`), multiple Antigravity IDE profiles held separate Google accounts in memory while running concurrently, but closing and reopening a profile from scratch reverted to whichever account authenticated most recently. Reverse-engineering `/opt/antigravity/resources/bin/language_server` (v2.25.0) revealed that `codeassistclient.NewCompositeTokenStorage` ignores `GEMINI_FORCE_FILE_STORAGE` (`0` occurrences in binary) and defaults to `github.com/zalando/go-keyring` under a single global service key (`service="antigravity"`) unless `shouldBypassKeyring()` or `keyringRecentlyUnavailable()` returns `true`.
- **Active Contracts:**
  - `internal/profile/layout.go` (`EnsureProfileLayout`): creates/refreshes `<profileDir>/.gemini/cache/antigravity-keyring-unavailable` (mode `0600`) so `language_server`'s `keyringRecentlyUnavailable` check selects `fileTokenStorage` (`$HOME/.gemini/jetski-standalone-oauth-token`).
  - `internal/profile/launcher.go`, `internal/agent/env.go`, `internal/headless/manager.go`, `internal/quota/headless.go`:
    - Unconditionally export `GEMINI_FORCE_FILE_STORAGE="true"` for all isolated profile environments (for `agy` / `gemini-cli`).
    - On Linux (`runtime.GOOS == "linux"`), export `WSL_INTEROP="/dev/null"` when `WSL_INTEROP` and `WSL_DISTRO_NAME` are unset so `language_server`'s `wslDetector.Detect()` inside `shouldBypassKeyring()` permanently sets `useFileStatic = true` without expiring after 1 hour and without breaking D-Bus desktop portals (`xdg-open`, file pickers).
  - See [`.agent/knowledge/k-001-linux-language-server-keyring-bypass.md`](knowledge/k-001-linux-language-server-keyring-bypass.md) for binary-level analysis and anti-patterns.

### 2026-10-17 [Task 04.2] Task-Aware Trace Telemetry (`--trace-id` flag, `MAESTRI_TRACE_ID` ingestion, VictoriaLogs token delta logging)

- **Context:** Distributed tracing and telemetry correlation across dispatch tasks, interactive/detached agent runs, and reverse proxy gateway calls. Correlates token usage and process lifecycle events into VictoriaLogs via Vector.
- **Active Contracts:**
  - `internal/telemetry`:
    - Schema: Canonical VictoriaLogs/Vector fields `timestamp` (RFC3339Nano), `level`, `service` ("multigravity"), `app` ("multigravity"), `env`, `message`, `trace_id`, `task_id`, `profile`, `model`, `prompt_tokens`, `completion_tokens`, `total_tokens`, `token_delta`, `duration_ms`.
    - Shipment: Asynchronous HTTP POST (with 1.5s timeout and dropped buffered events on backpressure) to Vector pipeline endpoint `VECTOR_URL` or `http://${VECTOR_HOST:-localhost}:${VECTOR_HTTP_PORT:-8686}/logs`.
    - Local fallback: Appends NDJSON to `$MULTIGRAVITY_HOME/.multigravity/telemetry.jsonl`.
    - Custom writer override (`SetCustomWriter`) for testing and isolation.
  - `internal/dispatch`:
    - `Task` and `DispatchOptions` include `TraceID string json:"trace_id,omitempty"`.
    - Trace ID resolution hierarchy: `opts.TraceID` -> `taskEnv["MAESTRI_TRACE_ID"]` -> `taskEnv["MULTIGRAVITY_TRACE_ID"]` -> `os.Getenv("MAESTRI_TRACE_ID")`.
    - Injects `MAESTRI_TRACE_ID`, `MULTIGRAVITY_TRACE_ID`, and `MULTIGRAVITY_TASK_ID` into agent process environments.
    - Emits lifecycle telemetry events (`task_started`, `task_completed`, `task_failed`, `task_cancelled`, `task_audit_failed`) with trace context.
  - `internal/agent`:
    - `CreateSessionOptions` and `Session` include `TraceID string json:"trace_id,omitempty"`.
    - Injects `MAESTRI_TRACE_ID` and `MULTIGRAVITY_TRACE_ID` into session child process environments.
  - `internal/quota`:
    - `Sample` includes `TraceID string json:"trace_id,omitempty"` and `TaskID string json:"task_id,omitempty"`.
    - `HistoryQuery` includes `TraceID string json:"trace_id,omitempty"`.
    - Introduces `RecordTokensWithTrace(profile, model, promptTokens, completionTokens, totalTokens, traceID, taskID)` preserving backward-compatible `RecordTokens(...)`.
  - `internal/gateway`:
    - Extracts trace context via `extractTraceContext(r)` checking `X-Trace-ID`, `X-Maestri-Trace-ID`, `Traceparent` (extracting W3C 32-hex trace ID), query parameter `trace_id`, and `MAESTRI_TRACE_ID` environment variable.
    - Always echoes `X-Trace-ID: <trace_id>` in HTTP response headers (streaming SSE and unary JSON responses).
    - Emits structured `telemetry.Event` ("gateway_tokens_recorded") with token delta on every completed upstream request.
  - CLI & MCP:
    - CLI flags `--trace-id <id>` added to `dispatch run` and `agent run`.
    - MCP tool `dispatch_task` accepts optional `trace_id` string property.

### 2026-10-15 [Task 04.1] Dynamic Token Quota Budget Alerts and Auto-Throttle Policies

- **Context:** Automated threshold alerts for token burn rates, quota depletion velocity, and auto-throttling when profiles near quota depletion.
- **Active Contracts:**
  - `internal/quota`:
    - `throttle.go`: introduces `TokenBurnRate` (`TokensBurned`, `TokensPerMinute`, `TokensPerHour`, `SampleCount`), `ThrottlePolicy` (`DepletionThreshold` [default 10%], `ThrottleDelayMs` [default 1000ms], `MaxTokensPerHour`, `MaxTokensPerMinute`, `AutoThrottleEnabled`), and `ThrottleConfig` with fleet defaults and per-profile overrides.
    - Helpers: `CalculateTokenBurnRate`, `CalculateFleetBurnRates`, `LoadThrottleConfig`, `SaveThrottleConfig`, `DefaultThrottleConfig`. Persistence under `$MULTIGRAVITY_HOME/.multigravity/throttle-policy.json`.
  - `internal/alert`:
    - `alert.go`: adds alert kinds `KindQuotaNearingDepletion` (`"quota_nearing_depletion"`), `KindTokenBudgetExceeded` (`"token_budget_exceeded"`), and `KindTokenBurnRateHigh` (`"token_burn_rate_high"`).
    - `Alert` payload extends backward-compatibly with `TokensBurned`, `TokenBurnRate`, and `BudgetLimit`.
  - `internal/gateway`:
    - `router.go`: `ProfileNode` tracks `ThrottleDelay` and `IsThrottled`. `Score` penalizes nearing-depletion profiles (-400.0) so healthy nodes are prioritized. In `SelectProfile`, auto-throttle sets pacing delay when selecting depleted profiles.
    - `gateway.go` & `anthropic.go`: pacing delay (`time.After`) applied on throttled nodes; sets response headers `X-Throttled: true` and `X-Throttle-Delay: <delay>`.
  - `internal/cmd`:
    - `alerts.go`: surfaces token burn counts when alert reports them.
    - `quota_burn.go`: adds subcommands `multigravity quota burn [profile] [--window <dur>] [--json]` and `multigravity quota throttle [profile] [--json]`.

### 2026-10-15 [Task 03.9] Automated Token Estimation Calibration Against Live Cloud Code Responses

- **Context:** Automated calibration of token estimation heuristics against live Cloud Code and Language Server response metrics, refining the static `(n + 3) / 4` approximation dynamically across models and prompt/completion segments.
- **Active Contracts:**
  - `internal/quota`:
    - `calibration.go`: introduces `TokenCalibrator` with Bayesian prior baseline (`DefaultRunesPerToken = 4.0`, weight: 20 tokens / 80 runes), clamped within `[1.0, 8.0]` runes/token. Tracks overall and per-model statistics (`ModelCalibration`) across prompt and completion segments.
    - Package-level helpers: `CalibrateTokens`, `EstimateTokens` (delegating to active calibrator while preserving exact integer ceiling for uncalibrated baseline), `EstimateTokensForModel`, `EstimateSegmentTokens`, `GetCalibrationStats`, `ResetCalibration`.
    - Persistence: stores calibrated heuristics in `$MULTIGRAVITY_HOME/.multigravity/token-calibration.json` with dynamic path resolution and isolated in-memory test instances.
  - `internal/gateway`:
    - `usage.go`: `resolveUsage` accepts `model` and utilizes segment-aware calibrated estimates (`quota.EstimateSegmentTokens`). In `recordGatewayUsage`, when live `UpstreamUsage` is observed from Cloud Code SSE lines with valid token counts (`!estimated`), triggers `quota.CalibrateTokens(model, promptText, promptTokens, completionText, completionTokens)`.
  - `internal/headless`:
    - `runner.go`: `noteHeadlessTokens` feeds ground-truth tokens observed from `agy` CLI executions into `quota.CalibrateTokens`.
  - `internal/cmd`:
    - `quota.go`: introduces subcommand `multigravity quota calibration [--json]` reporting observation counts, effective ratio, prompt/completion breakdown, cumulative runes/tokens, and per-model ratios.

### 2026-10-15 [Task 03.8] Interactive Terminal Multiplexing Enhancements for Windows PTY

- **Context:** Enhanced pseudo-terminal (PTY) session multiplexing, terminal resize handling, and interactive session attachment across Windows and Unix platforms.
- **Active Contracts:**
  - `internal/agent`:
    - `pty_windows.go`: added ConPTY (`windowsConPTY`) support via `windows.CreatePseudoConsole`, `windows.ResizePseudoConsole`, and `windows.ClosePseudoConsole` with `STARTUPINFOEX` and `PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE`. Gracefully falls back to pipe-backed PTY (`windowsPipePTY`) with dimension tracking if ConPTY is unavailable.
    - `agent_test.go`: cross-platform execution for `TestManagerSessionLifecycle` supporting Windows `cmd.exe` and Unix `sh`, plus `TestSessionMultipleSubscribersAndResize` asserting multiplexed output chunk delivery and dynamic resize propagation across `Manager` and `SessionInstance`.
    - `pty_windows_test.go`: unit tests verifying Windows pipe and ConPTY lifecycle, read/write, and dimension fallback behaviors.
  - `internal/tui`:
    - `multiplexer.go`: decoupled interactive session multiplexing (`MultiplexSession`), handling raw mode (`term.MakeRaw`), stdout streaming, stdin forwarding, and resize coordination.
    - `resize_unix.go`: POSIX resize watcher listening on `syscall.SIGWINCH` and querying `term.GetSize`.
    - `resize_windows.go`: Windows resize watcher monitoring console screen buffer dimension changes (150ms interval) via `term.GetSize`.
    - `multiplexer_test.go`: unit tests for `MultiplexSession` output streaming, stdin forwarding, exit code propagation, and `WatchResize` lifecycle.
  - `internal/cmd`:
    - `agent.go`: refactored `attachInteractive` to delegate cleanly to `tui.MultiplexSession`, eliminating duplicate goroutines and removing obsolete `signal_unix.go` and `signal_windows.go`.

### 2026-10-15 [Task 03.7] Task Retry with Verification Feedback Loop (`--retry <n>`)

- **Context:** Automated task retry loop when post-session verification commands fail (`--verify <cmd>`), injecting verification output into consecutive agent sessions while preserving ephemeral worktrees without polluting branch history.
- **Active Contracts:**
  - `internal/dispatch`:
    - `Task` & `DispatchOptions`: added `Retry int` and `Task.RetryCount int`.
    - `SynthesizeRetryPrompt(originalPrompt, verifyCmd, verifyOutput string, attempt, maxRetries int)`: synthesizes prompt with verification failure header, command, output, and original instructions.
    - `watchSession`: loops across verification failures while `RetryCount < Retry`. Re-resolves command/sandbox, starts consecutive agent sessions in the existing worktree, logs retry banners (`=== Retrying Task (attempt X/Y) ===`), and preserves the same branch/worktree. If retries are exhausted, transitions to `StatusAuditFailed`.
  - `internal/cmd`:
    - `multigravity dispatch run`: adds `--retry <n>` flag. In foreground mode, transparently streams output across consecutive retry sessions. In detached mode, displays `Retry: <n>`.
    - `multigravity dispatch status`: displays `Retry: <n> (used: <count>)` when `task.Retry > 0`.
  - `internal/mcp`:
    - `dispatch_task`: exposes optional `retry` integer parameter (defaults to 0).

### 2026-10-14 [Task 03.6] Governance Context Injection in `dispatch run` (`AGENTS.md` / `TASK.md`)

- **Context:** Automatically inject project governance directives (`AGENTS.md`, `.agent/TASK.md`, `.agent/NOTES.md`) into autonomous dispatch prompts and agent environments.
- **Active Contracts:**
  - `internal/dispatch`:
    - `DiscoverGovernance(searchDirs ...string)`: scans target directory / repo root for `AGENTS.md`, `.agent/TASK.md` (or `TASK.md`), and `.agent/NOTES.md` (or `NOTES.md`). Extracts the active task snippet from `TASK.md` under `## Active Task` (bounded to 1,500 bytes).
    - `SynthesizePrompt(basePrompt string, gov *GovernanceContext)`: prepends non-interfering `[PROJECT GOVERNANCE CONTEXT]` header and active task excerpt to the base prompt; idempotent to prevent duplicate injection.
    - `InjectGovernanceEnv(env map[string]string, gov *GovernanceContext)`: populates `MULTIGRAVITY_GOVERNANCE="true"`, `MULTIGRAVITY_GOVERNANCE_AGENTS`, `MULTIGRAVITY_GOVERNANCE_TASK`, `MULTIGRAVITY_GOVERNANCE_NOTES`.
    - `DispatchOptions` & `Task`: added `NoGovernance bool` and `Governance bool` to allow opt-out and status reporting while preserving original caller `task.Prompt`.
  - `internal/cmd`:
    - `multigravity dispatch run`: adds `--no-governance` flag. Defaults to governance enabled when directives exist.
    - `multigravity dispatch status`: displays `Governance: injected` or `Governance: disabled`.
  - `internal/mcp`:
    - `dispatch_task`: exposes optional `governance` boolean parameter; defaults to raw prompts (`NoGovernance: true`) for MCP external clients.

### 2026-10-14 [Task 03.5] Verification and Audit Hook in Dispatch (`--verify <cmd>`)

- **Context:** Execute automated post-session verification commands (tests, linters) within ephemeral worktrees before tasks are marked completed or eligible for merge.
- **Active Contracts:**
  - `internal/dispatch`:
    - `TaskStatus`: added `StatusAuditFailed TaskStatus = "audit_failed"`.
    - `Task` & `DispatchOptions`: added `Verify`, `VerificationOutput`, and `VerificationExitCode`.
    - `watchSession`: keeps task running after agent session exit code 0 until `executeVerifyCommand` executes inside `task.WorktreePath`. If exit code != 0 or exec error, task is marked `StatusAuditFailed`, recording output and error without marking completed.
    - `ApplyTask`: rejects any task not in `StatusCompleted`, strictly preventing merging unverified or audit-failed worktrees.
    - `WaitForTask`: recognizes `StatusAuditFailed` as a terminal status.
    - `TaskDashboardSummary`: tracks `AuditFailed` count.
  - `internal/cmd`:
    - `multigravity dispatch run`: adds `--verify <cmd>` flag. In foreground mode, reports verification result or audit failure details with exit code error. In detached mode, prints verification command and records output.
    - `multigravity dispatch status`: displays `Verify Cmd`, `Verify Exit`, and `Verify Out`.
    - `multigravity dispatch list` & `dashboard`: visualizes `audit_failed` status in red (`✗ audit_failed`).
  - `internal/mcp`:
    - `dispatch_task` tool schema exposes optional `verify` property.

### 2026-10-14 [Task 03.4] Quota-Aware Route Recommendation (`multigravity route`)

- **Context:** Provide deterministic profile and model recommendations based on measured quotas, reset windows, and task complexity.
- **Active Contracts:**
  - `internal/quota`:
    - `RouteWeight`: `light`, `medium`, `heavy`. `ParseRouteWeight` normalizes with fallback to `medium`.
    - `ModelForWeight`: maps `light` -> `gemini-2.5-flash`, `medium` -> `gemini-3.8-flash-tiered`, `heavy` -> `gemini-2.5-pro`.
    - `RecommendRoute`: evaluates live active servers across 5h and weekly windows. Enforces critical quota guard ($\le 0.05$ ineligible), 5h burst bonus for light tasks, and sustained weekly quota thresholds for heavy tasks. Sorts candidates deterministically by eligibility, score, remaining fraction, reset time, and profile name.
  - `internal/cmd`:
    - `multigravity route [--weight light|medium|heavy] [--json]` renders human-readable recommendation block and candidate status table, or structured JSON contract with full security sanitization (zero CSRF tokens).

### 2026-10-14 [Task 03.3] Agent Process Sandboxing (`agent-guard wrap --repo`)

- **Context:** Enforce filesystem and execution boundaries for runners lacking native sandbox flags while preserving native sandbox handling for runners like Codex.
- **Active Contracts:**
  - `internal/agent`:
    - `SupportsNativeSandbox`: returns `true` for `codex`, `false` for others (`claude`, `aider`, `opencode`, `agy`, `custom`).
    - `WrapWithAgentGuard`: synthesizes `agent-guard wrap --role <role> --repo <repoPath> -- <command> [args...]` with default role `developer`. Avoids double-wrapping.
    - `ValidatePathWithinRepo` & `InterceptSubshellMutation`: parses subshell command chains (`;`, `&&`, `||`, `|`, `\n`) and flags, blocking mutation commands (`rm`, `touch`, `cp`, `mv`, `mkdir`, `rmdir`, `chmod`, `chown`, `truncate`, `sed -i`, `tee`, `git -C`, `git --git-dir`, `git --work-tree`, `>`, `>>`) that target paths outside repository root.
    - `StartSession`: wraps commands with `agent-guard wrap` when `opts.Sandbox` is true for non-native runners.
  - `internal/dispatch`:
    - `CreateTask`: when `opts.Sandbox` is true, non-native runners are wrapped with `WrapWithAgentGuard`, while native runners (`codex`) use native `--sandbox workspace-write`. Propagates `Sandbox` and `Role` to task records and session options.
  - `internal/cmd`:
    - `dispatch run` supports `--sandbox`.
    - `agent run` supports `--sandbox` and `--repo`.
    - `multigravity agent-guard wrap --role <role> --repo <repo> -- <cmd> [args...]` provides native supervisor execution and subshell mutation validation. Direct binary invocation dispatched if binary is named/symlinked as `agent-guard`.

### 2026-10-14 [Task 03.2] Dispatch Apply (`multigravity dispatch apply <task-id>`)

- **Context:** Parity with `codex apply`, merging audited worktree diffs into base repository.
- **Active Contracts:**
  - `internal/worktree`: `GitApply` applies unified diffs to working trees via `git apply --whitespace=nowarn [-3] [--check] -`. Enforces trailing newline requirement on incoming patches to avoid patch corruption.
  - `internal/dispatch`: `ApplyTask` validates `StatusCompleted`, pre-flights diff with `git apply --check`, applies cleanly into `repoRoot`, prunes ephemeral worktrees upon success (unless `--no-prune` or `--dry-run`), and marks `task.Metadata["applied"] = "true"`.
  - `internal/cmd`: `dispatch apply <task-id>` supports `--dry-run`, `--no-prune`, `--3way`, `--repo`, and `--json`.

### 2026-10-14 [Task 03.1] Native Codex CLI Driver in Dispatch and Agent

- **Context:** Official support for OpenAI Codex CLI (`codex`) as a runner in `dispatch` and `agent`.
- **Active Contracts:**
  - `internal/agent`: `DetectAgentType` identifies `codex`. `BuildAgentEnv` isolates `CODEX_HOME` in `<profileDir>/.codex` and injects `OPENAI_BASE_URL` and `OPENAI_API_KEY` directed to multigravity gateway.
  - `internal/dispatch`: `ResolveAgentCommandWithCwd` synthesizes `codex exec -C <cwd> --sandbox workspace-write "<prompt>"` for autonomous tasks. Explicit user flags are preserved without duplication. Worktrees pass `-C <worktreePath>`.
  - MCP schema `dispatch_task` supports runner `'codex'`.

### 2026-10-14 [AI Archival & Project Mapping] ConnectRPC Annotation and Project UUID Alignment

- **Context:** Archived chats did not reappear in Antigravity IDE UI upon direct `.pbtxt` editing due to in-memory caching.
- **Active Contracts:**
  - Running instances require `POST /exa.language_server_pb.LanguageServerService/UpdateConversationAnnotations` ConnectRPC calls authenticated with `X-Codeium-Csrf-Token`.
  - Offline sync requires updating both `annotations/<cid>.pbtxt` and `agyhub_summaries_proto.pb`.
  - `ai sync` updates the 36-byte UUID `projectId` in SQLite metadata blobs and `.pb` summaries to match the target workspace UUID.

### 2026-10-04 [Task 90.1] Unified Dual Route Registration

- **Contract:** `registerPair` in `internal/server/routes.go` binds every handler to `/api/v1{suffix}` and `/api{suffix}`.
- **Exceptions:** `GET /events` (with twin `GET /api/v1/events`), and gateway routes (`/v1/chat/completions`, `/v1/messages`, `/v1/models`).

### 2026-10-04 [Task 00.2] Gateway Unauthenticated Failover

- **Contract:** Exhausting the profile pool due to unauthenticated profiles returns HTTP 401 (`profile_unauthenticated` for OpenAI, `authentication_error` for Anthropic), never 429. The upstream provider is not invoked.

### 2026-10-04 [Task 02.1] Runner Selection Decision Tree

- **Contract:** Dispatch into worktree for diffs $\rightarrow$ `dispatch`. PTY interactive session $\rightarrow$ `agent run`. Identical prompt across $\ge 2$ profiles with report $\rightarrow$ `exec`. Advancing quota window $\rightarrow$ `prime`. Single prompt & profile $\rightarrow$ `headless run`.

---

## Consolidated Historical Decisions

Previous architectural decisions from releases v1.4.0 through v2.2.0 have been consolidated into [`.agent/INVARIANTS.md`](INVARIANTS.md) and [`docs/adr/decisions.md`](../docs/adr/decisions.md), with task history recorded in [`.agent/ARCHIVE.md`](ARCHIVE.md).
