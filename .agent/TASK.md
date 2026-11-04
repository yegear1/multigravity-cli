# TASK.md — Tasks and Roadmap for Legacy Codebase

> Defines WHAT needs to be done. Detailed history lives in `git log` and `ARCHIVE.md`.
> Specifications for future backlog tasks live in `docs/adr/decisions.md`.

---

## Active Task

### [01.2] Next Milestone Planning & Triage
- Status: READY FOR PLANNING
- Priority: P2
- Description: Triage next roadmap milestone or features for multigravity-cli.

---

## Completed Tasks Log

| Task | Title | Commit(s) | Date |
|---|---|---|---|
| [90.3] | Modularize Dispatch Workspace & Filesystem Isolation | a8e8dd2 | 2026-11-04 |
| [90.2] | Generic HTTP Request Binding & Error Helpers | c3d7a8d | 2026-11-04 |
| [90.1] | Typed MCP Tool Registration & Generic Args Decoder | 23aeed6 | 2026-11-04 |
| [01.1] | Milestone Planning & Architectural Triage | — | 2026-11-02 |

---

## Backlog (Upcoming, in priority order)

> Tag pre-approved tasks having strictly disjoint file scopes with `[APPROVED]` `[PARALLEL: W1]` and `Paths: ...` for concurrent `task-queue` execution.

- [ ] *(Queue empty — awaiting next planning cycle)*

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
