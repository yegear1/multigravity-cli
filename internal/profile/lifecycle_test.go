package profile

import (
	"strings"
	"testing"
	"time"
)

func TestStopProfileNotRunning(t *testing.T) {
	_ = setupTestHome(t)

	err := CreateProfile(CreateOptions{Name: "idle-profile"})
	if err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	// Mock no PIDs
	oldPIDsFn := getProfilePIDsFn
	defer func() { getProfilePIDsFn = oldPIDsFn }()
	getProfilePIDsFn = func(name string) ([]int, error) {
		return nil, nil
	}

	err = StopProfile("idle-profile", false)
	if err != nil {
		t.Fatalf("expected StopProfile on idle profile to succeed, got: %v", err)
	}

	// Non-existent profile should error
	err = StopProfile("does-not-exist", false)
	if err == nil {
		t.Fatalf("expected StopProfile on non-existent profile to error, got nil")
	}
}

func TestStopProfileForce(t *testing.T) {
	_ = setupTestHome(t)

	err := CreateProfile(CreateOptions{Name: "running-profile"})
	if err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	killed := false
	oldPIDsFn := getProfilePIDsFn
	oldKillFn := killProcessFn
	defer func() {
		getProfilePIDsFn = oldPIDsFn
		killProcessFn = oldKillFn
	}()

	getProfilePIDsFn = func(name string) ([]int, error) {
		return []int{1234}, nil
	}
	killProcessFn = func(pid int) error {
		if pid == 1234 {
			killed = true
		}
		return nil
	}

	err = StopProfile("running-profile", true)
	if err != nil {
		t.Fatalf("expected StopProfile with force to succeed, got: %v", err)
	}
	if !killed {
		t.Fatalf("expected process 1234 to be killed")
	}
}

func TestStopProfileGracefulSuccess(t *testing.T) {
	_ = setupTestHome(t)

	err := CreateProfile(CreateOptions{Name: "graceful-profile"})
	if err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	terminated := false
	calls := 0
	oldPIDsFn := getProfilePIDsFn
	oldTermFn := terminateProcessFn
	oldWaitIter := stopWaitIterations
	oldWaitInterval := stopWaitInterval
	defer func() {
		getProfilePIDsFn = oldPIDsFn
		terminateProcessFn = oldTermFn
		stopWaitIterations = oldWaitIter
		stopWaitInterval = oldWaitInterval
	}()

	stopWaitInterval = 5 * time.Millisecond
	stopWaitIterations = 5

	getProfilePIDsFn = func(name string) ([]int, error) {
		calls++
		if calls == 1 {
			// First check before stop
			return []int{5678}, nil
		}
		// Terminated on first poll iteration
		return nil, nil
	}
	terminateProcessFn = func(pid int) error {
		if pid == 5678 {
			terminated = true
		}
		return nil
	}

	err = StopProfile("graceful-profile", false)
	if err != nil {
		t.Fatalf("expected StopProfile to succeed, got: %v", err)
	}
	if !terminated {
		t.Fatalf("expected process 5678 to be terminated gracefully")
	}
}

func TestStopProfileGracefulTimeoutFallback(t *testing.T) {
	_ = setupTestHome(t)

	err := CreateProfile(CreateOptions{Name: "stubborn-profile"})
	if err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	terminated := false
	killed := false
	oldPIDsFn := getProfilePIDsFn
	oldTermFn := terminateProcessFn
	oldKillFn := killProcessFn
	oldWaitIter := stopWaitIterations
	oldWaitInterval := stopWaitInterval
	defer func() {
		getProfilePIDsFn = oldPIDsFn
		terminateProcessFn = oldTermFn
		killProcessFn = oldKillFn
		stopWaitIterations = oldWaitIter
		stopWaitInterval = oldWaitInterval
	}()

	stopWaitInterval = 2 * time.Millisecond
	stopWaitIterations = 2

	getProfilePIDsFn = func(name string) ([]int, error) {
		return []int{9999}, nil
	}
	terminateProcessFn = func(pid int) error {
		if pid == 9999 {
			terminated = true
		}
		return nil
	}
	killProcessFn = func(pid int) error {
		if pid == 9999 {
			killed = true
		}
		return nil
	}

	err = StopProfile("stubborn-profile", false)
	if err != nil {
		t.Fatalf("expected StopProfile to succeed with fallback kill, got: %v", err)
	}
	if !terminated {
		t.Fatalf("expected process 9999 to have received terminate signal")
	}
	if !killed {
		t.Fatalf("expected process 9999 to have been killed after timeout")
	}
}

func TestRestartProfile(t *testing.T) {
	_ = setupTestHome(t)

	err := CreateProfile(CreateOptions{Name: "restart-profile"})
	if err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	launched := false
	var launchedArgs []string
	oldLaunchFn := launchProfileFn
	oldPIDsFn := getProfilePIDsFn
	defer func() {
		launchProfileFn = oldLaunchFn
		getProfilePIDsFn = oldPIDsFn
	}()

	getProfilePIDsFn = func(name string) ([]int, error) {
		return nil, nil // idle
	}

	launchProfileFn = func(name string, forwardArgs []string) error {
		if name == "restart-profile" {
			launched = true
			launchedArgs = forwardArgs
		}
		return nil
	}

	err = RestartProfile("restart-profile", []string{"--new-window", "/tmp"})
	if err != nil {
		t.Fatalf("expected RestartProfile to succeed, got: %v", err)
	}
	if !launched {
		t.Fatalf("expected profile to be relaunched")
	}
	if len(launchedArgs) != 2 || launchedArgs[0] != "--new-window" {
		t.Fatalf("expected forwarded args to match, got: %v", launchedArgs)
	}

	// Invalid name
	err = RestartProfile("invalid_name", nil)
	if err == nil || !strings.Contains(err.Error(), "invalid profile name") {
		t.Fatalf("expected invalid profile name error, got: %v", err)
	}
}
