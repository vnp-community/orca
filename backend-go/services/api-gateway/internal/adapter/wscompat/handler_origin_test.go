package wscompat

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/originpolicy"
)

func TestServeHTTPRejectsDisallowedOriginBeforeUpgrade(t *testing.T) {
	policy, err := originpolicy.Parse("https://orca.example.com")
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := New(logger, fakeSessionValidator{identity: Identity{TenantID: "t", UserID: "u"}}, nil, NewRegistry()).WithOriginPolicy(policy)

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	// 403 (not 101/400 from the upgrader) proves the check ran before Accept.
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403", rec.Code)
	}
}
