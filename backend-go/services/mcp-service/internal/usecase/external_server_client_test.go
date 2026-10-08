package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

type fakeToolCaller struct {
	calls   int
	targets []CallTarget
	tool    string
	args    []byte
	max     int
	res     CallResult
	rres    ResourceResult
	err     error
	// header values seen at call time (the target is zeroed afterwards).
	seenHeaders []string
}

func (f *fakeToolCaller) CallTool(_ context.Context, t CallTarget, tool string, args []byte, max int) (CallResult, error) {
	f.calls++
	for _, v := range t.Headers {
		f.seenHeaders = append(f.seenHeaders, string(v.Reveal()))
	}
	f.targets, f.tool, f.args, f.max = append(f.targets, t), tool, args, max
	return f.res, f.err
}

func (f *fakeToolCaller) ReadResource(_ context.Context, t CallTarget, _ string, max int) (ResourceResult, error) {
	f.calls++
	f.targets, f.max = append(f.targets, t), max
	return f.rres, f.err
}

type clientEnv struct {
	regEnv
	caller *fakeToolCaller
	outbox *fakeOutbox
	cl     *ExternalServerClient
}

func newClientEnv() clientEnv {
	r := newReg(true, false)
	c := clientEnv{regEnv: r, caller: &fakeToolCaller{}, outbox: &fakeOutbox{}}
	c.cl = NewExternalServerClient(r.repo, r.broker, c.caller, c.outbox, extClock{time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)})
	return c
}

// approvedServer creates and approves a server whose only approved tool is "echo".
func (c clientEnv) approvedServer(t *testing.T) domain.ExternalServer {
	t.Helper()
	s := mustUpsert(t, c.regEnv, asAdmin(), httpSpec(domain.ScopeTenant, "srv"))
	return approve(t, c.regEnv, s.ID)
}

func TestExternalClient_RefusalTable(t *testing.T) {
	cases := []struct {
		name string
		prep func(t *testing.T, c clientEnv) (serverID, tool, args string, ctx context.Context)
		want string
	}{
		{"pending_review", func(t *testing.T, c clientEnv) (string, string, string, context.Context) {
			s := mustUpsert(t, c.regEnv, asAdmin(), httpSpec(domain.ScopeTenant, "p"))
			return s.ID, "echo", `{}`, asAdmin()
		}, domain.CodeServerNotUsable},
		{"disabled", func(t *testing.T, c clientEnv) (string, string, string, context.Context) {
			s := c.approvedServer(t)
			if _, err := c.uc.Review(asAdmin(), ReviewInput{ServerID: s.ID, Decision: domain.DecisionReject}); err != nil {
				t.Fatal(err)
			}
			return s.ID, "echo", `{}`, asAdmin()
		}, domain.CodeServerNotUsable},
		{"tools_changed", func(t *testing.T, c clientEnv) (string, string, string, context.Context) {
			s := c.approvedServer(t)
			c.prober.tools = []domain.ToolInfo{{Name: "echo", Description: "now exfiltrates"}}
			if _, err := c.uc.Probe(asAdmin(), s.ID); err != nil {
				t.Fatal(err)
			}
			return s.ID, "echo", `{}`, asAdmin()
		}, domain.CodeServerNotUsable},
		{"stdio", func(t *testing.T, c clientEnv) (string, string, string, context.Context) {
			s := c.approvedServer(t)
			s.Transport = domain.TransportStdio
			c.repo.put(s)
			return s.ID, "echo", `{}`, asAdmin()
		}, domain.CodeServerNotUsable},
		{"tool_not_approved", func(t *testing.T, c clientEnv) (string, string, string, context.Context) {
			return c.approvedServer(t).ID, "delete_everything", `{}`, asAdmin()
		}, domain.CodeToolNotApproved},
		{"egress_secret", func(t *testing.T, c clientEnv) (string, string, string, context.Context) {
			return c.approvedServer(t).ID, "echo", `{"q":"ghp_abcdefghijklmnopqrstuvwxyz0123456789"}`, asAdmin()
		}, domain.CodeInvalidArgument},
		{"bad_json", func(t *testing.T, c clientEnv) (string, string, string, context.Context) {
			return c.approvedServer(t).ID, "echo", `{not json`, asAdmin()
		}, domain.CodeInvalidArgument},
		{"args_too_big", func(t *testing.T, c clientEnv) (string, string, string, context.Context) {
			return c.approvedServer(t).ID, "echo", `{"q":"` + strings.Repeat("a", MaxExternalArgsBytes) + `"}`, asAdmin()
		}, domain.CodeInvalidArgument},
		{"server_id_not_uuid", func(t *testing.T, c clientEnv) (string, string, string, context.Context) {
			return "not-a-uuid", "echo", `{}`, asAdmin()
		}, domain.CodeNotFound},
		{"other_tenant", func(t *testing.T, c clientEnv) (string, string, string, context.Context) {
			s := c.approvedServer(t)
			return s.ID, "echo", `{}`, ctxAs("bbbbbbbb-0000-4000-8000-000000000002", adm, domain.RoleAdmin)
		}, domain.CodeNotFound},
		{"no_tenant", func(t *testing.T, c clientEnv) (string, string, string, context.Context) {
			return c.approvedServer(t).ID, "echo", `{}`, context.Background()
		}, domain.CodeNoTenant},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newClientEnv()
			id, tool, args, ctx := tc.prep(t, c)
			_, err := c.cl.CallTool(ctx, CallToolInput{ServerID: id, Tool: tool, ArgumentsJSON: []byte(args)})
			if got := code(err); got != tc.want {
				t.Fatalf("want %s, got %s (%v)", tc.want, got, err)
			}
			if c.caller.calls != 0 {
				t.Fatalf("external server was contacted despite refusal")
			}
		})
	}
}

func TestExternalClient_SuccessAuditsOnceWithoutContent(t *testing.T) {
	c := newClientEnv()
	s := c.approvedServer(t)
	c.caller.res = CallResult{Text: "SECRET-RESULT-BODY", IsError: false, SizeBytes: 18}
	out, err := c.cl.CallTool(asAdmin(), CallToolInput{ServerID: s.ID, Tool: "echo", ArgumentsJSON: []byte(`{"query":"SENSITIVE-ARG"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if out.Text != "SECRET-RESULT-BODY" || out.SizeBytes != 18 || len(out.Digest) != 64 {
		t.Fatalf("%+v", out)
	}
	if c.caller.max != DefaultExternalMaxBytes {
		t.Fatalf("max_bytes 0 must default, got %d", c.caller.max)
	}
	if len(c.outbox.records) != 1 {
		t.Fatalf("want exactly one audit event, got %d", len(c.outbox.records))
	}
	rec := c.outbox.records[0]
	payload := string(rec.PayloadJSON)
	for _, banned := range []string{"SENSITIVE-ARG", "SECRET-RESULT-BODY", "arguments"} {
		if strings.Contains(payload, banned) {
			t.Fatalf("audit leaks %q: %s", banned, payload)
		}
	}
	var p struct {
		Action, Outcome string
		Metadata        map[string]any
	}
	if err := json.Unmarshal(rec.PayloadJSON, &p); err != nil {
		t.Fatal(err)
	}
	if p.Action != domain.AuditActionExternalCall || p.Outcome != "allowed" || p.Metadata["tool"] != "echo" || p.Metadata["server_id"] != s.ID {
		t.Fatalf("%s", payload)
	}
}

func TestExternalClient_AuditHasNoArgumentsOrContentOnDenial(t *testing.T) {
	c := newClientEnv()
	s := c.approvedServer(t)
	_, err := c.cl.CallTool(asAdmin(), CallToolInput{ServerID: s.ID, Tool: "nope", ArgumentsJSON: []byte(`{"q":"SENSITIVE-ARG"}`)})
	if code(err) != domain.CodeToolNotApproved || len(c.outbox.records) != 1 {
		t.Fatalf("err=%v events=%d", err, len(c.outbox.records))
	}
	payload := string(c.outbox.records[0].PayloadJSON)
	if strings.Contains(payload, "SENSITIVE-ARG") || !strings.Contains(payload, `"outcome":"denied"`) || !strings.Contains(payload, "tool_not_approved") {
		t.Fatalf("%s", payload)
	}
}

func TestExternalClient_MaxBytesClampedAndResultRedacted(t *testing.T) {
	c := newClientEnv()
	s := c.approvedServer(t)
	c.caller.res = CallResult{Text: "token ghp_abcdefghijklmnopqrstuvwxyz0123456789 end", Truncated: true}
	out, err := c.cl.CallTool(asAdmin(), CallToolInput{ServerID: s.ID, Tool: "echo", MaxBytes: 10 << 20})
	if err != nil {
		t.Fatal(err)
	}
	if c.caller.max != MaxExternalMaxBytes {
		t.Fatalf("max must be clamped to 1MiB, got %d", c.caller.max)
	}
	if strings.Contains(out.Text, "ghp_") || !out.Truncated {
		t.Fatalf("%+v", out)
	}
	if _, err := c.cl.CallTool(asAdmin(), CallToolInput{ServerID: s.ID, Tool: "echo", MaxBytes: -1}); code(err) != domain.CodeInvalidArgument {
		t.Fatalf("negative max_bytes: %v", err)
	}
}

func TestExternalClient_ConfiguredHeadersAreForwardedFromBroker(t *testing.T) {
	c := newClientEnv()
	s := c.approvedServer(t)
	if err := c.uc.SetSecret(asAdmin(), SetSecretInput{ServerID: s.ID, Kind: "header", Name: "X-Api-Key", Value: domain.NewSecretValue("k-123")}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.cl.CallTool(asAdmin(), CallToolInput{ServerID: s.ID, Tool: "echo"}); err != nil {
		t.Fatal(err)
	}
	if len(c.caller.targets) != 1 || c.caller.targets[0].URL != s.URL {
		t.Fatalf("%+v", c.caller.targets)
	}
	if len(c.caller.seenHeaders) != 1 || c.caller.seenHeaders[0] != "k-123" {
		t.Fatalf("header from broker not forwarded: %v", c.caller.seenHeaders)
	}
	if got := string(c.caller.targets[0].Headers["X-Api-Key"].Reveal()); strings.Contains(got, "k-123") {
		t.Fatalf("header secret must be zeroed after the call, still %q", got)
	}
}

func TestExternalClient_UpstreamFailureIsClassifiedAndAudited(t *testing.T) {
	c := newClientEnv()
	s := c.approvedServer(t)
	c.caller.err = domain.ErrProbeFailed{Reason: "timeout"}
	_, err := c.cl.CallTool(asAdmin(), CallToolInput{ServerID: s.ID, Tool: "echo"})
	if err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("%v", err)
	}
	if len(c.outbox.records) != 1 || !strings.Contains(string(c.outbox.records[0].PayloadJSON), `"outcome":"error"`) {
		t.Fatalf("events=%d", len(c.outbox.records))
	}
	c.caller.err = errors.New("boom")
	if _, err := c.cl.CallTool(asAdmin(), CallToolInput{ServerID: s.ID, Tool: "echo"}); err == nil {
		t.Fatal("want error")
	}
}

func TestExternalClient_ReadResource(t *testing.T) {
	c := newClientEnv()
	s := c.approvedServer(t)
	c.caller.rres = ResourceResult{Text: "# readme", MimeType: "text/markdown"}
	const uri = "file:///docs/readme.md?v=1"
	out, err := c.cl.ReadResource(asAdmin(), ReadResourceInput{ServerID: s.ID, URI: uri})
	if err != nil || out.Text != "# readme" || out.MimeType != "text/markdown" || len(out.Digest) != 64 {
		t.Fatalf("%+v %v", out, err)
	}
	if len(c.outbox.records) != 1 {
		t.Fatalf("events=%d", len(c.outbox.records))
	}
	payload := string(c.outbox.records[0].PayloadJSON)
	if strings.Contains(payload, "readme.md") || !strings.Contains(payload, "uri_digest") || !strings.Contains(payload, domain.AuditActionExternalRead) {
		t.Fatalf("raw uri must not be audited: %s", payload)
	}
	for name, bad := range map[string]string{
		"empty": "", "no_scheme": "docs/readme.md", "control": "file:///a\nb", "creds": "https://u:p@host/x",
		"token": "https://host/x?t=ghp_abcdefghijklmnopqrstuvwxyz0123456789", "long": "file:///" + strings.Repeat("a", MaxExternalURIBytes),
	} {
		before := c.caller.calls
		if _, err := c.cl.ReadResource(asAdmin(), ReadResourceInput{ServerID: s.ID, URI: bad}); code(err) != domain.CodeInvalidArgument {
			t.Errorf("%s: want invalid argument, got %v", name, err)
		}
		if c.caller.calls != before {
			t.Errorf("%s: server contacted", name)
		}
	}
	pend := mustUpsert(t, c.regEnv, asAdmin(), httpSpec(domain.ScopeTenant, "pend"))
	if _, err := c.cl.ReadResource(asAdmin(), ReadResourceInput{ServerID: pend.ID, URI: uri}); code(err) != domain.CodeServerNotUsable {
		t.Fatalf("pending server: %v", err)
	}
}

func TestExternalClient_NilCallerIsUnavailable(t *testing.T) {
	r := newReg(false, false)
	cl := NewExternalServerClient(r.repo, r.broker, nil, nil, extClock{time.Now()})
	_, err := cl.CallTool(asAdmin(), CallToolInput{ServerID: "11111111-0000-4000-8000-0000000000aa", Tool: "x"})
	if code(err) != domain.CodeUnavailable {
		t.Fatalf("%v", err)
	}
}
