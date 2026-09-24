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

// DiagnosticResult contains the outcome of system checks
type DiagnosticResult struct {
	Errors   int
	Warnings int
}

// RunDoctor executes all environment diagnostic checks and writes output to w
func RunDoctor(w io.Writer) (*DiagnosticResult, error) {
	res := &DiagnosticResult{}

	fmt.Fprintln(w, "Checking multigravity environment...")

	// 1. Platform Check
	platform := runtime.GOOS
	switch platform {
	case "linux", "darwin", "windows":
		fmt.Fprintf(w, "  [✓] Platform: %s\n", platform)
	default:
		fmt.Fprintf(w, "  [✗] Platform: Unknown (%s)\n", platform)
		res.Errors++
	}

	// 2. Antigravity / Agy Installation
	appPath, err := app.FindApp()
	if err == nil && appPath != "" {
		fmt.Fprintf(w, "  [✓] Antigravity / Agy: Found at %s\n", appPath)
	} else {
		fmt.Fprintln(w, "  [✗] Antigravity / Agy: Not found. Ensure it is installed or set MULTIGRAVITY_APP or AGY_APP.")
		res.Errors++
	}

	// 3. Path Check
	installed, err := exec.LookPath("multigravity")
	if err == nil && installed != "" {
		fmt.Fprintf(w, "  [✓] Global Binary: %s\n", installed)
	} else {
		fmt.Fprintln(w, "  [!] Global Binary: Not found in PATH. Run install script or update PATH.")
		res.Warnings++
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
			fmt.Fprintln(w, "  [✓] Application Icon: Found")
		} else {
			fmt.Fprintf(w, "  [!] Application Icon: Missing (%s). Shortcuts will have default icons.\n", checkedPath)
			res.Warnings++
		}
	}

	// 5. Base Directory
	base := config.GetMultigravityHome()
	info, err := os.Stat(base)
	if err == nil && info.IsDir() {
		testFile := filepath.Join(base, ".write-test")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err == nil {
			_ = os.Remove(testFile)
			fmt.Fprintf(w, "  [✓] Profile storage: %s (writable)\n", base)
		} else {
			fmt.Fprintf(w, "  [✗] Profile storage: %s (NOT writable)\n", base)
			res.Errors++
		}
	} else {
		fmt.Fprintf(w, "  [!] Profile storage: %s (Not yet created)\n", base)
	}

	fmt.Fprintln(w, "")
	if res.Errors == 0 {
		if res.Warnings == 0 {
			fmt.Fprintln(w, "✓ Your environment looks perfect!")
		} else {
			fmt.Fprintf(w, "Found %d warning(s). Multigravity should still work, but some features might be degraded.\n", res.Warnings)
		}
	} else {
		fmt.Fprintf(w, "✗ Found %d error(s) and %d warning(s). Please fix the errors above.\n", res.Errors, res.Warnings)
	}

	return res, nil
}
