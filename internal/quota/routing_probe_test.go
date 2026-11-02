package quota

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
)

func TestProbeInactiveProfiles(t *testing.T) {
	// Hook FindActiveServers to return only "saasaba" as active
	cleanupServers := SetTestHooks(func(target string) ([]ActiveServer, error) {
		return []ActiveServer{
			{
				Profile:    "saasaba",
				PID:        1234,
				Port:       40001,
				QuotaKnown: true,
			},
		}, nil
	})
	defer cleanupServers()

	// Hook probeServerFn
	probedCalls := make(map[string]int)
	cleanupProbe := SetProbeTestHook(func(prof string) (*ActiveServer, error) {
		probedCalls[prof]++
		if prof == "joaoww" {
			return &ActiveServer{
				Profile:    "joaoww",
				PID:        5678,
				Port:       40002,
				QuotaKnown: true,
			}, nil
		}
		return nil, fmt.Errorf("probe failed for %s", prof)
	})
	defer cleanupProbe()

	// Probe specific list of profiles
	awakened, err := ProbeInactiveProfiles([]string{"saasaba", "joaoww", "luisfmb"})
	if err != nil {
		t.Fatalf("unexpected error probing inactive profiles: %v", err)
	}

	// "saasaba" was already active, so probeServerFn should not be called for it
	if probedCalls["saasaba"] != 0 {
		t.Errorf("expected saasaba to be skipped from probe, but got %d calls", probedCalls["saasaba"])
	}

	// "joaoww" and "luisfmb" should be probed
	if probedCalls["joaoww"] != 1 {
		t.Errorf("expected joaoww to be probed once, got %d", probedCalls["joaoww"])
	}
	if probedCalls["luisfmb"] != 1 {
		t.Errorf("expected luisfmb to be probed once, got %d", probedCalls["luisfmb"])
	}

	// Only "joaoww" succeeded
	expectedAwakened := []string{"joaoww"}
	if !reflect.DeepEqual(awakened, expectedAwakened) {
		t.Errorf("expected awakened %v, got %v", expectedAwakened, awakened)
	}
}

func TestProbeInactiveProfiles_AlreadyAllActive(t *testing.T) {
	cleanupServers := SetTestHooks(func(target string) ([]ActiveServer, error) {
		return []ActiveServer{
			{Profile: "prof1", QuotaKnown: true},
			{Profile: "prof2", QuotaKnown: true},
		}, nil
	})
	defer cleanupServers()

	cleanupProbe := SetProbeTestHook(func(prof string) (*ActiveServer, error) {
		t.Fatalf("probe hook should not be called when all profiles are active, called with %s", prof)
		return nil, nil
	})
	defer cleanupProbe()

	awakened, err := ProbeInactiveProfiles([]string{"prof1", "prof2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(awakened) != 0 {
		t.Errorf("expected 0 awakened profiles, got %v", awakened)
	}
}

func TestProbeInactiveProfiles_SortOrder(t *testing.T) {
	cleanupServers := SetTestHooks(func(target string) ([]ActiveServer, error) {
		return nil, nil
	})
	defer cleanupServers()

	cleanupProbe := SetProbeTestHook(func(prof string) (*ActiveServer, error) {
		return &ActiveServer{Profile: prof, QuotaKnown: true}, nil
	})
	defer cleanupProbe()

	awakened, err := ProbeInactiveProfiles([]string{"beta", "alpha"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sort.Strings(awakened)
	expected := []string{"alpha", "beta"}
	if !reflect.DeepEqual(awakened, expected) {
		t.Errorf("expected %v, got %v", expected, awakened)
	}
}
