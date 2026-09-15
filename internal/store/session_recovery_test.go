package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/itplus/pushsdk-gateway/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSessionExpiryTransaction(t *testing.T) {
	dsn := os.Getenv("GATEWAY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set GATEWAY_TEST_DATABASE_URL to run PostgreSQL recovery tests")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("recovery_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE") }()
	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	poolConfig.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	data := &Store{pool: pool}
	contents, err := os.ReadFile(filepath.Join("..", "..", "db", "schema.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(contents)); err != nil {
		t.Fatal(err)
	}
	terminal := config.Terminal{SerialNumber: "DEVICE-1", PushSDKSerial: "PUSH-1", Username: "gateway", Password: "test-password", LoginPasswordDigest: "sha256", SecurityVersion: 4, CommandIntervalSeconds: 5, ErrorDelaySeconds: 30}
	if err := data.SeedConfiguredTerminals(ctx, []config.Terminal{terminal}); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE terminals SET connection_status = 'online'"); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Truncate(time.Microsecond)
	state := PushSDKSession{TerminalSerialNumber: terminal.SerialNumber, ConfigurationFingerprint: terminal.SessionFingerprint(), PayloadMode: "plaintext", Salt: strings.Repeat("s", 64), LoginChallenge: strings.Repeat("c", 64), NextChallenge: strings.Repeat("a", 64), Iterations: 4096, CreatedAt: at, NextChallengeAt: &at, Authenticated: true}
	if err := data.UpsertPushSDKSession(ctx, state); err != nil {
		t.Fatal(err)
	}
	// Fail the offline update after DELETE to exercise transactional rollback.
	if _, err := pool.Exec(ctx, `CREATE FUNCTION reject_offline() RETURNS trigger LANGUAGE plpgsql AS $$
	BEGIN IF NEW.connection_status = 'offline' THEN RAISE EXCEPTION 'test failure'; END IF; RETURN NEW; END $$;
	CREATE TRIGGER reject_offline BEFORE UPDATE ON terminals FOR EACH ROW EXECUTE FUNCTION reject_offline();`); err != nil {
		t.Fatal(err)
	}
	if _, err := data.ExpirePushSDKSession(ctx, terminal.SerialNumber, "expired"); err == nil {
		t.Fatal("expected expiry failure")
	}
	sessions, err := data.PushSDKSessions(ctx)
	if err != nil || len(sessions) != 1 {
		t.Fatalf("checkpoint lost on rollback: %v %v", sessions, err)
	}
	if _, err := pool.Exec(ctx, "DROP TRIGGER reject_offline ON terminals"); err != nil {
		t.Fatal(err)
	}
	event, err := data.ExpirePushSDKSession(ctx, terminal.SerialNumber, "expired")
	if err != nil || event == nil {
		t.Fatalf("expiry: %v %v", event, err)
	}
	var status string
	var lastSeen time.Time
	if err := pool.QueryRow(ctx, "SELECT connection_status, last_seen_at FROM terminals").Scan(&status, &lastSeen); err != nil {
		t.Fatal(err)
	}
	if status != "offline" || !lastSeen.Equal(at) {
		t.Fatalf("status/last seen = %s %v; want offline %v", status, lastSeen, at)
	}
	_, _, err = data.QueueISAPICommand(ctx, NewISAPICommand{
		TerminalSerialNumber: terminal.SerialNumber,
		CreatedBy:            AdminIdentity{ID: 1, Username: "test"}, Method: "GET",
		URL: "/ISAPI/System/time", DataFormat: "noData", ExpiresAt: time.Now().Add(time.Minute),
	})
	if !errors.Is(err, ErrISAPICommandTerminalOffline) {
		t.Fatalf("expired terminal accepted a command: %v", err)
	}
	sessions, err = data.PushSDKSessions(ctx)
	if err != nil || len(sessions) != 0 {
		t.Fatalf("expired checkpoint retained: %v %v", sessions, err)
	}
	if event, err := data.ExpirePushSDKSession(ctx, terminal.SerialNumber, "expired"); err != nil || event != nil {
		t.Fatalf("duplicate transition: %v %v", event, err)
	}
}
