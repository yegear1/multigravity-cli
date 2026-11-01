---
author_type: human
author: Architecture Team
reviewed_by: Human Lead
date: 2026-10-14
status: accepted
type: architectural-decision-log
---

# Architectural Decisions Log

> 🏛️ **Progressive Disclosure:** Historical and cumulative decisions live here so AI agents do not load them into working context on every interaction.
> Active runtime contracts stay in [`.agent/NOTES.md`](../../.agent/NOTES.md). Formal individual specs live in [`.agent/adr/`](../../.agent/adr/).

---

## How to Record Architectural Decisions

1. When a substantive design decision or trade-off is approved, append it here chronologically.
2. In `.agent/NOTES.md`, maintain only **Active Contracts** and link to this file for background rationale.
3. Keep specifications for future complex backlog items documented here to prevent bloating `.agent/TASK.md`.

---

## Cumulative Decisions

### 2026-11-01 [Subagent Wave Orchestration Hardening: Timeout Process Termination, Worktree .venv/PYTHONPATH Seeding, and Diagnostic Tails]

- **Context:** Real-world multi-account parallel wave execution (`multigravity dispatch plan` via `task-queue`) exposed five operational bottlenecks: (1) `agy -p` with `gemini-3.8-flash-medium` and unrestricted thinking generated ~20,000 thinking tokens in a single turn and ran redundant baseline tests before creating files, exceeding 3m timeouts; (2) when `ExecutePlan` hit `context.DeadlineExceeded`, `WaitForTask` detached without terminating the running child session, and subsequent cross-process `multigravity dispatch cancel` calls failed to kill `task.PID` when `m.agentMgr` had no in-memory session, leaving orphaned `agy` processes that later mutated the base working tree; (3) ephemeral git worktrees (`.multigravity/worktrees/<id>`) lacked access to the base repository's `.venv` and `node_modules`, and Python editable installs (`.pth`) inside `<repo>/.venv` shadowed `<worktree>/src` unless `PYTHONPATH` prioritized the worktree; (4) `SubtaskResult` omitted `VerificationOutput`, `VerificationExitCode`, and `LogTail`, forcing orchestrator agents to burn tokens inspecting raw logs; and (5) `multigravity route` model IDs needed alignment with current `agy models` CLI identifiers (`gemini-3.8-flash-low`, `gemini-3.8-flash-medium`, `gemini-3.8-flash-high`, `claude-sonnet-4-6`).
- **Decision:**
  - `internal/profile/env.go` & `internal/agent/env.go`: Introduce `PopulateWorktreeProjectEnv(envMap map[string]string, baseRepoPath, worktreePath string)` to automatically inject `UV_PROJECT_ENVIRONMENT`, `VIRTUAL_ENV`, `<baseRepo>/.venv/bin` into `PATH`, and `<worktree>/src:<worktree>` into `PYTHONPATH` (plus `NODE_PATH` and `<baseRepo>/node_modules/.bin` for Node projects) whenever an agent session runs inside an ephemeral worktree.
  - `internal/dispatch/plan.go` & `internal/dispatch/manager.go`: Ensure `ExecutePlan` forcibly cancels any still-running subtask (`m.CancelTask(repoRoot, taskID, true)`) when `planCtx` expires, add OS PID termination fallback in `CancelTask` for cross-process CLI invocations, and populate `VerificationExitCode`, bounded `VerificationOutput`, and bounded `LogTail` on `SubtaskResult`.
  - `internal/quota/routing.go`: Align `ModelForWeight` and route recommendations with live `agy` CLI model identifiers and expose a recommended `Effort` level (`low`, `medium`, `high`).
  - See [`.agent/knowledge/k-002-dispatch-plan-worktree-orphan-and-venv-isolation.md`](../../.agent/knowledge/k-002-dispatch-plan-worktree-orphan-and-venv-isolation.md).

### 2026-10-23 [Linux D-Bus Keyring Bypass in Antigravity `language_server`]

- **Context:** Antigravity's Go `language_server` (`codeassistclient.NewCompositeTokenStorage`) does not read `GEMINI_FORCE_FILE_STORAGE`. On Linux desktops with `gnome-keyring-daemon` (`org.freedesktop.secrets`), it stores OAuth tokens in the session-wide keyring under a single global service key (`"antigravity"`), causing any newly launched IDE profile to load the last account that signed in across the OS session.
- **Decision:**
  - Refresh `<profileDir>/.gemini/cache/antigravity-keyring-unavailable` during `EnsureProfileLayout` and headless startup (`keyringRecentlyUnavailable` check).
  - Export `GEMINI_FORCE_FILE_STORAGE=true` unconditionally for all isolated profile environments, and on Linux export `WSL_INTEROP=/dev/null` (when unset) so `wslDetector.Detect()` inside `language_server`'s `shouldBypassKeyring()` permanently forces file-based token storage (`<profileDir>/.gemini/jetski-standalone-oauth-token`) without disrupting D-Bus desktop portals (`xdg-open`, native file pickers).

### 2026-10-14 [Codex CLI Runner Integration]

- **Context:** Need to support OpenAI Codex CLI (`codex`) as a first-class runner alongside `agy` for headless autonomous tasks and interactive agent sessions.
- **Decision:**
  - `internal/agent`: `DetectAgentType` identifies `codex` binaries; `BuildAgentEnv` isolates `CODEX_HOME` in `<profileDir>/.codex` and injects `OPENAI_BASE_URL` pointing to the local multigravity gateway.
  - `internal/dispatch`: `ResolveAgentCommandWithCwd` synthesizes `codex exec -C <cwd> --sandbox workspace-write "<prompt>"` for autonomous tasks. Respects explicit user flags without duplication.
  - Worktree integration: ephemeral git worktree path is injected into `-C <worktreePath>` ensuring confinement.

### 2026-10-14 [ConnectRPC AI Archival & Project UUID Mapping]

- **Context:** Conversations unarchived or synchronized across profiles (`ai sync`) were not appearing in the Antigravity IDE UI because the Language Server maintains an in-memory cache and binary protobuf summaries (`agyhub_summaries_proto.pb`).
- **Decision:**
  - Archiving/unarchiving while the profile is running must invoke `POST /exa.language_server_pb.LanguageServerService/UpdateConversationAnnotations` over ConnectRPC using the local CSRF token.
  - Project ID in Antigravity conversation blobs is a 36-byte UUID v4 string. Direct byte-level replacement preserves protobuf wire integrity while matching the destination profile's workspace mapping.

### 2026-10-04 [Single Registration for Dual HTTP Route Prefixes]

- **Context:** Server routes were duplicated across `/api/v1/...` and `/api/...` (100+ manual registrations).
- **Decision:** Introduced `registerPair` helper in `internal/server/routes.go` to register both `/api/v1` and legacy `/api` aliases simultaneously. Canonical SSE events remain on `GET /events` and gateway completions on `/v1/chat/completions` and `/v1/messages`.

### 2026-09-27 [Native Stdio & HTTP MCP Server]

- **Context:** External agent tools (Claude Desktop, Cursor, Aider, Antigravity) needed native MCP protocol support for worktrees, tasks, quotas, and profile diagnostics.
- **Decision:** Built `internal/mcp` implementing JSON-RPC 2.0 with MCP specification `2024-11-05`. Stdio transport reserves `stdout` exclusively for single-line JSON-RPC messages and routes logs to `stderr`. HTTP transport supports both stateless `POST /mcp` and stateful SSE streams.

### 2026-09-26 [Headless Daemon & Vault Isolation]

- **Context:** Headless execution and multi-profile priming required running language servers and CLI prompts without graphical Electron windows and without altering OS keyrings.
- **Decision:** Built `internal/headless` manager. Direct OAuth2 PKCE login saves tokens in `<profileDir>/.gemini/antigravity-cli/antigravity-oauth-token` (0600 mode) and sets `GEMINI_FORCE_FILE_STORAGE=true`. The OS keychain/credential manager is never modified, enabling concurrent multi-profile execution.

---

## Architectural Roadmaps & Backlog Task Specifications

### [Spec 03.2] Dispatch Apply (`multigravity dispatch apply <task-id>`)
- **Objective:** Provide parity with `codex apply`, cleanly merging audited worktree diffs into the target base branch.
- **Mechanics:**
  1. Validate that the task status is `completed` and the diff is clean.
  2. Execute controlled patch application (`git apply` or cherry-pick) on the base repository working tree.
  3. Prune ephemeral worktree upon success (`multigravity worktree prune`).

### [Spec 03.3] Agent Process Sandboxing (`agent-guard wrap --repo`)
- **Objective:** Enforce filesystem and execution boundaries for runners lacking native sandbox flags.
- **Mechanics:**
  - Wrap agent execution with `agent-guard wrap --role developer --repo <path> -- <cmd>` when `--sandbox` or `--repo` is specified.
  - Intercept subshell commands attempting destructive mutations outside the repo tree.

### [Spec 03.4] Quota-Aware Route Recommendation (`multigravity route`)
- **Objective:** Provide deterministic profile and model recommendations based on measured quotas and reset windows.
- **Mechanics:**
  - CLI command `multigravity route [--weight light|medium|heavy] [--json]`.
  - Evaluates remaining fractions and reset times across active profiles.

### [Spec 03.5] Dispatch Post-Verification Hook (`--verify <cmd>`)
- **Objective:** Run validation commands (e.g. tests or linters) inside the worktree before marking a task ready.
- **Mechanics:**
  - Flag `--verify <cmd>` in `dispatch run`. If verification exits non-zero, mark task as `audit_failed` and capture output.

### [Spec 03.6] Governance Context Injection
- **Objective:** Automatically inject governance awareness (`AGENTS.md`, `.agent/TASK.md`) into agent prompts when dispatched into target repositories.

### [Spec 03.7] Task Retry with Verification Feedback Loop (`--retry <n>`)
- **Objective:** Automatically retry tasks when post-session verification commands fail, feeding verification failure output into subsequent prompt sessions without polluting branch history.
- **Mechanics:**
  - Flag `--retry <n>` in `dispatch run` and `Retry` in `DispatchOptions` / `Task`.
  - Reuses the existing ephemeral git worktree and branch in place across retry attempts.
  - Generates feedback prompts via `SynthesizeRetryPrompt` embedding the verification command, exit code, failure output, and original task instructions.
  - Updates `Task.RetryCount`, logs retry markers (`=== Retrying Task (attempt X/Y) ===`), and exposes live streaming across retry sessions.

### [Spec 05.2] Ephemeral Worktree Host Project Environment Seeding (`.venv`, `PYTHONPATH`, `NODE_PATH`)
- **Objective:** Eliminate manual `UV_PROJECT_ENVIRONMENT` and `PYTHONPATH` boilerplate in orchestrator `Verify:` commands when running Python (`uv`/`pytest`/`ruff`/`mypy`) or Node (`npm`/`vitest`) validation inside `.multigravity/worktrees/<id>`.
- **Mechanics:**
  1. Add `PopulateWorktreeProjectEnv(envMap map[string]string, baseRepoPath, worktreePath string)` in `internal/profile/env.go`.
  2. When `<baseRepoPath>/.venv` exists, set `UV_PROJECT_ENVIRONMENT` and `VIRTUAL_ENV` (if unset), prepend `<baseRepoPath>/.venv/bin` (or `Scripts` on Windows) to `PATH`, and prepend `<worktreePath>/src` (if present) and `<worktreePath>` to `PYTHONPATH` so worktree modules always precede editable `.pth` links in `<baseRepoPath>/.venv`.
  3. When `<baseRepoPath>/node_modules` exists, set `NODE_PATH=<baseRepoPath>/node_modules` (if unset) and prepend `<baseRepoPath>/node_modules/.bin` to `PATH`.
  4. Invoke `PopulateWorktreeProjectEnv` from `BuildAgentEnv` (`internal/agent/env.go`) when `opts.WorktreeID != ""` and `opts.Cwd` / `opts.RepoPath` resolve a worktree inside `<baseRepoPath>/.multigravity/worktrees/`.

### [Spec 05.3] Plan Timeout Process-Tree Termination, Cross-Process PID Cancellation & Diagnostic Tail (`SubtaskResult`)
- **Objective:** Prevent orphaned `agy` / `language_server` processes from surviving `multigravity dispatch plan --timeout` and surface compact failure diagnostics directly in `SubtaskResult` JSON.
- **Mechanics:**
  1. In `ExecutePlan` (`internal/dispatch/plan.go`), if `planCtx.Err() != nil` or `completedTask.Status == StatusRunning` after `WaitForTask`, call `m.CancelTask(repoRoot, taskID, true)` and still capture any partial worktree diff before returning.
  2. In `CancelTask` (`internal/dispatch/manager.go`), if `m.agentMgr.KillSession(task.SessionID)` or `StopSession` returns an error (e.g., cross-process CLI invocation where `m.agentMgr` has no in-memory session) and `task.PID > 0`, kill the OS process (`os.FindProcess(task.PID)`).
  3. In `defaultExecuteVerifyCommand` (`internal/dispatch/manager.go`), when `dir` is inside `<repoRoot>/.multigravity/worktrees/<id>`, inject `PopulateWorktreeProjectEnv` into `cmd.Env` so post-session `--verify` commands also resolve `.venv` and `PYTHONPATH` automatically.
  4. Extend `SubtaskResult` (`internal/dispatch/plan.go`) backward-compatibly with `VerificationExitCode int json:"verification_exit_code,omitempty"`, `VerificationOutput string json:"verification_output,omitempty"` (last 1000 bytes), and `LogTail string json:"log_tail,omitempty"` (last 1000 bytes on failure/timeout).

### [Spec 05.4] Route Model Identifier Alignment & Reasoning Effort Recommendation (`multigravity route`)
- **Objective:** Align `multigravity route` model recommendations with active `agy models` CLI identifiers and recommend `--effort` flags to prevent 20k-token thinking loops on micro-contract tasks.
- **Mechanics:**
  1. Update `ModelForWeight` in `internal/quota/routing.go`: `light` -> `gemini-3.8-flash-low`, `medium` -> `gemini-3.8-flash-medium`, `heavy` -> `gemini-3.8-flash-high`.
  2. Add `EffortForWeight(w RouteWeight) string` (`light` -> `low`, `medium` -> `medium`, `heavy` -> `high`) and include `RecommendedEffort string json:"recommended_effort,omitempty"` in `RouteRecommendation`.

### [Spec 05.5] Synchronize `multigravity-orchestrator` Skill with Micro-Contract & Zero-Diff-Bloat Wave Patterns
- **Objective:** Update `skills/multigravity-orchestrator/SKILL.md` to document `Read:/Paths:/Verify:` micro-contracts, `--effort low` / `gemini-3.8-flash-low` defaults for scoped subtasks, and bounded `verification_output`/`log_tail` inspection.

### [Spec 05.6] Immutable SHA `BaseCommit` Resolution & Path-Scoped `GitDiff` (`internal/worktree`)
- **Objective:** Prevent reverse-patch corruption during parallel wave commits when `BaseCommit` is passed as a symbolic branch name (e.g. `"main"`), and allow scoping worktree diffs strictly to declared `Paths`.
- **Mechanics:**
  1. In `CreateWorktree` (`internal/worktree/manager.go`), after `GitWorktreeAdd` creates the worktree at `absTargetDir`, if `headCommit != ""` set `wt.BaseCommit = headCommit` (the resolved immutable commit SHA) rather than storing a symbolic branch name like `"main"` that advances as sibling wave tasks commit.
  2. Extend `DiffOptions` (`internal/worktree/types.go`) with `Paths []string json:"paths,omitempty"`.
  3. Update `GitDiff` (`internal/worktree/git.go`) and `GetWorktreeDiff` (`internal/worktree/manager.go`) to accept optional `paths ...string` (or `opts.Paths`) so `git diff` and untracked file collection (`ls-files --others --exclude-standard`) filter strictly to matching paths when specified.

### [Spec 05.7] Secondary-Account Quota Auto-Selection (`SelectEligibleProfiles` in `internal/quota`)
- **Objective:** Enable `multigravity dispatch plan` to automatically route subtasks across secondary Antigravity accounts with $> 5\%$ quota while preserving the active IDE account's quota for human planning.
- **Mechanics:**
  1. Add `SelectEligibleProfiles(count int, weight RouteWeight, excludeProfiles ...string) ([]string, error)` (and testable `SelectEligibleProfilesFromCandidates(candidates []RouteCandidate, count int, excludeProfiles ...string) []string`) in `internal/quota/routing.go`.
  2. Partition eligible candidates (`Eligible == true`, i.e., `RemainingFraction > 0.05`) into non-excluded (secondary) profiles first, sorted by `RecommendRoute` score, followed by excluded profiles as a fallback only when fewer than `count` secondary profiles are eligible.

### [Spec 05.8] Native Micro-Contracts, `agy` Model/Effort Normalization, Auto-Profile Routing & Atomic Wave Commits (`internal/dispatch` & `internal/cmd`)
- **Objective:** Eliminate external bash/python wrapper overhead (`run-wave.sh`) and reduce worker/orchestrator token consumption by building micro-contract prompt synthesis, `agy` flag normalization,Verify-on-Timeout recovery, path-scoped atomic commits, and zero-diff-bloat JSON output directly into `multigravity dispatch plan`.
- **Mechanics:**
  1. **`agy` `--model` / `--effort` Flag Normalization (`internal/dispatch/manager.go`):** In `ResolveAgentCommandWithCwd`, when the runner is `agy` and `args` contain both `--model <m>` (with `-low`, `-medium`, or `-high` suffix) and `--effort <e>`, normalize the model suffix to `<e>` and strip the redundant `--effort <e>` flag so `agy` never aborts with `conflicts with --effort=...`.
  2. **Micro-Contract Fields & Fast-Edit Prompt Synthesis (`internal/dispatch/plan.go`):** Extend `SubtaskSpec` with `Read []string`, `Paths []string`, `CommitMessage string`, and `Model string`, and `PlanRequest` with `DefaultModel string`, `ApplyAtomic bool`, and `OmitDiff bool`. When `st.Read` or `st.Paths` are non-empty, default `NoGovernance: true` on `DispatchOptions` (avoiding duplicate `AGENTS.md`/`TASK.md` injection) and synthesize a compact micro-contract prompt (`READ ONLY`, `ALLOWED MUTABLE PATHS`, `Do NOT run baseline tests before editing`, `Exit immediately once validation passes without writing a prose summary`).
  3. **Auto-Profile Routing:** In `ExecutePlan`, when `st.Profile == ""` or `"auto"`, automatically resolve eligible secondary profiles via `quota.SelectEligibleProfiles` (detecting current profile from `HOME` under `AntigravityProfiles/<name>`) and assign subtasks in round-robin.
  4. **Verify-on-Timeout, `--apply-atomic` & `--omit-diff`:** When `req.ApplyAtomic` is true, if a subtask timed out or completed, run `st.Verify` in the worktree if needed, extract the diff against the worktree's `BaseCommit` (`HEAD` at creation) filtered by `st.Paths`, apply to `repoRoot`, stage `st.Paths`, create an atomic `git commit -m <CommitMessage>`, populate `Commit string` on `SubtaskResult`, and remove the ephemeral worktree and branch. When `req.OmitDiff` is true, omit raw unified diffs from `SubtaskResult.Diff` and `UnifiedSummary.CombinedDiff`.
  5. **CLI Flags (`internal/cmd/dispatch.go`):** Add `--apply-atomic` and `--omit-diff` flags to `multigravity dispatch plan`.
