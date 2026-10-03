package usecase

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// actionOPA allows only the listed actions and records what was asked, so a
// test can tell "read" (preview) from "write" (save).
type actionOPA struct {
	mu      sync.Mutex
	allowed map[string]bool
	asked   []string
}

func (o *actionOPA) Decision(_ context.Context, _ domain.GrantLevel, action, _ string) (bool, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.asked = append(o.asked, action)
	return o.allowed[action], nil
}

func permissionedPromptUC(t *testing.T, allowed ...string) (*GenerateAgentPrompt, *fakeTaskRepository, *fakeAICompleter, *actionOPA) {
	t.Helper()
	tasks := newFakeTaskRepository()
	tasks.tasks["t1"] = domain.Task{ID: "t1", TenantID: "tenant-1", ProjectID: "p1", Title: "Build widget"}
	grants := &fakeGrantRepository{grants: []domain.Grant{{TaskID: "t1", SubjectID: "user-1", Level: domain.GrantLevelOwner}}}
	opa := &actionOPA{allowed: map[string]bool{}}
	for _, a := range allowed {
		opa.allowed[a] = true
	}
	completer := &fakeAICompleter{content: "generated"}
	perm := NewResolvePermission(tasks, grants, &fakeTeamScopeResolver{}, opa, nil)
	uc := NewGenerateAgentPrompt(tasks, &fakeAIProviderContextResolver{}, &fakeProjectExecutionResolver{connectionID: "conn-1", connected: true}, completer).WithPermissionCheck(perm)
	return uc, tasks, completer, opa
}

func TestGenerateAgentPrompt_Permission_PreviewNeedsRead(t *testing.T) {
	uc, _, completer, opa := permissionedPromptUC(t, "read")
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	if _, err := uc.Execute(ctx, GenerateAgentPromptInput{TaskID: "t1"}); err != nil {
		t.Fatalf("a reader may preview: %v", err)
	}
	if len(opa.asked) == 0 || opa.asked[0] != "read" || completer.gotConn == "" {
		t.Errorf("preview must ask for read and then run, asked=%v conn=%q", opa.asked, completer.gotConn)
	}
}

func TestGenerateAgentPrompt_Permission_SaveNeedsWrite(t *testing.T) {
	uc, tasks, completer, _ := permissionedPromptUC(t, "read") // read-only
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")

	_, err := uc.Execute(ctx, GenerateAgentPromptInput{TaskID: "t1", Save: true})
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindPermissionDenied {
		t.Fatalf("a read-only user must not save a prompt, got %v", err)
	}
	if completer.gotConn != "" {
		t.Error("the AI must not be called for a denied request")
	}
	if tasks.tasks["t1"].PromptTemplate != "" {
		t.Error("nothing may be persisted for a denied request")
	}

	uc, tasks, _, _ = permissionedPromptUC(t, "read", "write")
	if _, err := uc.Execute(ctx, GenerateAgentPromptInput{TaskID: "t1", Save: true}); err != nil {
		t.Fatalf("a writer may save: %v", err)
	}
	if tasks.tasks["t1"].PromptTemplate != "generated" {
		t.Errorf("want the prompt saved, got %q", tasks.tasks["t1"].PromptTemplate)
	}
}

func TestGenerateAgentPrompt_Permission_StrangerIsDeniedBeforeAnyLookup(t *testing.T) {
	uc, _, completer, _ := permissionedPromptUC(t, "read", "write")
	ctx := withIdentity(context.Background(), "tenant-1", "stranger")

	_, err := uc.Execute(ctx, GenerateAgentPromptInput{TaskID: "t1"})
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindPermissionDenied {
		t.Fatalf("a user with no grant must be denied, got %v", err)
	}
	if completer.gotConn != "" {
		t.Error("the AI must not be called for a user with no grant")
	}
}
