# TASK.md — Tasks and Roadmap for Legacy Codebase

> Defines WHAT needs to be done. Detailed history lives in `git log` and `ARCHIVE.md`.
> Specifications for future backlog tasks live in `docs/adr/decisions.md`.

---

## Active Task

### 📌 Task [05.13]: *(Queue Empty — Awaiting Next Ideation)*

- **Description:** All tasks in Wave `W1`–`W5` (`[05.6]`–`[05.12]`) have been executed, verified, and committed.
- **Systems Involved:** —
- **Action Type:**
  - [x] Read-only / Documentation
  - [ ] Source code changes
- **Status:** READY FOR PLANNING
  *(Workflow: `READY FOR PLANNING` → `PLANNING` on presenting plan → approval → `RUNNING`)*

### Acceptance Criteria
- [ ] Define and approve next task in `.agent/TASK.md`.

---

## Completed Tasks Log

| Task | Title | Commit(s) | Date |
|---|---|---|---|
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
