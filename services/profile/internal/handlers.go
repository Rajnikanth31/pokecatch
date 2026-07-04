package profile

import (
	"encoding/json"
	"errors"
	"net/http"
)

// Handlers is the HTTP adapter for the profile service. The gateway authenticates
// the caller and sets X-Account-Id; these handlers trust that header (it is
// stripped from inbound client requests and only ever set by the gateway) and
// enforce ownership in the use-case layer.
type Handlers struct {
	col    *Collection
	trades *TradeManager
}

func NewHandlers(col *Collection, trades *TradeManager) *Handlers {
	return &Handlers{col: col, trades: trades}
}

func (h *Handlers) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/creatures", h.listCreatures)
	mux.HandleFunc("POST /v1/creatures/{id}/evolve", h.evolve)
	mux.HandleFunc("PUT /v1/team", h.setTeam)
	mux.HandleFunc("POST /v1/trades", h.openTrade)
	mux.HandleFunc("POST /v1/trades/{id}/stage", h.stageTrade)
	mux.HandleFunc("POST /v1/trades/{id}/lock", h.lockTrade)
	mux.HandleFunc("POST /v1/trades/{id}/commit", h.commitTrade)
}

func (h *Handlers) listCreatures(w http.ResponseWriter, r *http.Request) {
	acct := caller(r)
	cursor := r.URL.Query().Get("cursor")
	list, next, err := h.col.List(r.Context(), acct, cursor, 50)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"creatures": list, "next_cursor": next})
}

func (h *Handlers) evolve(w http.ResponseWriter, r *http.Request) {
	oc, err := h.col.Evolve(r.Context(), caller(r), r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, oc)
}

func (h *Handlers) setTeam(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Creatures []string `json:"creatures"`
	}
	if !decode(w, r, &req) {
		return
	}
	if err := h.col.SetTeam(r.Context(), caller(r), req.Creatures); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"status": "saved"})
}

func (h *Handlers) openTrade(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PartnerID string `json:"partner_id"`
	}
	if !decode(w, r, &req) {
		return
	}
	t := h.trades.Open(caller(r), req.PartnerID)
	writeJSON(w, 201, map[string]string{"trade_id": t.ID})
}

func (h *Handlers) stageTrade(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Creatures []string `json:"creatures"`
	}
	if !decode(w, r, &req) {
		return
	}
	if err := h.trades.Stage(r.Context(), r.PathValue("id"), caller(r), req.Creatures); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"status": "staged"})
}

func (h *Handlers) lockTrade(w http.ResponseWriter, r *http.Request) {
	if err := h.trades.Lock(r.PathValue("id"), caller(r)); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"status": "locked"})
}

func (h *Handlers) commitTrade(w http.ResponseWriter, r *http.Request) {
	if err := h.trades.Commit(r.Context(), r.PathValue("id")); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"status": "committed"})
}

// --- helpers ---------------------------------------------------------------

func caller(r *http.Request) string { return r.Header.Get("X-Account-Id") }

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "bad_request"})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeErr maps profile domain errors to status codes in one place.
func writeErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotOwner):
		writeJSON(w, http.StatusForbidden, map[string]string{"code": "not_owner"})
	case errors.Is(err, ErrTeamTooBig), errors.Is(err, ErrTeamDup), errors.Is(err, ErrEmptyTeam):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"code": "invalid_team"})
	case errors.Is(err, ErrCannotEvolve), errors.Is(err, ErrLevelTooLow):
		writeJSON(w, http.StatusConflict, map[string]string{"code": "cannot_evolve"})
	case errors.Is(err, ErrTradeNotFound), errors.Is(err, ErrNotParticipant),
		errors.Is(err, ErrTradeClosed), errors.Is(err, ErrNotLocked):
		writeJSON(w, http.StatusConflict, map[string]string{"code": "trade_error"})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"code": "internal"})
	}
}
