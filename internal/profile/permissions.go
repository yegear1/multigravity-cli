package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// DefaultReadOnlyCommands is the canonical inventory of read-only developer commands
var DefaultReadOnlyCommands = []string{
	// Git inspection and status
	"git status",
	"git log",
	"git diff",
	"git show",
	"git branch",
	"git tag",
	"git remote",
	"git rev-parse",
	"git describe",
	"git config --get",
	"git config --list",
	// Shell / POSIX / system inspection
	"ls",
	"cat",
	"head",
	"tail",
	"grep",
	"rg",
	"find",
	"which",
	"whereis",
	"where",
	"file",
	"stat",
	"wc",
	"uname",
	"pwd",
	"echo",
	"env",
	"printenv",
	"df",
	"du",
	"ps",
	"uptime",
	"date",
	"whoami",
	"hostname",
	"tree",
	// Dev tooling, package managers & linters (npm, pnpm, uv)
	"npm test",
	"npm run lint",
	"npm run check",
	"npm run typecheck",
	"npm list",
	"npm view",
	"npm audit",
	"npm outdated",
	"pnpm test",
	"pnpm run lint",
	"pnpm run check",
	"pnpm run typecheck",
	"pnpm list",
	"pnpm audit",
	"pnpm outdated",
	"uv run ruff check",
	"uv run ruff format --check",
	"uv run pytest",
	"uv run pyright",
	"uv run mypy",
	"uv pip list",
	"uv tree",
	// Linters and test runners directly
	"ruff check",
	"ruff format --check",
	"pytest",
	"pyright",
	"mypy",
	"eslint",
	"tsc --noEmit",
	"prettier --check",
	// Windows utilities
	"dir",
	"type",
	"Get-ChildItem",
	"Get-Content",
	"Get-Process",
	"Get-Item",
	"Get-Location",
}

// SeedDefaultPermissions ensures default read-only command grants exist in config.json
func SeedDefaultPermissions(configFile string) (bool, error) {
	if err := os.MkdirAll(filepath.Dir(configFile), 0755); err != nil {
		return false, fmt.Errorf("failed to create config dir: %w", err)
	}

	data := make(map[string]interface{})
	fileExists := false
	if raw, err := os.ReadFile(configFile); err == nil {
		fileExists = true
		_ = json.Unmarshal(raw, &data)
	}

	userSettings, ok := data["userSettings"].(map[string]interface{})
	if !ok {
		userSettings = make(map[string]interface{})
		data["userSettings"] = userSettings
	}

	gpg, ok := userSettings["globalPermissionGrants"].(map[string]interface{})
	if !ok {
		gpg = make(map[string]interface{})
		userSettings["globalPermissionGrants"] = gpg
	}

	rawAllow, ok := gpg["allow"].([]interface{})
	if !ok {
		rawAllow = []interface{}{}
	}

	existingSet := make(map[string]struct{})
	for _, item := range rawAllow {
		if s, ok := item.(string); ok {
			existingSet[s] = struct{}{}
		}
	}

	changed := false
	for _, c := range DefaultReadOnlyCommands {
		cStr := fmt.Sprintf("command(%s)", c)
		uStr := fmt.Sprintf("unsandboxed(%s)", c)

		if _, exists := existingSet[cStr]; !exists {
			rawAllow = append(rawAllow, cStr)
			existingSet[cStr] = struct{}{}
			changed = true
		}
		if _, exists := existingSet[uStr]; !exists {
			rawAllow = append(rawAllow, uStr)
			existingSet[uStr] = struct{}{}
			changed = true
		}
	}

	gpg["allow"] = rawAllow

	if changed || !fileExists {
		f, err := os.OpenFile(configFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return false, fmt.Errorf("failed to open config file: %w", err)
		}
		defer f.Close()

		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		if err := enc.Encode(data); err != nil {
			return false, fmt.Errorf("failed to write config file: %w", err)
		}
		return true, nil
	}

	return false, nil
}
