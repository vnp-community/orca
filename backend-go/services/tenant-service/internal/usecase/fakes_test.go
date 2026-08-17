package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// The fakes in this file back every *_test.go in this package — the
// "test against fakes, not a real database" pattern from
// specs/backend-go/standards/testing-strategy.md's unit-test section,
// mirroring usage-service's internal/usecase/record_usage_session_test.go.

func withTenant(ctx context.Context, companyID string) context.Context {
	return tenant.WithTenantID(ctx, companyID)
}

type fakeCompanyRepository struct {
	byID      map[string]domain.Company
	createErr error
	getErr    error
	existsErr error
}

func newFakeCompanyRepository() *fakeCompanyRepository {
	return &fakeCompanyRepository{byID: map[string]domain.Company{}}
}

func (f *fakeCompanyRepository) Create(ctx context.Context, c domain.Company) (domain.Company, error) {
	if f.createErr != nil {
		return domain.Company{}, f.createErr
	}
	f.byID[c.ID] = c
	return c, nil
}

func (f *fakeCompanyRepository) Get(ctx context.Context, id string) (domain.Company, bool, error) {
	if f.getErr != nil {
		return domain.Company{}, false, f.getErr
	}
	c, ok := f.byID[id]
	return c, ok, nil
}

func (f *fakeCompanyRepository) Exists(ctx context.Context, id string) (bool, error) {
	if f.existsErr != nil {
		return false, f.existsErr
	}
	_, ok := f.byID[id]
	return ok, nil
}

type departmentKey struct{ companyID, id string }

type fakeDepartmentRepository struct {
	byKey     map[departmentKey]domain.Department
	createErr error
	getErr    error
}

func newFakeDepartmentRepository() *fakeDepartmentRepository {
	return &fakeDepartmentRepository{byKey: map[departmentKey]domain.Department{}}
}

func (f *fakeDepartmentRepository) Create(ctx context.Context, d domain.Department) (domain.Department, error) {
	if f.createErr != nil {
		return domain.Department{}, f.createErr
	}
	f.byKey[departmentKey{d.CompanyID, d.ID}] = d
	return d, nil
}

func (f *fakeDepartmentRepository) Get(ctx context.Context, companyID, id string) (domain.Department, bool, error) {
	if f.getErr != nil {
		return domain.Department{}, false, f.getErr
	}
	d, ok := f.byKey[departmentKey{companyID, id}]
	return d, ok, nil
}

type fakeUserProfileRepository struct {
	byUserID  map[string]domain.UserProfile
	upsertErr error
	getErr    error
}

func newFakeUserProfileRepository() *fakeUserProfileRepository {
	return &fakeUserProfileRepository{byUserID: map[string]domain.UserProfile{}}
}

func (f *fakeUserProfileRepository) Upsert(ctx context.Context, p domain.UserProfile) error {
	if f.upsertErr != nil {
		return f.upsertErr
	}
	f.byUserID[p.UserID] = p
	return nil
}

func (f *fakeUserProfileRepository) Get(ctx context.Context, companyID, userID string) (domain.UserProfile, bool, error) {
	if f.getErr != nil {
		return domain.UserProfile{}, false, f.getErr
	}
	p, ok := f.byUserID[userID]
	if !ok || p.CompanyID != companyID {
		return domain.UserProfile{}, false, nil
	}
	return p, true, nil
}

type teamKey struct{ companyID, id string }

type fakeTeamRepository struct {
	byKey     map[teamKey]domain.Team
	members   map[string][]domain.TeamMember // teamID -> members
	createErr error
	getErr    error
	addErr    error
	listErr   error
	layersErr error
}

func newFakeTeamRepository() *fakeTeamRepository {
	return &fakeTeamRepository{
		byKey:   map[teamKey]domain.Team{},
		members: map[string][]domain.TeamMember{},
	}
}

func (f *fakeTeamRepository) Create(ctx context.Context, t domain.Team) (domain.Team, error) {
	if f.createErr != nil {
		return domain.Team{}, f.createErr
	}
	f.byKey[teamKey{t.CompanyID, t.ID}] = t
	return t, nil
}

func (f *fakeTeamRepository) Get(ctx context.Context, companyID, id string) (domain.Team, bool, error) {
	if f.getErr != nil {
		return domain.Team{}, false, f.getErr
	}
	t, ok := f.byKey[teamKey{companyID, id}]
	return t, ok, nil
}

func (f *fakeTeamRepository) AddMember(ctx context.Context, m domain.TeamMember) error {
	if f.addErr != nil {
		return f.addErr
	}
	members := f.members[m.TeamID]
	for i, existing := range members {
		if existing.UserID == m.UserID {
			members[i] = m
			f.members[m.TeamID] = members
			return nil
		}
	}
	f.members[m.TeamID] = append(members, m)
	return nil
}

func (f *fakeTeamRepository) ListMembers(ctx context.Context, teamID string) ([]domain.TeamMember, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.members[teamID], nil
}

func (f *fakeTeamRepository) ListUserTeamLayers(ctx context.Context, companyID, userID string) ([]domain.TeamSettingsLayer, error) {
	if f.layersErr != nil {
		return nil, f.layersErr
	}
	var layers []domain.TeamSettingsLayer
	for key, team := range f.byKey {
		if key.companyID != companyID {
			continue
		}
		for _, m := range f.members[team.ID] {
			if m.UserID == userID {
				layers = append(layers, domain.TeamSettingsLayer{
					TeamID:   team.ID,
					Priority: m.Priority,
					Settings: team.Settings,
				})
			}
		}
	}
	return layers, nil
}

type fakeProfileCache struct {
	byUserID map[string]domain.ResolvedProfile
	getCalls int
	setCalls int
}

func newFakeProfileCache() *fakeProfileCache {
	return &fakeProfileCache{byUserID: map[string]domain.ResolvedProfile{}}
}

func (f *fakeProfileCache) Get(ctx context.Context, userID string) (domain.ResolvedProfile, bool) {
	f.getCalls++
	p, ok := f.byUserID[userID]
	return p, ok
}

func (f *fakeProfileCache) Set(ctx context.Context, userID string, profile domain.ResolvedProfile, ttl time.Duration) {
	f.setCalls++
	f.byUserID[userID] = profile
}

func (f *fakeProfileCache) Invalidate(ctx context.Context, userID string) {
	delete(f.byUserID, userID)
}

type fakeCacheInvalidationPublisher struct {
	calls []string // userIDs, in call order
	err   error
}

func newFakeCacheInvalidationPublisher() *fakeCacheInvalidationPublisher {
	return &fakeCacheInvalidationPublisher{}
}

func (f *fakeCacheInvalidationPublisher) PublishProfileInvalidated(ctx context.Context, tenantID, userID string) error {
	f.calls = append(f.calls, userID)
	return f.err
}

var errFakeRepository = errors.New("fake repository error")
