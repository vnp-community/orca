package mcpserver

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// capabilities declares ONLY what is implemented. Resources: list, templates,
// read, and subscribe when the provider has a working event source;
// resources.listChanged is not emitted yet. Prompts: list/get + list_changed
// (driven by Handler.NotifyPromptsChanged).
func (e *engine) capabilities() *mcp.ServerCapabilities {
	caps := &mcp.ServerCapabilities{Tools: &mcp.ToolCapabilities{}, Logging: &mcp.LoggingCapabilities{}}
	if e.resources != nil {
		caps.Resources = &mcp.ResourceCapabilities{Subscribe: e.resources.Subscribable()}
	}
	if e.prompts != nil {
		caps.Prompts = &mcp.PromptCapabilities{ListChanged: true}
	}
	return caps
}

// resourceError maps provider errors to JSON-RPC errors. Not-found, forbidden
// and malformed share one answer.
func (e *engine) resourceError(ctx context.Context, uri string, err error) error {
	var rpc *jsonrpc.Error
	switch {
	case errors.Is(err, ErrResourceNotFound):
		return mcp.ResourceNotFoundError(uri)
	case errors.Is(err, ErrSubscriptionUnsupported):
		return invalidParams("subscription not supported for this resource")
	case errors.Is(err, ErrSubscriptionLimit):
		return invalidParams("too many resource subscriptions")
	case errors.As(err, &rpc):
		return err
	case ctx.Err() != nil:
		return ctx.Err()
	}
	e.log.ErrorContext(ctx, "mcp resources: provider failed", "error", err)
	return &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: "internal error"}
}

func (e *engine) listResources(ctx context.Context, req mcp.Request) (mcp.Result, error) {
	p, ok := principalOf(req)
	params, _ := req.GetParams().(*mcp.ListResourcesParams)
	if params == nil {
		params = &mcp.ListResourcesParams{}
	}
	if !ok {
		return nil, &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: "internal error"}
	}
	items, err := e.resources.ListResources(ctx, p)
	if err != nil {
		return nil, e.resourceError(ctx, "", err)
	}
	page, next, err := Paginate(e.cursors, "resources", req.GetSession().ID(), params.Cursor, items, e.pageSize)
	if err != nil {
		return nil, err
	}
	return &mcp.ListResourcesResult{Resources: page, NextCursor: next}, nil
}

// listResourceTemplates is a small fixed set: no pagination.
func (e *engine) listResourceTemplates(ctx context.Context, req mcp.Request) (mcp.Result, error) {
	p, ok := principalOf(req)
	if !ok {
		return nil, &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: "internal error"}
	}
	items, err := e.resources.ListTemplates(ctx, p)
	if err != nil {
		return nil, e.resourceError(ctx, "", err)
	}
	return &mcp.ListResourceTemplatesResult{ResourceTemplates: items}, nil
}

func (e *engine) readResource(ctx context.Context, req mcp.Request) (mcp.Result, error) {
	p, ok := principalOf(req)
	params, _ := req.GetParams().(*mcp.ReadResourceParams)
	if !ok || params == nil {
		return nil, &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: "internal error"}
	}
	res, err := e.resources.ReadResource(ctx, p, req.GetSession().ID(), params.URI)
	if err != nil {
		return nil, e.resourceError(ctx, params.URI, err)
	}
	return res, nil
}

func (e *engine) listPrompts(ctx context.Context, req mcp.Request) (mcp.Result, error) {
	p, ok := principalOf(req)
	params, _ := req.GetParams().(*mcp.ListPromptsParams)
	if params == nil {
		params = &mcp.ListPromptsParams{}
	}
	if !ok {
		return nil, &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: "internal error"}
	}
	items, err := e.prompts.ListPrompts(ctx, p, acceptLanguage(req))
	if err != nil {
		return nil, e.promptError(ctx, err)
	}
	page, next, err := Paginate(e.cursors, "prompts", req.GetSession().ID(), params.Cursor, items, e.pageSize)
	if err != nil {
		return nil, err
	}
	return &mcp.ListPromptsResult{Prompts: page, NextCursor: next}, nil
}

func (e *engine) getPrompt(ctx context.Context, req mcp.Request) (mcp.Result, error) {
	p, ok := principalOf(req)
	params, _ := req.GetParams().(*mcp.GetPromptParams)
	if !ok || params == nil {
		return nil, &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: "internal error"}
	}
	res, err := e.prompts.GetPrompt(ctx, p, req.GetSession().ID(), params.Name, params.Arguments, acceptLanguage(req))
	if err != nil {
		return nil, e.promptError(ctx, err)
	}
	return res, nil
}

func (e *engine) promptError(ctx context.Context, err error) error {
	var rpc *jsonrpc.Error
	switch {
	case errors.As(err, &rpc):
		return err
	case ctx.Err() != nil:
		return ctx.Err()
	}
	e.log.ErrorContext(ctx, "mcp prompts: provider failed", "error", err)
	return &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: "internal error"}
}

func acceptLanguage(req mcp.Request) string {
	if x := req.GetExtra(); x != nil && x.Header != nil {
		return x.Header.Get("Accept-Language")
	}
	return ""
}

// ---- subscriptions -------------------------------------------------------

func (e *engine) subscribeHandler() func(context.Context, *mcp.SubscribeRequest) error {
	if e.resources == nil || !e.resources.Subscribable() {
		return nil
	}
	return func(ctx context.Context, req *mcp.SubscribeRequest) error {
		p, ok := principalOf(req)
		if !ok || req.Params == nil {
			return &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: "internal error"}
		}
		if err := e.resources.Subscribe(ctx, p, req.Session.ID(), req.Params.URI); err != nil {
			return e.resourceError(ctx, req.Params.URI, err)
		}
		e.subs.track(req.Session, e.resources.EndSession)
		return nil
	}
}

func (e *engine) unsubscribeHandler() func(context.Context, *mcp.UnsubscribeRequest) error {
	if e.resources == nil || !e.resources.Subscribable() {
		return nil
	}
	return func(_ context.Context, req *mcp.UnsubscribeRequest) error {
		if req.Params != nil {
			e.resources.Unsubscribe(req.Session.ID(), req.Params.URI)
		}
		return nil
	}
}

// subscribedSessions remembers sessions holding subscriptions so a provider
// can close one, and runs a watcher per session that ends its subscriptions
// when the session ends (client DELETE, idle timeout or transport loss).
type subscribedSessions struct {
	mu sync.Mutex
	m  map[string]*mcp.ServerSession
}

func (s *subscribedSessions) track(ss *mcp.ServerSession, end func(sessionID string)) {
	id := ss.ID()
	s.mu.Lock()
	if s.m == nil {
		s.m = map[string]*mcp.ServerSession{}
	}
	if cur, ok := s.m[id]; ok && cur == ss {
		s.mu.Unlock()
		return
	}
	s.m[id] = ss
	s.mu.Unlock()
	go func() {
		_ = ss.Wait()
		s.mu.Lock()
		if s.m[id] == ss {
			delete(s.m, id)
		}
		s.mu.Unlock()
		end(id)
	}()
}

func (s *subscribedSessions) get(id string) *mcp.ServerSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[id]
}

// ResourceUpdated / CloseSession implement ResourceNotifier.
func (e *engine) ResourceUpdated(uri string) {
	if e.server == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.server.ResourceUpdated(ctx, &mcp.ResourceUpdatedNotificationParams{URI: uri}); err != nil {
		e.log.Warn("mcp resources/updated failed", "error", err)
	}
}

func (e *engine) CloseSession(sessionID string) {
	if ss := e.subs.get(sessionID); ss != nil {
		_ = ss.Close()
	}
}

// promptsChanged replaces a never-listed sentinel prompt: the SDK only emits
// prompts/list_changed through AddPrompt/RemovePrompts. prompts/list and
// prompts/get are answered by the PromptProvider, never from the SDK registry.
func (e *engine) promptsChanged() {
	if e.server == nil || e.prompts == nil {
		return
	}
	e.server.AddPrompt(&mcp.Prompt{Name: "orca_list_changed_sentinel"},
		func(context.Context, *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			return nil, invalidParams("unknown prompt")
		})
}
