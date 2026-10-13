package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"time"

	"github.com/itplus/pushsdk-gateway/internal/accesscontrol"
	"github.com/itplus/pushsdk-gateway/internal/activity"
	"github.com/jackc/pgx/v5"
)

type AttachmentMessage struct {
	SchemaVersion        int                       `json:"schemaVersion"`
	Kind                 string                    `json:"kind"`
	Source               string                    `json:"source"`
	EventID              string                    `json:"eventId"`
	TerminalSerialNumber string                    `json:"terminalSerialNumber"`
	PushSDKSerial        string                    `json:"pushSdkSerial"`
	AttachmentID         string                    `json:"attachmentId"`
	Availability         string                    `json:"availability"`
	Reason               *string                   `json:"reason"`
	Attachment           *accesscontrol.Attachment `json:"attachment"`
}

// AttachmentQueue reuses the signed delivery transport while keeping leases,
// receipts and failures separate from attendance-event delivery.
type AttachmentQueue struct{ Store *Store }

func (q AttachmentQueue) ClaimEventDelivery(ctx context.Context) (*EventDelivery, error) {
	tx, err := q.Store.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var d EventDelivery
	var eventBody []byte
	var parent int64
	err = tx.QueryRow(ctx, `SELECT a.id,a.event_delivery_id,d.terminal_serial_number,d.endpoint_url,d.signing_key_id,a.attempts,a.request_body,d.request_body
 FROM event_attachment_deliveries a JOIN event_deliveries d ON d.id=a.event_delivery_id
 JOIN event_delivery_routes r ON r.terminal_serial_number=d.terminal_serial_number
 WHERE r.enabled AND d.status='delivered' AND ((a.status='pending' AND a.next_attempt_at<=now()) OR (a.status='sending' AND a.lease_until<=now()))
 ORDER BY a.id FOR UPDATE OF a SKIP LOCKED LIMIT 1`).Scan(&d.ID, &parent, &d.TerminalSerialNumber, &d.EndpointURL, &d.SigningKeyID, &d.Attempts, &d.Body, &eventBody)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	endpoint, err := url.Parse(d.EndpointURL)
	if err != nil {
		return nil, err
	}
	endpoint.Path += "/attachments"
	if endpoint.RawPath != "" {
		endpoint.RawPath += "/attachments"
	}
	d.EndpointURL = endpoint.String()
	d.Attachment = true
	d.Attempts++
	if d.Body == nil {
		var event EventMessage
		if err = json.Unmarshal(eventBody, &event); err != nil {
			return nil, err
		}
		message := AttachmentMessage{SchemaVersion: 1, Kind: "event_attachment", Source: event.Source, EventID: event.EventID, TerminalSerialNumber: event.TerminalSerialNumber, PushSDKSerial: event.PushSDKSerial, AttachmentID: "Picture", Availability: "not_provided"}
		if event.SchemaVersion == 1 {
			message.Source = "pushsdk"
		}
		if message.Source == "pushsdk" {
			var format, payload string
			err = tx.QueryRow(ctx, `SELECT e.data_format,e.payload_base64 FROM device_events e JOIN event_deliveries d ON d.device_event_id=e.id WHERE d.id=$1`, parent).Scan(&format, &payload)
			if err != nil {
				return nil, err
			}
			message.Attachment, message.Availability, message.Reason = accesscontrol.DeliveryPicture(format, payload)
		}
		d.Body, err = json.Marshal(message)
		if err != nil {
			return nil, err
		}
		if _, err = tx.Exec(ctx, `UPDATE event_attachment_deliveries SET request_body=$2,availability=$3,reason=$4 WHERE id=$1`, d.ID, d.Body, message.Availability, message.Reason); err != nil {
			return nil, err
		}
	}
	_, err = tx.Exec(ctx, `UPDATE event_attachment_deliveries SET status='sending',attempts=$2,lease_until=now()+interval '60 seconds' WHERE id=$1`, d.ID, d.Attempts)
	if err != nil {
		return nil, err
	}
	return &d, tx.Commit(ctx)
}

func (q AttachmentQueue) FinishEventDelivery(ctx context.Context, d EventDelivery, status string, code int, failure string, next time.Time) error {
	tag, err := q.Store.pool.Exec(ctx, `UPDATE event_attachment_deliveries SET status=$3,last_http_status=NULLIF($4,0),last_error=NULLIF($5,''),next_attempt_at=$6,lease_until=NULL,delivered_at=CASE WHEN $3='delivered' THEN now() ELSE NULL END WHERE id=$1 AND attempts=$2 AND status='sending'`, d.ID, d.Attempts, status, code, failure, next)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("attachment delivery lease is no longer owned")
	}
	return nil
}

func enqueueAttachment(ctx context.Context, tx pgx.Tx, parent int64) error {
	_, err := tx.Exec(ctx, `INSERT INTO event_attachment_deliveries(event_delivery_id) VALUES($1)`, parent)
	return err
}

type AttachmentDelivery struct {
	EventDelivery
	EventDeliveryID int64   `json:"eventDeliveryId"`
	Availability    *string `json:"availability"`
	Reason          *string `json:"reason"`
}

func (s *Store) AttachmentDeliveries(ctx context.Context, limit, offset int) ([]AttachmentDelivery, int64, error) {
	var total int64
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM event_attachment_deliveries`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `SELECT a.id,a.event_delivery_id,d.terminal_serial_number,d.endpoint_url,a.status,a.attempts,a.next_attempt_at,a.last_http_status,a.last_error,a.created_at,a.delivered_at,a.availability,a.reason FROM event_attachment_deliveries a JOIN event_deliveries d ON d.id=a.event_delivery_id ORDER BY a.id DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	result := []AttachmentDelivery{}
	for rows.Next() {
		var a AttachmentDelivery
		if err = rows.Scan(&a.ID, &a.EventDeliveryID, &a.TerminalSerialNumber, &a.EndpointURL, &a.Status, &a.Attempts, &a.NextAttemptAt, &a.LastHTTPStatus, &a.LastError, &a.CreatedAt, &a.DeliveredAt, &a.Availability, &a.Reason); err != nil {
			return nil, 0, err
		}
		result = append(result, a)
	}
	return result, total, rows.Err()
}

func (s *Store) RetryAttachmentDelivery(ctx context.Context, id, actor int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var serial string
	err = tx.QueryRow(ctx, `SELECT d.terminal_serial_number FROM event_deliveries d JOIN event_attachment_deliveries a ON a.event_delivery_id=d.id JOIN terminals t ON t.serial_number=d.terminal_serial_number WHERE a.id=$1 FOR UPDATE OF t`, id).Scan(&serial)
	if err != nil {
		return err
	}
	var returned int64
	err = tx.QueryRow(ctx, `UPDATE event_attachment_deliveries SET status='pending',next_attempt_at=now(),last_error=NULL WHERE id=$1 AND status='failed' RETURNING id`, id).Scan(&returned)
	if err != nil {
		return err
	}
	_, err = insertGatewayActivity(ctx, tx, activity.Event{Kind: "admin.attachment_delivery_retried", Terminal: serial, Message: "failed attachment queued for retry", Fields: map[string]any{"attachmentDeliveryId": id, "adminUserId": actor}})
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type AttachmentBackfillPreview struct {
	ThroughDeliveryID int64   `json:"throughDeliveryId"`
	Eligible          int     `json:"eligible"`
	Token             string  `json:"token"`
	IDs               []int64 `json:"-"`
}

func previewAttachments(ctx context.Context, tx pgx.Tx, serial string, through int64) (*AttachmentBackfillPreview, error) {
	var enabled bool
	if err := tx.QueryRow(ctx, `SELECT enabled FROM event_delivery_routes WHERE terminal_serial_number=$1`, serial).Scan(&enabled); err != nil {
		return nil, err
	}
	if !enabled {
		return nil, ErrBackfillRouteDisabled
	}
	if through == 0 {
		if err := tx.QueryRow(ctx, `SELECT coalesce(max(id),0) FROM event_deliveries WHERE terminal_serial_number=$1`, serial).Scan(&through); err != nil {
			return nil, err
		}
	}
	rows, err := tx.Query(ctx, `SELECT d.id,d.endpoint_url,d.signing_key_id FROM event_deliveries d WHERE d.terminal_serial_number=$1 AND d.id<=$2 AND d.status='delivered' AND NOT EXISTS(SELECT 1 FROM event_attachment_deliveries a WHERE a.event_delivery_id=d.id) ORDER BY d.id LIMIT 5001`, serial, through)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	p := &AttachmentBackfillPreview{ThroughDeliveryID: through, IDs: []int64{}}
	digest := sha256.New()
	enc := json.NewEncoder(digest)
	_ = enc.Encode(serial)
	_ = enc.Encode(through)
	for rows.Next() {
		var id int64
		var endpoint, key string
		if err = rows.Scan(&id, &endpoint, &key); err != nil {
			return nil, err
		}
		p.IDs = append(p.IDs, id)
		_ = enc.Encode([]any{id, endpoint, key})
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	p.Eligible = len(p.IDs)
	if p.Eligible > 5000 {
		return nil, ErrBackfillTooLarge
	}
	p.Token = hex.EncodeToString(digest.Sum(nil))
	return p, nil
}

func (s *Store) PreviewAttachmentBackfill(ctx context.Context, serial string, through int64) (*AttachmentBackfillPreview, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	return previewAttachments(ctx, tx, serial, through)
}

func (s *Store) QueueAttachmentBackfill(ctx context.Context, serial string, through int64, token string, actor int64) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	var locked string
	if err = tx.QueryRow(ctx, `SELECT serial_number FROM terminals WHERE serial_number=$1 FOR UPDATE`, serial).Scan(&locked); err != nil {
		return 0, err
	}
	p, err := previewAttachments(ctx, tx, serial, through)
	if err != nil {
		return 0, err
	}
	if p.Token != token {
		return 0, ErrBackfillPreviewChanged
	}
	if p.Eligible == 0 {
		return 0, ErrBackfillEmpty
	}
	for _, id := range p.IDs {
		if err = enqueueAttachment(ctx, tx, id); err != nil {
			return 0, err
		}
	}
	_, err = insertGatewayActivity(ctx, tx, activity.Event{Kind: "admin.attachment_backfill_queued", Terminal: serial, Message: "historical attachment receipts queued without attendance replay", Fields: map[string]any{"queuedCount": p.Eligible, "throughDeliveryId": through, "previewToken": token, "adminUserId": actor}})
	if err != nil {
		return 0, err
	}
	return p.Eligible, tx.Commit(ctx)
}
