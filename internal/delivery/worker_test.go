package delivery

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/itplus/pushsdk-gateway/internal/store"
)

func TestDeliverySignatureAndReceipt(t *testing.T) {
	key := strings.Repeat("a", 64)
	body := []byte(`{"eventId":"exact-Event-007"}`)
	receiver := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ := io.ReadAll(r.Body)
		if string(got) != string(body) || r.Header.Get("X-PushSDK-Key-Id") != "tenant" || r.Header.Get("X-PushSDK-Signature") != Signature(key, r.Header.Get("X-PushSDK-Timestamp"), got) {
			t.Error("event bytes or signature changed")
		}
		fmt.Fprint(w, `{"schemaVersion":1,"eventId":"exact-Event-007","receipt":"stored"}`)
	}))
	defer receiver.Close()
	worker := New(nil, map[string]string{"tenant": key}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	worker.client = receiver.Client()
	status, code, err := worker.send(context.Background(), store.EventDelivery{EndpointURL: receiver.URL, SigningKeyID: "tenant", Body: body})
	if status != "delivered" || code != 200 || err != "" {
		t.Fatalf("result = %s %d %s", status, code, err)
	}
}

func TestDeliveryResponseClassification(t *testing.T) {
	for _, test := range []struct {
		name         string
		code         int
		body, status string
	}{
		{"outage", 503, ``, "pending"}, {"throttled", 429, ``, "pending"}, {"unauthorized", 401, ``, "failed"},
		{"invalid event", 422, ``, "failed"}, {"accepted without durable receipt", 202, ``, "failed"},
		{"redirect", 302, ``, "failed"}, {"wrong event", 200, `{"schemaVersion":1,"eventId":"other","receipt":"stored"}`, "failed"},
		{"extra receipt fields", 200, `{"schemaVersion":1,"eventId":"event","receipt":"stored","extra":true}`, "failed"},
		{"trailing receipt", 200, `{"schemaVersion":1,"eventId":"event","receipt":"stored"} {}`, "failed"},
		{"duplicate receipt fields", 200, `{"schemaVersion":1,"eventId":"other","eventId":"event","receipt":"stored"}`, "failed"},
		{"receipt field case", 200, `{"SchemaVersion":1,"eventId":"event","receipt":"stored"}`, "failed"},
		{"missing receipt field", 200, `{"schemaVersion":1,"eventId":"event"}`, "failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			receiver := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", "https://example.invalid")
				w.WriteHeader(test.code)
				fmt.Fprint(w, test.body)
			}))
			defer receiver.Close()
			worker := New(nil, map[string]string{"tenant": strings.Repeat("a", 64)}, slog.New(slog.NewTextHandler(io.Discard, nil)))
			worker.client.Transport = receiver.Client().Transport
			status, code, _ := worker.send(context.Background(), store.EventDelivery{EndpointURL: receiver.URL, SigningKeyID: "tenant", Body: []byte(`{"eventId":"event"}`)})
			if status != test.status || code != test.code {
				t.Fatalf("result = %s/%d", status, code)
			}
		})
	}
}

func TestRetryDelayIsBounded(t *testing.T) {
	if RetryDelay(1) != 5*time.Second || RetryDelay(2) != 10*time.Second || RetryDelay(100000) != 300*time.Second {
		t.Fatal("invalid retry schedule")
	}
}
