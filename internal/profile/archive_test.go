package profile

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestExportAndImportProfile(t *testing.T) {
	tempHome := setupTestHome(t)

	// Create profile
	err := CreateProfile(CreateOptions{
		Name:             "exportable",
		IsolatedDotfiles: true,
	})
	if err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	pDir := filepath.Join(tempHome, "exportable")

	// Regular data
	userDataDir := filepath.Join(pDir, ".config", "Antigravity")
	_ = os.MkdirAll(userDataDir, 0755)
	_ = os.WriteFile(filepath.Join(userDataDir, "settings.json"), []byte(`{"theme": "dark"}`), 0644)

	// Cache directories
	cacheDir := filepath.Join(pDir, ".cache")
	_ = os.MkdirAll(cacheDir, 0755)
	_ = os.WriteFile(filepath.Join(cacheDir, "temp.log"), []byte("volatile cache"), 0644)

	gpuCacheDir := filepath.Join(userDataDir, "GPUCache")
	_ = os.MkdirAll(gpuCacheDir, 0755)
	_ = os.WriteFile(filepath.Join(gpuCacheDir, "data.bin"), []byte("gpu data"), 0644)

	tempDir := t.TempDir()
	tarGzOut := filepath.Join(tempDir, "exportable.tar.gz")

	// 1. Export without cache (default)
	exportedPath, err := ExportProfile("exportable", tarGzOut, false)
	if err != nil {
		t.Fatalf("ExportProfile failed: %v", err)
	}
	if exportedPath != tarGzOut {
		t.Errorf("expected exported path %q, got %q", tarGzOut, exportedPath)
	}

	// Verify tar.gz contents do not contain cache
	f, err := os.Open(tarGzOut)
	if err != nil {
		t.Fatalf("failed to open exported tar: %v", err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("failed to read gzip: %v", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	foundSettings := false
	foundCache := false

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("error reading tar entry: %v", err)
		}
		if filepath.Base(hdr.Name) == "settings.json" {
			foundSettings = true
		}
		if filepath.Base(hdr.Name) == "temp.log" || filepath.Base(hdr.Name) == "data.bin" {
			foundCache = true
		}
	}

	if !foundSettings {
		t.Errorf("expected settings.json in archive, but was not found")
	}
	if foundCache {
		t.Errorf("expected volatile cache files to be excluded from export, but found some")
	}

	// 2. Export with cache
	tarGzWithCache := filepath.Join(tempDir, "exportable_full.tar.gz")
	_, err = ExportProfile("exportable", tarGzWithCache, true)
	if err != nil {
		t.Fatalf("ExportProfile with cache failed: %v", err)
	}

	fFull, _ := os.Open(tarGzWithCache)
	defer fFull.Close()
	grFull, _ := gzip.NewReader(fFull)
	defer grFull.Close()
	trFull := tar.NewReader(grFull)
	foundCacheInFull := false
	for {
		hdr, err := trFull.Next()
		if err == io.EOF {
			break
		}
		if filepath.Base(hdr.Name) == "temp.log" {
			foundCacheInFull = true
		}
	}
	if !foundCacheInFull {
		t.Errorf("expected temp.log in full export with cache, but not found")
	}

	// 3. Export to zip
	zipOut := filepath.Join(tempDir, "exportable.zip")
	_, err = ExportProfile("exportable", zipOut, false)
	if err != nil {
		t.Fatalf("ExportProfile to zip failed: %v", err)
	}

	// 4. Import from tar.gz with explicit name
	importedName, err := ImportProfile(tarGzOut, "imported-tar")
	if err != nil {
		t.Fatalf("ImportProfile failed: %v", err)
	}
	if importedName != "imported-tar" {
		t.Errorf("expected imported name 'imported-tar', got %q", importedName)
	}

	importedDir := filepath.Join(tempHome, "imported-tar")
	importedSettings := filepath.Join(importedDir, ".config", "Antigravity", "settings.json")
	if _, err := os.Stat(importedSettings); os.IsNotExist(err) {
		t.Fatalf("expected imported settings %q to exist", importedSettings)
	}

	// 5. Import from zip with inferred name
	importedZipName, err := ImportProfile(zipOut, "")
	if err != nil {
		// Since "exportable" already exists, it should fail
		if err == nil {
			t.Errorf("expected error importing duplicate inferred name, got nil")
		}
	}

	importedZipName, err = ImportProfile(zipOut, "imported-zip")
	if err != nil {
		t.Fatalf("ImportProfile from zip failed: %v", err)
	}
	if importedZipName != "imported-zip" {
		t.Errorf("expected imported name 'imported-zip', got %q", importedZipName)
	}

	// 6. Import non-existent archive
	_, err = ImportProfile(filepath.Join(tempDir, "nonexistent.tar.gz"), "fail")
	if err == nil {
		t.Errorf("expected error importing non-existent archive, got nil")
	}
}

func TestZipSlipProtection(t *testing.T) {
	tempHome := setupTestHome(t)
	tempDir := t.TempDir()

	// Create a malicious tar.gz with path traversal
	maliciousTar := filepath.Join(tempDir, "malicious.tar.gz")
	f, err := os.Create(maliciousTar)
	if err != nil {
		t.Fatalf("failed to create test tar: %v", err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	hdr := &tar.Header{
		Name:     "../evil.txt",
		Mode:     0644,
		Size:     4,
		Typeflag: tar.TypeReg,
	}
	_ = tw.WriteHeader(hdr)
	_, _ = tw.Write([]byte("evil"))
	_ = tw.Close()
	_ = gw.Close()

	_, err = ImportProfile(maliciousTar, "safe-profile")
	if err == nil {
		t.Errorf("expected Zip Slip attack to be rejected, but import succeeded")
	}

	// Ensure evil.txt was not created outside
	if _, err := os.Stat(filepath.Join(tempHome, "..", "evil.txt")); err == nil {
		t.Errorf("security violation: evil.txt was extracted outside target directory!")
	}
}
