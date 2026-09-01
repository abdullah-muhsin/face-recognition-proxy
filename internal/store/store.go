package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

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

func (s *Store) SyncTerminals(ctx context.Context, terminals []config.Terminal) error {
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
			terminal.SerialNumber, terminal.PushSDKSerial, terminal.Username, terminal.CredentialFingerprint(), terminal.Security, terminal.CommandSeconds, terminal.ErrorDelay)
		if err != nil {
			return fmt.Errorf("sync terminal %s: %w", terminal.SerialNumber, err)
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) EnsureAdmin(ctx context.Context, username, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO admin_users (username, password_hash) VALUES ($1, $2)
        ON CONFLICT (username) DO UPDATE SET password_hash = EXCLUDED.password_hash, updated_at = now()`, username, string(hash))
	if err != nil {
		return fmt.Errorf("upsert admin user: %w", err)
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

func (s *Store) SetTerminalState(ctx context.Context, serial, status string, lastError *string) error {
	command := `UPDATE terminals SET connection_status = $2, last_seen_at = now(), last_error = $3, updated_at = now() WHERE serial_number = $1`
	if status == "offline" {
		command = `UPDATE terminals SET connection_status = $2, last_error = $3, updated_at = now() WHERE serial_number = $1`
	}
	result, err := s.pool.Exec(ctx, command, serial, status, lastError)
	if err != nil {
		return fmt.Errorf("set terminal state: %w", err)
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("terminal %s is not registered", serial)
	}
	return nil
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

type AttendanceRecord struct {
	ID                   int64     `json:"id"`
	TerminalSerialNumber string    `json:"terminalSerialNumber"`
	VendorEventID        string    `json:"vendorEventId"`
	OccurredAt           time.Time `json:"occurredAt"`
	EmployeeNumber       string    `json:"employeeNumber"`
	EmployeeName         *string   `json:"employeeName"`
	VerificationMethod   string    `json:"verificationMethod"`
	AttendanceStatus     *string   `json:"attendanceStatus"`
	StatusValue          *int      `json:"statusValue"`
	SourceFormat         string    `json:"sourceFormat"`
	ReceivedAt           time.Time `json:"receivedAt"`
}

type NewAttendanceRecord struct {
	TerminalSerialNumber string
	VendorEventID        string
	OccurredAt           time.Time
	EmployeeNumber       string
	EmployeeName         *string
	VerificationMethod   string
	AttendanceStatus     *string
	StatusValue          *int
	SourceFormat         string
}

func (s *Store) InsertAttendance(ctx context.Context, record NewAttendanceRecord) (bool, error) {
	result, err := s.pool.Exec(ctx, `INSERT INTO attendance_records
        (terminal_serial_number, vendor_event_id, occurred_at, employee_number, employee_name, verification_method, attendance_status, status_value, source_format)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
        ON CONFLICT (terminal_serial_number, vendor_event_id) DO NOTHING`,
		record.TerminalSerialNumber, record.VendorEventID, record.OccurredAt, record.EmployeeNumber, record.EmployeeName,
		record.VerificationMethod, record.AttendanceStatus, record.StatusValue, record.SourceFormat)
	if err != nil {
		return false, fmt.Errorf("insert attendance: %w", err)
	}
	return result.RowsAffected() == 1, nil
}

// InsertAttendanceBatch is atomic: the device only receives success when every
// accepted item in its Event request has been durably handled.
func (s *Store) InsertAttendanceBatch(ctx context.Context, records []NewAttendanceRecord) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	inserted := 0
	for _, record := range records {
		result, err := tx.Exec(ctx, `INSERT INTO attendance_records
            (terminal_serial_number, vendor_event_id, occurred_at, employee_number, employee_name, verification_method, attendance_status, status_value, source_format)
            VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
            ON CONFLICT (terminal_serial_number, vendor_event_id) DO NOTHING`,
			record.TerminalSerialNumber, record.VendorEventID, record.OccurredAt, record.EmployeeNumber, record.EmployeeName,
			record.VerificationMethod, record.AttendanceStatus, record.StatusValue, record.SourceFormat)
		if err != nil {
			return 0, fmt.Errorf("insert attendance: %w", err)
		}
		inserted += int(result.RowsAffected())
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return inserted, nil
}

type AttendancePage struct {
	Records []AttendanceRecord `json:"records"`
	Total   int64              `json:"total"`
}

type Overview struct {
	AttendanceTotal int64           `json:"attendanceTotal"`
	Terminals       []TerminalState `json:"terminals"`
}

func (s *Store) Overview(ctx context.Context) (Overview, error) {
	var overview Overview
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM attendance_records`).Scan(&overview.AttendanceTotal); err != nil {
		return overview, err
	}
	states, err := s.TerminalStates(ctx)
	if err != nil {
		return overview, err
	}
	overview.Terminals = states
	return overview, nil
}

func (s *Store) Attendance(ctx context.Context, limit, offset int) (AttendancePage, error) {
	var page AttendancePage
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM attendance_records`).Scan(&page.Total); err != nil {
		return page, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id, terminal_serial_number, vendor_event_id, occurred_at, employee_number, employee_name,
        verification_method, attendance_status, status_value, source_format, received_at
        FROM attendance_records ORDER BY occurred_at DESC, id DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	page.Records = []AttendanceRecord{}
	for rows.Next() {
		var record AttendanceRecord
		if err := rows.Scan(&record.ID, &record.TerminalSerialNumber, &record.VendorEventID, &record.OccurredAt, &record.EmployeeNumber,
			&record.EmployeeName, &record.VerificationMethod, &record.AttendanceStatus, &record.StatusValue, &record.SourceFormat, &record.ReceivedAt); err != nil {
			return page, err
		}
		page.Records = append(page.Records, record)
	}
	return page, rows.Err()
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

func (s *Store) CreateSession(ctx context.Context, userID int64, tokenHash []byte, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO admin_sessions (admin_user_id, token_hash, expires_at) VALUES ($1,$2,$3)`, userID, tokenHash, expiresAt)
	return err
}

func (s *Store) SessionValid(ctx context.Context, tokenHash []byte) (bool, error) {
	var found bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM admin_sessions WHERE token_hash = $1 AND expires_at > now())`, tokenHash).Scan(&found)
	return found, err
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash []byte) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM admin_sessions WHERE token_hash = $1`, tokenHash)
	return err
}

func (s *Store) PurgeExpiredSessions(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM admin_sessions WHERE expires_at <= now()`)
	return err
}
