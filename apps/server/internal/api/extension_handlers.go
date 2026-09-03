package api

import (
	"archive/zip"
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"kpi-schedule-bot/server/internal/model"
)

var (
	dynamicZipMu    sync.RWMutex
	cachedZipBytes  []byte
	cachedZipMod    time.Time
)

// getExtensionDownload serves the packaged browser extension zip file.
// It checks explicit configuration, standard disk locations, or zips the dist directory on the fly.
func (h *handlers) getExtensionDownload(w http.ResponseWriter, r *http.Request) {
	var candidates []string
	if h.extensionZipPath != "" {
		candidates = append(candidates, h.extensionZipPath)
	}
	if envPath := os.Getenv("EXTENSION_ZIP_PATH"); envPath != "" {
		candidates = append(candidates, envPath)
	}
	candidates = append(candidates,
		"kpi-schedule-sync.zip",
		"../extension/dist/kpi-schedule-sync.zip",
		"../../apps/extension/dist/kpi-schedule-sync.zip",
		"apps/extension/dist/kpi-schedule-sync.zip",
		"data/kpi-schedule-sync.zip",
		"/data/kpi-schedule-sync.zip",
	)

	for _, path := range candidates {
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
			w.Header().Set("Content-Type", "application/zip")
			w.Header().Set("Content-Disposition", `attachment; filename="kpi-schedule-sync.zip"`)
			http.ServeFile(w, r, path)
			return
		}
	}

	// Fallback: check if an unpacked dist directory exists and zip it dynamically
	distDirs := []string{
		"../extension/dist",
		"../../apps/extension/dist",
		"apps/extension/dist",
	}
	for _, dir := range distDirs {
		manifestPath := filepath.Join(dir, "manifest.json")
		if fi, err := os.Stat(manifestPath); err == nil {
			zipData, modTime, err := getOrBuildZip(dir, fi.ModTime())
			if err != nil {
				slog.Error("building dynamic extension zip", "error", err)
				model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, "failed to package extension")
				return
			}
			w.Header().Set("Content-Type", "application/zip")
			w.Header().Set("Content-Disposition", `attachment; filename="kpi-schedule-sync.zip"`)
			http.ServeContent(w, r, "kpi-schedule-sync.zip", modTime, bytes.NewReader(zipData))
			return
		}
	}

	model.WriteError(w, http.StatusNotFound, model.ErrInvalidRequest, "extension package not found")
}

func getOrBuildZip(sourceDir string, modTime time.Time) ([]byte, time.Time, error) {
	dynamicZipMu.RLock()
	if cachedZipBytes != nil && !cachedZipMod.Before(modTime) {
		defer dynamicZipMu.RUnlock()
		return cachedZipBytes, cachedZipMod, nil
	}
	dynamicZipMu.RUnlock()

	dynamicZipMu.Lock()
	defer dynamicZipMu.Unlock()

	if cachedZipBytes != nil && !cachedZipMod.Before(modTime) {
		return cachedZipBytes, cachedZipMod, nil
	}

	data, err := zipDirectory(sourceDir)
	if err != nil {
		return nil, time.Time{}, err
	}
	cachedZipBytes = data
	cachedZipMod = modTime
	return data, modTime, nil
}

func zipDirectory(sourceDir string) ([]byte, error) {
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if strings.HasSuffix(rel, ".zip") {
			return nil
		}

		w, err := zw.Create(rel)
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(w, f)
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
