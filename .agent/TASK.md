# TASK.md — Tasks and Roadmap for Legacy Codebase

> Defines WHAT needs to be done. Detailed history lives in `git log` and `ARCHIVE.md`.
> Specifications for future backlog tasks live in `docs/adr/decisions.md`.

---

## Active Task

### [06.6] Line and size bounds on InlineRead injection
- Status: COMPLETED
- Priority: P1
- Model: `gemini-3.8-flash-medium`
- Read: `internal/dispatch/plan.go:200-280`
- Paths: `internal/dispatch/plan.go, internal/dispatch/plan_test.go`
- Verify: `go test -v ./internal/dispatch -run TestSynthesizeSubtaskPromptWithRepoSizeBounds`
- Description: Cap non-ranged `Read:` entries to 150 lines and 5 KB.

---

## Completed Tasks Log

| Task | Title | Commit(s) | Date |
|---|---|---|---|
| [06.6] | Line and size bounds on `InlineRead` injection (cap non-ranged `Read:` entries to 150 lines and 5 KB) | `92663c3` | 2026-11-01 |
| [06.5] | Profile-aware concurrency throttling (clamp workers to `min(req.Workers, activeEligibleProfiles)` to prevent single-profile starvation) | `6dbb07f` | 2026-11-01 |
| [06.3] | Verify-on-Timeout recovery in Go plan executor (run `Verify:` on deadline if diff exists; recover as `completed` on exit 0) | `eed33f1` | 2026-11-01 |
| [06.4] | Default model fallback before wave linting (prevent un-modelled tasks from falsely failing `TierMicro` path/package rules) | `6bb3b0a` | 2026-11-01 |
| [06.1] | Default compact JSON output and persist run artifacts (`.multigravity/runs/<plan_id>.json`, `--full` flag for verbose JSON) | [`da17053`](https://github.com/yegear1/multigravity-cli/commit/da17053) | 2026-11-01 |
| [06.2] | Auto-symlink ignored build dependencies in ephemeral worktrees (`node_modules`, `target`, `.venv`, `.cargo`) | [`eee44eb`](https://github.com/yegear1/multigravity-cli/commit/eee44eb) | 2026-11-01 |
| [05.27] | Synchronize `skills/multigravity-orchestrator/SKILL.md` and `task-queue` with installed-runner auto-discovery and `quota-history.jsonl` persistent 429 cooldowns | [`9d9855b`](https://github.com/yegear1/multigravity-cli/commit/9d9855b) | 2026-11-01 |
| [05.26] | Persist and reload profile 429 cooldowns via `<profileDir>/.multigravity/quota-history.jsonl` (`Sample.CooldownUntil`, `SourceCooldown = "cooldown"`) and parse `"Resets in ..."` durations (`ParseQuotaResetDuration`) | [`cb2f996`](https://github.com/yegear1/multigravity-cli/commit/cb2f996) | 2026-11-01 |
| [05.25] | Skip uninstalled agent runners in `ResolveAgentCommandWithCwd` / `ExecutePlan` via `DiscoverInstalledAgent` (`claude`, `agy`, `codex`, `opencode`, `aider`) | [`0d0985a`](https://github.com/yegear1/multigravity-cli/commit/0d0985a) | 2026-11-01 |
| [05.24] | Synchronize `README.md` and `README.pt-br.md` command tables and usage sections with `multigravity dispatch wave` (`--lint`), `dispatch apply`, `dispatch plan` (`--apply-atomic`, `--omit-diff`), `multigravity route` (`--task`, `--paths`, `--read`), and `quota` subcommands (`burn`, `throttle`, `calibration`) | [`dc116fd`](https://github.com/yegear1/multigravity-cli/commit/dc116fd) | 2026-11-01 |
| [05.23] | Update `skills/multigravity-orchestrator/SKILL.md` and `skills/multigravity/SKILL.md` in Technical English with Epic 05 (`05.14`–`05.22`) contracts | [`472433c`](https://github.com/yegear1/multigravity-cli/commit/472433c) | 2026-11-01 |
| [05.22] | Set `DefaultSafetyQuotaThreshold = 0.01` (1% stable quota floor), add `SafetyThresholdForModel`, and wire `PreflightProfilesForModel` into `ExecutePlan` and `ExecuteTaskWave` with fallback to `0.01` floor | [`a7c815e`](https://github.com/yegear1/multigravity-cli/commit/a7c815e) | 2026-11-01 |
| [05.18] | Add `LintTaskWave` in `internal/dispatch/wave.go` and `--lint` flag to `multigravity dispatch wave` in `internal/cmd/dispatch.go` | [`7637c47`](https://github.com/yegear1/multigravity-cli/commit/7637c47) | 2026-11-01 |
| [05.19] | Clean governance tags (`[APPROVED]`, `[PARALLEL: Wx]`, `[SEQ: Wx]`) from `WaveTaskSpec.Title` in `ParseTaskWave` | [`3efaa40`](https://github.com/yegear1/multigravity-cli/commit/3efaa40) | 2026-11-01 |
| [05.21] | Include `Verify:` in `SynthesizeSubtaskPromptWithRepo`, pre-inject trailing lines of sibling `*_test.go` files within residual `maxBudget`, and record `TokensUsed` in `SubtaskResult` | [`1524e9d`](https://github.com/yegear1/multigravity-cli/commit/1524e9d) | 2026-11-01 |
| [05.20] | Wire `ClassifyTaskComplexity` into `SynthesizeSubtaskPromptWithRepo` / `ExecutePlan` and pass `--model <resolvedModel>` into `dOpts.Args` | [`9e27365`](https://github.com/yegear1/multigravity-cli/commit/9e27365) | 2026-11-01 |
| [05.17] | Gate model escalation in `ExecuteTaskWave` on `res.FailureReason` (`rate_limited`, `verify_failed`, `empty_diff`/`timeout`) | [`3492921`](https://github.com/yegear1/multigravity-cli/commit/3492921) | 2026-11-01 |
| [05.16] | Add structured `FailureReason string` (`ClassifySubtaskFailure`) in `SubtaskResult` inspecting status, exit code, verification output, and `log_tail` | [`266bec2`](https://github.com/yegear1/multigravity-cli/commit/266bec2) | 2026-11-01 |
| [05.15] | Add quota pre-flight (`DefaultSafetyQuotaThreshold = 0.20`, cooldown tracking, and `PreflightProfilesForModel`) in `internal/quota/routing.go` | [`8be6cae`](https://github.com/yegear1/multigravity-cli/commit/8be6cae) | 2026-11-01 |
| [05.14] | Automate `.agent/TASK.md` promotion and Completed Tasks Log recording in `dispatch wave` | [`eb5f857`](https://github.com/yegear1/multigravity-cli/commit/eb5f857) | 2026-11-01 |
| [05.13.5] | Synchronize `skills/multigravity-orchestrator/SKILL.md` with inference tier matrix, complexity classification, and cascade escalation | [`d6af449`](https://github.com/yegear1/multigravity-cli/commit/d6af449) | 2026-11-01 |
| [05.13.4] | Add `--task`, `--paths`, and `--read` support with inference tier visualization to `multigravity route` in `internal/cmd/route.go` | [`74198d6`](https://github.com/yegear1/multigravity-cli/commit/74198d6) | 2026-11-01 |
| [05.13.3b] | Wire `quota.EscalateModel` and `SynthesizeCascadeRetryPrompt` into `ExecuteTaskWave` in `internal/dispatch/wave.go` and add unit tests in `internal/dispatch/wave_test.go` | [`19248bb`](https://github.com/yegear1/multigravity-cli/commit/19248bb) | 2026-11-01 |
| [05.13.3a] | Add `SynthesizeCascadeRetryPrompt` helper in `internal/dispatch/plan.go` and unit tests in `internal/dispatch/plan_test.go` | [`f4b34d2`](https://github.com/yegear1/multigravity-cli/commit/f4b34d2) | 2026-11-01 |
| [05.13.2] | Implement static zero-token task complexity classifier (`ClassifyTaskComplexity`) in `internal/quota/classifier.go` | [`407eec5`](https://github.com/yegear1/multigravity-cli/commit/407eec5) | 2026-11-01 |
| [05.13.1] | Define canonical inference tiers (`TierMicro`, `TierStandard`, `TierReasoning`, `TierArchitecture`), model metadata ranking matrix, and `EscalateModel(model)` in `internal/quota/tiers.go` | [`65bb810`](https://github.com/yegear1/multigravity-cli/commit/65bb810) | 2026-11-01 |
| [05.12] | Synchronize `skills/multigravity-orchestrator/SKILL.md` with `multigravity dispatch wave`, line-ranged `Read: file:start-end` pre-injection, `gemini-3.8-flash-low` defaults, and model-aware quota routing | [`2b8e717`](https://github.com/yegear1/multigravity-cli/commit/2b8e717) | 2026-11-01 |
| [05.11] | Implement native `.agent/TASK.md` wave parser (`ParseTaskWave`) and `multigravity dispatch wave` CLI subcommand with auto-retry profile exclusion and automatic `TASK.md` completion logging | [`74d1e09`](https://github.com/yegear1/multigravity-cli/commit/74d1e09) | 2026-11-01 |
| [05.10] | Add line-ranged `Read:` pre-injection (`file:start-end`), `gemini-3.8-flash-low` micro-contract default, `empty_diff` `LogTail` audit, model-aware auto-profile routing, and `--apply-atomic` / `--omit-diff` in `internal/dispatch/plan.go` | [`2f53f12`](https://github.com/yegear1/multigravity-cli/commit/2f53f12) | 2026-11-01 |
| [05.8] | Add model-aware quota bucket routing (`BucketGroupForModel`, `ScoreServerForBucket`, `SelectEligibleProfilesForModel`) and prevent missing-group false-positive `1.0` scores in `internal/quota` | [`4bc9bb9`](https://github.com/yegear1/multigravity-cli/commit/4bc9bb9) | 2026-11-01 |
| [05.9] | Normalize `agy` `--model`/`--effort` flag conflicts in `ResolveAgentCommandWithCwd` and reject Go `testing: warning: no tests to run` false-positive `exit code 0` in `defaultExecuteVerifyCommand` | [`1714133`](https://github.com/yegear1/multigravity-cli/commit/1714133) | 2026-11-01 |
| [05.7] | Add `SelectEligibleProfiles` and `SelectEligibleProfilesFromCandidates` with secondary-account priority (`excludeProfiles`) in `internal/quota` | [`a842530`](https://github.com/yegear1/multigravity-cli/commit/a842530) | 2026-11-01 |
| [05.6] | Resolve symbolic `BaseCommit` refs to immutable SHA in `CreateWorktree` and add `Paths` filtering to `DiffOptions` / `GitDiff` | [`080fba6`](https://github.com/yegear1/multigravity-cli/commit/080fba6) | 2026-11-01 |
| [05.5] | Synchronize `multigravity-orchestrator` skill with `Read:/Paths:/Verify:` micro-contracts, tiered model IDs, and bounded diagnostic tail handling | [`d7c7ee8`](https://github.com/yegear1/multigravity-cli/commit/d7c7ee8) | 2026-11-01 |
| [05.4] | Align `multigravity route` model identifiers with live `agy models` (`gemini-3.8-flash-low/medium/high`) and expose `recommended_effort` (`low`/`medium`/`high`) | [`8c50ba4`](https://github.com/yegear1/multigravity-cli/commit/8c50ba4) | 2026-11-01 |
| [05.3] | Terminate running subtasks on `ExecutePlan` timeout, add cross-process PID fallback in `CancelTask`, and expose bounded `verification_output`/`log_tail` in `SubtaskResult` | [`2837a59`](https://github.com/yegear1/multigravity-cli/commit/2837a59) | 2026-11-01 |
| [05.2] | Seed base repository `.venv` (`UV_PROJECT_ENVIRONMENT`, `VIRTUAL_ENV`, `PYTHONPATH` worktree precedence) and `NODE_PATH` into ephemeral worktree agent environments | [`b1e5ae5`](https://github.com/yegear1/multigravity-cli/commit/b1e5ae5) | 2026-11-01 |
| [05.1] | Centralize PATH enrichment, preserve host toolchain variables (CARGO_HOME/RUSTUP_HOME), and diagnose toolchain bleed in doctor | [`3966f07`](https://github.com/yegear1/multigravity-cli/commit/3966f07) | 2026-10-31 |
| [00.6] | Audit and scaffold Open Source Readiness (licenses, community governance, identity sanitization, security policy) | [`0c4e093`](https://github.com/yegear1/multigravity-cli/commit/0c4e093) | 2026-10-31 |
| [04.4] | Enforce per-profile OAuth token file isolation on Linux by bypassing global D-Bus Secret Service keyring in `language_server` | working tree | 2026-10-23 |
| [00.5] | Align repository governance with brownfield template knowledge base schema and documentation standards | working tree | 2026-10-18 |
| [04.3] | Fix Google OAuth token exchange HTTP 400 error by supplying canonical client secret | working tree | 2026-10-15 |
| [04.2] | Task-aware trace telemetry (`--trace-id` flag, `MAESTRI_TRACE_ID` ingestion, VictoriaLogs token delta logging) | [`5035ba2`](https://github.com/ye-dev/multigravity-cli/commit/5035ba2) | 2026-10-17 |
| [04.1] | Dynamic token quota budget alerts and auto-throttle policies | [`257b63f`](https://github.com/ye-dev/multigravity-cli/commit/257b63f) | 2026-10-15 |
| [03.9] | Automated token estimation calibration against live Cloud Code responses | [`1ba96de`](https://github.com/ye-dev/multigravity-cli/commit/1ba96de) | 2026-10-15 |
| [03.8] | Interactive terminal multiplexing enhancements for Windows PTY | [`855a815`](https://github.com/ye-dev/multigravity-cli/commit/855a815) | 2026-10-15 |
| [03.7] | Task retry with verification feedback loop (`--retry <n>`) | [`01da780`](https://github.com/ye-dev/multigravity-cli/commit/01da780) | 2026-10-15 |
| [03.6] | Governance context injection in `dispatch run` (`AGENTS.md` / `TASK.md`) | [`d534be6`](https://github.com/ye-dev/multigravity-cli/commit/d534be6) | 2026-10-14 |
| [03.5] | Verification and audit hook in dispatch (`--verify <cmd>`) | [`d534be6`](https://github.com/ye-dev/multigravity-cli/commit/d534be6) | 2026-10-14 |
| [03.4] | Quota-aware route recommendation CLI command (`multigravity route`) | [`d534be6`](https://github.com/ye-dev/multigravity-cli/commit/d534be6) | 2026-10-14 |
| [03.3] | Agent process sandboxing via wrapper `agent-guard wrap --repo` | [`d534be6`](https://github.com/ye-dev/multigravity-cli/commit/d534be6) | 2026-10-14 |
| [03.2] | Command `multigravity dispatch apply <task-id>` to merge approved worktrees | [`d534be6`](https://github.com/ye-dev/multigravity-cli/commit/d534be6) | 2026-10-14 |
| [00.4] | Align repository governance and agent artifacts with brownfield template | [`d534be6`](https://github.com/ye-dev/multigravity-cli/commit/d534be6) | 2026-10-14 |
| [03.1] | Native Codex CLI driver in dispatch and agent (`--agent codex`, sandbox and `-C`) | [`e8e0547`](https://github.com/ye-dev/multigravity-cli/commit/e8e0547) | 2026-10-14 |
| [00.3] | Archive consolidated NOTES into ARCHIVE.md | working tree | 2026-10-14 |
| [90.1] | Unified registration for dual HTTP route prefixes (`/api/v1` and `/api`) | working tree | 2026-10-04 |
| [00.2] | Gateway test coverage gaps and unauthenticated failover fixes | working tree | 2026-10-04 |
| [02.1] | Agent runner decision tree and documentation alignment | working tree | 2026-10-04 |

---

## Backlog (Upcoming, in priority order)

> Tag pre-approved tasks having strictly disjoint file scopes with `[APPROVED]` `[PARALLEL: W1]` and `Paths: ...` for concurrent `task-queue` execution.

- [x] **[05.6]** `[APPROVED]` `[PARALLEL: W1]` Resolve symbolic `BaseCommit` refs to immutable SHA in `CreateWorktree` and add `Paths` filtering to `DiffOptions` / `GitDiff` (`080fba6`)
- [x] **[05.7]** `[APPROVED]` `[PARALLEL: W1]` Add `SelectEligibleProfiles` and `SelectEligibleProfilesFromCandidates` with secondary-account priority (`excludeProfiles`) in `internal/quota` (`a842530`)
- [x] **[05.8]** `[APPROVED]` `[PARALLEL: W2]` Add model-aware quota bucket routing (`BucketGroupForModel`, `ScoreServerForBucket`, `SelectEligibleProfilesForModel`) and prevent missing-group false-positive `1.0` scores in `internal/quota` *(Point 2.E)* (`4bc9bb9`)
- [x] **[05.9]** `[APPROVED]` `[PARALLEL: W2]` Normalize `agy` `--model`/`--effort` flag conflicts in `ResolveAgentCommandWithCwd` and reject Go `testing: warning: no tests to run` false-positive `exit code 0` in `defaultExecuteVerifyCommand` *(Points 2.C & 2.D)* (`1714133`)
- [x] **[05.10]** `[APPROVED]` `[SEQ: W3]` Add line-ranged `Read:` pre-injection (`file:start-end`), `gemini-3.8-flash-low` micro-contract default with single-grep escape hatch, `empty_diff` `LogTail` audit, model-aware auto-profile routing, and `--apply-atomic` / `--omit-diff` execution in `internal/dispatch/plan.go` *(Points 2.B, 2.C, 2.D, 2.E)* (`2f53f12`)
- [x] **[05.11]** `[APPROVED]` `[SEQ: W4]` Implement native `.agent/TASK.md` wave parser (`ParseTaskWave`) and `multigravity dispatch wave` CLI subcommand with auto-retry profile exclusion and automatic `TASK.md` completion logging *(Point 2.A)* (`74d1e09`)
- [x] **[05.12]** `[APPROVED]` `[SEQ: W5]` Synchronize `skills/multigravity-orchestrator/SKILL.md` with `multigravity dispatch wave`, line-ranged `Read: file:start-end` pre-injection, `gemini-3.8-flash-low` defaults, and model-aware quota routing *(Points 2.A–2.E)* (`2b8e717`)
- [x] **[05.13.1]** `[APPROVED]` `[PARALLEL: W6]` Define canonical inference tiers (`TierMicro`, `TierStandard`, `TierReasoning`, `TierArchitecture`), model metadata ranking matrix, and `EscalateModel(model)` in `internal/quota/tiers.go` (`65bb810`)
- [x] **[05.13.2]** `[APPROVED]` `[PARALLEL: W6]` Implement static zero-token task complexity classifier (`ClassifyTaskComplexity`) in `internal/quota/classifier.go` (`407eec5`)
- [x] **[05.13.3a]** `[APPROVED]` `[SEQ: W7]` Add `SynthesizeCascadeRetryPrompt` helper in `internal/dispatch/plan.go` and unit tests in `internal/dispatch/plan_test.go` (`f4b34d2`)
- [x] **[05.13.3b]** `[APPROVED]` `[SEQ: W7b]` Wire `quota.EscalateModel` and `SynthesizeCascadeRetryPrompt` into `ExecuteTaskWave` in `internal/dispatch/wave.go` and add unit tests in `internal/dispatch/wave_test.go` (`19248bb`)
- [x] **[05.13.4]** `[APPROVED]` `[PARALLEL: W8]` Add `--task`, `--paths`, and `--read` support with inference tier visualization to `multigravity route` in `internal/cmd/route.go` (`74198d6`)
- [x] **[05.13.5]** `[APPROVED]` `[PARALLEL: W8]` Synchronize `skills/multigravity-orchestrator/SKILL.md` with inference tier matrix, complexity classification, and cascade escalation (`d6af449`)

> **Epic 05.14–05.21 — Orchestration token economy.** Derived from the `[05.13]` execution retrospective and architectural review. Grouped into 4 disjoint-file waves (`W9`–`W12`).

- [x] **[05.14]** `[APPROVED]` `[PARALLEL: W9]` Automate `.agent/TASK.md` promotion in `dispatch wave`: prepend completed tasks to `## Completed Tasks Log`, mark `## Active Task` Acceptance Criteria `[x]`, and promote the next uncompleted backlog task into `## Active Task` (`eb5f857`)
- [x] **[05.15]** `[APPROVED]` `[PARALLEL: W9]` Quota pre-flight before dispatch: add `DefaultSafetyQuotaThreshold = 0.20`, cooldown tracking (`RecordProfileCooldown` / `IsProfileOnCooldown`), and `PreflightProfilesForModel(count int, model string, weight RouteWeight, minFraction float64, excludeProfiles ...string)` rejecting profiles below threshold or under active 429 cooldown while preserving existing `ScoreServerForBucket` behavior (`8be6cae`)
- [x] **[05.16]** `[APPROVED]` `[PARALLEL: W9]` Add structured `FailureReason string` (`timeout | rate_limited | empty_diff | verify_failed | agent_error`) in `SubtaskResult` (`ClassifySubtaskFailure`) inspecting status, exit code, verification output, and `log_tail` (429/rate-limit/quota indicators) so empty or terse logs never hide the root cause (`266bec2`)
- [x] **[05.17]** `[APPROVED]` `[PARALLEL: W10]` Gate model escalation in `ExecuteTaskWave` on `res.FailureReason`: on `rate_limited`, record profile cooldown and retry with the **same** model on an alternate profile; on `verify_failed` (with non-empty verification output), escalate via `quota.EscalateModel`; on `empty_diff` or `timeout` with empty diagnostics, skip retry and preserve failure reason (`3492921`)
- [x] **[05.20]** `[APPROVED]` `[PARALLEL: W10]` Wire `ClassifyTaskComplexity` into `SynthesizeSubtaskPromptWithRepo` / `ExecutePlan` when `st.Model == ""` and `defaultModel == ""` (preserving `gemini-3.8-flash-low` for single-path or line-ranged micro-contracts) AND pass `--model <resolvedModel>` into `dOpts.Args` when `st.Command == ""` and `--model` is not already present in `st.Args` (`9e27365`)
- [x] **[05.19]** `[APPROVED]` `[PARALLEL: W11]` Clean governance tags (`[APPROVED]`, `[PARALLEL: Wx]`, `[SEQ: Wx]`) from `WaveTaskSpec.Title` in `ParseTaskWave` so commit messages (`feat(id): <clean title>`) and worker prompts do not leak markdown metadata tags (`3efaa40`)
- [x] **[05.21]** `[APPROVED]` `[PARALLEL: W11]` Include `Verify: <st.Verify>` in `SynthesizeSubtaskPromptWithRepo`, pre-inject trailing lines of sibling `*_test.go` files for `st.Paths` using only residual space within the 15 KB `maxBudget` after all explicit `st.Read` slices are loaded, and record estimated prompt/completion `TokensUsed int` in `SubtaskResult` (`1524e9d`)
- [x] **[05.18]** `[APPROVED]` `[SEQ: W12]` Add `LintTaskWave(waveSpec *WaveSpec) []string` in `internal/dispatch/wave.go` and `--lint` flag to `multigravity dispatch wave` in `internal/cmd/dispatch.go`: report violations when a `TierMicro` (`gemini-3.8-flash-low`) task has `> 2` `Paths:` files or spans multiple Go packages, or when any task has `> 4` `Read:` entries or missing `Verify:`; exit non-zero when violations are found (`7637c47`)
- [x] **[05.22]** `[APPROVED]` `[PARALLEL: W13]` Set `DefaultSafetyQuotaThreshold = 0.01` (1% stable quota floor), add `SafetyThresholdForModel(model string, weight RouteWeight) float64` (`0.15` for `TierReasoning`/`TierArchitecture` or `WeightHeavy`, `0.01` for `TierMicro`/`TierStandard`), and wire `PreflightProfilesForModel` into `ExecutePlan` and `ExecuteTaskWave` with fallback to `0.01` floor (`a7c815e`)
- [x] **[05.23]** `[APPROVED]` `[PARALLEL: W13]` Update `skills/multigravity-orchestrator/SKILL.md` and `skills/multigravity/SKILL.md` in Technical English with Epic 05 (`05.14`–`05.22`) contracts (`dispatch wave --lint`, `failure_reason`-gated retry, 1% quota floor / 15% complex-task threshold, sibling `*_test.go` pre-injection, `tokens_used`, `multigravity route --task/--paths/--read`) (`472433c`)
- [x] **[05.24]** `[APPROVED]` `[PARALLEL: W13]` Synchronize `README.md` and `README.pt-br.md` command tables and usage sections with `multigravity dispatch wave` (`--lint`), `dispatch apply`, `dispatch plan` (`--apply-atomic`, `--omit-diff`), `multigravity route` (`--task`, `--paths`, `--read`), and `quota` subcommands (`burn`, `throttle`, `calibration`) (`dc116fd`)
- [x] **[05.25]** `[APPROVED]` `[PARALLEL: W14]` Skip uninstalled agent runners in `ResolveAgentCommandWithCwd` / `ExecutePlan` by probing installed binaries (`claude`, `agy`, `codex`, `opencode`, `aider`) on enriched `PATH`, and pass `--dangerously-skip-permissions --disable-slash-commands` when `agy` runs autonomously (`0d0985a`)
- [x] **[05.26]** `[APPROVED]` `[PARALLEL: W14]` Persist and reload profile 429 cooldowns via `<profileDir>/.multigravity/quota-history.jsonl` (`Sample.CooldownUntil`, `SourceCooldown = "cooldown"`), add `ParseQuotaResetDuration` (`"Resets in ..."`), and record cooldowns on `rate_limited` subtasks in `ExecutePlan` and `ExecuteTaskWave` (`cb2f996`)
- [x] **[05.27]** `[APPROVED]` `[PARALLEL: W14]` Synchronize `skills/multigravity-orchestrator/SKILL.md` with installed-runner auto-discovery and `quota-history.jsonl` persistent 429 cooldowns (`9d9855b`)

> **Epic 06 — Orchestration Resiliency, Token Economy & Fast-Track Recovery.** Derived from `triage-mail` parallel dispatch wave retrospectives. Grouped into disjoint-file waves (`W1`–`W4`).

- [x] **[06.1]** `[APPROVED]` `[PARALLEL: W1]` Default compact JSON output and persist run artifacts (`.multigravity/runs/<plan_id>.json`, `--full` flag for verbose JSON) (`da17053`)
- [x] **[06.2]** `[APPROVED]` `[PARALLEL: W1]` Auto-symlink ignored build dependencies in ephemeral worktrees (`node_modules`, `target`, `.venv`, `.cargo`) (`eee44eb`)
- [x] **[06.3]** `[APPROVED]` `[PARALLEL: W2]` Verify-on-Timeout recovery in Go plan executor (run `Verify:` on deadline if diff exists; recover as `completed` on exit 0) (eed33f1)
  - `Model:` `gemini-3.8-flash-medium`
  - `Read:` `internal/dispatch/plan.go:500-600`
  - `Paths:` `internal/dispatch/plan.go, internal/dispatch/plan_test.go`
  - `Verify:` `go test -v ./internal/dispatch -run TestExecutePlanVerifyOnTimeout`
- [x] **[06.4]** `[APPROVED]` `[PARALLEL: W2]` Default model fallback before wave linting (prevent un-modelled tasks from falsely failing `TierMicro` path/package rules) (6bb3b0a)
  - `Model:` `gemini-3.8-flash-medium`
  - `Read:` `internal/dispatch/wave.go:230-265`
  - `Paths:` `internal/dispatch/wave.go, internal/dispatch/wave_test.go`
  - `Verify:` `go test -v ./internal/dispatch -run TestLintTaskWaveDefaultModel`
- [x] **[06.5]** `[APPROVED]` `[SEQ: W3]` Profile-aware concurrency throttling (clamp workers to `min(req.Workers, activeEligibleProfiles)` to prevent single-profile starvation) (6dbb07f)
  - `Model:` `gemini-3.8-flash-medium`
  - `Read:` `internal/dispatch/plan.go:490-545`
  - `Paths:` `internal/dispatch/plan.go, internal/dispatch/plan_test.go`
  - `Verify:` `go test -v ./internal/dispatch -run TestProfileAwareConcurrency`
- [x] **[06.6]** `[APPROVED]` `[SEQ: W4]` Line and size bounds on `InlineRead` injection (cap non-ranged `Read:` entries to 150 lines and 5 KB) (`92663c3`)
  - `Model:` `gemini-3.8-flash-medium`
  - `Read:` `internal/dispatch/plan.go:200-280`
  - `Paths:` `internal/dispatch/plan.go, internal/dispatch/plan_test.go`
  - `Verify:` `go test -v ./internal/dispatch -run TestSynthesizeSubtaskPromptWithRepoSizeBounds`

---

## Release / Cycle Wrap-up (Not the next task)

Release/tag only with explicit human request. When triggered, the ID is `[99.1]`. Do not number feature, hygiene, or CI tasks as `99.x`. Do not calculate next task ID from this section.

- [ ] **[99.1]** Prepare Release (Git Tag) and Sanitize Context — Human permission required

---

## Future Backlog / Ideas (Unprioritized)

- [ ] *(Queue empty — awaiting next ideation)*

---

## How to Keep this File Lean

1. Detail only in the active task. When complete $\rightarrow$ log one line and promote the next task.
2. Backlog is a list of titles. Detailed design specifications live in `docs/adr/decisions.md`. Next ID = last ID in log (or active task).
