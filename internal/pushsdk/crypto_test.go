package pushsdk

import (
	"testing"

	"github.com/itplus/pushsdk-gateway/internal/config"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	terminal := config.Terminal{Username: "gateway", Password: "correct-horse", Security: 4}
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

func TestParseEncryptionParametersRequiresCompleteNegotiatedParameters(t *testing.T) {
	if _, err := ParseEncryptionParameters("4", "", "0123456789abcdef", 4); err == nil {
		t.Fatal("expected incomplete parameters to fail")
	}
	if _, err := ParseEncryptionParameters("3", "00112233445566778899aabbccddeeff", "0123456789abcdef", 4); err == nil {
		t.Fatal("expected security downgrade to fail")
	}
}
