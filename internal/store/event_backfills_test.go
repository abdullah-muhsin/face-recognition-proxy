package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func seedRetainedBackfillEvent(t *testing.T, data *Store, fingerprint string, at time.Time, person, name string, serial int64) {
	t.Helper()
	ctx := context.Background()
	_, err := data.pool.Exec(ctx, `INSERT INTO access_event_sync_runs (uuid,terminal_serial_number,created_by_username,search_id,status) VALUES ('11111111-1111-4111-8111-111111111111','DEVICE','admin','test','awaiting_time') ON CONFLICT DO NOTHING;
 INSERT INTO isapi_commands (uuid,terminal_serial_number,created_by_username,method,url,data_format,request_data,status,expires_at) VALUES ('22222222-2222-4222-8222-222222222222','DEVICE','admin','POST','/ISAPI/AccessControl/AcsEvent?format=json','jsonData',convert_to('{}','UTF8'),'queued',now()+interval '1 hour') ON CONFLICT DO NOTHING;`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = data.pool.Exec(ctx, `INSERT INTO retained_access_events (terminal_serial_number,source_sync_run_uuid,source_command_uuid,source_record,source_sha256,major_event_type,sub_event_type,occurred_at,employee_number,employee_name,event_serial_number) VALUES ('DEVICE','11111111-1111-4111-8111-111111111111','22222222-2222-4222-8222-222222222222',convert_to('{}','UTF8'),decode($1,'hex'),5,75,$2,$3,$4,$5)`, fingerprint, at, person, name, serial)
	if err != nil {
		t.Fatal(err)
	}
}

func TestBackfillPreviewDeduplicatesSourcesAndQueuesImmutableBodiesOnce(t *testing.T) {
	data := deliveryTestStore(t)
	ctx := context.Background()
	routes, _ := data.DeliveryRoutes(ctx)
	route := routes[0]
	route.Enabled = false
	if err := data.SaveDeliveryRoute(ctx, route, 1); err != nil {
		t.Fatal(err)
	}
	at := time.Now().Add(-24 * time.Hour).Truncate(time.Second)
	if _, err := data.InsertDeviceEventBatch(ctx, []NewDeviceEvent{faceEvent("old-scan", at)}); err != nil {
		t.Fatal(err)
	}
	seedRetainedBackfillEvent(t, data, strings.Repeat("a", 64), at, "007", "Name", 241)
	seedRetainedBackfillEvent(t, data, strings.Repeat("b", 64), at.Add(time.Minute), "008", "Other", 242)
	seedRetainedBackfillEvent(t, data, strings.Repeat("c", 64), at.Add(time.Minute), " 008 ", "Invalid", 243)
	route.Enabled = true
	if err := data.SaveDeliveryRoute(ctx, route, 1); err != nil {
		t.Fatal(err)
	}
	from, until := at.Add(-time.Minute), at.Add(time.Hour)
	p, err := data.PreviewEventBackfill(ctx, "DEVICE", from, until)
	if err != nil || p.Eligible != 2 || p.DuplicateSources != 1 || p.Invalid != 1 || p.PushSDKEvents != 1 || p.ISAPIEvents != 1 || len(p.Identities) != 2 {
		t.Fatalf("preview: %+v %v", p, err)
	}
	_, total, _ := data.EventDeliveries(ctx, 25, 0)
	if total != 0 {
		t.Fatal("preview created deliveries")
	}
	batch, count, err := data.QueueEventBackfill(ctx, "DEVICE", from, until, p.Token, 1)
	if err != nil || batch == 0 || count != 2 {
		t.Fatalf("queue: %d %d %v", batch, count, err)
	}
	if _, _, err = data.QueueEventBackfill(ctx, "DEVICE", from, until, p.Token, 1); !errors.Is(err, ErrBackfillPreviewChanged) {
		t.Fatalf("repeated preview applied: %v", err)
	}
	next, err := data.PreviewEventBackfill(ctx, "DEVICE", from, until)
	if err != nil || next.Eligible != 0 || next.AlreadyQueued != 2 || next.DuplicateSources != 1 {
		t.Fatalf("after queue: %+v %v", next, err)
	}
	d, err := data.ClaimEventDelivery(ctx)
	if err != nil || d == nil || d.BackfillID == nil || *d.BackfillID != batch {
		t.Fatalf("claim: %+v %v", d, err)
	}
	var live EventMessage
	if err = json.Unmarshal(d.Body, &live); err != nil || live.SchemaVersion != 2 || live.Source != "pushsdk" || live.EventID != "old-scan" || !live.OccurredAt.Equal(at) {
		t.Fatalf("live body changed: %s %v", d.Body, err)
	}
	d, err = data.ClaimEventDelivery(ctx)
	if err != nil || d == nil || d.RetainedEventID == nil || d.DeviceEventID != nil {
		t.Fatalf("ISAPI claim: %+v %v", d, err)
	}
	var archive EventMessage
	if err = json.Unmarshal(d.Body, &archive); err != nil || archive.Source != "isapi" || archive.EventState != nil || archive.EventID != strings.Repeat("b", 64) || *archive.EmployeeNumber != "008" {
		t.Fatalf("ISAPI source altered: %s %v", d.Body, err)
	}
}

func TestBackfillRejectsChangedPreviewAndSerializesConcurrentSubmissions(t *testing.T) {
	data := deliveryTestStore(t)
	ctx := context.Background()
	routes, _ := data.DeliveryRoutes(ctx)
	r := routes[0]
	r.Enabled = false
	if err := data.SaveDeliveryRoute(ctx, r, 1); err != nil {
		t.Fatal(err)
	}
	at := time.Now().Add(-time.Hour).Truncate(time.Second)
	if _, err := data.InsertDeviceEventBatch(ctx, []NewDeviceEvent{faceEvent("stored", at)}); err != nil {
		t.Fatal(err)
	}
	r.Enabled = true
	if err := data.SaveDeliveryRoute(ctx, r, 1); err != nil {
		t.Fatal(err)
	}
	from, until := at.Add(-time.Minute), at.Add(time.Minute)
	p, err := data.PreviewEventBackfill(ctx, "DEVICE", from, until)
	if err != nil {
		t.Fatal(err)
	}
	r.EndpointURL = "https://new.invalid/events"
	if err = data.SaveDeliveryRoute(ctx, r, 1); err != nil {
		t.Fatal(err)
	}
	if _, _, err = data.QueueEventBackfill(ctx, "DEVICE", from, until, p.Token, 1); !errors.Is(err, ErrBackfillPreviewChanged) {
		t.Fatalf("changed destination accepted: %v", err)
	}
	p, err = data.PreviewEventBackfill(ctx, "DEVICE", from, until)
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for i := 0; i < 2; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, _, err := data.QueueEventBackfill(ctx, "DEVICE", from, until, p.Token, 1)
			results <- err
		}()
	}
	wait.Wait()
	close(results)
	success := 0
	rejected := 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrBackfillPreviewChanged) {
			rejected++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || rejected != 1 {
		t.Fatalf("concurrent submissions: %d success %d stale", success, rejected)
	}
	_, total, _ := data.EventDeliveries(ctx, 25, 0)
	if total != 1 {
		t.Fatalf("double queue: %d", total)
	}
}

func TestBackfillQueueFailureRollsBackBatchAndDeliveries(t *testing.T) {
	data := deliveryTestStore(t)
	ctx := context.Background()
	at := time.Now().Add(-time.Hour).Truncate(time.Second)
	seedRetainedBackfillEvent(t, data, strings.Repeat("d", 64), at, "009", "Archive", 244)
	p, err := data.PreviewEventBackfill(ctx, "DEVICE", at.Add(-time.Minute), at.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	_, err = data.pool.Exec(ctx, `CREATE FUNCTION reject_backfill_delivery() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test failure'; END $$; CREATE TRIGGER reject_backfill_delivery BEFORE INSERT ON event_deliveries FOR EACH ROW EXECUTE FUNCTION reject_backfill_delivery()`)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = data.QueueEventBackfill(ctx, "DEVICE", p.StartsAt, p.EndsAt, p.Token, 1); err == nil {
		t.Fatal("failed queue committed")
	}
	var count int
	if err = data.pool.QueryRow(ctx, `SELECT count(*) FROM event_delivery_backfills`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("batch escaped rollback: %d %v", count, err)
	}
	_, total, _ := data.EventDeliveries(ctx, 25, 0)
	if total != 0 {
		t.Fatal("delivery escaped rollback")
	}
}
