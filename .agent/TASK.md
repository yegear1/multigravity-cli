# TASK.md — Tasks and Roadmap for Legacy Codebase

> Defines WHAT needs to be done. Detailed history lives in `git log` and `ARCHIVE.md`.
> Specifications for future backlog tasks live in `docs/adr/decisions.md`.

---

## Active Task

### 📌 Task [05.1]: Centralize PATH enrichment, preserve host toolchain variables (CARGO_HOME/RUSTUP_HOME), and diagnose toolchain bleed in doctor

- **Description:** Centralize duplicated PATH enrichment logic (`launcher.go`, `headless/manager.go`, `agent/env.go`) into `internal/profile/env.go`. Expand path candidates to discover native Linux Node/JS runtimes (`pnpm`, `nvm`, `fnm`, `volta`), and on WSL2/Linux guarantee native Linux binaries precede `/mnt/c/` Windows mounts. Automatically preserve host `CARGO_HOME` and `RUSTUP_HOME` (and default `GOPATH`) in isolated profile environments to eliminate broken `$HOME`-relative compiler shims and prevent accidental toolchain installs inside profile directories. Add a diagnostic check in `multigravity doctor` to detect orphaned toolchains (`.cargo`, `.rustup`) inside profile directories.
- **Systems Involved:** `internal/profile`, `internal/headless`, `internal/agent`, `internal/doctor`, `.agent/INVARIANTS.md`, `.agent/NOTES.md`
- **Action Type:**
  - [x] Read-only / Documentation
  - [x] Source code changes
- **Status:** COMPLETED
  *(Workflow: `READY FOR PLANNING` → `PLANNING` on presenting plan → approval → `RUNNING`)*

### Acceptance Criteria
- [x] Create `internal/profile/env.go` consolidating `BuildEnrichedPATH(hostHome string) string` and environment seeding helpers (`PopulateToolchainEnv`).
- [x] Add discovery for common JS/Node bin directories: `~/.local/share/pnpm`, `~/.nvm/versions/node/*/bin`, `~/.fnm/current/bin`, `~/.volta/bin`.
- [x] On WSL2 / Linux, ensure Linux system and user paths precede any `/mnt/c/` Windows paths.
- [x] Export `CARGO_HOME` and `RUSTUP_HOME` pointing to `realHome` if unset in `launcher.go`, `headless/manager.go`, and `agent/env.go`.
- [x] Refactor `internal/headless` and `internal/agent` to consume the centralized PATH enrichment from `internal/profile`.
- [x] Add diagnostic rule in `internal/doctor` warning if `.cargo` or `.rustup` directory exists inside any profile directory under `~/AntigravityProfiles/`.
- [x] Add unit tests asserting PATH enrichment (including Node detection, WSL2 ordering, deduplication), toolchain env population, and doctor toolchain detection.
- [x] Verify full test suite passes with `go test ./...` and `go build ./cmd/multigravity`.
- [x] Update `.agent/INVARIANTS.md` and `.agent/NOTES.md` to document the active contracts.

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


- [ ] *(Queue empty — awaiting next prioritization)*

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
