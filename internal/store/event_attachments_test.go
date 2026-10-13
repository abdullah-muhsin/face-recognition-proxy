package store

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"testing"
	"time"
)

func finishTestAttachments(t *testing.T, data *Store) {
	t.Helper()
	q := AttachmentQueue{Store: data}
	for {
		d, err := q.ClaimEventDelivery(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if d == nil {
			return
		}
		if err = q.FinishEventDelivery(context.Background(), *d, "delivered", 200, "", time.Now()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAttachmentsWaitForEventAndRetainIndependentImmutableLeases(t *testing.T) {
	data := deliveryTestStore(t)
	ctx := context.Background()
	q := AttachmentQueue{Store: data}
	if _, err := data.InsertDeviceEventBatch(ctx, []NewDeviceEvent{faceEvent("photo-event", time.Now())}); err != nil {
		t.Fatal(err)
	}
	if d, err := q.ClaimEventDelivery(ctx); err != nil || d != nil {
		t.Fatal("attachment escaped undelivered parent", err)
	}
	event, err := data.ClaimEventDelivery(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = data.FinishEventDelivery(ctx, *event, "delivered", 200, "", time.Now()); err != nil {
		t.Fatal(err)
	}
	d, err := q.ClaimEventDelivery(ctx)
	if err != nil || d == nil {
		t.Fatal(err)
	}
	var body AttachmentMessage
	if err = json.Unmarshal(d.Body, &body); err != nil || body.EventID != "photo-event" || body.Availability != "not_provided" || !d.Attachment || d.EndpointURL != "https://school.invalid/events/attachments" {
		t.Fatalf("bad attachment contract: %v", err)
	}
	if next, err := q.ClaimEventDelivery(ctx); err != nil || next != nil {
		t.Fatal("leased attachment claimed twice")
	}
	if _, err = data.pool.Exec(ctx, `UPDATE event_attachment_deliveries SET lease_until=now()-interval '1 second'`); err != nil {
		t.Fatal(err)
	}
	resumed, err := q.ClaimEventDelivery(ctx)
	if err != nil || resumed == nil || resumed.Attempts != 2 || string(resumed.Body) != string(d.Body) {
		t.Fatal("crash recovery changed body", err)
	}
	if err = q.FinishEventDelivery(ctx, *d, "delivered", 200, "", time.Now()); err == nil {
		t.Fatal("stale worker owned lease")
	}
	if err = q.FinishEventDelivery(ctx, *resumed, "failed", 422, "receiver_rejected", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err = data.RetryAttachmentDelivery(ctx, d.ID, 1); err != nil {
		t.Fatal(err)
	}
	routes, _ := data.DeliveryRoutes(ctx)
	changed := routes[0]
	changed.EndpointURL = "https://other.invalid/events"
	if err = data.SaveDeliveryRoute(ctx, changed, 1); !errors.Is(err, ErrDeliveryRouteBusy) {
		t.Fatal("pending photo allowed route change", err)
	}
	paused := routes[0]
	paused.Enabled = false
	if err = data.SaveDeliveryRoute(ctx, paused, 1); err != nil {
		t.Fatal(err)
	}
	if next, err := q.ClaimEventDelivery(ctx); err != nil || next != nil {
		t.Fatal("paused photo claimed")
	}
	paused.Enabled = true
	if err = data.SaveDeliveryRoute(ctx, paused, 1); err != nil {
		t.Fatal(err)
	}
	finishTestAttachments(t, data)
	var attempts int
	var status string
	if err = data.pool.QueryRow(ctx, `SELECT status,attempts FROM event_deliveries WHERE id=$1`, event.ID).Scan(&status, &attempts); err != nil || status != "delivered" || attempts != 1 {
		t.Fatal("photo retry replayed attendance", err)
	}
}

func TestAttachmentBackfillIsExplicitBoundedAndDoesNotReplayAttendance(t *testing.T) {
	data := deliveryTestStore(t)
	ctx := context.Background()
	if _, err := data.InsertDeviceEventBatch(ctx, []NewDeviceEvent{faceEvent("historical", time.Now())}); err != nil {
		t.Fatal(err)
	}
	d, err := data.ClaimEventDelivery(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = data.FinishEventDelivery(ctx, *d, "delivered", 200, "", time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err = data.pool.Exec(ctx, `DELETE FROM event_attachment_deliveries`); err != nil {
		t.Fatal(err)
	} // pre-upgrade archive
	p, err := data.PreviewAttachmentBackfill(ctx, "DEVICE", 0)
	if err != nil || p.Eligible != 1 || p.ThroughDeliveryID != d.ID {
		t.Fatal("bad preview", err)
	}
	if _, err = data.QueueAttachmentBackfill(ctx, "DEVICE", p.ThroughDeliveryID, "changed", 1); !errors.Is(err, ErrBackfillPreviewChanged) {
		t.Fatal("unreviewed preview queued")
	}
	if n, err := data.QueueAttachmentBackfill(ctx, "DEVICE", p.ThroughDeliveryID, p.Token, 1); err != nil || n != 1 {
		t.Fatal("backfill failed", err)
	}
	if _, err = data.QueueAttachmentBackfill(ctx, "DEVICE", p.ThroughDeliveryID, p.Token, 1); !errors.Is(err, ErrBackfillPreviewChanged) {
		t.Fatal("backfill replayed")
	}
	finishTestAttachments(t, data)
	if next, err := data.ClaimEventDelivery(ctx); err != nil || next != nil {
		t.Fatal("backfill replayed attendance")
	}
	if p, err = data.PreviewAttachmentBackfill(ctx, "DEVICE", 0); err != nil || p.Eligible != 0 {
		t.Fatal("already queued attachments still eligible")
	}
}

func TestAttachmentQueuePreservesActualArchivedJPEG(t *testing.T) {
	data := deliveryTestStore(t)
	ctx := context.Background()
	e := faceEvent("multipart-photo", time.Now())
	metadata, _ := base64.StdEncoding.DecodeString(e.PayloadBase64)
	var jpegBytes bytes.Buffer
	if err := jpeg.Encode(&jpegBytes, image.NewGray(image.Rect(0, 0, 3, 3)), nil); err != nil {
		t.Fatal(err)
	}
	e.DataFormat = "boundaryData"
	payload := fmt.Sprintf("Content-Type: multipart/form-data; boundary=test\r\n\r\n--test\r\nContent-Disposition: form-data; name=\"AccessControllerEvent\"\r\nContent-Type: application/json\r\n\r\n%s\r\n--test\r\nContent-Disposition: form-data; name=\"Picture\"; filename=\"scan.jpg\"\r\nContent-Type: image/jpeg\r\n\r\n%s\r\n--test--\r\n", metadata, jpegBytes.Bytes())
	e.PayloadBase64 = base64.StdEncoding.EncodeToString([]byte(payload))
	if _, err := data.InsertDeviceEventBatch(ctx, []NewDeviceEvent{e}); err != nil {
		t.Fatal(err)
	}
	event, err := data.ClaimEventDelivery(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = data.FinishEventDelivery(ctx, *event, "delivered", 200, "", time.Now()); err != nil {
		t.Fatal(err)
	}
	photo, err := (AttachmentQueue{Store: data}).ClaimEventDelivery(ctx)
	if err != nil || photo == nil {
		t.Fatal(err)
	}
	var message AttachmentMessage
	if err = json.Unmarshal(photo.Body, &message); err != nil || message.Attachment == nil || message.Availability != "available" || message.Attachment.DataBase64 != base64.StdEncoding.EncodeToString(jpegBytes.Bytes()) {
		t.Fatal("JPEG did not survive archive/outbox", err)
	}
}
