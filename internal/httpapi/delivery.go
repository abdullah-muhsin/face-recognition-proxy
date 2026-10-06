package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/itplus/pushsdk-gateway/internal/store"
	"github.com/jackc/pgx/v5"
)

func validDeliveryEndpoint(value string) bool {
	u, err := url.Parse(value)
	return err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && u.Fragment == "" && u.Opaque == "" && len(value) <= 2048
}

func (s *Server) deliveryConfiguration(writer http.ResponseWriter, request *http.Request) {
	routes, err := s.store.DeliveryRoutes(request.Context())
	if err != nil {
		s.internalError(writer, "list delivery routes", err)
		return
	}
	keys := []string{}
	for key := range s.config.DeliverySigningKeys {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	writeJSON(writer, http.StatusOK, map[string]any{"routes": routes, "signingKeyIds": keys})
}

func (s *Server) saveDeliveryRoute(writer http.ResponseWriter, request *http.Request) {
	identity, ok := administratorIdentity(request.Context())
	if !ok {
		writeError(writer, 401, "administrator required")
		return
	}
	terminal, ok := s.config.TerminalBySerialNumber(request.PathValue("serial"))
	if !ok {
		writeError(writer, 404, "unknown terminal")
		return
	}
	var input struct {
		EndpointURL  string `json:"endpointUrl"`
		SigningKeyID string `json:"signingKeyId"`
		Enabled      *bool  `json:"enabled"`
	}
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, 400, err.Error())
		return
	}
	if input.Enabled == nil || !validDeliveryEndpoint(input.EndpointURL) {
		writeError(writer, 422, "endpointUrl must be an absolute HTTPS URL without credentials or fragments; enabled must be a boolean")
		return
	}
	if _, ok = s.config.DeliverySigningKeys[input.SigningKeyID]; !ok {
		writeError(writer, 422, "signingKeyId is not configured on the gateway")
		return
	}
	route := store.DeliveryRoute{TerminalSerialNumber: terminal.SerialNumber, EndpointURL: input.EndpointURL, SigningKeyID: input.SigningKeyID, Enabled: *input.Enabled}
	if err := s.store.SaveDeliveryRoute(request.Context(), route, identity.ID); errors.Is(err, store.ErrDeliveryRouteBusy) {
		writeError(writer, 409, err.Error())
		return
	} else if err != nil {
		s.internalError(writer, "save delivery route", err)
		return
	}
	writeJSON(writer, http.StatusOK, route)
}

func (s *Server) eventDeliveries(writer http.ResponseWriter, request *http.Request) {
	limit, offset, err := pagination(request)
	if err != nil {
		writeError(writer, 400, err.Error())
		return
	}
	rows, total, err := s.store.EventDeliveries(request.Context(), limit, offset)
	if err != nil {
		s.internalError(writer, "list event deliveries", err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"deliveries": rows, "total": total})
}

func (s *Server) retryEventDelivery(writer http.ResponseWriter, request *http.Request) {
	identity, ok := administratorIdentity(request.Context())
	if !ok {
		writeError(writer, 401, "administrator required")
		return
	}
	id, err := strconv.ParseInt(request.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(writer, 400, "invalid delivery id")
		return
	}
	if err = s.store.RetryEventDelivery(request.Context(), id, identity.ID); errors.Is(err, pgx.ErrNoRows) {
		writeError(writer, 409, "only failed deliveries can be retried")
		return
	} else if err != nil {
		s.internalError(writer, "retry event delivery", err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"id": id, "status": "pending"})
}

func (s *Server) eventBackfill(writer http.ResponseWriter, request *http.Request) {
	identity, ok := administratorIdentity(request.Context())
	if !ok {
		writeError(writer, 401, "administrator required")
		return
	}
	terminal, ok := s.config.TerminalBySerialNumber(request.PathValue("serial"))
	if !ok {
		writeError(writer, 404, "unknown terminal")
		return
	}
	var input struct {
		StartsAt     string `json:"startsAt"`
		EndsAt       string `json:"endsAt"`
		PreviewToken string `json:"previewToken"`
	}
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, 400, err.Error())
		return
	}
	from, fromErr := time.Parse(time.RFC3339Nano, input.StartsAt)
	until, untilErr := time.Parse(time.RFC3339Nano, input.EndsAt)
	if strings.HasSuffix(input.StartsAt, "-00:00") || strings.HasSuffix(input.EndsAt, "-00:00") || fromErr != nil || untilErr != nil || !until.After(from) || until.Sub(from) > 31*24*time.Hour || until.After(time.Now()) {
		writeError(writer, 422, "select a past range of at most 31 days using RFC3339 timestamps with offsets; the end is exclusive")
		return
	}
	var err error
	if request.PathValue("action") != "preview" && request.PathValue("action") != "queue" {
		writeError(writer, 404, "unknown backfill action")
		return
	}
	if request.PathValue("action") == "preview" {
		if input.PreviewToken != "" {
			writeError(writer, 422, "previewToken is only used when queuing a preview")
			return
		}
		var preview *store.BackfillPreview
		preview, err = s.store.PreviewEventBackfill(request.Context(), terminal.SerialNumber, from, until)
		if err == nil {
			writeJSON(writer, 200, preview)
			return
		}
	} else {
		if len(input.PreviewToken) != 64 {
			writeError(writer, 422, "previewToken is required; preview the backfill first")
			return
		}
		var id int64
		var count int
		id, count, err = s.store.QueueEventBackfill(request.Context(), terminal.SerialNumber, from, until, input.PreviewToken, identity.ID)
		if err == nil {
			writeJSON(writer, 200, map[string]any{"backfillId": id, "queuedCount": count})
			return
		}
	}
	if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, store.ErrBackfillRouteDisabled) || errors.Is(err, store.ErrBackfillPreviewChanged) || errors.Is(err, store.ErrBackfillEmpty) {
		writeError(writer, 409, err.Error())
		return
	}
	if errors.Is(err, store.ErrBackfillTooLarge) {
		writeError(writer, 422, err.Error())
		return
	}
	s.internalError(writer, "historical event delivery", err)
}

func (s *Server) deleteDeliveryRoute(writer http.ResponseWriter, request *http.Request) {
	identity, ok := administratorIdentity(request.Context())
	if !ok {
		writeError(writer, 401, "administrator required")
		return
	}
	terminal, ok := s.config.TerminalBySerialNumber(request.PathValue("serial"))
	if !ok {
		writeError(writer, 404, "unknown terminal")
		return
	}
	if err := s.store.DeleteDeliveryRoute(request.Context(), terminal.SerialNumber, identity.ID); errors.Is(err, pgx.ErrNoRows) {
		writeError(writer, 404, "destination not configured")
		return
	} else if errors.Is(err, store.ErrDeliveryRouteUnfinished) {
		writeError(writer, 409, err.Error())
		return
	} else if err != nil {
		s.internalError(writer, "remove delivery destination", err)
		return
	}
	writeJSON(writer, 200, map[string]any{"terminalSerialNumber": terminal.SerialNumber, "removed": true})
}
