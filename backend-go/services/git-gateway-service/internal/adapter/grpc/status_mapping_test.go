package grpc

import (
	"testing"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

func TestToProtoGitStatus_CarriesEveryAgentField(t *testing.T) {
	added := 3
	status := domain.GitStatus{
		Branch:            "refs/heads/task/abc",
		ConflictOperation: "merge",
		Head:              "abc123",
		DidHitLimit:       true,
		StatusLength:      9001,
		Upstream:          &domain.StatusUpstream{HasUpstream: true, UpstreamName: "origin/x", Ahead: 2, Behind: 1},
		Files: []domain.FileStatus{
			{Path: "b.go", State: domain.FileStateRenamed, Area: "staged", OldPath: "a.go", Added: &added},
			{Path: "c.go", State: domain.FileStateModified, Area: "unstaged", ConflictKind: "both_modified", ConflictStatus: "unresolved"},
		},
	}

	resp := toProtoGitStatus(status)

	if resp.GetBranch() != "refs/heads/task/abc" || resp.GetConflictOperation() != "merge" || resp.GetHead() != "abc123" ||
		!resp.GetDidHitLimit() || resp.GetStatusLength() != 9001 {
		t.Errorf("response fields lost: %+v", resp)
	}
	if u := resp.GetUpstream(); u == nil || !u.GetHasUpstream() || u.GetUpstreamName() != "origin/x" || u.GetAhead() != 2 || u.GetBehind() != 1 {
		t.Errorf("upstream lost: %+v", u)
	}
	f := resp.GetFiles()[0]
	if f.GetArea() != "staged" || f.GetOldPath() != "a.go" || f.Added == nil || f.GetAdded() != 3 || f.Removed != nil {
		t.Errorf("file row lost data (removed must stay unset): %+v", f)
	}
	if c := resp.GetFiles()[1]; c.GetConflictKind() != "both_modified" || c.GetConflictStatus() != "unresolved" {
		t.Errorf("conflict data lost: %+v", c)
	}
}

func TestToProtoGitStatus_NoUpstreamStaysNil(t *testing.T) {
	if toProtoGitStatus(domain.GitStatus{Branch: "main"}).GetUpstream() != nil {
		t.Error("a host-local status has no upstream summary; it must stay unset, not hasUpstream=false")
	}
}
