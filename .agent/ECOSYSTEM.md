# Multi-Repository Ecosystem Topology & Contracts (ECOSYSTEM.md)

> 🎯 **Purpose:** Canonical source of truth for topology, sibling services, shared contracts, and cross-repo boundaries in the agent and multi-profile ecosystem.
>
> ⚠️ **Mandatory Rules for the Agent:**
> 1. You MUST NOT break active contracts consumed by external clients or sibling repositories without an expand/contract deprecation period.
> 2. You MUST NOT attempt to directly mutate files outside this repository tree.
> 3. You MUST mock external service interactions in local automated tests.

---

## 1. Repository & Component Topology Matrix

| Component / Service | Role / Responsibility | Relationship | Location / Spec | Owner / Team |
| :--- | :--- | :---: | :--- | :--- |
| **`yegear1/multigravity-cli`** *(Current)* | Core CLI binary, background daemon, MCP server, AI gateway, and worktree manager | `Self` | `github.com/ye-dev/multigravity-cli` | Platform Core |
| **Antigravity IDE (`agy`)** | Official Google AI IDE and language server runtime | `Upstream & Target` | Local application binaries & `$PATH` | Google DeepMind / Cloud Code |
| **Google Cloud Code API** | Upstream AI completion and quota management backend | `Upstream (Dependency)` | `https://cloudcode-pa.googleapis.com` | Google Cloud |
| **External Agent Runners** | Autonomous CLI agents (`codex`, `claude`, `aider`, `cursor`) dispatched into worktrees | `Downstream (Consumer)` | Local tools & subshells | Community / End-Users |
| **Desktop / Web UIs** | Aggregator dashboards, Tauri/Wails shells, and browser UI | `Downstream (Consumer)` | `internal/server/ui` & `/ui` | Platform Frontend |

*(Relationships: `Self` [this repository], `Upstream (Dependency)` [this repo consumes it], `Downstream (Consumer)` [it consumes this repo], `Peer` [bidirectional interaction])*

---

## 2. Shared Contracts & Source of Truth

- **Contract Strategy:** Native Go structs in `internal/`, HTTP REST schemas, and standard MCP protocol specifications.
- **Contract Formats:**
  - **MCP Protocol:** Model Context Protocol specification `2024-11-05` (stdio and SSE/HTTP).
  - **OpenAI Compatible Gateway:** `/v1/chat/completions` and `/v1/models`.
  - **Anthropic Compatible Gateway:** `/v1/messages`.
  - **REST API:** Dual endpoints `/api/v1/...` and `/api/...` (SSE on `/events`).
  - **Language Server RPC:** ConnectRPC (`POST /exa.language_server_pb.LanguageServerService/*`).
- **Drift Prevention:** Every change affecting public payloads (`--json` output, MCP tool schemas, REST response structs) MUST retain existing keys and undergo regression testing before completing tasks.

---

## 3. Cross-System Interfaces Catalog

### A. Consumed by this Repository (Upstream Dependencies)

| Source Service | Protocol / Transport | Target Endpoint | Contract / Schema | Fallback / Blast Radius |
| :--- | :---: | :--- | :--- | :--- |
| **Language Server** | ConnectRPC over HTTPS | `POST /exa.language_server_pb...` | Protobuf over JSON/ConnectRPC | Failover to direct file read or CLI fallback |
| **Google OAuth2** | HTTPS / PKCE | `https://oauth2.googleapis.com/token` | OAuth2 Authorization Code PKCE | Fail-closed (`profile_unauthenticated`) |
| **Cloud Code API** | HTTPS / gRPC-Web | `POST /v1internal:fetchAvailableModels` | Google Internal RPC JSON | Failover to static supported models list |

### B. Exposed by this Repository (Downstream Consumers)

| Route / Tool | Consumer(s) | Payload / Schema | Breaking Change Risk | Deprecation Policy |
| :--- | :--- | :--- | :---: | :--- |
| `POST /v1/chat/completions` | Cursor, Aider, Codex CLI, scripts | OpenAI Chat Completion API | **HIGH** | Strict backward compatibility |
| `POST /v1/messages` | Claude Code, Anthropic SDKs | Anthropic Messages API with SSE | **HIGH** | Strict backward compatibility |
| `GET /api/v1/quota` | Dashboards, TUI, status monitors | JSON Quota Summary (`ActiveServer`) | **MEDIUM** | Additive-first |
| `MCP Tools (*)` | Claude Desktop, Antigravity, Cursor | JSON-RPC 2.0 / MCP 2024-11-05 | **HIGH** | Keep tool names and required args intact |

---

## 4. Blast Radius & Contract Evolution Rules

1. **Additive-First (Expand / Contract):**
   - NEVER delete, rename, or change data types of existing fields in active contracts directly.
   - When introducing new capabilities, add new fields as optional (`omitempty`).
   - Deprecate old fields over a documented cycle before removal.
2. **Hermetic Testing & Zero Cross-Repo Mutation:**
   - Automated test suites MUST NOT require live network access to upstream Google servers or real IDE processes.
   - Use test hooks (`SetTestHooks`, `SetRunnerTestHooks`) and mock HTTP/ConnectRPC responders.
3. **Machine-Readable Stability:**
   - Command flags supporting `--json` are considered public contracts and MUST NEVER output non-JSON text to `stdout` when `--json` is set.
