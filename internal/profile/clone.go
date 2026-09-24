package profile

import (
	"fmt"
	"os"

	"github.com/ye-dev/multigravity-cli/internal/config"
	"github.com/ye-dev/multigravity-cli/internal/shortcut"
)

// CloneProfile copies an existing profile to a new profile destination
func CloneProfile(src, dest string) error {
	if err := config.ValidateProfileName(src); err != nil {
		return fmt.Errorf("invalid source profile name: %w", err)
	}
	if err := config.ValidateProfileName(dest); err != nil {
		return fmt.Errorf("invalid destination profile name: %w", err)
	}

	srcDir := config.GetProfileDir(src)
	destDir := config.GetProfileDir(dest)

	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return fmt.Errorf("source profile %q does not exist", src)
	}

	if _, err := os.Stat(destDir); err == nil {
		return fmt.Errorf("destination profile %q already exists", dest)
	}

	if err := CopyDir(srcDir, destDir); err != nil {
		return fmt.Errorf("failed to clone profile from %q to %q: %w", src, dest, err)
	}

	_ = shortcut.CreateShortcut(dest)

	return nil
}
