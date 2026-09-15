package pushsdk

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/itplus/pushsdk-gateway/internal/activity"
	"github.com/itplus/pushsdk-gateway/internal/config"
	"github.com/itplus/pushsdk-gateway/internal/monitor"
	"github.com/itplus/pushsdk-gateway/internal/store"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

type recoveryStore struct {
	*store.Store // Unexpected calls fail the test instead of silently succeeding.
	checkpoint   *store.PushSDKSession
	status       string
	lastSeen     time.Time
	activities   []activity.Event
	expiryError  error
}

func (s *recoveryStore) UpsertPushSDKSession(_ context.Context, state store.PushSDKSession) error {
	s.checkpoint = &state
	if state.Authenticated {
		s.lastSeen = *state.NextChallengeAt
	}
	return nil
}
func (s *recoveryStore) RecordGatewayActivity(_ context.Context, event activity.Event) (activity.Event, error) {
	s.activities = append(s.activities, event)
	return event, nil
}
func (s *recoveryStore) TransitionTerminalState(ctx context.Context, _ string, status string, _ *string, event activity.Event) (activity.Event, error) {
	s.status = status
	return s.RecordGatewayActivity(ctx, event)
}
func (s *recoveryStore) ExpirePushSDKSession(ctx context.Context, serial, reason string) (*activity.Event, error) {
	if s.expiryError != nil {
		return nil, s.expiryError
	}
	s.checkpoint = nil
	if s.status == "offline" {
		return nil, nil
	}
	s.status = "offline"
	event, _ := s.RecordGatewayActivity(ctx, activity.Event{Kind: activity.KindPushSDKSessionExpired, Terminal: serial, Message: reason})
	return &event, nil
}
func (s *recoveryStore) ClaimISAPICommands(context.Context, string, int) ([]store.ISAPICommandDelivery, []activity.Event, error) {
	return nil, nil, nil
}

func recoveryService(t *testing.T) (*Service, *recoveryStore, config.Terminal) {
	t.Helper()
	terminal := config.Terminal{SerialNumber: "DEVICE-1", PushSDKSerial: "PUSH-1", Username: "gateway", Password: "test-password", LoginPasswordDigest: "sha256", SecurityVersion: 4, CommandIntervalSeconds: 5, ErrorDelaySeconds: 30}
	data := &recoveryStore{status: "offline"}
	svc := NewService(config.Config{Terminals: []config.Terminal{terminal}}, data, monitor.NewHub(), NewMetrics(prometheus.NewRegistry()), slog.New(slog.NewTextHandler(io.Discard, nil)))
	return svc, data, terminal
}

func sendRecoveryRequest(svc *Service, action, body, auth string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/iot/PUSH-1/global/0-global/model/service/operate/PUSH/"+action, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if auth != "" {
		request.Header.Set("My-Custom-Auth", auth)
	}
	response := httptest.NewRecorder()
	svc.ServeHTTP(response, request)
	return response
}

func loginRecoveryDevice(t *testing.T, svc *Service, terminal config.Terminal) string {
	t.Helper()
	response := sendRecoveryRequest(svc, "AuthInfo", "", "")
	if response.Code != 200 {
		t.Fatalf("AuthInfo: %d %s", response.Code, response.Body)
	}
	var info authInfoResponse
	if err := json.Unmarshal(response.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	password := expectedLoginPassword(terminal, info.Data.Salt, info.Data.Challenge, info.Data.Iterations)
	body, _ := json.Marshal(map[string]any{"data": map[string]string{"username": terminal.Username, "loginPassword": password}})
	response = sendRecoveryRequest(svc, "Login", string(body), "")
	if response.Code != 200 {
		t.Fatalf("Login: %d %s", response.Code, response.Body)
	}
	return expectedCustomAuth(terminal, info.Data.Salt, response.Header().Get("My-Custom-Challenge"))
}

func assertActiveSessions(t *testing.T, svc *Service, want float64) {
	t.Helper()
	metric := &dto.Metric{}
	if err := svc.metrics.SessionsActive.Write(metric); err != nil {
		t.Fatal(err)
	}
	if metric.GetGauge().GetValue() != want {
		t.Fatalf("active sessions = %v, want %v", metric.GetGauge().GetValue(), want)
	}
}

func TestExpiredDeviceCanAuthenticateAgainWithoutGatewayReset(t *testing.T) {
	for _, proactive := range []bool{false, true} {
		name := "on request"
		if proactive {
			name = "silent device sweep"
		}
		t.Run(name, func(t *testing.T) {
			svc, data, terminal := recoveryService(t)
			oldAuth := loginRecoveryDevice(t, svc, terminal)
			assertActiveSessions(t, svc, 1)
			session, _ := svc.sessions.Get(terminal.PushSDKSerial)
			session.NextChallengeAt = time.Now().Add(-16 * time.Second)
			lastSeen := data.lastSeen
			if proactive {
				if err := svc.ExpireSessions(context.Background(), time.Now()); err != nil {
					t.Fatal(err)
				}
			}
			response := sendRecoveryRequest(svc, "Event", `{"eventNum":0,"eventList":[]}`, oldAuth)
			if response.Code != 401 || response.Header().Get("My-Custom-Challenge") != "" {
				t.Fatalf("expired response: %d %v", response.Code, response.Header())
			}
			var failure successResponse
			if err := json.Unmarshal(response.Body.Bytes(), &failure); err != nil {
				t.Fatal(err)
			}
			if failure.Code != invalidSessionCode {
				t.Fatalf("failure = %#v", failure)
			}
			if data.status != "offline" || data.checkpoint != nil || !data.lastSeen.Equal(lastSeen) {
				t.Fatalf("expiry retained stale online state: %#v", data)
			}
			assertActiveSessions(t, svc, 0)
			if err := svc.ExpireSessions(context.Background(), time.Now()); err != nil {
				t.Fatal(err)
			}
			assertActiveSessions(t, svc, 0)
			freshAuth := loginRecoveryDevice(t, svc, terminal)
			// A stale retry must not destroy the new, authenticated session.
			if got := sendRecoveryRequest(svc, "CommandRequest", "", oldAuth); got.Code != 401 {
				t.Fatalf("old proof accepted: %d", got.Code)
			}
			if got := sendRecoveryRequest(svc, "CommandRequest", "", freshAuth); got.Code != 200 {
				t.Fatalf("fresh poll rejected: %d %s", got.Code, got.Body)
			}
			if data.status != "online" || !data.lastSeen.After(lastSeen) {
				t.Fatal("fresh authentication did not restore online/last seen")
			}
			assertActiveSessions(t, svc, 1)
		})
	}
}

func TestExpiryCommitFailureRetainsSessionForRetry(t *testing.T) {
	svc, data, terminal := recoveryService(t)
	loginRecoveryDevice(t, svc, terminal)
	session, _ := svc.sessions.Get(terminal.PushSDKSerial)
	session.NextChallengeAt = time.Now().Add(-16 * time.Second)
	data.expiryError = errors.New("database unavailable")
	if err := svc.ExpireSessions(context.Background(), time.Now()); err == nil {
		t.Fatal("expiry failure lost")
	}
	if !session.Authenticated || data.checkpoint == nil || data.status != "online" {
		t.Fatal("uncommitted expiry discarded session")
	}
	assertActiveSessions(t, svc, 1)
	data.expiryError = nil
	if err := svc.ExpireSessions(context.Background(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if data.status != "offline" || data.checkpoint != nil {
		t.Fatal("expiry retry failed")
	}
	assertActiveSessions(t, svc, 0)
}

func TestMaintenanceDoesNotInterruptAnActiveExchange(t *testing.T) {
	svc, data, terminal := recoveryService(t)
	loginRecoveryDevice(t, svc, terminal)
	session, _ := svc.sessions.Get(terminal.PushSDKSerial)
	session.NextChallengeAt = time.Now().Add(-16 * time.Second)
	lock := svc.terminalLocks[terminal.PushSDKSerial]
	lock.Lock()
	if err := svc.ExpireSessions(context.Background(), time.Now()); err != nil {
		t.Fatal(err)
	}
	lock.Unlock()
	if data.status != "online" {
		t.Fatal("maintenance expired an in-flight exchange")
	}
	if err := svc.ExpireSessions(context.Background(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if data.status != "offline" {
		t.Fatal("maintenance did not expire idle session")
	}
}
