package domain

// Outbox subjects of the solution flow; payloads never carry the options document.
const (
	SubjectSolutionProposed = "orca.request.solution.proposed"
	SubjectSolutionApproved = "orca.request.solution.approved"
)

// SubjectTypeForGate maps a registry gate to the Approval subject it opens.
func SubjectTypeForGate(g GateSubject) (SubjectType, bool) {
	switch g {
	case GateSolution:
		return SubjectSolution, true
	case GateFindings:
		return SubjectFindings, true
	case GateAnswer:
		return SubjectAnswer, true
	}
	return "", false
}
