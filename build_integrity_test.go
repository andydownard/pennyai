package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestBuildIntegrity_IndexReferencesExist parses static/index.html and verifies
// every JS/CSS asset it references actually exists in static/assets/.
// Catches the deployment failure where HTML references non-existent JS bundles.
func TestBuildIntegrity_IndexReferencesExist(t *testing.T) {
	indexPath := "static/index.html"
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		t.Skip("static/index.html not found (dev environment without build)")
	}

	data, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("Failed to read %s: %v", indexPath, err)
	}
	html := string(data)

	re := regexp.MustCompile(`(?:src|href)="(/assets/[^"]+)"`)
	matches := re.FindAllStringSubmatch(html, -1)
	if len(matches) == 0 {
		t.Fatal("No /assets/ references found in index.html — build may be broken")
	}

	for _, m := range matches {
		assetPath := m[1]
		filePath := "static" + assetPath
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("index.html references %s but file does not exist at %s", assetPath, filePath)
		}
	}
}

// TestBuildIntegrity_CriticalAssetsExist ensures the minimum viable set of
// static files are present for the app to function.
func TestBuildIntegrity_CriticalAssetsExist(t *testing.T) {
	// static/ is build output and not tracked in git. Without a build there is
	// nothing to check; the Dockerfile verifies the real build.
	if _, err := os.Stat("static/index.html"); os.IsNotExist(err) {
		t.Skip("static/ not built (run the frontend build to check it)")
	}
	required := []string{
		"static/index.html",
		"static/sw.js",
		"static/manifest.json",
	}
	for _, f := range required {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			t.Errorf("Critical static file missing: %s", f)
		}
	}

	entries, err := os.ReadDir("static/assets")
	if err != nil {
		t.Skip("static/assets/ not found (dev environment without build)")
	}
	hasJS := false
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".js") {
			hasJS = true
			break
		}
	}
	if !hasJS {
		t.Error("No .js files found in static/assets/ — frontend build output is missing")
	}
}
