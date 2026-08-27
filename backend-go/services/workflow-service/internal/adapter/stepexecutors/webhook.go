package stepexecutors

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// webhookStepConfig is the Webhook step type's config shape — the native
// HTTP call workflow-service.md §4/§9 describes as "the one step type
// making a native HTTP call from this service's own network position".
type webhookStepConfig struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Body    json.RawMessage   `json:"body"`
}

type webhookResultOutput struct {
	StatusCode int    `json:"statusCode,omitempty"`
	Body       string `json:"body,omitempty"`
	Error      string `json:"error,omitempty"`
}

// maxWebhookResponseBytes bounds how much of a webhook target's response
// this service will read — §9's "bound response size/request time"
// requirement. A misbehaving or malicious target streaming gigabytes back
// must not exhaust this service's memory.
const maxWebhookResponseBytes = 1 << 20 // 1 MiB

// WebhookExecutor is the real, in-process implementation of the Webhook
// step type — native net/http, no relay hop. See §9's SSRF requirements:
// this is the ONE step type that gives an unvalidated, user-supplied URL a
// direct path to this service's own network position, so it carries the
// SSRF-prevention checks the other four step types don't need.
//
// Known gap (see README): the allowlist here is a basic, config-driven
// hostname allowlist (empty by default — see internal/config), not the
// full "reject private/link-local/loopback IP ranges, re-validated after
// every redirect hop, per-tenant domain allowlist" posture §9 specifies as
// required. Precedence between the two checks: when an allowlist is
// configured, an exact hostname match is trusted outright (that's the
// point of an operator-curated allowlist — it may legitimately include an
// internal target) and the automatic IP-range block is skipped for that
// host; when no allowlist is configured, the private/loopback/link-local
// IP block is the sole safety net and applies unconditionally. Redirects
// are disabled outright rather than re-validated per hop — simpler and
// strictly safer for a stub, but a genuine narrowing from the full spec.
type WebhookExecutor struct {
	client         *http.Client
	allowlistHosts map[string]struct{} // empty = no additional restriction beyond the SSRF IP block
}

// NewWebhookExecutor constructs a WebhookExecutor. allowlistHosts, when
// non-empty, additionally restricts targets to those exact hostnames (case
// -insensitive) — loaded from config, empty by default per this build's
// instructions ("basic SSRF-prevention... allowlist itself is just loaded
// from config/empty-by-default").
func NewWebhookExecutor(allowlistHosts []string, client *http.Client) *WebhookExecutor {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	// Never follow redirects: a target that 302s to an internal address
	// would otherwise bypass the pre-request SSRF check entirely — see
	// §9's "re-validated after every redirect hop" requirement. Refusing
	// to follow redirects at all is the simplest way to satisfy that for
	// this scaffold; a future revision that wants to follow redirects must
	// re-run resolveAndCheckSSRF on every hop's Location, not just the
	// first request.
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return errWebhookRedirectRefused
	}

	allowlist := make(map[string]struct{}, len(allowlistHosts))
	for _, h := range allowlistHosts {
		allowlist[strings.ToLower(strings.TrimSpace(h))] = struct{}{}
	}
	return &WebhookExecutor{client: client, allowlistHosts: allowlist}
}

var errWebhookRedirectRefused = fmt.Errorf("stepexecutors: webhook: redirects are not followed (SSRF hardening)")

func (e *WebhookExecutor) Execute(ctx context.Context, stepConfigJSON string) (domain.StepResult, error) {
	var cfg webhookStepConfig
	if err := json.Unmarshal([]byte(stepConfigJSON), &cfg); err != nil {
		return domain.StepResult{}, fmt.Errorf("stepexecutors: webhook: invalid step config JSON: %w", err)
	}

	method := strings.ToUpper(strings.TrimSpace(cfg.Method))
	if method == "" {
		method = http.MethodPost
	}

	parsedURL, err := e.checkTarget(ctx, cfg.URL)
	if err != nil {
		return domain.StepResult{}, fmt.Errorf("stepexecutors: webhook: target rejected: %w", err)
	}

	var body io.Reader
	if len(cfg.Body) > 0 {
		body = bytes.NewReader(cfg.Body)
	}

	req, err := http.NewRequestWithContext(ctx, method, parsedURL.String(), body)
	if err != nil {
		return domain.StepResult{}, fmt.Errorf("stepexecutors: webhook: build request: %w", err)
	}
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return domain.StepResult{}, fmt.Errorf("stepexecutors: webhook: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxWebhookResponseBytes))
	if err != nil {
		return domain.StepResult{}, fmt.Errorf("stepexecutors: webhook: reading response: %w", err)
	}

	status := domain.ResultStatusCompleted
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// A non-2xx response is a legitimate business-level "failed" step
		// outcome, not an executor malfunction — the request was sent and
		// answered, the target just reported failure.
		status = domain.ResultStatusFailed
	}

	output, err := json.Marshal(webhookResultOutput{StatusCode: resp.StatusCode, Body: string(respBody)})
	if err != nil {
		return domain.StepResult{}, fmt.Errorf("stepexecutors: webhook: marshal output: %w", err)
	}
	return domain.StepResult{Status: status, OutputJSON: string(output)}, nil
}

// checkTarget validates rawURL against §9's SSRF requirements this scaffold
// implements: scheme must be http/https, and — unless the host is an exact
// match in a configured allowlist (trusted outright, see the doc comment
// above) — the resolved IP must not be private/loopback/link-local. If a
// non-empty allowlist is configured and the host doesn't match it, the
// request is rejected before any DNS lookup happens at all.
func (e *WebhookExecutor) checkTarget(ctx context.Context, rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme %q", parsed.Scheme)
	}
	host := parsed.Hostname()
	if host == "" {
		return nil, fmt.Errorf("url has no host")
	}

	if len(e.allowlistHosts) > 0 {
		if _, ok := e.allowlistHosts[strings.ToLower(host)]; ok {
			// Operator explicitly trusted this exact host — skip the
			// automatic private/loopback/link-local block for it.
			return parsed, nil
		}
		return nil, fmt.Errorf("host %q is not in the webhook allowlist", host)
	}

	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolving host %q: %w", host, err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("host %q did not resolve to any address", host)
	}
	for _, ip := range ips {
		if isDisallowedWebhookTarget(ip.IP) {
			return nil, fmt.Errorf("host %q resolves to a private/loopback/link-local address, which is not allowed (SSRF hardening)", host)
		}
	}

	return parsed, nil
}

// isDisallowedWebhookTarget reports whether ip is inside a range §9
// requires rejecting: loopback, link-local, or otherwise private
// (RFC 1918 / ULA) — cluster-metadata endpoints and internal services
// typically live in exactly these ranges.
func isDisallowedWebhookTarget(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsPrivate() ||
		ip.IsUnspecified()
}
