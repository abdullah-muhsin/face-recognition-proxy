package pushsdk

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"regexp"

	"github.com/itplus/pushsdk-gateway/internal/config"
	"golang.org/x/crypto/pbkdf2"
)

var hex32 = regexp.MustCompile(`^[0-9a-fA-F]{32}$`)

func randomAlphaNumeric(length int) (string, error) {
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	value := make([]byte, length)
	limit := 256 - (256 % len(alphabet))
	for index := 0; index < length; {
		var input [32]byte
		if _, err := io.ReadFull(rand.Reader, input[:]); err != nil {
			return "", err
		}
		for _, random := range input {
			if int(random) >= limit {
				continue
			}
			value[index] = alphabet[int(random)%len(alphabet)]
			index++
			if index == length {
				break
			}
		}
	}
	return string(value), nil
}

func passwordHash(terminal config.Terminal, salt string) string {
	sum := sha256.Sum256([]byte(terminal.Username + salt + terminal.Password))
	return hex.EncodeToString(sum[:])
}

func expectedLoginPassword(terminal config.Terminal, salt, challenge string, iterations int) string {
	value := pbkdf2.Key([]byte(passwordHash(terminal, salt)+challenge), []byte(salt), iterations, 64, loginPasswordHash(terminal.LoginPasswordDigest))
	return hex.EncodeToString(value)
}

// loginPasswordHash selects the one digest explicitly configured for this
// terminal. Config validation rejects every other value, so this never guesses
// a different digest after an authentication failure.
func loginPasswordHash(digest string) func() hash.Hash {
	switch digest {
	case "sha1":
		return sha1.New
	case "sha256":
		return sha256.New
	default:
		panic("terminal login password digest was not validated")
	}
}

func expectedCustomAuth(terminal config.Terminal, salt, challenge string) string {
	sum := sha256.Sum256([]byte(passwordHash(terminal, salt) + challenge))
	return hex.EncodeToString(sum[:])
}

type EncryptionParameters struct {
	SecurityVersion int
	IV              []byte
	Random          string
}

func ParseEncryptionParameters(security, iv, random string, expectedSecurity int) (EncryptionParameters, error) {
	if security == "" || iv == "" || random == "" {
		return EncryptionParameters{}, fmt.Errorf("security, iv, and random must all be present")
	}
	if security != fmt.Sprintf("%d", expectedSecurity) {
		return EncryptionParameters{}, fmt.Errorf("security version is not negotiated")
	}
	if !hex32.MatchString(iv) {
		return EncryptionParameters{}, fmt.Errorf("iv must be exactly 32 hexadecimal characters")
	}
	if len([]byte(random)) != 16 {
		return EncryptionParameters{}, fmt.Errorf("random must be exactly 16 UTF-8 bytes")
	}
	decoded, err := hex.DecodeString(iv)
	if err != nil {
		return EncryptionParameters{}, fmt.Errorf("decode iv: %w", err)
	}
	return EncryptionParameters{SecurityVersion: expectedSecurity, IV: decoded, Random: random}, nil
}

func deriveKey(terminal config.Terminal, salt string, iterations int, params EncryptionParameters) []byte {
	keyLength := 16
	if params.SecurityVersion == 4 {
		keyLength = 32
	}
	return pbkdf2.Key([]byte(passwordHash(terminal, salt)+params.Random), []byte(salt), iterations, keyLength, sha256.New)
}

func decrypt(terminal config.Terminal, salt string, iterations int, params EncryptionParameters, encrypted []byte) ([]byte, error) {
	encoded, err := base64.StdEncoding.DecodeString(string(encrypted))
	if err != nil {
		return nil, fmt.Errorf("request body must be base64 ciphertext: %w", err)
	}
	block, err := aes.NewCipher(deriveKey(terminal, salt, iterations, params))
	if err != nil {
		return nil, err
	}
	if len(encoded) == 0 || len(encoded)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext has invalid AES-CBC length")
	}
	plain := make([]byte, len(encoded))
	cipher.NewCBCDecrypter(block, params.IV).CryptBlocks(plain, encoded)
	return unpadPKCS7(plain)
}

func encrypt(terminal config.Terminal, salt string, iterations int, params EncryptionParameters, plain []byte) ([]byte, error) {
	block, err := aes.NewCipher(deriveKey(terminal, salt, iterations, params))
	if err != nil {
		return nil, err
	}
	padded := padPKCS7(plain, aes.BlockSize)
	ciphertext := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, params.IV).CryptBlocks(ciphertext, padded)
	encoded := base64.StdEncoding.EncodeToString(ciphertext)
	return []byte(encoded), nil
}

func padPKCS7(value []byte, size int) []byte {
	padding := size - len(value)%size
	result := make([]byte, len(value)+padding)
	copy(result, value)
	for i := len(value); i < len(result); i++ {
		result[i] = byte(padding)
	}
	return result
}

func unpadPKCS7(value []byte) ([]byte, error) {
	if len(value) == 0 || len(value)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("invalid padded plaintext length")
	}
	padding := int(value[len(value)-1])
	if padding < 1 || padding > aes.BlockSize || padding > len(value) {
		return nil, fmt.Errorf("invalid PKCS#7 padding")
	}
	for _, item := range value[len(value)-padding:] {
		if int(item) != padding {
			return nil, fmt.Errorf("invalid PKCS#7 padding")
		}
	}
	return value[:len(value)-padding], nil
}

func constantTimeEqual(left, right string) bool {
	return hmac.Equal([]byte(left), []byte(right))
}

// randomCustomChallenge creates the exact lower-case hexadecimal form required
// for the My-Custom-Challenge response header.
func randomCustomChallenge() (string, error) {
	value := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}
