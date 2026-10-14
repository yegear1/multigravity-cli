# TASK.md — Tasks and Roadmap for Legacy Codebase

> Defines WHAT needs to be done. Detailed history lives in `git log` and `ARCHIVE.md`.
> Specifications for future backlog tasks live in `docs/adr/decisions.md`.

---

## Active Task

### 📌 Task [03.7]: Task retry with verification feedback loop (`--retry <n>`)

- **Description:** Implement automated task retries when verification fails, feeding verification output back into consecutive agent prompt sessions.
- **Systems Involved:** `dispatch`, `cmd`
- **Action Type:**
  - [ ] Read-only / Documentation
  - [x] Source code changes
- **Status:** READY FOR PLANNING
  *(Workflow: `READY FOR PLANNING` → `PLANNING` on presenting plan → approval → `RUNNING`)*

### Acceptance Criteria
- [ ] Support `--retry <n>` option in `dispatch run` and `DispatchOptions`.
- [ ] Feed verification failure output into agent retry prompt.
- [ ] Preserve ephemeral worktree across retries without polluting branch history.
- [ ] Automated tests in `internal/dispatch` and `internal/cmd` pass with exit code 0.

---

## Completed Tasks Log

| Task | Title | Commit(s) | Date |
|---|---|---|---|
| [03.6] | Governance context injection in `dispatch run` (`AGENTS.md` / `TASK.md`) | working tree | 2026-10-14 |
| [03.5] | Verification and audit hook in dispatch (`--verify <cmd>`) | working tree | 2026-10-14 |
| [03.4] | Quota-aware route recommendation CLI command (`multigravity route`) | working tree | 2026-10-14 |
| [03.3] | Agent process sandboxing via wrapper `agent-guard wrap --repo` | working tree | 2026-10-14 |
| [03.2] | Command `multigravity dispatch apply <task-id>` to merge approved worktrees | working tree | 2026-10-14 |
| [00.4] | Align repository governance and agent artifacts with brownfield template | working tree | 2026-10-14 |
| [03.1] | Native Codex CLI driver in dispatch and agent (`--agent codex`, sandbox and `-C`) | [`e8e0547`](https://github.com/ye-dev/multigravity-cli/commit/e8e0547) | 2026-10-14 |
| [00.3] | Archive consolidated NOTES into ARCHIVE.md | working tree | 2026-10-14 |
| [90.1] | Unified registration for dual HTTP route prefixes (`/api/v1` and `/api`) | working tree | 2026-10-04 |
| [00.2] | Gateway test coverage gaps and unauthenticated failover fixes | working tree | 2026-10-04 |
| [02.1] | Agent runner decision tree and documentation alignment | working tree | 2026-10-04 |

---

## Backlog (Upcoming, in priority order)

- [ ] **[03.8]** Interactive terminal multiplexing enhancements for Windows PTY — `[tui]`, `[agent]`

---

## Release / Cycle Wrap-up (Not the next task)

Release/tag only with explicit human request. When triggered, the ID is `[99.1]`. Do not number feature, hygiene, or CI tasks as `99.x`. Do not calculate next task ID from this section.

- [ ] **[99.1]** Prepare Release (Git Tag) and Sanitize Context — Human permission required

---

## Future Backlog / Ideas (Unprioritized)

- [ ] Interactive terminal multiplexing enhancements for Windows PTY
- [ ] Automated token estimation calibration against live Cloud Code responses

---

## How to Keep this File Lean

1. Detail only in the active task. When complete $\rightarrow$ log one line and promote the next task.
2. Backlog is a list of titles. Detailed design specifications live in `docs/adr/decisions.md`. Next ID = last ID in log (or active task).
