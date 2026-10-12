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
	idleError    error
}

func (s *recoveryStore) PushSDKSessions(context.Context) ([]store.PushSDKSession, error) {
	if s.checkpoint == nil {
		return nil, nil
	}
	return []store.PushSDKSession{*s.checkpoint}, nil
}

func (s *recoveryStore) DeletePushSDKSession(context.Context, string) error {
	s.checkpoint = nil
	return nil
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
	if status == "offline" && s.idleError != nil {
		return activity.Event{}, s.idleError
	}
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

func TestIdleDeviceResumesWithLatestProof(t *testing.T) {
	for _, proactive := range []bool{false, true} {
		name := "before sweep"
		if proactive {
			name = "after idle sweep"
		}
		t.Run(name, func(t *testing.T) {
			svc, data, terminal := recoveryService(t)
			auth := loginRecoveryDevice(t, svc, terminal)
			session, _ := svc.sessions.Get(terminal.PushSDKSerial)
			session.NextChallengeAt = time.Now().Add(-24 * time.Hour)
			if err := svc.persistSession(context.Background(), session); err != nil {
				t.Fatal(err)
			}
			lastSeen := data.lastSeen
			if proactive {
				if err := svc.MaintainSessions(context.Background(), time.Now()); err != nil {
					t.Fatal(err)
				}
				if data.status != "offline" || data.checkpoint == nil || !session.Authenticated || session.Online || !data.lastSeen.Equal(lastSeen) {
					t.Fatal("idle transition did not preserve authentication and last contact")
				}
				assertActiveSessions(t, svc, 0)
				activities := len(data.activities)
				if err := svc.MaintainSessions(context.Background(), time.Now()); err != nil {
					t.Fatal(err)
				}
				if len(data.activities) != activities {
					t.Fatal("duplicate idle transition")
				}
			}
			// Invalid traffic cannot restore contact or destroy the valid checkpoint.
			status := data.status
			challenge := session.NextChallenge
			if got := sendRecoveryRequest(svc, "CommandRequest", "", strings.Repeat("0", 64)); got.Code != 401 {
				t.Fatalf("invalid proof: %d", got.Code)
			}
			if data.status != status || !data.lastSeen.Equal(lastSeen) || data.checkpoint.NextChallenge != challenge {
				t.Fatal("invalid proof changed state")
			}
			response := sendRecoveryRequest(svc, "CommandRequest", "", auth)
			if response.Code != 200 || response.Header().Get("My-Custom-Challenge") == "" || response.Header().Get("My-Custom-Challenge") == challenge {
				t.Fatalf("idle session did not resume and rotate challenge: %d", response.Code)
			}
			if data.status != "online" || !session.Online || !data.lastSeen.After(lastSeen) {
				t.Fatal("resume did not restore contact")
			}
			assertActiveSessions(t, svc, 1)
			// An already-used proof is rejected even after another long idle period.
			session.NextChallengeAt = time.Now().Add(-24 * time.Hour)
			if err := svc.MaintainSessions(context.Background(), time.Now()); err != nil {
				t.Fatal(err)
			}
			if got := sendRecoveryRequest(svc, "CommandRequest", "", auth); got.Code != 401 {
				t.Fatalf("used proof accepted: %d", got.Code)
			}
			if data.status != "offline" || session.Online {
				t.Fatal("replay restored online state")
			}
			assertActiveSessions(t, svc, 0)
		})
	}
}

func TestIdleCommitFailureRetainsOnlineStateForRetry(t *testing.T) {
	svc, data, terminal := recoveryService(t)
	loginRecoveryDevice(t, svc, terminal)
	session, _ := svc.sessions.Get(terminal.PushSDKSerial)
	session.NextChallengeAt = time.Now().Add(-61 * time.Second)
	data.idleError = errors.New("database unavailable")
	if err := svc.MaintainSessions(context.Background(), time.Now()); err == nil {
		t.Fatal("idle transition failure lost")
	}
	if !session.Authenticated || !session.Online || data.checkpoint == nil || data.status != "online" {
		t.Fatal("uncommitted transition changed state")
	}
	assertActiveSessions(t, svc, 1)
	data.idleError = nil
	if err := svc.MaintainSessions(context.Background(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if data.status != "offline" || data.checkpoint == nil || !session.Authenticated || session.Online {
		t.Fatal("idle retry failed")
	}
	assertActiveSessions(t, svc, 0)
}

func TestDeviceResumesAfterGatewayRestart(t *testing.T) {
	for _, pause := range []time.Duration{30 * time.Second, 24 * time.Hour} {
		t.Run(pause.String(), func(t *testing.T) {
			svc, data, terminal := recoveryService(t)
			auth := loginRecoveryDevice(t, svc, terminal)
			session, _ := svc.sessions.Get(terminal.PushSDKSerial)
			session.NextChallengeAt = time.Now().Add(-pause)
			if err := svc.persistSession(context.Background(), session); err != nil {
				t.Fatal(err)
			}
			svc.sessions = NewSessions()
			data.status = "offline"
			if err := svc.RestoreSessions(context.Background()); err != nil {
				t.Fatal(err)
			}
			assertActiveSessions(t, svc, 0)
			if err := svc.MaintainSessions(context.Background(), time.Now()); err != nil {
				t.Fatal(err)
			}
			response := sendRecoveryRequest(svc, "CommandRequest", "", auth)
			if response.Code != 200 || response.Header().Get("My-Custom-Challenge") == "" {
				t.Fatalf("restored session rejected: %d", response.Code)
			}
			if data.status != "online" || time.Since(data.lastSeen) > time.Second {
				t.Fatal("successful poll did not renew online session")
			}
			assertActiveSessions(t, svc, 1)
		})
	}
}

func TestFreshAuthenticationReplacesExistingSession(t *testing.T) {
	for _, idle := range []bool{false, true} {
		name := "online"
		if idle {
			name = "idle"
		}
		t.Run(name, func(t *testing.T) {
			svc, data, terminal := recoveryService(t)
			oldAuth := loginRecoveryDevice(t, svc, terminal)
			previous, _ := svc.sessions.Get(terminal.PushSDKSerial)
			if idle {
				previous.NextChallengeAt = time.Now().Add(-time.Hour)
				if err := svc.MaintainSessions(context.Background(), time.Now()); err != nil {
					t.Fatal(err)
				}
			}
			freshAuth := loginRecoveryDevice(t, svc, terminal)
			current, _ := svc.sessions.Get(terminal.PushSDKSerial)
			if current == previous || current.Salt == previous.Salt {
				t.Fatal("fresh authentication did not replace session")
			}
			assertActiveSessions(t, svc, 1)
			if response := sendRecoveryRequest(svc, "CommandRequest", "", oldAuth); response.Code != 401 {
				t.Fatalf("old proof accepted: %d", response.Code)
			}
			if response := sendRecoveryRequest(svc, "CommandRequest", "", freshAuth); response.Code != 200 {
				t.Fatalf("old retry disrupted new session: %d", response.Code)
			}
			if data.status != "online" || data.checkpoint == nil {
				t.Fatal("replacement session lost")
			}
			assertActiveSessions(t, svc, 1)
		})
	}
}

func TestMaintenanceDoesNotInterruptAnActiveExchange(t *testing.T) {
	svc, data, terminal := recoveryService(t)
	loginRecoveryDevice(t, svc, terminal)
	session, _ := svc.sessions.Get(terminal.PushSDKSerial)
	session.NextChallengeAt = time.Now().Add(-61 * time.Second)
	lock := svc.terminalLocks[terminal.PushSDKSerial]
	lock.Lock()
	if err := svc.MaintainSessions(context.Background(), time.Now()); err != nil {
		t.Fatal(err)
	}
	lock.Unlock()
	if data.status != "online" {
		t.Fatal("maintenance interrupted an in-flight exchange")
	}
	if err := svc.MaintainSessions(context.Background(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if data.status != "offline" || data.checkpoint == nil {
		t.Fatal("idle checkpoint was not retained")
	}
}

func TestLogoutRevokesIdleSession(t *testing.T) {
	svc, data, terminal := recoveryService(t)
	auth := loginRecoveryDevice(t, svc, terminal)
	session, _ := svc.sessions.Get(terminal.PushSDKSerial)
	session.NextChallengeAt = time.Now().Add(-time.Hour)
	if err := svc.MaintainSessions(context.Background(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if got := sendRecoveryRequest(svc, "Logout", "", auth); got.Code != 200 {
		t.Fatalf("logout: %d", got.Code)
	}
	if data.checkpoint != nil || data.status != "offline" {
		t.Fatal("logout did not revoke session")
	}
	if got := sendRecoveryRequest(svc, "CommandRequest", "", auth); got.Code != 401 {
		t.Fatalf("revoked proof accepted: %d", got.Code)
	}
	assertActiveSessions(t, svc, 0)
}

func TestLateLoginCannotDiscardAuthenticatedSession(t *testing.T) {
	svc, data, terminal := recoveryService(t)
	auth := loginRecoveryDevice(t, svc, terminal)
	session, _ := svc.sessions.Get(terminal.PushSDKSerial)
	session.CreatedAt = time.Now().Add(-time.Hour)
	if got := sendRecoveryRequest(svc, "Login", `{}`, ""); got.Code != 401 {
		t.Fatalf("late login: %d", got.Code)
	}
	if data.checkpoint == nil || !session.Authenticated {
		t.Fatal("late login discarded valid authentication")
	}
	if got := sendRecoveryRequest(svc, "CommandRequest", "", auth); got.Code != 200 {
		t.Fatalf("valid proof rejected: %d", got.Code)
	}
	assertActiveSessions(t, svc, 1)
}

func TestIncompleteLoginStillExpires(t *testing.T) {
	for _, sweep := range []bool{false, true} {
		t.Run(map[bool]string{false: "on request", true: "sweep"}[sweep], func(t *testing.T) {
			svc, data, terminal := recoveryService(t)
			if got := sendRecoveryRequest(svc, "AuthInfo", "", ""); got.Code != 200 {
				t.Fatalf("AuthInfo: %d", got.Code)
			}
			session, _ := svc.sessions.Get(terminal.PushSDKSerial)
			session.CreatedAt = time.Now().Add(-16 * time.Second)
			if sweep {
				if err := svc.MaintainSessions(context.Background(), time.Now()); err != nil {
					t.Fatal(err)
				}
			}
			if got := sendRecoveryRequest(svc, "Login", `{}`, ""); got.Code != 401 {
				t.Fatalf("expired login: %d", got.Code)
			}
			if data.checkpoint != nil || data.status != "offline" {
				t.Fatal("expired handshake retained")
			}
			assertActiveSessions(t, svc, 0)
		})
	}
}

func TestChangedConfigurationRevokesPersistedSession(t *testing.T) {
	svc, data, terminal := recoveryService(t)
	auth := loginRecoveryDevice(t, svc, terminal)
	svc.config.Terminals[0].Password = "changed-password"
	svc.sessions = NewSessions()
	data.status = "offline"
	if err := svc.RestoreSessions(context.Background()); err != nil {
		t.Fatal(err)
	}
	if data.checkpoint != nil {
		t.Fatal("checkpoint with old credentials restored")
	}
	if got := sendRecoveryRequest(svc, "CommandRequest", "", auth); got.Code != 401 {
		t.Fatalf("old credentials accepted: %d", got.Code)
	}
	assertActiveSessions(t, svc, 0)
}
