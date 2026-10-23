# INVARIANTS.md — System Invariants and Untouchable Rules

> 🧱 **Chesterton's Fence Principle:**
> *"Never remove a rule, filter, or unusual piece of code until you understand why it was put there."*
>
> This file is the source of truth for **legacy system behaviors, platform quirks, and rigid contracts** that may appear redundant, suboptimal, or unusual at first glance, but **MUST NOT BE CHANGED** without explicit user consent.

---

## How to Use this File (for the Agent)

1. **Check before any change:** Always consult this file before planning changes to profile structures, launchers, authentication, or gateway routes.
2. **Document implicit rules upon discovery:** When investigating bugs or platform edge cases, document findings here immediately.
3. **Do not confuse with `NOTES.md`:** `NOTES.md` tracks recent active decisions and temporary contracts. `INVARIANTS.md` tracks **strict constraints and untouchable legacy quirks**.

---

## 1. CLI Naming and Argument Contracts

| Touchpoint | Invariant Rule | Rationale / Impact |
| :--- | :--- | :--- |
| `validate_name` | `^[a-zA-Z0-9][a-zA-Z0-9-]*$` | Profile names map directly to OS directories and `.desktop`/`.app`/`.lnk` files. Spaces or symbols break shell escaping and XDG path specifications. |
| Profile Invocation | `multigravity <name> [args...]` | When the first argument does not match an internal command, it MUST be dispatched as a profile launch with argument passthrough. |
| Antigravity Flags | `--user-data-dir` and `--extensions-dir` | Canonical Electron / Code OSS flags for user data and extension isolation. NEVER replace with non-standard flags. |

---

## 2. Environment Variables & Canonical Directories

- **`MULTIGRAVITY_HOME`:** Base storage directory for profiles. Default: `$HOME/AntigravityProfiles` (or `%USERPROFILE%\AntigravityProfiles`).
- **`MULTIGRAVITY_APP`:** Allows overriding the exact executable path of the IDE.
- **`HOME` Isolation:**
  - Antigravity writes AI assistant configurations to `~/.gemini` and `~/.antigravity`.
  - Redirecting `HOME` to the profile directory is how the CLI strictly isolates AI sessions and token storage.
  - **Caution:** Removing `HOME` redirection breaks AI session isolation. The correct solution for Git/SSH access is **preserving essential host dotfiles (`.gitconfig`, `.ssh`) via symlink** inside the profile, never bypassing `HOME` isolation.

---

## 3. Platform-Specific Behavior

### macOS (Darwin)
- **Keychains:** `Library/Keychains` MUST be symlinked to `$HOME/Library/Keychains` if absent; otherwise macOS refuses to persist credentials.
- **Shortcuts:** Created at `$HOME/Applications/Multigravity <name>.app` matching canonical macOS bundle structure with `Contents/MacOS/run`, `Info.plist`, and embedded `Contents/Resources/icon.icns` (`//go:embed`).

### Linux
- **XDG Base Directory:** `XDG_CONFIG_HOME`, `XDG_CACHE_HOME`, `XDG_DATA_HOME`, and `XDG_STATE_HOME` MUST point to dedicated subdirectories inside the profile to prevent host pollution.
- **Shortcuts:** Generated in `~/.local/share/applications/multigravity-<name>.desktop` pointing to wrapper script `~/.local/share/multigravity/launchers/<name>.sh` with icon at `~/.local/share/multigravity/icon.png`. The wrapper exports `MULTIGRAVITY_LAUNCH_WAIT=1` and strips `ELECTRON_RUN_AS_NODE` prior to `exec`: WSL (`wslg.exe`) terminates GUI sessions if shortcut processes exit prematurely, while Cursor/VS Code export `ELECTRON_RUN_AS_NODE=1`, causing Electron binaries to reject `--user-data-dir`. WSL shortcuts cannot use `--cd "~"` (`Wsl/E_INVALIDARG`); absolute paths (`/home/<user>`) are mandatory.

### Windows
- **User Environment:** `$env:USERPROFILE`, `$env:APPDATA`, and `$env:LOCALAPPDATA` MUST point to profile subfolders.
- **Shortcuts:** `.lnk` files created in `$env:APPDATA\Microsoft\Windows\Start Menu\Programs\` with `IconLocation` pointing to `%APPDATA%\multigravity\icon.ico` (extracted from embedded assets).

---

## 4. MCP (Model Context Protocol) Sharing

- **Global Config vs Tokens:** Antigravity stores MCP server configurations in `~/.gemini/config/mcp_config.json` and tool schemas in `~/.gemini/antigravity/mcp`. Authentication tokens (e.g. `jetski-standalone-oauth-token`) live at the root of `~/.gemini/`, **NOT** inside `config/`.
- **Safe Symlinks:** Symlinking `mcp_config.json` and `antigravity/mcp` does **NOT** expose host AI authentication tokens.
- **Opt-out (`.isolated_mcp`):** Any profile containing the `.isolated_mcp` sentinel file MUST retain private MCP definitions without host linking.

---

## 5. Skills and Plugins Sharing

- **Global Customizations vs Tokens:** Skills reside in `~/.gemini/config/skills/` and Plugins in `~/.gemini/config/plugins/` as markdown playbooks (`SKILL.md`), scripts, and templates without credentials.
- **Safe Symlinks:** Symlinking `skills` and `plugins` shares capability playbooks without exposing sessions or tokens.
- **Opt-out (`.isolated_skills`):** Profiles containing `.isolated_skills` keep their `skills/` and `plugins/` directories strictly isolated.

---

## 6. Configuration & Permissions Sharing (`config.json`)

- **Global Assistant Permissions:** Execution grants (persistent approvals for MCP tools, shell commands, and Git via *"Always allow"*) reside in `~/.gemini/config/config.json` under `.userSettings.globalPermissionGrants.allow`.
- **Zero Credentials:** `config.json` holds only UI preferences (e.g. `themeMode`, `conversationWidth`) and permission grants. OAuth tokens are never stored here.
- **Real-Time Symlinking:** Symlinking `config.json` ensures permissions granted in one profile reflect across all profiles instantly.
- **Opt-out (`.isolated_config`):** Profiles with `.isolated_config` retain private `config.json` instances.
- **Read-Only Permission Seeding:** Seeded profiles guarantee canonical read-only commands (`git`, posix utilities, linters, `docker ps/inspect`) in "Always allow" without duplicates.

---

## 7. Developer Auth Sharing & PATH Preservation

- **Git & GitHub CLI (`gh`):** GitHub CLI auth (`~/.config/gh` or `%APPDATA%\GitHub CLI`) and `~/.git-credentials` are shared via symlink by default to enable git operations in integrated terminals.
- **Opt-out (`.isolated_gh` / `.isolated_dotfiles`):** Profiles with these sentinels keep GitHub CLI auth strictly isolated for separate corporate/personal identities.
- **PATH Preservation:** Antigravity enriches `PATH` with host user binary directories (`~/.local/bin`, `~/.cargo/bin`, etc.) at launch time (`launch_profile`), ensuring binary access without breaking `HOME` isolation.

---

## 8. Canonical Antigravity Directory Mapping

Reference mapping of where Antigravity and AI subsystems store data, tokens, and artifacts relative to `$HOME`:

| Resource | Canonical Path in Antigravity | Purpose / Content | Isolation / Sensitivity |
| :--- | :--- | :--- | :--- |
| **Skills** | `~/.gemini/config/skills/<name>/SKILL.md` | Agent playbooks, scripts, instructions | Safe to share via symlink. Opt-out: `.isolated_skills`. |
| **Plugins** | `~/.gemini/config/plugins/<name>/` | IDE plugins and AI extension manifests | Safe to share via symlink. Opt-out: `.isolated_skills`. |
| **Permissions** | `~/.gemini/config/config.json` | `.globalPermissionGrants.allow` grants | Safe to share via symlink (zero tokens). Opt-out: `.isolated_config`. |
| **MCP Config** | `~/.gemini/config/mcp_config.json` | Server endpoints and definitions | Safe to share via symlink. Opt-out: `.isolated_mcp`. |
| **MCP Schemas** | `~/.gemini/antigravity/mcp/<server>/` | Local tool schemas and RPC specs | Safe to share via symlink. Opt-out: `.isolated_mcp`. |
| **Auth Tokens** | Root of `~/.gemini/` (`jetski-standalone-oauth-token`, `installation_id`) + OS Keyrings | Session tokens and AI login state | **STRICTLY ISOLATED.** Outside `config/`. NEVER share or export. |
| **Conversations** | `~/.gemini/antigravity/` (`conversations/*.db`, `brain/<uuid>/*`) | Chat logs, plans, agent memory | Managed via `multigravity ai export/sync/import` with token scrubbing. Snapshots store restore points under `$MULTIGRAVITY_HOME/.snapshots/`. |
| **IDE Settings** | `$MULTIGRAVITY_HOME/<profile>/User/settings.json` | Theme, editor font, shortcuts | Profile-specific. |
| **Dev Auth** | `~/.config/gh/`, `~/.git-credentials`, `~/.gitconfig`, `~/.ssh` | Git identity, SSH keys, GitHub CLI | Shared by default for dev ergonomics; opt-out via `.isolated_gh`. |

---

## 9. Prohibition of Global Keyring / Credential Manager Manipulation

- **Chesterton's Fence:** Simplistic community tools dynamically overwrite the global OS Credential Manager entry (`gemini:antigravity`) before each call and restore it afterward.
- **Why Strictly Prohibited in Multigravity:**
  1. **Concurrency Hazard:** OS keyrings are global singletons per OS user. Running concurrent agents triggers race conditions, corrupting credentials.
  2. **Session Loss Risk:** Crashes, hard kills (`kill -9`), or timeouts leave the host unauthenticated or logged into secondary accounts.
  3. **Process Isolation is Superior:** Multigravity isolates via `--user-data-dir`, `--extensions-dir`, and independent `$HOME` directories.
- **Profile File Vault (`multigravity login` & IDE Sign-In):** Headless authentication stores refresh tokens exclusively in `<profile>/.gemini/antigravity-cli/antigravity-oauth-token` and `<profile>/.gemini/jetski-standalone-oauth-token` (mode `0600`). All isolated profile launches (IDE, agent, headless) unconditionally export `GEMINI_FORCE_FILE_STORAGE=true`, refresh `<profile>/.gemini/cache/antigravity-keyring-unavailable` (mode `0600`), and on Linux export `WSL_INTEROP=/dev/null` (when `WSL_INTEROP` and `WSL_DISTRO_NAME` are unset) so Antigravity's `language_server` (`codeassistclient.NewCompositeTokenStorage`) bypasses the shared OS D-Bus Secret Service keyring (`service="antigravity"`) and persists IDE sign-ins directly to `<profile>/.gemini/jetski-standalone-oauth-token`. The host credential manager is never touched.
- **`multigravity exec` Concurrency:** Fan-out runs existing headless runners per profile. It does not touch the host keyring or alter worktrees.

---

## 10. IDE Graphical Logs and Token Redaction

- Electron logs to `<user-data-dir>/logs/main.log`. The embedded language server logs to `language_server.log`.
- `multigravity logs` and `GET /api/v1/profiles/{name}/ide/logs` read strictly `main.log`.
- The spawn line in `main.log` contains `--csrf_token` and `--host_bridge_token`. Any exposed reading MUST redact these values to `[redacted]`.
- **Prohibition on CSRF Serialization:** CSRF tokens MUST NEVER be exposed or serialized in `multigravity quota`, `multigravity quota --json`, the `quota_summary` MCP tool, or `GET /api/v1/quota*` endpoints.

---

## 11. MCP & Skills Catalog

- `multigravity catalog` and `GET /api/v1/catalog` are strictly read-only.
- Environment variables, headers, and arguments from `mcp_config.json` MUST be redacted from text and JSON output. URL query strings and user credentials are stripped.

---

## 12. Gateway Credentials & Router Quotas

- Access tokens for completions originate from the profile vault: `antigravity-oauth-token` takes priority over `jetski-standalone-oauth-token`.
- Inbound `Authorization` headers received by `serve` are never forwarded upstream.
- Unauthenticated profiles return HTTP 401 `profile_unauthenticated` (or `authentication_error` on `/v1/messages`). If failover exhausts the pool solely due to missing credentials, the status code MUST remain 401, NOT 429.
- `remaining_fraction: 1` is not the initial state. Without language server telemetry, `quota_known` is false and fraction is 0. Quota 0 never reverts to 1.

---

## 13. Gateway Model Identifiers

- Supported model IDs match the live response of `POST /v1internal:fetchAvailableModels` in Cloud Code.
- `gemini-3.1-pro-high` is replaced upstream by `gemini-pro-agent`.
- Gemini 3.6 variants use IDs `gemini-3.6-flash-low`, `gemini-3.6-flash-medium`, and `gemini-3.6-flash-high`.
- Gemini 3.7 and 3.8 tiered IDs are `gemini-3.7-flash-tiered` and `gemini-3.8-flash-tiered`. Model suffixes (`-low`, `-medium`, `-high`) and OpenAI `reasoning_effort` map to `generationConfig.thinkingConfig.thinkingLevel` (`LOW`, `MEDIUM`, `HIGH`). `MINIMAL` is rejected upstream and must not be sent.

---

## 14. Native MCP Server (`internal/mcp`)

- Stdio transport (`multigravity mcp serve`) reserves `stdout` strictly for single-line JSON-RPC messages separated by `\n`. All diagnostics and logs MUST go to `stderr`.
- MCP handshake follows specification `2024-11-05`, reporting execution errors with `isError: true` in `CallToolResult` rather than aborting the connection.
- HTTP transport supports stateless `POST /mcp` and stateful SSE streams.

---

## 15. AI Conversation Lifecycle: Archival & Project IDs

- **Archival Invariant:**
  - Archiving state is maintained across SQLite (`conversations/<cid>.db`), text annotations (`annotations/<cid>.pbtxt`), and binary summary caches (`agyhub_summaries_proto.pb`).
  - When the profile is running, modifying `.pbtxt` directly is ignored by the in-memory cache. Archival operations MUST call `POST /exa.language_server_pb.LanguageServerService/UpdateConversationAnnotations` over ConnectRPC using the local CSRF token.
- **Project ID Mapping (`ai sync`):**
  - Project IDs are 36-character UUID v4 strings stored in protobuf blobs.
  - Sinking conversations into a target profile requires updating the `projectId` to match the target's workspace UUID (`~/.gemini/config/projects/<id>.json`). Direct 36-byte replacement preserves protobuf wire integrity.

---

## 16. Process Detection Without Prefix Collisions

- **Strict Path Delimitation (`lineMatchesPath`):** Process inspection routines (POSIX `ps` or Windows WMI/PowerShell) MUST validate boundary delimiters (`/`, `\`, spaces, quotes, `=`, line boundaries) around `profileDir` and `userDataDir`.
- **Prefix Collision Prevention:** Substring matching causes false positives between profiles where one is a prefix of another (e.g. `yegear2` active falsely reporting `yegear` as active). All checks MUST use `lineMatchesPath`.
