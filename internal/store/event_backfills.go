package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/itplus/pushsdk-gateway/internal/activity"
	"github.com/jackc/pgx/v5"
)

var ErrBackfillPreviewChanged = errors.New("the archive or destination changed; preview the backfill again")
var ErrBackfillRouteDisabled = errors.New("an enabled destination is required for backfill")
var ErrBackfillTooLarge = errors.New("select a smaller range; a backfill can contain at most 10000 source records")
var ErrBackfillEmpty = errors.New("no undelivered eligible events in this preview")
var eventIdentifier = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,64}$`)

type BackfillIdentity struct {
	EmployeeNumber string    `json:"employeeNumber"`
	Names          []string  `json:"names"`
	Events         int       `json:"events"`
	FirstScan      time.Time `json:"firstScan"`
	LastScan       time.Time `json:"lastScan"`
}

type BackfillPreview struct {
	Token            string             `json:"token"`
	Route            DeliveryRoute      `json:"route"`
	StartsAt         time.Time          `json:"startsAt"`
	EndsAt           time.Time          `json:"endsAt"`
	Eligible         int                `json:"eligible"`
	AlreadyQueued    int                `json:"alreadyQueued"`
	DuplicateSources int                `json:"duplicateSources"`
	Invalid          int                `json:"invalid"`
	PushSDKEvents    int                `json:"pushSdkEvents"`
	ISAPIEvents      int                `json:"isapiEvents"`
	Identities       []BackfillIdentity `json:"identities"`
	candidates       []backfillCandidate
}

type backfillCandidate struct {
	Source string `json:"source"`
	ID     int64  `json:"id"`
	Body   []byte `json:"body"`
}

func (s *Store) PreviewEventBackfill(ctx context.Context, serial string, from, until time.Time) (*BackfillPreview, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	preview, err := previewEventBackfill(ctx, tx, serial, from, until)
	if err != nil {
		return nil, err
	}
	return preview, tx.Commit(ctx)
}

func previewEventBackfill(ctx context.Context, tx pgx.Tx, serial string, from, until time.Time) (*BackfillPreview, error) {
	p := &BackfillPreview{StartsAt: from, EndsAt: until, Identities: []BackfillIdentity{}, candidates: []backfillCandidate{}}
	err := tx.QueryRow(ctx, `SELECT terminal_serial_number,endpoint_url,signing_key_id,enabled FROM event_delivery_routes WHERE terminal_serial_number=$1`, serial).Scan(&p.Route.TerminalSerialNumber, &p.Route.EndpointURL, &p.Route.SigningKeyID, &p.Route.Enabled)
	if err != nil {
		return nil, err
	}
	if !p.Route.Enabled {
		return nil, ErrBackfillRouteDisabled
	}
	// Read both original archives without changing them. PushSDK rows come first
	// so an exact shared serial/time/person/name tuple can represent both sources.
	rows, err := tx.Query(ctx, `SELECT a.source,a.id,a.event_id,t.pushsdk_serial,a.event_state,a.occurred_at,a.employee_number,a.employee_name,a.event_serial_number,a.queued FROM (
 SELECT 'pushsdk'::TEXT source,d.id,d.vendor_event_id event_id,d.terminal_serial_number,p.event_state,p.occurred_at,p.employee_number,p.employee_name,p.event_serial_number,
 EXISTS(SELECT 1 FROM event_deliveries e WHERE e.device_event_id=d.id) queued
 FROM device_events d JOIN access_event_projections p ON p.device_event_id=d.id
 WHERE d.terminal_serial_number=$1 AND p.major_event_type=5 AND p.sub_event_type=75 AND p.event_state='active' AND p.occurred_at >= $2 AND p.occurred_at < $3
 UNION ALL
 SELECT 'isapi',r.id,encode(r.source_sha256,'hex'),r.terminal_serial_number,NULL::TEXT,r.occurred_at,r.employee_number,r.employee_name,r.event_serial_number,
 EXISTS(SELECT 1 FROM event_deliveries e WHERE e.retained_event_id=r.id)
 FROM retained_access_events r WHERE r.terminal_serial_number=$1 AND r.major_event_type=5 AND r.sub_event_type=75 AND r.occurred_at >= $2 AND r.occurred_at < $3
 ) a JOIN terminals t ON t.serial_number=a.terminal_serial_number ORDER BY a.source DESC,a.id LIMIT 10001`, serial, from, until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type sharedIdentity struct {
		At          string
		Person      string
		Name        string
		NamePresent bool
		Serial      int64
	}
	pushSDK := map[sharedIdentity]bool{}
	identities := map[string]*BackfillIdentity{}
	count := 0
	for rows.Next() {
		count++
		if count > 10000 {
			return nil, ErrBackfillTooLarge
		}
		m := EventMessage{SchemaVersion: 2, TerminalSerialNumber: serial, MajorEventType: 5, SubEventType: 75}
		var id int64
		var queued bool
		if err = rows.Scan(&m.Source, &id, &m.EventID, &m.PushSDKSerial, &m.EventState, &m.OccurredAt, &m.EmployeeNumber, &m.EmployeeName, &m.EventSerialNumber, &queued); err != nil {
			return nil, err
		}
		if !validBackfillMessage(m) {
			p.Invalid++
			continue
		}
		var shared sharedIdentity
		if m.EventSerialNumber != nil {
			shared = sharedIdentity{At: m.OccurredAt.UTC().Format(time.RFC3339Nano), Person: *m.EmployeeNumber, Serial: *m.EventSerialNumber}
			if m.EmployeeName != nil {
				shared.Name = *m.EmployeeName
				shared.NamePresent = true
			}
			if m.Source == "isapi" && pushSDK[shared] {
				p.DuplicateSources++
				continue
			}
			if m.Source == "pushsdk" {
				pushSDK[shared] = true
			}
		}
		if queued {
			p.AlreadyQueued++
			continue
		}
		body, err := json.Marshal(m)
		if err != nil {
			return nil, err
		}
		p.candidates = append(p.candidates, backfillCandidate{Source: m.Source, ID: id, Body: body})
		p.Eligible++
		if m.Source == "pushsdk" {
			p.PushSDKEvents++
		} else {
			p.ISAPIEvents++
		}
		identity := identities[*m.EmployeeNumber]
		if identity == nil {
			identity = &BackfillIdentity{EmployeeNumber: *m.EmployeeNumber, Names: []string{}, FirstScan: *m.OccurredAt, LastScan: *m.OccurredAt}
			identities[*m.EmployeeNumber] = identity
		}
		identity.Events++
		if m.OccurredAt.Before(identity.FirstScan) {
			identity.FirstScan = *m.OccurredAt
		}
		if m.OccurredAt.After(identity.LastScan) {
			identity.LastScan = *m.OccurredAt
		}
		if m.EmployeeName != nil {
			found := false
			for _, name := range identity.Names {
				if name == *m.EmployeeName {
					found = true
					break
				}
			}
			if !found {
				identity.Names = append(identity.Names, *m.EmployeeName)
			}
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	for _, identity := range identities {
		sort.Strings(identity.Names)
		p.Identities = append(p.Identities, *identity)
	}
	sort.Slice(p.Identities, func(i, j int) bool { return p.Identities[i].EmployeeNumber < p.Identities[j].EmployeeNumber })
	// Bind approval to the destination and exact immutable bodies, not just a
	// count. Concurrent delivery or archive changes require a fresh preview.
	manifest, err := json.Marshal(struct {
		Preview    *BackfillPreview
		Candidates []backfillCandidate
	}{p, p.candidates})
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(manifest)
	p.Token = hex.EncodeToString(digest[:])
	return p, nil
}

func validBackfillMessage(m EventMessage) bool {
	if !eventIdentifier.MatchString(m.EventID) || m.OccurredAt == nil || m.EmployeeNumber == nil || !utf8.ValidString(*m.EmployeeNumber) || utf8.RuneCountInString(*m.EmployeeNumber) == 0 || utf8.RuneCountInString(*m.EmployeeNumber) > 32 {
		return false
	}
	for _, r := range *m.EmployeeNumber {
		if unicode.IsSpace(r) {
			return false
		}
	}
	if m.EmployeeName != nil && (!utf8.ValidString(*m.EmployeeName) || utf8.RuneCountInString(*m.EmployeeName) > 100) {
		return false
	}
	return m.EventSerialNumber == nil || *m.EventSerialNumber >= 0
}

func (s *Store) QueueEventBackfill(ctx context.Context, serial string, from, until time.Time, token string, actor int64) (int64, int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback(ctx)
	var terminal string
	if err = tx.QueryRow(ctx, `SELECT serial_number FROM terminals WHERE serial_number=$1 FOR UPDATE`, serial).Scan(&terminal); err != nil {
		return 0, 0, err
	}
	p, err := previewEventBackfill(ctx, tx, serial, from, until)
	if err != nil {
		return 0, 0, err
	}
	if p.Token != token {
		return 0, 0, ErrBackfillPreviewChanged
	}
	if p.Eligible == 0 {
		return 0, 0, ErrBackfillEmpty
	}
	var batch int64
	err = tx.QueryRow(ctx, `INSERT INTO event_delivery_backfills (terminal_serial_number,starts_at,ends_at,preview_sha256,queued_count,created_by) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`, serial, from, until, token, p.Eligible, actor).Scan(&batch)
	if err != nil {
		return 0, 0, err
	}
	for _, candidate := range p.candidates {
		var live, retained *int64
		if candidate.Source == "pushsdk" {
			live = &candidate.ID
		} else {
			retained = &candidate.ID
		}
		_, err = tx.Exec(ctx, `INSERT INTO event_deliveries (device_event_id,retained_event_id,backfill_id,terminal_serial_number,endpoint_url,signing_key_id,request_body) VALUES ($1,$2,$3,$4,$5,$6,$7)`, live, retained, batch, serial, p.Route.EndpointURL, p.Route.SigningKeyID, candidate.Body)
		if err != nil {
			return 0, 0, err
		}
	}
	_, err = insertGatewayActivity(ctx, tx, activity.Event{Kind: "admin.event_backfill_queued", Terminal: serial, Message: "previewed historical events queued for delivery", Fields: map[string]any{"backfillId": batch, "queuedCount": p.Eligible, "startsAt": from, "endsAt": until, "adminUserId": actor}})
	if err != nil {
		return 0, 0, err
	}
	return batch, p.Eligible, tx.Commit(ctx)
}
