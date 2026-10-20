package grpcclient

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	"google.golang.org/grpc"
)

type fakeBranchCompare struct {
	gitgatewayv1.GitGatewayServiceClient
	resp *gitgatewayv1.BranchCompareResponse
	err  error
	got  *gitgatewayv1.BranchCompareRequest
}

func (f *fakeBranchCompare) BranchCompare(_ context.Context, in *gitgatewayv1.BranchCompareRequest, _ ...grpc.CallOption) (*gitgatewayv1.BranchCompareResponse, error) {
	f.got = in
	return f.resp, f.err
}

func TestGitReviewDiffReader_MapsEntriesAndAsksForTheWorktree(t *testing.T) {
	f := &fakeBranchCompare{resp: &gitgatewayv1.BranchCompareResponse{Status: "ready", BaseOid: "b", HeadOid: "h", CompareRef: "feat/x", MergeBase: "m",
		Entries: []*gitgatewayv1.GitChangeEntry{{Path: "a.go", Status: "modified", Added: 4, Removed: 1}, {Path: "n.go", OldPath: "o.go", Status: "renamed"}}}}
	r := NewGitReviewDiffReader(f)
	ctx := tenant.WithTenantID(context.Background(), "7b1f3c9e-0a53-4a9a-9f55-2d8d9f1d7a11")
	d, err := r.BranchCompare(ctx, "wt-1", "main")
	if err != nil {
		t.Fatal(err)
	}
	if f.got.GetWorktreeId() != "wt-1" || f.got.GetBaseRef() != "main" {
		t.Fatalf("asked %+v", f.got)
	}
	if d.Status != "ready" || d.HeadOID != "h" || len(d.Entries) != 2 || d.Entries[0].Added != 4 || d.Entries[1].OldPath != "o.go" {
		t.Fatalf("diff %+v", d)
	}
}

func TestGitReviewDiffReader_ErrorsAreErrors(t *testing.T) {
	ctx := tenant.WithTenantID(context.Background(), "7b1f3c9e-0a53-4a9a-9f55-2d8d9f1d7a11")
	if _, err := NewGitReviewDiffReader(&fakeBranchCompare{err: errors.New("down")}).BranchCompare(ctx, "w", "main"); err == nil {
		t.Fatalf("a failing call must fail")
	}
	bad := &fakeBranchCompare{resp: &gitgatewayv1.BranchCompareResponse{Status: "invalid-base", ErrorMessage: "no such ref"}}
	if _, err := NewGitReviewDiffReader(bad).BranchCompare(ctx, "w", "nope"); err == nil {
		t.Fatalf("a base git-gateway cannot resolve must be an error")
	}
	if _, err := NewGitReviewDiffReader(nil).BranchCompare(ctx, "w", "main"); err == nil {
		t.Fatalf("no client, no diff")
	}
	if _, err := NewGitReviewDiffReader(&fakeBranchCompare{}).BranchCompare(context.Background(), "w", "main"); err == nil {
		t.Fatalf("no tenant, no diff")
	}
}
