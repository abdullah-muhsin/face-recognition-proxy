package store

import (
	"context"
	"encoding/base64"
	"encoding/json"
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

func deliveryTestStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("GATEWAY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set GATEWAY_TEST_DATABASE_URL for PostgreSQL delivery tests")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("delivery_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Close(); _, _ = admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); admin.Close() })
	data := &Store{pool: pool}
	if err = data.Migrate(ctx, filepath.Join("..", "..", "db", "migrations")); err != nil {
		t.Fatal(err)
	}
	if err = data.SeedConfiguredTerminals(ctx, []config.Terminal{{SerialNumber: "DEVICE", PushSDKSerial: "GN0953979", Username: "test", Password: "secret", SecurityVersion: 4, LoginPasswordDigest: "sha256", CommandIntervalSeconds: 5, ErrorDelaySeconds: 30}}); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO admin_users (username,password_hash) VALUES ('admin','test')`); err != nil {
		t.Fatal(err)
	}
	if err = data.SaveDeliveryRoute(ctx, DeliveryRoute{TerminalSerialNumber: "DEVICE", EndpointURL: "https://school.invalid/events", SigningKeyID: "tenant", Enabled: true}, 1); err != nil {
		t.Fatal(err)
	}
	return data
}

func faceEvent(id string, at time.Time) NewDeviceEvent {
	body := fmt.Sprintf(`{"eventType":"AccessControllerEvent","eventState":"active","dateTime":%q,"AccessControllerEvent":{"majorEventType":5,"subEventType":75,"employeeNoString":"007","name":"Name","serialNo":241}}`, at.Format(time.RFC3339Nano))
	return NewDeviceEvent{TerminalSerialNumber: "DEVICE", VendorEventID: id, DataFormat: "jsonData", PayloadBase64: base64.StdEncoding.EncodeToString([]byte(body))}
}

func TestDeliveryAtomicityDeduplicationOldLiveScansAndImmutableDestination(t *testing.T) {
	data := deliveryTestStore(t)
	ctx := context.Background()
	event := faceEvent("Exact-UUID", time.Now())
	if _, err := data.InsertDeviceEventBatch(ctx, []NewDeviceEvent{event, event, faceEvent("old", time.Now().Add(-2*time.Hour))}); err != nil {
		t.Fatal(err)
	}
	rows, total, err := data.EventDeliveries(ctx, 25, 0)
	if err != nil || total != 2 {
		t.Fatalf("deliveries: %d %v", total, err)
	}
	routes, _ := data.DeliveryRoutes(ctx)
	changed := routes[0]
	changed.EndpointURL = "https://other.invalid/events"
	if err = data.SaveDeliveryRoute(ctx, changed, 1); !errors.Is(err, ErrDeliveryRouteBusy) {
		t.Fatalf("rerouted pending event: %v", err)
	}
	d, err := data.ClaimEventDelivery(ctx)
	if err != nil || d == nil || d.ID != rows[1].ID || d.Attempts != 1 {
		t.Fatalf("claim: %v %v", d, err)
	}
	var source EventMessage
	if err = json.Unmarshal(d.Body, &source); err != nil || source.EventID != "Exact-UUID" || *source.EmployeeNumber != "007" {
		t.Fatalf("source changed: %s %v", d.Body, err)
	}
	if next, err := data.ClaimEventDelivery(ctx); err != nil || next == nil {
		t.Fatalf("old live scan was not queued: %v %v", next, err)
	} else if err = data.FinishEventDelivery(ctx, *next, "delivered", 200, "", time.Now()); err != nil {
		t.Fatal(err)
	}
	if next, err := data.ClaimEventDelivery(ctx); err != nil || next != nil {
		t.Fatalf("live lease reclaimed: %v %v", next, err)
	}
	// A crash or a lost acknowledgement makes the exact same bytes eligible again.
	if _, err = data.pool.Exec(ctx, `UPDATE event_deliveries SET lease_until=now()-interval '1 second'`); err != nil {
		t.Fatal(err)
	}
	replay, err := data.ClaimEventDelivery(ctx)
	if err != nil || replay == nil || replay.Attempts != 2 || string(replay.Body) != string(d.Body) {
		t.Fatalf("lease recovery: %v %v", replay, err)
	}
	if err = data.FinishEventDelivery(ctx, *d, "delivered", 200, "", time.Now()); err == nil {
		t.Fatal("stale worker retained lease ownership")
	}
	if err = data.FinishEventDelivery(ctx, *replay, "failed", 401, "receiver_rejected", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err = data.RetryEventDelivery(ctx, replay.ID, 1); err != nil {
		t.Fatal(err)
	}
	if err = data.RetryEventDelivery(ctx, replay.ID, 1); err == nil {
		t.Fatal("retried a nonfailed delivery")
	}
	replay, err = data.ClaimEventDelivery(ctx)
	if err != nil || replay == nil {
		t.Fatal(err)
	}
	if err = data.FinishEventDelivery(ctx, *replay, "delivered", 200, "", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err = data.SaveDeliveryRoute(ctx, changed, 1); err != nil {
		t.Fatal(err)
	}
	rows, _, _ = data.EventDeliveries(ctx, 25, 0)
	if rows[0].EndpointURL != "https://school.invalid/events" {
		t.Fatal("historical destination changed")
	}
	// An outbox write failure rejects the entire device batch, including its archive.
	if _, err = data.pool.Exec(ctx, `CREATE FUNCTION reject_delivery() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test failure'; END $$;
 CREATE TRIGGER reject_delivery BEFORE INSERT ON event_deliveries FOR EACH ROW EXECUTE FUNCTION reject_delivery()`); err != nil {
		t.Fatal(err)
	}
	if _, err = data.InsertDeviceEventBatch(ctx, []NewDeviceEvent{faceEvent("rollback", time.Now())}); err == nil {
		t.Fatal("event acknowledged without durable delivery")
	}
	var count int
	if err = data.pool.QueryRow(ctx, `SELECT count(*) FROM device_events WHERE vendor_event_id='rollback'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("event escaped rollback: %d %v", count, err)
	}
}

func TestDeliveryPauseAndTransientRetry(t *testing.T) {
	data := deliveryTestStore(t)
	ctx := context.Background()
	if _, err := data.InsertDeviceEventBatch(ctx, []NewDeviceEvent{faceEvent("event", time.Now())}); err != nil {
		t.Fatal(err)
	}
	routes, _ := data.DeliveryRoutes(ctx)
	r := routes[0]
	r.Enabled = false
	if err := data.SaveDeliveryRoute(ctx, r, 1); err != nil {
		t.Fatal(err)
	}
	if d, err := data.ClaimEventDelivery(ctx); err != nil || d != nil {
		t.Fatal("paused queue was claimed")
	}
	if _, err := data.InsertDeviceEventBatch(ctx, []NewDeviceEvent{faceEvent("paused", time.Now())}); err != nil {
		t.Fatal(err)
	}
	_, total, _ := data.EventDeliveries(ctx, 25, 0)
	if total != 1 {
		t.Fatal("paused route collected deliveries")
	}
	r.Enabled = true
	if err := data.SaveDeliveryRoute(ctx, r, 1); err != nil {
		t.Fatal(err)
	}
	d, err := data.ClaimEventDelivery(ctx)
	if err != nil || d == nil {
		t.Fatal(err)
	}
	if err = data.FinishEventDelivery(ctx, *d, "pending", 503, "receiver_unavailable", time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if d, err = data.ClaimEventDelivery(ctx); err != nil || d != nil {
		t.Fatal("retry happened before its deadline")
	}
}

func TestDeliveryForwardMigration(t *testing.T) {
	data := deliveryTestStore(t)
	ctx := context.Background()
	if _, err := data.pool.Exec(ctx, `DROP TABLE event_deliveries; DROP TABLE event_delivery_backfills; DROP TABLE event_delivery_routes; DELETE FROM schema_migrations WHERE name IN ('013_event_delivery.sql','014_delivery_backfills.sql')`); err != nil {
		t.Fatal(err)
	}
	if err := data.Migrate(ctx, filepath.Join("..", "..", "db", "migrations")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := data.EventDeliveries(ctx, 25, 0); err != nil {
		t.Fatal(err)
	}
}

func TestDeliveryCapturesMultipartFaceMetadataWithoutMedia(t *testing.T) {
	data := deliveryTestStore(t)
	ctx := context.Background()
	event := faceEvent("multipart-identity", time.Now())
	metadata, err := base64.StdEncoding.DecodeString(event.PayloadBase64)
	if err != nil {
		t.Fatal(err)
	}
	payload := "Content-Type: multipart/form-data; boundary=MIME_boundary\r\n\r\n" +
		"--MIME_boundary\r\nContent-Disposition: form-data; name=\"AccessControllerEvent\"\r\nContent-Type: application/json\r\n\r\n" + string(metadata) +
		"\r\n--MIME_boundary\r\nContent-Disposition: form-data; name=\"Picture\"; filename=\"face.jpg\"\r\nContent-Type: image/jpeg\r\n\r\nprivate-image-bytes\r\n--MIME_boundary--\r\n"
	event.DataFormat = "boundaryData"
	event.PayloadBase64 = base64.StdEncoding.EncodeToString([]byte(payload))
	if _, err = data.InsertDeviceEventBatch(ctx, []NewDeviceEvent{event}); err != nil {
		t.Fatal(err)
	}
	d, err := data.ClaimEventDelivery(ctx)
	if err != nil || d == nil {
		t.Fatalf("multipart event not queued: %v %v", d, err)
	}
	var message EventMessage
	if err = json.Unmarshal(d.Body, &message); err != nil || message.EventID != event.VendorEventID || message.EmployeeNumber == nil || *message.EmployeeNumber != "007" || message.OccurredAt == nil {
		t.Fatalf("multipart metadata changed: %v", err)
	}
	var fields map[string]any
	if err = json.Unmarshal(d.Body, &fields); err != nil || len(fields) != 12 {
		t.Fatalf("unexpected delivery fields: %v", err)
	}
	if strings.Contains(string(d.Body), "private-image-bytes") || strings.Contains(string(d.Body), "Picture") {
		t.Fatal("media escaped the archive")
	}
}

func TestDestinationRemovalPreservesDeliveredHistoryAndBlocksUnfinishedWork(t *testing.T) {
	data := deliveryTestStore(t)
	ctx := context.Background()
	if _, err := data.InsertDeviceEventBatch(ctx, []NewDeviceEvent{faceEvent("event", time.Now())}); err != nil {
		t.Fatal(err)
	}
	if err := data.DeleteDeliveryRoute(ctx, "DEVICE", 1); !errors.Is(err, ErrDeliveryRouteUnfinished) {
		t.Fatalf("removed route with pending work: %v", err)
	}
	d, err := data.ClaimEventDelivery(ctx)
	if err != nil || d == nil {
		t.Fatal(err)
	}
	if err = data.FinishEventDelivery(ctx, *d, "delivered", 200, "", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err = data.DeleteDeliveryRoute(ctx, "DEVICE", 1); err != nil {
		t.Fatal(err)
	}
	routes, err := data.DeliveryRoutes(ctx)
	if err != nil || len(routes) != 0 {
		t.Fatal("destination retained")
	}
	deliveries, total, err := data.EventDeliveries(ctx, 25, 0)
	if err != nil || total != 1 || deliveries[0].Status != "delivered" {
		t.Fatal("delivery audit lost")
	}
}
