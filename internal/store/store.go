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
	"strconv"
	"strings"
	"time"

	"github.com/itplus/pushsdk-gateway/internal/accesscontrol"
	"github.com/itplus/pushsdk-gateway/internal/activity"
	"github.com/itplus/pushsdk-gateway/internal/config"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type Store struct{ pool *pgxpool.Pool }

const schemaBaselineMigration = "schema-baseline-2026-09-02"

var ErrUnknownArchiveEventSource = errors.New("unknown event archive source")

type migrationFile struct {
	name     string
	contents []byte
	checksum string
}

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
	canonicalSchema, err := os.ReadFile(filepath.Join(filepath.Dir(directory), "schema.sql"))
	if err != nil {
		return fmt.Errorf("read canonical schema: %w", err)
	}
	legacyFiles, err := loadMigrationFiles(filepath.Join(directory, "legacy"))
	if err != nil {
		return err
	}
	forwardFiles, err := loadMigrationFiles(directory)
	if err != nil {
		return err
	}
	if _, err := s.pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
        name TEXT PRIMARY KEY, checksum TEXT NOT NULL, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return fmt.Errorf("create migration registry: %w", err)
	}
	baselineApplied, err := s.ensureCanonicalSchema(ctx, canonicalSchema, forwardFiles)
	if err != nil {
		return err
	}
	if !baselineApplied {
		if err := s.applyMigrationFiles(ctx, legacyFiles); err != nil {
			return err
		}
	}
	if err := s.applyMigrationFiles(ctx, forwardFiles); err != nil {
		return err
	}
	return nil
}

// ensureCanonicalSchema installs the final CREATE-only schema for an empty
// database. A database with pre-baseline migration records keeps its immutable
// incremental history and is upgraded through db/migrations/legacy instead.
func (s *Store) ensureCanonicalSchema(ctx context.Context, schema []byte, forwardFiles []migrationFile) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin canonical schema check: %w", err)
	}
	defer tx.Rollback(ctx)

	var recorded string
	err = tx.QueryRow(ctx, `SELECT checksum FROM schema_migrations WHERE name = $1`, schemaBaselineMigration).Scan(&recorded)
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return false, fmt.Errorf("check canonical schema baseline: %w", err)
	}

	var migrationCount int64
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&migrationCount); err != nil {
		return false, fmt.Errorf("count recorded migrations: %w", err)
	}
	if migrationCount != 0 {
		return false, nil
	}

	var terminalsExist bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass('public.terminals') IS NOT NULL`).Scan(&terminalsExist); err != nil {
		return false, fmt.Errorf("check for an unregistered schema: %w", err)
	}
	if terminalsExist {
		return false, errors.New("database has terminal tables but no recorded migration history")
	}

	sum := sha256.Sum256(schema)
	checksum := hex.EncodeToString(sum[:])
	if _, err := tx.Exec(ctx, string(schema)); err != nil {
		return false, fmt.Errorf("create canonical schema: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (name, checksum) VALUES ($1, $2)`, schemaBaselineMigration, checksum); err != nil {
		return false, fmt.Errorf("record canonical schema baseline: %w", err)
	}
	for _, migration := range forwardFiles {
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (name, checksum) VALUES ($1, $2)`, migration.name, migration.checksum); err != nil {
			return false, fmt.Errorf("record canonical schema forward migration %s: %w", migration.name, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit canonical schema baseline: %w", err)
	}
	return true, nil
}

func migrationFiles(directory string) ([]string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("read migrations directory %s: %w", directory, err)
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	return files, nil
}

func loadMigrationFiles(directory string) ([]migrationFile, error) {
	names, err := migrationFiles(directory)
	if err != nil {
		return nil, err
	}
	files := make([]migrationFile, 0, len(names))
	for _, name := range names {
		contents, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", name, err)
		}
		sum := sha256.Sum256(contents)
		files = append(files, migrationFile{name: name, contents: contents, checksum: hex.EncodeToString(sum[:])})
	}
	return files, nil
}

func (s *Store) applyMigrationFiles(ctx context.Context, files []migrationFile) error {
	for _, migration := range files {
		var recorded string
		err := s.pool.QueryRow(ctx, `SELECT checksum FROM schema_migrations WHERE name = $1`, migration.name).Scan(&recorded)
		if err == nil {
			if recorded != migration.checksum {
				return fmt.Errorf("migration %s has changed after application", migration.name)
			}
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("check migration %s: %w", migration.name, err)
		}
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", migration.name, err)
		}
		if _, err = tx.Exec(ctx, string(migration.contents)); err == nil {
			_, err = tx.Exec(ctx, `INSERT INTO schema_migrations (name, checksum) VALUES ($1, $2)`, migration.name, migration.checksum)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", migration.name, err)
		}
		if err = tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", migration.name, err)
		}
	}
	return nil
}

// SeedConfiguredTerminals upserts every deployed terminal mapping. A
// process restart cannot prove that a terminal is still reachable, so every
// configured terminal starts offline until its next valid PushSDK request
// confirms a restored session or completes a fresh authentication exchange.
// Historical terminal rows are intentionally retained because device events
// reference their canonical serial numbers.
func (s *Store) SeedConfiguredTerminals(ctx context.Context, terminals []config.Terminal) error {
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

// ReconcileConfiguredAdministrator maintains the one administrator account
// configured by ADMIN_USERNAME and ADMIN_PASSWORD. The application has no
// multi-user provisioning interface, so retaining a previous configured
// username would create an undocumented second administrative path.
func (s *Store) ReconcileConfiguredAdministrator(ctx context.Context, username, password string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin administrator reconciliation: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM admin_users WHERE username <> $1`, username); err != nil {
		return fmt.Errorf("remove replaced administrator: %w", err)
	}
	var currentHash string
	err = tx.QueryRow(ctx, `SELECT password_hash FROM admin_users WHERE username = $1`, username).Scan(&currentHash)
	if errors.Is(err, pgx.ErrNoRows) {
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if hashErr != nil {
			return fmt.Errorf("hash administrator password: %w", hashErr)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO admin_users (username, password_hash) VALUES ($1, $2)`, username, string(hash)); err != nil {
			return fmt.Errorf("create configured administrator: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("load configured administrator: %w", err)
	} else if bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(password)) != nil {
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if hashErr != nil {
			return fmt.Errorf("hash administrator password: %w", hashErr)
		}
		if _, err := tx.Exec(ctx, `UPDATE admin_users SET password_hash = $2, updated_at = now() WHERE username = $1`, username, string(hash)); err != nil {
			return fmt.Errorf("update configured administrator: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit administrator reconciliation: %w", err)
	}
	return nil
}

type TerminalState struct {
	SerialNumber          string              `json:"serialNumber"`
	PushSDKSerial         string              `json:"pushSdkSerial"`
	Status                string              `json:"status"`
	LastSeenAt            *time.Time          `json:"lastSeenAt"`
	LastError             *string             `json:"lastError"`
	LatestAccessEventSync *AccessEventSyncRun `json:"latestAccessEventSync,omitempty"`
}

// TransitionTerminalState updates a terminal's current state and records the
// matching operational event in one transaction. A state shown to an administrator
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
	rows, err := s.pool.Query(ctx, `SELECT terminal.serial_number, terminal.pushsdk_serial, terminal.connection_status,
		terminal.last_seen_at, terminal.last_error,
		sync.uuid, sync.created_by_username, sync.status, sync.search_started_at,
		sync.search_ended_at, sync.total_matches, sync.pages_completed,
		sync.records_imported, sync.records_duplicate, sync.failure, sync.created_at,
		sync.completed_at
		FROM terminals terminal
		LEFT JOIN LATERAL (
			SELECT uuid, created_by_username, status, search_started_at, search_ended_at,
				total_matches, pages_completed, records_imported, records_duplicate, failure,
				created_at, completed_at
			FROM access_event_sync_runs
			WHERE terminal_serial_number = terminal.serial_number
			ORDER BY created_at DESC, uuid DESC
			LIMIT 1
		) sync ON TRUE
		ORDER BY terminal.serial_number`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	states := []TerminalState{}
	for rows.Next() {
		var state TerminalState
		var latestUUID, latestCreatedBy, latestStatus *string
		var latestSearchStarted, latestSearchEnded *time.Time
		var latestTotal, latestPages, latestImported, latestDuplicate *int
		var latestFailure *string
		var latestCreatedAt, latestCompletedAt *time.Time
		if err := rows.Scan(
			&state.SerialNumber, &state.PushSDKSerial, &state.Status, &state.LastSeenAt, &state.LastError,
			&latestUUID, &latestCreatedBy, &latestStatus, &latestSearchStarted, &latestSearchEnded,
			&latestTotal, &latestPages, &latestImported, &latestDuplicate, &latestFailure,
			&latestCreatedAt, &latestCompletedAt,
		); err != nil {
			return nil, err
		}
		if latestUUID != nil {
			if latestCreatedBy == nil || latestStatus == nil || latestPages == nil || latestImported == nil || latestDuplicate == nil || latestCreatedAt == nil {
				return nil, errors.New("latest retained event sync is incomplete")
			}
			state.LatestAccessEventSync = &AccessEventSyncRun{
				UUID:                 *latestUUID,
				TerminalSerialNumber: state.SerialNumber,
				CreatedByUsername:    *latestCreatedBy,
				Status:               *latestStatus,
				SearchStartedAt:      latestSearchStarted,
				SearchEndedAt:        latestSearchEnded,
				TotalMatches:         latestTotal,
				PagesCompleted:       *latestPages,
				RecordsImported:      *latestImported,
				RecordsDuplicate:     *latestDuplicate,
				Failure:              latestFailure,
				CreatedAt:            *latestCreatedAt,
				CompletedAt:          latestCompletedAt,
			}
		}
		states = append(states, state)
	}
	return states, rows.Err()
}

// PushSDKSession contains only protocol state already shared with a terminal.
// It deliberately excludes the terminal password, which remains in the
// protected environment and is used only when validating protocol messages.
type PushSDKSession struct {
	TerminalSerialNumber     string
	ConfigurationFingerprint string
	PayloadMode              string
	Salt                     string
	LoginChallenge           string
	NextChallenge            string
	Iterations               int
	CreatedAt                time.Time
	NextChallengeAt          *time.Time
	Authenticated            bool
}

func (s *Store) UpsertPushSDKSession(ctx context.Context, session PushSDKSession) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin PushSDK checkpoint: %w", err)
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `INSERT INTO pushsdk_sessions
		(terminal_serial_number, configuration_fingerprint, payload_mode, salt, login_challenge, next_challenge, iterations, created_at, next_challenge_at, authenticated)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (terminal_serial_number) DO UPDATE SET
		  configuration_fingerprint = EXCLUDED.configuration_fingerprint,
		  payload_mode = EXCLUDED.payload_mode,
		  salt = EXCLUDED.salt,
		  login_challenge = EXCLUDED.login_challenge,
		  next_challenge = EXCLUDED.next_challenge,
		  iterations = EXCLUDED.iterations,
		  created_at = EXCLUDED.created_at,
		  next_challenge_at = EXCLUDED.next_challenge_at,
		  authenticated = EXCLUDED.authenticated`,
		session.TerminalSerialNumber,
		session.ConfigurationFingerprint,
		session.PayloadMode,
		session.Salt,
		session.LoginChallenge,
		session.NextChallenge,
		session.Iterations,
		session.CreatedAt,
		session.NextChallengeAt,
		session.Authenticated,
	)
	if err != nil {
		return fmt.Errorf("upsert PushSDK session: %w", err)
	}
	if session.Authenticated {
		// Only a successfully authenticated exchange refreshes last seen.
		// A timeout or an unauthenticated retry must never make a device look live.
		if _, err := tx.Exec(ctx, `UPDATE terminals SET last_seen_at = $2, updated_at = now()
			WHERE serial_number = $1`, session.TerminalSerialNumber, session.NextChallengeAt); err != nil {
			return fmt.Errorf("update terminal last seen: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// ExpirePushSDKSession atomically removes unusable credentials and marks the
// terminal offline without advancing its last authenticated contact time.
// Repeated expiry/rejection does not emit repeated offline transitions.
func (s *Store) ExpirePushSDKSession(ctx context.Context, serial, reason string) (*activity.Event, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM pushsdk_sessions WHERE terminal_serial_number = $1`, serial); err != nil {
		return nil, err
	}
	result, err := tx.Exec(ctx, `UPDATE terminals SET connection_status = 'offline', last_error = $2, updated_at = now()
		WHERE serial_number = $1 AND connection_status <> 'offline'`, serial, reason)
	if err != nil {
		return nil, err
	}
	var event *activity.Event
	if result.RowsAffected() > 0 {
		stored, err := insertGatewayActivity(ctx, tx, activity.Event{
			Kind: activity.KindPushSDKSessionExpired, Terminal: serial, Message: reason,
		})
		if err != nil {
			return nil, err
		}
		event = &stored
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return event, nil
}

func (s *Store) PushSDKSessions(ctx context.Context) ([]PushSDKSession, error) {
	rows, err := s.pool.Query(ctx, `SELECT terminal_serial_number, configuration_fingerprint, payload_mode, salt, login_challenge, next_challenge, iterations, created_at, next_challenge_at, authenticated
		FROM pushsdk_sessions ORDER BY terminal_serial_number`)
	if err != nil {
		return nil, fmt.Errorf("load PushSDK sessions: %w", err)
	}
	defer rows.Close()
	sessions := []PushSDKSession{}
	for rows.Next() {
		var session PushSDKSession
		if err := rows.Scan(
			&session.TerminalSerialNumber,
			&session.ConfigurationFingerprint,
			&session.PayloadMode,
			&session.Salt,
			&session.LoginChallenge,
			&session.NextChallenge,
			&session.Iterations,
			&session.CreatedAt,
			&session.NextChallengeAt,
			&session.Authenticated,
		); err != nil {
			return nil, fmt.Errorf("scan PushSDK session: %w", err)
		}
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate PushSDK sessions: %w", err)
	}
	return sessions, nil
}

func (s *Store) DeletePushSDKSession(ctx context.Context, terminalSerial string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM pushsdk_sessions WHERE terminal_serial_number = $1`, terminalSerial)
	if err != nil {
		return fmt.Errorf("delete PushSDK session: %w", err)
	}
	return nil
}

type DeviceEvent struct {
	ID                   int64        `json:"id"`
	Source               string       `json:"source"`
	TerminalSerialNumber string       `json:"terminalSerialNumber"`
	SourceRecordID       string       `json:"sourceRecordId"`
	DataFormat           string       `json:"dataFormat"`
	PayloadAvailable     bool         `json:"payloadAvailable"`
	ReceivedAt           time.Time    `json:"receivedAt"`
	AccessEvent          *AccessEvent `json:"accessEvent,omitempty"`
}

// AccessEvent is a strict projection of a documented JSON
// AccessControllerEvent. It is nil when the raw source event is not that
// declared form; the raw archive remains available in either case.
type AccessEvent struct {
	Category            string               `json:"category,omitempty"`
	MajorEventType      int                  `json:"majorEventType"`
	SubEventType        int                  `json:"subEventType"`
	SubtypeLabel        string               `json:"subtypeLabel,omitempty"`
	EventDescription    *string              `json:"eventDescription"`
	EventState          *string              `json:"eventState"`
	OccurredAt          *time.Time           `json:"occurredAt"`
	SourceIPAddress     *string              `json:"sourceIpAddress"`
	SourceMACAddress    *string              `json:"sourceMacAddress"`
	ChannelID           *int                 `json:"channelId"`
	ActivePostCount     *int                 `json:"activePostCount"`
	ShortSerialNumber   *string              `json:"shortSerialNumber"`
	DeviceName          *string              `json:"deviceName"`
	EmployeeNumber      *string              `json:"employeeNumber"`
	EmployeeName        *string              `json:"employeeName"`
	CardNumber          *string              `json:"cardNumber"`
	CardReaderNumber    *int                 `json:"cardReaderNumber"`
	DoorNumber          *int                 `json:"doorNumber"`
	EventSerialNumber   *int64               `json:"eventSerialNumber"`
	FrontSerialNumber   *int64               `json:"frontSerialNumber"`
	UserType            *string              `json:"userType"`
	CurrentVerifyMode   *string              `json:"currentVerifyMode"`
	CurrentEvent        *bool                `json:"currentEvent"`
	Mask                *string              `json:"mask"`
	PicturesNumber      *int                 `json:"picturesNumber"`
	PurePwdVerifyEnable *bool                `json:"purePwdVerifyEnable"`
	FaceRect            *AccessEventFaceRect `json:"faceRect,omitempty"`
}

// AccessEventFaceRect preserves the event's declared JSON number text. It is
// not a display-space or image-space transformation.
type AccessEventFaceRect struct {
	Height *string `json:"height"`
	Width  *string `json:"width"`
	X      *string `json:"x"`
	Y      *string `json:"y"`
}

type AccessEventSubtype struct {
	Code  int     `json:"code"`
	Label *string `json:"label,omitempty"`
}

type DeviceEventPayload struct {
	ID             int64   `json:"id"`
	Source         string  `json:"source"`
	SourceRecordID string  `json:"sourceRecordId"`
	DataFormat     string  `json:"dataFormat"`
	PayloadBase64  *string `json:"payloadBase64"`
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
		var id int64
		err = tx.QueryRow(ctx, `INSERT INTO device_events
			(terminal_serial_number, vendor_event_id, data_format, payload_base64)
			VALUES ($1,$2,$3,$4)
			ON CONFLICT (terminal_serial_number, vendor_event_id) DO NOTHING
			RETURNING id`, event.TerminalSerialNumber, event.VendorEventID, event.DataFormat, event.PayloadBase64).Scan(&id)
		inserted := err == nil
		if errors.Is(err, pgx.ErrNoRows) {
			err = nil
		}
		if err != nil {
			return nil, fmt.Errorf("insert device event: %w", err)
		}
		persisted[index].Inserted = inserted
		if inserted {
			if err := insertAccessEventProjection(ctx, tx, id, event); err != nil {
				return nil, err
			}
		}
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
	Events   []DeviceEvent        `json:"events"`
	Total    int64                `json:"total"`
	Subtypes []AccessEventSubtype `json:"subtypes"`
}

type DeviceEventQuery struct {
	Limit          int
	Offset         int
	MajorEventType *int
	SubEventType   *int
	Terminal       string
	Source         string
}

type Overview struct {
	DeviceEventTotal int64           `json:"deviceEventTotal"`
	Terminals        []TerminalState `json:"terminals"`
}

func (s *Store) Overview(ctx context.Context) (Overview, error) {
	var overview Overview
	if err := s.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM device_events) + (SELECT count(*) FROM retained_access_events)`).Scan(&overview.DeviceEventTotal); err != nil {
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
	return s.QueryDeviceEvents(ctx, DeviceEventQuery{Limit: limit, Offset: offset})
}

func (s *Store) QueryDeviceEvents(ctx context.Context, query DeviceEventQuery) (DeviceEventPage, error) {
	var page DeviceEventPage
	clauses, arguments := archiveEventClauses(query)
	where := strings.Join(clauses, " AND ")
	if err := s.pool.QueryRow(ctx, archiveEventCTE+`SELECT count(*) FROM archive_events WHERE `+where, arguments...).Scan(&page.Total); err != nil {
		return page, err
	}
	limitIndex := len(arguments) + 1
	offsetIndex := len(arguments) + 2
	arguments = append(arguments, query.Limit, query.Offset)
	rows, err := s.pool.Query(ctx, archiveEventCTE+`SELECT source, id, terminal_serial_number, source_record_id, data_format, payload_available, received_at,
		major_event_type, sub_event_type, event_description, occurred_at, employee_number, employee_name,
		card_number, card_reader_number, door_number, source_ip_address,
		event_state, source_mac_address, channel_id, active_post_count,
		short_serial_number, device_name, event_serial_number, front_serial_number,
		user_type, current_verify_mode, current_event, mask, pictures_number,
		pure_pwd_verify_enable, face_rect_height, face_rect_width, face_rect_x, face_rect_y
		FROM archive_events
		WHERE `+where+` ORDER BY received_at DESC, id DESC LIMIT $`+strconv.Itoa(limitIndex)+` OFFSET $`+strconv.Itoa(offsetIndex), arguments...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	page.Events = []DeviceEvent{}
	for rows.Next() {
		var event DeviceEvent
		var access AccessEvent
		var faceRect AccessEventFaceRect
		var major, subtype *int
		if err := rows.Scan(&event.Source, &event.ID, &event.TerminalSerialNumber, &event.SourceRecordID, &event.DataFormat, &event.PayloadAvailable, &event.ReceivedAt,
			&major, &subtype, &access.EventDescription, &access.OccurredAt, &access.EmployeeNumber, &access.EmployeeName,
			&access.CardNumber, &access.CardReaderNumber, &access.DoorNumber, &access.SourceIPAddress,
			&access.EventState, &access.SourceMACAddress, &access.ChannelID, &access.ActivePostCount,
			&access.ShortSerialNumber, &access.DeviceName, &access.EventSerialNumber, &access.FrontSerialNumber,
			&access.UserType, &access.CurrentVerifyMode, &access.CurrentEvent, &access.Mask, &access.PicturesNumber,
			&access.PurePwdVerifyEnable, &faceRect.Height, &faceRect.Width, &faceRect.X, &faceRect.Y); err != nil {
			return page, err
		}
		if major != nil && subtype != nil {
			access.MajorEventType = *major
			access.SubEventType = *subtype
			access.Category, _ = accesscontrol.CategoryForMajorEventType(*major)
			access.SubtypeLabel, _ = accesscontrol.SubtypeLabel(*major, *subtype)
			if faceRect.Height != nil || faceRect.Width != nil || faceRect.X != nil || faceRect.Y != nil {
				access.FaceRect = &faceRect
			}
			event.AccessEvent = &access
		}
		page.Events = append(page.Events, event)
	}
	if err := rows.Err(); err != nil {
		return page, err
	}
	page.Subtypes, err = s.accessEventSubtypes(ctx, query)
	return page, err
}

const archiveEventCTE = `WITH archive_events AS (
	SELECT 'pushsdk'::TEXT AS source, de.id, de.terminal_serial_number, de.vendor_event_id AS source_record_id,
		de.data_format, (de.payload_base64 IS NOT NULL) AS payload_available, de.received_at,
		ace.major_event_type, ace.sub_event_type, ace.event_description, ace.occurred_at,
		ace.employee_number, ace.employee_name, ace.card_number, ace.card_reader_number, ace.door_number,
		ace.source_ip_address, ace.event_state, ace.source_mac_address, ace.channel_id, ace.active_post_count,
		ace.short_serial_number, ace.device_name, ace.event_serial_number, ace.front_serial_number,
		ace.user_type, ace.current_verify_mode, ace.current_event, ace.mask, ace.pictures_number,
		ace.pure_pwd_verify_enable, ace.face_rect_height, ace.face_rect_width, ace.face_rect_x, ace.face_rect_y
	FROM device_events de
	LEFT JOIN access_event_projections ace ON ace.device_event_id = de.id
	UNION ALL
	SELECT 'isapi'::TEXT AS source, retained.id, retained.terminal_serial_number, encode(retained.source_sha256, 'hex') AS source_record_id,
		'jsonData'::TEXT AS data_format, TRUE AS payload_available, retained.imported_at,
		retained.major_event_type, retained.sub_event_type, NULL::TEXT AS event_description, retained.occurred_at,
		retained.employee_number, retained.employee_name, retained.card_number, retained.card_reader_number, retained.door_number,
		NULL::TEXT AS source_ip_address, NULL::TEXT AS event_state, NULL::TEXT AS source_mac_address, NULL::INTEGER AS channel_id, NULL::INTEGER AS active_post_count,
		NULL::TEXT AS short_serial_number, NULL::TEXT AS device_name, retained.event_serial_number, NULL::BIGINT AS front_serial_number,
		retained.user_type, retained.current_verify_mode, NULL::BOOLEAN AS current_event, retained.mask, NULL::INTEGER AS pictures_number,
		NULL::BOOLEAN AS pure_pwd_verify_enable, retained.face_rect_height, retained.face_rect_width, retained.face_rect_x, retained.face_rect_y
	FROM retained_access_events retained
)
`

func archiveEventClauses(query DeviceEventQuery) ([]string, []any) {
	clauses := []string{"TRUE"}
	arguments := []any{}
	if query.MajorEventType != nil {
		arguments = append(arguments, *query.MajorEventType)
		clauses = append(clauses, "major_event_type = $"+strconv.Itoa(len(arguments)))
	}
	if query.SubEventType != nil {
		arguments = append(arguments, *query.SubEventType)
		clauses = append(clauses, "sub_event_type = $"+strconv.Itoa(len(arguments)))
	}
	if query.Terminal != "" {
		arguments = append(arguments, query.Terminal)
		clauses = append(clauses, "terminal_serial_number = $"+strconv.Itoa(len(arguments)))
	}
	if query.Source != "" {
		arguments = append(arguments, query.Source)
		clauses = append(clauses, "source = $"+strconv.Itoa(len(arguments)))
	}
	return clauses, arguments
}

func (s *Store) accessEventSubtypes(ctx context.Context, query DeviceEventQuery) ([]AccessEventSubtype, error) {
	if query.MajorEventType == nil {
		return []AccessEventSubtype{}, nil
	}
	query.SubEventType = nil
	clauses, arguments := archiveEventClauses(query)
	rows, err := s.pool.Query(ctx, archiveEventCTE+`SELECT sub_event_type
		FROM archive_events
		WHERE `+strings.Join(clauses, " AND ")+`
		GROUP BY sub_event_type
		ORDER BY sub_event_type`, arguments...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	subtypes := []AccessEventSubtype{}
	for rows.Next() {
		var subtype AccessEventSubtype
		if err := rows.Scan(&subtype.Code); err != nil {
			return nil, err
		}
		if label, known := accesscontrol.SubtypeLabel(*query.MajorEventType, subtype.Code); known {
			subtype.Label = &label
		}
		subtypes = append(subtypes, subtype)
	}
	return subtypes, rows.Err()
}

func insertAccessEventProjection(ctx context.Context, tx pgx.Tx, deviceEventID int64, event NewDeviceEvent) error {
	projection, classified := accesscontrol.Extract(event.DataFormat, event.PayloadBase64)
	if !classified {
		_, err := tx.Exec(ctx, `INSERT INTO access_event_projections
			(device_event_id, schema_version, classification_status)
			VALUES ($1, $2, 'unclassified')
			ON CONFLICT (device_event_id) DO UPDATE SET
				schema_version = EXCLUDED.schema_version,
				classification_status = EXCLUDED.classification_status,
				major_event_type = NULL,
				sub_event_type = NULL,
				event_description = NULL,
				occurred_at = NULL,
				employee_number = NULL,
				employee_name = NULL,
				card_number = NULL,
				card_reader_number = NULL,
				door_number = NULL,
				source_ip_address = NULL,
				event_state = NULL,
				source_mac_address = NULL,
				channel_id = NULL,
				active_post_count = NULL,
				short_serial_number = NULL,
				device_name = NULL,
				event_serial_number = NULL,
				front_serial_number = NULL,
				user_type = NULL,
				current_verify_mode = NULL,
				current_event = NULL,
				mask = NULL,
				pictures_number = NULL,
				pure_pwd_verify_enable = NULL,
				face_rect_height = NULL,
				face_rect_width = NULL,
				face_rect_x = NULL,
				face_rect_y = NULL`, deviceEventID, accesscontrol.SchemaVersion)
		if err != nil {
			return fmt.Errorf("record unclassified access event: %w", err)
		}
		return nil
	}
	var faceRectHeight, faceRectWidth, faceRectX, faceRectY *string
	if projection.FaceRect != nil {
		faceRectHeight = projection.FaceRect.Height
		faceRectWidth = projection.FaceRect.Width
		faceRectX = projection.FaceRect.X
		faceRectY = projection.FaceRect.Y
	}
	_, err := tx.Exec(ctx, `INSERT INTO access_event_projections
		(device_event_id, schema_version, classification_status, major_event_type, sub_event_type,
		event_description, occurred_at, employee_number, employee_name, card_number,
		card_reader_number, door_number, source_ip_address, event_state, source_mac_address,
		channel_id, active_post_count, short_serial_number, device_name, event_serial_number,
		front_serial_number, user_type, current_verify_mode, current_event, mask, pictures_number,
		pure_pwd_verify_enable, face_rect_height, face_rect_width, face_rect_x, face_rect_y)
		VALUES ($1, $2, 'classified', $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
		$13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27,
		$28, $29, $30)
		ON CONFLICT (device_event_id) DO UPDATE SET
		schema_version = EXCLUDED.schema_version,
		classification_status = EXCLUDED.classification_status,
		major_event_type = EXCLUDED.major_event_type,
		sub_event_type = EXCLUDED.sub_event_type,
		event_description = EXCLUDED.event_description,
		occurred_at = EXCLUDED.occurred_at,
		employee_number = EXCLUDED.employee_number,
		employee_name = EXCLUDED.employee_name,
		card_number = EXCLUDED.card_number,
		card_reader_number = EXCLUDED.card_reader_number,
		door_number = EXCLUDED.door_number,
		source_ip_address = EXCLUDED.source_ip_address,
		event_state = EXCLUDED.event_state,
		source_mac_address = EXCLUDED.source_mac_address,
		channel_id = EXCLUDED.channel_id,
		active_post_count = EXCLUDED.active_post_count,
		short_serial_number = EXCLUDED.short_serial_number,
		device_name = EXCLUDED.device_name,
		event_serial_number = EXCLUDED.event_serial_number,
		front_serial_number = EXCLUDED.front_serial_number,
		user_type = EXCLUDED.user_type,
		current_verify_mode = EXCLUDED.current_verify_mode,
		current_event = EXCLUDED.current_event,
		mask = EXCLUDED.mask,
		pictures_number = EXCLUDED.pictures_number,
		pure_pwd_verify_enable = EXCLUDED.pure_pwd_verify_enable,
		face_rect_height = EXCLUDED.face_rect_height,
		face_rect_width = EXCLUDED.face_rect_width,
		face_rect_x = EXCLUDED.face_rect_x,
		face_rect_y = EXCLUDED.face_rect_y`,
		deviceEventID, accesscontrol.SchemaVersion, projection.MajorEventType, projection.SubEventType,
		projection.EventDescription, projection.OccurredAt, projection.EmployeeNumber, projection.EmployeeName,
		projection.CardNumber, projection.CardReaderNumber, projection.DoorNumber, projection.SourceIPAddress,
		projection.EventState, projection.SourceMACAddress, projection.ChannelID, projection.ActivePostCount,
		projection.ShortSerialNumber, projection.DeviceName, projection.EventSerialNumber, projection.FrontSerialNumber,
		projection.UserType, projection.CurrentVerifyMode, projection.CurrentEvent, projection.Mask, projection.PicturesNumber,
		projection.PurePwdVerifyEnable, faceRectHeight, faceRectWidth, faceRectX, faceRectY)
	if err != nil {
		return fmt.Errorf("record classified access event: %w", err)
	}
	return nil
}

// BackfillAccessEventProjections applies the same strict extractor to raw
// archive rows that predate the current projection schema. Every row is
// interpreted at most once per explicit schema version.
func (s *Store) BackfillAccessEventProjections(ctx context.Context) (int64, error) {
	var projected int64
	for {
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return projected, err
		}
		rows, err := tx.Query(ctx, `SELECT de.id, de.data_format, de.payload_base64
			FROM device_events de
			LEFT JOIN access_event_projections ace ON ace.device_event_id = de.id
			WHERE ace.device_event_id IS NULL OR ace.schema_version < $1
			ORDER BY de.id
			LIMIT 500
			FOR UPDATE OF de SKIP LOCKED`, accesscontrol.SchemaVersion)
		if err != nil {
			_ = tx.Rollback(ctx)
			return projected, fmt.Errorf("load event projection backfill batch: %w", err)
		}
		var batch []struct {
			id     int64
			format string
			data   *string
		}
		for rows.Next() {
			var row struct {
				id     int64
				format string
				data   *string
			}
			if err := rows.Scan(&row.id, &row.format, &row.data); err != nil {
				rows.Close()
				_ = tx.Rollback(ctx)
				return projected, fmt.Errorf("scan event projection backfill batch: %w", err)
			}
			batch = append(batch, row)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			_ = tx.Rollback(ctx)
			return projected, fmt.Errorf("iterate event projection backfill batch: %w", err)
		}
		rows.Close()
		for _, row := range batch {
			payload := ""
			if row.data != nil {
				payload = *row.data
			}
			if err := insertAccessEventProjection(ctx, tx, row.id, NewDeviceEvent{DataFormat: row.format, PayloadBase64: payload}); err != nil {
				_ = tx.Rollback(ctx)
				return projected, err
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return projected, fmt.Errorf("commit event projection backfill batch: %w", err)
		}
		projected += int64(len(batch))
		if len(batch) == 0 {
			return projected, nil
		}
	}
}

func (s *Store) DeviceEventPayload(ctx context.Context, id int64) (DeviceEventPayload, bool, error) {
	return s.ArchiveEventPayload(ctx, "pushsdk", id)
}

func (s *Store) ArchiveEventPayload(ctx context.Context, source string, id int64) (DeviceEventPayload, bool, error) {
	var payload DeviceEventPayload
	var err error
	switch source {
	case "pushsdk":
		err = s.pool.QueryRow(ctx, `SELECT id, 'pushsdk'::TEXT, vendor_event_id, data_format, payload_base64
			FROM device_events WHERE id = $1`, id).Scan(&payload.ID, &payload.Source, &payload.SourceRecordID, &payload.DataFormat, &payload.PayloadBase64)
	case "isapi":
		err = s.pool.QueryRow(ctx, `SELECT id, 'isapi'::TEXT, encode(source_sha256, 'hex'), 'jsonData'::TEXT, encode(source_record, 'base64')
			FROM retained_access_events WHERE id = $1`, id).Scan(&payload.ID, &payload.Source, &payload.SourceRecordID, &payload.DataFormat, &payload.PayloadBase64)
	default:
		return payload, false, fmt.Errorf("%w %q", ErrUnknownArchiveEventSource, source)
	}
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
