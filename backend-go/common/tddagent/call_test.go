package tddagent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type recorder struct {
	method, params string
	result         string
	err            error
}

func (r *recorder) relay(_ context.Context, method, paramsJSON string) (string, error) {
	r.method, r.params = method, paramsJSON
	return r.result, r.err
}

func TestCallSendsTheMcpCallToolShapeAndDecodesTheResult(t *testing.T) {
	r := &recorder{result: `{"ok":true,"isError":false,"result":{"run_id":"run-1","status":"running"}}`}
	var out struct {
		RunID  string `json:"run_id"`
		Status string `json:"status"`
	}
	err := Call(context.Background(), r.relay, "/srv/projects/repo", "run_get", map[string]any{"run_id": "run-1"}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if out.RunID != "run-1" || out.Status != "running" || r.method != "mcp.callTool" {
		t.Fatalf("out %+v method %q", out, r.method)
	}
	var p map[string]any
	if err := json.Unmarshal([]byte(r.params), &p); err != nil {
		t.Fatal(err)
	}
	if p["server"] != "tdd-agent" || p["projectDir"] != "/srv/projects/repo" || p["tool"] != "run_get" {
		t.Fatalf("params %v", p)
	}
	if _, has := p["timeoutMs"]; has {
		t.Fatalf("timeoutMs is sent only when asked for: %v", p)
	}
}

func TestCallWithoutArgumentsSendsAnEmptyObjectNotNull(t *testing.T) {
	r := &recorder{result: `{"ok":true,"result":{}}`}
	if err := Call(context.Background(), r.relay, "/d", "testspec_get", nil, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.params, `"arguments":{}`) {
		t.Fatalf("params %s", r.params)
	}
}

func TestCallCanBoundTheToolCall(t *testing.T) {
	r := &recorder{result: `{"ok":true,"result":{}}`}
	if err := Call(context.Background(), r.relay, "/d", "testspec_start", nil, nil, WithTimeoutMs(60000)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.params, `"timeoutMs":60000`) {
		t.Fatalf("params %s", r.params)
	}
}

func TestCallClassifiesFailures(t *testing.T) {
	cases := []struct {
		name   string
		r      *recorder
		assert func(error) bool
	}{
		{"tool error", &recorder{result: `{"ok":true,"isError":true,"error":{"code":"RUN_NOT_FOUND","message":"no"}}`},
			func(err error) bool { var te *ToolError; return errors.As(err, &te) && te.Code == "RUN_NOT_FOUND" }},
		{"tool error without a reason", &recorder{result: `{"ok":true,"isError":true}`},
			func(err error) bool { var te *ToolError; return errors.As(err, &te) && te.Code == "TDD_AGENT_TOOL_ERROR" }},
		{"bridge refusal", &recorder{result: `{"ok":false,"error":{"code":"MCP_TOOL_NOT_ALLOWED","message":"no"}}`},
			func(err error) bool {
				var be *BridgeError
				return errors.As(err, &be) && be.Code == "MCP_TOOL_NOT_ALLOWED" && !errors.Is(err, ErrTransport)
			}},
		{"bridge refusal without a reason", &recorder{result: `{"ok":false}`},
			func(err error) bool { var be *BridgeError; return errors.As(err, &be) && be.Code == "MCP_BRIDGE_FAILED" }},
		{"server crash is transient", &recorder{result: `{"ok":false,"error":{"code":"MCP_SERVER_FAILED","message":"exit"}}`},
			func(err error) bool { return errors.Is(err, ErrTransport) }},
		{"server timeout is transient", &recorder{result: `{"ok":false,"error":{"code":"MCP_TIMEOUT","message":"slow"}}`},
			func(err error) bool { return errors.Is(err, ErrTransport) }},
		{"relay down", &recorder{err: errors.New("agent offline")},
			func(err error) bool { return errors.Is(err, ErrTransport) && strings.Contains(err.Error(), "agent offline") }},
		{"old agent (not json)", &recorder{result: `not json`},
			func(err error) bool { return err != nil && !errors.Is(err, ErrTransport) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := Call(context.Background(), tc.r.relay, "/d", "run_get", nil, nil); !tc.assert(err) {
				t.Fatalf("unexpected error %v", err)
			}
		})
	}
}

func TestCallReportsAResultItCannotDecode(t *testing.T) {
	r := &recorder{result: `{"ok":true,"result":"a string"}`}
	var out struct{ RunID string }
	err := Call(context.Background(), r.relay, "/d", "run_get", nil, &out)
	if err == nil || errors.Is(err, ErrTransport) {
		t.Fatalf("want a decode error, got %v", err)
	}
}
