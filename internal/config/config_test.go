package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRequiresExplicitSecuritySettingsAndCurrentTerminalSchema(t *testing.T) {
	directory := t.TempDir()
	terminalsFile := filepath.Join(directory, "terminals.json")
	if err := os.WriteFile(terminalsFile, []byte(`[
  {
    "serialNumber": "DEVICE-1",
    "pushSdkSerial": "PUSH-1",
    "username": "gateway",
    "passwordEnvironmentVariable": "PUSHSDK_TERMINAL_PASSWORD",
    "loginPasswordDigest": "sha256",
    "securityVersion": 4,
    "commandIntervalSeconds": 5,
    "errorDelaySeconds": 30
  }
]`), 0o600); err != nil {
		t.Fatal(err)
	}
	setRequiredEnvironment(t, terminalsFile)
	if _, err := Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	t.Setenv("COOKIE_SECURE", "")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "COOKIE_SECURE is required") {
		t.Fatalf("Load() error = %v, want missing COOKIE_SECURE", err)
	}
}

func TestLoadRejectsDeprecatedDigestFieldWithoutCompatibilityFallback(t *testing.T) {
	directory := t.TempDir()
	terminalsFile := filepath.Join(directory, "terminals.json")
	if err := os.WriteFile(terminalsFile, []byte(`[
  {
    "serialNumber": "DEVICE-1",
    "pushSdkSerial": "PUSH-1",
    "username": "gateway",
    "passwordEnvironmentVariable": "PUSHSDK_TERMINAL_PASSWORD",
    "digest": "sha256",
    "securityVersion": 4,
    "commandIntervalSeconds": 5,
    "errorDelaySeconds": 30
  }
]`), 0o600); err != nil {
		t.Fatal(err)
	}
	setRequiredEnvironment(t, terminalsFile)
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("Load() error = %v, want deprecated digest field rejection", err)
	}
}

func TestLoadPreservesConfiguredCredentialBytes(t *testing.T) {
	directory := t.TempDir()
	terminalsFile := filepath.Join(directory, "terminals.json")
	if err := os.WriteFile(terminalsFile, []byte(`[
  {
    "serialNumber": "DEVICE-1",
    "pushSdkSerial": "PUSH-1",
    "username": "gateway",
    "passwordEnvironmentVariable": "PUSHSDK_TERMINAL_PASSWORD",
    "loginPasswordDigest": "sha256",
    "securityVersion": 4,
    "commandIntervalSeconds": 5,
    "errorDelaySeconds": 30
  }
]`), 0o600); err != nil {
		t.Fatal(err)
	}
	setRequiredEnvironment(t, terminalsFile)
	t.Setenv("PUSHSDK_TERMINAL_PASSWORD", " exact-password ")
	configuration, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if configuration.Terminals[0].Password != " exact-password " {
		t.Fatalf("terminal password was normalized: %q", configuration.Terminals[0].Password)
	}
}

func TestSessionFingerprintChangesWithEveryPushSDKContractSetting(t *testing.T) {
	base := Terminal{
		SerialNumber:           "DEVICE-1",
		PushSDKSerial:          "PUSH-1",
		Username:               "gateway",
		Password:               "terminal-password",
		LoginPasswordDigest:    "sha256",
		SecurityVersion:        4,
		CommandIntervalSeconds: 5,
		ErrorDelaySeconds:      30,
	}
	baseline := base.SessionFingerprint()
	for name, mutate := range map[string]func(*Terminal){
		"push SDK serial":       func(terminal *Terminal) { terminal.PushSDKSerial = "PUSH-2" },
		"username":              func(terminal *Terminal) { terminal.Username = "other" },
		"password":              func(terminal *Terminal) { terminal.Password = "other-password" },
		"login password digest": func(terminal *Terminal) { terminal.LoginPasswordDigest = "sha1" },
		"security version":      func(terminal *Terminal) { terminal.SecurityVersion = 3 },
		"command interval":      func(terminal *Terminal) { terminal.CommandIntervalSeconds = 10 },
		"error delay":           func(terminal *Terminal) { terminal.ErrorDelaySeconds = 45 },
	} {
		t.Run(name, func(t *testing.T) {
			mutated := base
			mutate(&mutated)
			if mutated.SessionFingerprint() == baseline {
				t.Fatal("configuration change did not invalidate persisted session state")
			}
		})
	}
}

func setRequiredEnvironment(t *testing.T, terminalsFile string) {
	t.Helper()
	t.Setenv("GATEWAY_LISTEN_ADDRESS", ":8080")
	t.Setenv("DATABASE_URL", "postgres://pushsdk:password@postgres:5432/pushsdk?sslmode=disable")
	t.Setenv("GATEWAY_TERMINALS_FILE", terminalsFile)
	t.Setenv("ADMIN_USERNAME", "operator")
	t.Setenv("ADMIN_PASSWORD", "operator-password")
	t.Setenv("WEB_DIR", "/app/web")
	t.Setenv("MIGRATIONS_DIR", "/app/migrations")
	t.Setenv("COOKIE_SECURE", "true")
	t.Setenv("SESSION_TTL", "12h")
	t.Setenv("METRICS_ALLOW_CIDRS", "127.0.0.1/32")
	t.Setenv("PUSHSDK_TERMINAL_PASSWORD", "terminal-password")
}
