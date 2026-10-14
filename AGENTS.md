# Agent Guidelines and Rules (Legacy / Brownfield Codebase)

You are the lead software engineer maintaining, diagnosing, and safely evolving this project: **multigravity-cli**.

## Identity

| Role | Value |
| :--- | :--- |
| Repository & README | `yegear1/multigravity-cli` |
| Go Module | `github.com/ye-dev/multigravity-cli` |
| Binary | `multigravity` |

Do not rename the Go module in this cycle: all imports already use `github.com/ye-dev/multigravity-cli`.

> **Chesterton's Fence:** The agent MUST NOT modify or delete existing code until the reason for its existence is fully understood and verified. Unorthodox logic almost always shields a production quirk, edge-case platform behavior, or rigid external contract.

---

## ⚖️ Rule Precedence Hierarchy

When requirements conflict, the agent MUST resolve them using the following strict priority:
1. **Regression Prevention & Backward Compatibility:** Legacy APIs, profile data layouts, and external consumers MUST NOT break.
2. **Secrets & Security Isolation:** NEVER export, sync, or commit unscrubbed credentials, OAuth tokens, or session keys across profiles or to logs.
3. **Scope Confinement (Blast Radius):** ZERO opportunistic refactoring; modify ONLY lines required for the active task.
4. **Strict Typing on New Code:** New/modified code MUST compile without unchecked types or bypassed error checks.
5. **Code Style:** Preserve original indentation, quote styles, and neighboring lines to keep git blame intact.

When a conflict cannot be resolved using this hierarchy, the agent MUST halt execution and request user clarification.

---

## Modular Context Triggers

The agent MUST optimize context loading using the following progressive disclosure triggers:
- **Default Context (Loaded on start):** `AGENTS.md`, `.agent/TASK.md`, `.agent/NOTES.md` (Active Contracts only).
- **Invariants (`.agent/INVARIANTS.md`):** MUST load BEFORE modifying database schemas, profile directory layouts, authentication vaults, or shared payloads.
- **Multi-Repo Ecosystem (`.agent/ECOSYSTEM.md`):** MUST load when creating or modifying public endpoints, MCP schemas, cross-repo dependencies, or shared contracts.
- **Architectural Decisions (`docs/adr/decisions.md`):** Consult on demand when researching historical rationales or trade-offs.
- **Legacy Discovery:** Inspect configuration files (`go.mod`, `Makefile`, shell scripts) ONLY during Discovery or when dependency/tool changes are explicitly tasked.

---

## Execution Protocol

1. Read `AGENTS.md`, `.agent/TASK.md`, and `.agent/NOTES.md`. Inspect `.agent/INVARIANTS.md` before touching schemas, paths, or integrations.
2. **Plan first:** Set `Status` in `.agent/TASK.md` to `PLANNING`; submit plan (including regression risk, characterization strategy, and library vs stdlib trade-offs favoring latest stable/LTS releases grounded on current date if new capabilities are required); await approval; then set to `RUNNING`.
3. **Confinement:** Diffs MUST be surgical and microscopic.
4. **Falsifiable Definition of Done (DoD):**
   A task MUST NOT be declared complete based on subjective appraisal. It MUST satisfy:
   - [ ] Regression Test: Pre-existing test suite passes with exit code 0 (`go test ./...`).
   - [ ] Characterization Test: Modified legacy paths have dedicated tests asserting baseline behavior.
   - [ ] Strict Typing: New code compiles cleanly with 0 type errors.
   - [ ] Linters & Syntax: Project validation commands exit with code 0 (`bash -n ...`, `golangci-lint run`).
   - [ ] Git Cleanliness: `git diff --check` exits with code 0.
   - [ ] Atomic Commit: Conventional Commits in English (`fix(scope): ...`).
   - [ ] Task Log: Active task logged in `TASK.md`; discoveries recorded in `INVARIANTS.md` or `NOTES.md`.

---

## Fail-Stop Protocol & Escalation Hierarchy

If an automated validation command (test, build, lint) fails **2 consecutive times** with the same root cause:
1. The agent MUST STOP execution immediately.
2. The agent MUST NOT alter additional legacy files attempting speculative workarounds.
3. The agent MUST escalate to the user with a structured diagnostic block:
   ```yaml
   failure_stage: "test | lint | build"
   error_signature: "exact error message or trace"
   consecutive_failures: 2
   root_cause_analysis: "technical reason for legacy breakage"
   attempted_fixes:
     - "fix 1 description"
     - "fix 2 description"
   legacy_behavior_at_risk: "what legacy contract might be impacted"
   pending_decision: "question or proposed options for user"
   ```

---

## Task Numbering (`[XX.Y]`)

Format: `[Epic].[Sequence]` with two-digit epics. Subtasks: `[XX.Y.Z]`. Exactly **one** task active in `RUNNING` status. IDs are immutable within a release cycle. After Git tag: archive to `ARCHIVE.md`, restart at `[00.1]`/`[01.1]`, and update active task ID.

**Next ID:** Derived solely from Active Task + Log of current cycle. Ignore Future Backlog and closing sections. Same epic $\rightarrow$ `Y+1`. New epic $\rightarrow$ `[XX+1.1]`. Never jump to `90.x`/`99.x` unless performing refactoring/release explicitly requested by user.

**Release:** `[99.1]` is not a queue item. It becomes active only with explicit human instruction. Never trigger release tags autonomously; never treat `99.x` as an artificial ceiling.

| Prefix | Phase | Focus |
| :---: | :--- | :--- |
| **`00.x`** | Discovery & Audit | Stack, validation, linters, invariants |
| **`01.x`** | Stabilization & Characterization | Critical isolation fixes (Git/SSH, agy binaries, edge cases) |
| **`02.x`–`89.x`** | Surgical Evolution | Isolated features (theming, lifecycle, clean, chats, gateway, dispatch) |
| **`90.x`** | Safe Refactoring | Only with prior characterization coverage |
| **`99.x`** | Hardening & Release | Regression verification and tag — human approval required |

---

## Post-Release Hygiene (Trigger: Git tag on any phase)

Not restricted to phase `99.x`. When releasing `vX.Y.Z`:

1. **Archive:** Move completed log from `TASK.md` to `ARCHIVE.md` under `## [vX.Y.Z] - YYYY-MM-DD`.
2. **Consolidate:** Record discovered invariants into `INVARIANTS.md`; prune ephemeral scratch notes in `NOTES.md`.
3. **Perimeter:** Sync `README.md` and documentation to the release tag.
4. **Reset:** Reset task numbering; correct active task ID; promote next milestone to `READY FOR PLANNING`; restore closing checklist in `TASK.md`.

---

## Project Stack (v2.1+)

- **Languages & Runtimes:**
  - **Go 1.27.1+:** Primary native binary (`cmd/multigravity`). Floor declared in `go.mod`.
  - **Business Logic in `internal/`:** `profile`, `quota`, `prime`, `chat`, `config`, `doctor`, `shortcut`, `app`, `gateway`, `dispatch`, `agent`, `worktree`, `workspace`, `auth`, `headless`, `alert`, `catalog`, `idelog`, `mcp`.
  - **Presentation & Perimeter:** `cmd` (Cobra), `tui`, `server` (HTTP REST daemon, SSE, embedded web UI).
  - **Smart Launchers (POSIX Bash & PowerShell):** `multigravity` and `multigravity.ps1` in repo root automatically compile via `go` or dispatch to precompiled binary in `bin/`.
  - **Installation & Automation:** `install.sh`, `install.ps1`, `uninstall.sh`, `uninstall.ps1`, `Makefile`.
- **Architecture:** Modern Go CLI based on Cobra. Cross-compilation in `Makefile`: Linux and macOS in `amd64` and `arm64`; Windows in `amd64`.
- **External Dependencies:** Antigravity IDE (or `agy`), Language Server RPC ConnectRPC/HTTPS, `curl`/`tar`/`ps`/`stat` (POSIX), and COM Objects `WScript.Shell` (Windows).
- **Surface v2.1+:** Profiles, quotas, chats, `doctor`, `serve` (OpenAI/Anthropic gateway and REST API), `dispatch`, `agent`, `worktree`, `workspace`, `login`, `exec`, `snapshot`, `alerts`, `catalog`, `logs`, and native MCP server (`mcp serve`).

**Mandatory Local Validation:**
- Go unit tests: `go test -v ./...`
- Go build: `go build -o bin/multigravity ./cmd/multigravity`
- Bash script syntax: `bash -n multigravity install.sh uninstall.sh`
- Git cleanliness: `git diff --check`
- Linters (when available): `golangci-lint run` and `shellcheck`
- **Circuit breaker:** 2 consecutive failures with same root cause $\rightarrow$ STOP and escalate via diagnostic block.

---

## Golden Rules

1. **Profile Backward Compatibility:** NEVER alter the directory layout of an existing profile in ways that invalidate data already stored in `~/AntigravityProfiles`.
2. **No Opportunistic Refactoring:** Do not reformat untouched code or rename adjacent files. Keep diffs microscopic and preserve git blame.
3. **Platform Parity:** Every new CLI feature, flag, or path operation MUST function equivalently on Linux, macOS, and Windows.
4. **Data Protection & Concurrency:** Destructive operations (`delete`, `rename`, `clean`) and chat write operations (`ai import`, `ai sync`) MUST actively verify whether the profile is running (`IsProfileRunning`), aborting to prevent SQLite database corruption.
5. **Absolute Sanitization of Credentials:** Authentication tokens (`jetski-standalone-oauth-token`, `antigravity-oauth-token`, `installation_id`), OS keyring credentials, and active session files MUST NEVER be exported, synced, or linked via symlink across profiles.
6. **Decoupling for Aggregators & UIs:**
   - Packages in `internal/` MUST keep business logic strictly separated from terminal presentation. Do not mix ANSI escape formatting or interactive terminal prompts directly into profile, quota, or chat management routines.
   - Return typed errors and clean structs so they can be consumed by the CLI, REST daemon, SSE stream, MCP server, or desktop UIs (Tauri, Wails, Svelte).
7. **Machine-Readable Contracts (`--json`):** Query and telemetry commands (`list`, `quota`, `quota history`, `stats`, `ai list`, `doctor`, `mcp status`, `catalog`, `alerts`, `workspace`, `logs`, `snapshot`, `login status`) MUST provide structured JSON contracts for external aggregators and dashboards.
8. **Secure Headless Invocation:** When executing the `language_server` or `agy` in background/headless mode, explicitly export `$HOME` / `%USERPROFILE%` directed to the profile root to enforce strict quota and identity isolation.
9. **Characterization Before Refactoring:** If legacy logic lacks test coverage, write characterization tests asserting baseline behavior before modifying the code.
10. **Consultative Dependencies:** When addressing missing capabilities, do NOT silently write complex in-house logic from scratch or install packages autonomously. Recommend battle-tested libraries alongside zero-dependency/stdlib alternatives, detailing maintenance and compatibility trade-offs for user approval.
11. **Stable & LTS Versioning:** Default to the newest stable release (favoring LTS) grounded on current date, strictly avoiding unapproved pre-releases (`alpha`/`beta`/`rc`/`canary`). Existing peer dependencies, runtime constraints, and `.agent/INVARIANTS.md` take strict precedence over latest major versions.
12. **Agent Documentation Language:** MUST write all agent-facing directives, skills, invariants, and task logs (`AGENTS.md`, `.agent/*`) in concise Technical English to minimize token consumption and maximize instruction compliance across LLM providers.

---

## Code Quality & Contrast Pairs

Preserve git blame on unmodified lines. Do not reformat adjacent legacy code.

### Contrast Pairs (DO / DON'T)

```go
// BAD: Renaming legacy payload fields breaks external consumers (MCP clients, REST API, scripts)
type ProfileSummary struct {
    ID   string `json:"id"`   // BROKEN: consumers expect 'name'
    Path string `json:"path"` // BROKEN: consumers expect 'profile_dir'
}

// GOOD: Preserves legacy keys while safely adding backward-compatible extensions
type ProfileSummary struct {
    Name       string `json:"name"`                  // Preserved invariant
    ProfileDir string `json:"profile_dir"`           // Preserved invariant
    ID         string `json:"id,omitempty"`          // Non-breaking addition
    Path       string `json:"path,omitempty"`        // Non-breaking addition
}
```

---

## Git Conventions

- **Surgical Commits:** Modify ONLY code essential to the active task.
- **Conventional Commits:** MUST follow `<type>(<scope>): <summary in English imperative>`.
  - `fix(isolation): symlink user gitconfig and ssh keys into profile`
  - `feat(app): add support for agy executable detection`
  - `test(gateway): add characterization test for session validation`
  - `docs(invariants): document delimiter rules in process matching`
- **Safety:** Local commits allowed upon user confirmation. **`git push` is prohibited** without explicit human instruction. Deployments and releases require human authorization.
