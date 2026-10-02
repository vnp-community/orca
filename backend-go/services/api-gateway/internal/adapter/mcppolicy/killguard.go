package mcppolicy

import (
	"context"
	"net/http"
	"sync"
	"time"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

// ErrKillSwitchActive is returned by KillGuardVerifier when an administrator
// stopped this tenant/client/grant/session.
var ErrKillSwitchActive = mcpserver.ErrKillSwitchActive

const (
	killGuardTTL = 5 * time.Second
	// staleKillOK bounds how long a cached answer may stand in for an
	// unreachable mcp-service, keeping the worst-case lag well under 60s.
	staleKillOK = 30 * time.Second
)

type killKey struct{ tenant, client, grant, session string }

type killEntry struct {
	blocked bool
	reason  string
	at      time.Time
}

// KillGuard answers "is this principal stopped?" from mcp-service, cached for
// 5s so /mcp does not add an RPC per request.
type KillGuard struct {
	c       mcpv1.McpServiceClient
	timeout time.Duration
	now     func() time.Time

	mu    sync.Mutex
	cache map[killKey]killEntry
}

func NewKillGuard(c mcpv1.McpServiceClient, timeout time.Duration) *KillGuard {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &KillGuard{c: c, timeout: timeout, now: time.Now, cache: map[killKey]killEntry{}}
}

// Blocked reports whether the principal is stopped. On an RPC failure it
// serves a recent cached answer, else returns the error (callers fail closed).
func (k *KillGuard) Blocked(ctx context.Context, p mcpserver.Principal, sessionID string) (bool, string, error) {
	key := killKey{p.TenantID, p.ClientID, p.GrantID, sessionID}
	k.mu.Lock()
	e, ok := k.cache[key]
	k.mu.Unlock()
	if ok && k.now().Sub(e.at) < killGuardTTL {
		return e.blocked, e.reason, nil
	}
	rctx := gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: p.TenantID, UserID: p.UserID, Role: p.Role})
	rctx, cancel := context.WithTimeout(rctx, k.timeout)
	defer cancel()
	resp, err := k.c.GetKillState(rctx, &mcpv1.GetKillStateRequest{ClientId: p.ClientID, GrantId: p.GrantID, SessionId: sessionID})
	if err != nil {
		if ok && k.now().Sub(e.at) < staleKillOK {
			return e.blocked, e.reason, nil
		}
		return false, "", err
	}
	k.mu.Lock()
	if len(k.cache) > 4096 {
		k.cache = map[killKey]killEntry{}
	}
	k.cache[key] = killEntry{blocked: resp.GetActive(), reason: resp.GetReason(), at: k.now()}
	k.mu.Unlock()
	return resp.GetActive(), resp.GetReason(), nil
}

// KillGuardVerifier decorates a TokenVerifier: a principal that is kill-
// switched is refused before any handler runs (the SDK middleware maps an
// unknown verifier error to 401 invalid_token, which is fail-closed), and an
// unreachable kill state is a 503 rather than a silent pass.
type KillGuardVerifier struct {
	Inner mcpserver.TokenVerifier
	Guard *KillGuard
}

func (v KillGuardVerifier) Verify(ctx context.Context, r *http.Request) (mcpserver.Principal, error) {
	p, err := v.Inner.Verify(ctx, r)
	if err != nil {
		return p, err
	}
	// Session-scope kill switches are keyed by the non-secret session row id and
	// checked by the session host after it resolves the session; the
	// Mcp-Session-Id header is a bearer secret and must never be a lookup key.
	blocked, _, gerr := v.Guard.Blocked(ctx, p, "")
	switch {
	case gerr != nil:
		return mcpserver.Principal{}, mcpserver.ErrVerifierUnavailable
	case blocked:
		return mcpserver.Principal{}, ErrKillSwitchActive
	}
	return p, nil
}

// WatchKill returns a context that is cancelled shortly after the principal is
// kill-switched, so a long-running tool can be stopped. The executor opts in
// by wrapping its dispatch context with it.
func (g *Gate) WatchKill(ctx context.Context, guard *KillGuard, p mcpserver.Principal, every time.Duration) (context.Context, context.CancelFunc) {
	if every <= 0 {
		every = killGuardTTL
	}
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if blocked, _, err := guard.Blocked(ctx, p, SessionIDFromContext(ctx)); err == nil && blocked {
					cancel()
					return
				}
			}
		}
	}()
	return ctx, cancel
}

var _ mcpserver.TokenVerifier = KillGuardVerifier{}
