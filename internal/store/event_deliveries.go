package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/itplus/pushsdk-gateway/internal/activity"
	"github.com/jackc/pgx/v5"
)

var ErrDeliveryRouteBusy = errors.New("pending deliveries must finish before changing the endpoint or key")

var ErrDeliveryRouteUnfinished = errors.New("finish or resolve unfinished deliveries before removing this destination")

type DeliveryRoute struct {
	TerminalSerialNumber string `json:"terminalSerialNumber"`
	EndpointURL          string `json:"endpointUrl"`
	SigningKeyID         string `json:"signingKeyId"`
	Enabled              bool   `json:"enabled"`
}

type EventDelivery struct {
	ID                   int64      `json:"id"`
	DeviceEventID        *int64     `json:"deviceEventId"`
	RetainedEventID      *int64     `json:"retainedEventId"`
	BackfillID           *int64     `json:"backfillId"`
	TerminalSerialNumber string     `json:"terminalSerialNumber"`
	EndpointURL          string     `json:"endpointUrl"`
	SigningKeyID         string     `json:"signingKeyId"`
	Status               string     `json:"status"`
	Attempts             int        `json:"attempts"`
	NextAttemptAt        time.Time  `json:"nextAttemptAt"`
	LastHTTPStatus       *int       `json:"lastHttpStatus"`
	LastError            *string    `json:"lastError"`
	CreatedAt            time.Time  `json:"createdAt"`
	DeliveredAt          *time.Time `json:"deliveredAt"`
	Body                 []byte     `json:"-"`
}

// EventMessage is the complete, versioned contract. Media and credentials never
// leave the source archive. Identifiers retain their exact vendor spelling.
type EventMessage struct {
	SchemaVersion        int        `json:"schemaVersion"`
	Source               string     `json:"source,omitempty"`
	EventID              string     `json:"eventId"`
	TerminalSerialNumber string     `json:"terminalSerialNumber"`
	PushSDKSerial        string     `json:"pushSdkSerial"`
	MajorEventType       int        `json:"majorEventType"`
	SubEventType         int        `json:"subEventType"`
	EventState           *string    `json:"eventState"`
	OccurredAt           *time.Time `json:"occurredAt"`
	EmployeeNumber       *string    `json:"employeeNoString"`
	EmployeeName         *string    `json:"employeeName"`
	EventSerialNumber    *int64     `json:"eventSerialNumber"`
}

func enqueueEventDelivery(ctx context.Context, tx pgx.Tx, eventID int64) error {
	var route DeliveryRoute
	var message EventMessage
	message.SchemaVersion = 2
	message.Source = "pushsdk"
	err := tx.QueryRow(ctx, `SELECT r.terminal_serial_number,r.endpoint_url,r.signing_key_id,r.enabled,
 d.vendor_event_id,t.pushsdk_serial,p.major_event_type,p.sub_event_type,p.event_state,p.occurred_at,
 p.employee_number,p.employee_name,p.event_serial_number
 FROM device_events d JOIN access_event_projections p ON p.device_event_id=d.id
 JOIN terminals t ON t.serial_number=d.terminal_serial_number
 JOIN event_delivery_routes r ON r.terminal_serial_number=d.terminal_serial_number
 WHERE d.id=$1 AND r.enabled
 AND p.major_event_type=5 AND p.sub_event_type=75 AND p.event_state='active'
 FOR SHARE OF r`, eventID).Scan(&route.TerminalSerialNumber, &route.EndpointURL, &route.SigningKeyID, &route.Enabled,
		&message.EventID, &message.PushSDKSerial, &message.MajorEventType, &message.SubEventType, &message.EventState, &message.OccurredAt,
		&message.EmployeeNumber, &message.EmployeeName, &message.EventSerialNumber)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("select event delivery route: %w", err)
	}
	message.TerminalSerialNumber = route.TerminalSerialNumber
	body, err := json.Marshal(message)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO event_deliveries (device_event_id,terminal_serial_number,endpoint_url,signing_key_id,request_body) VALUES ($1,$2,$3,$4,$5)`, eventID, route.TerminalSerialNumber, route.EndpointURL, route.SigningKeyID, body)
	return err
}

func (s *Store) SaveDeliveryRoute(ctx context.Context, route DeliveryRoute, actorID int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Event ingestion takes the terminal row lock first too; route replacement
	// therefore cannot race an event becoming eligible under the old destination.
	var serial string
	if err = tx.QueryRow(ctx, `SELECT serial_number FROM terminals WHERE serial_number=$1 FOR UPDATE`, route.TerminalSerialNumber).Scan(&serial); err != nil {
		return err
	}
	var blocked bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM event_delivery_routes r WHERE r.terminal_serial_number=$1
 AND (r.endpoint_url<>$2 OR r.signing_key_id<>$3)
 AND EXISTS(SELECT 1 FROM event_deliveries d WHERE d.terminal_serial_number=$1 AND d.status IN ('pending','sending')))`, serial, route.EndpointURL, route.SigningKeyID).Scan(&blocked)
	if err != nil {
		return err
	}
	if blocked {
		return ErrDeliveryRouteBusy
	}
	_, err = tx.Exec(ctx, `INSERT INTO event_delivery_routes (terminal_serial_number,endpoint_url,signing_key_id,enabled,updated_by)
 VALUES ($1,$2,$3,$4,$5) ON CONFLICT (terminal_serial_number) DO UPDATE SET endpoint_url=EXCLUDED.endpoint_url,
 signing_key_id=EXCLUDED.signing_key_id,enabled=EXCLUDED.enabled,updated_by=EXCLUDED.updated_by,updated_at=now()`, serial, route.EndpointURL, route.SigningKeyID, route.Enabled, actorID)
	if err != nil {
		return err
	}
	_, err = insertGatewayActivity(ctx, tx, activityForRoute(serial, actorID))
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) DeliveryRoutes(ctx context.Context) ([]DeliveryRoute, error) {
	rows, err := s.pool.Query(ctx, `SELECT terminal_serial_number,endpoint_url,signing_key_id,enabled FROM event_delivery_routes ORDER BY terminal_serial_number`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	routes := []DeliveryRoute{}
	for rows.Next() {
		var r DeliveryRoute
		if err = rows.Scan(&r.TerminalSerialNumber, &r.EndpointURL, &r.SigningKeyID, &r.Enabled); err != nil {
			return nil, err
		}
		routes = append(routes, r)
	}
	return routes, rows.Err()
}

func (s *Store) EventDeliveries(ctx context.Context, limit, offset int) ([]EventDelivery, int64, error) {
	var total int64
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM event_deliveries`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id,device_event_id,retained_event_id,backfill_id,terminal_serial_number,endpoint_url,signing_key_id,status,attempts,next_attempt_at,last_http_status,last_error,created_at,delivered_at FROM event_deliveries ORDER BY id DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	deliveries := []EventDelivery{}
	for rows.Next() {
		var d EventDelivery
		if err = rows.Scan(&d.ID, &d.DeviceEventID, &d.RetainedEventID, &d.BackfillID, &d.TerminalSerialNumber, &d.EndpointURL, &d.SigningKeyID, &d.Status, &d.Attempts, &d.NextAttemptAt, &d.LastHTTPStatus, &d.LastError, &d.CreatedAt, &d.DeliveredAt); err != nil {
			return nil, 0, err
		}
		deliveries = append(deliveries, d)
	}
	return deliveries, total, rows.Err()
}

func (s *Store) ClaimEventDelivery(ctx context.Context) (*EventDelivery, error) {
	var d EventDelivery
	err := s.pool.QueryRow(ctx, `WITH candidate AS (
 SELECT d.id FROM event_deliveries d JOIN event_delivery_routes r ON r.terminal_serial_number=d.terminal_serial_number
 WHERE r.enabled AND ((d.status='pending' AND d.next_attempt_at<=now()) OR (d.status='sending' AND d.lease_until<=now()))
 ORDER BY d.id FOR UPDATE OF d SKIP LOCKED LIMIT 1)
 UPDATE event_deliveries d SET status='sending',attempts=attempts+1,lease_until=now()+interval '60 seconds'
 FROM candidate c WHERE d.id=c.id RETURNING d.id,d.device_event_id,d.retained_event_id,d.backfill_id,d.terminal_serial_number,d.endpoint_url,d.signing_key_id,d.attempts,d.request_body`).Scan(&d.ID, &d.DeviceEventID, &d.RetainedEventID, &d.BackfillID, &d.TerminalSerialNumber, &d.EndpointURL, &d.SigningKeyID, &d.Attempts, &d.Body)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &d, err
}

func (s *Store) FinishEventDelivery(ctx context.Context, d EventDelivery, status string, httpStatus int, failure string, next time.Time) error {
	tag, err := s.pool.Exec(ctx, `UPDATE event_deliveries SET status=$3,last_http_status=NULLIF($4,0),last_error=NULLIF($5,''),
 next_attempt_at=$6,lease_until=NULL,delivered_at=CASE WHEN $3='delivered' THEN now() ELSE NULL END
 WHERE id=$1 AND attempts=$2 AND status='sending'`, d.ID, d.Attempts, status, httpStatus, failure, next)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("event delivery lease is no longer owned")
	}
	return nil
}

func (s *Store) RetryEventDelivery(ctx context.Context, id int64, actorID int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var serial string
	if err = tx.QueryRow(ctx, `SELECT terminal_serial_number FROM event_deliveries WHERE id=$1`, id).Scan(&serial); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `SELECT serial_number FROM terminals WHERE serial_number=$1 FOR UPDATE`, serial); err != nil {
		return err
	}

	err = tx.QueryRow(ctx, `UPDATE event_deliveries SET status='pending',next_attempt_at=now(),last_error=NULL
 WHERE id=$1 AND status='failed' RETURNING terminal_serial_number`, id).Scan(&serial)
	if err != nil {
		return err
	}
	_, err = insertGatewayActivity(ctx, tx, activityForRetry(serial, id, actorID))
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func activityForRoute(serial string, actor int64) activity.Event {
	return activity.Event{Kind: "admin.event_delivery_route_saved", Terminal: serial, Message: "event delivery configuration saved", Fields: map[string]any{"adminUserId": actor}}
}
func activityForRetry(serial string, id, actor int64) activity.Event {
	return activity.Event{Kind: "admin.event_delivery_retried", Terminal: serial, Message: "failed event delivery queued for retry", Fields: map[string]any{"deliveryId": id, "adminUserId": actor}}
}

// Removing configuration preserves source archives, delivered bodies and audit.
// Unfinished deliveries require their route to remain available for retries.
func (s *Store) DeleteDeliveryRoute(ctx context.Context, serial string, actorID int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var terminal string
	if err = tx.QueryRow(ctx, `SELECT serial_number FROM terminals WHERE serial_number=$1 FOR UPDATE`, serial).Scan(&terminal); err != nil {
		return err
	}
	if err = tx.QueryRow(ctx, `SELECT terminal_serial_number FROM event_delivery_routes WHERE terminal_serial_number=$1 FOR UPDATE`, serial).Scan(&terminal); err != nil {
		return err
	}
	var unfinished bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM event_deliveries WHERE terminal_serial_number=$1 AND status<>'delivered')`, serial).Scan(&unfinished); err != nil {
		return err
	}
	if unfinished {
		return ErrDeliveryRouteUnfinished
	}
	if _, err = tx.Exec(ctx, `DELETE FROM event_delivery_routes WHERE terminal_serial_number=$1`, serial); err != nil {
		return err
	}
	if _, err = insertGatewayActivity(ctx, tx, activity.Event{Kind: "admin.event_delivery_route_removed", Terminal: serial, Message: "event delivery destination removed", Fields: map[string]any{"adminUserId": actorID}}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
