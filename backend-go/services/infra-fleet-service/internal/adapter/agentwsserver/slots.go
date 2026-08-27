package agentwsserver

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// DefaultConnectTimeout mirrors AGENT_CONNECT_TIMEOUT_MS (60s) — how long a
// slot stays pending before Register's onExpired fires, independent of the
// issued token's own (potentially much longer, e.g. 30-day permanent)
// nominal TTL — see token_endpoint.go's doc comment on why those two
// timeouts are deliberately different.
const DefaultConnectTimeout = 60 * time.Second

// Registry is the in-memory, single-use pending-token slot store an inbound
// agent.handshake's token is checked against — one entry per issued token
// that hasn't yet been consumed by a real connection, keyed by
// SHA-256(token) hex so a heap dump or log line never exposes a live
// plaintext token (mirrors agent-ws-server.ts's pendingSlots/hashToken,
// added there under FIX TASK-AWS-002). Safe for concurrent use.
type Registry struct {
	ttl time.Duration

	mu    sync.Mutex
	slots map[string]*slot
}

type slot struct {
	devServerID string
	timer       *time.Timer
}

// NewRegistry constructs a Registry whose slots expire after ttl if nobody
// consumes them — pass DefaultConnectTimeout in production; tests pass a
// short TTL so they don't need to sleep 60 real seconds.
func NewRegistry(ttl time.Duration) *Registry {
	return &Registry{ttl: ttl, slots: make(map[string]*slot)}
}

// Register issues a pending slot for agentToken, associated with
// devServerID. Re-registering the SAME token cancels the previous slot's
// timer first — an idempotent re-register, matching registerSlot's own
// "Clear any existing slot for same token" behavior (an agent reconnect
// cycle that re-issues the same token must not leak the old timer).
// onExpired (may be nil) fires with a human-readable reason if nobody calls
// Consume with this token within the Registry's ttl. The returned disposer
// cancels the timer and removes the slot early — e.g. if the DevServer is
// deleted before the agent ever connects.
func (r *Registry) Register(agentToken, devServerID string, onExpired func(reason string)) (unregister func()) {
	hash := hashToken(agentToken)

	r.mu.Lock()
	r.removeLocked(hash)

	timer := time.AfterFunc(r.ttl, func() {
		r.mu.Lock()
		_, stillPending := r.slots[hash]
		delete(r.slots, hash)
		r.mu.Unlock()

		// stillPending is false if Consume (or another Register racing in)
		// already removed this slot before the timer fired — Stop() alone
		// cannot prevent an already-fired callback from running, so this
		// re-check is what actually makes expiry mutually exclusive with a
		// concurrent Consume.
		if stillPending && onExpired != nil {
			onExpired(fmt.Sprintf(
				"direct-websocket: agent did not connect within %s. Configure your agent with AGENT_TOKEN=%s",
				r.ttl, agentToken,
			))
		}
	})
	r.slots[hash] = &slot{devServerID: devServerID, timer: timer}
	r.mu.Unlock()

	return func() {
		r.mu.Lock()
		r.removeLocked(hash)
		r.mu.Unlock()
	}
}

// removeLocked cancels and deletes hash's slot, if present. Caller must
// hold r.mu.
func (r *Registry) removeLocked(hash string) {
	if s, ok := r.slots[hash]; ok {
		s.timer.Stop()
		delete(r.slots, hash)
	}
}

// Consume looks up agentToken and, if a pending slot exists (registered,
// not yet consumed, not yet expired), cancels its expiry timer, removes it
// — slots are single-use, so a second Consume of the same token fails — and
// returns the DevServer ID it was registered for.
func (r *Registry) Consume(agentToken string) (devServerID string, ok bool) {
	hash := hashToken(agentToken)

	r.mu.Lock()
	defer r.mu.Unlock()
	s, found := r.slots[hash]
	if !found {
		return "", false
	}
	s.timer.Stop()
	delete(r.slots, hash)
	return s.devServerID, true
}

// Has reports whether agentToken currently has a live pending slot, without
// consuming it — used by token_endpoint.go's GET listing to hide a token
// that has already been consumed by a real handshake or whose connect-time
// slot has already expired, independent of that token's own longer nominal
// TTL.
func (r *Registry) Has(agentToken string) bool {
	hash := hashToken(agentToken)
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.slots[hash]
	return ok
}

// Stop cancels every pending slot's timer — call on service shutdown.
func (r *Registry) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for hash, s := range r.slots {
		s.timer.Stop()
		delete(r.slots, hash)
	}
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
