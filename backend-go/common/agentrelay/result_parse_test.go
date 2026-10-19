package agentrelay

import (
	"errors"
	"strings"
	"testing"
)

func TestParseExecResultAgentGenerations(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		check func(t *testing.T, r ExecResult)
	}{
		{"oldest agent: stdout, stderr, exit code only", `{"stdout":"a","stderr":"b","exitCode":0,"timedOut":false}`, func(t *testing.T, r ExecResult) {
			if r.Stdout != "a" || r.Stderr != "b" || r.ExitCode == nil || *r.ExitCode != 0 {
				t.Errorf("%+v", r)
			}
			if r.Changes != nil || r.Parsed != nil || r.Truncated != nil || r.Applied != nil || r.Stats.Usage != nil || r.Stats.ToolCalls != nil || r.Warnings != nil {
				t.Errorf("optional fields must be empty: %+v", r)
			}
		}},
		{"null exit code", `{"stdout":"","stderr":"spawn failed","exitCode":null,"timedOut":true}`, func(t *testing.T, r ExecResult) {
			if r.ExitCode != nil || !r.TimedOut || r.ExitCodeOr(-1) != -1 {
				t.Errorf("%+v", r)
			}
		}},
		{"missing exit code", `{"stdout":"x"}`, func(t *testing.T, r ExecResult) {
			if r.ExitCode != nil {
				t.Errorf("exit code = %v", *r.ExitCode)
			}
		}},
		{"zero exit code is not nil", `{"exitCode":0}`, func(t *testing.T, r ExecResult) {
			if r.ExitCode == nil || r.ExitCodeOr(-1) != 0 {
				t.Errorf("%+v", r)
			}
		}},
		{"null optional objects", `{"stdout":"x","exitCode":0,"changes":null,"parsed":null,"usage":null,"toolCalls":null,"truncated":null,"applied":null,"warnings":null}`, func(t *testing.T, r ExecResult) {
			if r.Changes != nil || r.Parsed != nil || r.Stats.Usage != nil {
				t.Errorf("%+v", r)
			}
		}},
		{"changes unavailable", `{"exitCode":0,"changes":{"available":false,"reason":"NOT_A_GIT_REPO"}}`, func(t *testing.T, r ExecResult) {
			if r.Changes == nil || r.Changes.Available || r.Changes.Reason != "NOT_A_GIT_REPO" {
				t.Errorf("%+v", r.Changes)
			}
		}},
		{"head not reported stays nil", `{"exitCode":0,"changes":{"available":true,"headBefore":null,"headAfter":"abc","headMoved":true,"changedFiles":[]}}`, func(t *testing.T, r ExecResult) {
			if r.Changes.HeadBefore != nil || r.Changes.HeadAfter == nil || *r.Changes.HeadAfter != "abc" || !r.Changes.HeadMoved {
				t.Errorf("%+v", r.Changes)
			}
		}},
		{"truncated is an object", `{"exitCode":0,"truncated":{"stdout":true,"stderr":false}}`, func(t *testing.T, r ExecResult) {
			if r.Truncated == nil || !r.Truncated.Stdout || r.Truncated.Stderr {
				t.Errorf("%+v", r.Truncated)
			}
		}},
		{"parsed ok keeps raw value", `{"exitCode":0,"parsed":{"ok":true,"value":{"a":[1,2]}}}`, func(t *testing.T, r ExecResult) {
			if !r.Parsed.OK || string(r.Parsed.Value) != `{"a":[1,2]}` || r.Parsed.Code != "" {
				t.Errorf("%+v", r.Parsed)
			}
		}},
		{"parsed refused", `{"exitCode":0,"parsed":{"ok":false,"code":"RESULT_BLOCK_MISSING","detail":"no END marker found"}}`, func(t *testing.T, r ExecResult) {
			if r.Parsed.OK || r.Parsed.Value != nil || r.Parsed.Code != "RESULT_BLOCK_MISSING" || r.Parsed.Detail != "no END marker found" {
				t.Errorf("%+v", r.Parsed)
			}
		}},
		{"applied echo", `{"exitCode":0,"applied":{"accessMode":"readonly","workspaceKind":"repo_root"},"warnings":["READONLY_VIOLATION"]}`, func(t *testing.T, r ExecResult) {
			if r.Applied.AccessMode != "readonly" || r.Applied.WorkspaceKind != "repo_root" || len(r.Warnings) != 1 {
				t.Errorf("%+v", r)
			}
		}},
		{"usage without cost keeps cost nil", `{"exitCode":0,"usage":{"turns":3,"durationMs":2000.9,"inputTokens":1e3}}`, func(t *testing.T, r ExecResult) {
			u := r.Stats.Usage
			if u == nil || u.CostUSD != nil || u.Turns != 3 || u.DurationMS != 2000 || u.InputTokens != 1000 {
				t.Errorf("%+v", u)
			}
		}},
		{"usage with zero cost keeps zero", `{"exitCode":0,"usage":{"costUsd":0}}`, func(t *testing.T, r ExecResult) {
			if r.Stats.Usage.CostUSD == nil || *r.Stats.Usage.CostUSD != 0 {
				t.Errorf("%+v", r.Stats.Usage)
			}
		}},
		{"negative usage is clamped", `{"exitCode":0,"usage":{"turns":-5,"outputTokens":-1}}`, func(t *testing.T, r ExecResult) {
			if r.Stats.Usage.Turns != 0 || r.Stats.Usage.OutputTokens != 0 {
				t.Errorf("%+v", r.Stats.Usage)
			}
		}},
		{"unknown keys are ignored", `{"exitCode":0,"futureField":{"a":1}}`, func(t *testing.T, r ExecResult) {}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, err := parseExecResult(tc.body)
			if err != nil {
				t.Fatal(err)
			}
			tc.check(t, r)
		})
	}
}

func TestStatsAreBounded(t *testing.T) {
	var b strings.Builder
	b.WriteString(`{"exitCode":0,"toolCalls":{`)
	for i := range 300 {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`"tool` + strings.Repeat("x", 3) + string(rune('A'+i%26)) + string(rune('a'+i/26)) + `":1`)
	}
	b.WriteString(`,"zero":0,"neg":-2,"` + strings.Repeat("é", 80) + `":4}}`)
	r, err := parseExecResult(b.String())
	if err != nil {
		t.Fatal(err)
	}
	if n := len(r.Stats.ToolCalls); n > maxToolNames {
		t.Errorf("kept %d tool names, limit %d", n, maxToolNames)
	}
	for name, n := range r.Stats.ToolCalls {
		if len(name) > maxToolNameBytes || n <= 0 {
			t.Errorf("tool %q = %d", name, n)
		}
		if name == "zero" || name == "neg" {
			t.Errorf("non-positive count kept: %s", name)
		}
	}
}

func TestParseExecResultMalformed(t *testing.T) {
	for _, body := range []string{"", "   ", "{", `"str"`, `{"stdout":5}`, `{"exitCode":"x"}`} {
		if _, err := parseExecResult(body); !errors.Is(err, ErrMalformedResult) {
			t.Errorf("body %q: err = %v", body, err)
		}
	}
}

func TestTruncateUTF8(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		limit int
		want  string
	}{
		{"shorter", "abc", 10, "abc"},
		{"exact", "abc", 3, "abc"},
		{"cut ascii", "abcdef", 4, "abcd"},
		{"never splits a rune", "aéb", 2, "a"},
		{"cut after a whole rune", "aéb", 3, "aé"},
		{"four byte rune", "a😀", 4, "a"},
		{"zero limit", "abc", 0, ""},
		{"negative limit", "abc", -1, ""},
		{"empty", "", 5, ""},
	}
	for _, tc := range tests {
		if got := TruncateUTF8(tc.in, tc.limit); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}
