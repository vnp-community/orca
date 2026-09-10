package infrafleetclient

import (
	"context"

	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"
)

// fakeProjectClient/fakeInfraFleetPicker/fakeAIProviderClient are minimal
// test doubles for the three ports usecase.ServerResolver/
// usecase.ProviderResolver depend on — TASK-WF-002-03 widened every step
// executor's constructor to take real *usecase.ServerResolver/
// *usecase.ProviderResolver (concrete structs, not interfaces), so these
// tests build real resolvers backed by fakes rather than faking the
// resolver types themselves.
type fakeProjectClient struct {
	devServerID string
	err         error
}

func (f *fakeProjectClient) GetProject(ctx context.Context, id string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.devServerID, nil
}

type fakeInfraFleetPicker struct {
	connectionID string
	err          error
}

func (f *fakeInfraFleetPicker) PickByTag(ctx context.Context, tag string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.connectionID, nil
}

type fakeAIProviderClient struct {
	accountID     string
	err           error
	accountStatus string
	statusErr     error
}

func (f *fakeAIProviderClient) ResolveForProject(ctx context.Context, userID, projectID string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.accountID, nil
}

func (f *fakeAIProviderClient) GetAccountStatus(ctx context.Context, accountID string) (string, error) {
	if f.statusErr != nil {
		return "", f.statusErr
	}
	return f.accountStatus, nil
}

// newPassthroughServerResolver builds a real ServerResolver whose
// ProjectClient/InfraFleetPicker are never invoked — for tests that only
// exercise TargetKindServer's pure connection-id passthrough (the shape
// every executor test used before this task, with a literal ConnectionID
// like "conn-1" now spelled "server:conn-1").
func newPassthroughServerResolver() *usecase.ServerResolver {
	return usecase.NewServerResolver(&fakeProjectClient{}, &fakeInfraFleetPicker{})
}

// newNoopProviderResolver builds a real ProviderResolver whose
// AIProviderClient always resolves to an empty account id — for tests that
// don't care about provider resolution at all (Shell/Notification
// executors don't take a ProviderResolver; Agent executor tests that don't
// assert on Model/AccountID use this).
func newNoopProviderResolver() *usecase.ProviderResolver {
	return usecase.NewProviderResolver(&fakeAIProviderClient{})
}

// usecaseServerResolverWithProject builds a ServerResolver whose
// ProjectClient always resolves to devServerID — for TargetKindProject
// end-to-end tests.
func usecaseServerResolverWithProject(devServerID string) *usecase.ServerResolver {
	return usecase.NewServerResolver(&fakeProjectClient{devServerID: devServerID}, &fakeInfraFleetPicker{})
}

// usecaseServerResolverWithFleetTag builds a ServerResolver whose
// InfraFleetPicker always resolves to connectionID — for TargetKindFleetTag
// end-to-end tests.
func usecaseServerResolverWithFleetTag(connectionID string) *usecase.ServerResolver {
	return usecase.NewServerResolver(&fakeProjectClient{}, &fakeInfraFleetPicker{connectionID: connectionID})
}

// usecaseProviderResolverWithActiveAccount builds a ProviderResolver whose
// AIProviderClient reports any pinned account as active — for explicit-pin
// end-to-end tests (the pin's own AccountID/Model are what should reach
// relay params, not anything from GetAccountStatus).
func usecaseProviderResolverWithActiveAccount() *usecase.ProviderResolver {
	return usecase.NewProviderResolver(&fakeAIProviderClient{accountStatus: "active"})
}
