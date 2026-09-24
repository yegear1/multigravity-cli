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

func TestChatActiveAndArchivedFiltering(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("MULTIGRAVITY_HOME", tmpDir)
	t.Setenv("MULTIGRAVITY_TEST_SHORTCUTS_DIR", tmpDir)

	_ = profile.CreateProfile(profile.CreateOptions{Name: "filter-prof"})
	profDir := filepath.Join(tmpDir, "filter-prof")
	geminiDir := filepath.Join(profDir, ".gemini", "antigravity")
	_ = os.MkdirAll(filepath.Join(geminiDir, "conversations"), 0755)
	_ = os.MkdirAll(filepath.Join(geminiDir, "annotations"), 0755)
	_ = os.MkdirAll(filepath.Join(geminiDir, "brain", "active-conv", ".system_generated", "logs"), 0755)
	_ = os.MkdirAll(filepath.Join(geminiDir, "brain", "archived-conv", ".system_generated", "logs"), 0755)

	// Create active conversation
	_ = os.WriteFile(filepath.Join(geminiDir, "conversations", "active-conv.db"), []byte("data"), 0644)
	_ = os.WriteFile(filepath.Join(geminiDir, "annotations", "active-conv.pbtxt"), []byte("last_user_view_time:{seconds:1790000000 nanos:0}"), 0644)
	transcriptActive := `{"type":"USER_INPUT","content":"<USER_REQUEST>\n@[AGENTS.md] Como funciona a CLI?\n</USER_REQUEST>"}`
	_ = os.WriteFile(filepath.Join(geminiDir, "brain", "active-conv", ".system_generated", "logs", "transcript.jsonl"), []byte(transcriptActive), 0644)

	// Create archived conversation
	_ = os.WriteFile(filepath.Join(geminiDir, "conversations", "archived-conv.db"), []byte("data"), 0644)
	_ = os.WriteFile(filepath.Join(geminiDir, "annotations", "archived-conv.pbtxt"), []byte("archived:true archival_status_timestamp:{seconds:1790001000 nanos:0} title:\"Old Task\""), 0644)

	// 1. GetFilteredConversations with FilterAll
	allConvs, err := GetFilteredConversations("filter-prof", FilterAll)
	if err != nil {
		t.Fatalf("FilterAll failed: %v", err)
	}
	if len(allConvs) != 2 {
		t.Fatalf("expected 2 conversations, got %d", len(allConvs))
	}

	// Active should be first
	if allConvs[0].Archived {
		t.Errorf("expected first conversation to be active")
	}
	if allConvs[0].Title != "Como funciona a CLI?" {
		t.Errorf("expected title extracted from transcript, got: %q", allConvs[0].Title)
	}
	if !allConvs[1].Archived {
		t.Errorf("expected second conversation to be archived")
	}
	if allConvs[1].Title != "Old Task" {
		t.Errorf("expected title 'Old Task', got: %q", allConvs[1].Title)
	}

	// 2. FilterActive
	activeConvs, err := GetFilteredConversations("filter-prof", FilterActive)
	if err != nil {
		t.Fatalf("FilterActive failed: %v", err)
	}
	if len(activeConvs) != 1 || activeConvs[0].ID != "active-conv" {
		t.Errorf("FilterActive unexpected result: %+v", activeConvs)
	}

	// 3. FilterArchived
	archivedConvs, err := GetFilteredConversations("filter-prof", FilterArchived)
	if err != nil {
		t.Fatalf("FilterArchived failed: %v", err)
	}
	if len(archivedConvs) != 1 || archivedConvs[0].ID != "archived-conv" {
		t.Errorf("FilterArchived unexpected result: %+v", archivedConvs)
	}

	// 4. Output formatting
	var buf strings.Builder
	if err := ListConversationsFilter(&buf, "filter-prof", FilterAll); err != nil {
		t.Fatalf("ListConversationsFilter failed: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "active") || !strings.Contains(out, "archived") {
		t.Errorf("expected active and archived in output, got:\n%s", out)
	}
	if !strings.Contains(out, "SIZE") || !strings.Contains(out, "Total size:") {
		t.Errorf("expected SIZE and Total size in output, got:\n%s", out)
	}

	// 5. Check size calculation
	if allConvs[0].SizeBytes <= 0 || allConvs[0].Size == "" {
		t.Errorf("expected size > 0, got bytes=%d size=%s", allConvs[0].SizeBytes, allConvs[0].Size)
	}
}

func TestChatFormatBytes(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0B"},
		{500, "500B"},
		{1024, "1.0K"},
		{1536, "1.5K"},
		{1048576, "1.0M"},
		{5242880, "5.0M"},
		{1073741824, "1.0G"},
	}

	for _, tt := range tests {
		got := formatBytes(tt.bytes)
		if got != tt.want {
			t.Errorf("formatBytes(%d) = %s, want %s", tt.bytes, got, tt.want)
		}
	}
}
