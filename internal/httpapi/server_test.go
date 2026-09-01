package httpapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/itplus/pushsdk-gateway/internal/config"
)

func TestWebAppServesVueEntryForHistoryRoutes(t *testing.T) {
	webDir := t.TempDir()
	writeWebFile(t, webDir, "index.html", "operator console")
	server := &Server{config: config.Config{WebDir: webDir}}

	for _, route := range []string{"/app/", "/app/overview", "/app/attendance"} {
		response := httptest.NewRecorder()
		server.webApp(response, httptest.NewRequest(http.MethodGet, route, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d", route, response.Code, http.StatusOK)
		}
		if body := response.Body.String(); body != "operator console" {
			t.Fatalf("%s body = %q, want Vue entry document", route, body)
		}
	}
}

func TestWebAppServesAssetsAndDoesNotMaskMissingAssets(t *testing.T) {
	webDir := t.TempDir()
	writeWebFile(t, webDir, "index.html", "operator console")
	writeWebFile(t, webDir, "assets/app.js", "console.log('gateway')")
	server := &Server{config: config.Config{WebDir: webDir}}

	assetResponse := httptest.NewRecorder()
	server.webApp(assetResponse, httptest.NewRequest(http.MethodGet, "/app/assets/app.js", nil))
	if assetResponse.Code != http.StatusOK {
		t.Fatalf("asset status = %d, want %d", assetResponse.Code, http.StatusOK)
	}
	if body := assetResponse.Body.String(); body != "console.log('gateway')" {
		t.Fatalf("asset body = %q, want static asset", body)
	}

	missingResponse := httptest.NewRecorder()
	server.webApp(missingResponse, httptest.NewRequest(http.MethodGet, "/app/assets/missing.js", nil))
	if missingResponse.Code != http.StatusNotFound {
		t.Fatalf("missing asset status = %d, want %d", missingResponse.Code, http.StatusNotFound)
	}
}

func writeWebFile(t *testing.T, root, name, contents string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create web asset directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write web asset: %v", err)
	}
}
