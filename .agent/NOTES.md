# NOTES.md — Active Decisions and Contracts for Legacy Codebase

> Stores WHY recent changes and technical discoveries were made.
> For untouchable constraints, consult [`.agent/INVARIANTS.md`](INVARIANTS.md).
> For cumulative architectural logs and design specs, consult [`docs/adr/decisions.md`](../docs/adr/decisions.md).

---

## How to Use this File (for the Agent)

1. **Read before planning any task.**
2. **Record a new entry when:**
   - An unusual behavior or platform pitfall is investigated/resolved.
   - A new data contract (schema/payload) is expanded backward-compatibly.
   - Conscious technical debt is incurred during a fix.
3. **Keep entries concise and prune once consolidated into `INVARIANTS.md` or `ARCHIVE.md`.**

4. **CRITICAL INVARIANT:** NEVER create task-specific headings (e.g., `## [Task XX.Y] Description` or `### [Task XX.Y]`) in `NOTES.md`. Task completion details live in `git log` and `.agent/ARCHIVE.md`. Architectural decisions live in `docs/adr/decisions.md`. In `NOTES.md`, maintain and update ONLY the live tables/lists of active contracts, interfaces, and topology.

---

## Active Technical Contracts & Decisions

- **Pre-Approved Tasks (`APPROVED`), Parallel Waves (`[PARALLEL: Wx]`) & `task-queue` Skill:** Tasks finalized during interactive planning use `Status: APPROVED` in `.agent/TASK.md` (with independent disjoint-file tasks tagged `[PARALLEL: Wx]` + `Paths:`), allowing fresh sessions (`proceed`) or isolated worker subagents and multi-account quota failover (`task-queue` skill on `proceed queue`) to transition directly to `RUNNING` without a redundant `PLANNING` pause, halting only when ambiguities, unapproved dependencies, or blockers require human intervention.
- **Ephemeral Worktree Subagent Execution Gotchas (`k-002`):** See [`.agent/knowledge/k-002-dispatch-plan-worktree-orphan-and-venv-isolation.md`](knowledge/k-002-dispatch-plan-worktree-orphan-and-venv-isolation.md) for root causes and approved fixes covering (1) `agy -p` 20k thinking token timeouts, `--model` vs `--effort` flag conflict rules (`gemini-3.8-flash-low/medium/high` already encode effort), and single-grep escape hatch + line-ranged `Read: file:start-end` pre-injection, (2) orphaned detached child processes surviving `ExecutePlan` deadline and cross-process `CancelTask`, (3) `.venv` editable-install (`.pth`) shadowing in ephemeral worktrees without `PYTHONPATH` precedence, (4) bounded `verification_output`/`log_tail` diagnostics in `SubtaskResult` (including on `empty_diff`) and rejecting Go `testing: warning: no tests to run` false-positives, (5) diffing worktrees strictly against `HEAD` (`git diff HEAD -- <paths>`) / immutable SHA `BaseCommit` rather than moving `main` during multi-task wave commits, and (6) model-aware bucket routing (`gemini`/`argon`/`3p`) with failed-profile retry exclusion and native `.agent/TASK.md` wave execution (`multigravity dispatch wave`).

## Consolidated Historical Decisions

Previous architectural decisions from releases v1.4.0 through v2.2.0 have been consolidated into [`.agent/INVARIANTS.md`](INVARIANTS.md) and [`docs/adr/decisions.md`](../docs/adr/decisions.md), with task history recorded in [`.agent/ARCHIVE.md`](ARCHIVE.md).

---

### 2. Multi-Agent Collaboration Contracts (Optional)

> Canonical 3-tier subagent topology, delegation micro-contracts, and worktree isolation. If operating single-agent, leave as default.

| Role | Interface / Channel | Responsibilities | Boundary & Invariants |
|---|---|---|---|
| **`Lead / Orchestrator`** | `.agent/TASK.md` / Interactive Prompt | Decomposes initiatives into wave-compatible tasks, manages task lifecycle, routes to model/quota tiers, interacts with human. | Sole manager of `TASK.md` and commit log. Does not run parallel worker tasks directly. |
| **`Worker / Implementer`** | Worktree / Target Workspace | Executes atomic code/doc changes strictly adhering to delegated micro-contract (`Read:`, `Paths:`, `Verify:`). | Restricted exclusively to `Paths:`. Adheres to DoD in `AGENTS.md`. |
| **`Reviewer / Sentinel`** | Diffs / Test Harness (`Verify:`) | Read-only inspection of diffs against task DoD, regression tests, and security blast radius. | Read-only; reports structured Pass/Fail diagnostic payload. |

#### Worktree Isolation & Dependency Bootstrap Contract

| Element | Protocol / Mechanism | Invariant / Boundary |
|---|---|---|
| **Workspace Provisioning** | `git worktree add -b task-<id> <path> HEAD` | Default ephemeral path (`${TMPDIR:-/tmp}/worktrees/task-<id>`) or `.worktrees/task-<id>` (`.worktrees/` in `.gitignore`). |
| **Dependency Bootstrap** | Fast-Link vs. Isolated-Install | Fast-Link (symlink `node_modules/` or `.venv/`) when `Paths:` has no manifest changes (<50ms, zero-copy). Isolated-Install when manifests change. |
| **Environment Injection** | Copy `.env` / `.env.example` to worktree | Ensures test harness execution without leaking uncommitted tokens or mutating root config. |
| **Teardown & GC** | `git worktree remove --force` + `branch -D` + `prune` | Immediate cleanup post-integration; trap declared in runners to prevent dangling worktree locks. |

#### Multi-Account Quota Routing & Failover Contract

| Element | Protocol / Mechanism | Invariant / Boundary |
|---|---|---|
| **Profile Routing Priority** | `select-profile.sh --exclude-current` | Autonomous workers route to secondary accounts with highest quota; active IDE profile reserved for Lead Orchestrator. |
| **Quota Floor Circuit Breaker** | `remaining_fraction <= 0.05` | Immediate disqualification of profiles with <=5% quota; halts execution if all candidate profiles are below floor. |
| **Model Tier Selection** | `gemini-3.8-flash-medium` | Low token footprint default; escalation to `gemini-4-argon-medium` permitted only for complex architecture or max 1 retry after verification failure. |
| **Cooldown Tracking** | `<profile>/.multigravity/quota-history.jsonl` | 429 rate limit backoff and cooldown persistence to prevent immediate re-dispatch. |
| **Worktree Recovery** | Worktree verify on timeout | Runs `Verify:` inside worktree on timeout; if code 0 and matches `Paths:`, recovers diff cleanly. |