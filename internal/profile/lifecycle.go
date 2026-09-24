package profile

import (
	"fmt"
	"os"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/config"
)

var (
	getProfilePIDsFn   = getProfilePIDsOS
	terminateProcessFn = terminateProcessOS
	killProcessFn      = killProcessOS
	launchProfileFn    = defaultLaunchProfile

	stopWaitInterval   = 200 * time.Millisecond
	stopWaitIterations = 15
)

// GetProfilePIDs checks for running processes associated with the profile
func GetProfilePIDs(name string) ([]int, error) {
	return getProfilePIDsFn(name)
}

// IsProfileRunning returns true if the profile has active processes
func IsProfileRunning(name string) bool {
	pids, err := GetProfilePIDs(name)
	return err == nil && len(pids) > 0
}

// StopProfile stops a running profile gracefully or forcefully
func StopProfile(name string, force bool) error {
	if err := config.ValidateProfileName(name); err != nil {
		return err
	}

	profileDir := config.GetProfileDir(name)
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		return fmt.Errorf("profile %q does not exist", name)
	}

	pids, err := GetProfilePIDs(name)
	if err != nil {
		return fmt.Errorf("failed to query processes for profile %q: %w", name, err)
	}

	if len(pids) == 0 {
		fmt.Printf("Profile %q is not running.\n", name)
		return nil
	}

	fmt.Printf("Stopping profile %q...\n", name)

	if force {
		for _, pid := range pids {
			_ = killProcessFn(pid)
		}
		fmt.Printf("Profile %q forcefully stopped.\n", name)
		return nil
	}

	// Graceful SIGTERM / close
	for _, pid := range pids {
		_ = terminateProcessFn(pid)
	}

	waited := 0
	for waited < stopWaitIterations {
		time.Sleep(stopWaitInterval)
		waited++
		remaining, _ := GetProfilePIDs(name)
		if len(remaining) == 0 {
			fmt.Printf("Profile %q stopped gracefully.\n", name)
			return nil
		}
	}

	fmt.Printf("Terminating remaining processes for profile %q...\n", name)
	remaining, _ := GetProfilePIDs(name)
	for _, pid := range remaining {
		_ = killProcessFn(pid)
	}
	fmt.Printf("Profile %q stopped.\n", name)
	return nil
}

// RestartProfile gracefully stops a running profile and launches it again
func RestartProfile(name string, forwardArgs []string) error {
	if err := config.ValidateProfileName(name); err != nil {
		return err
	}

	profileDir := config.GetProfileDir(name)
	if _, err := os.Stat(profileDir); os.IsNotExist(err) {
		return fmt.Errorf("profile %q does not exist", name)
	}

	if err := StopProfile(name, false); err != nil {
		return err
	}

	time.Sleep(500 * time.Millisecond)
	return LaunchProfile(name, forwardArgs)
}

func defaultLaunchProfile(name string, forwardArgs []string) error {
	fmt.Printf("Launching Antigravity profile %q\n", name)
	return nil
}

// LaunchProfile launches the profile (extended in task 90.4)
func LaunchProfile(name string, forwardArgs []string) error {
	return launchProfileFn(name, forwardArgs)
}
