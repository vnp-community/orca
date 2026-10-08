package usecase

import "github.com/stablyai/orca-go/services/request-service/internal/domain"

// NewAnswerApprovalHandler serves subject_type=answer: the reporter accepting the answer completes the request without a plan.
func NewAnswerApprovalHandler(d AnalysisApprovalHandlerDeps) SubjectHandler {
	return newAnalysisApprovalHandler(d, domain.SubjectAnswer, false, domain.SolutionKindAnswer)
}
