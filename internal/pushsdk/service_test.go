package pushsdk

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/itplus/pushsdk-gateway/internal/config"
	"github.com/itplus/pushsdk-gateway/internal/store"
)

func TestAuthInfoEncryptionModesAreDistinct(t *testing.T) {
	terminal := config.Terminal{SecurityVersion: 4}

	mode, err := payloadModeForAuthInfo(terminal, nil)
	if err != nil {
		t.Fatalf("empty AuthInfo: %v", err)
	}
	if mode != PlaintextPayload {
		t.Fatal("empty AuthInfo must select plaintext mode")
	}

	mode, err = payloadModeForAuthInfo(terminal, []byte(`{"data":{"securityVersion":[3,4]}}`))
	if err != nil {
		t.Fatalf("negotiated AuthInfo: %v", err)
	}
	if mode != EncryptedPayload {
		t.Fatal("negotiated AuthInfo must select encrypted mode")
	}

	if _, err := payloadModeForAuthInfo(terminal, []byte(`{"data":{"securityVersion":[3]}}`)); err == nil {
		t.Fatal("missing configured security version must be rejected")
	}
}

func TestInvalidSessionResponseUsesVendorRecoveryEnvelope(t *testing.T) {
	response := httptest.NewRecorder()
	(&Service{}).respondErrorWithCode(response, http.StatusUnauthorized, invalidSessionCode, invalidSessionMessage)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("response status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	var payload successResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Code != invalidSessionCode || payload.ErrorMsg != invalidSessionMessage {
		t.Fatalf("response = %#v, want invalid-session recovery envelope", payload)
	}
}

func TestPlaintextSessionRejectsEncryptionQueryParameters(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "http://gateway.example/iot/DEVICE/global/0-global/model/service/operate/PUSH/Login?security=4&iv=00112233445566778899aabbccddeeff&random=0123456789abcdef", nil)
	session := &Session{PayloadMode: PlaintextPayload}
	if _, _, err := payloadForSession(request, config.Terminal{SecurityVersion: 4}, session, "Login", []byte(`{"data":{}}`), true); err == nil {
		t.Fatal("plaintext session accepted encryption query parameters")
	}
}

func TestAuthenticatedRequestUsesCustomAuthNotChallenge(t *testing.T) {
	terminal := config.Terminal{Username: "gateway", Password: "correct-horse", LoginPasswordDigest: "sha256", SecurityVersion: 4}
	session := &Session{Salt: "salt", NextChallenge: "next-challenge", PayloadMode: PlaintextPayload}
	request := httptest.NewRequest(http.MethodPost, "http://gateway.example/iot/DEVICE/global/0-global/model/service/operate/PUSH/CommandRequest", nil)
	request.Header.Set("My-Custom-Auth", expectedCustomAuth(terminal, session.Salt, session.NextChallenge))

	if !constantTimeEqual(request.Header.Get("My-Custom-Auth"), expectedCustomAuth(terminal, session.Salt, session.NextChallenge)) {
		t.Fatal("valid custom-auth header did not validate")
	}
	if constantTimeEqual(request.Header.Get("My-Custom-Auth"), session.NextChallenge) {
		t.Fatal("custom-auth must not be compared to the raw challenge")
	}
}

func TestActionResponseBodiesUseDocumentedTopLevelSchemas(t *testing.T) {
	tests := []struct {
		name     string
		body     any
		wantKeys []string
		forbid   string
	}{
		{
			name:     "command request",
			body:     commandRequestResponse{successResponse: succeeded(), CommandNum: 0, CommandList: []commandRequestItem{}},
			wantKeys: []string{"status", "code", "errorMsg", "commandNum", "commandList"},
			forbid:   "data",
		},
		{
			name:     "command result",
			body:     commandResultResponse{successResponse: succeeded(), IsPendingCommand: false},
			wantKeys: []string{"status", "code", "errorMsg", "isPendingCommand"},
			forbid:   "data",
		},
		{
			name: "event",
			body: []eventResponseItem{{UUID: "event-1", successResponse: succeeded()}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := json.Marshal(test.body)
			if err != nil {
				t.Fatalf("marshal response: %v", err)
			}
			if test.name == "event" {
				var values []map[string]any
				if err := json.Unmarshal(encoded, &values); err != nil || len(values) != 1 {
					t.Fatalf("event response must be a top-level array: %s", encoded)
				}
				return
			}
			var value map[string]any
			if err := json.Unmarshal(encoded, &value); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			for _, key := range test.wantKeys {
				if _, found := value[key]; !found {
					t.Fatalf("response lacks %q: %s", key, encoded)
				}
			}
			if _, found := value[test.forbid]; found {
				t.Fatalf("response contains forbidden %q wrapper: %s", test.forbid, encoded)
			}
		})
	}
}

func TestParseCommandResultBatchRequiresDocumentedTopLevelFields(t *testing.T) {
	if _, err := ParseCommandResultBatch([]byte(`{"commandNum":0,"commandList":[]}`)); err != nil {
		t.Fatalf("valid no-command result: %v", err)
	}
	if _, err := ParseCommandResultBatch([]byte(`{"data":{"commandNum":0,"commandList":[]}}`)); err == nil {
		t.Fatal("unexpected data wrapper must be rejected")
	}
}

func TestParseCommandResultBatchPreservesDeclaredFormatAndBase64(t *testing.T) {
	const uuid = "1a2b3c4d-5e6f-4789-8abc-def012345678"
	results, err := ParseCommandResultBatch([]byte(`{"commandNum":1,"commandList":[{"UUID":"` + uuid + `","dataFormat":"jsonData","data":"eyJvayI6dHJ1ZX0="}]}`))
	if err != nil {
		t.Fatalf("ParseCommandResultBatch() error = %v", err)
	}
	if len(results) != 1 || results[0].UUID != uuid || results[0].DataFormat == nil || *results[0].DataFormat != "jsonData" || results[0].DataBase64 != "eyJvayI6dHJ1ZX0=" {
		t.Fatalf("results = %#v", results)
	}
}

func TestCommandRequestItemForPreservesEveryVendorMethodAndRequestFormat(t *testing.T) {
	for _, delivery := range []store.ISAPICommandDelivery{
		{UUID: "00000000-0000-4000-8000-000000000001", Method: "GET", URL: "/ISAPI/System/deviceInfo", DataFormat: "noData", Data: []byte{}},
		{UUID: "00000000-0000-4000-8000-000000000002", Method: "POST", URL: "/ISAPI/AccessControl/UserInfo/Record?format=json", DataFormat: "jsonData", Data: []byte(`{"User":true}`)},
		{UUID: "00000000-0000-4000-8000-000000000003", Method: "PUT", URL: "/ISAPI/AccessControl/remoteCheck?format=json", DataFormat: "xmlData", Data: []byte("<RemoteCheck/>")},
		{UUID: "00000000-0000-4000-8000-000000000004", Method: "DELETE", URL: "/ISAPI/AccessControl/UserInfo/Record?format=json", DataFormat: "boundaryData", Data: []byte{0x00, 0x01}},
	} {
		t.Run(delivery.Method+"/"+delivery.DataFormat, func(t *testing.T) {
			item := commandRequestItemFor(delivery)
			if item.UUID != delivery.UUID || item.URL != delivery.Method+" "+delivery.URL || item.DataFormat != delivery.DataFormat {
				t.Fatalf("wire item = %#v", item)
			}
			if want := base64.StdEncoding.EncodeToString(delivery.Data); item.Data != want {
				t.Fatalf("wire data = %q, want %q", item.Data, want)
			}
		})
	}
}

func TestParseCommandResultBatchPreservesEveryDeclaredResponseFormat(t *testing.T) {
	for _, test := range []struct {
		format string
		data   string
	}{
		{format: "jsonData", data: "eyJvayI6dHJ1ZX0="},
		{format: "xmlData", data: "PFJlc3BvbnNlLz4="},
		{format: "boundaryData", data: "AAE="},
	} {
		t.Run(test.format, func(t *testing.T) {
			const uuid = "1a2b3c4d-5e6f-4789-8abc-def012345678"
			body := []byte(`{"commandNum":1,"commandList":[{"UUID":"` + uuid + `","dataFormat":"` + test.format + `","data":"` + test.data + `"}]}`)
			results, err := ParseCommandResultBatch(body)
			if err != nil {
				t.Fatalf("ParseCommandResultBatch() error = %v", err)
			}
			if len(results) != 1 || results[0].DataFormat == nil || *results[0].DataFormat != test.format || results[0].DataBase64 != test.data {
				t.Fatalf("results = %#v", results)
			}
		})
	}
}

func TestParseCommandResultBatchPreservesUndeclaredResultFormat(t *testing.T) {
	const uuid = "1a2b3c4d-5e6f-4789-8abc-def012345678"
	results, err := ParseCommandResultBatch([]byte(`{"commandNum":1,"commandList":[{"UUID":"` + uuid + `","data":"eyJvayI6dHJ1ZX0="}]}`))
	if err != nil {
		t.Fatalf("ParseCommandResultBatch() error = %v", err)
	}
	if len(results) != 1 || results[0].UUID != uuid || results[0].DataFormat != nil || results[0].DataBase64 != "eyJvayI6dHJ1ZX0=" {
		t.Fatalf("results = %#v", results)
	}
}

func TestParseCommandResultBatchRejectsMissingOrAmbiguousDataFormat(t *testing.T) {
	const uuid = "1a2b3c4d-5e6f-4789-8abc-def012345678"
	for _, payload := range []string{
		`{"commandNum":1,"commandList":[{"UUID":"` + uuid + `","dataFormat":null,"data":""}]}`,
		`{"commandNum":1,"commandList":[{"UUID":"` + uuid + `","dataFormat":"noData","data":"eA=="}]}`,
		`{"commandNum":1,"commandList":[{"UUID":"` + uuid + `","dataFormat":"jsonData","data":""}]}`,
	} {
		if _, err := ParseCommandResultBatch([]byte(payload)); err == nil {
			t.Fatalf("ParseCommandResultBatch() accepted %s", payload)
		}
	}
}
