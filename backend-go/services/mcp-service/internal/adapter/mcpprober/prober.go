// Package mcpprober lists the tools of a remote MCP server over Streamable
// HTTP without being usable as an SSRF primitive: the destination is
// resolved here, every resolved address is checked, and the connection is made
// to the checked IP itself (resolve-then-pin), so DNS rebinding between check
// and connect is impossible. There is no proxy and no redirect following.
package mcpprober

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"syscall"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"
)

// Limits (BE-MCP-SOL-014 section C).
const (
	TotalTimeout   = 15 * time.Second // whole probe: initialize + tools/list pages
	DialTimeout    = 5 * time.Second
	MaxBodyBytes   = 1 << 20
	MaxTools       = 200
	MaxPages       = 20
	DefaultVersion = "2025-06-18"
)

// Resolver is the DNS dependency; tests replace it to simulate rebinding.
type Resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

type Config struct {
	Policy   domain.ExternalURLPolicy
	Resolver Resolver // default net.DefaultResolver
	// Blocked overrides domain.IsBlockedIP. Tests only.
	Blocked         func(netip.Addr) bool
	ProtocolVersion string
	// TLSConfig overrides the client TLS config. Tests only.
	TLSConfig *tls.Config
}

type Prober struct {
	cfg    Config
	client *http.Client
}

var _ usecase.ToolProber = (*Prober)(nil)

var errRedirect = errors.New("redirect refused")

func New(cfg Config) *Prober {
	if cfg.Resolver == nil {
		cfg.Resolver = net.DefaultResolver
	}
	if cfg.Blocked == nil {
		cfg.Blocked = domain.IsBlockedIP
	}
	if cfg.ProtocolVersion == "" {
		cfg.ProtocolVersion = DefaultVersion
	}
	p := &Prober{cfg: cfg}
	tlsCfg := cfg.TLSConfig
	if tlsCfg == nil {
		tlsCfg = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	p.client = &http.Client{
		Transport: &http.Transport{
			Proxy:               nil, // never honor HTTP(S)_PROXY: it would move the dial off the checked address
			DialContext:         p.dial,
			TLSClientConfig:     tlsCfg,
			TLSHandshakeTimeout: DialTimeout,
			ForceAttemptHTTP2:   true,
			DisableKeepAlives:   true,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return errRedirect },
	}
	return p
}

// dial resolves addr's host, rejects the connection if ANY resolved address is
// blocked, then connects to the IP literal. A Control hook re-checks the real
// remote address of every attempt.
func (p *Prober) dial(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, domain.ErrProbeFailed{Reason: "bad_address"}
	}
	allowlisted := p.cfg.Policy.HTTPAllowed(strings.ToLower(addr))
	ips, err := p.resolve(ctx, host)
	if err != nil {
		return nil, domain.ErrProbeFailed{Reason: "dns"}
	}
	if !allowlisted {
		for _, ip := range ips {
			if p.cfg.Blocked(ip) {
				return nil, domain.ErrSSRFBlocked("destination address is not allowed")
			}
		}
	}
	d := &net.Dialer{
		Timeout: DialTimeout,
		Control: func(_, address string, _ syscall.RawConn) error {
			ap, err := netip.ParseAddrPort(address)
			if err != nil || (!allowlisted && p.cfg.Blocked(ap.Addr())) {
				return domain.ErrSSRFBlocked("destination address is not allowed")
			}
			return nil
		},
	}
	var last error
	for _, ip := range ips {
		c, err := d.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return c, nil
		}
		last = err
	}
	if last == nil {
		last = errors.New("no address")
	}
	return nil, last
}

func (p *Prober) resolve(ctx context.Context, host string) ([]netip.Addr, error) {
	if ip, err := netip.ParseAddr(host); err == nil {
		return []netip.Addr{ip}, nil
	}
	ips, err := p.cfg.Resolver.LookupNetIP(ctx, "ip", host)
	if err != nil || len(ips) == 0 {
		return nil, errors.New("no addresses")
	}
	return ips, nil
}

// ListTools runs initialize, notifications/initialized and paged tools/list.
func (p *Prober) ListTools(ctx context.Context, t usecase.ProbeTarget) (usecase.ProbeResult, error) {
	if _, err := domain.ValidateExternalURL(t.URL, p.cfg.Policy); err != nil {
		return usecase.ProbeResult{}, err
	}
	// Total budget bounds the whole probe, not each request.
	ctx, cancel := context.WithTimeout(ctx, TotalTimeout)
	defer cancel()
	s := &session{p: p, url: t.URL, headers: t.Headers}
	defer s.close(context.WithoutCancel(ctx))

	initRes, err := s.call(ctx, 1, "initialize", map[string]any{
		"protocolVersion": p.cfg.ProtocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "orca-mcp-registry", "version": "1"},
	})
	if err != nil {
		return usecase.ProbeResult{}, err
	}
	var ir struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	_ = json.Unmarshal(initRes, &ir)
	s.version = ir.ProtocolVersion
	if err := s.notify(ctx, "notifications/initialized"); err != nil {
		return usecase.ProbeResult{}, err
	}
	var tools []domain.ToolInfo
	cursor := ""
	for page := 0; page < MaxPages; page++ {
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		raw, err := s.call(ctx, 2+page, "tools/list", params)
		if err != nil {
			return usecase.ProbeResult{}, err
		}
		var lr struct {
			Tools      []domain.ToolInfo `json:"tools"`
			NextCursor string            `json:"nextCursor"`
		}
		if err := json.Unmarshal(raw, &lr); err != nil {
			return usecase.ProbeResult{}, domain.ErrProbeFailed{Reason: "bad_response"}
		}
		tools = append(tools, lr.Tools...)
		if len(tools) > MaxTools {
			return usecase.ProbeResult{}, domain.ErrProbeFailed{Reason: "too_many_tools"}
		}
		if lr.NextCursor == "" {
			return usecase.ProbeResult{Tools: tools, ProtocolVersion: s.version}, nil
		}
		cursor = lr.NextCursor
	}
	return usecase.ProbeResult{}, domain.ErrProbeFailed{Reason: "too_many_pages"}
}

type session struct {
	p       *Prober
	url     string
	headers map[string]domain.SecretValue
	id      string
	version string
}

var forbiddenOutbound = map[string]bool{"host": true, "cookie": true, "content-length": true, "transfer-encoding": true, "connection": true, "upgrade": true}

func (s *session) request(ctx context.Context, method string, body []byte) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, s.url, bytes.NewReader(body))
	if err != nil {
		return nil, domain.ErrProbeFailed{Reason: "bad_request"}
	}
	req.Header.Set("Accept", "application/json, text/event-stream")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if s.id != "" {
		req.Header.Set("Mcp-Session-Id", s.id)
	}
	if s.version != "" {
		req.Header.Set("MCP-Protocol-Version", s.version)
	}
	// Only the configured header references are sent. The caller's own
	// credentials are never available here (no token passthrough).
	for name, v := range s.headers {
		l := strings.ToLower(name)
		if forbiddenOutbound[l] || strings.HasPrefix(l, "proxy-") {
			continue
		}
		req.Header.Set(name, string(v.Reveal()))
	}
	return req, nil
}

func (s *session) do(req *http.Request) (*http.Response, error) {
	resp, err := s.p.client.Do(req)
	if err != nil {
		if e := unwrapDomain(err); e != nil {
			return nil, e
		}
		switch {
		case errors.Is(err, errRedirect):
			return nil, domain.ErrProbeFailed{Reason: "redirect_refused"}
		case errors.Is(err, context.DeadlineExceeded):
			return nil, domain.ErrProbeFailed{Reason: "timeout"}
		}
		return nil, domain.ErrProbeFailed{Reason: "unreachable"}
	}
	return resp, nil
}

// unwrapDomain surfaces an SSRF block or classified failure raised inside the dialer.
func unwrapDomain(err error) error {
	var ae *apperrors.AppError
	if errors.As(err, &ae) && ae.Code == domain.CodeServerSSRFBlocked {
		return ae
	}
	var pf domain.ErrProbeFailed
	if errors.As(err, &pf) {
		return pf
	}
	return nil
}

func (s *session) call(ctx context.Context, id int, method string, params any) (json.RawMessage, error) {
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	req, err := s.request(ctx, http.MethodPost, body)
	if err != nil {
		return nil, err
	}
	resp, err := s.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, MaxBodyBytes))
		return nil, domain.ErrProbeFailed{Reason: fmt.Sprintf("http_%d", resp.StatusCode)}
	}
	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		s.id = sid
	}
	return readRPCResult(io.LimitReader(resp.Body, MaxBodyBytes+1), resp.Header.Get("Content-Type"), id)
}

func (s *session) notify(ctx context.Context, method string) error {
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": method})
	req, err := s.request(ctx, http.MethodPost, body)
	if err != nil {
		return err
	}
	resp, err := s.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, MaxBodyBytes))
	if resp.StatusCode/100 != 2 {
		return domain.ErrProbeFailed{Reason: fmt.Sprintf("http_%d", resp.StatusCode)}
	}
	return nil
}

// close ends the server-side session (best effort).
func (s *session) close(ctx context.Context) {
	if s.id == "" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := s.request(ctx, http.MethodDelete, nil)
	if err != nil {
		return
	}
	if resp, err := s.do(req); err == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		_ = resp.Body.Close()
	}
}

type rpcMessage struct {
	ID     json.RawMessage `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code int `json:"code"`
	} `json:"error"`
}

// readRPCResult accepts a plain JSON body or a bounded SSE stream and returns
// the result of the response whose id matches. The body reader is already
// capped at MaxBodyBytes+1, so an oversized body fails as bad_response.
func readRPCResult(r io.Reader, contentType string, id int) (json.RawMessage, error) {
	want := fmt.Sprint(id)
	match := func(data []byte) (json.RawMessage, bool, error) {
		var m rpcMessage
		if err := json.Unmarshal(data, &m); err != nil || m.ID == nil || strings.Trim(string(m.ID), `"`) != want {
			return nil, false, nil
		}
		if m.Error != nil {
			return nil, true, domain.ErrProbeFailed{Reason: "rpc_error"}
		}
		return m.Result, true, nil
	}
	if strings.HasPrefix(strings.ToLower(contentType), "text/event-stream") {
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 0, 64<<10), MaxBodyBytes+1)
		var data bytes.Buffer
		flush := func() (json.RawMessage, bool, error) {
			defer data.Reset()
			if data.Len() == 0 {
				return nil, false, nil
			}
			return match(data.Bytes())
		}
		for sc.Scan() {
			line := sc.Text()
			if line == "" {
				if res, ok, err := flush(); ok {
					return res, err
				}
				continue
			}
			if v, ok := strings.CutPrefix(line, "data:"); ok {
				data.WriteString(strings.TrimPrefix(v, " "))
			}
		}
		if res, ok, err := flush(); ok {
			return res, err
		}
		return nil, domain.ErrProbeFailed{Reason: "bad_response"}
	}
	b, err := io.ReadAll(r)
	if err != nil || len(b) > MaxBodyBytes {
		return nil, domain.ErrProbeFailed{Reason: "bad_response"}
	}
	res, ok, perr := match(b)
	if !ok {
		return nil, domain.ErrProbeFailed{Reason: "bad_response"}
	}
	return res, perr
}

// CheckHost implements usecase.EgressChecker: a spawn-time re-resolution that
// narrows the rebinding window for the agent host (residual risk R3).
func (p *Prober) CheckHost(ctx context.Context, host string) error {
	ctx, cancel := context.WithTimeout(ctx, DialTimeout)
	defer cancel()
	ips, err := p.resolve(ctx, host)
	if err != nil {
		return domain.ErrProbeFailed{Reason: "dns"}
	}
	for _, ip := range ips {
		if p.cfg.Blocked(ip) {
			return domain.ErrSSRFBlocked("destination address is not allowed")
		}
	}
	return nil
}
