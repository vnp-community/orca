package usecase

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
)

// Hồi quy TASK-E2E-017: repo trả sentinel miền (bọc %w) -> KindNotFound;
// mọi lỗi hạ tầng khác -> KindInternal, không bị nuốt thành NotFound.

var errInfraDown = errors.New("connection refused")

func wrappedNotFound(sentinel error) error {
	return fmt.Errorf("postgres: ctx: %w", sentinel)
}

func assertAppError(t *testing.T, err error, kind apperrors.Kind, code string) {
	t.Helper()
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperrors.AppError, got %T (%v)", err, err)
	}
	if appErr.Kind != kind || appErr.Code != code {
		t.Errorf("got kind=%v code=%q, want kind=%v code=%q", appErr.Kind, appErr.Code, kind, code)
	}
}

type scriptedAutomationRepository struct {
	fakeAutomationRepository
	getErr, updateErr, deleteErr error
}

func (f *scriptedAutomationRepository) Get(ctx context.Context, tenantID, id string) (domain.Automation, error) {
	if f.getErr != nil {
		return domain.Automation{}, f.getErr
	}
	return f.fakeAutomationRepository.Get(ctx, tenantID, id)
}

func (f *scriptedAutomationRepository) Update(ctx context.Context, tenantID string, a domain.Automation) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	return f.fakeAutomationRepository.Update(ctx, tenantID, a)
}

func (f *scriptedAutomationRepository) Delete(ctx context.Context, tenantID, id string) error {
	return f.deleteErr
}

func newScriptedRepo(t *testing.T) *scriptedAutomationRepository {
	t.Helper()
	r := &scriptedAutomationRepository{fakeAutomationRepository: *newFakeAutomationRepository()}
	seedAutomation(t, &r.fakeAutomationRepository, "tenant-1", "auto-1", `{"step_type":"agent"}`)
	return r
}

func TestUpdateAutomation_RepoErrorMapping(t *testing.T) {
	name := "x"
	cases := []struct {
		name   string
		mutate func(r *scriptedAutomationRepository)
		kind   apperrors.Kind
		code   string
	}{
		{"get not found", func(r *scriptedAutomationRepository) { r.getErr = wrappedNotFound(domain.ErrAutomationNotFound) }, apperrors.KindNotFound, "AUTOMATION_NOT_FOUND"},
		{"get infra error", func(r *scriptedAutomationRepository) { r.getErr = errInfraDown }, apperrors.KindInternal, "AUTOMATION_LOOKUP_FAILED"},
		{"update not found", func(r *scriptedAutomationRepository) { r.updateErr = wrappedNotFound(domain.ErrAutomationNotFound) }, apperrors.KindNotFound, "AUTOMATION_NOT_FOUND"},
		{"update infra error", func(r *scriptedAutomationRepository) { r.updateErr = errInfraDown }, apperrors.KindInternal, "AUTOMATION_UPDATE_FAILED"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newScriptedRepo(t)
			tc.mutate(repo)
			_, err := NewUpdateAutomation(repo).Execute(context.Background(), UpdateAutomationInput{TenantID: "tenant-1", ID: "auto-1", Name: &name})
			assertAppError(t, err, tc.kind, tc.code)
		})
	}
}

func TestDeleteAutomation_RepoErrorMapping(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		repo := newScriptedRepo(t)
		repo.deleteErr = wrappedNotFound(domain.ErrAutomationNotFound)
		err := NewDeleteAutomation(repo).Execute(context.Background(), DeleteAutomationInput{TenantID: "tenant-1", ID: "auto-1"})
		assertAppError(t, err, apperrors.KindNotFound, "AUTOMATION_NOT_FOUND")
	})
	t.Run("infra error", func(t *testing.T) {
		repo := newScriptedRepo(t)
		repo.deleteErr = errInfraDown
		err := NewDeleteAutomation(repo).Execute(context.Background(), DeleteAutomationInput{TenantID: "tenant-1", ID: "auto-1"})
		assertAppError(t, err, apperrors.KindInternal, "AUTOMATION_DELETE_FAILED")
	})
}

type statusFailingRunRepository struct {
	fakeAutomationRunRepository
	updateStatusErr error
}

func (f *statusFailingRunRepository) UpdateStatus(ctx context.Context, run domain.AutomationRun) error {
	return f.updateStatusErr
}

func TestRunNow_RepoErrorMapping(t *testing.T) {
	ctx := withTenant(context.Background(), "tenant-1")
	in := RunNowInput{AutomationID: "auto-1", RequestID: "req-1"}
	exec := &fakeWorkflowStepExecutor{result: ExecuteAdHocStepOutput{Status: "completed"}}

	t.Run("automation not found", func(t *testing.T) {
		repo := newScriptedRepo(t)
		repo.getErr = wrappedNotFound(domain.ErrAutomationNotFound)
		_, err := NewRunNow(repo, newFakeAutomationRunRepository(), exec).Execute(ctx, in)
		assertAppError(t, err, apperrors.KindNotFound, "AUTOMATION_NOT_FOUND")
	})
	t.Run("automation lookup infra error is not NotFound", func(t *testing.T) {
		repo := newScriptedRepo(t)
		repo.getErr = errInfraDown
		_, err := NewRunNow(repo, newFakeAutomationRunRepository(), exec).Execute(ctx, in)
		assertAppError(t, err, apperrors.KindInternal, "AUTOMATION_LOOKUP_FAILED")
	})
	t.Run("run status not found", func(t *testing.T) {
		runs := &statusFailingRunRepository{fakeAutomationRunRepository: *newFakeAutomationRunRepository(), updateStatusErr: wrappedNotFound(domain.ErrAutomationRunNotFound)}
		_, err := NewRunNow(newScriptedRepo(t), runs, exec).Execute(ctx, in)
		assertAppError(t, err, apperrors.KindNotFound, "AUTOMATION_RUN_NOT_FOUND")
	})
	t.Run("run status infra error", func(t *testing.T) {
		runs := &statusFailingRunRepository{fakeAutomationRunRepository: *newFakeAutomationRunRepository(), updateStatusErr: errInfraDown}
		_, err := NewRunNow(newScriptedRepo(t), runs, exec).Execute(ctx, in)
		assertAppError(t, err, apperrors.KindInternal, "AUTOMATION_RUN_UPDATE_FAILED")
	})
}
