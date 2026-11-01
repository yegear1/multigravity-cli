# TASK.md — Tasks and Roadmap for Legacy Codebase

> Defines WHAT needs to be done. Detailed history lives in `git log` and `ARCHIVE.md`.
> Specifications for future backlog tasks live in `docs/adr/decisions.md`.

---

## Active Task

### 📌 Task [05.2]: Ephemeral Worktree Host Project Environment Seeding (`.venv`, `PYTHONPATH`, `NODE_PATH`)

- **Description:** Seed host repository project environments (`.venv`, `UV_PROJECT_ENVIRONMENT`, `VIRTUAL_ENV`, `PYTHONPATH` with worktree `src/` precedence over editable `.pth` links, and `NODE_PATH` / `node_modules/.bin`) into isolated agent sessions running inside ephemeral git worktrees (`.multigravity/worktrees/<id>`), eliminating manual environment boilerplate in orchestrator `Verify:` commands.
- **Systems Involved:** `internal/profile`, `internal/agent`, `.agent/NOTES.md`
- **Action Type:**
  - [ ] Read-only / Documentation
  - [x] Source code changes
- **Status:** APPROVED
  *(Workflow: `READY FOR PLANNING` → `PLANNING` on presenting plan → approval → `RUNNING`)*

### Acceptance Criteria
- [ ] Add `PopulateWorktreeProjectEnv(envMap map[string]string, baseRepoPath, worktreePath string)` in `internal/profile/env.go`.
- [ ] When `<baseRepoPath>/.venv` exists, set `UV_PROJECT_ENVIRONMENT` and `VIRTUAL_ENV` (if unset), prepend `<baseRepoPath>/.venv/bin` (or `Scripts` on Windows) to `PATH`, and prepend `<worktreePath>/src` (if present) and `<worktreePath>` to `PYTHONPATH` so worktree packages shadow base repo editable `.pth` links.
- [ ] When `<baseRepoPath>/node_modules` exists, set `NODE_PATH=<baseRepoPath>/node_modules` (if unset) and prepend `<baseRepoPath>/node_modules/.bin` to `PATH`.
- [ ] Invoke `PopulateWorktreeProjectEnv` from `BuildAgentEnv` (`internal/agent/env.go`) when `opts.WorktreeID != ""` and `opts.Cwd` / `opts.RepoPath` resolve an ephemeral worktree under `<baseRepoPath>/.multigravity/worktrees/`.
- [ ] Add unit tests in `internal/profile/env_test.go` and `internal/agent/agent_test.go` verifying `.venv`, `PYTHONPATH`, and `node_modules` seeding.
- [ ] Verify full test suite passes with `go test -v ./internal/profile ./internal/agent`.

---

## Completed Tasks Log

| Task | Title | Commit(s) | Date |
|---|---|---|---|
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

- [ ] **[05.2]** `[APPROVED]` `[PARALLEL: W1]` Seed base repository `.venv` (`UV_PROJECT_ENVIRONMENT`, `VIRTUAL_ENV`, `PYTHONPATH` worktree precedence) and `NODE_PATH` into ephemeral worktree agent environments
  - `Read:` `internal/profile/env.go, internal/agent/env.go, internal/agent/types.go`
  - `Paths:` `internal/profile/env.go, internal/profile/env_test.go, internal/agent/env.go, internal/agent/agent_test.go`
  - `Verify:` `go test -v ./internal/profile ./internal/agent`
- [ ] **[05.3]** `[APPROVED]` `[PARALLEL: W1]` Terminate running subtasks on `ExecutePlan` timeout, add cross-process PID fallback in `CancelTask`, and expose bounded `verification_output`/`log_tail` in `SubtaskResult`
  - `Read:` `internal/dispatch/plan.go, internal/dispatch/manager.go, internal/dispatch/types.go`
  - `Paths:` `internal/dispatch/plan.go, internal/dispatch/plan_test.go, internal/dispatch/manager.go, internal/dispatch/dispatch_test.go`
  - `Verify:` `go test -v ./internal/dispatch`
- [ ] **[05.4]** `[APPROVED]` `[PARALLEL: W1]` Align `multigravity route` model identifiers with live `agy models` (`gemini-3.8-flash-low/medium/high`) and expose `recommended_effort` (`low`/`medium`/`high`)
  - `Read:` `internal/quota/routing.go, internal/quota/routing_test.go`
  - `Paths:` `internal/quota/routing.go, internal/quota/routing_test.go`
  - `Verify:` `go test -v ./internal/quota -run TestRecommendRoute`
- [ ] **[05.5]** `[APPROVED]` `[SEQ: W2]` Synchronize `multigravity-orchestrator` skill with `Read:/Paths:/Verify:` micro-contracts, `--effort low` defaults, and bounded diagnostic tail handling
  - `Read:` `skills/multigravity-orchestrator/SKILL.md, .agent/knowledge/k-002-dispatch-plan-worktree-orphan-and-venv-isolation.md`
  - `Paths:` `skills/multigravity-orchestrator/SKILL.md`
  - `Verify:` `git diff --check skills/multigravity-orchestrator/SKILL.md`

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
