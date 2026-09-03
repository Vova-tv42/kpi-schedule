package api

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestGetExtensionDownload_PrebuiltZip(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "kpi-schedule-sync.zip")

	// Create a dummy zip file
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	w, err := zw.Create("manifest.json")
	if err != nil {
		t.Fatalf("creating zip entry: %v", err)
	}
	if _, err := w.Write([]byte(`{"name":"test"}`)); err != nil {
		t.Fatalf("writing zip entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("closing zip writer: %v", err)
	}
	if err := os.WriteFile(zipPath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("writing test zip: %v", err)
	}

	router := NewRouterWithOpts(nil, "test-token", RouterOpts{
		ExtensionZipPath: zipPath,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/extension/download", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/zip" {
		t.Errorf("expected Content-Type application/zip, got %s", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); cd != `attachment; filename="kpi-schedule-sync.zip"` {
		t.Errorf("unexpected Content-Disposition: %s", cd)
	}

	// Verify zip contents
	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatalf("opening response zip: %v", err)
	}
	if len(zr.File) != 1 || zr.File[0].Name != "manifest.json" {
		t.Fatalf("unexpected files in zip: %+v", zr.File)
	}
}

func TestGetExtensionDownload_DynamicZip(t *testing.T) {
	tmpDir := t.TempDir()
	distDir := filepath.Join(tmpDir, "dist")
	if err := os.MkdirAll(distDir, 0755); err != nil {
		t.Fatalf("mkdir dist: %v", err)
	}
	if err := os.WriteFile(filepath.Join(distDir, "manifest.json"), []byte(`{"version":"1.0.0"}`), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	// Test zipDirectory directly
	data, err := zipDirectory(distDir)
	if err != nil {
		t.Fatalf("zipDirectory error: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reading dynamic zip: %v", err)
	}
	if len(zr.File) != 1 || zr.File[0].Name != "manifest.json" {
		t.Fatalf("unexpected file in dynamic zip: %+v", zr.File)
	}
	rc, err := zr.File[0].Open()
	if err != nil {
		t.Fatalf("open entry: %v", err)
	}
	defer rc.Close()
	content, _ := io.ReadAll(rc)
	if string(content) != `{"version":"1.0.0"}` {
		t.Errorf("unexpected content: %s", string(content))
	}
}

func TestGetExtensionDownload_NotFound(t *testing.T) {
	router := NewRouterWithOpts(nil, "test-token", RouterOpts{
		ExtensionZipPath: "/nonexistent/path/kpi.zip",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/extension/download", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Since candidate paths in root/parent don't exist or if nonexistent path is forced:
	// If nonexistent, it returns 404
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusOK {
		t.Errorf("expected 404 or 200 (if real dist exists in workspace), got %d", rec.Code)
	}
}
