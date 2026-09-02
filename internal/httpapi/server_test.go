package httpapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/itplus/pushsdk-gateway/internal/config"
)

func TestDeviceEventQueryUsesExplicitCategoryAndSubtypeCodes(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/events?category=event&subtype=75&terminal=DS-K1", nil)
	query, err := deviceEventQuery(request, 25, 0)
	if err != nil {
		t.Fatalf("deviceEventQuery() error = %v", err)
	}
	if query.MajorEventType == nil || *query.MajorEventType != 5 {
		t.Fatalf("major event type = %#v, want 5", query.MajorEventType)
	}
	if query.SubEventType == nil || *query.SubEventType != 75 {
		t.Fatalf("sub event type = %#v, want 75", query.SubEventType)
	}
	if query.Terminal != "DS-K1" {
		t.Fatalf("terminal = %q, want DS-K1", query.Terminal)
	}
}

func TestDeviceEventQueryRejectsAmbiguousAndInvalidFilters(t *testing.T) {
	for _, rawQuery := range []string{
		"category=all&subtype=75",
		"subtype=75",
		"category=event&category=alarm",
		"category=unknown",
		"category=event&subtype=-1",
		"category=event&extra=value",
	} {
		t.Run(rawQuery, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/events?"+rawQuery, nil)
			if _, err := deviceEventQuery(request, 25, 0); err == nil {
				t.Fatal("deviceEventQuery() accepted invalid filters")
			}
		})
	}
}

func TestWebAppServesVueEntryForHistoryRoutes(t *testing.T) {
	webDir := t.TempDir()
	writeWebFile(t, webDir, "index.html", "administration console")
	server := &Server{config: config.Config{WebDir: webDir}}

	for _, route := range []string{"/app/", "/app/board", "/app/events"} {
		response := httptest.NewRecorder()
		server.webApp(response, httptest.NewRequest(http.MethodGet, route, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d", route, response.Code, http.StatusOK)
		}
		if body := response.Body.String(); body != "administration console" {
			t.Fatalf("%s body = %q, want Vue entry document", route, body)
		}
	}
}

func TestWebAppServesAssetsAndDoesNotMaskMissingAssets(t *testing.T) {
	webDir := t.TempDir()
	writeWebFile(t, webDir, "index.html", "administration console")
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
