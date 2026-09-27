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
	"git ls-remote",
	"git ls-files",
	"git blame",
	"git shortlog",
	"git check-ignore",
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
	"bash -n",
	"sh -n",
	"diff",
	"cmp",
	"comm",
	"sort",
	"uniq",
	"cut",
	"tr",
	"column",
	"awk",
	"sed -n",
	"readlink",
	"realpath",
	"basename",
	"dirname",
	"id",
	"groups",
	"free",
	"lscpu",
	"lsblk",
	"lspci",
	"lsof",
	"netstat",
	"ss",
	"ip addr",
	"ip route",
	"ip link",
	"jq",
	"yq",
	"md5sum",
	"sha1sum",
	"sha256sum",
	"tar -tf",
	"tar -ztvf",
	"unzip -l",
	"gzip -l",
	"zcat",
	"less",
	"more",
	// Python & uv package manager / tools
	"python --version",
	"python3 --version",
	"python -V",
	"python3 -V",
	"uv --version",
	"uv version",
	"uv tree",
	"uv lock --check",
	"uv cache dir",
	"uv pip list",
	"uv pip freeze",
	"uv pip check",
	"uv pip show",
	"uv pip tree",
	"uv run ruff",
	"uv run ruff check",
	"uv run ruff format --check",
	"uv run pytest",
	"uv run pyright",
	"uv run mypy",
	"uv run python --version",
	"uv run python -V",
	// Node.js, npm & pnpm package managers
	"npm --version",
	"npm -v",
	"npm test",
	"npm run test",
	"npm run lint",
	"npm run check",
	"npm run typecheck",
	"npm list",
	"npm view",
	"npm audit",
	"npm outdated",
	"pnpm --version",
	"pnpm -v",
	"pnpm test",
	"pnpm run test",
	"pnpm run lint",
	"pnpm run check",
	"pnpm run typecheck",
	"pnpm list",
	"pnpm audit",
	"pnpm outdated",
	"pnpm eslint",
	"pnpm run eslint",
	"pnpm exec eslint",
	"pnpm exec ruff",
	"pnpm exec pyright",
	"pnpm why",
	"pnpm licenses list",
	"pnpm root",
	"pnpm store status",
	"pnpm store path",
	"pnpm env list",
	// Linters, formatters and test runners
	"ruff",
	"ruff check",
	"ruff format --check",
	"ruff rule",
	"ruff config",
	"ruff --version",
	"ruff -v",
	"pytest",
	"pyright",
	"pyright --version",
	"pyright -v",
	"mypy",
	"eslint",
	"eslint --version",
	"eslint -v",
	"npx eslint",
	"tsc --noEmit",
	"prettier --check",
	// Docker inspection (local machine, strictly read-only)
	"docker ps",
	"docker ps -a",
	"docker images",
	"docker image ls",
	"docker image inspect",
	"docker image history",
	"docker logs",
	"docker inspect",
	"docker stats --no-stream",
	"docker stats",
	"docker top",
	"docker port",
	"docker diff",
	"docker version",
	"docker info",
	"docker network ls",
	"docker network inspect",
	"docker volume ls",
	"docker volume inspect",
	"docker system df",
	"docker system info",
	"docker container ls",
	"docker container inspect",
	"docker container logs",
	"docker container top",
	"docker container port",
	"docker container diff",
	"docker compose ps",
	"docker compose logs",
	"docker compose config",
	"docker compose images",
	"docker compose top",
	"docker compose version",
	"docker compose ls",
	"docker-compose ps",
	"docker-compose logs",
	"docker-compose config",
	"docker-compose images",
	"docker-compose top",
	"docker-compose version",
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
