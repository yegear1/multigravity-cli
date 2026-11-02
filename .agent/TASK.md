# TASK.md — Tasks and Roadmap for Legacy Codebase

> Defines WHAT needs to be done. Detailed history lives in `git log` and `ARCHIVE.md`.
> Specifications for future backlog tasks live in `docs/adr/decisions.md`.

---

## Active Task

### [07.4] On-Demand Secondary Profile Quota Probing (`--probe` / Wave Pre-Flight)
- Status: RUNNING
- Priority: P2
- Description: Automatically probe and wake inactive secondary profile language servers during wave pre-flight (`--probe` flag in `multigravity dispatch wave` and preflight routines) to refresh live quota metrics without manual priming, enabling multi-account wave distribution.
- Acceptance Criteria:
  - [ ] Implement on-demand probing in `internal/quota/routing.go` (`ProbeInactiveProfiles(profiles []string) error` or similar) leveraging headless / quota status helpers.
  - [ ] Add `--probe` flag to `multigravity dispatch wave` in `internal/cmd/dispatch_wave.go` and wire into `WaveOptions` / `ExecuteTaskWave`.
  - [ ] Verify profile quota discovery and error handling with characterization tests in `internal/quota` and `internal/dispatch`.
  - [ ] `go test -v ./internal/quota/... ./internal/dispatch/...` passes with exit code 0.

---

## Completed Tasks Log

| Task | Title | Commit(s) | Date |
|---|---|---|---|
| [07.3] | AST Outline Extraction for Large Files in Subtask Pre-Injection (Token Economy) | `HEAD` | 2026-11-02 |
| [07.2] | Compact JSON Wave Output Sanitization (`ToCompact` & Diff/Log Truncation) | `69992b2` | 2026-11-02 |
| [07.1] | Worktree Ephemeral Branch Cleanup on Apply/Prune | `a1bae69` | 2026-11-02 |
| [90.1] | Componentize Server HTTP Handlers (internal/server/routes.go) | `8771e7d` | 2026-11-01 |
| [90.2] | Modularize Dispatch CLI Subcommands (internal/cmd/dispatch.go) | `00362c7` | 2026-11-01 |

---

## Backlog (Upcoming, in priority order)

> Tag pre-approved tasks having strictly disjoint file scopes with `[APPROVED]` `[PARALLEL: Wx]` and `Paths: ...` for concurrent `task-queue` execution.

- [x] **[07.1]** `[APPROVED]` `[PARALLEL: W1]` Worktree Ephemeral Branch Cleanup on Apply/Prune (a1bae69)
  - `Read:` `internal/worktree/manager.go, internal/worktree/git.go`
  - `Paths:` `internal/worktree/manager.go, internal/worktree/git.go, internal/worktree/manager_test.go`
  - `Verify:` `go test -v ./internal/worktree/...`
- [x] **[07.2]** `[APPROVED]` `[PARALLEL: W1]` Compact JSON Wave Output Sanitization (`ToCompact` & Diff/Log Truncation) (69992b2)
  - `Read:` `internal/dispatch/wave.go, internal/dispatch/types.go`
  - `Paths:` `internal/dispatch/wave.go, internal/dispatch/wave_test.go`
  - `Verify:` `go test -v ./internal/dispatch/... -run 'Test.*Compact.*'`
- [x] **[07.3]** `[APPROVED]` `[PARALLEL: W2]` AST Outline Extraction for Large Files in Subtask Pre-Injection (Token Economy)
  - `Read:` `internal/dispatch/plan.go`
  - `Paths:` `internal/dispatch/plan.go, internal/dispatch/outline.go, internal/dispatch/plan_test.go`
  - `Verify:` `go test -v ./internal/dispatch/... -run 'Test.*Outline.*'`
- [ ] **[07.4]** `[APPROVED]` `[PARALLEL: W2]` On-Demand Secondary Profile Quota Probing (`--probe` / Wave Pre-Flight)
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
