package authclient

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/internalcaller"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

const (
	DefaultMcpPrincipalTTL         = 30 * time.Second
	defaultMcpPrincipalNegativeTTL = 5 * time.Second
	mcpResolveTimeout              = 3 * time.Second
)

// McpPrincipalResolver implements usecase.McpPrincipalResolver over
// auth-service ResolveMcpPrincipal. Active answers are cached for ttl (30s),
// which is the worst-case delay before a revocation, demotion or deactivation
// is seen; inactive answers for 5s. Errors are never cached and never retried
// automatically (the RPC has a write side effect and fails closed upstream).
type McpPrincipalResolver struct {
	client authv1.AuthServiceClient
	ttl    time.Duration
	negTTL time.Duration
	now    func() time.Time

	// OnResolve, when set, is told how each Resolve ended (cache_hit, active,
	// inactive, error) for orca_mcp_principal_resolve_total. Set before use.
	OnResolve func(result string)

	// InternalToken, when set, is presented as the internal-caller secret that
	// auth-service requires on ResolveMcpPrincipal. Empty = rollout-safe no-op.
	InternalToken string

	mu    sync.Mutex
	cache map[string]mcpCacheEntry
	group singleflight.Group
}

type mcpCacheEntry struct {
	res     usecase.McpResolveResult
	userID  string
	tenant  string
	expires time.Time
}

func NewMcpPrincipalResolver(client authv1.AuthServiceClient, ttl time.Duration) *McpPrincipalResolver {
	if ttl <= 0 {
		ttl = DefaultMcpPrincipalTTL
	}
	neg := defaultMcpPrincipalNegativeTTL
	if neg > ttl {
		neg = ttl
	}
	return &McpPrincipalResolver{client: client, ttl: ttl, negTTL: neg, now: time.Now, cache: map[string]mcpCacheEntry{}}
}

func cacheKey(in usecase.McpResolveInput) string {
	return in.TokenUse + "|" + in.TenantID + "|" + in.UserID + "|" + in.JTI
}

func (c *McpPrincipalResolver) Resolve(ctx context.Context, in usecase.McpResolveInput) (usecase.McpResolveResult, error) {
	key := cacheKey(in)
	if res, ok := c.lookup(key); ok {
		c.observe("cache_hit")
		return res, nil
	}
	v, err, _ := c.group.Do(key, func() (any, error) {
		// Detach from the first caller's cancellation so one aborted request
		// does not fail every request waiting on the same lookup.
		rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), mcpResolveTimeout)
		defer cancel()
		if c.InternalToken != "" {
			rctx = metadata.AppendToOutgoingContext(rctx, internalcaller.MetadataKey, c.InternalToken)
		}
		resp, err := c.client.ResolveMcpPrincipal(rctx, &authv1.ResolveMcpPrincipalRequest{
			Jti: in.JTI, UserId: in.UserID, TenantId: in.TenantID, TokenUse: in.TokenUse,
			FamilyId: in.FamilyID, GrantId: in.GrantID, ClientId: in.ClientID,
		})
		if err != nil {
			return nil, fmt.Errorf("authclient: resolving mcp principal: %w", err)
		}
		res := usecase.McpResolveResult{Active: resp.GetActive(), InactiveReason: resp.GetInactiveReason(), Role: resp.GetRole()}
		c.store(key, in, res)
		if res.Active {
			c.observe("active")
		} else {
			c.observe("inactive")
		}
		return res, nil
	})
	if err != nil {
		c.observe("error")
		return usecase.McpResolveResult{}, err
	}
	return v.(usecase.McpResolveResult), nil
}

func (c *McpPrincipalResolver) observe(result string) {
	if c.OnResolve != nil {
		c.OnResolve(result)
	}
}

func (c *McpPrincipalResolver) lookup(key string) (usecase.McpResolveResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.cache[key]
	if !ok {
		return usecase.McpResolveResult{}, false
	}
	if !c.now().Before(e.expires) {
		delete(c.cache, key)
		return usecase.McpResolveResult{}, false
	}
	return e.res, true
}

func (c *McpPrincipalResolver) store(key string, in usecase.McpResolveInput, res usecase.McpResolveResult) {
	ttl := c.negTTL
	if res.Active {
		ttl = c.ttl
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if len(c.cache) > 10000 { // bound memory: drop expired entries, then everything
		for k, e := range c.cache {
			if !now.Before(e.expires) {
				delete(c.cache, k)
			}
		}
		if len(c.cache) > 10000 {
			c.cache = map[string]mcpCacheEntry{}
		}
	}
	c.cache[key] = mcpCacheEntry{res: res, userID: in.UserID, tenant: in.TenantID, expires: now.Add(ttl)}
}

// Invalidate drops every cached answer for a token id (e.g. right after the
// same replica revoked it); other replicas converge within the TTL.
func (c *McpPrincipalResolver) Invalidate(jti string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.cache {
		if len(k) >= len(jti) && k[len(k)-len(jti):] == jti {
			delete(c.cache, k)
		}
	}
}

var _ usecase.McpPrincipalResolver = (*McpPrincipalResolver)(nil)
