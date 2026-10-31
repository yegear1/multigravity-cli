# Contributing to multigravity-cli

Thank you for your interest in contributing to **multigravity-cli**! We welcome bug reports, improvements, documentation updates, and pull requests.

## ⚖️ Core Principles & Invariants

Before contributing code, please note the key invariants governing this project:

1. **Profile Backward Compatibility:** Never alter the filesystem layout or database schema of an existing profile in ways that invalidate data already stored in `~/AntigravityProfiles`.
2. **Secrets & Security Isolation:** Never export, synchronize, or symlink active OAuth tokens (`jetski-standalone-oauth-token`, `antigravity-oauth-token`), OS keyring credentials, or session cookies across profiles.
3. **Platform Parity:** Every CLI command, flag, and path operation must work consistently across Linux, macOS, and Windows.
4. **Machine-Readable Contracts:** Telemetry, query, and management commands (`list`, `quota`, `doctor`, `ai list`, `workspace`, etc.) must provide structured `--json` output for external aggregators, MCP clients, and desktop frontends.
5. **Decoupled Architecture:** Business logic under `internal/` must remain cleanly decoupled from terminal presentation (ANSI colors, interactive prompts) so routines can be safely invoked by CLI, REST daemon, SSE streams, or MCP tools.

---

## 🛠️ Development Setup

### Prerequisites

- **Go 1.27.1+** declared in `go.mod`
- **Git**
- **Make** (optional, recommended for build automation)
- **Bash / POSIX shell** (Linux/macOS) or **PowerShell** (Windows)

### Building Locally

```bash
# Clone the repository
git clone https://github.com/yegear1/multigravity-cli.git
cd multigravity-cli

# Build binary into bin/
go build -o bin/multigravity ./cmd/multigravity

# Or using Makefile
make build
```

---

## 🧪 Verification & Testing

All pull requests must pass local verification before submission:

```bash
# 1. Run unit tests
go test -v ./...

# 2. Build local binary
go build -o bin/multigravity ./cmd/multigravity

# 3. Validate bash scripts syntax
bash -n multigravity install.sh uninstall.sh

# 4. Check git whitespace / formatting
git diff --check
```

---

## 📝 Commit Message Guidelines

We enforce the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
<type>(<scope>): <short summary in imperative mood>

[optional body]

[optional footer(s)]
```

### Approved Types

- `feat`: A new feature or capability
- `fix`: A bug fix or patch
- `docs`: Documentation changes only
- `test`: Adding or updating tests
- `refactor`: Code changes that neither fix a bug nor add a feature
- `perf`: Code changes that improve performance
- `chore`: Maintenance, build tasks, or dependency updates

### Examples

- `fix(gateway): avoid false positive rate limit detection on endpoint port numbers`
- `feat(cmd): add --json flag support to workspace export command`
- `test(profile): add characterization test for process boundary matching`
- `docs(readme): update quickstart installation instructions`

---

## 📬 Submitting a Pull Request

1. Fork the repository and create your branch from `main`:
   ```bash
   git checkout -b feat/your-feature-name
   ```
2. Implement your changes keeping diffs focused and surgical.
3. Add characterization or unit tests covering new or modified behavior.
4. Ensure all automated validation steps pass (`go test -v ./...`, `git diff --check`).
5. Open a Pull Request referencing any related issues and detailing the motivation, blast radius, and verification steps.
