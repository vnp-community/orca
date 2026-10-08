package usecase

import "github.com/stablyai/orca-go/services/request-service/internal/domain"

// NewFindingsApprovalHandler serves subject_type=findings. follow_ups stay prefill data: this handler never spawns a child
// or changes the request type; the registry's CompletesAfterAnalysis moves a spike to completed.
func NewFindingsApprovalHandler(d AnalysisApprovalHandlerDeps) SubjectHandler {
	return newAnalysisApprovalHandler(d, domain.SubjectFindings, false, domain.SolutionKindFindings)
}
