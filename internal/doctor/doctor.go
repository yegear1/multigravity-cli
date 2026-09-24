package doctor

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/ye-dev/multigravity-cli/internal/app"
	"github.com/ye-dev/multigravity-cli/internal/config"
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

	// 4. Icon Check (macOS)
	if platform == "darwin" {
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

	report.Healthy = report.Errors == 0
	return report, nil
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

