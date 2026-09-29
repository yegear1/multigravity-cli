package quota

import (
	"github.com/ye-dev/multigravity-cli/internal/profile"
)

// BuildQuotaSummary builds a complete quota overview for JSON serialization.
// If targetProf is specified and has no live server, it returns that single profile with QuotaKnown: false.
// If targetProf is empty, it returns all registered profiles, with live servers having QuotaKnown: true
// and profiles without a live server having QuotaKnown: false (and no invented remaining fraction).
func BuildQuotaSummary(targetProf string, liveServers []ActiveServer) ([]ActiveServer, error) {
	for i := range liveServers {
		liveServers[i].QuotaKnown = true
	}

	if targetProf != "" {
		if len(liveServers) > 0 {
			return liveServers, nil
		}
		return []ActiveServer{
			{
				Profile:    targetProf,
				QuotaKnown: false,
			},
		}, nil
	}

	profiles, err := profile.ListProfiles()
	if err != nil {
		if len(liveServers) > 0 {
			return liveServers, nil
		}
		return []ActiveServer{}, nil
	}

	liveByProfile := make(map[string][]ActiveServer)
	for _, s := range liveServers {
		liveByProfile[s.Profile] = append(liveByProfile[s.Profile], s)
	}

	visited := make(map[string]bool)
	var result []ActiveServer
	for _, p := range profiles {
		visited[p] = true
		if srvs, ok := liveByProfile[p]; ok && len(srvs) > 0 {
			result = append(result, srvs...)
		} else {
			result = append(result, ActiveServer{
				Profile:    p,
				QuotaKnown: false,
			})
		}
	}

	for p, srvs := range liveByProfile {
		if !visited[p] {
			result = append(result, srvs...)
		}
	}

	if result == nil {
		result = []ActiveServer{}
	}
	return result, nil
}
