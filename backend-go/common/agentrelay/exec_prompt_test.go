package agentrelay

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const okResult = `{"stdout":"done","stderr":"","exitCode":0,"timedOut":false}`

func TestTargetSelection(t *testing.T) {
	tests := []struct {
		name         string
		target       Target
		viaDevServer bool
		wantID       string
	}{
		{"dev server only", Target{DevServerID: "ds-1"}, true, "ds-1"},
		{"connection only", Target{ConnectionID: "conn-1"}, false, "conn-1"},
		{"dev server wins when both are set", Target{DevServerID: "ds-1", ConnectionID: "conn-1"}, true, "ds-1"},
		{"empty target is sent as is", Target{}, false, ""},
	}
	for _, tc := range tests {
		for _, method := range []string{MethodExecPrompt, MethodComplete} {
			t.Run(tc.name+"/"+method, func(t *testing.T) {
				fc := &fakeClient{respond: answer(`{"content":"x","exitCode":0}`)}
				r := New(fc)
				var err error
				if method == MethodExecPrompt {
					_, err = r.ExecPrompt(context.Background(), tc.target, Plain(Run{Prompt: "p", WorktreePath: "/w"}))
				} else {
					_, err = r.Complete(context.Background(), tc.target, CompleteParams{Prompt: "p"})
				}
				if err != nil {
					t.Fatal(err)
				}
				calls := fc.snapshot()
				if len(calls) != 1 {
					t.Fatalf("calls = %d, want 1", len(calls))
				}
				c := calls[0]
				if c.viaDevServer != tc.viaDevServer || c.id != tc.wantID || c.method != method {
					t.Errorf("call = %+v, want viaDevServer=%v id=%q method=%q", c, tc.viaDevServer, tc.wantID, method)
				}
			})
		}
	}
}

func TestExecPromptSendsBuiltParams(t *testing.T) {
	fc := &fakeClient{respond: answer(okResult)}
	p := ReadOnly(Run{StepID: "s", Prompt: "hello", WorktreePath: "/repo", Env: map[string]string{"ORCA_REQUEST_ID": "r"}})
	if _, err := New(fc).ExecPrompt(context.Background(), Target{ConnectionID: "c"}, p); err != nil {
		t.Fatal(err)
	}
	var sent map[string]any
	if err := json.Unmarshal([]byte(fc.snapshot()[0].paramsJSON), &sent); err != nil {
		t.Fatal(err)
	}
	if sent["accessMode"] != "readonly" || sent["prompt"] != "hello" || sent["stepId"] != "s" {
		t.Errorf("sent = %v", sent)
	}
	if _, has := sent["trustPreset"]; has {
		t.Error("read-only run must not send a trust preset")
	}
}

func TestExecPromptRejectsInvalidParamsBeforeSending(t *testing.T) {
	tests := []struct {
		name string
		p    ExecParams
		want error
	}{
		{"read-only and write profile", ExecParams{Prompt: "p", WorktreePath: "/w", AccessMode: AccessReadOnly, PermissionProfile: "spec-author"}, ErrConflictingAccess},
		{"propose without root", ExecParams{Prompt: "p", WorktreePath: "/w", ProposeChangesets: true}, ErrProposeWithoutRoot},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fc := &fakeClient{respond: answer(okResult)}
			_, err := New(fc).ExecPrompt(context.Background(), Target{ConnectionID: "c"}, tc.p)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if n := len(fc.snapshot()); n != 0 {
				t.Errorf("client was called %d times", n)
			}
		})
	}
}

func TestExecPromptDeadline(t *testing.T) {
	tests := []struct {
		name         string
		opts         []Option
		timeoutMS    int
		wantDeadline bool
		wantAround   time.Duration
	}{
		{"default slack", nil, 60_000, true, 60*time.Second + DefaultTimeoutSlack},
		{"custom slack", []Option{WithTimeoutSlack(5 * time.Second)}, 60_000, true, 65 * time.Second},
		{"zero slack", []Option{WithTimeoutSlack(0)}, 60_000, true, 60 * time.Second},
		{"negative slack is zero", []Option{WithTimeoutSlack(-time.Hour)}, 60_000, true, 60 * time.Second},
		{"no timeout means no deadline", nil, 0, false, 0},
		{"deadline derivation disabled", []Option{WithoutCallDeadline()}, 60_000, false, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fc := &fakeClient{respond: answer(okResult)}
			p := Plain(Run{Prompt: "p", WorktreePath: "/w", TimeoutMS: tc.timeoutMS})
			if _, err := New(fc, tc.opts...).ExecPrompt(context.Background(), Target{ConnectionID: "c"}, p); err != nil {
				t.Fatal(err)
			}
			c := fc.snapshot()[0]
			if c.hasDeadline != tc.wantDeadline {
				t.Fatalf("hasDeadline = %v, want %v", c.hasDeadline, tc.wantDeadline)
			}
			if tc.wantDeadline && (c.timeLeft > tc.wantAround || c.timeLeft < tc.wantAround-5*time.Second) {
				t.Errorf("time left = %v, want about %v", c.timeLeft, tc.wantAround)
			}
		})
	}
}

func TestMetadataOption(t *testing.T) {
	attach := func(ctx context.Context) (context.Context, error) {
		return context.WithValue(ctx, metadataKey{}, "tenant-1"), nil
	}
	t.Run("attached to both RPCs", func(t *testing.T) {
		fc := &fakeClient{respond: answer(`{"content":"x","exitCode":0}`)}
		r := New(fc, WithMetadata(attach))
		if _, err := r.ExecPrompt(context.Background(), Target{ConnectionID: "c"}, Plain(Run{Prompt: "p", WorktreePath: "/w"})); err != nil {
			t.Fatal(err)
		}
		if _, err := r.Complete(context.Background(), Target{DevServerID: "d"}, CompleteParams{Prompt: "p"}); err != nil {
			t.Fatal(err)
		}
		for i, c := range fc.snapshot() {
			if c.metadata != "tenant-1" {
				t.Errorf("call %d metadata = %v", i, c.metadata)
			}
		}
	})
	t.Run("failure stops the call and keeps the cause", func(t *testing.T) {
		boom := errors.New("no tenant")
		fc := &fakeClient{respond: answer(okResult)}
		r := New(fc, WithMetadata(func(context.Context) (context.Context, error) { return nil, boom }))
		_, err := r.ExecPrompt(context.Background(), Target{ConnectionID: "c"}, Plain(Run{Prompt: "p", WorktreePath: "/w"}))
		if !errors.Is(err, boom) {
			t.Errorf("err = %v, want %v", err, boom)
		}
		_, err = r.Complete(context.Background(), Target{ConnectionID: "c"}, CompleteParams{Prompt: "p"})
		if !errors.Is(err, boom) {
			t.Errorf("complete err = %v, want %v", err, boom)
		}
		if n := len(fc.snapshot()); n != 0 {
			t.Errorf("client was called %d times", n)
		}
	})
	t.Run("nil function is ignored", func(t *testing.T) {
		fc := &fakeClient{respond: answer(okResult)}
		if _, err := New(fc, WithMetadata(nil)).ExecPrompt(context.Background(), Target{ConnectionID: "c"}, Plain(Run{Prompt: "p", WorktreePath: "/w"})); err != nil {
			t.Fatal(err)
		}
	})
}

func TestExecPromptRespectsCancellation(t *testing.T) {
	block := func(ctx context.Context) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	}
	t.Run("caller cancels", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		time.AfterFunc(20*time.Millisecond, cancel)
		_, err := New(&fakeClient{respond: block}).ExecPrompt(ctx, Target{ConnectionID: "c"}, Plain(Run{Prompt: "p", WorktreePath: "/w"}))
		if !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v, want Canceled", err)
		}
	})
	t.Run("derived deadline becomes ErrTimeout", func(t *testing.T) {
		r := New(&fakeClient{respond: block}, WithTimeoutSlack(0))
		_, err := r.ExecPrompt(context.Background(), Target{ConnectionID: "c"}, Plain(Run{Prompt: "p", WorktreePath: "/w", TimeoutMS: 1_000}))
		if err != ErrTimeout {
			t.Errorf("err = %v, want ErrTimeout", err)
		}
	})
}

func TestExecPromptMapsRelayErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		is   error
	}{
		{"read-only refused", status.Error(codes.FailedPrecondition, "agent.execPrompt: READONLY_MODE_UNSUPPORTED (FLAG_TOOLS_MISSING)"), ErrReadonlyUnsupported},
		{"knowledge hub refused", status.Error(codes.FailedPrecondition, "KNOWLEDGE_HUB_UNSUPPORTED"), ErrKnowledgeHubUnavailable},
		{"remote deadline", status.Error(codes.DeadlineExceeded, "slow"), ErrTimeout},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New(&fakeClient{respond: fail(tc.err)}).ExecPrompt(context.Background(), Target{ConnectionID: "c"}, Plain(Run{Prompt: "p", WorktreePath: "/w"}))
			if !errors.Is(err, tc.is) {
				t.Errorf("err = %v, want %v", err, tc.is)
			}
		})
	}
}

func TestExecPromptParsesResult(t *testing.T) {
	full := `{"stdout":"out","stderr":"err","exitCode":2,"timedOut":false,"stepId":"s","warnings":["READONLY_VIOLATION"],
	"applied":{"accessMode":"readonly","workspaceKind":"repo_root"},
	"changes":{"available":true,"headBefore":"a","headAfter":"b","headMoved":true,"changedFiles":[{"path":"x.go","change":"modified"}],"truncated":false},
	"parsed":{"ok":true,"value":{"verdict":"pass"}},"truncated":{"stdout":true,"stderr":false},
	"toolCalls":{"Read":3},"usage":{"turns":4,"durationMs":1500.5,"costUsd":0.25,"inputTokens":10,"outputTokens":20,"cacheReadTokens":30,"cacheCreationTokens":40}}`
	res, err := New(&fakeClient{respond: answer(full)}).ExecPrompt(context.Background(), Target{ConnectionID: "c"}, Plain(Run{Prompt: "p", WorktreePath: "/w"}))
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCodeOr(-1) != 2 || res.Stdout != "out" || res.StepID != "s" || len(res.Changes.ChangedFiles) != 1 || string(res.Parsed.Value) != `{"verdict":"pass"}` {
		t.Errorf("result = %+v", res)
	}
}

func TestMalformedResult(t *testing.T) {
	for _, body := range []string{"", "not json", `{"exitCode":"zero"}`, `[1,2]`} {
		_, err := New(&fakeClient{respond: answer(body)}).ExecPrompt(context.Background(), Target{ConnectionID: "c"}, Plain(Run{Prompt: "p", WorktreePath: "/w"}))
		if !errors.Is(err, ErrMalformedResult) {
			t.Errorf("body %q: err = %v, want ErrMalformedResult", body, err)
		}
	}
}

func TestRelayerIsRaceFree(t *testing.T) {
	fc := &fakeClient{respond: answer(`{"content":"x","stdout":"y","exitCode":0}`)}
	r := New(fc, WithMetadata(func(ctx context.Context) (context.Context, error) {
		return context.WithValue(ctx, metadataKey{}, "t"), nil
	}))
	var wg sync.WaitGroup
	for i := range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			target := Target{ConnectionID: "c"}
			if i%2 == 0 {
				target = Target{DevServerID: "d"}
			}
			if _, err := r.ExecPrompt(context.Background(), target, Plain(Run{Prompt: "p", WorktreePath: "/w", TimeoutMS: 1000})); err != nil {
				t.Error(err)
			}
			if _, err := r.Complete(context.Background(), target, CompleteParams{Prompt: "p", Timeout: time.Second}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if n := len(fc.snapshot()); n != 64 {
		t.Errorf("calls = %d, want 64", n)
	}
}
