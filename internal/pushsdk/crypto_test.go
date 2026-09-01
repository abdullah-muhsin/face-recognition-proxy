package pushsdk

import (
	"regexp"
	"testing"

	"github.com/itplus/pushsdk-gateway/internal/config"
)

var hex64 = regexp.MustCompile(`^[0-9a-f]{64}$`)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	terminal := config.Terminal{Username: "gateway", Password: "correct-horse", LoginPasswordDigest: "sha256", SecurityVersion: 4}
	params, err := ParseEncryptionParameters("4", "00112233445566778899aabbccddeeff", "0123456789abcdef", 4)
	if err != nil {
		t.Fatalf("ParseEncryptionParameters() error = %v", err)
	}
	ciphertext, err := encrypt(terminal, "salt", 4096, params, []byte(`{"data":"value"}`))
	if err != nil {
		t.Fatalf("encrypt() error = %v", err)
	}
	plain, err := decrypt(terminal, "salt", 4096, params, ciphertext)
	if err != nil {
		t.Fatalf("decrypt() error = %v", err)
	}
	if string(plain) != `{"data":"value"}` {
		t.Fatalf("decrypt() = %q", plain)
	}
}

func TestLoginPasswordDigestUsesOnlyConfiguredAlgorithm(t *testing.T) {
	sha256Terminal := config.Terminal{Username: "gateway", Password: "correct-horse", LoginPasswordDigest: "sha256"}
	sha1Terminal := sha256Terminal
	sha1Terminal.LoginPasswordDigest = "sha1"
	sha256Value := expectedLoginPassword(sha256Terminal, "salt", "challenge", 4096)
	sha1Value := expectedLoginPassword(sha1Terminal, "salt", "challenge", 4096)
	if sha256Value == sha1Value {
		t.Fatal("configured SHA-1 and SHA-256 login digests must differ")
	}
}

func TestCustomChallengeIsLowerCaseHex(t *testing.T) {
	challenge, err := randomCustomChallenge()
	if err != nil {
		t.Fatalf("randomCustomChallenge: %v", err)
	}
	if len(challenge) != 64 || !hex64.MatchString(challenge) {
		t.Fatalf("challenge = %q, want 64 lower-case hexadecimal characters", challenge)
	}
}

func TestParseEncryptionParametersRequiresCompleteNegotiatedParameters(t *testing.T) {
	if _, err := ParseEncryptionParameters("4", "", "0123456789abcdef", 4); err == nil {
		t.Fatal("expected incomplete parameters to fail")
	}
	if _, err := ParseEncryptionParameters("3", "00112233445566778899aabbccddeeff", "0123456789abcdef", 4); err == nil {
		t.Fatal("expected security downgrade to fail")
	}
}
