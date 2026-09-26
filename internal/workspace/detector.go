package workspace

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

var (
	isProfileRunningFn = profile.IsProfileRunning
	getProfilePIDsFn   = profile.GetProfilePIDs
	listProfilesFn     = profile.ListProfiles
)

// SetTestHooks allows injecting mock functions for hermetic tests
func SetTestHooks(
	runningFn func(name string) bool,
	pidsFn func(name string) ([]int, error),
	listFn func() ([]string, error),
) func() {
	prevRunning := isProfileRunningFn
	prevPids := getProfilePIDsFn
	prevList := listProfilesFn

	if runningFn != nil {
		isProfileRunningFn = runningFn
	}
	if pidsFn != nil {
		getProfilePIDsFn = pidsFn
	}
	if listFn != nil {
		listProfilesFn = listFn
	}

	return func() {
		isProfileRunningFn = prevRunning
		getProfilePIDsFn = prevPids
		listProfilesFn = prevList
	}
}

// FileURIToPath converts a file:// URI into a valid local OS filesystem path
func FileURIToPath(uriStr string) string {
	if uriStr == "" {
		return ""
	}
	if !strings.HasPrefix(uriStr, "file://") {
		return filepath.Clean(uriStr)
	}

	parsed, err := url.Parse(uriStr)
	if err != nil {
		// Fallback simple trim
		cleaned := strings.TrimPrefix(uriStr, "file://")
		return filepath.Clean(cleaned)
	}

	p := parsed.Path
	if runtime.GOOS == "windows" {
		// e.g. /C:/Users/... -> C:/Users/...
		if len(p) > 2 && p[0] == '/' && p[2] == ':' {
			p = p[1:]
		}
	}

	decoded, err := url.PathUnescape(p)
	if err != nil {
		decoded = p
	}

	return filepath.Clean(filepath.FromSlash(decoded))
}

// GetLastSelectedProject reads app_storage.json to find the last active project in Antigravity
func GetLastSelectedProject(profileName string) string {
	profileDir := config.GetProfileDir(profileName)
	dataDir := config.GetUserDataDir(profileDir)
	storagePath := filepath.Join(dataDir, "app_storage.json")

	data, err := os.ReadFile(storagePath)
	if err != nil {
		return ""
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return ""
	}

	if val, ok := m["new-convo-last-selected-project"].(string); ok && val != "" {
		return val
	}
	if val, ok := m["lastCreatedProjectId"].(string); ok && val != "" {
		return val
	}

	return ""
}

// getRunningProcessPaths inspects running process arguments for a profile
func getRunningProcessPaths(profileName string) []string {
	pids, err := getProfilePIDsFn(profileName)
	if err != nil || len(pids) == 0 {
		return nil
	}

	var paths []string
	if runtime.GOOS == "windows" {
		return paths
	}

	for _, pid := range pids {
		cmd := exec.Command("ps", "-p", fmt.Sprintf("%d", pid), "-o", "args=")
		out, err := cmd.Output()
		if err != nil {
			continue
		}
		argsStr := string(out)
		parts := strings.Fields(argsStr)
		for _, part := range parts {
			if strings.HasPrefix(part, "-") {
				continue
			}
			if filepath.IsAbs(part) {
				if fi, err := os.Stat(part); err == nil && fi.IsDir() {
					paths = append(paths, filepath.Clean(part))
				}
			}
		}
	}
	return paths
}

// GetProfileWorkspaces scans and maps all workspaces and projects for a given profile
func GetProfileWorkspaces(profileName string) ([]Workspace, error) {
	if err := config.ValidateProfileName(profileName); err != nil {
		return nil, err
	}

	profileDir := config.GetProfileDir(profileName)
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("profile %q does not exist", profileName)
	}

	projectsDir := filepath.Join(profileDir, ".gemini", "config", "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Workspace{}, nil
		}
		return nil, err
	}

	isRunning := isProfileRunningFn(profileName)
	lastSelectedID := ""
	var procPaths []string
	if isRunning {
		lastSelectedID = GetLastSelectedProject(profileName)
		procPaths = getRunningProcessPaths(profileName)
	}

	var workspaces []Workspace
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(projectsDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var raw rawProject
		if err := json.Unmarshal(data, &raw); err != nil {
			continue
		}

		folderURI := ""
		defaultBranch := ""
		for _, res := range raw.ProjectResources.Resources {
			if res.GitFolder != nil {
				folderURI = res.GitFolder.FolderURI
				defaultBranch = res.GitFolder.DefaultBranch
				break
			}
			if res.FolderURI != "" {
				folderURI = res.FolderURI
				break
			}
		}

		localPath := FileURIToPath(folderURI)
		isActive := false
		if isRunning {
			if lastSelectedID != "" && raw.ID == lastSelectedID {
				isActive = true
			} else {
				for _, p := range procPaths {
					if localPath != "" && (localPath == p || strings.HasPrefix(p, localPath)) {
						isActive = true
						break
					}
				}
			}
		}

		var gitInfo *GitRepoInfo
		if localPath != "" {
			gitInfo = DetectGitRepoInfo(localPath)
		}

		ws := Workspace{
			ID:              raw.ID,
			Name:            raw.Name,
			Profile:         profileName,
			Path:            localPath,
			URI:             folderURI,
			DefaultBranch:   defaultBranch,
			IsWorkspaceOnly: raw.IsWorkspaceOnly,
			IsActive:        isActive,
			Git:             gitInfo,
			Settings: WorkspaceSettings{
				FileAccessPolicy:    raw.Settings.FileAccessPolicy,
				SandboxMode:         raw.Settings.SandboxMode,
				AutoExecutionPolicy: raw.Settings.AutoExecutionPolicy,
			},
		}

		workspaces = append(workspaces, ws)
	}

	// Sort: Active first, then alphabetically by Name
	sort.Slice(workspaces, func(i, j int) bool {
		if workspaces[i].IsActive != workspaces[j].IsActive {
			return workspaces[i].IsActive
		}
		return workspaces[i].Name < workspaces[j].Name
	})

	return workspaces, nil
}

// GetAllWorkspaces returns all workspaces across all registered profiles
func GetAllWorkspaces() ([]Workspace, error) {
	names, err := listProfilesFn()
	if err != nil {
		return nil, err
	}

	var all []Workspace
	for _, name := range names {
		wsList, err := GetProfileWorkspaces(name)
		if err != nil {
			continue
		}
		all = append(all, wsList...)
	}

	sort.Slice(all, func(i, j int) bool {
		if all[i].IsActive != all[j].IsActive {
			return all[i].IsActive
		}
		if all[i].Profile != all[j].Profile {
			return all[i].Profile < all[j].Profile
		}
		return all[i].Name < all[j].Name
	})

	return all, nil
}

// GetActiveWorkspaces returns only workspaces that are currently active in running profiles
func GetActiveWorkspaces() ([]Workspace, error) {
	all, err := GetAllWorkspaces()
	if err != nil {
		return nil, err
	}

	var active []Workspace
	for _, ws := range all {
		if ws.IsActive {
			active = append(active, ws)
		}
	}
	return active, nil
}

// GetWorkspaceByPath searches for profiles and projects that reference or match the given path
func GetWorkspaceByPath(targetPath string) ([]Workspace, error) {
	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		absTarget = filepath.Clean(targetPath)
	}

	all, err := GetAllWorkspaces()
	if err != nil {
		return nil, err
	}

	var matched []Workspace
	for _, ws := range all {
		if ws.Path == "" {
			continue
		}
		absWs, err := filepath.Abs(ws.Path)
		if err != nil {
			absWs = filepath.Clean(ws.Path)
		}

		// Exact match or sub-path match
		if absWs == absTarget || strings.HasPrefix(absTarget, absWs+string(filepath.Separator)) {
			matched = append(matched, ws)
		}
	}

	return matched, nil
}

// GetProfileSummary returns a consolidated summary of a profile's workspaces
func GetProfileSummary(profileName string) (*ProfileWorkspacesSummary, error) {
	wsList, err := GetProfileWorkspaces(profileName)
	if err != nil {
		return nil, err
	}

	isRunning := isProfileRunningFn(profileName)
	var active *Workspace
	for i := range wsList {
		if wsList[i].IsActive {
			active = &wsList[i]
			break
		}
	}

	return &ProfileWorkspacesSummary{
		Profile:         profileName,
		IsRunning:       isRunning,
		ActiveWorkspace: active,
		Workspaces:      wsList,
		TotalWorkspaces: len(wsList),
	}, nil
}
