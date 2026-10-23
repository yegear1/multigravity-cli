---
id: "k-001"
title: "Bypass Global D-Bus Keyring in Antigravity language_server on Linux"
date: 2026-10-23
tags:
  - "auth"
  - "keyring"
  - "linux"
  - "language_server"
  - "isolation"
severity: "high"
affected_components:
  - "internal/profile/launcher.go"
  - "internal/profile/layout.go"
  - "internal/agent/env.go"
  - "internal/headless/manager.go"
  - "internal/quota/headless.go"
symptoms:
  - "Closing and reopening an Antigravity IDE profile on Linux loads the Google account of the last profile that signed in"
  - "Multiple IDE profiles work with different accounts while open in memory, but revert to a single shared account after restart"
  - "<profile>/.gemini/jetski-standalone-oauth-token is never created after signing in through the IDE UI on Linux"
---

# [k-001] Bypass Global D-Bus Keyring in Antigravity language_server on Linux

## Problem / Symptom
On Linux desktop sessions (`gnome-keyring-daemon` / `org.freedesktop.secrets`), multiple Antigravity IDE profiles can hold distinct Google accounts simultaneously in memory while open, but closing a profile and reopening it from scratch (`multigravity launch <profile>`) causes it to load whichever Google account authenticated most recently across any profile. Additionally, `<profile>/.gemini/jetski-standalone-oauth-token` is never written to disk during IDE sign-in.

```text
# From /opt/antigravity/resources/bin/language_server (codeassistclient.NewCompositeTokenStorage):
# KeyringTokenStorage uses github.com/zalando/go-keyring with a hardcoded service name "antigravity"
# over the user's session-wide D-Bus Secret Service (unix:path=/run/user/1000/bus).
```

---

## Approved Solution
Enforce file-based token storage (`$HOME/.gemini/jetski-standalone-oauth-token`) across all isolated profile launches (`internal/profile/launcher.go`, `internal/agent/env.go`, `internal/headless/manager.go`, `internal/quota/headless.go`) and profile layout initialization (`internal/profile/layout.go`):

1. Refresh `<profileDir>/.gemini/cache/antigravity-keyring-unavailable` (mode `0600`) in `EnsureProfileLayout` so `language_server`'s `keyringRecentlyUnavailable()` check selects `fileTokenStorage`.
2. Export `GEMINI_FORCE_FILE_STORAGE=true` unconditionally for all isolated profile environments (consumed by `gemini-cli` / `agy`).
3. On Linux (`runtime.GOOS == "linux"`), export `WSL_INTEROP=/dev/null` when `WSL_INTEROP` and `WSL_DISTRO_NAME` are unset so `language_server`'s `shouldBypassKeyring()` (`wslDetector.Detect()`) permanently sets `useFileStatic = true` without expiring after 1 hour.

```go
envMap["GEMINI_FORCE_FILE_STORAGE"] = "true"
if runtime.GOOS == "linux" && envMap["WSL_INTEROP"] == "" && envMap["WSL_DISTRO_NAME"] == "" {
	envMap["WSL_INTEROP"] = "/dev/null"
}
```

**Root Cause:** The Antigravity Go `language_server` binary does not read `GEMINI_FORCE_FILE_STORAGE` (`0` occurrences in binary). Its `codeassistclient.NewCompositeTokenStorage` defaults to `zalando/go-keyring` under the global D-Bus Secret Service key `service="antigravity"` unless `shouldBypassKeyring()` or `keyringRecentlyUnavailable()` returns `true`. `WSL_INTEROP` is inspected exclusively by `wslDetector.Detect()` inside `language_server` (`0` occurrences in Electron's `app.asar`), cleanly forcing per-profile `$HOME/.gemini/jetski-standalone-oauth-token` storage.

---

## What NEVER to Do
- ❌ **Anti-Pattern:** Clearing `DBUS_SESSION_BUS_ADDRESS` or `XDG_RUNTIME_DIR` to break `gnome-keyring` access.
  - *Why:* `dbusDetector.Detect()` in `language_server` still falls back to `os.Stat("$XDG_RUNTIME_DIR/bus")` (`/run/user/1000/bus`), and stripping D-Bus or `XDG_RUNTIME_DIR` breaks Electron's `xdg-desktop-portal` native file pickers and `xdg-open` OAuth browser launches.
- ❌ **Anti-Pattern:** Injecting fake `SSH_TTY` or `SSH_CLIENT` variables to trigger `sshDetector.Detect()`.
  - *Why:* Integrated terminals, shell prompts, `git`, and `ssh-agent` alter their behavior when `SSH_*` variables are present.

---

## Verification
```bash
go test -v ./internal/profile/... ./internal/agent/... ./internal/headless/... ./internal/quota/...
```
