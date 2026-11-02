# TASK.md — Tasks and Roadmap for Legacy Codebase

> Defines WHAT needs to be done. Detailed history lives in `git log` and `ARCHIVE.md`.
> Specifications for future backlog tasks live in `docs/adr/decisions.md`.

---

## Active Task

### Queue Empty — Awaiting Next Ideation
- Status: READY FOR PLANNING
- Priority: P1
- Description: Partition the monolithic `internal/server/routes.go` (2.795 LOC, 97 handlers) into domain-specific files (`routes_profile.go`, `routes_dispatch.go`, `routes_gateway.go`, `routes_quota.go`, `routes_system.go`) within package `server` while keeping exact API contracts, HTTP endpoints, and full test parity.
- Acceptance Criteria:
  - [ ] `internal/server/routes.go` reduced to router registration (`setupRoutes`), middleware (`corsMiddleware`), and response helpers (`writeJSON`, `writeError`).
  - [ ] Handlers extracted to domain files in package `server` without circular dependencies or signature changes.
  - [ ] All HTTP routes, methods, and JSON payloads remain 100% backward-compatible.
  - [ ] `go test -v ./internal/server/...` passes with exit code 0.
  - [ ] `golangci-lint run` / `git diff --check` passes cleanly.

---

## Completed Tasks Log

| Task | Title | Commit(s) | Date |
|---|---|---|---|
| [90.1] | Componentize Server HTTP Handlers (internal/server/routes.go) | `8771e7d` | 2026-11-01 |
| [90.2] | Modularize Dispatch CLI Subcommands (internal/cmd/dispatch.go) | `00362c7` | 2026-11-01 |

---

## Backlog (Upcoming, in priority order)

> Tag pre-approved tasks having strictly disjoint file scopes with `[APPROVED]` `[PARALLEL: W1]` and `Paths: ...` for concurrent `task-queue` execution.

- [x] **[90.1]** `[APPROVED]` `[PARALLEL: W1]` Componentize Server HTTP Handlers (internal/server/routes.go) (8771e7d)
  - `Read:` `internal/server/server.go, internal/server/routes.go`
  - `Paths:` `internal/server/routes.go, internal/server/routes_profile.go, internal/server/routes_dispatch.go, internal/server/routes_gateway.go, internal/server/routes_system.go`
  - `Verify:` `go test -v ./internal/server/...`
- [x] **[90.2]** `[APPROVED]` `[PARALLEL: W1]` Modularize Dispatch CLI Subcommands (internal/cmd/dispatch.go) (00362c7)
  - `Read:` `internal/cmd/root.go, internal/cmd/dispatch.go`
  - `Paths:` `internal/cmd/dispatch.go, internal/cmd/dispatch_run.go, internal/cmd/dispatch_wave.go, internal/cmd/dispatch_plan.go, internal/cmd/dispatch_apply.go, internal/cmd/dispatch_ops.go`
  - `Verify:` `go test -v ./internal/cmd/...`

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
