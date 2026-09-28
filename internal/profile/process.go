package profile

import "strings"

// lineMatchesPath checks if targetPath appears in line as a distinct directory path or token,
// avoiding false positives when a profile name is a prefix of another (e.g. yegear vs yegear2).
func lineMatchesPath(line, targetPath string) bool {
	if targetPath == "" {
		return false
	}
	start := 0
	for {
		idx := strings.Index(line[start:], targetPath)
		if idx == -1 {
			return false
		}
		pos := start + idx
		endPos := pos + len(targetPath)

		// Check right boundary: must be end of string or a path/argument separator
		validRight := false
		if endPos == len(line) {
			validRight = true
		} else {
			c := line[endPos]
			if c == '/' || c == '\\' || c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '"' || c == '\'' || c == '=' || c == ':' {
				validRight = true
			}
		}

		// Check left boundary: must be start of string or a path/argument separator
		validLeft := false
		if pos == 0 {
			validLeft = true
		} else {
			c := line[pos-1]
			if c == '/' || c == '\\' || c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '"' || c == '\'' || c == '=' || c == ':' {
				validLeft = true
			}
		}

		if validLeft && validRight {
			return true
		}
		start = pos + 1
		if start >= len(line) {
			return false
		}
	}
}
