package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strconv"
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
		EnabledFrom  string `json:"enabledFrom"`
		Enabled      *bool  `json:"enabled"`
	}
	if err := decodeJSON(request, &input); err != nil {
		writeError(writer, 400, err.Error())
		return
	}
	enabledFrom, err := time.Parse(time.RFC3339Nano, input.EnabledFrom)
	if err != nil || input.Enabled == nil || !validDeliveryEndpoint(input.EndpointURL) {
		writeError(writer, 422, "endpointUrl must be an absolute HTTPS URL without credentials or fragments; enabledFrom must be RFC3339 and enabled must be a boolean")
		return
	}
	if _, ok = s.config.DeliverySigningKeys[input.SigningKeyID]; !ok {
		writeError(writer, 422, "signingKeyId is not configured on the gateway")
		return
	}
	route := store.DeliveryRoute{TerminalSerialNumber: terminal.SerialNumber, EndpointURL: input.EndpointURL, SigningKeyID: input.SigningKeyID, EnabledFrom: enabledFrom, Enabled: *input.Enabled}
	if err = s.store.SaveDeliveryRoute(request.Context(), route, identity.ID); errors.Is(err, store.ErrDeliveryRouteBusy) {
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
