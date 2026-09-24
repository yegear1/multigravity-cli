//go:build windows

package quota

import (
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var winCsrfRegex = regexp.MustCompile(`--csrf_token\s+([a-f0-9-]+)`)
var winProfileRegex = regexp.MustCompile(`AntigravityProfiles[\\/]([^\\/ ]+)`)
var winNetstatRegex = regexp.MustCompile(`TCP\s+127\.0\.0\.1:(\d+)\s+.*LISTENING\s+(\d+)`)

func FindActiveServers(targetProf string) ([]ActiveServer, error) {
	// Query processes via PowerShell / Get-CimInstance
	psScript := `Get-CimInstance Win32_Process | Where-Object { $_.CommandLine -like "*language_server*" } | Select-Object ProcessId, ParentProcessId, CommandLine | ConvertTo-Json -Compress`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psScript)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	raw := string(out)
	if strings.TrimSpace(raw) == "" || raw == "null" {
		return nil, nil
	}

	// Parse netstat -ano for listening ports mapped to PIDs
	netCmd := exec.Command("netstat", "-ano")
	netOut, _ := netCmd.Output()
	pidPorts := make(map[int][]int)
	for _, line := range strings.Split(string(netOut), "\n") {
		m := winNetstatRegex.FindStringSubmatch(line)
		if len(m) >= 3 {
			port, err1 := strconv.Atoi(m[1])
			pid, err2 := strconv.Atoi(m[2])
			if err1 == nil && err2 == nil {
				pidPorts[pid] = append(pidPorts[pid], port)
			}
		}
	}

	client := NewClient(2 * time.Second)
	var servers []ActiveServer
	seenPorts := make(map[int]bool)

	// In PowerShell JSON output, if single object or array, extract
	// Quick regex scanning of processes from JSON or CommandLine
	// We can match entries using regex or simple parsing
	procBlocks := strings.Split(raw, "}")
	for _, block := range procBlocks {
		if !strings.Contains(block, "language_server") || !strings.Contains(block, "--csrf_token") {
			continue
		}

		csrfMatch := winCsrfRegex.FindStringSubmatch(block)
		if len(csrfMatch) < 2 {
			continue
		}
		csrf := csrfMatch[1]

		pidMatch := regexp.MustCompile(`"ProcessId":\s*(\d+)`).FindStringSubmatch(block)
		if len(pidMatch) < 2 {
			continue
		}
		pid, _ := strconv.Atoi(pidMatch[1])

		profile := "host"
		pMatch := winProfileRegex.FindStringSubmatch(block)
		if len(pMatch) >= 2 {
			profile = pMatch[1]
		}

		if targetProf != "" && profile != targetProf {
			continue
		}

		ports := pidPorts[pid]
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
