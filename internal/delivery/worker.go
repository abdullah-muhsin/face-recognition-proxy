package delivery

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/itplus/pushsdk-gateway/internal/store"
)

type Queue interface {
	ClaimEventDelivery(context.Context) (*store.EventDelivery, error)
	FinishEventDelivery(context.Context, store.EventDelivery, string, int, string, time.Time) error
}

type Worker struct {
	queue  Queue
	keys   map[string]string
	client *http.Client
	logger *slog.Logger
}

func New(queue Queue, keys map[string]string, logger *slog.Logger) *Worker {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &Worker{queue: queue, keys: keys, logger: logger, client: &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func Signature(key, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(timestamp + "\n"))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func RetryDelay(attempts int) time.Duration {
	delay := 5 * time.Second
	for i := 1; i < attempts && delay < 300*time.Second; i++ {
		delay *= 2
	}
	if delay > 300*time.Second {
		return 300 * time.Second
	}
	return delay
}

func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		for i := 0; i < 25; i++ {
			d, err := w.queue.ClaimEventDelivery(ctx)
			if err != nil {
				if ctx.Err() == nil {
					w.logger.Error("claim event delivery failed", "error", err)
				}
				break
			}
			if d == nil {
				break
			}
			status, code, failure := w.send(ctx, *d)
			if err = w.queue.FinishEventDelivery(ctx, *d, status, code, failure, time.Now().Add(RetryDelay(d.Attempts))); err != nil && ctx.Err() == nil {
				w.logger.Error("save event delivery result failed", "deliveryId", d.ID, "error", err)
			}
			if ctx.Err() != nil {
				return
			}
		}
	}
}

func (w *Worker) send(ctx context.Context, d store.EventDelivery) (string, int, string) {
	key, ok := w.keys[d.SigningKeyID]
	if !ok {
		return "failed", 0, "signing_key_unavailable"
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, d.EndpointURL, bytes.NewReader(d.Body))
	if err != nil {
		return "failed", 0, "invalid_destination"
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("X-PushSDK-Key-Id", d.SigningKeyID)
	request.Header.Set("X-PushSDK-Timestamp", timestamp)
	request.Header.Set("X-PushSDK-Signature", Signature(key, timestamp, d.Body))
	response, err := w.client.Do(request)
	if err != nil {
		return "pending", 0, "network_error"
	}
	defer response.Body.Close()
	code := response.StatusCode
	if code == http.StatusRequestTimeout || code == http.StatusTooManyRequests || code >= 500 {
		return "pending", code, "receiver_unavailable"
	}
	if code != http.StatusOK {
		return "failed", code, "receiver_rejected"
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 4097))
	if err != nil || len(body) > 4096 {
		return "failed", code, "invalid_receipt"
	}
	var source store.EventMessage
	if json.Unmarshal(d.Body, &source) != nil || !validReceipt(body, source.EventID, source.SchemaVersion) {
		return "failed", code, "invalid_receipt"
	}
	return "delivered", code, ""
}

func validReceipt(body []byte, eventID string, schemaVersion int) bool {
	decoder := json.NewDecoder(bytes.NewReader(body))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return false
	}
	seen := map[string]bool{}
	for decoder.More() {
		token, err := decoder.Token()
		name, ok := token.(string)
		if err != nil || !ok || seen[name] {
			return false
		}
		seen[name] = true
		switch name {
		case "schemaVersion":
			var version int
			if decoder.Decode(&version) != nil || version != schemaVersion {
				return false
			}
		case "eventId":
			var id string
			if decoder.Decode(&id) != nil || id != eventID {
				return false
			}
		case "receipt":
			var receipt string
			if decoder.Decode(&receipt) != nil || receipt != "stored" {
				return false
			}
		default:
			return false
		}
	}
	closing, err := decoder.Token()
	return err == nil && closing == json.Delim('}') && len(seen) == 3 && decoder.Decode(&struct{}{}) == io.EOF
}
