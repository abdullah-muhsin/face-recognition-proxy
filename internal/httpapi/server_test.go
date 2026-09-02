package httpapi

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
	request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/events?source=isapi", nil)
	query, err = deviceEventQuery(request, 25, 0)
	if err != nil || query.Source != "isapi" {
		t.Fatalf("source query = %#v, %v", query, err)
	}
}

func TestParseISAPICommandInputPreservesTextBytes(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/terminals/DS-K1/isapi-commands", strings.NewReader(`{
        "method":"PUT",
        "url":"/ISAPI/AccessControl/remoteCheck?format=json",
        "dataFormat":"jsonData",
        "textData":"{\n\t\"RemoteCheck\": true\n}",
        "expiresInSeconds":60
    }`))
	request.Header.Set("Content-Type", "application/json")
	parsed, err := parseISAPICommandInput(request)
	if err != nil {
		t.Fatalf("parseISAPICommandInput() error = %v", err)
	}
	if parsed.Method != "PUT" || parsed.URL != "/ISAPI/AccessControl/remoteCheck?format=json" || parsed.DataFormat != "jsonData" || parsed.ExpiresInSeconds != 60 {
		t.Fatalf("parsed metadata = %#v", parsed)
	}
	if !bytes.Equal(parsed.Data, []byte("{\n\t\"RemoteCheck\": true\n}")) {
		t.Fatalf("payload bytes = %q", parsed.Data)
	}
}

func TestParseISAPICommandInputRepresentsNoDataAsZeroBytes(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{
        "method":"GET",
        "url":"/ISAPI/System/deviceInfo",
        "dataFormat":"noData",
        "expiresInSeconds":60
    }`))
	request.Header.Set("Content-Type", "application/json")
	parsed, err := parseISAPICommandInput(request)
	if err != nil {
		t.Fatalf("parseISAPICommandInput() error = %v", err)
	}
	if parsed.Data == nil || len(parsed.Data) != 0 {
		t.Fatalf("noData bytes = %#v, want an explicit empty byte slice", parsed.Data)
	}
}

func TestParseISAPICommandInputAllowsEveryVendorMethodAndRequestFormat(t *testing.T) {
	formats := []struct {
		name       string
		dataFormat string
		field      string
		data       []byte
	}{
		{name: "no data", dataFormat: "noData", data: []byte{}},
		{name: "json", dataFormat: "jsonData", field: `,"textData":"{\"User\":true}"`, data: []byte(`{"User":true}`)},
		{name: "xml", dataFormat: "xmlData", field: `,"textData":"<User/>"`, data: []byte("<User/>")},
		{name: "boundary", dataFormat: "boundaryData", field: `,"dataBase64":"AAE="`, data: []byte{0x00, 0x01}},
	}
	for _, method := range []string{"GET", "POST", "PUT", "DELETE"} {
		for _, format := range formats {
			t.Run(method+"/"+format.name, func(t *testing.T) {
				body := fmt.Sprintf(`{"method":%q,"url":"/ISAPI/AccessControl/UserInfo/Record?format=json","dataFormat":%q%s,"expiresInSeconds":60}`,
					method, format.dataFormat, format.field)
				request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
				request.Header.Set("Content-Type", "application/json")
				parsed, err := parseISAPICommandInput(request)
				if err != nil {
					t.Fatalf("parseISAPICommandInput() error = %v", err)
				}
				if parsed.Method != method || parsed.DataFormat != format.dataFormat {
					t.Fatalf("parsed metadata = %#v", parsed)
				}
				if !bytes.Equal(parsed.Data, format.data) {
					t.Fatalf("payload bytes = %v, want %v", parsed.Data, format.data)
				}
			})
		}
	}
}

func TestParseISAPICommandInputRejectsAmbiguity(t *testing.T) {
	for _, body := range []string{
		`{"method":"get","url":"/ISAPI/System/deviceInfo","dataFormat":"noData","expiresInSeconds":60}`,
		`{"method":"GET","url":"https://example.invalid/ISAPI/System/deviceInfo","dataFormat":"noData","expiresInSeconds":60}`,
		`{"method":"GET","url":"/ISAPI/System/../deviceInfo","dataFormat":"noData","expiresInSeconds":60}`,
		`{"method":"GET","url":"/ISAPI/System/deviceInfo","dataFormat":"noData","textData":"{}","expiresInSeconds":60}`,
		`{"method":"PUT","url":"/ISAPI/System/deviceInfo","dataFormat":"boundaryData","dataBase64":"eA==\n","expiresInSeconds":60}`,
		`{"method":"GET","url":"/ISAPI/System/deviceInfo","dataFormat":"noData"}`,
	} {
		t.Run(body, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			if _, err := parseISAPICommandInput(request); err == nil {
				t.Fatalf("parseISAPICommandInput() accepted %s", body)
			}
		})
	}
}

func TestISAPICommandFilterRequiresOneKnownParameter(t *testing.T) {
	for _, rawQuery := range []string{
		"terminal=DS-K1",
		"terminal=DS-K1&terminal=DS-K2",
		"terminal=",
		"terminal=DS-K1&unknown=value",
	} {
		t.Run(rawQuery, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/isapi-commands?"+rawQuery, nil)
			terminal, err := isapiCommandTerminalFilter(request)
			if rawQuery == "terminal=DS-K1" {
				if err != nil || terminal != "DS-K1" {
					t.Fatalf("filter = %q, %v", terminal, err)
				}
				return
			}
			if err == nil {
				t.Fatal("isapiCommandTerminalFilter() accepted invalid query")
			}
		})
	}
}

func TestDeviceEventQueryRejectsAmbiguousAndInvalidFilters(t *testing.T) {
	for _, rawQuery := range []string{
		"category=all&subtype=75",
		"subtype=75",
		"category=event&category=alarm",
		"category=unknown",
		"category=event&subtype=-1",
		"source=unknown",
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
