package doctor

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ye-dev/multigravity-cli/internal/app"
	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/profile"
	"github.com/ye-dev/multigravity-cli/internal/shortcut"
)

// CheckStatus represents the severity/outcome of a diagnostic check
type CheckStatus string

const (
	StatusOK      CheckStatus = "ok"
	StatusWarning CheckStatus = "warning"
	StatusError   CheckStatus = "error"
)

// DiagnosticCheck represents a single diagnostic assertion
type DiagnosticCheck struct {
	Name    string      `json:"name"`
	Status  CheckStatus `json:"status"`
	Message string      `json:"message"`
	Detail  string      `json:"detail,omitempty"`
}

// DiagnosticReport contains the complete structured report
type DiagnosticReport struct {
	Platform string            `json:"platform"`
	Checks   []DiagnosticCheck `json:"checks"`
	Errors   int               `json:"errors"`
	Warnings int               `json:"warnings"`
	Healthy  bool              `json:"healthy"`
}

// DiagnosticResult contains the outcome of system checks
type DiagnosticResult struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
}

// Diagnose executes all environment diagnostic checks and returns a structured report
func Diagnose() (*DiagnosticReport, error) {
	report := &DiagnosticReport{
		Platform: runtime.GOOS,
		Checks:   make([]DiagnosticCheck, 0),
	}

	// 1. Platform Check
	platform := runtime.GOOS
	switch platform {
	case "linux", "darwin", "windows":
		report.Checks = append(report.Checks, DiagnosticCheck{
			Name:    "Platform",
			Status:  StatusOK,
			Message: fmt.Sprintf("Platform: %s", platform),
			Detail:  platform,
		})
	default:
		report.Checks = append(report.Checks, DiagnosticCheck{
			Name:    "Platform",
			Status:  StatusError,
			Message: fmt.Sprintf("Platform: Unknown (%s)", platform),
			Detail:  platform,
		})
		report.Errors++
	}

	// 2. Antigravity / Agy Installation
	appPath, err := app.FindApp()
	if err == nil && appPath != "" {
		report.Checks = append(report.Checks, DiagnosticCheck{
			Name:    "Antigravity / Agy",
			Status:  StatusOK,
			Message: fmt.Sprintf("Antigravity / Agy: Found at %s", appPath),
			Detail:  appPath,
		})
	} else {
		report.Checks = append(report.Checks, DiagnosticCheck{
			Name:    "Antigravity / Agy",
			Status:  StatusError,
			Message: "Antigravity / Agy: Not found. Ensure it is installed or set MULTIGRAVITY_APP or AGY_APP.",
		})
		report.Errors++
	}

	// 3. Path Check
	installed, err := exec.LookPath("multigravity")
	if err == nil && installed != "" {
		report.Checks = append(report.Checks, DiagnosticCheck{
			Name:    "Global Binary",
			Status:  StatusOK,
			Message: fmt.Sprintf("Global Binary: %s", installed),
			Detail:  installed,
		})
	} else {
		report.Checks = append(report.Checks, DiagnosticCheck{
			Name:    "Global Binary",
			Status:  StatusWarning,
			Message: "Global Binary: Not found in PATH. Run install script or update PATH.",
		})
		report.Warnings++
	}

	// 4. Git Runtime Check
	gitPath, err := exec.LookPath("git")
	if err == nil && gitPath != "" {
		gitVer := "unknown version"
		if verOut, verErr := exec.Command(gitPath, "--version").Output(); verErr == nil {
			gitVer = strings.TrimSpace(string(verOut))
		}
		report.Checks = append(report.Checks, DiagnosticCheck{
			Name:    "Git",
			Status:  StatusOK,
			Message: fmt.Sprintf("Git: Found at %s (%s)", gitPath, gitVer),
			Detail:  gitPath,
		})
	} else {
		report.Checks = append(report.Checks, DiagnosticCheck{
			Name:    "Git",
			Status:  StatusWarning,
			Message: "Git: Not found in PATH. Worktrees and task diffs will not function properly.",
		})
		report.Warnings++
	}

	// 5. Icon Check (macOS)
	if platform == "darwin" {
		if shortcut.HasEmbeddedIcon() {
			report.Checks = append(report.Checks, DiagnosticCheck{
				Name:    "Application Icon",
				Status:  StatusOK,
				Message: "Application Icon: Embedded (built-in)",
				Detail:  "embedded://assets/icon.icns",
			})
		} else {
			home, _ := os.UserHomeDir()
			candidates := []string{
				"icon.icns",
				filepath.Join(home, ".local", "share", "multigravity", "icon.icns"),
			}
			if exe, err := os.Executable(); err == nil && exe != "" {
				candidates = append(candidates, filepath.Join(filepath.Dir(exe), "icon.icns"))
			}

			iconFound := false
			var checkedPath string
			for _, p := range candidates {
				checkedPath = p
				if _, err := os.Stat(p); err == nil {
					iconFound = true
					break
				}
			}

			if iconFound {
				report.Checks = append(report.Checks, DiagnosticCheck{
					Name:    "Application Icon",
					Status:  StatusOK,
					Message: "Application Icon: Found",
					Detail:  checkedPath,
				})
			} else {
				report.Checks = append(report.Checks, DiagnosticCheck{
					Name:    "Application Icon",
					Status:  StatusWarning,
					Message: fmt.Sprintf("Application Icon: Missing (%s). Shortcuts will have default icons.", checkedPath),
					Detail:  checkedPath,
				})
				report.Warnings++
			}
		}
	}

	// 5. Base Directory
	base := config.GetMultigravityHome()
	info, err := os.Stat(base)
	if err == nil && info.IsDir() {
		testFile := filepath.Join(base, ".write-test")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err == nil {
			_ = os.Remove(testFile)
			report.Checks = append(report.Checks, DiagnosticCheck{
				Name:    "Profile Storage",
				Status:  StatusOK,
				Message: fmt.Sprintf("Profile storage: %s (writable)", base),
				Detail:  base,
			})
		} else {
			report.Checks = append(report.Checks, DiagnosticCheck{
				Name:    "Profile Storage",
				Status:  StatusError,
				Message: fmt.Sprintf("Profile storage: %s (NOT writable)", base),
				Detail:  base,
			})
			report.Errors++
		}
	} else {
		report.Checks = append(report.Checks, DiagnosticCheck{
			Name:    "Profile Storage",
			Status:  StatusWarning,
			Message: fmt.Sprintf("Profile storage: %s (Not yet created)", base),
			Detail:  base,
		})
		report.Warnings++
	}

	// 7. Profile Toolchain Isolation Check (Toolchain Bleed)
	if entries, err := os.ReadDir(base); err == nil {
		var bleedReports []string
		for _, entry := range entries {
			if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			pName := entry.Name()
			pDir := filepath.Join(base, pName)

			var detected []string
			if fi, err := os.Stat(filepath.Join(pDir, ".cargo")); err == nil && fi.IsDir() {
				detected = append(detected, ".cargo")
			}
			if fi, err := os.Stat(filepath.Join(pDir, ".rustup")); err == nil && fi.IsDir() {
				detected = append(detected, ".rustup")
			}

			if len(detected) > 0 {
				bleedReports = append(bleedReports, fmt.Sprintf("%s (%s)", pName, strings.Join(detected, ", ")))
			}
		}

		if len(bleedReports) > 0 {
			report.Checks = append(report.Checks, DiagnosticCheck{
				Name:    "Toolchain Isolation",
				Status:  StatusWarning,
				Message: fmt.Sprintf("Toolchain Bleed: Orphaned toolchains detected in profile(s): %s. Host toolchains should reside in REAL_HOME to avoid duplication.", strings.Join(bleedReports, "; ")),
				Detail:  strings.Join(bleedReports, "; "),
			})
			report.Warnings++
		} else {
			report.Checks = append(report.Checks, DiagnosticCheck{
				Name:    "Toolchain Isolation",
				Status:  StatusOK,
				Message: "Toolchain Isolation: Clean (no orphaned .cargo or .rustup inside profiles)",
			})
		}
	} else {
		report.Checks = append(report.Checks, DiagnosticCheck{
			Name:    "Toolchain Isolation",
			Status:  StatusOK,
			Message: "Toolchain Isolation: Clean (no profiles found)",
		})
	}

	// 8. Development Toolchains Check
	realHome := os.Getenv("REAL_HOME")
	if realHome == "" {
		realHome, _ = os.UserHomeDir()
	}
	enrichedPath := profile.BuildEnrichedPATH(realHome)
	toolCandidates := []string{"node", "pnpm", "go", "cargo", "rustc"}
	var foundTools []string
	var missingTools []string

	for _, tool := range toolCandidates {
		if path := lookInPath(tool, enrichedPath); path != "" {
			foundTools = append(foundTools, tool)
		} else {
			missingTools = append(missingTools, tool)
		}
	}
	pyFound := ""
	if p := lookInPath("python3", enrichedPath); p != "" {
		pyFound = "python3"
	} else if p := lookInPath("python", enrichedPath); p != "" {
		pyFound = "python"
	}
	if pyFound != "" {
		foundTools = append(foundTools, pyFound)
	} else {
		missingTools = append(missingTools, "python")
	}

	if len(foundTools) > 0 {
		report.Checks = append(report.Checks, DiagnosticCheck{
			Name:    "Development Toolchains",
			Status:  StatusOK,
			Message: fmt.Sprintf("Development Toolchains: Detected %s", strings.Join(foundTools, ", ")),
			Detail:  fmt.Sprintf("found: %s; missing: %s", strings.Join(foundTools, ", "), strings.Join(missingTools, ", ")),
		})
	} else {
		report.Checks = append(report.Checks, DiagnosticCheck{
			Name:    "Development Toolchains",
			Status:  StatusWarning,
			Message: "Development Toolchains: No compilers or runtimes detected (cargo, rustc, node, pnpm, go, python). Subagents may fail build tasks.",
			Detail:  "missing: cargo, rustc, node, pnpm, go, python",
		})
		report.Warnings++
	}

	report.Healthy = report.Errors == 0
	return report, nil
}

func lookInPath(file, pathList string) string {
	exts := []string{""}
	if runtime.GOOS == "windows" {
		exts = []string{".exe", ".cmd", ".bat", ""}
	}
	for _, dir := range filepath.SplitList(pathList) {
		if dir == "" {
			continue
		}
		for _, ext := range exts {
			target := filepath.Join(dir, file+ext)
			if fi, err := os.Stat(target); err == nil && !fi.IsDir() {
				return target
			}
		}
	}
	if lp, err := exec.LookPath(file); err == nil {
		return lp
	}
	return ""
}

// RunDoctor executes all environment diagnostic checks and writes output to w
func RunDoctor(w io.Writer) (*DiagnosticResult, error) {
	report, err := Diagnose()
	if err != nil {
		return nil, err
	}

	fmt.Fprintln(w, "Checking multigravity environment...")

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
	if report.Errors == 0 {
		if report.Warnings == 0 {
			fmt.Fprintln(w, "✓ Your environment looks perfect!")
		} else {
			fmt.Fprintf(w, "Found %d warning(s). Multigravity should still work, but some features might be degraded.\n", report.Warnings)
		}
	} else {
		fmt.Fprintf(w, "✗ Found %d error(s) and %d warning(s). Please fix the errors above.\n", report.Errors, report.Warnings)
	}

	return &DiagnosticResult{
		Errors:   report.Errors,
		Warnings: report.Warnings,
	}, nil
}

