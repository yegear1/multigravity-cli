package dispatch

import (
	"testing"
)

func TestParseUnifiedDiffEmpty(t *testing.T) {
	d := ParseUnifiedDiff("")
	if d == nil {
		t.Fatal("expected non-nil StructuredDiff")
	}
	if len(d.Files) != 0 {
		t.Errorf("expected 0 files, got %d", len(d.Files))
	}
	if d.Summary.FilesChanged != 0 || d.Summary.Additions != 0 || d.Summary.Deletions != 0 {
		t.Errorf("expected empty summary, got %+v", d.Summary)
	}
}

func TestParseUnifiedDiffModifiedFile(t *testing.T) {
	sampleDiff := `diff --git a/main.go b/main.go
index 83db48f..bf269f4 100644
--- a/main.go
+++ b/main.go
@@ -1,5 +1,6 @@
 package main
 
+import "fmt"
+
 func main() {
-	println("hello")
+	fmt.Println("hello world")
 }
`

	d := ParseUnifiedDiff(sampleDiff)
	if len(d.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(d.Files))
	}

	f := d.Files[0]
	if f.OldPath != "main.go" || f.NewPath != "main.go" {
		t.Errorf("unexpected paths: old=%q new=%q", f.OldPath, f.NewPath)
	}
	if f.Status != DiffFileModified {
		t.Errorf("expected modified status, got %q", f.Status)
	}
	if f.Additions != 3 {
		t.Errorf("expected 3 additions, got %d", f.Additions)
	}
	if f.Deletions != 1 {
		t.Errorf("expected 1 deletion, got %d", f.Deletions)
	}
	if len(f.Hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(f.Hunks))
	}

	h := f.Hunks[0]
	if h.OldStart != 1 || h.OldLines != 5 || h.NewStart != 1 || h.NewLines != 6 {
		t.Errorf("unexpected hunk line ranges: -%d,%d +%d,%d", h.OldStart, h.OldLines, h.NewStart, h.NewLines)
	}

	if d.Summary.FilesChanged != 1 || d.Summary.Additions != 3 || d.Summary.Deletions != 1 {
		t.Errorf("unexpected summary: %+v", d.Summary)
	}
}

func TestParseUnifiedDiffAddedAndDeletedFiles(t *testing.T) {
	sampleDiff := `diff --git a/new_file.txt b/new_file.txt
new file mode 100644
--- /dev/null
+++ b/new_file.txt
@@ -0,0 +1,2 @@
+Line 1
+Line 2
diff --git a/old_file.txt b/old_file.txt
deleted file mode 100644
--- a/old_file.txt
+++ /dev/null
@@ -1,2 +0,0 @@
-Old line 1
-Old line 2
diff --git a/logo.png b/logo.png
new file mode 100644
Binary files /dev/null and b/logo.png differ
`

	d := ParseUnifiedDiff(sampleDiff)
	if len(d.Files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(d.Files))
	}

	// 1. Added
	f1 := d.Files[0]
	if f1.NewPath != "new_file.txt" || f1.Status != DiffFileAdded {
		t.Errorf("expected added file, got %+v", f1)
	}
	if f1.Additions != 2 || f1.Deletions != 0 {
		t.Errorf("expected 2 additions, got +%d -%d", f1.Additions, f1.Deletions)
	}

	// 2. Deleted
	f2 := d.Files[1]
	if f2.OldPath != "old_file.txt" || f2.Status != DiffFileDeleted {
		t.Errorf("expected deleted file, got %+v", f2)
	}
	if f2.Additions != 0 || f2.Deletions != 2 {
		t.Errorf("expected 2 deletions, got +%d -%d", f2.Additions, f2.Deletions)
	}

	// 3. Binary
	f3 := d.Files[2]
	if !f3.Binary {
		t.Errorf("expected binary file, got binary=%v", f3.Binary)
	}

	if d.Summary.FilesChanged != 3 {
		t.Errorf("expected 3 files changed, got %d", d.Summary.FilesChanged)
	}
	if d.Summary.Additions != 2 || d.Summary.Deletions != 2 {
		t.Errorf("expected summary +2 -2, got +%d -%d", d.Summary.Additions, d.Summary.Deletions)
	}
}

func TestParseUnifiedDiffRenamedFile(t *testing.T) {
	sampleDiff := `diff --git a/original.go b/renamed.go
similarity index 100%
rename from original.go
rename to renamed.go
`
	d := ParseUnifiedDiff(sampleDiff)
	if len(d.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(d.Files))
	}
	f := d.Files[0]
	if f.Status != DiffFileRenamed {
		t.Errorf("expected renamed status, got %s", f.Status)
	}
	if f.OldPath != "original.go" || f.NewPath != "renamed.go" {
		t.Errorf("unexpected paths: old=%s, new=%s", f.OldPath, f.NewPath)
	}
}
