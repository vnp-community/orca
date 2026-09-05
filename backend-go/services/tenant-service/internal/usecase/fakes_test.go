package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
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

// assertAppError asserts err is an *apperrors.AppError with the given Kind —
// mirrors project-service/internal/usecase/fakes_test.go's helper of the
// same name.
func assertAppError(t *testing.T, err error, kind apperrors.Kind) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("expected an *apperrors.AppError, got %T: %v", err, err)
	}
	if ae.Kind != kind {
		t.Errorf("expected Kind=%v, got %v", kind, ae.Kind)
	}
}

type fakeCompanyRepository struct {
	byID      map[string]domain.Company
	createErr error
	getErr    error
	existsErr error
	updateErr error
	listErr   error
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

func (f *fakeCompanyRepository) List(ctx context.Context) ([]domain.Company, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	companies := make([]domain.Company, 0, len(f.byID))
	for _, c := range f.byID {
		companies = append(companies, c)
	}
	return companies, nil
}

func (f *fakeCompanyRepository) Exists(ctx context.Context, id string) (bool, error) {
	if f.existsErr != nil {
		return false, f.existsErr
	}
	_, ok := f.byID[id]
	return ok, nil
}

func (f *fakeCompanyRepository) Update(ctx context.Context, id string, patch domain.CompanySettingsPatch) (domain.Company, bool, error) {
	if f.updateErr != nil {
		return domain.Company{}, false, f.updateErr
	}
	c, ok := f.byID[id]
	if !ok {
		return domain.Company{}, false, nil
	}
	if patch.Name != "" {
		c.Name = patch.Name
	}
	if patch.SettingsJSON != "" {
		var settings domain.Settings
		if err := json.Unmarshal([]byte(patch.SettingsJSON), &settings); err != nil {
			return domain.Company{}, false, err
		}
		c.Settings = settings
	}
	f.byID[id] = c
	return c, true, nil
}

// fakeCompanyEmailDomainRepository is an in-memory CompanyEmailDomainRepository.
type fakeCompanyEmailDomainRepository struct {
	byDomain map[string]string // email_domain -> company_id

	resolveErr error
}

func newFakeCompanyEmailDomainRepository() *fakeCompanyEmailDomainRepository {
	return &fakeCompanyEmailDomainRepository{byDomain: map[string]string{}}
}

func (f *fakeCompanyEmailDomainRepository) Add(ctx context.Context, companyID, emailDomain string) error {
	f.byDomain[emailDomain] = companyID
	return nil
}

func (f *fakeCompanyEmailDomainRepository) Remove(ctx context.Context, emailDomain string) error {
	delete(f.byDomain, emailDomain)
	return nil
}

func (f *fakeCompanyEmailDomainRepository) ListForCompany(ctx context.Context, companyID string) ([]string, error) {
	var out []string
	for d, c := range f.byDomain {
		if c == companyID {
			out = append(out, d)
		}
	}
	return out, nil
}

func (f *fakeCompanyEmailDomainRepository) ResolveCompanyID(ctx context.Context, emailDomain string) (string, bool, error) {
	if f.resolveErr != nil {
		return "", false, f.resolveErr
	}
	companyID, ok := f.byDomain[emailDomain]
	return companyID, ok, nil
}

type departmentKey struct{ companyID, id string }

type fakeDepartmentRepository struct {
	byKey     map[departmentKey]domain.Department
	createErr error
	getErr    error
	listErr   error
	updateErr error
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

func (f *fakeDepartmentRepository) List(ctx context.Context, companyID string) ([]domain.Department, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []domain.Department
	for key, d := range f.byKey {
		if key.companyID == companyID {
			out = append(out, d)
		}
	}
	return out, nil
}

func (f *fakeDepartmentRepository) Update(ctx context.Context, companyID, id string, patch domain.DepartmentSettingsPatch) (domain.Department, bool, error) {
	if f.updateErr != nil {
		return domain.Department{}, false, f.updateErr
	}
	key := departmentKey{companyID, id}
	d, ok := f.byKey[key]
	if !ok {
		return domain.Department{}, false, nil
	}
	if patch.Name != "" {
		d.Name = patch.Name
	}
	if patch.SettingsJSON != "" {
		var settings domain.Settings
		if err := json.Unmarshal([]byte(patch.SettingsJSON), &settings); err != nil {
			return domain.Department{}, false, err
		}
		d.Settings = settings
	}
	f.byKey[key] = d
	return d, true, nil
}

type fakeUserProfileRepository struct {
	byUserID              map[string]domain.UserProfile
	onboardingStateByUser map[string]string
	upsertErr             error
	getErr                error
	listByDeptErr         error
	listByCompanyErr      error
	listByDeptCalls       int
	listByCompanyCalls    int
	getOnboardingStateErr error
	setOnboardingStateErr error
}

func newFakeUserProfileRepository() *fakeUserProfileRepository {
	return &fakeUserProfileRepository{
		byUserID:              map[string]domain.UserProfile{},
		onboardingStateByUser: map[string]string{},
	}
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

func (f *fakeUserProfileRepository) ListUserIDsByDepartment(ctx context.Context, companyID, departmentID string) ([]string, error) {
	f.listByDeptCalls++
	if f.listByDeptErr != nil {
		return nil, f.listByDeptErr
	}
	var out []string
	for _, p := range f.byUserID {
		if p.CompanyID == companyID && p.DepartmentID == departmentID {
			out = append(out, p.UserID)
		}
	}
	return out, nil
}

func (f *fakeUserProfileRepository) ListUserIDsByCompany(ctx context.Context, companyID string) ([]string, error) {
	f.listByCompanyCalls++
	if f.listByCompanyErr != nil {
		return nil, f.listByCompanyErr
	}
	var out []string
	for _, p := range f.byUserID {
		if p.CompanyID == companyID {
			out = append(out, p.UserID)
		}
	}
	return out, nil
}

func (f *fakeUserProfileRepository) GetOnboardingState(ctx context.Context, companyID, userID string) (string, bool, error) {
	if f.getOnboardingStateErr != nil {
		return "", false, f.getOnboardingStateErr
	}
	stateJSON, ok := f.onboardingStateByUser[userID]
	if !ok {
		return "", false, nil
	}
	return stateJSON, true, nil
}

func (f *fakeUserProfileRepository) SetOnboardingState(ctx context.Context, companyID, userID, stateJSON string) error {
	if f.setOnboardingStateErr != nil {
		return f.setOnboardingStateErr
	}
	f.onboardingStateByUser[userID] = stateJSON
	return nil
}

type teamKey struct{ companyID, id string }

type fakeTeamRepository struct {
	byKey       map[teamKey]domain.Team
	members     map[string][]domain.TeamMember // teamID -> members
	createErr   error
	getErr      error
	addErr      error
	listErr     error
	layersErr   error
	listByCoErr error
	removeErr   error
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

func (f *fakeTeamRepository) ListByCompany(ctx context.Context, companyID string) ([]domain.Team, error) {
	if f.listByCoErr != nil {
		return nil, f.listByCoErr
	}
	var out []domain.Team
	for key, t := range f.byKey {
		if key.companyID == companyID {
			out = append(out, t)
		}
	}
	return out, nil
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

func (f *fakeTeamRepository) RemoveMember(ctx context.Context, teamID, userID string) (bool, error) {
	if f.removeErr != nil {
		return false, f.removeErr
	}
	members := f.members[teamID]
	for i, m := range members {
		if m.UserID == userID {
			f.members[teamID] = append(members[:i], members[i+1:]...)
			return true, nil
		}
	}
	return false, nil
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
	byUserID        map[string]domain.ResolvedProfile
	getCalls        int
	setCalls        int
	invalidateCalls []string // userIDs, in call order
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
	f.invalidateCalls = append(f.invalidateCalls, userID)
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
