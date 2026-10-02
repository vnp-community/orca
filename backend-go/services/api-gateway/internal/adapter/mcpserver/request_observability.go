package mcpserver

import (
	"context"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// RequestRecorder is an OPTIONAL extension of Recorder (type-asserted, like
// SessionRecorder) that counts JSON-RPC requests for orca_mcp_requests_total.
// method is already bounded by MetricMethod; result is ok | rpc_error |
// http_4xx | http_5xx. Implementations must not add tenant/user/session labels.
type RequestRecorder interface {
	Request(method, result string)
}

// tracerName is the instrumentation scope of the MCP spans (mcp.request here,
// mcp.policy and mcp.dispatch in mcpmetrics).
const tracerName = "orca/api-gateway/mcp"

// metricMethods is the closed set of JSON-RPC methods used as a metric label
// and span attribute; anything else collapses so a client cannot mint series.
var metricMethods = map[string]struct{}{
	"initialize": {}, "ping": {}, "tools/list": {}, "tools/call": {},
	"resources/list": {}, "resources/templates/list": {}, "resources/read": {},
	"resources/subscribe": {}, "resources/unsubscribe": {},
	"prompts/list": {}, "prompts/get": {}, "logging/setLevel": {}, "completion/complete": {},
}

// MetricMethod maps a JSON-RPC method to a bounded label value.
func MetricMethod(method string) string {
	if _, ok := metricMethods[method]; ok {
		return method
	}
	if len(method) > len("notifications/") && method[:len("notifications/")] == "notifications/" {
		return "notification"
	}
	return "other"
}

// observed wraps the engine middleware with the mcp.request span and the
// request counter. Span attributes are an allow-list: method, session row id
// and protocol version only - never arguments, tokens or cookies.
func (e *engine) observed(next mcp.MethodHandler) mcp.MethodHandler {
	inner := e.middleware(next)
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		m := MetricMethod(method)
		ctx, span := otel.Tracer(tracerName).Start(ctx, "mcp.request", trace.WithAttributes(attribute.String("mcp.method", m)))
		res, err := inner(ctx, method, req)
		if ss := req.GetSession(); ss != nil {
			if e.host != nil {
				if ls := e.host.local(ss.ID()); ls != nil && ls.row() != "" {
					span.SetAttributes(attribute.String("mcp.session.row_id", ls.row()))
				}
			}
			if ip := serverSessionInit(ss); ip != "" {
				span.SetAttributes(attribute.String("mcp.protocol_version", ip))
			}
		}
		result := "ok"
		if err != nil {
			result = "rpc_error"
			span.SetStatus(codes.Error, "rpc_error")
		}
		span.End()
		if e.reqRec != nil {
			e.reqRec.Request(m, result)
		}
		return res, err
	}
}

func serverSessionInit(s mcp.Session) string {
	if ss, ok := s.(*mcp.ServerSession); ok {
		if p := ss.InitializeParams(); p != nil {
			return p.ProtocolVersion
		}
	}
	return ""
}

// statusRecorder captures the status code and keeps Flusher/Unwrap working,
// so SSE responses still stream through it.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return s.ResponseWriter.Write(b)
}

func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// observeHTTP counts transport-level failures (auth, origin, size, rate
// limit, 5xx) that never reach the JSON-RPC layer, under method="http".
func (h *Handler) observeHTTP(next http.Handler) http.Handler {
	rr, ok := h.rec.(RequestRecorder)
	if !ok {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sr := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(sr, r)
		switch {
		case sr.status >= 500:
			rr.Request("http", "http_5xx")
		case sr.status >= 400:
			rr.Request("http", "http_4xx")
		}
	})
}
