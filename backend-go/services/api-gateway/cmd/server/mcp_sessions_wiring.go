package main

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcppolicy"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpsession"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

// lateSessionCloser lets the mcp.session.close WS channel (registered before
// the /mcp handler exists) reach the handler once it is built.
type lateSessionCloser struct {
	mu sync.Mutex
	h  *mcpserver.Handler
}

func (l *lateSessionCloser) set(h *mcpserver.Handler) {
	l.mu.Lock()
	l.h = h
	l.mu.Unlock()
}

// CloseSession fans the close out to every replica (cancels in-flight tools,
// drops the resume buffer). No-op while /mcp is not mounted.
func (l *lateSessionCloser) CloseSession(rowID, reason string) {
	l.mu.Lock()
	h := l.h
	l.mu.Unlock()
	if h != nil {
		h.CloseSession(rowID, reason)
	}
}

// buildMCPSessionStack wires BE-MCP-SOL-004: the durable session store
// (mcp-service), core-NATS signals, the bounded JetStream resume buffer, the
// session kill-switch check and the verified-request context hook. Every
// piece degrades independently: no mcp-service -> in-process sessions; no NATS
// -> signals stay local and the resume buffer is a bounded in-memory one.
func buildMCPSessionStack(ctx context.Context, natsURL string, maxBufferBytes int64, client mcpv1.McpServiceClient, guard *mcppolicy.KillGuard, logger *slog.Logger) (opts []func(*mcpserver.Deps), cleanup func()) {
	var closers []func()
	opts = append(opts, func(d *mcpserver.Deps) { d.RequestContext = mcppolicy.RequestContext })
	if client != nil {
		store := mcpsession.NewGRPCStore(client)
		opts = append(opts, func(d *mcpserver.Deps) { d.Sessions = store })
	} else {
		logger.Warn("no mcp-service: MCP sessions live in this replica's memory only")
	}
	if guard != nil {
		opts = append(opts, func(d *mcpserver.Deps) {
			d.KillCheck = func(ctx context.Context, p mcpserver.Principal, row string) (bool, error) {
				blocked, _, err := guard.Blocked(ctx, p, row)
				return blocked, err
			}
		})
	}
	if natsURL != "" {
		if eph, closeEph, err := eventbus.NewEphemeral(natsURL); err != nil {
			logger.Warn("MCP cross-replica signals unavailable (NATS)", slog.Any("error", err))
		} else {
			closers = append(closers, closeEph)
			opts = append(opts, func(d *mcpserver.Deps) { d.Signals = eph })
		}
		lctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		log, err := mcpsession.NewLog(lctx, natsURL, maxBufferBytes)
		cancel()
		if err != nil {
			logger.Warn("MCP resume buffer unavailable (NATS JetStream): using a bounded in-memory buffer, resume works on one replica only", slog.Any("error", err))
		} else {
			closers = append(closers, log.Close)
			store := mcpsession.NewJetStreamEventStore(log, 0)
			opts = append(opts, func(d *mcpserver.Deps) { d.EventStore = store })
		}
	}
	return opts, func() {
		for _, c := range closers {
			c()
		}
	}
}
