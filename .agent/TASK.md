# TASK.md — Tasks and Roadmap for Legacy Codebase

> Defines WHAT needs to be done. Detailed history lives in `git log` and `ARCHIVE.md`.
> Specifications for future backlog tasks live in `docs/adr/decisions.md`.

---

## Active Task

### 📌 Task [04.4]: Enforce per-profile OAuth token file isolation on Linux by bypassing global D-Bus Secret Service keyring in `language_server`

- **Description:** Fix profile account bleed-over on Linux when closing and reopening Antigravity IDE profiles from scratch. Antigravity's `language_server` uses `zalando/go-keyring` over the Linux D-Bus Secret Service (`gnome-keyring-daemon`) under a single global service key (`"antigravity"`), ignoring `GEMINI_FORCE_FILE_STORAGE`. Force `language_server`'s `CompositeTokenStorage` into per-profile file storage (`$HOME/.gemini/jetski-standalone-oauth-token`) across IDE, agent, and headless launches by refreshing `<profileDir>/.gemini/cache/antigravity-keyring-unavailable` in `EnsureProfileLayout` and exporting `WSL_INTEROP=/dev/null` (when unset on Linux) alongside `GEMINI_FORCE_FILE_STORAGE=true`.
- **Systems Involved:** `internal/profile`, `internal/agent`, `internal/headless`, `internal/quota`
- **Action Type:**
  - [ ] Read-only / Documentation
  - [x] Source code changes
- **Status:** COMPLETED
  *(Workflow: `READY FOR PLANNING` → `PLANNING` on presenting plan → approval → `RUNNING`)*

### Acceptance Criteria
- [x] Touch/create `<profileDir>/.gemini/cache/antigravity-keyring-unavailable` during `EnsureProfileLayout` so `language_server`'s `keyringRecentlyUnavailable` check bypasses the global OS keyring.
- [x] Export `GEMINI_FORCE_FILE_STORAGE=true` unconditionally for isolated profile environments (`internal/profile/launcher.go`, `internal/agent/env.go`, `internal/headless/manager.go`, `internal/quota/headless.go`) and export `WSL_INTEROP=/dev/null` on Linux when unset so `language_server`'s `shouldBypassKeyring()` permanently selects file-based token storage (`<profile>/.gemini/jetski-standalone-oauth-token`) without disrupting D-Bus desktop portals (`xdg-open`, file pickers).
- [x] Update and expand characterization tests in `internal/profile/launcher_test.go` and `internal/profile/layout_test.go`.
- [x] Document the `language_server` `CompositeTokenStorage` keyring bypass mechanics in `.agent/knowledge/` and `.agent/INVARIANTS.md`.
- [x] All tests pass (`go test ./...`) and `git diff --check` exits with code 0.

---

## Completed Tasks Log

| Task | Title | Commit(s) | Date |
|---|---|---|---|
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
