package mcpprober

import (
	"context"
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"
)

// CallTimeout bounds one whole tools/call or resources/read session. It is
// separate from TotalTimeout: a call may legitimately run longer than a probe.
const CallTimeout = 20 * time.Second

var _ usecase.ToolCaller = (*Prober)(nil)

func (p *Prober) callTimeout() time.Duration {
	if p.cfg.CallTimeout > 0 {
		return p.cfg.CallTimeout
	}
	return CallTimeout
}

// openSession validates the URL and runs initialize + notifications/initialized.
// The caller must defer s.close and cancel.
func (p *Prober) openSession(ctx context.Context, t usecase.CallTarget) (*session, context.Context, context.CancelFunc, error) {
	if _, err := domain.ValidateExternalURL(t.URL, p.cfg.Policy); err != nil {
		return nil, ctx, func() {}, err
	}
	ctx, cancel := context.WithTimeout(ctx, p.callTimeout())
	s := &session{p: p, url: t.URL, headers: t.Headers}
	initRes, err := s.call(ctx, 1, "initialize", map[string]any{
		"protocolVersion": p.cfg.ProtocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "orca-mcp-registry", "version": "1"},
	})
	if err != nil {
		s.close(context.WithoutCancel(ctx))
		return nil, ctx, cancel, err
	}
	var ir struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	_ = json.Unmarshal(initRes, &ir)
	s.version = ir.ProtocolVersion
	if err := s.notify(ctx, "notifications/initialized"); err != nil {
		s.close(context.WithoutCancel(ctx))
		return nil, ctx, cancel, err
	}
	return s, ctx, cancel, nil
}

type contentPart struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	MimeType string `json:"mimeType"`
	Resource *struct {
		Text     string `json:"text"`
		MimeType string `json:"mimeType"`
	} `json:"resource"`
}

// CallTool runs initialize, notifications/initialized, one tools/call and close.
func (p *Prober) CallTool(ctx context.Context, t usecase.CallTarget, tool string, argsJSON []byte, maxBytes int) (usecase.CallResult, error) {
	s, ctx, cancel, err := p.openSession(ctx, t)
	defer cancel()
	if err != nil {
		return usecase.CallResult{}, err
	}
	defer s.close(context.WithoutCancel(ctx))
	var args any = map[string]any{}
	if len(argsJSON) > 0 {
		if err := json.Unmarshal(argsJSON, &args); err != nil {
			return usecase.CallResult{}, domain.ErrInvalidArgument("arguments_json is not valid JSON")
		}
	}
	raw, err := s.call(ctx, 2, "tools/call", map[string]any{"name": tool, "arguments": args})
	if err != nil {
		return usecase.CallResult{}, err
	}
	var r struct {
		Content []contentPart `json:"content"`
		IsError bool          `json:"isError"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return usecase.CallResult{}, domain.ErrProbeFailed{Reason: "bad_response"}
	}
	var texts []string
	for _, c := range r.Content {
		switch {
		case c.Type == "text":
			texts = append(texts, c.Text)
		case c.Type == "resource" && c.Resource != nil && c.Resource.Text != "":
			texts = append(texts, c.Resource.Text)
		}
	}
	text, truncated := truncateAtRune(strings.Join(texts, "\n"), maxBytes)
	return usecase.CallResult{Text: text, IsError: r.IsError, Truncated: truncated, SizeBytes: len(text)}, nil
}

// ReadResource runs initialize, notifications/initialized, one resources/read and close.
func (p *Prober) ReadResource(ctx context.Context, t usecase.CallTarget, uri string, maxBytes int) (usecase.ResourceResult, error) {
	s, ctx, cancel, err := p.openSession(ctx, t)
	defer cancel()
	if err != nil {
		return usecase.ResourceResult{}, err
	}
	defer s.close(context.WithoutCancel(ctx))
	raw, err := s.call(ctx, 2, "resources/read", map[string]any{"uri": uri})
	if err != nil {
		return usecase.ResourceResult{}, err
	}
	var r struct {
		Contents []contentPart `json:"contents"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return usecase.ResourceResult{}, domain.ErrProbeFailed{Reason: "bad_response"}
	}
	var texts []string
	mime := ""
	for _, c := range r.Contents {
		if c.Text == "" {
			continue // blob or empty: not text
		}
		texts = append(texts, c.Text)
		if mime == "" {
			mime = c.MimeType
		}
	}
	text, truncated := truncateAtRune(strings.Join(texts, "\n"), maxBytes)
	return usecase.ResourceResult{Text: text, MimeType: mime, Truncated: truncated, SizeBytes: len(text)}, nil
}

// truncateAtRune cuts s to at most max bytes without splitting a UTF-8 rune.
func truncateAtRune(s string, max int) (string, bool) {
	if max <= 0 || len(s) <= max {
		return s, false
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut], true
}
