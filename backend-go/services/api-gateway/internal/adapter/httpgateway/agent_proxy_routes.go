package httpgateway

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

// NewAgentProxyHandler builds a raw HTTP/WebSocket reverse proxy to
// infra-fleet-service's own HTTP server, which terminates the Dev Server
// Agent's connection directly (infra-fleet-service/cmd/server/main.go:
// mux.Handle("/agent", agentWSServer), mux.Handle("/api/agent-token",
// agentTokenIssuer)).
//
// Unlike every other downstream route in this package, this traffic is
// deliberately NOT translated through gRPC: the Dev Server Agent is an
// external process (runs on a physical dev machine, outside this cluster)
// speaking a raw JSON-RPC-over-binary-frame wire protocol
// (agent/src/shared/remote-runtime-*, Stack A), not a browser/REST/gRPC
// client — api-gateway has no reason to parse those frames, only to get
// bytes to infra-fleet-service and back. This mirrors the "never
// interpreting frame contents" principle notification_stream.go's WS<->gRPC
// bridge already follows for its own (gRPC-native) stream, just via a plain
// byte-for-byte proxy instead of a stream translation, since there is no
// gRPC-native equivalent to bridge to here.
//
// net/http/httputil.ReverseProxy transparently hijacks and pipes WS
// upgrades (since Go 1.12), so one proxy instance correctly serves both the
// WS handshake (/agent) and the plain HTTP token exchange
// (/api/agent-token).
//
// addr is a bare host:port on the internal orca-go-net network (e.g.
// "infra-fleet-service:8080") — see Config.InfraFleetHTTPAddr's doc comment
// for why this is a second address distinct from the gRPC one
// infra_routes.go's REST proxy handlers dial.
func NewAgentProxyHandler(addr string) http.Handler {
	target := &url.URL{Scheme: "http", Host: addr}
	return httputil.NewSingleHostReverseProxy(target)
}
