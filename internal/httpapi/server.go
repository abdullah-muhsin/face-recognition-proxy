package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/itplus/pushsdk-gateway/internal/accesscontrol"
	"github.com/itplus/pushsdk-gateway/internal/activity"
	"github.com/itplus/pushsdk-gateway/internal/config"
	"github.com/itplus/pushsdk-gateway/internal/monitor"
	"github.com/itplus/pushsdk-gateway/internal/store"
)

const (
	sessionCookie        = "pushsdk_admin_session"
	monitorHistoryLimit  = 100
	maxISAPICommandBytes = 8 << 20
)

type contextKey string

const administratorIdentityContextKey contextKey = "administrator-identity"

type Server struct {
	config      config.Config
	store       *store.Store
	hub         *monitor.Hub
	metrics     http.Handler
	logger      *slog.Logger
	metricsNets []netip.Prefix
}

func New(cfg config.Config, data *store.Store, hub *monitor.Hub, metrics http.Handler, logger *slog.Logger) (*Server, error) {
	prefixes := make([]netip.Prefix, 0, len(cfg.MetricsCIDRs))
	for _, cidr := range cfg.MetricsCIDRs {
		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			return nil, fmt.Errorf("METRICS_ALLOW_CIDRS %q: %w", cidr, err)
		}
		prefixes = append(prefixes, prefix)
	}
	return &Server{config: cfg, store: data, hub: hub, metrics: metrics, logger: logger, metricsNets: prefixes}, nil
}

func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /readyz", s.ready)
	mux.HandleFunc("GET /metrics", s.privateMetrics)
	mux.HandleFunc("POST /api/v1/auth/login", s.login)
	mux.HandleFunc("POST /api/v1/auth/logout", s.withSession(s.logout))
	mux.HandleFunc("GET /api/v1/admin/overview", s.withSession(s.overview))
	mux.HandleFunc("GET /api/v1/admin/events", s.withSession(s.deviceEvents))
	mux.HandleFunc("GET /api/v1/admin/events/{source}/{id}/payload", s.withSession(s.archiveEventPayload))
	mux.HandleFunc("GET /api/v1/admin/events/{id}/payload", s.withSession(s.deviceEventPayload))
	mux.HandleFunc("GET /api/v1/admin/isapi-commands", s.withSession(s.isapiCommands))
	mux.HandleFunc("GET /api/v1/admin/isapi-commands/{uuid}", s.withSession(s.isapiCommandPayload))
	mux.HandleFunc("POST /api/v1/admin/terminals/{serial}/isapi-commands", s.withSession(s.queueISAPICommand))
	mux.HandleFunc("POST /api/v1/admin/terminals/{serial}/retained-event-syncs", s.withSession(s.queueAccessEventSync))
	mux.HandleFunc("GET /api/v1/admin/retained-event-syncs/{uuid}", s.withSession(s.accessEventSyncRun))
	mux.HandleFunc("GET /api/v1/admin/activity", s.withSession(s.gatewayActivity))
	mux.HandleFunc("GET /ws/v1/monitor", s.withSession(s.monitorSocket))
	mux.HandleFunc("/", s.redirectRoot)
	mux.HandleFunc("GET /app/", s.webApp)
}

func (s *Server) health(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
}
func (s *Server) ready(writer http.ResponseWriter, request *http.Request) {
	if _, err := s.store.Overview(request.Context()); err != nil {
		writeError(writer, http.StatusServiceUnavailable, "database is unavailable")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]string{"status": "ready"})
}
func (s *Server) redirectRoot(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/" {
		http.NotFound(writer, request)
		return
	}
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		writeError(writer, http.StatusMethodNotAllowed, "method must be GET")
		return
	}
	http.Redirect(writer, request, "/app/", http.StatusPermanentRedirect)
}

// webApp serves static build assets by filename and the Vue entry document for
// history-mode browser routes. An asset request keeps the file server's normal
// 404 behavior; only extensionless administration-console paths resolve to
// index.html.
func (s *Server) webApp(writer http.ResponseWriter, request *http.Request) {
	relativePath := strings.TrimPrefix(request.URL.Path, "/app/")
	if relativePath == "" || path.Ext(relativePath) == "" {
		http.ServeFile(writer, request, filepath.Join(s.config.WebDir, "index.html"))
		return
	}
	http.StripPrefix("/app/", http.FileServer(http.Dir(s.config.WebDir))).ServeHTTP(writer, request)
}

type loginInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) login(writer http.ResponseWriter, request *http.Request) {
	var input loginInput
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	if input.Username == "" || input.Password == "" {
		writeError(writer, http.StatusBadRequest, "username and password are required")
		return
	}
	userID, valid, err := s.store.VerifyAdmin(request.Context(), input.Username, input.Password)
	if err != nil {
		s.internalError(writer, "verify admin login", err)
		return
	}
	if !valid {
		writeError(writer, http.StatusUnauthorized, "invalid credentials")
		return
	}
	rawToken, err := randomToken()
	if err != nil {
		s.internalError(writer, "create admin session", err)
		return
	}
	sum := sha256.Sum256([]byte(rawToken))
	storedActivity, err := s.store.CreateSessionWithActivity(
		request.Context(),
		userID,
		sum[:],
		time.Now().UTC().Add(s.config.SessionTTL),
		activity.Event{Kind: activity.KindAdminLogin, Message: "administrator session opened"},
	)
	if err != nil {
		s.internalError(writer, "persist admin session", err)
		return
	}
	http.SetCookie(writer, &http.Cookie{Name: sessionCookie, Value: rawToken, Path: "/", HttpOnly: true, Secure: s.config.CookieSecure, SameSite: http.SameSiteStrictMode, MaxAge: int(s.config.SessionTTL.Seconds())})
	s.hub.Publish(storedActivity)
	writeJSON(writer, http.StatusOK, map[string]string{"status": "authenticated"})
}
func (s *Server) logout(writer http.ResponseWriter, request *http.Request) {
	token, err := sessionToken(request)
	if err != nil {
		writeError(writer, http.StatusUnauthorized, "authentication required")
		return
	}
	sum := sha256.Sum256([]byte(token))
	storedActivity, err := s.store.DeleteSessionWithActivity(
		request.Context(),
		sum[:],
		activity.Event{Kind: activity.KindAdminLogout, Message: "administrator session closed"},
	)
	if err != nil {
		s.internalError(writer, "delete admin session", err)
		return
	}
	http.SetCookie(writer, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, Secure: s.config.CookieSecure, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	s.hub.Publish(storedActivity)
	writeJSON(writer, http.StatusOK, map[string]string{"status": "signed_out"})
}
func (s *Server) overview(writer http.ResponseWriter, request *http.Request) {
	overview, err := s.store.Overview(request.Context())
	if err != nil {
		s.internalError(writer, "load overview", err)
		return
	}
	writeJSON(writer, http.StatusOK, overview)
}
func (s *Server) deviceEvents(writer http.ResponseWriter, request *http.Request) {
	limit, offset, err := pagination(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	query, err := deviceEventQuery(request, limit, offset)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	page, err := s.store.QueryDeviceEvents(request.Context(), query)
	if err != nil {
		s.internalError(writer, "load device events", err)
		return
	}
	writeJSON(writer, http.StatusOK, page)
}

func deviceEventQuery(request *http.Request, limit, offset int) (store.DeviceEventQuery, error) {
	values := request.URL.Query()
	for key, values := range values {
		switch key {
		case "limit", "offset", "category", "subtype", "terminal", "source":
		default:
			return store.DeviceEventQuery{}, fmt.Errorf("unsupported events query parameter %q", key)
		}
		if len(values) != 1 {
			return store.DeviceEventQuery{}, fmt.Errorf("%s must appear at most once", key)
		}
	}
	query := store.DeviceEventQuery{Limit: limit, Offset: offset}
	category, categoryPresent := values["category"]
	if categoryPresent && category[0] != "all" {
		major, known := accesscontrol.MajorEventTypeForCategory(category[0])
		if !known {
			return store.DeviceEventQuery{}, errors.New("category must be one of all, alarm, exception, operation, or event")
		}
		query.MajorEventType = &major
	}
	if subtype, present := values["subtype"]; present {
		if !categoryPresent || query.MajorEventType == nil {
			return store.DeviceEventQuery{}, errors.New("subtype requires a category")
		}
		value, err := strconv.Atoi(subtype[0])
		if err != nil || value < 0 {
			return store.DeviceEventQuery{}, errors.New("subtype must be a non-negative integer")
		}
		query.SubEventType = &value
	}
	if terminal, present := values["terminal"]; present {
		if terminal[0] == "" {
			return store.DeviceEventQuery{}, errors.New("terminal must not be empty")
		}
		query.Terminal = terminal[0]
	}
	if source, present := values["source"]; present && source[0] != "all" {
		if source[0] != "pushsdk" && source[0] != "isapi" {
			return store.DeviceEventQuery{}, errors.New("source must be one of all, pushsdk, or isapi")
		}
		query.Source = source[0]
	}
	return query, nil
}

func (s *Server) deviceEventPayload(writer http.ResponseWriter, request *http.Request) {
	id, err := strconv.ParseInt(request.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(writer, http.StatusBadRequest, "event id must be a positive integer")
		return
	}
	payload, found, err := s.store.DeviceEventPayload(request.Context(), id)
	if err != nil {
		s.internalError(writer, "load device event payload", err)
		return
	}
	if !found {
		writeError(writer, http.StatusNotFound, "device event not found")
		return
	}
	response := deviceEventPayloadResponse{DeviceEventPayload: payload}
	if payload.PayloadBase64 != nil {
		if picture, found := accesscontrol.ExtractPicture(payload.DataFormat, *payload.PayloadBase64); found {
			response.Picture = &picture
		}
	}
	writeJSON(writer, http.StatusOK, response)
}

func (s *Server) archiveEventPayload(writer http.ResponseWriter, request *http.Request) {
	id, err := strconv.ParseInt(request.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(writer, http.StatusBadRequest, "event id must be a positive integer")
		return
	}
	payload, found, err := s.store.ArchiveEventPayload(request.Context(), request.PathValue("source"), id)
	if err != nil {
		if errors.Is(err, store.ErrUnknownArchiveEventSource) {
			writeError(writer, http.StatusBadRequest, err.Error())
			return
		}
		s.internalError(writer, "load event archive payload", err)
		return
	}
	if !found {
		writeError(writer, http.StatusNotFound, "device event not found")
		return
	}
	response := deviceEventPayloadResponse{DeviceEventPayload: payload}
	if payload.PayloadBase64 != nil {
		if picture, found := accesscontrol.ExtractPicture(payload.DataFormat, *payload.PayloadBase64); found {
			response.Picture = &picture
		}
	}
	writeJSON(writer, http.StatusOK, response)
}

type deviceEventPayloadResponse struct {
	store.DeviceEventPayload
	Picture *accesscontrol.Picture `json:"picture,omitempty"`
}

func (s *Server) gatewayActivity(writer http.ResponseWriter, request *http.Request) {
	limit, offset, err := pagination(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	page, err := s.store.GatewayActivities(request.Context(), limit, offset)
	if err != nil {
		s.internalError(writer, "load gateway activity", err)
		return
	}
	writeJSON(writer, http.StatusOK, page)
}

type isapiCommandInput struct {
	Method           string  `json:"method"`
	URL              string  `json:"url"`
	DataFormat       string  `json:"dataFormat"`
	TextData         *string `json:"textData"`
	DataBase64       *string `json:"dataBase64"`
	ExpiresInSeconds *int    `json:"expiresInSeconds"`
}

func (s *Server) queueISAPICommand(writer http.ResponseWriter, request *http.Request) {
	identity, found := administratorIdentity(request.Context())
	if !found {
		s.internalError(writer, "load administrator identity for ISAPI command", errors.New("identity missing from authenticated request"))
		return
	}
	input, err := parseISAPICommandInput(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	command, stored, err := s.store.QueueISAPICommand(request.Context(), store.NewISAPICommand{
		TerminalSerialNumber: request.PathValue("serial"),
		CreatedBy:            identity,
		Method:               input.Method,
		URL:                  input.URL,
		DataFormat:           input.DataFormat,
		Data:                 input.Data,
		ExpiresAt:            time.Now().UTC().Add(time.Duration(input.ExpiresInSeconds) * time.Second),
	})
	if errors.Is(err, store.ErrISAPICommandTerminalNotFound) {
		writeError(writer, http.StatusNotFound, err.Error())
		return
	}
	if errors.Is(err, store.ErrISAPICommandTerminalOffline) {
		writeError(writer, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		s.internalError(writer, "queue ISAPI command", err)
		return
	}
	s.hub.Publish(stored)
	writeJSON(writer, http.StatusCreated, command)
}

func (s *Server) queueAccessEventSync(writer http.ResponseWriter, request *http.Request) {
	if request.URL.RawQuery != "" {
		writeError(writer, http.StatusBadRequest, "retained event sync does not accept query parameters")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(writer, request.Body, 1))
	if err != nil {
		writeError(writer, http.StatusRequestEntityTooLarge, "retained event sync request must not have a body")
		return
	}
	if len(body) != 0 {
		writeError(writer, http.StatusBadRequest, "retained event sync request must not have a body")
		return
	}
	identity, found := administratorIdentity(request.Context())
	if !found {
		s.internalError(writer, "load administrator identity for retained event sync", errors.New("identity missing from authenticated request"))
		return
	}
	run, activities, err := s.store.QueueAccessEventSync(request.Context(), request.PathValue("serial"), identity)
	if errors.Is(err, store.ErrAccessEventSyncTerminalNotFound) {
		writeError(writer, http.StatusNotFound, err.Error())
		return
	}
	if errors.Is(err, store.ErrAccessEventSyncTerminalOffline) || errors.Is(err, store.ErrAccessEventSyncInProgress) {
		writeError(writer, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		s.internalError(writer, "queue retained event sync", err)
		return
	}
	for _, stored := range activities {
		s.hub.Publish(stored)
	}
	writeJSON(writer, http.StatusCreated, run)
}

func (s *Server) accessEventSyncRun(writer http.ResponseWriter, request *http.Request) {
	run, found, err := s.store.AccessEventSyncRun(request.Context(), request.PathValue("uuid"))
	if err != nil {
		s.internalError(writer, "load retained event sync", err)
		return
	}
	if !found {
		writeError(writer, http.StatusNotFound, "retained event sync not found")
		return
	}
	writeJSON(writer, http.StatusOK, run)
}

type parsedISAPICommandInput struct {
	Method           string
	URL              string
	DataFormat       string
	Data             []byte
	ExpiresInSeconds int
}

func parseISAPICommandInput(request *http.Request) (parsedISAPICommandInput, error) {
	var input isapiCommandInput
	if err := decodeISAPICommandJSON(request, &input); err != nil {
		return parsedISAPICommandInput{}, err
	}
	if input.ExpiresInSeconds == nil || *input.ExpiresInSeconds < 1 || *input.ExpiresInSeconds > 3600 {
		return parsedISAPICommandInput{}, errors.New("expiresInSeconds must be an integer from 1 to 3600")
	}
	if input.Method != "GET" && input.Method != "POST" && input.Method != "PUT" && input.Method != "DELETE" {
		return parsedISAPICommandInput{}, errors.New("method must be GET, POST, PUT, or DELETE")
	}
	if err := validateISAPIURL(input.URL); err != nil {
		return parsedISAPICommandInput{}, err
	}
	parsed := parsedISAPICommandInput{
		Method:           input.Method,
		URL:              input.URL,
		DataFormat:       input.DataFormat,
		Data:             []byte{},
		ExpiresInSeconds: *input.ExpiresInSeconds,
	}
	switch input.DataFormat {
	case "noData":
		if input.TextData != nil || input.DataBase64 != nil {
			return parsedISAPICommandInput{}, errors.New("noData commands must not include textData or dataBase64")
		}
	case "jsonData", "xmlData":
		if input.TextData == nil || input.DataBase64 != nil || *input.TextData == "" {
			return parsedISAPICommandInput{}, errors.New(input.DataFormat + " commands require non-empty textData and must not include dataBase64")
		}
		parsed.Data = []byte(*input.TextData)
	case "boundaryData":
		if input.TextData != nil || input.DataBase64 == nil || *input.DataBase64 == "" {
			return parsedISAPICommandInput{}, errors.New("boundaryData commands require non-empty dataBase64 and must not include textData")
		}
		decoded, err := base64.StdEncoding.DecodeString(*input.DataBase64)
		if err != nil || base64.StdEncoding.EncodeToString(decoded) != *input.DataBase64 {
			return parsedISAPICommandInput{}, errors.New("boundaryData dataBase64 must be canonical standard base64")
		}
		parsed.Data = decoded
	default:
		return parsedISAPICommandInput{}, errors.New("dataFormat must be jsonData, xmlData, boundaryData, or noData")
	}
	if len(parsed.Data) > maxISAPICommandBytes {
		return parsedISAPICommandInput{}, errors.New("ISAPI command data must not exceed 8 MiB")
	}
	return parsed, nil
}

func validateISAPIURL(raw string) error {
	if len(raw) < len("/ISAPI/")+1 || len(raw) > 4096 || !strings.HasPrefix(raw, "/ISAPI/") {
		return errors.New("url must be an absolute /ISAPI/ path from 8 to 4096 bytes")
	}
	if strings.Count(raw, "?") > 1 || strings.Contains(raw, "#") {
		return errors.New("url must not contain an ambiguous query or fragment")
	}
	for index := 0; index < len(raw); index++ {
		if raw[index] <= 0x20 || raw[index] >= 0x7f {
			return errors.New("url must contain only visible ASCII bytes")
		}
	}
	path := raw
	if queryAt := strings.IndexByte(path, '?'); queryAt >= 0 {
		path = path[:queryAt]
	}
	for _, segment := range strings.Split(strings.TrimPrefix(path, "/ISAPI/"), "/") {
		if segment == "" || segment == "." || segment == ".." {
			return errors.New("url path must contain explicit non-relative segments")
		}
	}
	return nil
}

func (s *Server) isapiCommands(writer http.ResponseWriter, request *http.Request) {
	limit, offset, err := pagination(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	terminal, err := isapiCommandTerminalFilter(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	page, err := s.store.QueryISAPICommands(request.Context(), terminal, limit, offset)
	if err != nil {
		s.internalError(writer, "load ISAPI commands", err)
		return
	}
	writeJSON(writer, http.StatusOK, page)
}

func isapiCommandTerminalFilter(request *http.Request) (string, error) {
	values := request.URL.Query()
	for key, value := range values {
		if key != "limit" && key != "offset" && key != "terminal" {
			return "", fmt.Errorf("unsupported ISAPI command query parameter %q", key)
		}
		if len(value) != 1 {
			return "", fmt.Errorf("%s must appear at most once", key)
		}
	}
	if terminal, present := values["terminal"]; present {
		if terminal[0] == "" {
			return "", errors.New("terminal must not be empty")
		}
		return terminal[0], nil
	}
	return "", nil
}

func (s *Server) isapiCommandPayload(writer http.ResponseWriter, request *http.Request) {
	payload, found, err := s.store.ISAPICommandPayload(request.Context(), request.PathValue("uuid"))
	if err != nil {
		s.internalError(writer, "load ISAPI command payload", err)
		return
	}
	if !found {
		writeError(writer, http.StatusNotFound, "ISAPI command not found")
		return
	}
	writeJSON(writer, http.StatusOK, payload)
}

func (s *Server) withSession(next http.HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		token, err := sessionToken(request)
		if err != nil {
			writeError(writer, http.StatusUnauthorized, "authentication required")
			return
		}
		sum := sha256.Sum256([]byte(token))
		identity, valid, err := s.store.AdminIdentityForSession(request.Context(), sum[:])
		if err != nil {
			s.internalError(writer, "validate admin session", err)
			return
		}
		if !valid {
			writeError(writer, http.StatusUnauthorized, "authentication required")
			return
		}
		next(writer, request.WithContext(context.WithValue(request.Context(), administratorIdentityContextKey, identity)))
	}
}

func administratorIdentity(ctx context.Context) (store.AdminIdentity, bool) {
	identity, found := ctx.Value(administratorIdentityContextKey).(store.AdminIdentity)
	return identity, found
}

func (s *Server) monitorSocket(writer http.ResponseWriter, request *http.Request) {
	upgrader := websocket.Upgrader{CheckOrigin: func(request *http.Request) bool { return sameOrigin(request) }}
	connection, err := upgrader.Upgrade(writer, request, nil)
	if err != nil {
		return
	}
	defer connection.Close()
	client, unsubscribe := s.hub.Subscribe()
	defer unsubscribe()
	history, err := s.store.GatewayActivities(request.Context(), monitorHistoryLimit, 0)
	if err != nil {
		s.logger.Error("load gateway activity for monitor", "error", err)
		return
	}
	for index := len(history.Activities) - 1; index >= 0; index-- {
		if err := connection.WriteJSON(history.Activities[index]); err != nil {
			return
		}
	}
	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	for {
		select {
		case message := <-client:
			if err := connection.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ping.C:
			if err := connection.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-request.Context().Done():
			return
		}
	}
}

func (s *Server) privateMetrics(writer http.ResponseWriter, request *http.Request) {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		writeError(writer, http.StatusForbidden, "metrics access denied")
		return
	}
	address, err := netip.ParseAddr(host)
	if err != nil {
		writeError(writer, http.StatusForbidden, "metrics access denied")
		return
	}
	for _, allowed := range s.metricsNets {
		if allowed.Contains(address) {
			s.metrics.ServeHTTP(writer, request)
			return
		}
	}
	writeError(writer, http.StatusForbidden, "metrics access denied")
}

func (s *Server) internalError(writer http.ResponseWriter, message string, err error) {
	s.logger.Error(message, "error", err)
	writeError(writer, http.StatusInternalServerError, "internal server error")
}

func sessionToken(request *http.Request) (string, error) {
	cookie, err := request.Cookie(sessionCookie)
	if err != nil || cookie.Value == "" {
		return "", errors.New("session cookie is missing")
	}
	return cookie.Value, nil
}
func randomToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
func decodeJSON(request *http.Request, target any) error {
	if contentType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type")); err != nil || contentType != "application/json" {
		return errors.New("Content-Type must be application/json")
	}
	decoder := json.NewDecoder(io.LimitReader(request.Body, 4097))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid JSON request")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request contains a trailing JSON value")
	}
	return nil
}

func decodeISAPICommandJSON(request *http.Request, target any) error {
	if contentType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type")); err != nil || contentType != "application/json" {
		return errors.New("Content-Type must be application/json")
	}
	maxJSONBytes := int64(base64.StdEncoding.EncodedLen(maxISAPICommandBytes) + 8192)
	if request.ContentLength > maxJSONBytes {
		return errors.New("ISAPI command request must not exceed the 8 MiB command-data limit")
	}
	decoder := json.NewDecoder(io.LimitReader(request.Body, maxJSONBytes+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errors.New("invalid JSON request")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request contains a trailing JSON value")
	}
	return nil
}
func pagination(request *http.Request) (int, int, error) {
	limit, offset := 50, 0
	if raw := request.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 200 {
			return 0, 0, errors.New("limit must be an integer from 1 to 200")
		}
		limit = value
	}
	if raw := request.URL.Query().Get("offset"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 {
			return 0, 0, errors.New("offset must be a non-negative integer")
		}
		offset = value
	}
	return limit, offset, nil
}
func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
func writeError(writer http.ResponseWriter, status int, message string) {
	writeJSON(writer, status, map[string]string{"error": message})
}
func sameOrigin(request *http.Request) bool {
	origin := request.Header.Get("Origin")
	if origin == "" {
		return false
	}
	proto := request.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		proto = "http"
		if request.TLS != nil {
			proto = "https"
		}
	}
	return origin == proto+"://"+request.Host
}
