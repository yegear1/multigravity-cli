package idelog

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/config"
)

const SourceIDE = "ide"

// pollInterval is how often Follow re-reads the IDE log.
var pollInterval = 200 * time.Millisecond

var (
	csrfTokenRe       = regexp.MustCompile(`(?i)(--csrf_token(?:=|\s+))\S+`)
	hostBridgeTokenRe = regexp.MustCompile(`(?i)(--host_bridge_token(?:=|\s+))\S+`)
)

// Snapshot is a sanitized tail of the graphical IDE main-process log.
type Snapshot struct {
	Profile string `json:"profile"`
	Path    string `json:"path"`
	Source  string `json:"source"`
	Logs    string `json:"logs"`
}

// MainLogPath returns <user-data-dir>/logs/main.log for the profile.
// The managed language server and dispatch tasks keep their own logs.
func MainLogPath(profileName string) (string, error) {
	if err := config.ValidateProfileName(profileName); err != nil {
		return "", err
	}
	profileDir := config.GetProfileDir(profileName)
	if _, err := os.Stat(profileDir); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("profile %q does not exist", profileName)
		}
		return "", err
	}
	return filepath.Join(config.GetUserDataDir(profileDir), "logs", "main.log"), nil
}

// Sanitize redacts session secrets that the IDE writes into main.log.
func Sanitize(text string) string {
	text = csrfTokenRe.ReplaceAllString(text, "${1}[redacted]")
	text = hostBridgeTokenRe.ReplaceAllString(text, "${1}[redacted]")
	return text
}

// Read returns the sanitized tail of the IDE main log.
// tailLines <= 0 returns the whole file.
func Read(profileName string, tailLines int) (Snapshot, error) {
	path, err := MainLogPath(profileName)
	if err != nil {
		return Snapshot{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Snapshot{}, fmt.Errorf("ide log not found for profile %q", profileName)
		}
		return Snapshot{}, err
	}
	return Snapshot{
		Profile: profileName,
		Path:    path,
		Source:  SourceIDE,
		Logs:    Sanitize(tailText(string(data), tailLines)),
	}, nil
}

// Follow streams sanitized IDE log text. The first value is the current tail.
// The channel closes when ctx is done.
func Follow(ctx context.Context, profileName string, tailLines int) (<-chan string, error) {
	path, err := MainLogPath(profileName)
	if err != nil {
		return nil, err
	}
	ch := make(chan string)
	go followLoop(ctx, path, tailLines, ch)
	return ch, nil
}

func followLoop(ctx context.Context, path string, tailLines int, ch chan<- string) {
	defer close(ch)

	var (
		offset  int64
		pending string
		started bool
	)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		if ctx.Err() != nil {
			return
		}
		nextOffset, chunk, truncated, missing, err := readDelta(path, offset, !started, tailLines, &pending)
		if err != nil {
			return
		}
		if truncated {
			offset = 0
			pending = ""
			started = false
		} else if !missing {
			offset = nextOffset
			started = true
			if chunk != "" && !send(ctx, ch, chunk) {
				return
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func send(ctx context.Context, ch chan<- string, chunk string) bool {
	select {
	case <-ctx.Done():
		return false
	case ch <- chunk:
		return true
	}
}

func readDelta(path string, offset int64, initial bool, tailLines int, pending *string) (int64, string, bool, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return offset, "", false, true, nil
		}
		return offset, "", false, false, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return offset, "", false, false, err
	}
	size := fi.Size()
	if size < offset {
		return 0, "", true, false, nil
	}
	if size == offset {
		return offset, "", false, false, nil
	}

	buf := make([]byte, size-offset)
	n, err := f.ReadAt(buf, offset)
	if n > 0 {
		buf = buf[:n]
	} else {
		buf = nil
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return offset, "", false, false, err
	}
	if n == 0 {
		return offset, "", false, false, nil
	}

	text := string(buf)
	if initial {
		return offset + int64(len(buf)), Sanitize(tailText(text, tailLines)), false, false, nil
	}

	*pending += text
	var b strings.Builder
	for {
		i := strings.IndexByte(*pending, '\n')
		if i < 0 {
			break
		}
		line := (*pending)[:i+1]
		*pending = (*pending)[i+1:]
		b.WriteString(Sanitize(line))
	}
	return offset + int64(len(buf)), b.String(), false, false, nil
}

func tailText(data string, n int) string {
	if n <= 0 {
		if data == "" || strings.HasSuffix(data, "\n") {
			return data
		}
		return data + "\n"
	}
	text := strings.TrimRight(data, "\n")
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n") + "\n"
}
