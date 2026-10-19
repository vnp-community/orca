package usecase

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
)

// TASK-E2E-017: an unknown account is NotFound for every account-id RPC; infra failures stay Internal.
func TestAccountUsecases_UnknownAccountIsNotFound(t *testing.T) {
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")
	missing := fmt.Errorf("repo: %w", domain.ErrAccountNotFound)

	calls := map[string]func(repo *fakeAccountRepository) error{
		"delete": func(r *fakeAccountRepository) error { return NewDeleteAccount(r).Execute(ctx, "x") },
		"update": func(r *fakeAccountRepository) error {
			_, err := NewUpdateAccount(r).Execute(ctx, UpdateFields{AccountID: "x"})
			return err
		},
		"rotate": func(r *fakeAccountRepository) error {
			_, err := NewRotateKey(r, &fakeCredentialBroker{}).Execute(ctx, RotateKeyInput{AccountID: "x"})
			return err
		},
		"write credential": func(r *fakeAccountRepository) error {
			_, err := NewWriteCredential(r, &fakeCredentialBroker{}).Execute(ctx, WriteCredentialForAccountInput{AccountID: "x"})
			return err
		},
		"test connection": func(r *fakeAccountRepository) error {
			_, err := NewTestConnection(r, &fakeInfraFleetClient{}, nil).Execute(ctx, TestConnectionInput{AccountID: "x"})
			return err
		},
		"record usage": func(r *fakeAccountRepository) error {
			_, err := NewRecordTokenUsage(r, &fakeUsageRepository{}, &fakeOutboxEnqueuer{}, nil).Execute(ctx, RecordTokenUsageInput{AccountID: "x"})
			return err
		},
		"resolve by id": func(r *fakeAccountRepository) error {
			_, err := NewResolveProvider(r).Resolve(ctx, ResolveProviderInput{AccountID: "x"})
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			repo := newFakeAccountRepository() // empty: Get returns ErrAccountNotFound
			repo.deleteErr, repo.updateErr = missing, missing
			assertAppError(t, call(repo), apperrors.KindNotFound, "AIPROVIDER_ACCOUNT_NOT_FOUND")
		})
	}
}

func TestAccountUsecases_InfrastructureFailureIsInternal(t *testing.T) {
	ctx := withIdentity(context.Background(), "tenant-1", "user-1")
	down := errors.New("connection refused")

	repo := newFakeAccountRepository()
	repo.deleteErr, repo.updateErr = down, down
	assertAppError(t, NewDeleteAccount(repo).Execute(ctx, "x"), apperrors.KindInternal, "AIPROVIDER_DELETE_FAILED")
	_, err := NewUpdateAccount(repo).Execute(ctx, UpdateFields{AccountID: "x"})
	assertAppError(t, err, apperrors.KindInternal, "AIPROVIDER_UPDATE_FAILED")

	// rotate used to turn any Get failure into NotFound, hiding outages.
	repo = newFakeAccountRepository()
	repo.getErr = down
	_, err = NewRotateKey(repo, &fakeCredentialBroker{}).Execute(ctx, RotateKeyInput{AccountID: "x"})
	assertAppError(t, err, apperrors.KindInternal, "AIPROVIDER_ACCOUNT_FETCH_FAILED")
}

func assertAppError(t *testing.T, err error, kind apperrors.Kind, code string) {
	t.Helper()
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("want AppError %s, got %v", code, err)
	}
	if appErr.Kind != kind || appErr.Code != code {
		t.Errorf("got kind=%v code=%s, want kind=%v code=%s", appErr.Kind, appErr.Code, kind, code)
	}
}
