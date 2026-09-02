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

	"github.com/itplus/pushsdk-gateway/internal/activity"
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

func (s *Service) recordGatewayActivity(ctx context.Context, event activity.Event) error {
	stored, err := s.store.RecordGatewayActivity(ctx, event)
	if err != nil {
		return err
	}
	s.hub.Publish(stored)
	return nil
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

	var result protocolResponse
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
		if recordErr := s.recordGatewayActivity(request.Context(), activity.Event{
			Kind:     activity.KindPushSDKRejected,
			Terminal: terminal.SerialNumber,
			Message:  message,
			Fields:   map[string]any{"action": action, "status": status},
		}); recordErr != nil {
			s.logger.Error("record rejected PushSDK request", "terminal", terminal.SerialNumber, "action", action, "error", recordErr)
		}
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

type protocolResponse struct {
	Body      any
	Challenge string
	session   *Session
}

type successResponse struct {
	Status   int    `json:"status"`
	Code     string `json:"code"`
	ErrorMsg string `json:"errorMsg"`
}

func succeeded() successResponse {
	return successResponse{Status: http.StatusOK, Code: "0x00000000", ErrorMsg: "Succeeded."}
}

type authInfoRequest struct {
	Data struct {
		SecurityVersions []int `json:"securityVersion"`
	} `json:"data"`
}

type authInfoResponse struct {
	Data authInfoResponseData `json:"data"`
}

type authInfoResponseData struct {
	Challenge        string `json:"challenge"`
	Salt             string `json:"salt"`
	Iterations       int    `json:"iterations"`
	IsDataEncrypted  bool   `json:"isDataEncrypt"`
	SecurityVersions []int  `json:"securityVersion,omitempty"`
}

func (s *Service) authInfo(ctx context.Context, terminal config.Terminal, body []byte) (protocolResponse, error) {
	mode, err := payloadModeForAuthInfo(terminal, body)
	if err != nil {
		return protocolResponse{}, err
	}
	started, err := s.sessions.Start(terminal, mode)
	if err != nil {
		return protocolResponse{}, fmt.Errorf("create AuthInfo session: %w", err)
	}
	session := started.Session
	storedActivity, err := s.store.TransitionTerminalState(
		ctx,
		terminal.SerialNumber,
		"authenticating",
		nil,
		activity.Event{
			Kind:     activity.KindPushSDKAuthInfo,
			Terminal: terminal.SerialNumber,
			Message:  "terminal started authentication",
			Fields:   map[string]any{"payloadMode": mode, "securityVersion": terminal.SecurityVersion},
		},
	)
	if err != nil {
		s.sessions.Remove(terminal.PushSDKSerial)
		if started.ReplacedAuthenticatedSession {
			s.metrics.SessionsActive.Dec()
		}
		return protocolResponse{}, err
	}
	if started.ReplacedAuthenticatedSession {
		s.metrics.SessionsActive.Dec()
	}
	data := authInfoResponseData{Challenge: session.LoginChallenge, Salt: session.Salt, Iterations: session.Iterations, IsDataEncrypted: mode.IsEncrypted()}
	if mode.IsEncrypted() {
		data.SecurityVersions = []int{terminal.SecurityVersion}
	}
	s.hub.Publish(storedActivity)
	return protocolResponse{Body: authInfoResponse{Data: data}}, nil
}

// payloadModeForAuthInfo represents the two mutually exclusive protocol modes.
// A zero-length AuthInfo body is the documented no-negotiation variant: every
// following payload in that session is JSON without encryption parameters. A
// non-empty body must be the documented negotiation object and must offer the
// configured security version; that session then requires encryption for every
// JSON payload after AuthInfo.
func payloadModeForAuthInfo(terminal config.Terminal, body []byte) (PayloadMode, error) {
	if len(body) == 0 {
		return PlaintextPayload, nil
	}
	var payload authInfoRequest
	if err := decodeExactJSON(body, &payload); err != nil {
		return "", badRequest("AuthInfo body: %v", err)
	}
	if len(payload.Data.SecurityVersions) == 0 {
		return "", badRequest("AuthInfo must declare securityVersion when it has a body")
	}
	for _, version := range payload.Data.SecurityVersions {
		if version != 3 && version != 4 {
			return "", badRequest("AuthInfo securityVersion may contain only 3 or 4")
		}
		if version == terminal.SecurityVersion {
			return EncryptedPayload, nil
		}
	}
	return "", unprocessable("terminal does not offer configured security version")
}

type loginRequest struct {
	Data struct {
		Username      string `json:"username"`
		LoginPassword string `json:"loginPassword"`
	} `json:"data"`
}

type loginResponse struct {
	successResponse
	Data loginResponseData `json:"data"`
}

type loginResponseData struct {
	CommandInterval int `json:"commandInterval"`
	ErrorDelay      int `json:"errorDelay"`
}

func (s *Service) login(ctx context.Context, terminal config.Terminal, request *http.Request, requestBody []byte) (protocolResponse, *EncryptionParameters, error) {
	session, found := s.sessions.Get(terminal.PushSDKSerial)
	if !found {
		return protocolResponse{}, nil, &ProtocolError{Status: http.StatusUnauthorized, Message: "Login requires AuthInfo"}
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.LoginChallengeExpired(time.Now().UTC()) {
		s.sessions.Remove(terminal.PushSDKSerial)
		return protocolResponse{}, nil, &ProtocolError{Status: http.StatusUnauthorized, Message: "AuthInfo challenge has expired"}
	}
	body, params, err := payloadForSession(request, terminal, session, "Login", requestBody, true)
	if err != nil {
		return protocolResponse{}, nil, err
	}
	var payload loginRequest
	if err := decodeExactJSON(body, &payload); err != nil {
		return protocolResponse{}, nil, badRequest("Login body: %v", err)
	}
	if payload.Data.Username != terminal.Username || !constantTimeEqual(payload.Data.LoginPassword, expectedLoginPassword(terminal, session.Salt, session.LoginChallenge, session.Iterations)) {
		return protocolResponse{}, nil, &ProtocolError{Status: http.StatusUnauthorized, Message: "Login credentials are invalid"}
	}
	previousChallenge := session.NextChallenge
	next, err := session.IssueNextChallenge()
	if err != nil {
		return protocolResponse{}, nil, err
	}
	session.Authenticated = true
	storedActivity, err := s.store.TransitionTerminalState(
		ctx,
		terminal.SerialNumber,
		"online",
		nil,
		activity.Event{
			Kind:     activity.KindPushSDKLogin,
			Terminal: terminal.SerialNumber,
			Message:  "terminal authenticated",
			Fields:   map[string]any{"payloadMode": session.PayloadMode, "securityVersion": terminal.SecurityVersion},
		},
	)
	if err != nil {
		session.Authenticated = false
		session.NextChallenge = previousChallenge
		return protocolResponse{}, nil, err
	}
	s.metrics.SessionsActive.Inc()
	s.hub.Publish(storedActivity)
	result := protocolResponse{Body: loginResponse{successResponse: succeeded(), Data: loginResponseData{CommandInterval: terminal.CommandIntervalSeconds, ErrorDelay: terminal.ErrorDelaySeconds}}}
	result.Challenge = next
	result.session = session
	return result, params, nil
}

type commandRequestResponse struct {
	successResponse
	CommandNum int `json:"commandNum"`
}

type commandResultResponse struct {
	successResponse
	IsPendingCommand bool `json:"isPendingCommand"`
}

type eventResponseItem struct {
	UUID string `json:"UUID"`
	successResponse
}

func (s *Service) authenticated(ctx context.Context, terminal config.Terminal, action string, request *http.Request, requestBody []byte) (protocolResponse, *EncryptionParameters, error) {
	session, found := s.sessions.Get(terminal.PushSDKSerial)
	if !found {
		return protocolResponse{}, nil, &ProtocolError{Status: http.StatusUnauthorized, Message: action + " requires Login"}
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if !session.Authenticated {
		return protocolResponse{}, nil, &ProtocolError{Status: http.StatusUnauthorized, Message: action + " requires Login"}
	}
	if !constantTimeEqual(request.Header.Get("My-Custom-Auth"), expectedCustomAuth(terminal, session.Salt, session.NextChallenge)) {
		return protocolResponse{}, nil, &ProtocolError{Status: http.StatusUnauthorized, Message: "request authentication header is invalid"}
	}
	requiresBody := action == "CommandResult" || action == "Event"
	body, params, err := payloadForSession(request, terminal, session, action, requestBody, requiresBody)
	if err != nil {
		return protocolResponse{}, nil, err
	}
	var result protocolResponse
	switch action {
	case "CommandRequest":
		if len(body) != 0 {
			return protocolResponse{}, nil, badRequest("CommandRequest body must be empty")
		}
		result = protocolResponse{Body: commandRequestResponse{successResponse: succeeded(), CommandNum: 0}}
	case "CommandResult":
		if err := validateEmptyCommandResult(body); err != nil {
			return protocolResponse{}, nil, err
		}
		result = protocolResponse{Body: commandResultResponse{successResponse: succeeded(), IsPendingCommand: false}}
	case "Event":
		result, err = s.events(ctx, terminal, body)
		if err != nil {
			return protocolResponse{}, nil, err
		}
	case "Logout":
		if len(body) != 0 {
			return protocolResponse{}, nil, badRequest("Logout body must be empty")
		}
		storedActivity, err := s.store.TransitionTerminalState(
			ctx,
			terminal.SerialNumber,
			"offline",
			nil,
			activity.Event{
				Kind:     activity.KindPushSDKLogout,
				Terminal: terminal.SerialNumber,
				Message:  "terminal logged out",
			},
		)
		if err != nil {
			return protocolResponse{}, nil, err
		}
		// Keep the key material until this encrypted Logout response is written.
		// AuthInfo will replace this no-longer-authenticated session when the
		// terminal connects again.
		session.Authenticated = false
		s.metrics.SessionsActive.Dec()
		s.hub.Publish(storedActivity)
		result = protocolResponse{Body: succeeded()}
	}
	next, err := session.IssueNextChallenge()
	if err != nil {
		return protocolResponse{}, nil, err
	}
	result.Challenge = next
	result.session = session
	return result, params, nil
}

func (s *Service) events(ctx context.Context, terminal config.Terminal, body []byte) (protocolResponse, error) {
	parsed, err := ParseEventBatch(terminal.SerialNumber, body)
	if err != nil {
		s.metrics.EventsRejected.Inc()
		return protocolResponse{}, err
	}
	events := make([]store.NewDeviceEvent, 0, len(parsed))
	results := make([]eventResponseItem, 0, len(parsed))
	for _, event := range parsed {
		s.metrics.EventsReceived.WithLabelValues(event.Format).Inc()
		events = append(events, event.Record)
		results = append(results, eventResponseItem{UUID: event.UUID, successResponse: succeeded()})
	}
	persisted, err := s.store.InsertDeviceEventBatch(ctx, events)
	if err != nil {
		s.metrics.EventsRejected.Inc()
		return protocolResponse{}, fmt.Errorf("persist Event batch: %w", err)
	}
	for index := range parsed {
		if persisted[index].Inserted {
			s.metrics.EventsPersisted.Inc()
		}
		s.hub.Publish(persisted[index].Activity)
	}
	return protocolResponse{Body: results}, nil
}

type commandResult struct {
	CommandNum  *int   `json:"commandNum"`
	CommandList *[]any `json:"commandList"`
}

func validateEmptyCommandResult(body []byte) error {
	var payload commandResult
	if err := decodeExactJSON(body, &payload); err != nil {
		return badRequest("CommandResult body: %v", err)
	}
	if payload.CommandNum == nil || payload.CommandList == nil {
		return badRequest("CommandResult must contain commandNum and commandList")
	}
	if *payload.CommandNum != 0 || len(*payload.CommandList) != 0 {
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
	if session.PayloadMode.IsEncrypted() {
		params, err := encryptionFromRequest(request, terminal.SecurityVersion)
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

func (s *Service) respond(writer http.ResponseWriter, terminal config.Terminal, result protocolResponse, params *EncryptionParameters) error {
	payload, err := json.Marshal(result.Body)
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
	_ = json.NewEncoder(writer).Encode(successResponse{Status: status, Code: "0xFFFFFFFF", ErrorMsg: message})
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
