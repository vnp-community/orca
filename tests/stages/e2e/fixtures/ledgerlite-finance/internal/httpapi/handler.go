// Package httpapi is SVC-001: the JSON HTTP surface of LedgerLite (API-001..API-006).
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/ledgerlite/ledgerlite/internal/ledger"
)

const maxBodyBytes = 1 << 20

// Handler serves the ledger over HTTP.
type Handler struct {
	svc *ledger.Service
	log *slog.Logger
}

// New returns the root http.Handler, including request logging (NFR-005).
// Routing uses only net/http patterns and log/slog (ADR-006, CT-001).
func New(svc *ledger.Service, log *slog.Logger) http.Handler {
	h := &Handler{svc: svc, log: log}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /accounts", h.createAccount)                             // API-001
	mux.HandleFunc("GET /accounts/{id}", h.getAccount)                            // API-002
	mux.HandleFunc("POST /transfers", h.postTransfer)                             // API-003
	mux.HandleFunc("GET /transfers/{id}", h.getTransfer)                          // API-004
	mux.HandleFunc("GET /journal", h.getJournal)                                  // API-005
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { // API-006, FR-007
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	return h.logRequests(mux)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (h *Handler) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		h.log.Info("request", "method", r.Method, "path", r.URL.Path, "status", rec.status, "duration_ms", time.Since(start).Milliseconds())
	})
}

type createAccountBody struct {
	Owner          string `json:"owner"`
	AllowOverdraft bool   `json:"allow_overdraft"`
	DailyLimit     int64  `json:"daily_limit"`
	InitialDeposit int64  `json:"initial_deposit"`
}

func (h *Handler) createAccount(w http.ResponseWriter, r *http.Request) {
	var b createAccountBody
	if !decode(w, r, &b) {
		return
	}
	acc, err := h.svc.CreateAccount(ledger.NewAccount{Owner: b.Owner, AllowOverdraft: b.AllowOverdraft, DailyLimit: b.DailyLimit, InitialDeposit: b.InitialDeposit})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, acc)
}

func (h *Handler) getAccount(w http.ResponseWriter, r *http.Request) {
	acc, err := h.svc.Account(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, acc)
}

type transferBody struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Amount int64  `json:"amount"`
}

func (h *Handler) postTransfer(w http.ResponseWriter, r *http.Request) {
	var b transferBody
	if !decode(w, r, &b) {
		return
	}
	t, replayed, err := h.svc.Transfer(ledger.TransferRequest{IdempotencyKey: r.Header.Get("Idempotency-Key"), From: b.From, To: b.To, Amount: b.Amount})
	if err != nil {
		writeError(w, err)
		return
	}
	if replayed {
		w.Header().Set("Idempotent-Replayed", "true")
		writeJSON(w, http.StatusOK, t)
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (h *Handler) getTransfer(w http.ResponseWriter, r *http.Request) {
	t, err := h.svc.GetTransfer(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *Handler) getJournal(w http.ResponseWriter, r *http.Request) {
	entries := h.svc.Journal(r.URL.Query().Get("account"))
	if entries == nil {
		entries = []ledger.Entry{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

// errorTable maps domain errors to HTTP status and a stable code (NFR-006).
var errorTable = []struct {
	err    error
	status int
	code   string
}{
	{ledger.ErrInvalidAmount, http.StatusBadRequest, "invalid_amount"},
	{ledger.ErrSameAccount, http.StatusBadRequest, "same_account"},
	{ledger.ErrInvalidAccount, http.StatusBadRequest, "invalid_account"},
	{ledger.ErrIdempotencyKeyRequired, http.StatusBadRequest, "idempotency_key_required"},
	{ledger.ErrAccountNotFound, http.StatusNotFound, "account_not_found"},
	{ledger.ErrTransferNotFound, http.StatusNotFound, "transfer_not_found"},
	{ledger.ErrIdempotencyConflict, http.StatusConflict, "idempotency_conflict"},
	{ledger.ErrAccountExists, http.StatusConflict, "account_exists"},
	{ledger.ErrInsufficientFunds, http.StatusUnprocessableEntity, "insufficient_funds"},
	{ledger.ErrDailyLimitExceeded, http.StatusUnprocessableEntity, "daily_limit_exceeded"},
}

func writeError(w http.ResponseWriter, err error) {
	status, code := http.StatusInternalServerError, "internal_error"
	for _, row := range errorTable {
		if errors.Is(err, row.err) {
			status, code = row.status, row.code
			break
		}
	}
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": err.Error()}})
}

// decode rejects unknown fields and non-integer amounts, so money never goes through floats (NFR-004).
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid_json", "message": err.Error()}})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
