# TASK.md — Tasks and Roadmap for Legacy Codebase

> Defines WHAT needs to be done. Detailed history lives in `git log` and `ARCHIVE.md`.
> Specifications for future backlog tasks live in `docs/adr/decisions.md`.

---

## Active Task

### [08.6] Hybrid Workspace Modes (Zero-I/O Read-Only Dispatch)
- Status: APPROVED
- Priority: P1
- Description: Add workspace modes (`--workspace=worktree|mirror|inherit`) to avoid ephemeral git worktree creation overhead for read-only / research tasks, executing safely in-place with filesystem mutation guards.
- Paths: `internal/cmd/dispatch_run.go`, `internal/dispatch/types.go`, `internal/dispatch/manager.go`, `internal/dispatch/plan.go`, `internal/dispatch/wave.go`

---

## Completed Tasks Log

| Task | Title | Commit(s) | Date |
|---|---|---|---|
| [08.5] | Dynamic Dispatch Roles & Execution Sandboxing | `d9dfdb8` | 2026-11-02 |
| [08.4] | Cross-Profile Shared Cooldown & Throttling Pool | `2607306` | 2026-11-02 |
| [08.3] | On-Demand Headless Agent Runner for Inactive Profiles | `ba0fe27` | 2026-11-02 |
| [08.2] | Build Isolation & Compiler Cache Strategy for Concurrent Worktrees | `ac20aae` | 2026-11-02 |
| [08.1] | Cross-Profile Toolchain & PATH Discovery | `5716c84` | 2026-11-02 |

---

## Backlog (Upcoming, in priority order)

> Tag pre-approved tasks having strictly disjoint file scopes with `[APPROVED]` `[PARALLEL: W1]` and `Paths: ...` for concurrent `task-queue` execution.

- [ ] **[08.7]** `[APPROVED]` `[SEQ: W7]` Native Antigravity Subagent Orchestration Bridge — Connect Multigravity macro-routing directly to Antigravity subagent workers via headless CLI invocation (`agy --headless -p`) and native MCP delegation tools with quota-aware profile failover. (`Paths: internal/mcp/server.go, internal/agent/runner.go, internal/dispatch/manager.go, internal/quota/routing.go`)

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
