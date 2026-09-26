package profile

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/ye-dev/multigravity-cli/internal/config"
)

const (
	includeProfile       = "profile"
	includeConversations = "conversations"
)

var snapshotIDPattern = regexp.MustCompile(`^[0-9]{8}T[0-9]{6}Z(?:-[0-9]+)?$`)

// Snapshot is a restore point for one profile and its conversations.
// Auth tokens and host credential trees are never stored in the archive.
type Snapshot struct {
	ID        string    `json:"id"`
	Profile   string    `json:"profile"`
	CreatedAt time.Time `json:"created_at"`
	Note      string    `json:"note,omitempty"`
	Archive   string    `json:"archive"`
	Bytes     int64     `json:"bytes"`
	Includes  []string  `json:"includes"`
}

// ProfileBusyError means a write was refused because the profile has live processes.
type ProfileBusyError struct {
	Name string
}

func (e *ProfileBusyError) Error() string {
	return fmt.Sprintf("profile %q is currently running — stop it first (multigravity stop %s)", e.Name, e.Name)
}

// SnapshotNotFoundError means the id is not a restore point of this profile.
type SnapshotNotFoundError struct {
	Profile string
	ID      string
}

func (e *SnapshotNotFoundError) Error() string {
	return fmt.Sprintf("snapshot %q not found for profile %q", e.ID, e.Profile)
}

// SnapshotDir is the store for a profile's restore points, outside the profile tree.
func SnapshotDir(name string) string {
	return filepath.Join(config.GetMultigravityHome(), ".snapshots", name)
}

// CreateSnapshot writes a restore point. It refuses while the profile is running
// so the archive is not taken from an unflushed SQLite tree.
func CreateSnapshot(name, note string) (Snapshot, error) {
	if err := config.ValidateProfileName(name); err != nil {
		return Snapshot{}, err
	}
	profileDir := config.GetProfileDir(name)
	if _, err := os.Stat(profileDir); err != nil {
		if os.IsNotExist(err) {
			return Snapshot{}, fmt.Errorf("profile %q does not exist", name)
		}
		return Snapshot{}, err
	}
	if IsProfileRunning(name) {
		return Snapshot{}, &ProfileBusyError{Name: name}
	}

	note = sanitizeNote(note)
	store := SnapshotDir(name)
	if err := os.MkdirAll(store, 0700); err != nil {
		return Snapshot{}, fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	created := time.Now().UTC().Truncate(time.Second)
	id := created.Format("20060102T150405Z")
	archivePath := filepath.Join(store, id+".tar.gz")
	for n := 2; fileExists(archivePath) || fileExists(filepath.Join(store, id+".json")); n++ {
		id = fmt.Sprintf("%s-%d", created.Format("20060102T150405Z"), n)
		archivePath = filepath.Join(store, id+".tar.gz")
	}

	includes, err := writeSnapshotArchive(profileDir, archivePath)
	if err != nil {
		_ = os.Remove(archivePath)
		return Snapshot{}, err
	}

	info, err := os.Stat(archivePath)
	if err != nil {
		_ = os.Remove(archivePath)
		return Snapshot{}, err
	}

	snap := Snapshot{
		ID:        id,
		Profile:   name,
		CreatedAt: created,
		Note:      note,
		Archive:   archivePath,
		Bytes:     info.Size(),
		Includes:  includes,
	}
	if err := writeSnapshotManifest(snap); err != nil {
		_ = os.Remove(archivePath)
		return Snapshot{}, err
	}
	return snap, nil
}

// ListSnapshots returns restore points for a profile, newest first.
func ListSnapshots(name string) ([]Snapshot, error) {
	if err := config.ValidateProfileName(name); err != nil {
		return nil, err
	}
	if !ProfileExists(name) {
		return nil, fmt.Errorf("profile %q does not exist", name)
	}
	entries, err := os.ReadDir(SnapshotDir(name))
	if err != nil {
		if os.IsNotExist(err) {
			return []Snapshot{}, nil
		}
		return nil, err
	}
	var snaps []Snapshot
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		snap, err := readSnapshotManifest(filepath.Join(SnapshotDir(name), entry.Name()))
		if err != nil {
			continue
		}
		if snap.Profile != name {
			continue
		}
		snaps = append(snaps, snap)
	}
	sort.Slice(snaps, func(i, j int) bool {
		return snaps[i].CreatedAt.After(snaps[j].CreatedAt)
	})
	if snaps == nil {
		snaps = []Snapshot{}
	}
	return snaps, nil
}

// RollbackSnapshot replaces profile and conversation files with a restore point.
// Token files and credential trees that were left out of the package stay on disk.
// The write is refused while the profile is running.
func RollbackSnapshot(name, id string) (Snapshot, error) {
	if err := config.ValidateProfileName(name); err != nil {
		return Snapshot{}, err
	}
	if err := validateSnapshotID(id); err != nil {
		return Snapshot{}, err
	}
	if !ProfileExists(name) {
		return Snapshot{}, fmt.Errorf("profile %q does not exist", name)
	}
	if IsProfileRunning(name) {
		return Snapshot{}, &ProfileBusyError{Name: name}
	}

	snap, err := loadSnapshot(name, id)
	if err != nil {
		return Snapshot{}, err
	}

	profileDir := config.GetProfileDir(name)
	home := config.GetMultigravityHome()
	preserveDir, err := os.MkdirTemp(home, ".snapshot-keep-")
	if err != nil {
		return Snapshot{}, fmt.Errorf("failed to stage preserved credentials: %w", err)
	}
	defer os.RemoveAll(preserveDir)

	if err := copyPreserved(profileDir, preserveDir); err != nil {
		return Snapshot{}, err
	}

	bak := filepath.Join(home, ".rollback-"+name)
	if _, err := os.Lstat(bak); err == nil {
		return Snapshot{}, fmt.Errorf("incomplete rollback leftover at %s", bak)
	}
	if err := os.Rename(profileDir, bak); err != nil {
		return Snapshot{}, fmt.Errorf("failed to move profile aside: %w", err)
	}

	restoreBak := func() {
		_ = os.RemoveAll(profileDir)
		_ = os.Rename(bak, profileDir)
	}

	if err := extractSnapshotArchive(snap.Archive, profileDir); err != nil {
		restoreBak()
		return Snapshot{}, err
	}
	if err := CopyDir(preserveDir, profileDir); err != nil {
		restoreBak()
		return Snapshot{}, fmt.Errorf("failed to restore preserved credentials: %w", err)
	}
	if err := os.RemoveAll(bak); err != nil {
		return Snapshot{}, fmt.Errorf("snapshot restored but failed to remove backup: %w", err)
	}
	return snap, nil
}

// DeleteSnapshot removes a restore point. It does not write the profile tree.
func DeleteSnapshot(name, id string) error {
	if err := config.ValidateProfileName(name); err != nil {
		return err
	}
	if err := validateSnapshotID(id); err != nil {
		return err
	}
	if !ProfileExists(name) {
		return fmt.Errorf("profile %q does not exist", name)
	}
	snap, err := loadSnapshot(name, id)
	if err != nil {
		return err
	}
	if err := os.Remove(snap.Archive); err != nil && !os.IsNotExist(err) {
		return err
	}
	manifest := filepath.Join(SnapshotDir(name), id+".json")
	if err := os.Remove(manifest); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func loadSnapshot(name, id string) (Snapshot, error) {
	manifest := filepath.Join(SnapshotDir(name), id+".json")
	snap, err := readSnapshotManifest(manifest)
	if err != nil {
		if os.IsNotExist(err) {
			return Snapshot{}, &SnapshotNotFoundError{Profile: name, ID: id}
		}
		return Snapshot{}, err
	}
	if snap.Profile != name || snap.ID != id {
		return Snapshot{}, &SnapshotNotFoundError{Profile: name, ID: id}
	}
	if _, err := os.Stat(snap.Archive); err != nil {
		if os.IsNotExist(err) {
			return Snapshot{}, fmt.Errorf("snapshot %q archive is missing", id)
		}
		return Snapshot{}, err
	}
	return snap, nil
}

func writeSnapshotArchive(profileDir, archivePath string) ([]string, error) {
	partial := archivePath + ".partial"
	out, err := os.OpenFile(partial, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot archive: %w", err)
	}

	conversations := false
	gw := gzip.NewWriter(out)
	tw := tar.NewWriter(gw)
	walkErr := filepath.WalkDir(profileDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(profileDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		if isCacheDir(relSlash) || isSecretPath(relSlash) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if relSlash == ".gemini/antigravity/conversations" || strings.HasPrefix(relSlash, ".gemini/antigravity/conversations/") {
			conversations = true
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		var linkTarget string
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err = os.Readlink(path)
			if err != nil {
				return fmt.Errorf("failed to readlink %q: %w", path, err)
			}
		}
		header, err := tar.FileInfoHeader(info, linkTarget)
		if err != nil {
			return err
		}
		header.Name = relSlash
		if d.IsDir() {
			header.Name += "/"
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(tw, f)
			f.Close()
			if copyErr != nil {
				return copyErr
			}
		}
		return nil
	})

	closeErr := errors.Join(walkErr, tw.Close(), gw.Close(), out.Close())
	if closeErr != nil {
		_ = os.Remove(partial)
		return nil, closeErr
	}
	if err := os.Rename(partial, archivePath); err != nil {
		_ = os.Remove(partial)
		return nil, err
	}
	if err := os.Chmod(archivePath, 0600); err != nil {
		return nil, err
	}

	includes := []string{includeProfile}
	if conversations {
		includes = append(includes, includeConversations)
	}
	return includes, nil
}

func extractSnapshotArchive(archivePath, dest string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("failed to read snapshot archive: %w", err)
	}
	defer gr.Close()

	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}

	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read snapshot entry: %w", err)
		}
		rel, err := cleanArchiveRel(header.Name)
		if err != nil {
			return err
		}
		if rel == "" {
			continue
		}
		if isSecretPath(rel) || isCacheDir(rel) {
			continue
		}
		target := filepath.Join(dest, filepath.FromSlash(rel))
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			_ = os.Remove(target)
			if err := os.Symlink(header.Linkname, target); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			mode := header.FileInfo().Mode().Perm()
			if mode == 0 {
				mode = 0644
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(out, tr)
			closeErr := out.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		}
	}
	return nil
}

func copyPreserved(profileDir, dest string) error {
	if err := os.MkdirAll(dest, 0700); err != nil {
		return err
	}
	return filepath.WalkDir(profileDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(profileDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		if isCacheDir(relSlash) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if !isSecretPath(relSlash) {
			return nil
		}
		target := filepath.Join(dest, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
				return err
			}
			_ = os.Remove(target)
			if err := os.Symlink(link, target); err != nil {
				return err
			}
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if err := CopyDir(path, target); err != nil {
				return err
			}
			return fs.SkipDir
		}
		return CopyFile(path, target)
	})
}

func writeSnapshotManifest(snap Snapshot) error {
	manifest := filepath.Join(SnapshotDir(snap.Profile), snap.ID+".json")
	partial := manifest + ".partial"
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(partial, append(data, '\n'), 0600); err != nil {
		return err
	}
	return os.Rename(partial, manifest)
}

func readSnapshotManifest(path string) (Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{}, err
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return Snapshot{}, err
	}
	return snap, nil
}

func cleanArchiveRel(name string) (string, error) {
	clean := filepath.ToSlash(filepath.Clean(name))
	if clean == "." {
		return "", nil
	}
	if strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, "../") || clean == ".." || filepath.IsAbs(name) {
		return "", fmt.Errorf("illegal file path in snapshot: %q", name)
	}
	return clean, nil
}

func validateSnapshotID(id string) error {
	if !snapshotIDPattern.MatchString(id) {
		return fmt.Errorf("invalid snapshot id %q", id)
	}
	return nil
}

func sanitizeNote(note string) string {
	note = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, strings.TrimSpace(note))
	if len(note) > 240 {
		note = note[:240]
	}
	return note
}

func isSentinelName(name string) bool {
	switch name {
	case config.SentinelIsolatedMCP,
		config.SentinelIsolatedSkills,
		config.SentinelIsolatedConfig,
		config.SentinelIsolatedGH,
		config.SentinelIsolatedDotfiles,
		config.SentinelShared,
		config.SentinelAuthOnly:
		return true
	default:
		return false
	}
}

func credentialFileName(name string) bool {
	if isSentinelName(name) {
		return false
	}
	lower := strings.ToLower(name)
	if strings.Contains(lower, "token") ||
		strings.Contains(lower, "oauth") ||
		strings.Contains(lower, "credential") ||
		strings.Contains(lower, "installation_id") {
		return true
	}
	// "antigravity" contains the substring "auth" and is the conversation tree, not a credential.
	if strings.Contains(lower, "antigravity") {
		return false
	}
	return strings.Contains(lower, "auth")
}

func isSecretPath(rel string) bool {
	rel = filepath.ToSlash(filepath.Clean(rel))
	if rel == "." || rel == "" {
		return false
	}
	secretRoots := []string{
		".ssh",
		".gnupg",
		".config/gh",
		"AppData/Roaming/GitHub CLI",
	}
	for _, root := range secretRoots {
		if rel == root || strings.HasPrefix(rel, root+"/") {
			return true
		}
	}
	return credentialFileName(filepath.Base(rel))
}

func fileExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}
