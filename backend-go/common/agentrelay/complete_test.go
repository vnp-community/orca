package agentrelay

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCompleteSendsOnlyGivenFields(t *testing.T) {
	tests := []struct {
		name string
		p    CompleteParams
		want string
	}{
		{"prompt only", CompleteParams{Prompt: "hi"}, `{"prompt":"hi"}`},
		{"with account", CompleteParams{Prompt: "hi", AccountID: "a1"}, `{"prompt":"hi","accountId":"a1"}`},
		{"everything", CompleteParams{Prompt: "hi", Format: FormatJSON, TaskID: "t", Model: "m", MaxTokens: 512, AccountID: "a1", Timeout: time.Minute},
			`{"prompt":"hi","format":"json","taskId":"t","model":"m","maxTokens":512,"accountId":"a1"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fc := &fakeClient{respond: answer(`{"content":"ok"}`)}
			if _, err := New(fc).Complete(context.Background(), Target{ConnectionID: "c"}, tc.p); err != nil {
				t.Fatal(err)
			}
			if got := fc.snapshot()[0].paramsJSON; got != tc.want {
				t.Errorf("params = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestCompleteTimeout(t *testing.T) {
	t.Run("deadline is exactly the timeout", func(t *testing.T) {
		fc := &fakeClient{respond: answer(`{"content":"ok"}`)}
		if _, err := New(fc).Complete(context.Background(), Target{ConnectionID: "c"}, CompleteParams{Prompt: "p", Timeout: 2 * time.Minute}); err != nil {
			t.Fatal(err)
		}
		c := fc.snapshot()[0]
		if !c.hasDeadline || c.timeLeft > 2*time.Minute || c.timeLeft < 2*time.Minute-5*time.Second {
			t.Errorf("deadline = %v (%v)", c.hasDeadline, c.timeLeft)
		}
	})
	t.Run("no timeout leaves the context alone", func(t *testing.T) {
		fc := &fakeClient{respond: answer(`{"content":"ok"}`)}
		if _, err := New(fc).Complete(context.Background(), Target{ConnectionID: "c"}, CompleteParams{Prompt: "p"}); err != nil {
			t.Fatal(err)
		}
		if fc.snapshot()[0].hasDeadline {
			t.Error("unexpected deadline")
		}
	})
	t.Run("expiry is ErrTimeout", func(t *testing.T) {
		block := func(ctx context.Context) (string, error) { <-ctx.Done(); return "", ctx.Err() }
		_, err := New(&fakeClient{respond: block}).Complete(context.Background(), Target{ConnectionID: "c"}, CompleteParams{Prompt: "p", Timeout: 10 * time.Millisecond})
		if err != ErrTimeout {
			t.Errorf("err = %v, want ErrTimeout", err)
		}
	})
}

func TestCompleteResult(t *testing.T) {
	tests := []struct {
		name string
		body string
		want CompleteResult
	}{
		{"content only", `{"content":"hello"}`, CompleteResult{Content: "hello"}},
		{"null usage and unknown keys", `{"content":"hi","usage":null,"extra":1}`, CompleteResult{Content: "hi"}},
		{"full", `{"content":"c","model":"m","provider":"anthropic","latencyMs":12.7,"usage":{"promptTokens":5,"completionTokens":6,"totalTokens":11}}`,
			CompleteResult{Content: "c", Model: "m", Provider: "anthropic", LatencyMS: 12, Usage: &CompleteUsage{PromptTokens: 5, CompletionTokens: 6, TotalTokens: 11}}},
		{"negative latency is clamped", `{"content":"c","latencyMs":-4}`, CompleteResult{Content: "c"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := New(&fakeClient{respond: answer(tc.body)}).Complete(context.Background(), Target{ConnectionID: "c"}, CompleteParams{Prompt: "p"})
			if err != nil {
				t.Fatal(err)
			}
			if got.Content != tc.want.Content || got.Model != tc.want.Model || got.Provider != tc.want.Provider || got.LatencyMS != tc.want.LatencyMS {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
			if (got.Usage == nil) != (tc.want.Usage == nil) || (got.Usage != nil && *got.Usage != *tc.want.Usage) {
				t.Errorf("usage = %+v, want %+v", got.Usage, tc.want.Usage)
			}
		})
	}
	t.Run("malformed", func(t *testing.T) {
		_, err := New(&fakeClient{respond: answer("nope")}).Complete(context.Background(), Target{ConnectionID: "c"}, CompleteParams{Prompt: "p"})
		if !errors.Is(err, ErrMalformedResult) {
			t.Errorf("err = %v", err)
		}
	})
}

func TestCompleteErrorClassification(t *testing.T) {
	t.Run("typed code with detail", func(t *testing.T) {
		raw := status.Error(codes.FailedPrecondition, "AI_COMPLETE_QUOTA_EXHAUSTED: monthly limit reached")
		_, err := New(&fakeClient{respond: fail(raw)}).Complete(context.Background(), Target{ConnectionID: "c"}, CompleteParams{Prompt: "p"})
		var ce *CompleteError
		if !errors.As(err, &ce) || ce.Code != "AI_COMPLETE_QUOTA_EXHAUSTED" || ce.Detail != "monthly limit reached" {
			t.Fatalf("err = %v", err)
		}
		if !errors.Is(err, raw) {
			t.Error("cause is lost")
		}
	})
	t.Run("other failures are wrapped with the method", func(t *testing.T) {
		raw := status.Error(codes.Unavailable, "connection refused")
		_, err := New(&fakeClient{respond: fail(raw)}).Complete(context.Background(), Target{ConnectionID: "c"}, CompleteParams{Prompt: "p"})
		var ce *CompleteError
		if errors.As(err, &ce) || status.Code(err) != codes.Unavailable || !errors.Is(err, raw) {
			t.Errorf("err = %v", err)
		}
	})
}
