// Package eventhub fans mcp-service domain events out to the StreamEvents
// subscribers connected to THIS replica. Every replica consumes the MCP
// stream through its own JetStream ephemeral consumer, so a subscriber sees
// events regardless of which replica wrote them (no core-NATS needed).
package eventhub

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/services/mcp-service/internal/adapter/wire"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

const (
	// MaxStreamsPerUser bounds subscriptions per user per replica.
	MaxStreamsPerUser = 5
	subscriberBuffer  = 64
	// replayGrace: an ephemeral consumer starts at the beginning of the stream;
	// events older than the hub start are history, not news.
	replayGrace = 10 * time.Second
)

// ApprovalReader loads an approval by id (the event carries ids only, so
// previews and hashes never travel through JetStream).
type ApprovalReader interface {
	GetApproval(ctx context.Context, tenantID, id string) (domain.Approval, error)
}

type sub struct {
	tenantID, userID string
	ch               chan *mcpv1.McpEvent
}

type Hub struct {
	approvals  ApprovalReader
	invalidate func(tenantID string)
	now        func() time.Time
	startedAt  time.Time

	mu   sync.Mutex
	subs map[*sub]struct{}
}

// New: invalidate is called for policy/kill-switch events so caches refresh
// before their TTL. It may be nil.
func New(approvals ApprovalReader, invalidate func(tenantID string)) *Hub {
	h := &Hub{approvals: approvals, invalidate: invalidate, now: func() time.Time { return time.Now().UTC() }, subs: map[*sub]struct{}{}}
	h.startedAt = h.now()
	return h
}

var ErrTooManyStreams = apperrors.New(apperrors.KindFailedPrecondition, "MCP_STREAM_LIMIT", "too many open event streams", nil)

// Subscribe registers a subscriber; the returned channel is closed when the
// buffer overflows (the client resyncs via approval.list) or cancel is called.
func (h *Hub) Subscribe(tenantID, userID string) (<-chan *mcpv1.McpEvent, func(), error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for s := range h.subs {
		if s.tenantID == tenantID && s.userID == userID {
			n++
		}
	}
	if n >= MaxStreamsPerUser {
		return nil, nil, ErrTooManyStreams
	}
	s := &sub{tenantID: tenantID, userID: userID, ch: make(chan *mcpv1.McpEvent, subscriberBuffer)}
	h.subs[s] = struct{}{}
	return s.ch, func() { h.remove(s) }, nil
}

func (h *Hub) remove(s *sub) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.subs[s]; ok {
		delete(h.subs, s)
		close(s.ch)
	}
}

// publish delivers to matching subscribers; userID=="" means every user of the tenant.
func (h *Hub) publish(tenantID, userID string, ev *mcpv1.McpEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.subs {
		if s.tenantID != tenantID || (userID != "" && s.userID != userID) {
			continue
		}
		select {
		case s.ch <- ev:
		default: // overflow: close the stream, the client resyncs from the source of truth
			delete(h.subs, s)
			close(s.ch)
		}
	}
}

func (h *Hub) stale(ev eventbus.Event) bool {
	return ev.OccurredAt.Before(h.startedAt.Add(-replayGrace))
}

func decode(ev eventbus.Event) map[string]any {
	var m map[string]any
	if json.Unmarshal(ev.Payload, &m) != nil {
		return nil
	}
	return m
}

func str(m map[string]any, k string) string { s, _ := m[k].(string); return s }

// HandleApprovalRequested: owner only; the full approval is read from the DB.
func (h *Hub) HandleApprovalRequested(ctx context.Context, ev eventbus.Event) error {
	if h.stale(ev) {
		return nil
	}
	m := decode(ev)
	if m == nil {
		return nil // poison payload: ack and drop
	}
	a, err := h.approvals.GetApproval(ctx, ev.TenantID, str(m, "approval_id"))
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return err
		}
		return nil // missing/foreign approval: nothing to show
	}
	h.publish(ev.TenantID, a.UserID, &mcpv1.McpEvent{Type: "approval.requested", Approval: wire.Approval(a, h.now())})
	return nil
}

func (h *Hub) HandleApprovalResolved(_ context.Context, ev eventbus.Event) error {
	if h.stale(ev) {
		return nil
	}
	m := decode(ev)
	if m == nil || str(m, "user_id") == "" {
		return nil
	}
	h.publish(ev.TenantID, str(m, "user_id"), &mcpv1.McpEvent{Type: "approval.resolved", Id: str(m, "approval_id"), Status: str(m, "status")})
	return nil
}

func (h *Hub) HandleKillSwitchChanged(_ context.Context, ev eventbus.Event) error {
	if h.invalidate != nil {
		h.invalidate(ev.TenantID)
	}
	if h.stale(ev) {
		return nil
	}
	m := decode(ev)
	if m == nil || str(m, "scope") != domain.KillScopeTenant {
		return nil
	}
	active, _ := m["active"].(bool)
	h.publish(ev.TenantID, "", &mcpv1.McpEvent{Type: "killswitch.changed", Active: active, Reason: str(m, "reason")})
	return nil
}

func (h *Hub) HandlePolicyChanged(_ context.Context, ev eventbus.Event) error {
	if h.invalidate != nil {
		h.invalidate(ev.TenantID)
	}
	return nil
}

func (h *Hub) HandleGrantRevoked(_ context.Context, ev eventbus.Event) error {
	if h.stale(ev) {
		return nil
	}
	m := decode(ev)
	if m == nil || str(m, "user_id") == "" {
		return nil
	}
	h.publish(ev.TenantID, str(m, "user_id"), &mcpv1.McpEvent{Type: "grant.revoked", GrantId: str(m, "grant_id")})
	return nil
}

// HandleSessionClosed tells the owner's UI streams that a session ended
// (CONTRACT McpEvent session.closed; session_id is the row id).
func (h *Hub) HandleSessionClosed(_ context.Context, ev eventbus.Event) error {
	if h.stale(ev) {
		return nil
	}
	m := decode(ev)
	if m == nil || str(m, "user_id") == "" || str(m, "session_id") == "" {
		return nil
	}
	h.publish(ev.TenantID, str(m, "user_id"), &mcpv1.McpEvent{Type: "session.closed", SessionId: str(m, "session_id")})
	return nil
}

// Run attaches one ephemeral consumer per subject and blocks until ctx ends.
func (h *Hub) Run(ctx context.Context, c *eventbus.Consumer, onErr func(subject string, err error)) {
	bind := map[string]eventbus.Handler{
		domain.SubjectApprovalRequested: h.HandleApprovalRequested,
		domain.SubjectApprovalResolved:  h.HandleApprovalResolved,
		domain.SubjectKillSwitchChanged: h.HandleKillSwitchChanged,
		domain.SubjectPolicyChanged:     h.HandlePolicyChanged,
		domain.SubjectGrantRevoked:      h.HandleGrantRevoked,
		domain.SubjectSessionClosed:     h.HandleSessionClosed,
	}
	var wg sync.WaitGroup
	for subject, fn := range bind {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := c.SubscribeEphemeral(ctx, "MCP", subject, fn); err != nil && onErr != nil {
				onErr(subject, err)
			}
		}()
	}
	wg.Wait()
}
