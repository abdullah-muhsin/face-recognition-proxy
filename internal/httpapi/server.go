package httpapi

import (
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
	"github.com/itplus/pushsdk-gateway/internal/activity"
	"github.com/itplus/pushsdk-gateway/internal/config"
	"github.com/itplus/pushsdk-gateway/internal/monitor"
	"github.com/itplus/pushsdk-gateway/internal/store"
)

const (
	sessionCookie       = "pushsdk_admin_session"
	monitorHistoryLimit = 100
)

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
	mux.HandleFunc("GET /api/v1/admin/events/{id}/payload", s.withSession(s.deviceEventPayload))
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
// 404 behavior; only extensionless operator-console paths resolve to index.html.
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
		activity.Event{Kind: activity.KindAdminLogin, Message: "operator session opened"},
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
		activity.Event{Kind: activity.KindAdminLogout, Message: "operator session closed"},
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
	page, err := s.store.DeviceEvents(request.Context(), limit, offset)
	if err != nil {
		s.internalError(writer, "load device events", err)
		return
	}
	writeJSON(writer, http.StatusOK, page)
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
	writeJSON(writer, http.StatusOK, payload)
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

func (s *Server) withSession(next http.HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		token, err := sessionToken(request)
		if err != nil {
			writeError(writer, http.StatusUnauthorized, "authentication required")
			return
		}
		sum := sha256.Sum256([]byte(token))
		valid, err := s.store.SessionValid(request.Context(), sum[:])
		if err != nil {
			s.internalError(writer, "validate admin session", err)
			return
		}
		if !valid {
			writeError(writer, http.StatusUnauthorized, "authentication required")
			return
		}
		next(writer, request)
	}
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
