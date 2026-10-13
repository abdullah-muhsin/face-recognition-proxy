package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/itplus/pushsdk-gateway/internal/store"
	"github.com/jackc/pgx/v5"
)

func (s *Server) attachmentDeliveries(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := pagination(r)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	rows, total, err := s.store.AttachmentDeliveries(r.Context(), limit, offset)
	if err != nil {
		s.internalError(w, "list attachment deliveries", err)
		return
	}
	writeJSON(w, 200, map[string]any{"deliveries": rows, "total": total})
}

func (s *Server) retryAttachment(w http.ResponseWriter, r *http.Request) {
	actor, ok := administratorIdentity(r.Context())
	if !ok {
		writeError(w, 401, "administrator required")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, 400, "invalid attachment delivery id")
		return
	}
	err = s.store.RetryAttachmentDelivery(r.Context(), id, actor.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 409, "only failed attachment deliveries can be retried")
		return
	}
	if err != nil {
		s.internalError(w, "retry attachment delivery", err)
		return
	}
	writeJSON(w, 200, map[string]any{"id": id, "status": "pending"})
}

func (s *Server) attachmentBackfill(w http.ResponseWriter, r *http.Request) {
	actor, ok := administratorIdentity(r.Context())
	if !ok {
		writeError(w, 401, "administrator required")
		return
	}
	terminal, ok := s.config.TerminalBySerialNumber(r.PathValue("serial"))
	if !ok {
		writeError(w, 404, "unknown terminal")
		return
	}
	var input struct {
		ThroughDeliveryID int64  `json:"throughDeliveryId"`
		PreviewToken      string `json:"previewToken"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	if input.ThroughDeliveryID < 0 {
		writeError(w, 422, "invalid delivery bound")
		return
	}
	var err error
	switch r.PathValue("action") {
	case "preview":
		if input.PreviewToken != "" {
			writeError(w, 422, "previewToken is only used for queuing")
			return
		}
		var p *store.AttachmentBackfillPreview
		p, err = s.store.PreviewAttachmentBackfill(r.Context(), terminal.SerialNumber, input.ThroughDeliveryID)
		if err == nil {
			writeJSON(w, 200, p)
			return
		}
	case "queue":
		if len(input.PreviewToken) != 64 || input.ThroughDeliveryID <= 0 {
			writeError(w, 422, "previewToken and throughDeliveryId are required")
			return
		}
		var count int
		count, err = s.store.QueueAttachmentBackfill(r.Context(), terminal.SerialNumber, input.ThroughDeliveryID, input.PreviewToken, actor.ID)
		if err == nil {
			writeJSON(w, 200, map[string]any{"queuedCount": count})
			return
		}
	default:
		writeError(w, 404, "unknown backfill action")
		return
	}
	if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, store.ErrBackfillRouteDisabled) || errors.Is(err, store.ErrBackfillPreviewChanged) || errors.Is(err, store.ErrBackfillEmpty) {
		writeError(w, 409, err.Error())
		return
	}
	if errors.Is(err, store.ErrBackfillTooLarge) {
		writeError(w, 422, "At most 5000 event receipts per photo backfill. Select a smaller throughDeliveryId.")
		return
	}
	s.internalError(w, "attachment backfill", err)
}
