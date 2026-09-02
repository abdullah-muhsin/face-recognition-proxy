package store

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/itplus/pushsdk-gateway/internal/accesscontrol"
	"github.com/itplus/pushsdk-gateway/internal/activity"
	"github.com/jackc/pgx/v5"
)

const accessEventSyncCommandTTL = 5 * time.Minute

var (
	ErrAccessEventSyncTerminalNotFound = errors.New("retained event sync terminal is not registered")
	ErrAccessEventSyncTerminalOffline  = errors.New("retained event sync terminal is offline")
	ErrAccessEventSyncInProgress       = errors.New("a retained event sync is already active for this terminal")
)

// AccessEventSyncRun is the durable state of one complete retained-event
// reconciliation. The run first reads the terminal clock, then pages the
// vendor archive in its exact response order.
type AccessEventSyncRun struct {
	UUID                 string     `json:"uuid"`
	TerminalSerialNumber string     `json:"terminalSerialNumber"`
	CreatedByUsername    string     `json:"createdByUsername"`
	Status               string     `json:"status"`
	SearchStartedAt      *time.Time `json:"searchStartedAt"`
	SearchEndedAt        *time.Time `json:"searchEndedAt"`
	TotalMatches         *int       `json:"totalMatches"`
	PagesCompleted       int        `json:"pagesCompleted"`
	RecordsImported      int        `json:"recordsImported"`
	RecordsDuplicate     int        `json:"recordsDuplicate"`
	Failure              *string    `json:"failure"`
	CreatedAt            time.Time  `json:"createdAt"`
	CompletedAt          *time.Time `json:"completedAt"`
}

type accessEventSyncCommand struct {
	RunUUID        string
	Terminal       string
	CreatedByID    *int64
	CreatedByName  string
	SearchID       string
	Status         string
	Phase          string
	Position       *int
	SearchStarted  *time.Time
	SearchEnded    *time.Time
	TotalMatches   *int
	PagesCompleted int
	Imported       int
	Duplicate      int
}

type retainedEventRecord struct {
	Source      []byte
	Fingerprint []byte
	Projection  accesscontrol.Projection
}

// QueueAccessEventSync starts a terminal-specific retained-event sync. The
// initial System/time command is intentionally separate from the AcsEvent
// search so search bounds are determined by the terminal, not by the gateway
// host or browser.
func (s *Store) QueueAccessEventSync(ctx context.Context, terminal string, identity AdminIdentity) (AccessEventSyncRun, []activity.Event, error) {
	if identity.ID == 0 || identity.Username == "" {
		return AccessEventSyncRun{}, nil, errors.New("retained event sync administrator identity is required")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return AccessEventSyncRun{}, nil, fmt.Errorf("begin retained event sync: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, terminal); err != nil {
		return AccessEventSyncRun{}, nil, fmt.Errorf("lock retained event sync terminal: %w", err)
	}
	expiryActivities, err := expireOverdueISAPICommandsTx(ctx, tx, &terminal)
	if err != nil {
		return AccessEventSyncRun{}, nil, err
	}
	var terminalStatus string
	err = tx.QueryRow(ctx, `SELECT connection_status FROM terminals WHERE serial_number = $1 FOR KEY SHARE`, terminal).Scan(&terminalStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return AccessEventSyncRun{}, nil, ErrAccessEventSyncTerminalNotFound
	}
	if err != nil {
		return AccessEventSyncRun{}, nil, fmt.Errorf("load retained event sync terminal: %w", err)
	}
	if terminalStatus != "online" {
		return AccessEventSyncRun{}, nil, ErrAccessEventSyncTerminalOffline
	}
	var active bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM access_event_sync_runs
		WHERE terminal_serial_number = $1 AND status IN ('awaiting_time', 'running')
	)`, terminal).Scan(&active); err != nil {
		return AccessEventSyncRun{}, nil, fmt.Errorf("check retained event sync state: %w", err)
	}
	if active {
		return AccessEventSyncRun{}, nil, ErrAccessEventSyncInProgress
	}
	runUUID, err := newISAPICommandUUID()
	if err != nil {
		return AccessEventSyncRun{}, nil, err
	}
	run := AccessEventSyncRun{
		UUID:                 runUUID,
		TerminalSerialNumber: terminal,
		CreatedByUsername:    identity.Username,
		Status:               "awaiting_time",
	}
	if err := tx.QueryRow(ctx, `INSERT INTO access_event_sync_runs
		(uuid, terminal_serial_number, created_by_admin_user_id, created_by_username, search_id, status)
		VALUES ($1,$2,$3,$4,$5,'awaiting_time')
		RETURNING created_at`, run.UUID, run.TerminalSerialNumber, identity.ID, run.CreatedByUsername, run.UUID).Scan(&run.CreatedAt); err != nil {
		return AccessEventSyncRun{}, nil, fmt.Errorf("insert retained event sync: %w", err)
	}
	command, commandActivity, err := queueISAPICommandTx(ctx, tx, NewISAPICommand{
		TerminalSerialNumber: terminal,
		CreatedBy:            identity,
		Method:               "GET",
		URL:                  "/ISAPI/System/time",
		DataFormat:           "noData",
		Data:                 []byte{},
		ExpiresAt:            time.Now().UTC().Add(accessEventSyncCommandTTL),
	})
	if err != nil {
		return AccessEventSyncRun{}, nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO access_event_sync_commands (command_uuid, sync_run_uuid, phase)
		VALUES ($1,$2,'time')`, command.UUID, run.UUID); err != nil {
		return AccessEventSyncRun{}, nil, fmt.Errorf("link retained event sync time command: %w", err)
	}
	queuedActivity, err := insertGatewayActivity(ctx, tx, activity.Event{
		Kind:     activity.KindAdminAccessEventSyncQueued,
		Terminal: terminal,
		Message:  "administrator queued a retained access-event sync",
		Fields: map[string]any{
			"syncRunId": run.UUID,
			"issuedBy":  identity.Username,
		},
	})
	if err != nil {
		return AccessEventSyncRun{}, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return AccessEventSyncRun{}, nil, fmt.Errorf("commit retained event sync: %w", err)
	}
	return run, append(expiryActivities, commandActivity, queuedActivity), nil
}

func (s *Store) AccessEventSyncRun(ctx context.Context, uuid string) (AccessEventSyncRun, bool, error) {
	var run AccessEventSyncRun
	err := s.pool.QueryRow(ctx, `SELECT uuid, terminal_serial_number, created_by_username, status,
		search_started_at, search_ended_at, total_matches, pages_completed, records_imported,
		records_duplicate, failure, created_at, completed_at
		FROM access_event_sync_runs WHERE uuid = $1`, uuid).Scan(
		&run.UUID, &run.TerminalSerialNumber, &run.CreatedByUsername, &run.Status,
		&run.SearchStartedAt, &run.SearchEndedAt, &run.TotalMatches, &run.PagesCompleted,
		&run.RecordsImported, &run.RecordsDuplicate, &run.Failure, &run.CreatedAt, &run.CompletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return run, false, nil
	}
	if err != nil {
		return run, false, fmt.Errorf("load retained event sync: %w", err)
	}
	return run, true, nil
}

// completeAccessEventSyncCommand is called in the same transaction that stores
// an exact CommandResult. A malformed result changes the run to failed and is
// retained in command audit; it is not interpreted, retried, or replaced.
func completeAccessEventSyncCommand(ctx context.Context, tx pgx.Tx, terminal string, result ISAPICommandResult, completedAt time.Time) ([]activity.Event, error) {
	var command accessEventSyncCommand
	err := tx.QueryRow(ctx, `SELECT link.sync_run_uuid, run.terminal_serial_number, run.created_by_admin_user_id,
		run.created_by_username, run.search_id, run.status, link.phase, link.search_result_position,
		run.search_started_at, run.search_ended_at, run.total_matches, run.pages_completed,
		run.records_imported, run.records_duplicate
		FROM access_event_sync_commands link
		JOIN access_event_sync_runs run ON run.uuid = link.sync_run_uuid
		WHERE link.command_uuid = $1 FOR UPDATE OF run`, result.UUID).Scan(
		&command.RunUUID, &command.Terminal, &command.CreatedByID, &command.CreatedByName,
		&command.SearchID, &command.Status, &command.Phase, &command.Position,
		&command.SearchStarted, &command.SearchEnded, &command.TotalMatches,
		&command.PagesCompleted, &command.Imported, &command.Duplicate,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load retained event sync command: %w", err)
	}
	if command.Terminal != terminal {
		return nil, fmt.Errorf("retained event sync command %s is linked to another terminal", result.UUID)
	}
	if command.Status != "awaiting_time" && command.Status != "running" {
		return nil, fmt.Errorf("retained event sync run %s is not active", command.RunUUID)
	}
	source, err := base64.StdEncoding.DecodeString(result.DataBase64)
	if err != nil {
		return failAccessEventSync(ctx, tx, command, "terminal command result is not decodable Base64")
	}
	switch command.Phase {
	case "time":
		return completeAccessEventSyncTime(ctx, tx, command, source)
	case "page":
		if result.DataFormat != nil && *result.DataFormat != "jsonData" {
			return failAccessEventSync(ctx, tx, command, "AcsEvent result declared a format other than jsonData")
		}
		return completeAccessEventSyncPage(ctx, tx, command, result.UUID, source, completedAt)
	default:
		return nil, fmt.Errorf("retained event sync command %s has unknown phase %q", result.UUID, command.Phase)
	}
}

func completeAccessEventSyncTime(ctx context.Context, tx pgx.Tx, command accessEventSyncCommand, source []byte) ([]activity.Event, error) {
	if command.Status != "awaiting_time" {
		return failAccessEventSync(ctx, tx, command, "System/time result arrived after retained-event search began")
	}
	terminalNow, err := accesscontrol.ParseTerminalTime(source)
	if err != nil {
		return failAccessEventSync(ctx, tx, command, "System/time result is not the documented terminal time XML")
	}
	request, err := accesscontrol.NewRetainedEventSearchRequest(command.SearchID, 0, terminalNow)
	if err != nil {
		return failAccessEventSync(ctx, tx, command, "terminal time cannot bound the retained-event search")
	}
	if command.CreatedByID == nil {
		return failAccessEventSync(ctx, tx, command, "sync administrator no longer exists before event search delivery")
	}
	if _, err := tx.Exec(ctx, `UPDATE access_event_sync_runs
		SET status = 'running', search_started_at = $2, search_ended_at = $3
		WHERE uuid = $1`, command.RunUUID,
		time.Date(2000, time.January, 1, 0, 0, 0, 0, terminalNow.Location()), terminalNow); err != nil {
		return nil, fmt.Errorf("start retained event search: %w", err)
	}
	next, commandActivity, err := queueISAPICommandTx(ctx, tx, NewISAPICommand{
		TerminalSerialNumber: command.Terminal,
		CreatedBy:            AdminIdentity{ID: *command.CreatedByID, Username: command.CreatedByName},
		Method:               "POST",
		URL:                  accesscontrol.RetainedEventSearchURL,
		DataFormat:           "jsonData",
		Data:                 request,
		ExpiresAt:            time.Now().UTC().Add(accessEventSyncCommandTTL),
	})
	if err != nil {
		return failAccessEventSync(ctx, tx, command, "could not queue the retained-event search command")
	}
	if _, err := tx.Exec(ctx, `INSERT INTO access_event_sync_commands
		(command_uuid, sync_run_uuid, phase, search_result_position) VALUES ($1,$2,'page',0)`, next.UUID, command.RunUUID); err != nil {
		return nil, fmt.Errorf("link first retained event page command: %w", err)
	}
	startedActivity, err := insertGatewayActivity(ctx, tx, activity.Event{
		Kind:     activity.KindDeviceAccessEventSyncStarted,
		Terminal: command.Terminal,
		Message:  "gateway started the terminal retained access-event search",
		Fields: map[string]any{
			"syncRunId": command.RunUUID,
			"pageSize":  accesscontrol.RetainedEventPageSize,
		},
	})
	if err != nil {
		return nil, err
	}
	return []activity.Event{commandActivity, startedActivity}, nil
}

func completeAccessEventSyncPage(ctx context.Context, tx pgx.Tx, command accessEventSyncCommand, commandUUID string, source []byte, completedAt time.Time) ([]activity.Event, error) {
	if command.Status != "running" || command.Position == nil || command.SearchStarted == nil || command.SearchEnded == nil {
		return failAccessEventSync(ctx, tx, command, "AcsEvent result does not match an active retained-event page")
	}
	page, err := accesscontrol.ParseRetainedEventPage(source)
	if err != nil {
		return failAccessEventSync(ctx, tx, command, "AcsEvent result is not the documented pagination response")
	}
	if page.SearchID != command.SearchID {
		return failAccessEventSync(ctx, tx, command, "AcsEvent result searchID does not match the sync run")
	}
	if command.TotalMatches != nil && page.TotalMatches != *command.TotalMatches {
		return failAccessEventSync(ctx, tx, command, "AcsEvent result totalMatches changed during the sync run")
	}
	nextPosition := *command.Position + page.NumOfMatches
	if nextPosition > page.TotalMatches {
		return failAccessEventSync(ctx, tx, command, "AcsEvent result exceeds totalMatches")
	}
	switch page.ResponseStatus {
	case "NO MATCH":
		if *command.Position != 0 {
			return failAccessEventSync(ctx, tx, command, "AcsEvent NO MATCH result appeared after the first page")
		}
	case "MORE":
		if page.NumOfMatches == 0 || nextPosition >= page.TotalMatches {
			return failAccessEventSync(ctx, tx, command, "AcsEvent MORE result does not advance to another page")
		}
	case "OK":
		if nextPosition != page.TotalMatches {
			return failAccessEventSync(ctx, tx, command, "AcsEvent OK result does not complete totalMatches")
		}
	}
	records := make([]retainedEventRecord, 0, len(page.InfoList))
	for _, raw := range page.InfoList {
		projection, err := accesscontrol.ExtractRetainedEvent(raw)
		if err != nil {
			return failAccessEventSync(ctx, tx, command, "AcsEvent result contains an undocumented record")
		}
		digest := sha256.Sum256(raw)
		records = append(records, retainedEventRecord{Source: raw, Fingerprint: digest[:], Projection: projection})
	}
	inserted, duplicate, err := insertRetainedAccessEvents(ctx, tx, command, commandUUID, records)
	if err != nil {
		return nil, err
	}
	pages := command.PagesCompleted + 1
	imported := command.Imported + inserted
	duplicates := command.Duplicate + duplicate
	pageActivity, err := insertGatewayActivity(ctx, tx, activity.Event{
		Kind:     activity.KindDeviceAccessEventSyncPageCaptured,
		Terminal: command.Terminal,
		Message:  "gateway retained one terminal access-event archive page",
		Fields: map[string]any{
			"syncRunId":        command.RunUUID,
			"searchPosition":   *command.Position,
			"recordsReturned":  page.NumOfMatches,
			"recordsImported":  inserted,
			"recordsDuplicate": duplicate,
			"totalMatches":     page.TotalMatches,
		},
	})
	if err != nil {
		return nil, err
	}
	if page.ResponseStatus == "MORE" {
		if command.CreatedByID == nil {
			return failAccessEventSync(ctx, tx, command, "sync administrator no longer exists before next page delivery")
		}
		request, err := accesscontrol.NewRetainedEventSearchRequest(command.SearchID, nextPosition, *command.SearchEnded)
		if err != nil {
			return nil, fmt.Errorf("build next retained event search page: %w", err)
		}
		if _, err := tx.Exec(ctx, `UPDATE access_event_sync_runs
			SET total_matches = $2, pages_completed = $3, records_imported = $4, records_duplicate = $5
			WHERE uuid = $1`, command.RunUUID, page.TotalMatches, pages, imported, duplicates); err != nil {
			return nil, fmt.Errorf("advance retained event sync: %w", err)
		}
		next, commandActivity, err := queueISAPICommandTx(ctx, tx, NewISAPICommand{
			TerminalSerialNumber: command.Terminal,
			CreatedBy:            AdminIdentity{ID: *command.CreatedByID, Username: command.CreatedByName},
			Method:               "POST",
			URL:                  accesscontrol.RetainedEventSearchURL,
			DataFormat:           "jsonData",
			Data:                 request,
			ExpiresAt:            time.Now().UTC().Add(accessEventSyncCommandTTL),
		})
		if err != nil {
			return failAccessEventSync(ctx, tx, command, "could not queue the next retained-event page")
		}
		if _, err := tx.Exec(ctx, `INSERT INTO access_event_sync_commands
			(command_uuid, sync_run_uuid, phase, search_result_position) VALUES ($1,$2,'page',$3)`, next.UUID, command.RunUUID, nextPosition); err != nil {
			return nil, fmt.Errorf("link next retained event page command: %w", err)
		}
		return []activity.Event{pageActivity, commandActivity}, nil
	}
	if imported+duplicates != page.TotalMatches {
		return failAccessEventSync(ctx, tx, command, "retained-event totals do not account for every device record")
	}
	if _, err := tx.Exec(ctx, `UPDATE access_event_sync_runs
		SET status = 'completed', total_matches = $2, pages_completed = $3,
			records_imported = $4, records_duplicate = $5, completed_at = $6
		WHERE uuid = $1`, command.RunUUID, page.TotalMatches, pages, imported, duplicates, completedAt); err != nil {
		return nil, fmt.Errorf("complete retained event sync: %w", err)
	}
	completedActivity, err := insertGatewayActivity(ctx, tx, activity.Event{
		Kind:     activity.KindDeviceAccessEventSyncCompleted,
		Terminal: command.Terminal,
		Message:  "gateway completed the terminal retained access-event sync",
		Fields: map[string]any{
			"syncRunId":        command.RunUUID,
			"totalMatches":     page.TotalMatches,
			"recordsImported":  imported,
			"recordsDuplicate": duplicates,
		},
	})
	if err != nil {
		return nil, err
	}
	return []activity.Event{pageActivity, completedActivity}, nil
}

func insertRetainedAccessEvents(ctx context.Context, tx pgx.Tx, command accessEventSyncCommand, commandUUID string, records []retainedEventRecord) (int, int, error) {
	inserted := 0
	for _, record := range records {
		projection := record.Projection
		var faceHeight, faceWidth, faceX, faceY *string
		if projection.FaceRect != nil {
			faceHeight = projection.FaceRect.Height
			faceWidth = projection.FaceRect.Width
			faceX = projection.FaceRect.X
			faceY = projection.FaceRect.Y
		}
		var id int64
		err := tx.QueryRow(ctx, `INSERT INTO retained_access_events
			(terminal_serial_number, source_sync_run_uuid, source_command_uuid, source_record, source_sha256,
			 major_event_type, sub_event_type, occurred_at, employee_number, employee_name, card_number,
			 card_reader_number, door_number, event_serial_number, user_type, current_verify_mode, mask,
			 face_rect_height, face_rect_width, face_rect_x, face_rect_y)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)
			ON CONFLICT (terminal_serial_number, source_sha256) DO NOTHING
			RETURNING id`, command.Terminal, command.RunUUID, commandUUID, record.Source, record.Fingerprint,
			projection.MajorEventType, projection.SubEventType, projection.OccurredAt, projection.EmployeeNumber,
			projection.EmployeeName, projection.CardNumber, projection.CardReaderNumber, projection.DoorNumber,
			projection.EventSerialNumber, projection.UserType, projection.CurrentVerifyMode, projection.Mask,
			faceHeight, faceWidth, faceX, faceY).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return 0, 0, fmt.Errorf("retain access-event record: %w", err)
		}
		inserted++
	}
	return inserted, len(records) - inserted, nil
}

func failAccessEventSync(ctx context.Context, tx pgx.Tx, command accessEventSyncCommand, reason string) ([]activity.Event, error) {
	var terminal string
	err := tx.QueryRow(ctx, `UPDATE access_event_sync_runs
		SET status = 'failed', failure = $2, completed_at = now()
		WHERE uuid = $1 AND status IN ('awaiting_time', 'running')
		RETURNING terminal_serial_number`, command.RunUUID, reason).Scan(&terminal)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fail retained event sync: %w", err)
	}
	stored, err := insertGatewayActivity(ctx, tx, activity.Event{
		Kind:     activity.KindDeviceAccessEventSyncFailed,
		Terminal: terminal,
		Message:  "gateway stopped a retained access-event sync",
		Fields: map[string]any{
			"syncRunId": command.RunUUID,
			"reason":    reason,
		},
	})
	if err != nil {
		return nil, err
	}
	return []activity.Event{stored}, nil
}

func failExpiredAccessEventSyncCommand(ctx context.Context, tx pgx.Tx, commandUUID string, delivered bool) ([]activity.Event, error) {
	var runUUID string
	err := tx.QueryRow(ctx, `SELECT sync_run_uuid FROM access_event_sync_commands WHERE command_uuid = $1`, commandUUID).Scan(&runUUID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load retained event sync for expired command: %w", err)
	}
	reason := "retained-event command expired before terminal delivery"
	if delivered {
		reason = "terminal did not return the retained-event command result before the command deadline"
	}
	return failAccessEventSync(ctx, tx, accessEventSyncCommand{RunUUID: runUUID}, reason)
}
