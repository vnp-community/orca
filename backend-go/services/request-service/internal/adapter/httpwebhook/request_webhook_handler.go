// Package httpwebhook serves POST /v1/request-webhooks/{source_name}: external systems create
// Requests with an HMAC-signed body and a tenant header, outside the user-JWT gateway path.
package httpwebhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

const (
	MaxBodyBytes    = 256 << 10
	HeaderTenant    = "X-Orca-Tenant-Id"
	HeaderSignature = "X-Orca-Signature"
	// HeaderTimestamp carries Unix seconds; the signed text is "<timestamp>.<raw body>".
	HeaderTimestamp = "X-Orca-Timestamp"
	// MaxClockSkew bounds how old (or new) a signed timestamp may be.
	MaxClockSkew = 5 * time.Minute
	// every authentication failure answers with this one body so a caller cannot tell which check failed.
	invalidSignatureBody = `{"error":"invalid signature"}`
)

// Source is what a (tenant, source name) pair is configured with.
type Source struct {
	Secret     []byte
	ReporterID string
}

type SourceResolver interface {
	Resolve(tenantID, sourceName string) (Source, bool)
}

type RequestCreator interface {
	Execute(ctx context.Context, in usecase.CreateRequestInput) (usecase.CreateRequestResult, error)
}

// ReplayGuard remembers accepted signatures (usecase.WebhookReplayGuard).
type ReplayGuard interface {
	FirstSeen(ctx context.Context, tenantID, source, signatureHeader string) (bool, error)
}

type Handler struct {
	sources SourceResolver
	creator RequestCreator
	replay  ReplayGuard
	// requireTimestamp refuses calls without a fresh X-Orca-Timestamp; false is the migration window for
	// senders that still sign only the body (REQUEST_WEBHOOK_REQUIRE_TIMESTAMP=false).
	requireTimestamp bool
	now              func() time.Time
}

func NewHandler(sources SourceResolver, creator RequestCreator) *Handler {
	return &Handler{sources: sources, creator: creator, now: time.Now}
}

// WithReplayProtection turns on the timestamp check and the nonce store. Production wiring always sets it.
func (h *Handler) WithReplayProtection(g ReplayGuard, requireTimestamp bool) *Handler {
	h.replay, h.requireTimestamp = g, requireTimestamp
	return h
}

// WithClock replaces the clock (tests).
func (h *Handler) WithClock(now func() time.Time) *Handler {
	h.now = now
	return h
}

// Mount registers the route on mux.
func (h *Handler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/request-webhooks/{source_name}", h.serve)
}

type payload struct {
	ProjectID string `json:"project_id"`
	Ref       string `json:"ref"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	URL       string `json:"url"`
	Hints     struct {
		IssueType string   `json:"issue_type"`
		Labels    []string `json:"labels"`
		Priority  string   `json:"priority"`
	} `json:"hints"`
}

func (h *Handler) serve(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeJSON(w, http.StatusRequestEntityTooLarge, `{"error":"payload too large"}`)
			return
		}
		writeJSON(w, http.StatusBadRequest, `{"error":"unreadable body"}`)
		return
	}
	sourceName := r.PathValue("source_name")
	tenantID := strings.TrimSpace(r.Header.Get(HeaderTenant))
	src, ok := h.authenticate(tenantID, sourceName, r.Header.Get(HeaderSignature), r.Header.Get(HeaderTimestamp), raw)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, invalidSignatureBody)
		return
	}
	if h.replay != nil {
		// Only a correctly signed call reaches the nonce store, so unauthenticated traffic cannot fill it.
		first, err := h.replay.FirstSeen(tenant.WithTenantID(r.Context(), tenantID), tenantID, sourceName, r.Header.Get(HeaderSignature))
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, `{"error":"replay check unavailable"}`)
			return
		}
		if !first {
			writeJSON(w, http.StatusOK, `{"created":false,"duplicate":true}`)
			return
		}
	}

	var p payload
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		writeJSON(w, http.StatusBadRequest, `{"error":"invalid payload"}`)
		return
	}
	ctx := tenant.WithUserID(tenant.WithTenantID(r.Context(), tenantID), src.ReporterID)
	res, err := h.creator.Execute(ctx, usecase.CreateRequestInput{
		ProjectID: p.ProjectID, Title: p.Title, Body: p.Body,
		Source:    domain.SourceRef{Provider: domain.SourceProviderWebhook, Site: sourceName, Ref: p.Ref, URL: p.URL},
		Hints:     domain.SourceHints{IssueType: p.Hints.IssueType, Labels: p.Hints.Labels, Priority: p.Hints.Priority},
		ActorKind: domain.ActorKindSystem,
	})
	if err != nil {
		writeAppError(w, err)
		return
	}
	b, _ := json.Marshal(map[string]any{"request_id": res.Request.ID, "created": res.Created})
	writeJSON(w, http.StatusOK, string(b))
}

// authenticate always runs one HMAC so an unknown source costs the same as a wrong signature or timestamp.
func (h *Handler) authenticate(tenantID, sourceName, header, timestamp string, body []byte) (Source, bool) {
	var src Source
	found := false
	if _, err := uuid.Parse(tenantID); err == nil {
		src, found = h.sources.Resolve(tenantID, sourceName)
	}
	secret := src.Secret
	if !found || len(secret) == 0 {
		secret = []byte("unconfigured")
		found = false
	}
	signed, fresh := body, true
	switch {
	case timestamp != "":
		signed, fresh = signedWithTimestamp(timestamp, body), h.fresh(timestamp)
	case h.requireTimestamp:
		fresh = false
	}
	sig, sigOK := parseSignature(header)
	mac := hmac.New(sha256.New, secret)
	mac.Write(signed)
	return src, found && sigOK && fresh && hmac.Equal(mac.Sum(nil), sig)
}

func signedWithTimestamp(timestamp string, body []byte) []byte {
	out := make([]byte, 0, len(timestamp)+1+len(body))
	out = append(out, timestamp...)
	out = append(out, '.')
	return append(out, body...)
}

// fresh accepts Unix seconds within MaxClockSkew of now, in either direction.
func (h *Handler) fresh(timestamp string) bool {
	secs, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	d := h.now().Sub(time.Unix(secs, 0))
	if d < 0 {
		d = -d
	}
	return d <= MaxClockSkew
}

func parseSignature(h string) ([]byte, bool) {
	hexPart, ok := strings.CutPrefix(strings.TrimSpace(h), "sha256=")
	if !ok {
		return nil, false
	}
	b, err := hex.DecodeString(hexPart)
	return b, err == nil && len(b) == sha256.Size
}

func writeAppError(w http.ResponseWriter, err error) {
	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		writeJSON(w, http.StatusInternalServerError, `{"error":"internal error"}`)
		return
	}
	code := http.StatusInternalServerError
	switch ae.Kind {
	case apperrors.KindInvalidArgument:
		code = http.StatusBadRequest
	case apperrors.KindNotFound:
		code = http.StatusNotFound
	case apperrors.KindFailedPrecondition, apperrors.KindAlreadyExists:
		code = http.StatusConflict
	}
	b, _ := json.Marshal(map[string]string{"error": ae.Code})
	writeJSON(w, code, string(b))
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}
