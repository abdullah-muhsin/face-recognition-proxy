package store

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/itplus/pushsdk-gateway/internal/activity"
	"github.com/jackc/pgx/v5"
)

var (
	ErrISAPICommandTerminalNotFound = errors.New("ISAPI command terminal is not registered")
	ErrISAPICommandTerminalOffline  = errors.New("ISAPI command terminal is offline")
)

// AdminIdentity identifies the configured administrator that performed an
// administration action. The username is stored with an ISAPI command so its
// audit record remains meaningful if the configured account is later replaced.
type AdminIdentity struct {
	ID       int64
	Username string
}

// NewISAPICommand is an exact ISAPI request ready for PushSDK transport. Data
// contains the payload bytes after the outer administration API has decoded
// its one documented representation; it is never parsed or reformatted here.
type NewISAPICommand struct {
	TerminalSerialNumber string
	CreatedBy            AdminIdentity
	Method               string
	URL                  string
	DataFormat           string
	Data                 []byte
	ExpiresAt            time.Time
}

type ISAPICommand struct {
	UUID                       string     `json:"uuid"`
	TerminalSerialNumber       string     `json:"terminalSerialNumber"`
	CreatedByUsername          string     `json:"createdByUsername"`
	Method                     string     `json:"method"`
	URL                        string     `json:"url"`
	DataFormat                 string     `json:"dataFormat"`
	Status                     string     `json:"status"`
	CreatedAt                  time.Time  `json:"createdAt"`
	ExpiresAt                  time.Time  `json:"expiresAt"`
	SentAt                     *time.Time `json:"sentAt"`
	CompletedAt                *time.Time `json:"completedAt"`
	ResponseDataFormat         *string    `json:"responseDataFormat"`
	ResponseDataFormatDeclared *bool      `json:"responseDataFormatDeclared"`
	ResponseDataAvailable      bool       `json:"responseDataAvailable"`
}

type ISAPICommandPayload struct {
	ISAPICommand
	RequestDataBase64  string  `json:"requestDataBase64"`
	ResponseDataBase64 *string `json:"responseDataBase64"`
}

type ISAPICommandPage struct {
	Commands []ISAPICommand `json:"commands"`
	Total    int64          `json:"total"`
}

// ISAPICommandDelivery is the exact request selected for a terminal's next
// CommandRequest response.
type ISAPICommandDelivery struct {
	UUID       string
	Method     string
	URL        string
	DataFormat string
	Data       []byte
}

// ISAPICommandResult is the exact data value supplied by a terminal in a
// CommandResult item. DataBase64 is intentionally retained verbatim rather
// than decoded and re-encoded.
type ISAPICommandResult struct {
	UUID       string
	DataFormat *string
	DataBase64 string
}

func (s *Store) QueueISAPICommand(ctx context.Context, input NewISAPICommand) (ISAPICommand, activity.Event, error) {
	var command ISAPICommand
	if input.CreatedBy.ID == 0 || input.CreatedBy.Username == "" {
		return command, activity.Event{}, errors.New("ISAPI command administrator identity is required")
	}
	if !input.ExpiresAt.After(time.Now().UTC()) {
		return command, activity.Event{}, errors.New("ISAPI command expiry must be in the future")
	}
	uuid, err := newISAPICommandUUID()
	if err != nil {
		return command, activity.Event{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return command, activity.Event{}, fmt.Errorf("begin queue ISAPI command: %w", err)
	}
	defer tx.Rollback(ctx)

	var terminalStatus string
	err = tx.QueryRow(ctx, `SELECT connection_status FROM terminals WHERE serial_number = $1 FOR KEY SHARE`, input.TerminalSerialNumber).Scan(&terminalStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return command, activity.Event{}, ErrISAPICommandTerminalNotFound
	}
	if err != nil {
		return command, activity.Event{}, fmt.Errorf("load ISAPI command terminal: %w", err)
	}
	if terminalStatus != "online" {
		return command, activity.Event{}, ErrISAPICommandTerminalOffline
	}

	command = ISAPICommand{
		UUID:                 uuid,
		TerminalSerialNumber: input.TerminalSerialNumber,
		CreatedByUsername:    input.CreatedBy.Username,
		Method:               input.Method,
		URL:                  input.URL,
		DataFormat:           input.DataFormat,
		Status:               "queued",
		ExpiresAt:            input.ExpiresAt.UTC(),
	}
	err = tx.QueryRow(ctx, `INSERT INTO isapi_commands
		(uuid, terminal_serial_number, created_by_admin_user_id, created_by_username, method, url, data_format, request_data, status, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'queued',$9)
		RETURNING created_at`, command.UUID, command.TerminalSerialNumber, input.CreatedBy.ID, command.CreatedByUsername,
		command.Method, command.URL, command.DataFormat, input.Data, command.ExpiresAt).Scan(&command.CreatedAt)
	if err != nil {
		return ISAPICommand{}, activity.Event{}, fmt.Errorf("insert ISAPI command: %w", err)
	}
	stored, err := insertGatewayActivity(ctx, tx, activity.Event{
		Kind:     activity.KindAdminISAPICommandQueued,
		Terminal: command.TerminalSerialNumber,
		Message:  "administrator queued an ISAPI command",
		Fields: commandActivityFields(command, map[string]any{
			"issuedBy": command.CreatedByUsername,
		}),
	})
	if err != nil {
		return ISAPICommand{}, activity.Event{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ISAPICommand{}, activity.Event{}, fmt.Errorf("commit queue ISAPI command: %w", err)
	}
	return command, stored, nil
}

// ClaimISAPICommands selects at most limit queued commands for the terminal.
// A command changes to sent in the same transaction that records its audit
// entry, before the PushSDK response is written, matching the vendor command
// handler's documented queue semantics.
func (s *Store) ClaimISAPICommands(ctx context.Context, terminalSerial string, limit int) ([]ISAPICommandDelivery, []activity.Event, error) {
	if limit < 1 || limit > 20 {
		return nil, nil, errors.New("ISAPI command delivery limit must be 1..20")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin claim ISAPI commands: %w", err)
	}
	defer tx.Rollback(ctx)

	activities, err := expireQueuedISAPICommands(ctx, tx, terminalSerial)
	if err != nil {
		return nil, nil, err
	}
	rows, err := tx.Query(ctx, `WITH selected AS (
		SELECT uuid, created_at
		FROM isapi_commands
		WHERE terminal_serial_number = $1 AND status = 'queued' AND expires_at > now()
		ORDER BY created_at ASC, uuid ASC
		LIMIT $2
		FOR UPDATE SKIP LOCKED
	), updated AS (
		UPDATE isapi_commands command
		SET status = 'sent', sent_at = now()
		FROM selected
		WHERE command.uuid = selected.uuid
		RETURNING command.uuid, command.method, command.url, command.data_format, command.request_data, command.sent_at
	)
	SELECT updated.uuid, updated.method, updated.url, updated.data_format, updated.request_data, updated.sent_at
	FROM updated JOIN selected ON selected.uuid = updated.uuid
	ORDER BY selected.created_at ASC, selected.uuid ASC`, terminalSerial, limit)
	if err != nil {
		return nil, nil, fmt.Errorf("claim queued ISAPI commands: %w", err)
	}
	defer rows.Close()
	deliveries := []ISAPICommandDelivery{}
	for rows.Next() {
		var delivery ISAPICommandDelivery
		var sentAt time.Time
		if err := rows.Scan(&delivery.UUID, &delivery.Method, &delivery.URL, &delivery.DataFormat, &delivery.Data, &sentAt); err != nil {
			return nil, nil, fmt.Errorf("scan claimed ISAPI command: %w", err)
		}
		deliveries = append(deliveries, delivery)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, nil, fmt.Errorf("iterate claimed ISAPI commands: %w", err)
	}
	rows.Close()
	for _, delivery := range deliveries {
		stored, err := insertGatewayActivity(ctx, tx, activity.Event{
			Kind:     activity.KindPushSDKCommandSent,
			Terminal: terminalSerial,
			Message:  "gateway sent an ISAPI command to the terminal",
			Fields: map[string]any{
				"commandId":  delivery.UUID,
				"method":     delivery.Method,
				"url":        delivery.URL,
				"dataFormat": delivery.DataFormat,
			},
		})
		if err != nil {
			return nil, nil, err
		}
		activities = append(activities, stored)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit claimed ISAPI commands: %w", err)
	}
	return deliveries, activities, nil
}

// CompleteISAPICommands correlates a CommandResult batch with commands sent to
// the same terminal. A byte-for-byte duplicate result is idempotent; a changed
// result or an unknown command is rejected rather than guessed or merged.
func (s *Store) CompleteISAPICommands(ctx context.Context, terminalSerial string, results []ISAPICommandResult) ([]activity.Event, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("begin complete ISAPI commands: %w", err)
	}
	defer tx.Rollback(ctx)
	activities, err := expireQueuedISAPICommands(ctx, tx, terminalSerial)
	if err != nil {
		return nil, false, err
	}
	for _, result := range results {
		var status, requestDataFormat string
		var responseFormat *string
		var responseFormatDeclared *bool
		var responseBase64 *string
		err := tx.QueryRow(ctx, `SELECT status, data_format, response_data_format, response_data_format_declared, response_data_base64
			FROM isapi_commands WHERE uuid = $1 AND terminal_serial_number = $2 FOR UPDATE`, result.UUID, terminalSerial).Scan(&status, &requestDataFormat, &responseFormat, &responseFormatDeclared, &responseBase64)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, fmt.Errorf("command result UUID %s is not a sent command for this terminal", result.UUID)
		}
		if err != nil {
			return nil, false, fmt.Errorf("load ISAPI command result %s: %w", result.UUID, err)
		}
		switch status {
		case "sent":
			if result.DataFormat == nil && (requestDataFormat != "noData" || result.DataBase64 != "") {
				return nil, false, fmt.Errorf("command result UUID %s omits dataFormat outside the documented noData form", result.UUID)
			}
			var completedAt time.Time
			if result.DataFormat == nil {
				err = tx.QueryRow(ctx, `UPDATE isapi_commands
					SET status = 'completed', completed_at = now(), response_data_format = NULL,
						response_data_format_declared = FALSE, response_data_base64 = $2
					WHERE uuid = $1
					RETURNING completed_at`, result.UUID, result.DataBase64).Scan(&completedAt)
			} else {
				err = tx.QueryRow(ctx, `UPDATE isapi_commands
					SET status = 'completed', completed_at = now(), response_data_format = $2,
						response_data_format_declared = TRUE, response_data_base64 = $3
					WHERE uuid = $1
					RETURNING completed_at`, result.UUID, *result.DataFormat, result.DataBase64).Scan(&completedAt)
			}
			if err != nil {
				return nil, false, fmt.Errorf("complete ISAPI command %s: %w", result.UUID, err)
			}
			stored, err := insertGatewayActivity(ctx, tx, activity.Event{
				Kind:     activity.KindPushSDKCommandCompleted,
				Terminal: terminalSerial,
				Message:  "terminal returned an ISAPI command result",
				Fields:   commandResultActivityFields(result),
			})
			if err != nil {
				return nil, false, err
			}
			activities = append(activities, stored)
		case "completed":
			if result.DataFormat == nil {
				if requestDataFormat != "noData" || result.DataBase64 != "" || responseFormat != nil || responseFormatDeclared == nil || *responseFormatDeclared || responseBase64 == nil || *responseBase64 != "" {
					return nil, false, fmt.Errorf("command result UUID %s conflicts with its completed result", result.UUID)
				}
				continue
			}
			if responseFormat == nil || responseFormatDeclared == nil || !*responseFormatDeclared || *responseFormat != *result.DataFormat || responseBase64 == nil || *responseBase64 != result.DataBase64 {
				return nil, false, fmt.Errorf("command result UUID %s conflicts with its completed result", result.UUID)
			}
		default:
			return nil, false, fmt.Errorf("command result UUID %s is in %s state, not sent", result.UUID, status)
		}
	}
	var pending bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM isapi_commands
		WHERE terminal_serial_number = $1 AND status = 'queued' AND expires_at > now()
	)`, terminalSerial).Scan(&pending); err != nil {
		return nil, false, fmt.Errorf("check pending ISAPI commands: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, false, fmt.Errorf("commit completed ISAPI commands: %w", err)
	}
	return activities, pending, nil
}

func (s *Store) QueryISAPICommands(ctx context.Context, terminalSerial string, limit, offset int) (ISAPICommandPage, error) {
	var page ISAPICommandPage
	if err := s.expireOverdueISAPICommands(ctx); err != nil {
		return page, err
	}
	var countQuery, listQuery string
	arguments := []any{limit, offset}
	if terminalSerial == "" {
		countQuery = `SELECT count(*) FROM isapi_commands`
		listQuery = `SELECT uuid, terminal_serial_number, created_by_username, method, url, data_format, status,
			created_at, expires_at, sent_at, completed_at, response_data_format, response_data_format_declared, response_data_base64 IS NOT NULL
			FROM isapi_commands ORDER BY created_at DESC, uuid DESC LIMIT $1 OFFSET $2`
	} else {
		countQuery = `SELECT count(*) FROM isapi_commands WHERE terminal_serial_number = $1`
		listQuery = `SELECT uuid, terminal_serial_number, created_by_username, method, url, data_format, status,
			created_at, expires_at, sent_at, completed_at, response_data_format, response_data_format_declared, response_data_base64 IS NOT NULL
			FROM isapi_commands WHERE terminal_serial_number = $1 ORDER BY created_at DESC, uuid DESC LIMIT $2 OFFSET $3`
		arguments = []any{terminalSerial, limit, offset}
	}
	if terminalSerial == "" {
		if err := s.pool.QueryRow(ctx, countQuery).Scan(&page.Total); err != nil {
			return page, err
		}
	} else if err := s.pool.QueryRow(ctx, countQuery, terminalSerial).Scan(&page.Total); err != nil {
		return page, err
	}
	rows, err := s.pool.Query(ctx, listQuery, arguments...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	page.Commands = []ISAPICommand{}
	for rows.Next() {
		var command ISAPICommand
		if err := scanISAPICommand(rows, &command); err != nil {
			return page, err
		}
		page.Commands = append(page.Commands, command)
	}
	return page, rows.Err()
}

func (s *Store) ISAPICommandPayload(ctx context.Context, uuid string) (ISAPICommandPayload, bool, error) {
	var payload ISAPICommandPayload
	if err := s.expireOverdueISAPICommands(ctx); err != nil {
		return payload, false, err
	}
	var requestData []byte
	err := s.pool.QueryRow(ctx, `SELECT uuid, terminal_serial_number, created_by_username, method, url, data_format, status,
		created_at, expires_at, sent_at, completed_at, response_data_format, response_data_format_declared, response_data_base64 IS NOT NULL,
		request_data, response_data_base64
		FROM isapi_commands WHERE uuid = $1`, uuid).Scan(
		&payload.UUID, &payload.TerminalSerialNumber, &payload.CreatedByUsername, &payload.Method, &payload.URL,
		&payload.DataFormat, &payload.Status, &payload.CreatedAt, &payload.ExpiresAt, &payload.SentAt,
		&payload.CompletedAt, &payload.ResponseDataFormat, &payload.ResponseDataFormatDeclared, &payload.ResponseDataAvailable, &requestData, &payload.ResponseDataBase64)
	if errors.Is(err, pgx.ErrNoRows) {
		return payload, false, nil
	}
	if err != nil {
		return payload, false, err
	}
	payload.RequestDataBase64 = base64.StdEncoding.EncodeToString(requestData)
	return payload, true, nil
}

func (s *Store) AdminIdentityForSession(ctx context.Context, tokenHash []byte) (AdminIdentity, bool, error) {
	var identity AdminIdentity
	err := s.pool.QueryRow(ctx, `SELECT administrator.id, administrator.username
		FROM admin_sessions session
		JOIN admin_users administrator ON administrator.id = session.admin_user_id
		WHERE session.token_hash = $1 AND session.expires_at > now()`, tokenHash).Scan(&identity.ID, &identity.Username)
	if errors.Is(err, pgx.ErrNoRows) {
		return AdminIdentity{}, false, nil
	}
	if err != nil {
		return AdminIdentity{}, false, err
	}
	return identity, true, nil
}

// expireOverdueISAPICommands makes the command's explicitly selected expiry a
// durable status transition even if the terminal has not polled again. Queue
// claiming repeats the same predicate in its transaction, so an overdue
// command is never delivered.
func (s *Store) expireOverdueISAPICommands(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `UPDATE isapi_commands
		SET status = 'expired', completed_at = now()
		WHERE status = 'queued' AND expires_at <= now()`)
	if err != nil {
		return fmt.Errorf("expire overdue ISAPI commands: %w", err)
	}
	return nil
}

func expireQueuedISAPICommands(ctx context.Context, tx pgx.Tx, terminalSerial string) ([]activity.Event, error) {
	rows, err := tx.Query(ctx, `UPDATE isapi_commands
		SET status = 'expired', completed_at = now()
		WHERE terminal_serial_number = $1 AND status = 'queued' AND expires_at <= now()
		RETURNING uuid, method, url, data_format`, terminalSerial)
	if err != nil {
		return nil, fmt.Errorf("expire queued ISAPI commands: %w", err)
	}
	defer rows.Close()
	commands := []ISAPICommand{}
	for rows.Next() {
		var command ISAPICommand
		if err := rows.Scan(&command.UUID, &command.Method, &command.URL, &command.DataFormat); err != nil {
			return nil, fmt.Errorf("scan expired ISAPI command: %w", err)
		}
		commands = append(commands, command)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate expired ISAPI commands: %w", err)
	}
	rows.Close()
	activities := []activity.Event{}
	for _, command := range commands {
		stored, err := insertGatewayActivity(ctx, tx, activity.Event{
			Kind:     activity.KindPushSDKCommandExpired,
			Terminal: terminalSerial,
			Message:  "queued ISAPI command expired before terminal delivery",
			Fields: commandActivityFields(command, map[string]any{
				"status": "expired",
			}),
		})
		if err != nil {
			return nil, err
		}
		activities = append(activities, stored)
	}
	return activities, nil
}

func scanISAPICommand(row interface{ Scan(...any) error }, command *ISAPICommand) error {
	return row.Scan(&command.UUID, &command.TerminalSerialNumber, &command.CreatedByUsername, &command.Method, &command.URL,
		&command.DataFormat, &command.Status, &command.CreatedAt, &command.ExpiresAt, &command.SentAt,
		&command.CompletedAt, &command.ResponseDataFormat, &command.ResponseDataFormatDeclared, &command.ResponseDataAvailable)
}

func commandResultActivityFields(result ISAPICommandResult) map[string]any {
	fields := map[string]any{
		"commandId":          result.UUID,
		"dataFormatDeclared": result.DataFormat != nil,
	}
	if result.DataFormat != nil {
		fields["dataFormat"] = *result.DataFormat
	}
	return fields
}

func commandActivityFields(command ISAPICommand, additional map[string]any) map[string]any {
	fields := map[string]any{
		"commandId":  command.UUID,
		"method":     command.Method,
		"url":        command.URL,
		"dataFormat": command.DataFormat,
	}
	for key, value := range additional {
		fields[key] = value
	}
	return fields
}

func newISAPICommandUUID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate ISAPI command UUID: %w", err)
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(bytes)
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32], nil
}
