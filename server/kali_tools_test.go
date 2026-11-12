package server

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"antifox/db"
)

func TestKaliToolCatalogIntegrity(t *testing.T) {
	cat := KaliToolCatalog()
	if len(cat) < 15 {
		t.Fatalf("expected at least 15 tools in KaliToolCatalog, got %d", len(cat))
	}
	seen := map[string]bool{}
	for _, sp := range cat {
		if sp.Key == "" || sp.Binary == "" || sp.Category == "" || sp.Description == "" || sp.InstallMethod == "" {
			t.Fatalf("incomplete KaliToolSpec: %+v", sp)
		}
		if !reToolKey.MatchString(sp.Key) {
			t.Fatalf("invalid tool key %q", sp.Key)
		}
		if seen[sp.Key] {
			t.Fatalf("duplicate tool key %q", sp.Key)
		}
		seen[sp.Key] = true
		if containsCJK(sp.Description) {
			t.Fatalf("tool %q description contains CJK characters: %q", sp.Key, sp.Description)
		}
		got, ok := LookupKaliSpec(sp.Key)
		if !ok || got.Key != sp.Key {
			t.Fatalf("LookupKaliSpec(%q) failed", sp.Key)
		}
	}
	// Check hyphenated binary lookup (interactsh-client -> interactsh_client).
	if sp, ok := LookupKaliSpec("interactsh-client"); !ok || sp.Key != "interactsh_client" {
		t.Fatalf("LookupKaliSpec(interactsh-client) = %+v, %v", sp, ok)
	}
}

func TestEnsureKaliBinDirAndFormatShellHint(t *testing.T) {
	tmp := t.TempDir()
	binDir := ensureKaliBinDir(tmp)

	cli := filepath.Join(binDir, "antifox-tool")
	st, err := os.Stat(cli)
	if err != nil {
		t.Fatalf("antifox-tool script not created: %v", err)
	}
	if st.Mode()&0o111 == 0 {
		t.Fatalf("antifox-tool is not executable: mode=%v", st.Mode())
	}

	// Create a fake binary in binDir and verify formatShellHint detects it as [installed].
	fakeBin := filepath.Join(binDir, "fake_kali_scanner")
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\necho v1.0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	execRaw, _ := json.Marshal(map[string]string{"binary": "fake_kali_scanner"})
	installedHint := formatShellHint(&db.Tool{
		Key:         "fake_kali_scanner",
		Description: "Fake scanner for unit test",
		Kind:        "shell",
		Exec:        execRaw,
	})
	if !strings.Contains(installedHint, "[installed]") {
		t.Fatalf("expected [installed] in hint, got %q", installedHint)
	}

	missingHint := formatShellHint(&db.Tool{
		Key:         "nonexistent_kali_xyz",
		Description: "Missing scanner",
		Kind:        "shell",
	})
	if !strings.Contains(missingHint, "[not installed") || !strings.Contains(missingHint, "ensure_kali_tool") {
		t.Fatalf("expected [not installed ... ensure_kali_tool] in hint, got %q", missingHint)
	}
}

func TestSelectReleaseAssetAndArchiveExtraction(t *testing.T) {
	assets := []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	}{
		{Name: "httpx_1.6.0_checksums.txt", BrowserDownloadURL: "https://example.com/checksums.txt"},
		{Name: "httpx_1.6.0_linux_amd64.deb", BrowserDownloadURL: "https://example.com/httpx.deb"},
		{Name: "httpx_1.6.0_linux_amd64.zip", BrowserDownloadURL: "https://example.com/httpx_linux_amd64.zip"},
		{Name: "httpx_1.6.0_linux_arm64.zip", BrowserDownloadURL: "https://example.com/httpx_linux_arm64.zip"},
		{Name: "httpx_1.6.0_macOS_arm64.zip", BrowserDownloadURL: "https://example.com/httpx_macos_arm64.zip"},
	}

	name, url := selectReleaseAsset(assets, "linux", "amd64")
	if name != "httpx_1.6.0_linux_amd64.zip" || url != "https://example.com/httpx_linux_amd64.zip" {
		t.Fatalf("selectReleaseAsset(linux/amd64) = (%q, %q)", name, url)
	}

	nameArm, _ := selectReleaseAsset(assets, "darwin", "arm64")
	if nameArm != "httpx_1.6.0_macOS_arm64.zip" {
		t.Fatalf("selectReleaseAsset(darwin/arm64) = %q", nameArm)
	}

	// Test zip extraction
	var zbuf bytes.Buffer
	zw := zip.NewWriter(&zbuf)
	zf, err := zw.Create("nested/httpx")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = zf.Write([]byte("#!/bin/sh\necho httpx-zip-ok\n"))
	_ = zw.Close()

	dstZip := filepath.Join(t.TempDir(), "httpx")
	if err := extractBinaryFromZip(zbuf.Bytes(), "httpx", dstZip); err != nil {
		t.Fatalf("extractBinaryFromZip: %v", err)
	}
	gotZip, err := os.ReadFile(dstZip)
	if err != nil || !strings.Contains(string(gotZip), "httpx-zip-ok") {
		t.Fatalf("extracted zip content mismatch: %q (%v)", string(gotZip), err)
	}

	// Test tar.gz extraction
	var tbuf bytes.Buffer
	gzw := gzip.NewWriter(&tbuf)
	tw := tar.NewWriter(gzw)
	payload := []byte("#!/bin/sh\necho ffuf-tar-ok\n")
	if err := tw.WriteHeader(&tar.Header{
		Name:     "ffuf_2.1.0/ffuf",
		Mode:     0o755,
		Size:     int64(len(payload)),
		Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatal(err)
	}
	_, _ = tw.Write(payload)
	_ = tw.Close()
	_ = gzw.Close()

	dstTar := filepath.Join(t.TempDir(), "ffuf")
	if err := extractBinaryFromTarGz(tbuf.Bytes(), "ffuf", dstTar); err != nil {
		t.Fatalf("extractBinaryFromTarGz: %v", err)
	}
	gotTar, err := os.ReadFile(dstTar)
	if err != nil || !strings.Contains(string(gotTar), "ffuf-tar-ok") {
		t.Fatalf("extracted tar.gz content mismatch: %q (%v)", string(gotTar), err)
	}
}
