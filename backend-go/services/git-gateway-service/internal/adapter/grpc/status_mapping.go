package grpc

import (
	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// toProtoGitStatus carries every field the executor reported, so api-gateway
// can rebuild the frontend's GitStatusResult without re-querying git.
func toProtoGitStatus(s domain.GitStatus) *gitgatewayv1.GetStatusResponse {
	resp := &gitgatewayv1.GetStatusResponse{
		Branch:            s.Branch,
		Files:             toProtoFileStatuses(s.Files),
		ConflictOperation: s.ConflictOperation,
		Head:              s.Head,
		DidHitLimit:       s.DidHitLimit,
		StatusLength:      int32(s.StatusLength),
	}
	if u := s.Upstream; u != nil {
		resp.Upstream = &gitgatewayv1.StatusUpstream{
			HasUpstream: u.HasUpstream, UpstreamName: u.UpstreamName, Ahead: int32(u.Ahead), Behind: int32(u.Behind),
		}
	}
	return resp
}

func toProtoFileStatuses(files []domain.FileStatus) []*gitgatewayv1.FileStatus {
	out := make([]*gitgatewayv1.FileStatus, 0, len(files))
	for _, f := range files {
		pf := &gitgatewayv1.FileStatus{
			Path: f.Path, State: string(f.State), Area: f.Area, OldPath: f.OldPath,
			ConflictKind: f.ConflictKind, ConflictStatus: f.ConflictStatus,
		}
		if f.Added != nil {
			added := int32(*f.Added)
			pf.Added = &added
		}
		if f.Removed != nil {
			removed := int32(*f.Removed)
			pf.Removed = &removed
		}
		out = append(out, pf)
	}
	return out
}
