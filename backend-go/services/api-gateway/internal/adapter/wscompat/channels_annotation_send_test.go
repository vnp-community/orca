package wscompat

import (
	"context"
	"errors"
	"strings"
	"testing"

	annotationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/annotation/v1"
	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// ── TASK-CR-03-02: formatReviewPrompt/formatFeedbackBlock ──────────────────

// TestFormatReviewPrompt_MatchesBLCR03Example is a golden-file-style
// regression guard against the exact prompt format
// docs/logic/code-review/BL-CR-03-gui-feedback-agent.md's worked example
// (lines 26-37) shows — structure identical, feedback text substituted —
// since this format is the contract the agent is expected to parse
// (BR-CR-09).
func TestFormatReviewPrompt_MatchesBLCR03Example(t *testing.T) {
	annotations := []*annotationv1.Annotation{
		{
			Anchor:  &annotationv1.Anchor{FilePath: "src/auth.ts", Line: 42, Side: annotationv1.Side_SIDE_NEW},
			Content: "Cần check null trước khi access user.role",
		},
		{
			Anchor:  &annotationv1.Anchor{FilePath: "src/api/routes.ts", Line: 128, Side: annotationv1.Side_SIDE_NEW},
			Content: "Thiếu authentication middleware",
		},
	}

	blocks := []string{
		formatFeedbackBlock(annotations[0], "if (user.role === 'admin') {", nil),
		formatFeedbackBlock(annotations[1], "app.get('/admin', adminHandler)", nil),
	}
	got := formatReviewPrompt("worktree-name", blocks)

	want := "Review feedback for worktree-name:\n\n" +
		"File: src/auth.ts, Line 42 (new)\n" +
		"Code: `if (user.role === 'admin') {`\n" +
		"Feedback: Cần check null trước khi access user.role\n\n" +
		"File: src/api/routes.ts, Line 128 (new)\n" +
		"Code: `app.get('/admin', adminHandler)`\n" +
		"Feedback: Thiếu authentication middleware"

	if got != want {
		t.Errorf("prompt format mismatch (BR-CR-09 regression):\ngot:\n%s\n\nwant:\n%s", got, want)
	}
}

func TestFormatFeedbackBlock_RangeLineAndSideOld(t *testing.T) {
	a := &annotationv1.Annotation{
		Anchor:  &annotationv1.Anchor{FilePath: "src/x.ts", Line: 10, EndLine: 12, Side: annotationv1.Side_SIDE_OLD},
		Content: "fix this",
	}
	got := formatFeedbackBlock(a, "old code", nil)
	if !strings.Contains(got, "Line 10-12 (old)") {
		t.Errorf("want range+old-side line descriptor, got: %s", got)
	}
}

func TestFormatFeedbackBlock_IncludesContextWhenPresent(t *testing.T) {
	a := &annotationv1.Annotation{
		Anchor:  &annotationv1.Anchor{FilePath: "src/x.ts", Line: 10, Side: annotationv1.Side_SIDE_NEW},
		Content: "feedback",
	}
	got := formatFeedbackBlock(a, "the line", []string{"before", "the line", "after"})
	if !strings.Contains(got, "Context:\nbefore\nthe line\nafter\n") {
		t.Errorf("want Context: block, got: %s", got)
	}
}

// ── TASK-CR-03-01: resolveCodeContext / sliceLinesAround ───────────────────

func TestSliceLinesAround_SingleLineWithContext(t *testing.T) {
	content := "l1\nl2\nl3\nl4\nl5"
	codeLine, ctxLines := sliceLinesAround(content, 3, 0, 2)
	if codeLine != "l3" {
		t.Errorf("want codeLine=l3, got %q", codeLine)
	}
	if strings.Join(ctxLines, ",") != "l1,l2,l3,l4,l5" {
		t.Errorf("want full ±2 context, got %v", ctxLines)
	}
}

func TestSliceLinesAround_RangeClampedAtBounds(t *testing.T) {
	content := "l1\nl2\nl3"
	codeLine, ctxLines := sliceLinesAround(content, 2, 3, 2)
	if codeLine != "l2\nl3" {
		t.Errorf("want codeLine=l2\\nl3, got %q", codeLine)
	}
	if strings.Join(ctxLines, ",") != "l1,l2,l3" {
		t.Errorf("want context clamped to file bounds, got %v", ctxLines)
	}
}

func TestNormalizeRelativePath_StripsLeadingSlashAndTraversal(t *testing.T) {
	cases := map[string]string{
		"/etc/passwd":      "etc/passwd",
		"../../etc/passwd": "../../etc/passwd", // filepath.Clean can't remove leading .. safely; TrimPrefix only strips leading separator
		"src/auth.ts":      "src/auth.ts",
		"./src/../auth.ts": "auth.ts",
	}
	for in, want := range cases {
		if got := normalizeRelativePath(in); got != want {
			t.Errorf("normalizeRelativePath(%q) = %q, want %q", in, got, want)
		}
	}
}

// ── TASK-CR-03-03: annotation.sendToAgent orchestration ────────────────────

// fakeSendStream is a minimal test double for
// infrafleetv1.InfraFleetService_AttachPtyClient — embeds the (nil)
// interface so it satisfies every method (mirrors this package's
// fakeInfraFleetClient/fakeGitGatewayClient convention) and overrides only
// Send, the one method terminalStreamEntry.send calls.
type fakeSendStream struct {
	infrafleetv1.InfraFleetService_AttachPtyClient

	sent [][]byte
	err  error
}

func (f *fakeSendStream) Send(frame *infrafleetv1.PtyClientFrame) error {
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, frame.GetInput().GetData())
	return nil
}

// newTestTerminalCtx wires a fake AttachPty stream directly into a
// terminalStreamRegistry attached to ctx, mirroring how
// registerTerminalCreateChannel populates it for real — good enough for
// SendReviewFeedbackToAgent's tests, which only ever call entry.send.
func newTestTerminalCtx(t *testing.T, ptyID string, fake *fakeSendStream) context.Context {
	t.Helper()
	streams := newTerminalStreamRegistry()
	streams.put(ptyID, &terminalStreamEntry{stream: fake})
	return terminalStreamsContext(context.Background(), streams)
}

func TestAnnotationSendToAgent_HappyPath(t *testing.T) {
	annotations := []*annotationv1.Annotation{
		{Id: "a1", Anchor: &annotationv1.Anchor{FilePath: "src/x.ts", Line: 1, Side: annotationv1.Side_SIDE_NEW}, Content: "fix 1"},
		{Id: "a2", Anchor: &annotationv1.Anchor{FilePath: "src/y.ts", Line: 2, Side: annotationv1.Side_SIDE_NEW}, Content: "fix 2"},
	}
	var markedIDs []string
	annClient := &fakeAnnotationClient{
		listAnnotationsFunc: func(ctx context.Context, in *annotationv1.ListAnnotationsRequest) (*annotationv1.ListAnnotationsResponse, error) {
			return &annotationv1.ListAnnotationsResponse{Annotations: annotations}, nil
		},
		markAnnotationsSentFunc: func(ctx context.Context, in *annotationv1.MarkAnnotationsSentRequest) (*annotationv1.MarkAnnotationsSentResponse, error) {
			markedIDs = in.GetIds()
			return &annotationv1.MarkAnnotationsSentResponse{Annotations: annotations}, nil
		},
	}
	gitClient := &fakeGitGatewayClient{
		readFileFunc: func(ctx context.Context, in *gitgatewayv1.ReadFileRequest) (*gitgatewayv1.ReadFileResponse, error) {
			return &gitgatewayv1.ReadFileResponse{Content: []byte("line1\nline2\nline3")}, nil
		},
	}

	fakeStream := &fakeSendStream{}
	ctx := newTestTerminalCtx(t, "pty-1", fakeStream)

	result, err := SendReviewFeedbackToAgent(ctx, annClient, gitClient, "wt-1", "pty-1", "my-worktree")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["sent"] != 2 {
		t.Errorf("want sent=2, got %v", result["sent"])
	}
	if len(fakeStream.sent) != 1 {
		t.Fatalf("want exactly 1 PTY send, got %d", len(fakeStream.sent))
	}
	prompt := string(fakeStream.sent[0])
	if !strings.Contains(prompt, "src/x.ts") || !strings.Contains(prompt, "src/y.ts") {
		t.Errorf("want both files in prompt, got: %s", prompt)
	}
	if len(markedIDs) != 2 || markedIDs[0] != "a1" || markedIDs[1] != "a2" {
		t.Errorf("want MarkAnnotationsSent called with [a1 a2], got %v", markedIDs)
	}
}

func TestAnnotationSendToAgent_EmptyListSendsNothing(t *testing.T) {
	annClient := &fakeAnnotationClient{
		listAnnotationsFunc: func(ctx context.Context, in *annotationv1.ListAnnotationsRequest) (*annotationv1.ListAnnotationsResponse, error) {
			return &annotationv1.ListAnnotationsResponse{}, nil
		},
	}
	fakeStream := &fakeSendStream{}
	ctx := newTestTerminalCtx(t, "pty-1", fakeStream)

	result, err := SendReviewFeedbackToAgent(ctx, annClient, &fakeGitGatewayClient{}, "wt-1", "pty-1", "wt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["sent"] != 0 {
		t.Errorf("want sent=0, got %v", result["sent"])
	}
	if len(fakeStream.sent) != 0 {
		t.Errorf("want zero PTY sends for empty annotation list, got %d", len(fakeStream.sent))
	}
}

func TestAnnotationSendToAgent_SideOldUsesOriginalCodeNeverReadsFile(t *testing.T) {
	annotations := []*annotationv1.Annotation{
		{Id: "a1", Anchor: &annotationv1.Anchor{FilePath: "src/x.ts", Line: 1, Side: annotationv1.Side_SIDE_OLD}, Content: "fix", OriginalCode: "old snapshot"},
	}
	annClient := &fakeAnnotationClient{
		listAnnotationsFunc: func(ctx context.Context, in *annotationv1.ListAnnotationsRequest) (*annotationv1.ListAnnotationsResponse, error) {
			return &annotationv1.ListAnnotationsResponse{Annotations: annotations}, nil
		},
		markAnnotationsSentFunc: func(ctx context.Context, in *annotationv1.MarkAnnotationsSentRequest) (*annotationv1.MarkAnnotationsSentResponse, error) {
			return &annotationv1.MarkAnnotationsSentResponse{}, nil
		},
	}
	readFileCalled := false
	gitClient := &fakeGitGatewayClient{
		readFileFunc: func(ctx context.Context, in *gitgatewayv1.ReadFileRequest) (*gitgatewayv1.ReadFileResponse, error) {
			readFileCalled = true
			return &gitgatewayv1.ReadFileResponse{Content: []byte("should not be used")}, nil
		},
	}
	fakeStream := &fakeSendStream{}
	ctx := newTestTerminalCtx(t, "pty-1", fakeStream)

	result, err := SendReviewFeedbackToAgent(ctx, annClient, gitClient, "wt-1", "pty-1", "wt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if readFileCalled {
		t.Error("side=SIDE_OLD must not call gitClient.ReadFile")
	}
	prompt := result["prompt"].(string)
	if !strings.Contains(prompt, "old snapshot") {
		t.Errorf("want OriginalCode used directly, got prompt: %s", prompt)
	}
}

func TestAnnotationSendToAgent_ReadFileFailureFallsBackButOtherBlocksUnaffected(t *testing.T) {
	annotations := []*annotationv1.Annotation{
		{Id: "a1", Anchor: &annotationv1.Anchor{FilePath: "src/missing.ts", Line: 1, Side: annotationv1.Side_SIDE_NEW}, Content: "fix 1", OriginalCode: "fallback code"},
		{Id: "a2", Anchor: &annotationv1.Anchor{FilePath: "src/ok.ts", Line: 1, Side: annotationv1.Side_SIDE_NEW}, Content: "fix 2"},
	}
	annClient := &fakeAnnotationClient{
		listAnnotationsFunc: func(ctx context.Context, in *annotationv1.ListAnnotationsRequest) (*annotationv1.ListAnnotationsResponse, error) {
			return &annotationv1.ListAnnotationsResponse{Annotations: annotations}, nil
		},
		markAnnotationsSentFunc: func(ctx context.Context, in *annotationv1.MarkAnnotationsSentRequest) (*annotationv1.MarkAnnotationsSentResponse, error) {
			return &annotationv1.MarkAnnotationsSentResponse{}, nil
		},
	}
	gitClient := &fakeGitGatewayClient{
		readFileFunc: func(ctx context.Context, in *gitgatewayv1.ReadFileRequest) (*gitgatewayv1.ReadFileResponse, error) {
			if in.GetPath() == "src/missing.ts" {
				return nil, errors.New("file not found")
			}
			return &gitgatewayv1.ReadFileResponse{Content: []byte("ok code")}, nil
		},
	}
	fakeStream := &fakeSendStream{}
	ctx := newTestTerminalCtx(t, "pty-1", fakeStream)

	result, err := SendReviewFeedbackToAgent(ctx, annClient, gitClient, "wt-1", "pty-1", "wt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	prompt := result["prompt"].(string)
	if !strings.Contains(prompt, "fallback code") {
		t.Errorf("want fallback to OriginalCode on ReadFile failure, got: %s", prompt)
	}
	if !strings.Contains(prompt, "src/ok.ts") {
		t.Errorf("want other block unaffected, got: %s", prompt)
	}
}

func TestAnnotationSendToAgent_MarkSentFailureStillReportsSentAndPrompt(t *testing.T) {
	annotations := []*annotationv1.Annotation{
		{Id: "a1", Anchor: &annotationv1.Anchor{FilePath: "src/x.ts", Line: 1, Side: annotationv1.Side_SIDE_NEW}, Content: "fix", OriginalCode: "x"},
	}
	annClient := &fakeAnnotationClient{
		listAnnotationsFunc: func(ctx context.Context, in *annotationv1.ListAnnotationsRequest) (*annotationv1.ListAnnotationsResponse, error) {
			return &annotationv1.ListAnnotationsResponse{Annotations: annotations}, nil
		},
		markAnnotationsSentFunc: func(ctx context.Context, in *annotationv1.MarkAnnotationsSentRequest) (*annotationv1.MarkAnnotationsSentResponse, error) {
			return nil, errors.New("annotation-service unavailable")
		},
	}
	gitClient := &fakeGitGatewayClient{
		readFileFunc: func(ctx context.Context, in *gitgatewayv1.ReadFileRequest) (*gitgatewayv1.ReadFileResponse, error) {
			return &gitgatewayv1.ReadFileResponse{Content: []byte("x")}, nil
		},
	}
	fakeStream := &fakeSendStream{}
	ctx := newTestTerminalCtx(t, "pty-1", fakeStream)

	result, err := SendReviewFeedbackToAgent(ctx, annClient, gitClient, "wt-1", "pty-1", "wt")
	if err != nil {
		t.Fatalf("want no client-visible error when only mark-sent fails (PTY delivery already succeeded), got: %v", err)
	}
	if result["sent"] != 1 {
		t.Errorf("want sent=1, got %v", result["sent"])
	}
	if result["prompt"] == nil || result["prompt"] == "" {
		t.Errorf("want prompt still present, got %v", result["prompt"])
	}
	if result["markSentError"] == nil {
		t.Errorf("want markSentError surfaced, got none")
	}
	if len(fakeStream.sent) != 1 {
		t.Errorf("want PTY delivery to have already happened, got %d sends", len(fakeStream.sent))
	}
}

func TestAnnotationSendToAgent_NormalizesPathBeforeReadFile(t *testing.T) {
	annotations := []*annotationv1.Annotation{
		{Id: "a1", Anchor: &annotationv1.Anchor{FilePath: "/../../etc/passwd", Line: 1, Side: annotationv1.Side_SIDE_NEW}, Content: "fix", OriginalCode: "fallback"},
	}
	annClient := &fakeAnnotationClient{
		listAnnotationsFunc: func(ctx context.Context, in *annotationv1.ListAnnotationsRequest) (*annotationv1.ListAnnotationsResponse, error) {
			return &annotationv1.ListAnnotationsResponse{Annotations: annotations}, nil
		},
		markAnnotationsSentFunc: func(ctx context.Context, in *annotationv1.MarkAnnotationsSentRequest) (*annotationv1.MarkAnnotationsSentResponse, error) {
			return &annotationv1.MarkAnnotationsSentResponse{}, nil
		},
	}
	var gotPath string
	gitClient := &fakeGitGatewayClient{
		readFileFunc: func(ctx context.Context, in *gitgatewayv1.ReadFileRequest) (*gitgatewayv1.ReadFileResponse, error) {
			gotPath = in.GetPath()
			return &gitgatewayv1.ReadFileResponse{Content: []byte("x")}, nil
		},
	}
	fakeStream := &fakeSendStream{}
	ctx := newTestTerminalCtx(t, "pty-1", fakeStream)

	if _, err := SendReviewFeedbackToAgent(ctx, annClient, gitClient, "wt-1", "pty-1", "wt"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.HasPrefix(gotPath, "/") {
		t.Errorf("want leading slash stripped before ReadFile, got %q", gotPath)
	}
}

func TestAnnotationSendToAgent_NoTerminalStreamRegistry(t *testing.T) {
	// side=SIDE_OLD so this doesn't also need a working gitClient.ReadFile —
	// the thing under test here is the missing terminalStreamRegistry.
	annotations := []*annotationv1.Annotation{
		{Id: "a1", Anchor: &annotationv1.Anchor{FilePath: "src/x.ts", Line: 1, Side: annotationv1.Side_SIDE_OLD}, Content: "fix", OriginalCode: "x"},
	}
	annClient := &fakeAnnotationClient{
		listAnnotationsFunc: func(ctx context.Context, in *annotationv1.ListAnnotationsRequest) (*annotationv1.ListAnnotationsResponse, error) {
			return &annotationv1.ListAnnotationsResponse{Annotations: annotations}, nil
		},
	}
	// No terminalStreamsContext wrapping — the REST-transport case.
	_, err := SendReviewFeedbackToAgent(context.Background(), annClient, &fakeGitGatewayClient{}, "wt-1", "pty-1", "wt")
	if !errors.Is(err, errNoTerminalStreamRegistry) {
		t.Errorf("want errNoTerminalStreamRegistry, got %v", err)
	}
}

// ── TASK-CR-03-04: annotation.sendToAgent wired via RegisterRealChannels ──

func TestAnnotationSendToAgentChannel_Registered(t *testing.T) {
	r := NewRegistry()
	registerAnnotationSendChannel(r, &fakeAnnotationClient{}, &fakeGitGatewayClient{})
	if _, ok := r.handlers["annotation.sendToAgent"]; !ok {
		t.Fatal("want annotation.sendToAgent registered")
	}
}
