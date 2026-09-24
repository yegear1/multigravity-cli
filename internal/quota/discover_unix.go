//go:build !windows

package quota

import (
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)


var csrfRegex = regexp.MustCompile(`--csrf_token\s+([a-f0-9-]+)`)
var profileRegex = regexp.MustCompile(`AntigravityProfiles/([^/ ]+)`)
var listenRegex = regexp.MustCompile(`:(\d+)\s+\(LISTEN\)`)
var ssPortRegex = regexp.MustCompile(`:(\d+)\s+`)

func FindActiveServers(targetProf string) ([]ActiveServer, error) {
	cmd := exec.Command("ps", "-eo", "pid,ppid,args")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(out), "\n")
	client := NewClient(2 * time.Second)

	var servers []ActiveServer
	seenPorts := make(map[int]bool)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "language_server") || !strings.Contains(line, "--csrf_token") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}

		pidStr := parts[0]
		ppidStr := parts[1]
		cmdArgs := strings.Join(parts[2:], " ")

		csrfMatch := csrfRegex.FindStringSubmatch(cmdArgs)
		if len(csrfMatch) < 2 {
			continue
		}
		csrf := csrfMatch[1]

		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}

		profile := "host"
		if ppidStr != "" && ppidStr != "0" {
			pCmd := exec.Command("ps", "-p", ppidStr, "-o", "args=")
			if pOut, err := pCmd.Output(); err == nil {
				pMatch := profileRegex.FindStringSubmatch(string(pOut))
				if len(pMatch) >= 2 {
					profile = pMatch[1]
				}
			}
		}

		if targetProf != "" && profile != targetProf {
			continue
		}

		var ports []int
		// Try lsof first
		lsofCmd := exec.Command("lsof", "-Pan", "-p", pidStr, "-i")
		if lsofOut, err := lsofCmd.Output(); err == nil {
			for _, l := range strings.Split(string(lsofOut), "\n") {
				if strings.Contains(l, "LISTEN") {
					m := listenRegex.FindStringSubmatch(l)
					if len(m) >= 2 {
						if p, err := strconv.Atoi(m[1]); err == nil {
							ports = append(ports, p)
						}
					}
				}
			}
		}

		// Fallback to ss -tulpn
		if len(ports) == 0 {
			ssCmd := exec.Command("ss", "-tulpn")
			if ssOut, err := ssCmd.Output(); err == nil {
				for _, l := range strings.Split(string(ssOut), "\n") {
					if strings.Contains(l, "pid="+pidStr+",") || strings.Contains(l, "pid="+pidStr+")") {
						m := ssPortRegex.FindStringSubmatch(l)
						if len(m) >= 2 {
							if p, err := strconv.Atoi(m[1]); err == nil {
								ports = append(ports, p)
							}
						}
					}
				}
			}
		}

		for _, port := range ports {
			if seenPorts[port] {
				continue
			}
			res, err := client.RetrieveUserQuotaSummary(port, csrf)
			if err == nil && res != nil {
				seenPorts[port] = true
				servers = append(servers, ActiveServer{
					Profile: profile,
					PID:     pid,
					Port:    port,
					CSRF:    csrf,
					Data:    res,
				})
				break
			}
		}
	}

	return servers, nil
}
