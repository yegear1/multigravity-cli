package catalog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

const (
	StatusOK      = "ok"
	StatusWarning = "warning"
	StatusError   = "error"
)

// Check is one diagnostic assertion, same shape as doctor.
type Check struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

// Server is one MCP server definition. Env, headers, and args are never copied.
type Server struct {
	Name      string `json:"name"`
	Source    string `json:"source"`
	Transport string `json:"transport"`
	Command   string `json:"command,omitempty"`
	URL       string `json:"url,omitempty"`
	Status    string `json:"status"`
	Detail    string `json:"detail,omitempty"`
}

// Skill is one skills directory entry.
type Skill struct {
	Name        string `json:"name"`
	Source      string `json:"source"`
	Description string `json:"description,omitempty"`
	Path        string `json:"path"`
	Status      string `json:"status"`
	Detail      string `json:"detail,omitempty"`
}

// Plugin is one plugins directory entry.
type Plugin struct {
	Name   string `json:"name"`
	Source string `json:"source"`
	Path   string `json:"path"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// ProfileBinding is the existing share/isolate mode for one profile.
type ProfileBinding struct {
	Profile string `json:"profile"`
	MCP     string `json:"mcp"`
	Skills  string `json:"skills"`
}

// Report is the inventory plus health of MCP servers and skills.
type Report struct {
	Scope    string           `json:"scope"`
	Checks   []Check          `json:"checks"`
	Servers  []Server         `json:"servers"`
	Skills   []Skill          `json:"skills"`
	Plugins  []Plugin         `json:"plugins"`
	Profiles []ProfileBinding `json:"profiles,omitempty"`
	Errors   int              `json:"errors"`
	Warnings int              `json:"warnings"`
	Healthy  bool             `json:"healthy"`
}

type mcpFile struct {
	MCPServers map[string]mcpServer `json:"mcpServers"`
}

type mcpServer struct {
	Command string `json:"command"`
	URL     string `json:"url"`
	HTTPURL string `json:"httpUrl"`
}

type pluginMeta struct {
	Name string `json:"name"`
}

// Inspect inventories MCP servers, skills, and plugins and reports their health.
// An empty profile name reads the host catalog and each profile's sharing mode.
// Isolated or standalone profiles add their private entries. Sharing itself is unchanged.
func Inspect(profileName string) (*Report, error) {
	report := &Report{
		Scope:   "host",
		Checks:  []Check{},
		Servers: []Server{},
		Skills:  []Skill{},
		Plugins: []Plugin{},
	}

	if profileName != "" {
		if err := config.ValidateProfileName(profileName); err != nil {
			return nil, err
		}
		if !profile.ProfileExists(profileName) {
			return nil, fmt.Errorf("profile %q does not exist", profileName)
		}
		report.Scope = profileName
		root := config.GetProfileDir(profileName)
		appendTree(report, profileName, root)
		mcpMode, skillsMode := bindingModes(profileName)
		report.Profiles = []ProfileBinding{{
			Profile: profileName,
			MCP:     mcpMode,
			Skills:  skillsMode,
		}}
		appendBinding(report, profileName, false)
		finish(report)
		return report, nil
	}

	appendTree(report, "host", hostHome())
	names, err := profile.ListProfiles()
	if err != nil {
		return nil, err
	}
	report.Profiles = []ProfileBinding{}
	for _, name := range names {
		mcpMode, skillsMode := bindingModes(name)
		report.Profiles = append(report.Profiles, ProfileBinding{
			Profile: name,
			MCP:     mcpMode,
			Skills:  skillsMode,
		})
		appendBinding(report, name, true)
		root := config.GetProfileDir(name)
		if mcpMode == "isolated" || mcpMode == "standalone" {
			readMCP(report, name, filepath.Join(root, ".gemini", "config", "mcp_config.json"))
		}
		if skillsMode == "isolated" || skillsMode == "standalone" {
			readSkills(report, name, filepath.Join(root, ".gemini", "config", "skills"))
			readPlugins(report, name, filepath.Join(root, ".gemini", "config", "plugins"))
		}
	}
	finish(report)
	return report, nil
}

// Run writes a doctor-style diagnosis followed by the inventory.
func Run(w io.Writer, report *Report) {
	fmt.Fprintln(w, "Checking MCP servers and skills...")
	for _, check := range report.Checks {
		switch check.Status {
		case StatusOK:
			fmt.Fprintf(w, "  [✓] %s\n", check.Message)
		case StatusWarning:
			fmt.Fprintf(w, "  [!] %s\n", check.Message)
		case StatusError:
			fmt.Fprintf(w, "  [✗] %s\n", check.Message)
		}
	}

	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "MCP servers")
	if len(report.Servers) == 0 {
		fmt.Fprintln(w, "  (none)")
	}
	for _, srv := range report.Servers {
		target := srv.Command
		if srv.Transport == "http" {
			target = srv.URL
		}
		fmt.Fprintf(w, "  %s  %s  %s  %s\n", srv.Source, srv.Name, srv.Transport, target)
	}

	fmt.Fprintln(w, "Skills")
	if len(report.Skills) == 0 {
		fmt.Fprintln(w, "  (none)")
	}
	for _, skill := range report.Skills {
		fmt.Fprintf(w, "  %s  %s", skill.Source, skill.Name)
		if skill.Description != "" {
			fmt.Fprintf(w, "  %s", skill.Description)
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "Plugins")
	if len(report.Plugins) == 0 {
		fmt.Fprintln(w, "  (none)")
	}
	for _, plugin := range report.Plugins {
		fmt.Fprintf(w, "  %s  %s\n", plugin.Source, plugin.Name)
	}

	fmt.Fprintln(w, "")
	if report.Errors == 0 {
		if report.Warnings == 0 {
			fmt.Fprintln(w, "✓ MCP servers and skills look healthy.")
		} else {
			fmt.Fprintf(w, "Found %d warning(s). Sharing still works, but some tools may fail to start.\n", report.Warnings)
		}
	} else {
		fmt.Fprintf(w, "✗ Found %d error(s) and %d warning(s). Please fix the errors above.\n", report.Errors, report.Warnings)
	}
}

func finish(report *Report) {
	sort.Slice(report.Servers, func(i, j int) bool {
		if report.Servers[i].Source == report.Servers[j].Source {
			return report.Servers[i].Name < report.Servers[j].Name
		}
		return report.Servers[i].Source < report.Servers[j].Source
	})
	sort.Slice(report.Skills, func(i, j int) bool {
		if report.Skills[i].Source == report.Skills[j].Source {
			return report.Skills[i].Name < report.Skills[j].Name
		}
		return report.Skills[i].Source < report.Skills[j].Source
	})
	sort.Slice(report.Plugins, func(i, j int) bool {
		if report.Plugins[i].Source == report.Plugins[j].Source {
			return report.Plugins[i].Name < report.Plugins[j].Name
		}
		return report.Plugins[i].Source < report.Plugins[j].Source
	})
	report.Healthy = report.Errors == 0
}

func appendTree(report *Report, source, root string) {
	readMCP(report, source, filepath.Join(root, ".gemini", "config", "mcp_config.json"))
	readSkills(report, source, filepath.Join(root, ".gemini", "config", "skills"))
	readPlugins(report, source, filepath.Join(root, ".gemini", "config", "plugins"))
}

func appendBinding(report *Report, name string, countDangling bool) {
	mcpMode, skillsMode := bindingModes(name)
	check := Check{
		Name:    "profile " + name,
		Status:  StatusOK,
		Message: fmt.Sprintf("Profile %s: MCP %s, skills %s", name, mcpMode, skillsMode),
		Detail:  mcpMode + "," + skillsMode,
	}
	root := config.GetProfileDir(name)
	broken := (mcpMode == "shared" && dangling(filepath.Join(root, ".gemini", "config", "mcp_config.json"))) ||
		(skillsMode == "shared" && (dangling(filepath.Join(root, ".gemini", "config", "skills")) || dangling(filepath.Join(root, ".gemini", "config", "plugins"))))
	if countDangling && broken {
		check.Status = StatusError
		check.Message += " (dangling symlink)"
		report.Errors++
	}
	report.Checks = append(report.Checks, check)
}

func bindingModes(name string) (string, string) {
	mcpMode := "none"
	skillsMode := "none"
	if st, err := profile.GetMcpStatus(name); err == nil {
		mcpMode = st.Mode
	}
	if st, err := profile.GetSkillsStatus(name); err == nil {
		skillsMode = st.Mode
	}
	return mcpMode, skillsMode
}

func readMCP(report *Report, source, path string) {
	kind, err := pathKind(path)
	if err != nil {
		add(report, Check{
			Name:    source + " MCP config",
			Status:  StatusWarning,
			Message: fmt.Sprintf("%s MCP config: not configured", source),
		})
		return
	}
	if kind == kindDangling {
		add(report, Check{
			Name:    source + " MCP config",
			Status:  StatusError,
			Message: fmt.Sprintf("%s MCP config: dangling symlink", source),
			Detail:  path,
		})
		return
	}
	if kind != kindFile {
		add(report, Check{
			Name:    source + " MCP config",
			Status:  StatusError,
			Message: fmt.Sprintf("%s MCP config: expected a file", source),
			Detail:  path,
		})
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		add(report, Check{
			Name:    source + " MCP config",
			Status:  StatusError,
			Message: fmt.Sprintf("%s MCP config: unreadable", source),
			Detail:  path,
		})
		return
	}
	var file mcpFile
	if err := json.Unmarshal(data, &file); err != nil || file.MCPServers == nil {
		if err != nil {
			add(report, Check{
				Name:    source + " MCP config",
				Status:  StatusError,
				Message: fmt.Sprintf("%s MCP config: invalid JSON", source),
				Detail:  path,
			})
			return
		}
		add(report, Check{
			Name:    source + " MCP config",
			Status:  StatusWarning,
			Message: fmt.Sprintf("%s MCP config: no mcpServers", source),
			Detail:  path,
		})
		return
	}

	names := make([]string, 0, len(file.MCPServers))
	for name := range file.MCPServers {
		names = append(names, name)
	}
	sort.Strings(names)
	add(report, Check{
		Name:    source + " MCP config",
		Status:  StatusOK,
		Message: fmt.Sprintf("%s MCP config: %d %s", source, len(names), plural(len(names), "server", "servers")),
		Detail:  path,
	})
	for _, name := range names {
		srv := classifyServer(source, name, file.MCPServers[name])
		report.Servers = append(report.Servers, srv)
		add(report, Check{
			Name:    source + " MCP " + name,
			Status:  srv.Status,
			Message: fmt.Sprintf("%s MCP server %s: %s", source, name, srv.Detail),
			Detail:  srv.Detail,
		})
	}
}

func classifyServer(source, name string, raw mcpServer) Server {
	srv := Server{Name: name, Source: source, Status: StatusOK}
	endpoint := strings.TrimSpace(raw.HTTPURL)
	if endpoint == "" {
		endpoint = strings.TrimSpace(raw.URL)
	}
	if endpoint != "" {
		srv.Transport = "http"
		safe, detail, status := classifyURL(endpoint)
		srv.URL = safe
		srv.Status = status
		srv.Detail = detail
		return srv
	}
	srv.Transport = "stdio"
	command := strings.TrimSpace(raw.Command)
	srv.Command = command
	if command == "" {
		srv.Status = StatusError
		srv.Detail = "missing command and url"
		return srv
	}
	if strings.ContainsAny(command, `/\`) || filepath.IsAbs(command) {
		if _, err := os.Stat(command); err != nil {
			srv.Status = StatusError
			srv.Detail = fmt.Sprintf("command %q not found", command)
			return srv
		}
		srv.Detail = fmt.Sprintf("stdio command %q", command)
		return srv
	}
	if _, err := exec.LookPath(command); err != nil {
		srv.Status = StatusWarning
		srv.Detail = fmt.Sprintf("command %q not in PATH", command)
		return srv
	}
	srv.Detail = fmt.Sprintf("stdio command %q", command)
	return srv
}

func classifyURL(raw string) (string, string, string) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", "invalid url", StatusError
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), "http " + parsed.String(), StatusOK
}

func readSkills(report *Report, source, path string) {
	entries, check := readDirEntries(source, "skills", path)
	if check.Status != StatusOK {
		add(report, check)
		return
	}
	if len(entries) == 0 {
		add(report, Check{
			Name:    source + " skills",
			Status:  StatusWarning,
			Message: fmt.Sprintf("%s skills: directory empty", source),
			Detail:  path,
		})
		return
	}
	add(report, Check{
		Name:    source + " skills",
		Status:  StatusOK,
		Message: fmt.Sprintf("%s skills: %d %s", source, len(entries), plural(len(entries), "entry", "entries")),
		Detail:  path,
	})
	for _, entry := range entries {
		skillPath := filepath.Join(path, entry.Name())
		if dangling(skillPath) {
			skill := Skill{Name: entry.Name(), Source: source, Path: skillPath, Status: StatusError, Detail: "dangling symlink"}
			report.Skills = append(report.Skills, skill)
			add(report, Check{
				Name:    source + " skill " + entry.Name(),
				Status:  StatusError,
				Message: fmt.Sprintf("%s skill %s: dangling symlink", source, entry.Name()),
				Detail:  skillPath,
			})
			continue
		}
		info, err := os.Stat(skillPath)
		skillFile := filepath.Join(skillPath, "SKILL.md")
		if err != nil || !info.IsDir() {
			skill := Skill{Name: entry.Name(), Source: source, Path: skillPath, Status: StatusWarning, Detail: "not a skill directory"}
			report.Skills = append(report.Skills, skill)
			add(report, Check{
				Name:    source + " skill " + entry.Name(),
				Status:  StatusWarning,
				Message: fmt.Sprintf("%s skill %s: not a skill directory", source, entry.Name()),
				Detail:  skillPath,
			})
			continue
		}
		if _, err := os.Stat(skillFile); err != nil {
			skill := Skill{Name: entry.Name(), Source: source, Path: skillPath, Status: StatusWarning, Detail: "missing SKILL.md"}
			report.Skills = append(report.Skills, skill)
			add(report, Check{
				Name:    source + " skill " + entry.Name(),
				Status:  StatusWarning,
				Message: fmt.Sprintf("%s skill %s: missing SKILL.md", source, entry.Name()),
				Detail:  skillPath,
			})
			continue
		}
		metaName, desc := readSkillMeta(skillFile)
		if metaName == "" {
			metaName = entry.Name()
		}
		skill := Skill{Name: metaName, Source: source, Description: desc, Path: skillPath, Status: StatusOK, Detail: "SKILL.md present"}
		report.Skills = append(report.Skills, skill)
		add(report, Check{
			Name:    source + " skill " + metaName,
			Status:  StatusOK,
			Message: fmt.Sprintf("%s skill %s: SKILL.md present", source, metaName),
			Detail:  skillPath,
		})
	}
}

func readPlugins(report *Report, source, path string) {
	entries, check := readDirEntries(source, "plugins", path)
	if check.Status != StatusOK {
		add(report, check)
		return
	}
	if len(entries) == 0 {
		add(report, Check{
			Name:    source + " plugins",
			Status:  StatusWarning,
			Message: fmt.Sprintf("%s plugins: directory empty", source),
			Detail:  path,
		})
		return
	}
	add(report, Check{
		Name:    source + " plugins",
		Status:  StatusOK,
		Message: fmt.Sprintf("%s plugins: %d %s", source, len(entries), plural(len(entries), "entry", "entries")),
		Detail:  path,
	})
	for _, entry := range entries {
		pluginPath := filepath.Join(path, entry.Name())
		if dangling(pluginPath) {
			plugin := Plugin{Name: entry.Name(), Source: source, Path: pluginPath, Status: StatusError, Detail: "dangling symlink"}
			report.Plugins = append(report.Plugins, plugin)
			add(report, Check{
				Name:    source + " plugin " + entry.Name(),
				Status:  StatusError,
				Message: fmt.Sprintf("%s plugin %s: dangling symlink", source, entry.Name()),
				Detail:  pluginPath,
			})
			continue
		}
		metaPath := filepath.Join(pluginPath, "plugin.json")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			plugin := Plugin{Name: entry.Name(), Source: source, Path: pluginPath, Status: StatusWarning, Detail: "missing plugin.json"}
			report.Plugins = append(report.Plugins, plugin)
			add(report, Check{
				Name:    source + " plugin " + entry.Name(),
				Status:  StatusWarning,
				Message: fmt.Sprintf("%s plugin %s: missing plugin.json", source, entry.Name()),
				Detail:  pluginPath,
			})
			continue
		}
		var meta pluginMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			plugin := Plugin{Name: entry.Name(), Source: source, Path: pluginPath, Status: StatusError, Detail: "invalid plugin.json"}
			report.Plugins = append(report.Plugins, plugin)
			add(report, Check{
				Name:    source + " plugin " + entry.Name(),
				Status:  StatusError,
				Message: fmt.Sprintf("%s plugin %s: invalid plugin.json", source, entry.Name()),
				Detail:  pluginPath,
			})
			continue
		}
		name := meta.Name
		if name == "" {
			name = entry.Name()
		}
		plugin := Plugin{Name: name, Source: source, Path: pluginPath, Status: StatusOK, Detail: "plugin.json present"}
		report.Plugins = append(report.Plugins, plugin)
		add(report, Check{
			Name:    source + " plugin " + name,
			Status:  StatusOK,
			Message: fmt.Sprintf("%s plugin %s: plugin.json present", source, name),
			Detail:  pluginPath,
		})
	}
}

const (
	kindMissing = iota
	kindDangling
	kindFile
	kindDir
	kindOther
)

func pathKind(path string) (int, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return kindMissing, err
		}
		return kindOther, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		if _, err := os.Stat(path); err != nil {
			return kindDangling, nil
		}
		info, err = os.Stat(path)
		if err != nil {
			return kindDangling, nil
		}
	}
	if info.Mode().IsRegular() {
		return kindFile, nil
	}
	if info.IsDir() {
		return kindDir, nil
	}
	return kindOther, nil
}

func dangling(path string) bool {
	kind, _ := pathKind(path)
	return kind == kindDangling
}

func readDirEntries(source, label, path string) ([]os.DirEntry, Check) {
	kind, err := pathKind(path)
	if err != nil || kind == kindMissing {
		return nil, Check{
			Name:    source + " " + label,
			Status:  StatusWarning,
			Message: fmt.Sprintf("%s %s: not configured", source, label),
		}
	}
	if kind == kindDangling {
		return nil, Check{
			Name:    source + " " + label,
			Status:  StatusError,
			Message: fmt.Sprintf("%s %s: dangling symlink", source, label),
			Detail:  path,
		}
	}
	if kind != kindDir {
		return nil, Check{
			Name:    source + " " + label,
			Status:  StatusError,
			Message: fmt.Sprintf("%s %s: expected a directory", source, label),
			Detail:  path,
		}
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, Check{
			Name:    source + " " + label,
			Status:  StatusError,
			Message: fmt.Sprintf("%s %s: unreadable", source, label),
			Detail:  path,
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return entries, Check{Status: StatusOK}
}

func readSkillMeta(path string) (string, string) {
	f, err := os.Open(path)
	if err != nil {
		return "", ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 4096), 64*1024)
	if !sc.Scan() || strings.TrimSpace(sc.Text()) != "---" {
		return "", ""
	}
	var name, desc string
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "---" {
			break
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		switch strings.TrimSpace(key) {
		case "name":
			name = val
		case "description":
			desc = val
		}
	}
	return name, desc
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func add(report *Report, check Check) {
	switch check.Status {
	case StatusError:
		report.Errors++
	case StatusWarning:
		report.Warnings++
	}
	report.Checks = append(report.Checks, check)
}

func hostHome() string {
	if home := os.Getenv("REAL_HOME"); home != "" {
		return home
	}
	home, _ := os.UserHomeDir()
	return home
}
