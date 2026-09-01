package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Config is deliberately explicit: the service will not invent defaults for
// credentials, terminals, or public security settings.
type Config struct {
	ListenAddress string
	DatabaseURL   string
	TerminalsFile string
	AdminUsername string
	AdminPassword string
	CookieSecure  bool
	SessionTTL    time.Duration
	WebDir        string
	MigrationsDir string
	MetricsCIDRs  []string
	Terminals     []Terminal
}

type Terminal struct {
	SerialNumber                string `json:"serialNumber"`
	PushSDKSerial               string `json:"pushSdkSerial"`
	Username                    string `json:"username"`
	PasswordEnvironmentVariable string `json:"passwordEnvironmentVariable"`
	Password                    string `json:"-"`
	Digest                      string `json:"digest"`
	Security                    int    `json:"securityVersion"`
	CommandSeconds              int    `json:"commandIntervalSeconds"`
	ErrorDelay                  int    `json:"errorDelaySeconds"`
}

func Load() (Config, error) {
	c := Config{
		ListenAddress: env("GATEWAY_LISTEN_ADDRESS"),
		DatabaseURL:   env("DATABASE_URL"),
		TerminalsFile: env("GATEWAY_TERMINALS_FILE"),
		AdminUsername: env("ADMIN_USERNAME"),
		AdminPassword: env("ADMIN_PASSWORD"),
		WebDir:        env("WEB_DIR"),
		MigrationsDir: env("MIGRATIONS_DIR"),
	}
	if raw := os.Getenv("COOKIE_SECURE"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("COOKIE_SECURE: %w", err)
		}
		c.CookieSecure = value
	}
	if raw := os.Getenv("SESSION_TTL"); raw != "" {
		value, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("SESSION_TTL: %w", err)
		}
		c.SessionTTL = value
	}
	if raw := os.Getenv("METRICS_ALLOW_CIDRS"); raw != "" {
		for _, value := range strings.Split(raw, ",") {
			value = strings.TrimSpace(value)
			if value != "" {
				c.MetricsCIDRs = append(c.MetricsCIDRs, value)
			}
		}
	}
	if err := c.loadTerminals(); err != nil {
		return Config{}, err
	}
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

func (c *Config) loadTerminals() error {
	contents, err := os.ReadFile(c.TerminalsFile)
	if err != nil {
		return fmt.Errorf("read GATEWAY_TERMINALS_FILE: %w", err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(contents)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&c.Terminals); err != nil {
		return fmt.Errorf("decode terminals: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("decode terminals: trailing JSON value")
	}
	for index := range c.Terminals {
		name := c.Terminals[index].PasswordEnvironmentVariable
		c.Terminals[index].Password = os.Getenv(name)
	}
	return nil
}

func (c Config) Validate() error {
	required := map[string]string{
		"GATEWAY_LISTEN_ADDRESS": c.ListenAddress,
		"DATABASE_URL":           c.DatabaseURL,
		"GATEWAY_TERMINALS_FILE": c.TerminalsFile,
		"ADMIN_USERNAME":         c.AdminUsername,
		"ADMIN_PASSWORD":         c.AdminPassword,
		"WEB_DIR":                c.WebDir,
		"MIGRATIONS_DIR":         c.MigrationsDir,
	}
	for name, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	if c.SessionTTL <= 0 {
		return fmt.Errorf("SESSION_TTL must be positive")
	}
	if len(c.MetricsCIDRs) == 0 {
		return fmt.Errorf("METRICS_ALLOW_CIDRS is required")
	}
	if len(c.Terminals) == 0 {
		return fmt.Errorf("at least one terminal is required")
	}
	seenDevice, seenPush := map[string]bool{}, map[string]bool{}
	for i, terminal := range c.Terminals {
		prefix := fmt.Sprintf("terminals[%d]", i)
		for field, value := range map[string]string{
			"serialNumber": terminal.SerialNumber, "pushSdkSerial": terminal.PushSDKSerial,
			"username": terminal.Username, "passwordEnvironmentVariable": terminal.PasswordEnvironmentVariable, "digest": terminal.Digest,
		} {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("%s.%s is required", prefix, field)
			}
		}
		if seenDevice[terminal.SerialNumber] || seenPush[terminal.PushSDKSerial] {
			return fmt.Errorf("%s has a duplicate serial", prefix)
		}
		seenDevice[terminal.SerialNumber], seenPush[terminal.PushSDKSerial] = true, true
		if terminal.Digest != "sha256" {
			return fmt.Errorf("%s.digest must be sha256", prefix)
		}
		if terminal.Security != 3 && terminal.Security != 4 {
			return fmt.Errorf("%s.securityVersion must be 3 or 4", prefix)
		}
		if terminal.CommandSeconds < 1 || terminal.CommandSeconds > 300 {
			return fmt.Errorf("%s.commandIntervalSeconds must be 1..300", prefix)
		}
		if terminal.ErrorDelay < 1 || terminal.ErrorDelay > 300 {
			return fmt.Errorf("%s.errorDelaySeconds must be 1..300", prefix)
		}
		if !environmentVariableName.MatchString(terminal.PasswordEnvironmentVariable) {
			return fmt.Errorf("%s.passwordEnvironmentVariable is invalid", prefix)
		}
		if len(terminal.Password) < 1 || len(terminal.Password) > 64 {
			return fmt.Errorf("environment variable %s for %s must contain 1..64 characters", terminal.PasswordEnvironmentVariable, prefix)
		}
	}
	return nil
}

func (c Config) TerminalByPushSDKSerial(serial string) (Terminal, bool) {
	for _, terminal := range c.Terminals {
		if terminal.PushSDKSerial == serial {
			return terminal, true
		}
	}
	return Terminal{}, false
}

// CredentialFingerprint lets operational logs identify a changed terminal
// credential without disclosing the username or password.
func (t Terminal) CredentialFingerprint() string {
	sum := sha256.Sum256([]byte(t.Username + "\x00" + t.Password))
	return hex.EncodeToString(sum[:8])
}

func env(name string) string { return strings.TrimSpace(os.Getenv(name)) }

var environmentVariableName = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
