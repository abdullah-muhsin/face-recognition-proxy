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
	LoginPasswordDigest         string `json:"loginPasswordDigest"`
	SecurityVersion             int    `json:"securityVersion"`
	CommandIntervalSeconds      int    `json:"commandIntervalSeconds"`
	ErrorDelaySeconds           int    `json:"errorDelaySeconds"`
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
	rawCookieSecure, present := os.LookupEnv("COOKIE_SECURE")
	if !present || rawCookieSecure == "" {
		return Config{}, fmt.Errorf("COOKIE_SECURE is required")
	}
	cookieSecure, err := strconv.ParseBool(rawCookieSecure)
	if err != nil {
		return Config{}, fmt.Errorf("COOKIE_SECURE: %w", err)
	}
	c.CookieSecure = cookieSecure
	rawSessionTTL, present := os.LookupEnv("SESSION_TTL")
	if !present || rawSessionTTL == "" {
		return Config{}, fmt.Errorf("SESSION_TTL is required")
	}
	sessionTTL, err := time.ParseDuration(rawSessionTTL)
	if err != nil {
		return Config{}, fmt.Errorf("SESSION_TTL: %w", err)
	}
	c.SessionTTL = sessionTTL
	rawMetricsCIDRs, present := os.LookupEnv("METRICS_ALLOW_CIDRS")
	if !present || rawMetricsCIDRs == "" {
		return Config{}, fmt.Errorf("METRICS_ALLOW_CIDRS is required")
	}
	for _, value := range strings.Split(rawMetricsCIDRs, ",") {
		if value == "" {
			return Config{}, fmt.Errorf("METRICS_ALLOW_CIDRS contains an empty entry")
		}
		c.MetricsCIDRs = append(c.MetricsCIDRs, value)
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
	required := []struct {
		name  string
		value string
	}{
		{name: "GATEWAY_LISTEN_ADDRESS", value: c.ListenAddress},
		{name: "DATABASE_URL", value: c.DatabaseURL},
		{name: "GATEWAY_TERMINALS_FILE", value: c.TerminalsFile},
		{name: "ADMIN_USERNAME", value: c.AdminUsername},
		{name: "ADMIN_PASSWORD", value: c.AdminPassword},
		{name: "WEB_DIR", value: c.WebDir},
		{name: "MIGRATIONS_DIR", value: c.MigrationsDir},
	}
	for _, setting := range required {
		if strings.TrimSpace(setting.value) == "" {
			return fmt.Errorf("%s is required", setting.name)
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
			"username": terminal.Username, "passwordEnvironmentVariable": terminal.PasswordEnvironmentVariable, "loginPasswordDigest": terminal.LoginPasswordDigest,
		} {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("%s.%s is required", prefix, field)
			}
		}
		if seenDevice[terminal.SerialNumber] {
			return fmt.Errorf("%s.serialNumber is duplicated", prefix)
		}
		if seenPush[terminal.PushSDKSerial] {
			return fmt.Errorf("%s.pushSdkSerial is duplicated", prefix)
		}
		seenDevice[terminal.SerialNumber], seenPush[terminal.PushSDKSerial] = true, true
		if terminal.LoginPasswordDigest != "sha1" && terminal.LoginPasswordDigest != "sha256" {
			return fmt.Errorf("%s.loginPasswordDigest must be sha1 or sha256", prefix)
		}
		if terminal.SecurityVersion != 3 && terminal.SecurityVersion != 4 {
			return fmt.Errorf("%s.securityVersion must be 3 or 4", prefix)
		}
		if terminal.CommandIntervalSeconds < 1 || terminal.CommandIntervalSeconds > 300 {
			return fmt.Errorf("%s.commandIntervalSeconds must be 1..300", prefix)
		}
		if terminal.ErrorDelaySeconds < 30 || terminal.ErrorDelaySeconds > 300 {
			return fmt.Errorf("%s.errorDelaySeconds must be 30..300", prefix)
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

func env(name string) string { return os.Getenv(name) }

var environmentVariableName = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
