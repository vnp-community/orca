package mcpserver

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// progressInterval caps notifications/progress at 5 per second per token.
const progressInterval = 200 * time.Millisecond

// obsTransport wraps the SDK's StreamableServerTransport so the gateway can
// (a) inject messages into the session from another replica (cancellation),
// (b) coalesce progress notifications and (c) know how many requests are in
// flight. The inner transport still serves HTTP; only the Connection is wrapped.
type obsTransport struct {
	inner *mcp.StreamableServerTransport
	conn  *obsConn
	rec   SessionRecorder
	now   func() time.Time
}

func (t *obsTransport) Connect(ctx context.Context) (mcp.Connection, error) {
	c, err := t.inner.Connect(ctx)
	if err != nil {
		return nil, err
	}
	t.conn = &obsConn{Connection: c, rec: t.rec, now: t.now, in: make(chan readResult), inject: make(chan jsonrpc.Message, 16),
		done: make(chan struct{}), prog: map[string]*progressState{}}
	return t.conn, nil
}

func (t *obsTransport) SupportsProtocolVersion(v string) bool {
	return t.inner.SupportsProtocolVersion(v)
}

type readResult struct {
	msg jsonrpc.Message
	err error
}

type progressState struct {
	last    time.Time
	pending *jsonrpc.Request
	ctx     context.Context
	timer   *time.Timer
}

type obsConn struct {
	mcp.Connection
	rec SessionRecorder
	now func() time.Time

	pumpOnce sync.Once
	in       chan readResult
	inject   chan jsonrpc.Message
	done     chan struct{}
	closeMu  sync.Once

	inflight atomic.Int64

	pmu    sync.Mutex
	prog   map[string]*progressState
	closed bool
}

// injectedHeader marks messages that came from a peer replica, so the
// middleware does not re-broadcast them (signal ping-pong).
const injectedHeader = "X-Orca-Injected"

// Inject feeds a message into the session as if the client had sent it.
func (c *obsConn) Inject(msg jsonrpc.Message) bool {
	select {
	case c.inject <- msg:
		return true
	default:
		return false
	}
}

func (c *obsConn) pump() {
	for {
		msg, err := c.Connection.Read(context.Background())
		select {
		case c.in <- readResult{msg, err}:
		case <-c.done:
			return
		}
		if err != nil {
			return
		}
	}
}

func (c *obsConn) Read(ctx context.Context) (jsonrpc.Message, error) {
	c.pumpOnce.Do(func() { go c.pump() })
	select {
	case m := <-c.inject:
		return m, nil
	case r := <-c.in:
		if req, ok := r.msg.(*jsonrpc.Request); ok && req.IsCall() {
			c.inflight.Add(1)
		}
		return r.msg, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.done:
		return nil, io.EOF
	}
}

// Inflight is the number of calls received and not yet answered.
func (c *obsConn) Inflight() int64 { return c.inflight.Load() }

func (c *obsConn) Write(ctx context.Context, msg jsonrpc.Message) error {
	switch m := msg.(type) {
	case *jsonrpc.Response:
		c.flushProgress() // progress must never trail the final response
		if c.inflight.Add(-1) < 0 {
			c.inflight.Store(0)
		}
	case *jsonrpc.Request:
		if m.Method == "notifications/progress" && !m.IsCall() {
			if tok, ok := progressToken(m.Params); ok {
				return c.throttleProgress(ctx, tok, m)
			}
		}
	}
	return c.Connection.Write(ctx, msg)
}

func progressToken(params json.RawMessage) (string, bool) {
	var p struct {
		Token json.RawMessage `json:"progressToken"`
	}
	if json.Unmarshal(params, &p) != nil || len(p.Token) == 0 {
		return "", false
	}
	return string(p.Token), true
}

func (c *obsConn) throttleProgress(ctx context.Context, tok string, m *jsonrpc.Request) error {
	c.pmu.Lock()
	if c.closed {
		c.pmu.Unlock()
		return io.ErrClosedPipe
	}
	st := c.prog[tok]
	if st == nil {
		st = &progressState{}
		c.prog[tok] = st
	}
	now := c.now()
	if now.Sub(st.last) >= progressInterval {
		st.last, st.pending = now, nil
		if st.timer != nil {
			st.timer.Stop()
			st.timer = nil
		}
		c.pmu.Unlock()
		return c.Connection.Write(ctx, m)
	}
	if st.pending != nil && c.rec != nil {
		c.rec.ProgressCoalesced(1)
	}
	st.pending, st.ctx = m, context.WithoutCancel(ctx) // keeps the request routing value
	if st.timer == nil {
		wait := progressInterval - now.Sub(st.last)
		st.timer = time.AfterFunc(wait, func() { c.flushToken(tok) })
	}
	c.pmu.Unlock()
	return nil
}

func (c *obsConn) flushToken(tok string) {
	c.pmu.Lock()
	st := c.prog[tok]
	if st == nil || st.pending == nil || c.closed {
		c.pmu.Unlock()
		return
	}
	m, ctx := st.pending, st.ctx
	st.pending, st.timer, st.last = nil, nil, c.now()
	c.pmu.Unlock()
	_ = c.Connection.Write(ctx, m) // best effort: the stream may be gone
}

// flushProgress writes the newest pending progress of every token now.
func (c *obsConn) flushProgress() {
	c.pmu.Lock()
	type item struct {
		m   *jsonrpc.Request
		ctx context.Context
	}
	var out []item
	for tok, st := range c.prog {
		if st.timer != nil {
			st.timer.Stop()
		}
		if st.pending != nil {
			out = append(out, item{st.pending, st.ctx})
		}
		delete(c.prog, tok)
	}
	c.pmu.Unlock()
	for _, it := range out {
		_ = c.Connection.Write(it.ctx, it.m)
	}
}

// Notify sends a server notification on the standalone stream.
func (c *obsConn) Notify(method string, params any) error {
	var raw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return err
		}
		raw = b
	}
	return c.Connection.Write(context.Background(), &jsonrpc.Request{Method: method, Params: raw})
}

func (c *obsConn) Close() error {
	c.closeMu.Do(func() { close(c.done) })
	c.pmu.Lock()
	c.closed = true
	for _, st := range c.prog {
		if st.timer != nil {
			st.timer.Stop()
		}
	}
	c.prog = map[string]*progressState{}
	c.pmu.Unlock()
	return c.Connection.Close()
}
