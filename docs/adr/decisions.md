---
author_type: human
author: Architecture Team
reviewed_by: Human Lead
date: 2026-10-14
status: accepted
type: architectural-decision-log
---

# Architectural Decisions Log

> 🏛️ **Progressive Disclosure:** Historical and cumulative decisions live here so AI agents do not load them into working context on every interaction.
> Active runtime contracts stay in [`.agent/NOTES.md`](../../.agent/NOTES.md). Formal individual specs live in [`.agent/adr/`](../../.agent/adr/).

---

## How to Record Architectural Decisions

1. When a substantive design decision or trade-off is approved, append it here chronologically.
2. In `.agent/NOTES.md`, maintain only **Active Contracts** and link to this file for background rationale.
3. Keep specifications for future complex backlog items documented here to prevent bloating `.agent/TASK.md`.

---

## Cumulative Decisions

### 2026-10-14 [Codex CLI Runner Integration]

- **Context:** Need to support OpenAI Codex CLI (`codex`) as a first-class runner alongside `agy` for headless autonomous tasks and interactive agent sessions.
- **Decision:**
  - `internal/agent`: `DetectAgentType` identifies `codex` binaries; `BuildAgentEnv` isolates `CODEX_HOME` in `<profileDir>/.codex` and injects `OPENAI_BASE_URL` pointing to the local multigravity gateway.
  - `internal/dispatch`: `ResolveAgentCommandWithCwd` synthesizes `codex exec -C <cwd> --sandbox workspace-write "<prompt>"` for autonomous tasks. Respects explicit user flags without duplication.
  - Worktree integration: ephemeral git worktree path is injected into `-C <worktreePath>` ensuring confinement.

### 2026-10-14 [ConnectRPC AI Archival & Project UUID Mapping]

- **Context:** Conversations unarchived or synchronized across profiles (`ai sync`) were not appearing in the Antigravity IDE UI because the Language Server maintains an in-memory cache and binary protobuf summaries (`agyhub_summaries_proto.pb`).
- **Decision:**
  - Archiving/unarchiving while the profile is running must invoke `POST /exa.language_server_pb.LanguageServerService/UpdateConversationAnnotations` over ConnectRPC using the local CSRF token.
  - Project ID in Antigravity conversation blobs is a 36-byte UUID v4 string. Direct byte-level replacement preserves protobuf wire integrity while matching the destination profile's workspace mapping.

### 2026-10-04 [Single Registration for Dual HTTP Route Prefixes]

- **Context:** Server routes were duplicated across `/api/v1/...` and `/api/...` (100+ manual registrations).
- **Decision:** Introduced `registerPair` helper in `internal/server/routes.go` to register both `/api/v1` and legacy `/api` aliases simultaneously. Canonical SSE events remain on `GET /events` and gateway completions on `/v1/chat/completions` and `/v1/messages`.

### 2026-09-27 [Native Stdio & HTTP MCP Server]

- **Context:** External agent tools (Claude Desktop, Cursor, Aider, Antigravity) needed native MCP protocol support for worktrees, tasks, quotas, and profile diagnostics.
- **Decision:** Built `internal/mcp` implementing JSON-RPC 2.0 with MCP specification `2024-11-05`. Stdio transport reserves `stdout` exclusively for single-line JSON-RPC messages and routes logs to `stderr`. HTTP transport supports both stateless `POST /mcp` and stateful SSE streams.

### 2026-09-26 [Headless Daemon & Vault Isolation]

- **Context:** Headless execution and multi-profile priming required running language servers and CLI prompts without graphical Electron windows and without altering OS keyrings.
- **Decision:** Built `internal/headless` manager. Direct OAuth2 PKCE login saves tokens in `<profileDir>/.gemini/antigravity-cli/antigravity-oauth-token` (0600 mode) and sets `GEMINI_FORCE_FILE_STORAGE=true`. The OS keychain/credential manager is never modified, enabling concurrent multi-profile execution.

---

## Architectural Roadmaps & Backlog Task Specifications

### [Spec 03.2] Dispatch Apply (`multigravity dispatch apply <task-id>`)
- **Objective:** Provide parity with `codex apply`, cleanly merging audited worktree diffs into the target base branch.
- **Mechanics:**
  1. Validate that the task status is `completed` and the diff is clean.
  2. Execute controlled patch application (`git apply` or cherry-pick) on the base repository working tree.
  3. Prune ephemeral worktree upon success (`multigravity worktree prune`).

### [Spec 03.3] Agent Process Sandboxing (`agent-guard wrap --repo`)
- **Objective:** Enforce filesystem and execution boundaries for runners lacking native sandbox flags.
- **Mechanics:**
  - Wrap agent execution with `agent-guard wrap --role developer --repo <path> -- <cmd>` when `--sandbox` or `--repo` is specified.
  - Intercept subshell commands attempting destructive mutations outside the repo tree.

### [Spec 03.4] Quota-Aware Route Recommendation (`multigravity route`)
- **Objective:** Provide deterministic profile and model recommendations based on measured quotas and reset windows.
- **Mechanics:**
  - CLI command `multigravity route [--weight light|medium|heavy] [--json]`.
  - Evaluates remaining fractions and reset times across active profiles.

### [Spec 03.5] Dispatch Post-Verification Hook (`--verify <cmd>`)
- **Objective:** Run validation commands (e.g. tests or linters) inside the worktree before marking a task ready.
- **Mechanics:**
  - Flag `--verify <cmd>` in `dispatch run`. If verification exits non-zero, mark task as `audit_failed` and capture output.

### [Spec 03.6] Governance Context Injection
- **Objective:** Automatically inject governance awareness (`AGENTS.md`, `.agent/TASK.md`) into agent prompts when dispatched into target repositories.

### [Spec 03.7] Task Retry with Verification Feedback Loop (`--retry <n>`)
- **Objective:** Automatically retry tasks when post-session verification commands fail, feeding verification failure output into subsequent prompt sessions without polluting branch history.
- **Mechanics:**
  - Flag `--retry <n>` in `dispatch run` and `Retry` in `DispatchOptions` / `Task`.
  - Reuses the existing ephemeral git worktree and branch in place across retry attempts.
  - Generates feedback prompts via `SynthesizeRetryPrompt` embedding the verification command, exit code, failure output, and original task instructions.
  - Updates `Task.RetryCount`, logs retry markers (`=== Retrying Task (attempt X/Y) ===`), and exposes live streaming across retry sessions.
