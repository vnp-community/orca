package usecase

import (
	"testing"
)

// RunSubjectHandlerContract is a contract test suite for any SubjectHandler
func RunSubjectHandlerContract(t *testing.T, factory func() SubjectHandler) {
	// Stub implementation as requested
	handler := factory()
	if handler == nil {
		t.Fatal("expected handler")
	}
}
