// Package mcpmetrics exposes the orca_mcp_* Prometheus metrics of the /mcp
// edge and the OTel spans that tie one MCP request to its policy decision and
// channel dispatch (BE-MCP-SOL-015 section C). It plugs into mcpserver through
// the Recorder/SessionRecorder/RequestRecorder interfaces and through
// decorators of PolicyGate, ToolExecutor and Dispatcher, so mcpserver itself
// never imports Prometheus.
//
// Cardinality rule: every label value comes from a closed set (JSON-RPC
// methods, risk levels, outcomes, catalog tool names). Tenant, user, session,
// client and trace ids are never labels; unknown tool names collapse to
// "unknown" so a client cannot mint series.
package mcpmetrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics owns a private registry (not the default one) and every collector.
type Metrics struct {
	reg *prometheus.Registry

	requests         *prometheus.CounterVec
	toolCalls        *prometheus.CounterVec
	toolDuration     *prometheus.HistogramVec
	sessionsActive   prometheus.Gauge
	sseStreamsActive prometheus.Gauge
	resume           *prometheus.CounterVec
	approvals        *prometheus.CounterVec
	policyDenials    *prometheus.CounterVec
	authFailures     *prometheus.CounterVec
	principalResolve *prometheus.CounterVec
	identityMismatch prometheus.Counter
	sseDropped       *prometheus.CounterVec
	rateLimited      prometheus.Counter
	cancelled        prometheus.Counter
	sessionsClosed   *prometheus.CounterVec
}

// New registers all collectors on a fresh registry.
func New() *Metrics {
	m := &Metrics{reg: prometheus.NewRegistry()}
	m.requests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "orca_mcp_requests_total", Help: "MCP JSON-RPC requests by method and outcome; method=http counts transport-level failures."}, []string{"method", "result"})
	m.toolCalls = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "orca_mcp_tool_calls_total", Help: "tools/call by catalog tool, policy decision and result."}, []string{"tool", "decision", "result"})
	m.toolDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "orca_mcp_tool_duration_seconds", Help: "Wall time of admitted tools/call by risk and namespace.",
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60}}, []string{"risk", "namespace"})
	m.sessionsActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "orca_mcp_sessions_active", Help: "MCP sessions live on this replica (the cluster-wide count is exposed by mcp-service)."})
	m.sseStreamsActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "orca_mcp_sse_streams_active", Help: "Open SSE streams on this replica."})
	m.resume = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "orca_mcp_resume_total", Help: "SSE resume attempts by result (ok|gap|unsupported)."}, []string{"result"})
	m.approvals = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "orca_mcp_approvals_total", Help: "Approval waits seen by the gateway (denied includes expiry and wait timeouts)."}, []string{"outcome"})
	m.policyDenials = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "orca_mcp_policy_denials_total", Help: "Tool calls denied by the policy gate."}, []string{"reason"})
	m.authFailures = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "orca_mcp_auth_failures_total", Help: "Rejected /mcp authentications by reason."}, []string{"reason"})
	m.principalResolve = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "orca_mcp_principal_resolve_total", Help: "ResolveMcpPrincipal outcomes (cache_hit|active|inactive|error)."}, []string{"result"})
	m.identityMismatch = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "orca_mcp_session_identity_mismatch_total", Help: "Requests presenting a session id that belongs to another identity."})
	m.sseDropped = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "orca_mcp_sse_events_dropped_total", Help: "SSE events dropped before delivery, by reason."}, []string{"reason"})
	m.rateLimited = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "orca_mcp_rate_limited_total", Help: "Requests or streams refused by a rate/stream limit."})
	m.cancelled = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "orca_mcp_requests_cancelled_total", Help: "In-flight requests cancelled by the client."})
	m.sessionsClosed = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "orca_mcp_sessions_closed_total", Help: "Sessions ended on this replica by reason."}, []string{"reason"})
	m.reg.MustRegister(m.requests, m.toolCalls, m.toolDuration, m.sessionsActive, m.sseStreamsActive, m.resume,
		m.approvals, m.policyDenials, m.authFailures, m.principalResolve, m.identityMismatch, m.sseDropped,
		m.rateLimited, m.cancelled, m.sessionsClosed)
	return m
}

// Registry lets the composition root add collectors (e.g. Go runtime).
func (m *Metrics) Registry() *prometheus.Registry { return m.reg }

// Handler serves the Prometheus text exposition of this registry.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{})
}

// pick returns v when it is in the closed set, else fallback.
func pick(v, fallback string, allowed ...string) string {
	for _, a := range allowed {
		if v == a {
			return v
		}
	}
	return fallback
}

// ---- mcpserver.Recorder ----

var authReasons = []string{"no_credentials", "invalid_token", "insufficient_scope", "cookie_only",
	"verifier_unavailable", "kill_switch", "expired", "audience", "origin", "revoked"}

func (m *Metrics) AuthFailure(reason string) {
	m.authFailures.WithLabelValues(pick(reason, "other", authReasons...)).Inc()
}
func (m *Metrics) IdentityMismatch() { m.identityMismatch.Inc() }
func (m *Metrics) RateLimited()      { m.rateLimited.Inc() }

// ---- mcpserver.SessionRecorder ----

var closeReasons = []string{"client_delete", "idle", "user", "admin", "kill_switch", "token_revoked", "shutdown", "expired"}

func (m *Metrics) SessionOpened() { m.sessionsActive.Inc() }
func (m *Metrics) SessionClosed(reason string) {
	m.sessionsActive.Dec()
	m.sessionsClosed.WithLabelValues(pick(reason, "other", closeReasons...)).Inc()
}
func (m *Metrics) StreamOpened()     { m.sseStreamsActive.Inc() }
func (m *Metrics) StreamClosed()     { m.sseStreamsActive.Dec() }
func (m *Metrics) RequestCancelled() { m.cancelled.Inc() }
func (m *Metrics) Resume(result string) {
	m.resume.WithLabelValues(pick(result, "other", "ok", "gap", "unsupported")).Inc()
}
func (m *Metrics) ProgressCoalesced(dropped int) {
	if dropped > 0 {
		m.sseDropped.WithLabelValues("coalesced").Add(float64(dropped))
	}
}

// ---- mcpserver.RequestRecorder ----

func (m *Metrics) Request(method, result string) {
	m.requests.WithLabelValues(pick(method, "other", knownMethods...), pick(result, "rpc_error", "ok", "rpc_error", "http_4xx", "http_5xx")).Inc()
}

var knownMethods = []string{"initialize", "ping", "tools/list", "tools/call", "resources/list", "resources/templates/list",
	"resources/read", "resources/subscribe", "resources/unsubscribe", "prompts/list", "prompts/get", "logging/setLevel",
	"completion/complete", "notification", "http", "other"}

// ObservePrincipalResolve is the authclient.McpPrincipalResolver.OnResolve hook.
func (m *Metrics) ObservePrincipalResolve(result string) {
	m.principalResolve.WithLabelValues(pick(result, "other", "cache_hit", "active", "inactive", "error")).Inc()
}
