---
id: "k-002"
title: "Prevent Orphaned Agy Subprocesses, Thinking Token Timeouts, and .venv Shadowing in Ephemeral Worktree Plans"
date: 2026-11-01
tags:
  - "dispatch"
  - "worktree"
  - "agy"
  - "python"
  - "uv"
  - "task-queue"
severity: "high"
affected_components:
  - "internal/dispatch/plan.go"
  - "internal/dispatch/manager.go"
  - "internal/agent/env.go"
  - "internal/profile/env.go"
  - "skills/multigravity-orchestrator/SKILL.md"
symptoms:
  - "Command failed to run: .../.multigravity/worktrees/...: no such directory"
  - "verify_exit_code: 4 (pytest ExitCode.USAGE_ERROR: test file not created before plan timeout)"
  - "Orphaned agy and language_server processes continuing after multigravity dispatch plan timeout"
  - "Editable install (.pth) in base repo .venv importing stale base repo src/ instead of ephemeral worktree src/"
---

# [k-002] Prevent Orphaned Agy Subprocesses, Thinking Token Timeouts, and .venv Shadowing in Ephemeral Worktree Plans

## Problem / Symptom

During multi-account parallel wave execution (`multigravity dispatch plan` via `task-queue`), five interacting issues caused token waste, false verification failures, and working-tree race conditions:

1. **Unbounded Thinking Token Burn Before File Creation (`agy -p`):**
   - When dispatched with `--model gemini-3.8-flash-medium` without `--effort low` or anti-baseline prompt guards, the worker subagent executed baseline `pytest` twice and generated `19,993` thinking tokens in a single planner turn (`> 2m07s`), hitting the `3m` plan timeout before creating the target test file (`pytest` exit code `4`).
2. **Orphaned Detached Subprocesses Surviving `ExecutePlan` Timeout:**
   - In `internal/dispatch/plan.go` (`ExecutePlan`), when `planCtx` timed out (`context.DeadlineExceeded`), `m.WaitForTask(planCtx, repoRoot, taskID)` returned the still-running task without calling `m.CancelTask(repoRoot, taskID, true)`.
   - Moreover, when an external script subsequently invoked a separate CLI process (`multigravity dispatch cancel <task-id>`), `CancelTask` only looked up `task.SessionID` in the new process's empty in-memory `m.agentMgr` map without falling back to killing `task.PID`.
   - As a result, the orphaned `agy` process continued running in the background after `.multigravity/worktrees/<id>` was deleted, observed `no such directory`, re-attached the worktree branch, and mutated files in the main repository working tree during the next wave.
3. **Ephemeral Worktree `.venv` Absence & Editable Install (`.pth`) Shadowing:**
   - Ephemeral git worktrees under `<repo>/.multigravity/worktrees/<id>` do not contain untracked `.venv` or `node_modules` directories from `<repo>`.
   - Even when `UV_PROJECT_ENVIRONMENT=<repo>/.venv` is passed, Python editable installs (`_editable_impl.pth`) inside `<repo>/.venv/lib/python3.x/site-packages/` point to `<repo>/src` (the unedited main checkout) rather than `<worktree>/src` unless `PYTHONPATH=<worktree>/src:<worktree>` precedes site-packages.
4. **Zero Diagnostic Context on `SubtaskResult` Failure:**
   - `SubtaskResult` in `internal/dispatch/plan.go` did not surface `VerificationOutput`, `VerificationExitCode`, or a bounded `LogTail` from `Task`, forcing the orchestrator agent to burn primary-account tokens inspecting `.gemini/antigravity-cli/brain/` transcripts to distinguish timeouts from test failures.
5. **Duplicate Internal Retry Conflict when `verify` + Explicit `args` are Combined:**
   - Passing `"verify"` inside `subtasks` while also passing explicit `agy` `"args": ["-p", "<prompt>", ...]` causes `watchSession` to synthesize retry feedback into `opts.Prompt`, which is ignored when custom `args` are supplied.

```text
{"step_index":11,"output_tokens":19993,"thinking":"..."}
{"step_index":12,"content":"Command failed to run: /repo/.multigravity/worktrees/wave-1793545570-task-01-1: no such directory."}
```

---

## Approved Solution

1. **Terminate Active Subtasks on Plan Deadline & Add Cross-Process PID Kill Fallback:**
   - In `ExecutePlan` (`internal/dispatch/plan.go`), if `planCtx.Err() != nil` or `completedTask.Status == StatusRunning` after `WaitForTask`, immediately invoke `m.CancelTask(repoRoot, taskID, true)` before returning `PlanResult`.
   - In `CancelTask` (`internal/dispatch/manager.go`), if `m.agentMgr.KillSession(task.SessionID)` returns an error (e.g., invoked from a separate CLI process where `m.agentMgr` has no in-memory session) and `task.PID > 0`, terminate the OS process / process group for `task.PID`.
2. **Auto-Seed Worktree Toolchain & Virtualenv Paths (`PopulateWorktreeProjectEnv`):**
   - When dispatching a task into an ephemeral worktree (`worktreePath != ""` and `baseRepoPath != ""`), automatically inspect `baseRepoPath`:
     - If `<baseRepoPath>/.venv` exists: set `UV_PROJECT_ENVIRONMENT=<baseRepoPath>/.venv`, `VIRTUAL_ENV=<baseRepoPath>/.venv`, prepend `<baseRepoPath>/.venv/bin` to `PATH`, and prepend `<worktreePath>/src:<worktreePath>` to `PYTHONPATH` so the worktree's source tree always shadows the editable `.pth` link in `<baseRepoPath>/.venv`.
     - If `<baseRepoPath>/node_modules` exists: set `NODE_PATH=<baseRepoPath>/node_modules` and prepend `<baseRepoPath>/node_modules/.bin` to `PATH`.
3. **Include Bounded Diagnostic Fields in `SubtaskResult`:**
   - Extend `SubtaskResult` backward-compatibly with `VerificationExitCode int`, `VerificationOutput string` (truncated to last 1000 bytes), and `LogTail string` (last 1000 bytes when `Status != StatusCompleted`) so orchestrators can diagnose failures from the JSON summary alone without reading logs or transcripts.
4. **Default Subagent Waves to Tiered Model IDs (`gemini-3.8-flash-low`/`medium`) & Fast-Edit Micro-Prompts:**
   - For pre-specified micro-contract tasks (`Read:` / `Paths:` / `Verify:`), pass `--model gemini-3.8-flash-low` or `--model gemini-3.8-flash-medium` to `agy` and instruct the worker not to run baseline test suites prior to editing.
   - **Important `agy` CLI Constraint:** Because `agy` model IDs already encode the effort suffix (`-low`, `-medium`, `-high`), NEVER pass a conflicting `--effort` flag alongside a tiered `--model` ID (`error: invalid model selection (--model "gemini-3.8-flash-medium" --effort "low"): --model gemini-3.8-flash-medium conflicts with --effort=low`).
5. **Diff Worktrees Strictly Against Worktree `HEAD` in Multi-Task Wave Loops:**
   - When committing multiple parallel tasks sequentially onto `main`, `main` advances after the first task commits. Diffing subsequent worktrees against `main` (`multigravity dispatch diff`) produces reverse patches of earlier wave commits. Always stage intent-to-add (`git add -N -- <matched_files>`) and run `git diff HEAD -- <matched_files>` inside each worktree.

**Root Cause:** `ExecutePlan` relied on in-process waiter detachment on timeout without terminating child OS processes, and ephemeral git worktrees lacked `PYTHONPATH` precedence over the parent repository's `.venv` editable install.

---

## What NEVER to Do
- ❌ **Anti-Pattern:** Letting `multigravity dispatch plan` exit on `--timeout` while leaving child `agy` processes running in `.multigravity/worktrees/`.
  - *Why:* After the wave runner deletes `.multigravity/` and prunes worktrees, the surviving `agy` agent attempts recovery via `git` commands and corrupts the main repository working tree.
- ❌ **Anti-Pattern:** Running `pytest` inside a git worktree against the parent repository's `.venv` without `PYTHONPATH=src`.
  - *Why:* Python's `.pth` file in `<repo>/.venv` resolves imports to `<repo>/src` instead of `<worktree>/src`, causing tests in the worktree to silently test unmodified code from `main`.
- ❌ **Anti-Pattern:** Dumping full `run.log` or raw `git diff` outputs into the orchestrator's stdout on wave failure.
  - *Why:* Floods the primary orchestrator context window; always bound failure output to a compact tail (`<= 1000` bytes).

---

## Verification
```bash
go test -v ./internal/dispatch ./internal/agent ./internal/profile ./internal/quota
```

---

## References
- Empirical execution telemetry from `vault-scribe` multi-account wave run (`W1`/`W2` across `joaoww`, `saasaba`, `luisfmb`).
