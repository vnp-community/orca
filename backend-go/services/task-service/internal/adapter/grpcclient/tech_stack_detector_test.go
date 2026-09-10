package grpcclient

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
)

// fakeGitGatewayServiceClient implements gitgatewayv1.GitGatewayServiceClient
// directly — same fake-the-generated-client-port convention as
// fakeInfraFleetServiceClient. respByPath lets a test script per-path
// ReadFile responses/errors; any path not present in either map panics via
// the embed, catching an unexpected probe.
type fakeGitGatewayServiceClient struct {
	gitgatewayv1.GitGatewayServiceClient

	respByPath map[string]*gitgatewayv1.ReadFileResponse
	errByPath  map[string]error
	gotPaths   []string
}

func (f *fakeGitGatewayServiceClient) ReadFile(ctx context.Context, in *gitgatewayv1.ReadFileRequest, _ ...grpc.CallOption) (*gitgatewayv1.ReadFileResponse, error) {
	f.gotPaths = append(f.gotPaths, in.GetPath())
	if err, ok := f.errByPath[in.GetPath()]; ok {
		return nil, err
	}
	if resp, ok := f.respByPath[in.GetPath()]; ok {
		return resp, nil
	}
	return nil, status.Error(codes.NotFound, "not found")
}

func TestTechStackDetector_NotFoundOnFirstCandidateContinuesToNext(t *testing.T) {
	git := &fakeGitGatewayServiceClient{
		errByPath: map[string]error{"package.json": status.Error(codes.NotFound, "not found")},
		respByPath: map[string]*gitgatewayv1.ReadFileResponse{
			"go.mod": {Content: []byte("module example.com/foo\n")},
		},
	}
	resolver := &fakeProjectExecutionResolver{connected: true, worktreeID: "wt-1"}
	d := NewTechStackDetector(git, resolver)

	stack, err := d.Detect(context.Background(), "tenant-1", "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, lang := range stack.Languages {
		if lang == "Go" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected Go to be detected, got %+v", stack)
	}
	// Every candidate is still probed (best-effort, not short-circuited).
	if len(git.gotPaths) != len(techStackCandidates) {
		t.Errorf("expected all %d candidates probed, got %d: %v", len(techStackCandidates), len(git.gotPaths), git.gotPaths)
	}
}

func TestTechStackDetector_PopulatesFrameworksFromPackageJSON(t *testing.T) {
	git := &fakeGitGatewayServiceClient{
		respByPath: map[string]*gitgatewayv1.ReadFileResponse{
			"package.json": {Content: []byte(`{"dependencies":{"react":"^18.0.0"}}`)},
		},
	}
	resolver := &fakeProjectExecutionResolver{connected: true, worktreeID: "wt-1"}
	d := NewTechStackDetector(git, resolver)

	stack, err := d.Detect(context.Background(), "tenant-1", "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stack.Languages) == 0 || stack.Languages[0] != "JavaScript/TypeScript" {
		t.Errorf("expected JavaScript/TypeScript language, got %+v", stack)
	}
	if len(stack.Frameworks) == 0 || stack.Frameworks[0] != "React" {
		t.Errorf("expected React framework, got %+v", stack)
	}
}

func TestTechStackDetector_NotConnected_ReturnsZeroValueNoError(t *testing.T) {
	git := &fakeGitGatewayServiceClient{}
	resolver := &fakeProjectExecutionResolver{connected: false}
	d := NewTechStackDetector(git, resolver)

	stack, err := d.Detect(context.Background(), "tenant-1", "p1")
	if err != nil {
		t.Fatalf("expected nil error even when not connected, got %v", err)
	}
	if len(stack.Languages) != 0 || len(stack.Frameworks) != 0 {
		t.Errorf("expected a zero-value TechStack, got %+v", stack)
	}
	if len(git.gotPaths) != 0 {
		t.Errorf("expected no ReadFile calls when not connected, got %v", git.gotPaths)
	}
}

func TestTechStackDetector_ResolveError_ReturnsZeroValueNoError(t *testing.T) {
	git := &fakeGitGatewayServiceClient{}
	resolver := &fakeProjectExecutionResolver{err: errors.New("resolve failed")}
	d := NewTechStackDetector(git, resolver)

	stack, err := d.Detect(context.Background(), "tenant-1", "p1")
	if err != nil {
		t.Fatalf("expected nil error (best-effort detection never fails), got %v", err)
	}
	if len(stack.Languages) != 0 {
		t.Errorf("expected empty TechStack, got %+v", stack)
	}
}
