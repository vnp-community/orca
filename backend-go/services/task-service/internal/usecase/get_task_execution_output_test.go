package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type denyPermission struct{}

func (denyPermission) Execute(context.Context, ResolvePermissionInput) (domain.GrantLevel, error) {
	return domain.GrantLevelUnspecified, errNoGrant(nil)
}

type allowPermission struct{ asked []ResolvePermissionInput }

func (a *allowPermission) Execute(_ context.Context, in ResolvePermissionInput) (domain.GrantLevel, error) {
	a.asked = append(a.asked, in)
	return domain.GrantLevelUser, nil
}

func repoWithOutput(output string) *fakeTaskRepository {
	repo := newFakeTaskRepository()
	task, _ := domain.NewTask("t1", "tenant-1", "Title", domain.StatusReview, "", "")
	task.LastExecutionOutput = output
	repo.tasks["t1"] = task
	return repo
}

func TestGetTaskExecutionOutput_RedactsSecretsAndNeedsReadAccess(t *testing.T) {
	perm := &allowPermission{}
	uc := NewGetTaskExecutionOutput(repoWithOutput("done\nAWS_KEY=AKIAIOSFODNN7EXAMPLE\nok"), perm)

	got, err := uc.Execute(withIdentity(context.Background(), "tenant-1", "user-1"), "t1")
	if err != nil || !got.Found {
		t.Fatalf("got %+v %v", got, err)
	}
	if strings.Contains(got.Output, "AKIAIOSFODNN7EXAMPLE") || !got.Redacted {
		t.Fatalf("the key must not leave task-service: %q redacted=%v", got.Output, got.Redacted)
	}
	if len(perm.asked) != 1 || perm.asked[0].Action != "read" || perm.asked[0].UserID != "user-1" || perm.asked[0].TaskID != "t1" {
		t.Fatalf("read access is checked for the caller: %+v", perm.asked)
	}
}

func TestGetTaskExecutionOutput_CallerWithoutAGrantIsRefused(t *testing.T) {
	uc := NewGetTaskExecutionOutput(repoWithOutput("secret plan"), denyPermission{})
	_, err := uc.Execute(withIdentity(context.Background(), "tenant-1", "user-1"), "t1")
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindPermissionDenied {
		t.Fatalf("got %v", err)
	}
}

func TestGetTaskExecutionOutput_NothingStoredIsNotFoundTrue(t *testing.T) {
	uc := NewGetTaskExecutionOutput(repoWithOutput(""), &allowPermission{})
	got, err := uc.Execute(withIdentity(context.Background(), "tenant-1", "user-1"), "t1")
	if err != nil || got.Found || got.Output != "" {
		t.Fatalf("got %+v %v", got, err)
	}
}

func TestGetTaskExecutionOutput_KeepsTheEndOfALongOutputOnACharacterBoundary(t *testing.T) {
	long := strings.Repeat("ế", 6000) + "KẾT QUẢ CUỐI"
	uc := NewGetTaskExecutionOutput(repoWithOutput(long), &allowPermission{})
	got, err := uc.Execute(withIdentity(context.Background(), "tenant-1", "user-1"), "t1")
	if err != nil || !got.Truncated || len(got.Output) > MaxExecutionOutputBytes || !utf8.ValidString(got.Output) || !strings.HasSuffix(got.Output, "KẾT QUẢ CUỐI") {
		t.Fatalf("got %d bytes truncated=%v valid=%v %v", len(got.Output), got.Truncated, utf8.ValidString(got.Output), err)
	}
}

func TestGetTaskExecutionOutput_ServiceCallerWithoutUserIsTenantScoped(t *testing.T) {
	uc := NewGetTaskExecutionOutput(repoWithOutput("x"), denyPermission{})
	got, err := uc.Execute(withIdentity(context.Background(), "tenant-1", ""), "t1")
	if err != nil || !got.Found {
		t.Fatalf("sibling services carry no user: %+v %v", got, err)
	}
	if _, err := uc.Execute(withIdentity(context.Background(), "tenant-1", ""), "missing"); err == nil {
		t.Fatal("a missing task is an error")
	}
	if _, err := uc.Execute(context.Background(), "t1"); err == nil {
		t.Fatal("a tenant is required")
	}
}
