package pushsdk

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/itplus/pushsdk-gateway/internal/config"
)

func TestAuthInfoEncryptionModesAreDistinct(t *testing.T) {
	terminal := config.Terminal{Security: 4}

	encrypted, err := authInfoEncryption(terminal, nil)
	if err != nil {
		t.Fatalf("empty AuthInfo: %v", err)
	}
	if encrypted {
		t.Fatal("empty AuthInfo must select plaintext mode")
	}

	encrypted, err = authInfoEncryption(terminal, []byte(`{"data":{"securityVersion":[3,4]}}`))
	if err != nil {
		t.Fatalf("negotiated AuthInfo: %v", err)
	}
	if !encrypted {
		t.Fatal("negotiated AuthInfo must select encrypted mode")
	}

	if _, err := authInfoEncryption(terminal, []byte(`{"data":{"securityVersion":[3]}}`)); err == nil {
		t.Fatal("missing configured security version must be rejected")
	}
}

func TestPlaintextSessionRejectsEncryptionQueryParameters(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "http://gateway.example/iot/DEVICE/global/0-global/model/service/operate/PUSH/Login?security=4&iv=00112233445566778899aabbccddeeff&random=0123456789abcdef", nil)
	session := &Session{}
	if _, _, err := payloadForSession(request, config.Terminal{Security: 4}, session, "Login", []byte(`{"data":{}}`), true); err == nil {
		t.Fatal("plaintext session accepted encryption query parameters")
	}
}

func TestAuthenticatedRequestUsesCustomAuthNotChallenge(t *testing.T) {
	terminal := config.Terminal{Username: "gateway", Password: "correct-horse", Security: 4}
	session := &Session{Salt: "salt", NextChallenge: "next-challenge", Encrypted: false}
	request := httptest.NewRequest(http.MethodPost, "http://gateway.example/iot/DEVICE/global/0-global/model/service/operate/PUSH/CommandRequest", nil)
	request.Header.Set("My-Custom-Auth", expectedCustomAuth(terminal, session.Salt, session.NextChallenge))

	if !constantTimeEqual(request.Header.Get("My-Custom-Auth"), expectedCustomAuth(terminal, session.Salt, session.NextChallenge)) {
		t.Fatal("valid custom-auth header did not validate")
	}
	if constantTimeEqual(request.Header.Get("My-Custom-Auth"), session.NextChallenge) {
		t.Fatal("custom-auth must not be compared to the raw challenge")
	}
}
