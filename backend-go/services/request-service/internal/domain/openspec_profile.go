package domain

type OpenSpecProfile string

const (
	OpenSpecProfileFull  OpenSpecProfile = "full"
	OpenSpecProfileLight OpenSpecProfile = "light"
	OpenSpecProfileNone  OpenSpecProfile = "none"
)

func OpenSpecProfileFor(t RequestType) OpenSpecProfile {
	switch t {
	case RequestTypeChangeRequest, RequestTypeRefactor:
		return OpenSpecProfileFull
	case RequestTypeBug, RequestTypeSecurity, RequestTypePerformance:
		return OpenSpecProfileLight
	case RequestTypeTask, RequestTypeDocs, RequestTypeOpsRequest, RequestTypeHotfix, RequestTypeSpike, RequestTypeQuestion:
		return OpenSpecProfileNone
	default:
		return OpenSpecProfileNone
	}
}
