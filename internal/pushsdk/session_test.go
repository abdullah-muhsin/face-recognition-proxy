package pushsdk

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/itplus/pushsdk-gateway/internal/config"
)

func TestPersistentSessionStateIsCompleteAndDoesNotContainPassword(t *testing.T) {
	terminal := config.Terminal{
		SerialNumber:           "DEVICE-1",
		PushSDKSerial:          "PUSH-1",
		Username:               "gateway",
		Password:               "unambiguous-terminal-password",
		LoginPasswordDigest:    "sha256",
		SecurityVersion:        4,
		CommandIntervalSeconds: 5,
		ErrorDelaySeconds:      30,
	}
	nextChallengeAt := time.Date(2026, time.September, 2, 12, 0, 0, 0, time.UTC)
	session := &Session{
		Terminal:        terminal,
		PayloadMode:     EncryptedPayload,
		Salt:            strings.Repeat("S", 64),
		LoginChallenge:  strings.Repeat("L", 64),
		NextChallenge:   strings.Repeat("a", 64),
		Iterations:      keyDerivationIterations,
		CreatedAt:       nextChallengeAt.Add(-time.Second),
		NextChallengeAt: nextChallengeAt,
		Authenticated:   true,
	}

	persisted := session.PersistentState()
	if !validStoredSession(persisted) {
		t.Fatal("complete authenticated session was not valid for persistence")
	}
	if persisted.ConfigurationFingerprint != terminal.SessionFingerprint() {
		t.Fatal("persistent state is not bound to the terminal configuration")
	}
	encoded, err := json.Marshal(persisted)
	if err != nil {
		t.Fatalf("marshal persistent state: %v", err)
	}
	if strings.Contains(string(encoded), terminal.Password) {
		t.Fatal("persistent session state contains terminal password")
	}
}

func TestStoredSessionValidationRejectsMalformedState(t *testing.T) {
	terminal := config.Terminal{
		SerialNumber:           "DEVICE-1",
		PushSDKSerial:          "PUSH-1",
		Username:               "gateway",
		Password:               "terminal-password",
		LoginPasswordDigest:    "sha256",
		SecurityVersion:        4,
		CommandIntervalSeconds: 5,
		ErrorDelaySeconds:      30,
	}
	at := time.Now().UTC()
	session := (&Session{
		Terminal:        terminal,
		PayloadMode:     PlaintextPayload,
		Salt:            strings.Repeat("s", 64),
		LoginChallenge:  strings.Repeat("l", 64),
		NextChallenge:   strings.Repeat("a", 64),
		Iterations:      keyDerivationIterations,
		CreatedAt:       at,
		NextChallengeAt: at,
		Authenticated:   true,
	}).PersistentState()

	session.NextChallenge = "not-a-challenge"
	if validStoredSession(session) {
		t.Fatal("malformed next challenge was accepted")
	}
}

func TestChallengeExpiryUsesExactVendorDeadline(t *testing.T) {
	deadline := time.Date(2026, time.September, 2, 12, 0, 15, 0, time.UTC)
	session := &Session{Terminal: config.Terminal{CommandIntervalSeconds: 5}, CreatedAt: deadline.Add(-15 * time.Second), NextChallengeAt: deadline.Add(-15 * time.Second)}
	if !session.LoginChallengeExpired(deadline) {
		t.Fatal("login challenge remained valid at its deadline")
	}
	if !session.NextChallengeExpired(deadline) {
		t.Fatal("next challenge remained valid at its deadline")
	}
	if session.NextChallengeExpired(deadline.Add(-time.Nanosecond)) {
		t.Fatal("next challenge expired before its deadline")
	}
}
