//go:build !windows

package shortcut

func createShortcutWindows(profile string) error {
	return nil
}

func removeShortcutWindows(profile string) error {
	return nil
}
