package pushsdk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/itplus/pushsdk-gateway/internal/config"
	"github.com/itplus/pushsdk-gateway/internal/monitor"
	"github.com/itplus/pushsdk-gateway/internal/store"
)

const maxRequestBytes = 8 << 20

type Service struct {
	config   config.Config
	sessions *Sessions
	store    *store.Store
	hub      *monitor.Hub
	metrics  *Metrics
	logger   *slog.Logger
}

func NewService(cfg config.Config, data *store.Store, hub *monitor.Hub, metrics *Metrics, logger *slog.Logger) *Service {
	return &Service{config: cfg, sessions: NewSessions(), store: data, hub: hub, metrics: metrics, logger: logger}
}

func (s *Service) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	serial, action, ok := parseRoute(request.URL.Path)
	if !ok {
		http.NotFound(writer, request)
		return
	}
	if request.Method != http.MethodPost {
		s.respondError(writer, http.StatusMethodNotAllowed, "method must be POST")
		return
	}
	terminal, found := s.config.TerminalByPushSDKSerial(serial)
	if !found {
		s.respondError(writer, http.StatusNotFound, "terminal is not registered")
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, maxRequestBytes)
	body, err := readAll(request.Body)
	if err != nil {
		s.respondError(writer, http.StatusRequestEntityTooLarge, "request body is too large")
		return
	}

	var result response
	var params *EncryptionParameters
	switch action {
	case "AuthInfo":
		if request.URL.RawQuery != "" {
			err = badRequest("AuthInfo does not permit query parameters")
		} else if len(body) > 0 {
			err = requireContentType(request, "application/json")
		} else if request.Header.Get("Content-Type") != "" {
			err = requireContentType(request, "application/json")
		}
		if err == nil {
			result, err = s.authInfo(request.Context(), terminal, body)
		}
	case "Login":
		result, params, err = s.login(request.Context(), terminal, request, body)
	case "CommandRequest", "CommandResult", "Event", "Logout":
		result, params, err = s.authenticated(request.Context(), terminal, action, request, body)
	default:
		err = &ProtocolError{Status: http.StatusNotFound, Message: "unsupported PushSDK action"}
	}
	if err != nil {
		status, message := protocolError(err)
		s.metrics.Requests.WithLabelValues(action, "rejected").Inc()
		s.logger.Warn("pushsdk request rejected", "terminal", terminal.SerialNumber, "action", action, "status", status, "error", message)
		s.hub.Publish(monitor.Event{Kind: "pushsdk.rejected", Terminal: terminal.SerialNumber, Message: message, Fields: map[string]any{"action": action, "status": status}})
		s.respondError(writer, status, message)
		return
	}
	s.metrics.Requests.WithLabelValues(action, "accepted").Inc()
	if err := s.respond(writer, terminal, result, params); err != nil {
		s.logger.Error("write PushSDK response", "terminal", terminal.SerialNumber, "action", action, "error", err)
	}
}

func parseRoute(path string) (serial, action string, ok bool) {
	parts := strings.Split(path, "/")
	if len(parts) != 10 || parts[1] != "iot" || parts[3] != "global" || parts[4] != "0-global" || parts[5] != "model" || parts[6] != "service" || parts[7] != "operate" || parts[8] != "PUSH" {
		return "", "", false
	}
	if parts[2] == "" || parts[9] == "" {
		return "", "", false
	}
	return parts[2], parts[9], true
}

type response struct {
	Status    int    `json:"status"`
	Code      string `json:"code"`
	ErrorMsg  string `json:"errorMsg"`
	Data      any    `json:"data,omitempty"`
	Challenge string
	session   *Session
}

func success(data any) response {
	return response{Status: http.StatusOK, Code: "0x00000000", ErrorMsg: "Succeeded.", Data: data}
}

type authInfoRequest struct {
	Data struct {
		SecurityVersions []int `json:"securityVersion"`
	} `json:"data"`
}

func (s *Service) authInfo(ctx context.Context, terminal config.Terminal, body []byte) (response, error) {
	encrypted, err := authInfoEncryption(terminal, body)
	if err != nil {
		return response{}, err
	}
	session, replacedAuthenticated, err := s.sessions.Start(terminal, encrypted)
	if err != nil {
		return response{}, fmt.Errorf("create AuthInfo session: %w", err)
	}
	if err := s.store.SetTerminalState(ctx, terminal.SerialNumber, "authenticating", nil); err != nil {
		return response{}, err
	}
	if replacedAuthenticated {
		s.metrics.SessionsActive.Dec()
	}
	s.hub.Publish(monitor.Event{Kind: "pushsdk.auth_info", Terminal: terminal.SerialNumber, Message: "terminal started authentication", Fields: map[string]any{"encrypted": encrypted, "securityVersion": terminal.Security}})
	return success(map[string]any{"challenge": session.LoginChallenge, "salt": session.Salt, "iterations": session.Iterations, "isDataEncrypt": encrypted, "securityVersion": []int{3, 4}}), nil
}

// authInfoEncryption represents the two mutually exclusive protocol modes.
// A zero-length AuthInfo body is the documented no-negotiation variant: every
// following payload in that session is JSON without encryption parameters. A
// non-empty body must be the documented negotiation object and must offer the
// configured security version; that session then requires encryption for every
// JSON payload after AuthInfo.
func authInfoEncryption(terminal config.Terminal, body []byte) (bool, error) {
	if len(body) == 0 {
		return false, nil
	}
	var payload authInfoRequest
	if err := decodeExactJSON(body, &payload); err != nil {
		return false, badRequest("AuthInfo body: %v", err)
	}
	if len(payload.Data.SecurityVersions) == 0 {
		return false, badRequest("AuthInfo must declare securityVersion when it has a body")
	}
	for _, version := range payload.Data.SecurityVersions {
		if version != 3 && version != 4 {
			return false, badRequest("AuthInfo securityVersion may contain only 3 or 4")
		}
		if version == terminal.Security {
			return true, nil
		}
	}
	return false, unprocessable("terminal does not offer configured security version")
}

type loginRequest struct {
	Data struct {
		Username      string `json:"username"`
		LoginPassword string `json:"loginPassword"`
	} `json:"data"`
}

func (s *Service) login(ctx context.Context, terminal config.Terminal, request *http.Request, requestBody []byte) (response, *EncryptionParameters, error) {
	session, found := s.sessions.Get(terminal.PushSDKSerial)
	if !found {
		return response{}, nil, &ProtocolError{Status: http.StatusUnauthorized, Message: "Login requires AuthInfo"}
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.LoginExpired(time.Now().UTC()) {
		s.sessions.Remove(terminal.PushSDKSerial)
		return response{}, nil, &ProtocolError{Status: http.StatusUnauthorized, Message: "AuthInfo challenge has expired"}
	}
	body, params, err := payloadForSession(request, terminal, session, "Login", requestBody, true)
	if err != nil {
		return response{}, nil, err
	}
	var payload loginRequest
	if err := decodeExactJSON(body, &payload); err != nil {
		return response{}, nil, badRequest("Login body: %v", err)
	}
	if payload.Data.Username != terminal.Username || !constantTimeEqual(payload.Data.LoginPassword, expectedLoginPassword(terminal, session.Salt, session.LoginChallenge, session.Iterations)) {
		return response{}, nil, &ProtocolError{Status: http.StatusUnauthorized, Message: "Login credentials are invalid"}
	}
	next, err := session.IssueNextChallenge()
	if err != nil {
		return response{}, nil, err
	}
	session.Authenticated = true
	if err := s.store.SetTerminalState(ctx, terminal.SerialNumber, "online", nil); err != nil {
		return response{}, nil, err
	}
	s.metrics.SessionsActive.Inc()
	s.hub.Publish(monitor.Event{Kind: "pushsdk.login", Terminal: terminal.SerialNumber, Message: "terminal authenticated", Fields: map[string]any{"encrypted": session.Encrypted, "securityVersion": terminal.Security}})
	result := success(map[string]any{"commandInterval": terminal.CommandSeconds, "errorDelay": terminal.ErrorDelay})
	result.Challenge = next
	result.session = session
	return result, params, nil
}

func (s *Service) authenticated(ctx context.Context, terminal config.Terminal, action string, request *http.Request, requestBody []byte) (response, *EncryptionParameters, error) {
	session, found := s.sessions.Get(terminal.PushSDKSerial)
	if !found {
		return response{}, nil, &ProtocolError{Status: http.StatusUnauthorized, Message: action + " requires Login"}
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if !session.Authenticated {
		return response{}, nil, &ProtocolError{Status: http.StatusUnauthorized, Message: action + " requires Login"}
	}
	if !constantTimeEqual(request.Header.Get("My-Custom-Auth"), expectedCustomAuth(terminal, session.Salt, session.NextChallenge)) {
		return response{}, nil, &ProtocolError{Status: http.StatusUnauthorized, Message: "request authentication header is invalid"}
	}
	requiresBody := action == "CommandResult" || action == "Event"
	body, params, err := payloadForSession(request, terminal, session, action, requestBody, requiresBody)
	if err != nil {
		return response{}, nil, err
	}
	var result response
	switch action {
	case "CommandRequest":
		if len(body) != 0 {
			return response{}, nil, badRequest("CommandRequest body must be empty")
		}
		result = success(map[string]any{"commandNum": 0, "commandList": []any{}})
	case "CommandResult":
		if err := validateEmptyCommandResult(body); err != nil {
			return response{}, nil, err
		}
		result = success(map[string]any{"isPendingCommand": false})
	case "Event":
		result, err = s.events(ctx, terminal, body)
		if err != nil {
			return response{}, nil, err
		}
	case "Logout":
		if len(body) != 0 {
			return response{}, nil, badRequest("Logout body must be empty")
		}
		// Keep the key material until this encrypted Logout response is written.
		// The session is already unauthenticated after this point, and AuthInfo
		// will replace it on the terminal's next connection.
		session.Authenticated = false
		s.metrics.SessionsActive.Dec()
		if err := s.store.SetTerminalState(ctx, terminal.SerialNumber, "offline", nil); err != nil {
			return response{}, nil, err
		}
		s.hub.Publish(monitor.Event{Kind: "pushsdk.logout", Terminal: terminal.SerialNumber, Message: "terminal logged out"})
		result = success(nil)
	}
	next, err := session.IssueNextChallenge()
	if err != nil {
		return response{}, nil, err
	}
	result.Challenge = next
	result.session = session
	return result, params, nil
}

func (s *Service) events(ctx context.Context, terminal config.Terminal, body []byte) (response, error) {
	parsed, err := ParseEventBatch(terminal.SerialNumber, body)
	if err != nil {
		s.metrics.EventsRejected.Inc()
		return response{}, err
	}
	records := make([]store.NewAttendanceRecord, 0, len(parsed))
	results := make([]map[string]any, 0, len(parsed))
	for _, event := range parsed {
		s.metrics.EventsReceived.WithLabelValues(event.Format).Inc()
		if event.Attendance != nil {
			records = append(records, *event.Attendance)
		}
		results = append(results, map[string]any{"UUID": event.UUID, "status": http.StatusOK, "code": "0x00000000", "errorMsg": "Succeeded."})
	}
	inserted, err := s.store.InsertAttendanceBatch(ctx, records)
	if err != nil {
		s.metrics.EventsRejected.Inc()
		return response{}, fmt.Errorf("persist Event batch: %w", err)
	}
	for range inserted {
		s.metrics.EventsAccepted.Inc()
	}
	for _, event := range parsed {
		if event.Attendance != nil {
			s.hub.Publish(monitor.Event{Kind: "attendance.received", Terminal: terminal.SerialNumber, Message: "attendance record persisted", Fields: map[string]any{"eventId": event.UUID, "format": event.Format, "employeeNumber": event.Attendance.EmployeeNumber}})
		} else {
			s.hub.Publish(monitor.Event{Kind: "pushsdk.event_ignored", Terminal: terminal.SerialNumber, Message: "non-attendance event acknowledged", Fields: map[string]any{"eventId": event.UUID, "format": event.Format, "reason": event.Reason}})
		}
	}
	return success(map[string]any{"eventList": results}), nil
}

type commandResult struct {
	Data struct {
		CommandNum  int   `json:"commandNum"`
		CommandList []any `json:"commandList"`
	} `json:"data"`
}

func validateEmptyCommandResult(body []byte) error {
	var payload commandResult
	if err := decodeExactJSON(body, &payload); err != nil {
		return badRequest("CommandResult body: %v", err)
	}
	if payload.Data.CommandNum != 0 || len(payload.Data.CommandList) != 0 {
		return unprocessable("CommandResult is invalid: this gateway does not issue device commands")
	}
	return nil
}

func encryptionFromRequest(request *http.Request, security int) (EncryptionParameters, error) {
	query := request.URL.Query()
	if len(query) != 3 {
		return EncryptionParameters{}, errors.New("only security, iv, and random query parameters are allowed")
	}
	for _, key := range []string{"security", "iv", "random"} {
		if len(query[key]) != 1 {
			return EncryptionParameters{}, fmt.Errorf("%s must appear exactly once", key)
		}
	}
	return ParseEncryptionParameters(query.Get("security"), query.Get("iv"), query.Get("random"), security)
}

func payloadForSession(request *http.Request, terminal config.Terminal, session *Session, action string, body []byte, required bool) ([]byte, *EncryptionParameters, error) {
	if session.Encrypted {
		params, err := encryptionFromRequest(request, terminal.Security)
		if err != nil {
			return nil, nil, badRequest("%s encryption parameters: %v", action, err)
		}
		if required && len(body) == 0 {
			return nil, nil, badRequest("%s body is required", action)
		}
		if len(body) == 0 {
			return body, &params, nil
		}
		if err := requireContentType(request, "application/octet-stream"); err != nil {
			return nil, nil, err
		}
		plain, err := decrypt(terminal, session.Salt, session.Iterations, params, body)
		if err != nil {
			return nil, nil, badRequest("%s decrypt: %v", action, err)
		}
		return plain, &params, nil
	}
	if request.URL.RawQuery != "" {
		return nil, nil, badRequest("%s does not permit query parameters without negotiated encryption", action)
	}
	if required {
		if err := requireContentType(request, "application/json"); err != nil {
			return nil, nil, err
		}
		if len(body) == 0 {
			return nil, nil, badRequest("%s body is required", action)
		}
	}
	return body, nil, nil
}

func requireContentType(request *http.Request, expected string) error {
	contentType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || contentType != expected {
		return &ProtocolError{Status: http.StatusUnsupportedMediaType, Message: "Content-Type must be " + expected}
	}
	return nil
}

func decodeExactJSON(body []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("trailing JSON value")
	}
	return nil
}

func (s *Service) respond(writer http.ResponseWriter, terminal config.Terminal, result response, params *EncryptionParameters) error {
	payload, err := json.Marshal(struct {
		Status   int    `json:"status"`
		Code     string `json:"code"`
		ErrorMsg string `json:"errorMsg"`
		Data     any    `json:"data,omitempty"`
	}{result.Status, result.Code, result.ErrorMsg, result.Data})
	if err != nil {
		return err
	}
	if params != nil {
		if result.session == nil {
			return errors.New("encrypted response has no request session")
		}
		payload, err = encrypt(terminal, result.session.Salt, result.session.Iterations, *params, payload)
		if err != nil {
			return err
		}
	}
	if result.Challenge != "" {
		writer.Header().Set("My-Custom-Challenge", result.Challenge)
	}
	if params != nil {
		writer.Header().Set("Content-Type", "application/octet-stream")
	} else {
		writer.Header().Set("Content-Type", "application/json")
	}
	writer.WriteHeader(http.StatusOK)
	_, err = writer.Write(payload)
	return err
}

func (s *Service) respondError(writer http.ResponseWriter, status int, message string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(response{Status: status, Code: "0xFFFFFFFF", ErrorMsg: message})
}

func protocolError(err error) (int, string) {
	var protocol *ProtocolError
	if errors.As(err, &protocol) {
		return protocol.Status, protocol.Message
	}
	return http.StatusInternalServerError, "internal gateway error"
}

func readAll(body io.ReadCloser) ([]byte, error) {
	defer body.Close()
	buffer := new(bytes.Buffer)
	if _, err := buffer.ReadFrom(body); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
