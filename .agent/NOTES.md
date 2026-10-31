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
