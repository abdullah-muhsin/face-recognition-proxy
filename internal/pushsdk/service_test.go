package pushsdk

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/itplus/pushsdk-gateway/internal/config"
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
			body:     commandRequestResponse{successResponse: succeeded(), CommandNum: 0},
			wantKeys: []string{"status", "code", "errorMsg", "commandNum"},
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

func TestValidateEmptyCommandResultRequiresDocumentedTopLevelFields(t *testing.T) {
	if err := validateEmptyCommandResult([]byte(`{"commandNum":0,"commandList":[]}`)); err != nil {
		t.Fatalf("valid no-command result: %v", err)
	}
	if err := validateEmptyCommandResult([]byte(`{"data":{"commandNum":0,"commandList":[]}}`)); err == nil {
		t.Fatal("unexpected data wrapper must be rejected")
	}
}
