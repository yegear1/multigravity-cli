# TASK.md — Tasks and Roadmap for Legacy Codebase

> Defines WHAT needs to be done. Detailed history lives in `git log` and `ARCHIVE.md`.
> Specifications for future backlog tasks live in `docs/adr/decisions.md`.

---

## Active Task

### [07.1] Worktree Ephemeral Branch Cleanup on Apply/Prune
- Status: READY FOR PLANNING
- Priority: P1
- Description: Automatically delete ephemeral git branches (`multigravity/wave-*`, `multigravity/retry-*`) created for dispatch tasks and waves when worktrees are pruned or applied, preventing local ref leakage in the host repository.
- Acceptance Criteria:
  - [ ] `PruneWorktree` and `PruneAll` in `internal/worktree/manager.go` delete the associated local git branch (`git branch -D <branch>`) when `branch` is non-empty and starts with `multigravity/`.
  - [ ] `dispatch wave` and `dispatch plan` ensure worktree cleanup removes both filesystem directories and git branch references upon task completion or failure.
  - [ ] Characterization tests in `internal/worktree/` and `internal/dispatch/` verify git branches are pruned cleanly with zero orphan branches.
  - [ ] `go test -v ./internal/worktree/... ./internal/dispatch/...` passes with exit code 0.

---

## Completed Tasks Log

| Task | Title | Commit(s) | Date |
|---|---|---|---|
| [90.1] | Componentize Server HTTP Handlers (internal/server/routes.go) | `8771e7d` | 2026-11-01 |
| [90.2] | Modularize Dispatch CLI Subcommands (internal/cmd/dispatch.go) | `00362c7` | 2026-11-01 |

---

## Backlog (Upcoming, in priority order)

> Tag pre-approved tasks having strictly disjoint file scopes with `[APPROVED]` `[PARALLEL: Wx]` and `Paths: ...` for concurrent `task-queue` execution.

- [ ] **[07.1]** `[PARALLEL: W1]` Worktree Ephemeral Branch Cleanup on Apply/Prune
  - `Read:` `internal/worktree/manager.go, internal/worktree/git.go`
  - `Paths:` `internal/worktree/manager.go, internal/worktree/git.go, internal/worktree/manager_test.go`
  - `Verify:` `go test -v ./internal/worktree/...`
- [ ] **[07.2]** `[PARALLEL: W1]` Compact JSON Wave Output Sanitization (`ToCompact` & Diff/Log Truncation)
  - `Read:` `internal/dispatch/wave.go, internal/dispatch/types.go`
  - `Paths:` `internal/dispatch/wave.go, internal/dispatch/wave_test.go`
  - `Verify:` `go test -v ./internal/dispatch/... -run 'Test.*Compact.*'`
- [ ] **[07.3]** `[PARALLEL: W2]` AST Outline Extraction for Large Files in Subtask Pre-Injection (Token Economy)
  - `Read:` `internal/dispatch/plan.go`
  - `Paths:` `internal/dispatch/plan.go, internal/dispatch/outline.go, internal/dispatch/plan_test.go`
  - `Verify:` `go test -v ./internal/dispatch/... -run 'Test.*Outline.*'`
- [ ] **[07.4]** `[PARALLEL: W2]` On-Demand Secondary Profile Quota Probing (`--probe` / Wave Pre-Flight)
  - `Read:` `internal/quota/routing.go, internal/dispatch/wave.go`
  - `Paths:` `internal/quota/routing.go, internal/dispatch/wave.go, internal/cmd/dispatch_wave.go`
  - `Verify:` `go test -v ./internal/quota/... ./internal/dispatch/...`

---

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
