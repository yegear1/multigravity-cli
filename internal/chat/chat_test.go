package chat

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ye-dev/multigravity-cli/internal/profile"
)

func setupTestProfileWithChat(t *testing.T, baseDir, profName string) {
	t.Helper()
	profDir := filepath.Join(baseDir, profName)
	geminiDir := filepath.Join(profDir, ".gemini", "antigravity")
	_ = os.MkdirAll(filepath.Join(geminiDir, "conversations"), 0755)
	_ = os.MkdirAll(filepath.Join(geminiDir, "annotations"), 0755)
	_ = os.MkdirAll(filepath.Join(geminiDir, "brain", "test-uuid-1"), 0755)

	// Create conversation db, annotation, and brain markdown
	_ = os.WriteFile(filepath.Join(geminiDir, "conversations", "test-uuid-1.db"), []byte("sqlite data"), 0644)
	_ = os.WriteFile(filepath.Join(geminiDir, "annotations", "test-uuid-1.pbtxt"), []byte("title: \"Test Chat Session\""), 0644)
	_ = os.WriteFile(filepath.Join(geminiDir, "brain", "test-uuid-1", "plan.md"), []byte("# Plan"), 0644)

	// Create sensitive token file that must NOT be exported
	_ = os.WriteFile(filepath.Join(geminiDir, "jetski-standalone-oauth-token"), []byte("secret"), 0600)
	_ = os.WriteFile(filepath.Join(geminiDir, "conversations", "auth_token.txt"), []byte("secret"), 0600)
}

func TestChatListExportImportSync(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", tmpDir)
	t.Setenv("MULTIGRAVITY_TEST_SHORTCUTS_DIR", tmpDir)

	_ = profile.CreateProfile(profile.CreateOptions{Name: "source-prof"})
	_ = profile.CreateProfile(profile.CreateOptions{Name: "dest-prof"})

	setupTestProfileWithChat(t, tmpDir, "source-prof")

	// 1. Test ListConversations
	if err := ListConversations("source-prof"); err != nil {
		t.Fatalf("ListConversations failed: %v", err)
	}

	// 2. Test ExportConversations
	exportTar := filepath.Join(tmpDir, "source-export.tar.gz")
	if err := ExportConversations("source-prof", exportTar); err != nil {
		t.Fatalf("ExportConversations failed: %v", err)
	}

	// Verify that secret tokens were omitted from archive
	f, err := os.Open(exportTar)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	foundDB := false
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(hdr.Name, "token") || strings.Contains(hdr.Name, "oauth") {
			t.Errorf("archive contains sensitive file: %s", hdr.Name)
		}
		if strings.Contains(hdr.Name, "test-uuid-1.db") {
			foundDB = true
		}
	}
	if !foundDB {
		t.Errorf("test-uuid-1.db not found in export archive")
	}

	// 3. Test ImportConversations into empty dest
	if err := ImportConversations(exportTar, "dest-prof"); err != nil {
		t.Fatalf("ImportConversations failed: %v", err)
	}

	destDB := filepath.Join(tmpDir, "dest-prof", ".gemini", "antigravity", "conversations", "test-uuid-1.db")
	if _, err := os.Stat(destDB); err != nil {
		t.Fatalf("imported conversation db missing: %v", err)
	}

	// 4. Test SyncConversations
	_ = profile.CreateProfile(profile.CreateOptions{Name: "sync-prof"})
	if err := SyncConversations("source-prof", "sync-prof"); err != nil {
		t.Fatalf("SyncConversations failed: %v", err)
	}

	syncDB := filepath.Join(tmpDir, "sync-prof", ".gemini", "antigravity", "conversations", "test-uuid-1.db")
	if _, err := os.Stat(syncDB); err != nil {
		t.Fatalf("synced conversation db missing: %v", err)
	}
}
