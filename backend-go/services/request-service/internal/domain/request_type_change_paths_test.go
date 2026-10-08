package domain

import "testing"

func TestChangeTypeAllowed(t *testing.T) {
	allowed := map[[2]RequestType]bool{
		{RequestTypeBug, RequestTypeChangeRequest}:         true,
		{RequestTypeTask, RequestTypeChangeRequest}:        true,
		{RequestTypeRefactor, RequestTypeChangeRequest}:    true,
		{RequestTypePerformance, RequestTypeChangeRequest}: true,
		{RequestTypeDocs, RequestTypeTask}:                 true,
		{RequestTypeDocs, RequestTypeChangeRequest}:        true,
		{RequestTypeSecurity, RequestTypeHotfix}:           true,
	}
	child := map[RequestType]bool{RequestTypeSpike: true, RequestTypeQuestion: true, RequestTypeHotfix: true}
	for _, from := range AllRequestTypes() {
		for _, to := range AllRequestTypes() {
			err := ChangeTypeAllowed(from, to)
			var want string
			switch {
			case from == to:
				want = "REQUEST_TYPE_UNCHANGED"
			case child[from]:
				want = "REQUEST_TYPE_CHANGE_USE_CHILD"
			case allowed[[2]RequestType{from, to}]:
				want = ""
			default:
				want = "REQUEST_TYPE_CHANGE_NOT_ALLOWED"
			}
			if want == "" && err != nil || want != "" && codeOf(err) != want {
				t.Errorf("%s -> %s: want %q, got %v", from, to, want, err)
			}
		}
	}
}
