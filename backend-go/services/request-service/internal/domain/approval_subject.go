package domain

type SubjectType string

const (
	SubjectRequestType SubjectType = "request_type"
	SubjectSolution    SubjectType = "solution"
	SubjectFindings    SubjectType = "findings"
	SubjectAnswer      SubjectType = "answer"
	SubjectPlan        SubjectType = "plan"
	SubjectPhase       SubjectType = "phase"
	SubjectTaskList    SubjectType = "task_list"
	SubjectPreDeploy   SubjectType = "pre_deploy"
)

var AllSubjectTypes = []SubjectType{
	SubjectRequestType,
	SubjectSolution,
	SubjectFindings,
	SubjectAnswer,
	SubjectPlan,
	SubjectPhase,
	SubjectTaskList,
	SubjectPreDeploy,
}

func (s SubjectType) Valid() bool {
	for _, v := range AllSubjectTypes {
		if v == s {
			return true
		}
	}
	return false
}
