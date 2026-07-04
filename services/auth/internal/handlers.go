package auth

import (
	"encoding/json"
	"errors"
	"net/http"
)

// Handlers is the HTTP adapter over the Service. It does only transport concerns —
// decode, call the use case, map errors to status codes, encode. No business logic
// leaks in here (clean architecture: delivery layer is thin).
type Handlers struct{ svc *Service }

func NewHandlers(svc *Service) *Handlers { return &Handlers{svc: svc} }

// Routes registers the auth endpoints on a mux (Go 1.22 method+path patterns).
func (h *Handlers) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/auth/register", h.register)
	mux.HandleFunc("POST /v1/auth/login", h.login)
	mux.HandleFunc("POST /v1/auth/refresh", h.refresh)
}

type registerReq struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}
type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	DeviceID string `json:"device_id"`
}
type refreshReq struct {
	RefreshToken string `json:"refresh_token"`
}
type tokenResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

func (h *Handlers) register(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if !decode(w, r, &req) {
		return
	}
	pair, err := h.svc.Register(r.Context(), req.Email, req.Password, req.DisplayName)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toResp(pair))
}

func (h *Handlers) login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if !decode(w, r, &req) {
		return
	}
	pair, err := h.svc.Login(r.Context(), req.Email, req.Password, req.DeviceID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toResp(pair))
}

func (h *Handlers) refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshReq
	if !decode(w, r, &req) {
		return
	}
	pair, err := h.svc.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toResp(pair))
}

func toResp(p TokenPair) tokenResp {
	return tokenResp{AccessToken: p.AccessToken, RefreshToken: p.RefreshToken, ExpiresIn: p.ExpiresIn}
}

// --- transport helpers -----------------------------------------------------

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16) // bound request size (abuse guard)
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

// writeErr maps domain errors to HTTP status codes in one place.
func writeErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrEmailTaken):
		writeJSON(w, http.StatusConflict, map[string]string{"code": "email_taken"})
	case errors.Is(err, ErrBadCreds):
		writeJSON(w, http.StatusUnauthorized, map[string]string{"code": "invalid_credentials"})
	case errors.Is(err, ErrRefreshBad):
		writeJSON(w, http.StatusUnauthorized, map[string]string{"code": "refresh_invalid"})
	case errors.Is(err, ErrPasswordShort):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"code": "password_too_short"})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"code": "internal"})
	}
}
