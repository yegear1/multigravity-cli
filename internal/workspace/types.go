package workspace

// Workspace represents an Antigravity project/workspace mapped to a profile
type Workspace struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Profile         string            `json:"profile"`
	Path            string            `json:"path"`
	URI             string            `json:"uri"`
	DefaultBranch   string            `json:"default_branch,omitempty"`
	IsWorkspaceOnly bool              `json:"is_workspace_only"`
	IsActive        bool              `json:"is_active"`
	Git             *GitRepoInfo      `json:"git,omitempty"`
	Settings        WorkspaceSettings `json:"settings"`
}

// WorkspaceSettings represents the agent/sandbox execution settings for the workspace
type WorkspaceSettings struct {
	FileAccessPolicy    string `json:"file_access_policy,omitempty"`
	SandboxMode         bool   `json:"sandbox_mode"`
	AutoExecutionPolicy string `json:"auto_execution_policy,omitempty"`
}

// GitRepoInfo represents Git repository metadata for a workspace path
type GitRepoInfo struct {
	IsGitRepo      bool   `json:"is_git_repo"`
	Branch         string `json:"branch,omitempty"`
	RemoteURL      string `json:"remote_url,omitempty"`
	CommitHash     string `json:"commit_hash,omitempty"`
	CommitMessage  string `json:"commit_message,omitempty"`
	IsClean        bool   `json:"is_clean"`
	ModifiedFiles  int    `json:"modified_files"`
	UntrackedFiles int    `json:"untracked_files"`
}

// ProfileWorkspacesSummary consolidates all workspaces belonging to a profile
type ProfileWorkspacesSummary struct {
	Profile         string      `json:"profile"`
	IsRunning       bool        `json:"is_running"`
	ActiveWorkspace *Workspace  `json:"active_workspace,omitempty"`
	Workspaces      []Workspace `json:"workspaces"`
	TotalWorkspaces int         `json:"total_workspaces"`
}

// Raw JSON schemas matching Antigravity's ~/.gemini/config/projects/*.json
type rawProject struct {
	ID               string              `json:"id"`
	Name             string              `json:"name"`
	ProjectResources rawProjectResources `json:"projectResources"`
	Settings         rawProjectSettings  `json:"settings"`
	IsWorkspaceOnly  bool                `json:"isWorkspaceOnly"`
}

type rawProjectResources struct {
	Resources []rawResource `json:"resources"`
}

type rawResource struct {
	GitFolder *rawGitFolder `json:"gitFolder,omitempty"`
	FolderURI string        `json:"folderUri,omitempty"`
}

type rawGitFolder struct {
	FolderURI     string `json:"folderUri"`
	DefaultBranch string `json:"defaultBranch"`
}

type rawProjectSettings struct {
	FileAccessPolicy    string `json:"fileAccessPolicy"`
	SandboxMode         bool   `json:"sandboxMode"`
	AutoExecutionPolicy string `json:"autoExecutionPolicy"`
}
