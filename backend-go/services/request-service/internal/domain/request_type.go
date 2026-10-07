package domain

type RequestType string

const (
	RequestTypeChangeRequest RequestType = "change_request"
	RequestTypeBug           RequestType = "bug"
	RequestTypeHotfix        RequestType = "hotfix"
	RequestTypeTask          RequestType = "task"
	RequestTypeSpike         RequestType = "spike"
	RequestTypeQuestion      RequestType = "question"
	RequestTypeRefactor      RequestType = "refactor"
	RequestTypeSecurity      RequestType = "security"
	RequestTypePerformance   RequestType = "performance"
	RequestTypeDocs          RequestType = "docs"
	RequestTypeOpsRequest    RequestType = "ops_request"
)

func AllRequestTypes() []RequestType {
	return []RequestType{
		RequestTypeChangeRequest,
		RequestTypeBug,
		RequestTypeHotfix,
		RequestTypeTask,
		RequestTypeSpike,
		RequestTypeQuestion,
		RequestTypeRefactor,
		RequestTypeSecurity,
		RequestTypePerformance,
		RequestTypeDocs,
		RequestTypeOpsRequest,
	}
}

func ParseRequestType(s string) (RequestType, error) {
	for _, rt := range AllRequestTypes() {
		if string(rt) == s {
			return rt, nil
		}
	}
	return "", ErrRequestInvalidType(s)
}
