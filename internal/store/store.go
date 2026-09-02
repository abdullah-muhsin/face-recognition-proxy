package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/itplus/pushsdk-gateway/internal/activity"
	"github.com/itplus/pushsdk-gateway/internal/config"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type Store struct{ pool *pgxpool.Pool }

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

func (s *Store) Migrate(ctx context.Context, directory string) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("read migrations directory: %w", err)
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		return fmt.Errorf("no SQL migrations in %s", directory)
	}
	if _, err := s.pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
        name TEXT PRIMARY KEY, checksum TEXT NOT NULL, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return fmt.Errorf("create migration registry: %w", err)
	}
	for _, name := range files {
		contents, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		sum := sha256.Sum256(contents)
		checksum := hex.EncodeToString(sum[:])
		var recorded string
		err = s.pool.QueryRow(ctx, `SELECT checksum FROM schema_migrations WHERE name = $1`, name).Scan(&recorded)
		if err == nil {
			if recorded != checksum {
				return fmt.Errorf("migration %s has changed after application", name)
			}
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", name, err)
		}
		if _, err = tx.Exec(ctx, string(contents)); err == nil {
			_, err = tx.Exec(ctx, `INSERT INTO schema_migrations (name, checksum) VALUES ($1, $2)`, name, checksum)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if err = tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
	}
	return nil
}

// SynchronizeConfiguredTerminals upserts every deployed terminal mapping. A
// process restart cannot prove that an old socket is still alive, so every
// configured terminal starts as offline until it completes AuthInfo and Login
// again. Historical terminal rows are intentionally retained because device
// events reference their canonical serial numbers.
func (s *Store) SynchronizeConfiguredTerminals(ctx context.Context, terminals []config.Terminal) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, terminal := range terminals {
		_, err = tx.Exec(ctx, `INSERT INTO terminals
            (serial_number, pushsdk_serial, username, credential_fingerprint, security_version, command_interval_seconds, error_delay_seconds)
            VALUES ($1,$2,$3,$4,$5,$6,$7)
            ON CONFLICT (serial_number) DO UPDATE SET
              pushsdk_serial = EXCLUDED.pushsdk_serial,
              username = EXCLUDED.username,
              credential_fingerprint = EXCLUDED.credential_fingerprint,
              security_version = EXCLUDED.security_version,
              command_interval_seconds = EXCLUDED.command_interval_seconds,
			  error_delay_seconds = EXCLUDED.error_delay_seconds,
			  connection_status = 'offline',
			  last_error = NULL,
			  updated_at = now()`,
			terminal.SerialNumber, terminal.PushSDKSerial, terminal.Username, terminal.CredentialFingerprint(), terminal.SecurityVersion, terminal.CommandIntervalSeconds, terminal.ErrorDelaySeconds)
		if err != nil {
			return fmt.Errorf("reconcile terminal %s: %w", terminal.SerialNumber, err)
		}
	}
	return tx.Commit(ctx)
}

// ReconcileConfiguredOperator maintains the one operator account configured by
// ADMIN_USERNAME and ADMIN_PASSWORD. The application has no multi-user
// provisioning interface, so retaining a previous configured username would
// create an undocumented second administrative path.
func (s *Store) ReconcileConfiguredOperator(ctx context.Context, username, password string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin operator reconciliation: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM admin_users WHERE username <> $1`, username); err != nil {
		return fmt.Errorf("remove replaced operator: %w", err)
	}
	var currentHash string
	err = tx.QueryRow(ctx, `SELECT password_hash FROM admin_users WHERE username = $1`, username).Scan(&currentHash)
	if errors.Is(err, pgx.ErrNoRows) {
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if hashErr != nil {
			return fmt.Errorf("hash operator password: %w", hashErr)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO admin_users (username, password_hash) VALUES ($1, $2)`, username, string(hash)); err != nil {
			return fmt.Errorf("create configured operator: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("load configured operator: %w", err)
	} else if bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(password)) != nil {
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if hashErr != nil {
			return fmt.Errorf("hash operator password: %w", hashErr)
		}
		if _, err := tx.Exec(ctx, `UPDATE admin_users SET password_hash = $2, updated_at = now() WHERE username = $1`, username, string(hash)); err != nil {
			return fmt.Errorf("update configured operator: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit operator reconciliation: %w", err)
	}
	return nil
}

type TerminalState struct {
	SerialNumber  string     `json:"serialNumber"`
	PushSDKSerial string     `json:"pushSdkSerial"`
	Status        string     `json:"status"`
	LastSeenAt    *time.Time `json:"lastSeenAt"`
	LastError     *string    `json:"lastError"`
}

// TransitionTerminalState updates a terminal's current state and records the
// matching operational event in one transaction. A state shown to an operator
// therefore always has a durable explanation in gateway activity history.
func (s *Store) TransitionTerminalState(ctx context.Context, serial, status string, lastError *string, event activity.Event) (activity.Event, error) {
	if event.Terminal != serial {
		return activity.Event{}, fmt.Errorf("gateway activity terminal %q does not match state transition terminal %q", event.Terminal, serial)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return activity.Event{}, fmt.Errorf("begin terminal state transition: %w", err)
	}
	defer tx.Rollback(ctx)
	command := `UPDATE terminals SET connection_status = $2, last_seen_at = now(), last_error = $3, updated_at = now() WHERE serial_number = $1`
	if status == "offline" {
		command = `UPDATE terminals SET connection_status = $2, last_error = $3, updated_at = now() WHERE serial_number = $1`
	}
	result, err := tx.Exec(ctx, command, serial, status, lastError)
	if err != nil {
		return activity.Event{}, fmt.Errorf("set terminal state: %w", err)
	}
	if result.RowsAffected() != 1 {
		return activity.Event{}, fmt.Errorf("terminal %s is not registered", serial)
	}
	stored, err := insertGatewayActivity(ctx, tx, event)
	if err != nil {
		return activity.Event{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return activity.Event{}, fmt.Errorf("commit terminal state transition: %w", err)
	}
	return stored, nil
}

func (s *Store) TerminalStates(ctx context.Context) ([]TerminalState, error) {
	rows, err := s.pool.Query(ctx, `SELECT serial_number, pushsdk_serial, connection_status, last_seen_at, last_error FROM terminals ORDER BY serial_number`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	states := []TerminalState{}
	for rows.Next() {
		var state TerminalState
		if err := rows.Scan(&state.SerialNumber, &state.PushSDKSerial, &state.Status, &state.LastSeenAt, &state.LastError); err != nil {
			return nil, err
		}
		states = append(states, state)
	}
	return states, rows.Err()
}

type DeviceEvent struct {
	ID                   int64     `json:"id"`
	TerminalSerialNumber string    `json:"terminalSerialNumber"`
	VendorEventID        string    `json:"vendorEventId"`
	DataFormat           string    `json:"dataFormat"`
	PayloadAvailable     bool      `json:"payloadAvailable"`
	ReceivedAt           time.Time `json:"receivedAt"`
}

type DeviceEventPayload struct {
	ID            int64   `json:"id"`
	VendorEventID string  `json:"vendorEventId"`
	DataFormat    string  `json:"dataFormat"`
	PayloadBase64 *string `json:"payloadBase64"`
}

type NewDeviceEvent struct {
	TerminalSerialNumber string
	VendorEventID        string
	DataFormat           string
	PayloadBase64        string
}

type DeviceEventPersistence struct {
	Inserted bool
	Activity activity.Event
}

// InsertDeviceEventBatch is atomic: the terminal receives success only when
// every item in its Event request has been durably retained exactly as sent.
// Each returned value aligns with the input and records whether that event was
// newly inserted rather than a safe vendor-UUID retry, together with the
// matching committed gateway activity.
func (s *Store) InsertDeviceEventBatch(ctx context.Context, events []NewDeviceEvent) ([]DeviceEventPersistence, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	persisted := make([]DeviceEventPersistence, len(events))
	for index, event := range events {
		result, err := tx.Exec(ctx, `INSERT INTO device_events
			(terminal_serial_number, vendor_event_id, data_format, payload_base64)
			VALUES ($1,$2,$3,$4)
			ON CONFLICT (terminal_serial_number, vendor_event_id) DO NOTHING`,
			event.TerminalSerialNumber, event.VendorEventID, event.DataFormat, event.PayloadBase64)
		if err != nil {
			return nil, fmt.Errorf("insert device event: %w", err)
		}
		persisted[index].Inserted = result.RowsAffected() == 1
		kind, message := activity.KindDeviceEventDuplicate, "device event payload was already retained"
		if persisted[index].Inserted {
			kind, message = activity.KindDeviceEventPersisted, "device event payload persisted"
		}
		storedActivity, err := insertGatewayActivity(ctx, tx, activity.Event{
			Kind:     kind,
			Terminal: event.TerminalSerialNumber,
			Message:  message,
			Fields: map[string]any{
				"eventId":    event.VendorEventID,
				"dataFormat": event.DataFormat,
			},
		})
		if err != nil {
			return nil, err
		}
		persisted[index].Activity = storedActivity
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return persisted, nil
}

type DeviceEventPage struct {
	Events []DeviceEvent `json:"events"`
	Total  int64         `json:"total"`
}

type Overview struct {
	DeviceEventTotal int64           `json:"deviceEventTotal"`
	Terminals        []TerminalState `json:"terminals"`
}

func (s *Store) Overview(ctx context.Context) (Overview, error) {
	var overview Overview
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM device_events`).Scan(&overview.DeviceEventTotal); err != nil {
		return overview, err
	}
	states, err := s.TerminalStates(ctx)
	if err != nil {
		return overview, err
	}
	overview.Terminals = states
	return overview, nil
}

func (s *Store) DeviceEvents(ctx context.Context, limit, offset int) (DeviceEventPage, error) {
	var page DeviceEventPage
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM device_events`).Scan(&page.Total); err != nil {
		return page, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id, terminal_serial_number, vendor_event_id, data_format, payload_base64 IS NOT NULL, received_at
		FROM device_events ORDER BY received_at DESC, id DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	page.Events = []DeviceEvent{}
	for rows.Next() {
		var event DeviceEvent
		if err := rows.Scan(&event.ID, &event.TerminalSerialNumber, &event.VendorEventID, &event.DataFormat, &event.PayloadAvailable, &event.ReceivedAt); err != nil {
			return page, err
		}
		page.Events = append(page.Events, event)
	}
	return page, rows.Err()
}

func (s *Store) DeviceEventPayload(ctx context.Context, id int64) (DeviceEventPayload, bool, error) {
	var payload DeviceEventPayload
	err := s.pool.QueryRow(ctx, `SELECT id, vendor_event_id, data_format, payload_base64
		FROM device_events WHERE id = $1`, id).Scan(&payload.ID, &payload.VendorEventID, &payload.DataFormat, &payload.PayloadBase64)
	if errors.Is(err, pgx.ErrNoRows) {
		return payload, false, nil
	}
	if err != nil {
		return payload, false, err
	}
	return payload, true, nil
}

type GatewayActivityPage struct {
	Activities []activity.Event `json:"activities"`
	Total      int64            `json:"total"`
}

func (s *Store) GatewayActivities(ctx context.Context, limit, offset int) (GatewayActivityPage, error) {
	var page GatewayActivityPage
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM gateway_activities`).Scan(&page.Total); err != nil {
		return page, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id, occurred_at, kind, terminal_serial_number, message, fields
		FROM gateway_activities ORDER BY occurred_at DESC, id DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	page.Activities = []activity.Event{}
	for rows.Next() {
		var event activity.Event
		var terminal *string
		var fields []byte
		if err := rows.Scan(&event.ID, &event.At, &event.Kind, &terminal, &event.Message, &fields); err != nil {
			return page, err
		}
		if terminal != nil {
			event.Terminal = *terminal
		}
		if err := json.Unmarshal(fields, &event.Fields); err != nil {
			return page, fmt.Errorf("decode gateway activity %d fields: %w", event.ID, err)
		}
		page.Activities = append(page.Activities, event)
	}
	return page, rows.Err()
}

func (s *Store) RecordGatewayActivity(ctx context.Context, event activity.Event) (activity.Event, error) {
	stored, err := insertGatewayActivity(ctx, s.pool, event)
	if err != nil {
		return activity.Event{}, err
	}
	return stored, nil
}

type activityQueryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func insertGatewayActivity(ctx context.Context, queryer activityQueryer, event activity.Event) (activity.Event, error) {
	if event.Kind == "" {
		return activity.Event{}, errors.New("gateway activity kind is required")
	}
	if event.Message == "" {
		return activity.Event{}, errors.New("gateway activity message is required")
	}
	fields := []byte(`{}`)
	if event.Fields != nil {
		encoded, err := json.Marshal(event.Fields)
		if err != nil {
			return activity.Event{}, fmt.Errorf("encode gateway activity fields: %w", err)
		}
		fields = encoded
	}
	var terminal any
	if event.Terminal != "" {
		terminal = event.Terminal
	}
	if err := queryer.QueryRow(ctx, `INSERT INTO gateway_activities
		(kind, terminal_serial_number, message, fields)
		VALUES ($1, $2, $3, $4::jsonb)
		RETURNING id, occurred_at`, event.Kind, terminal, event.Message, string(fields)).Scan(&event.ID, &event.At); err != nil {
		return activity.Event{}, fmt.Errorf("record gateway activity: %w", err)
	}
	return event, nil
}

func (s *Store) VerifyAdmin(ctx context.Context, username, password string) (int64, bool, error) {
	var id int64
	var encoded string
	err := s.pool.QueryRow(ctx, `SELECT id, password_hash FROM admin_users WHERE username = $1`, username).Scan(&id, &encoded)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(encoded), []byte(password)); err != nil {
		return 0, false, nil
	}
	return id, true, nil
}

func (s *Store) CreateSessionWithActivity(ctx context.Context, userID int64, tokenHash []byte, expiresAt time.Time, event activity.Event) (activity.Event, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return activity.Event{}, fmt.Errorf("begin admin session: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `INSERT INTO admin_sessions (admin_user_id, token_hash, expires_at) VALUES ($1,$2,$3)`, userID, tokenHash, expiresAt); err != nil {
		return activity.Event{}, fmt.Errorf("create admin session: %w", err)
	}
	stored, err := insertGatewayActivity(ctx, tx, event)
	if err != nil {
		return activity.Event{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return activity.Event{}, fmt.Errorf("commit admin session: %w", err)
	}
	return stored, nil
}

func (s *Store) SessionValid(ctx context.Context, tokenHash []byte) (bool, error) {
	var found bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM admin_sessions WHERE token_hash = $1 AND expires_at > now())`, tokenHash).Scan(&found)
	return found, err
}

func (s *Store) DeleteSessionWithActivity(ctx context.Context, tokenHash []byte, event activity.Event) (activity.Event, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return activity.Event{}, fmt.Errorf("begin admin sign out: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM admin_sessions WHERE token_hash = $1`, tokenHash); err != nil {
		return activity.Event{}, fmt.Errorf("delete admin session: %w", err)
	}
	stored, err := insertGatewayActivity(ctx, tx, event)
	if err != nil {
		return activity.Event{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return activity.Event{}, fmt.Errorf("commit admin sign out: %w", err)
	}
	return stored, nil
}

func (s *Store) PurgeExpiredSessions(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM admin_sessions WHERE expires_at <= now()`)
	return err
}
