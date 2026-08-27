# TASK-004: Update + extend `bootstrap_test.go` for the tenant-provisioning saga

**From Solution:** SOL-002
**Priority:** P0
**Service:** `auth-service`
**File:** `services/auth-service/internal/usecase/bootstrap_test.go`
**Depends on:** TASK-003
**Status:** `[ ]` TODO

---

## Context

TASK-003 changes `NewBootstrap`'s signature (adds a `TenantProvisioner`
param) and `BootstrapConfig`'s shape (`TenantID` → `CompanyName`). Every
existing test in `bootstrap_test.go` constructs both directly and needs a
mechanical update, plus this task adds the new saga-specific coverage
SOL-002 calls for.

## Changes to make

### Step 1 — add a fake `TenantProvisioner`, update every existing test

Add near the file's other fakes (`newFakeUserRepository`,
`fakeAuditRepository`, `fakeHasher`, `fakeClock`):

```go
type fakeTenantProvisioner struct {
	nextTenantID string // returned by CreateCompany; defaults to "generated-tenant-1" if unset
	createErr    error
	calledWith   []string // company names CreateCompany was called with, in order
}

func (f *fakeTenantProvisioner) CreateCompany(_ context.Context, name string) (string, error) {
	f.calledWith = append(f.calledWith, name)
	if f.createErr != nil {
		return "", f.createErr
	}
	if f.nextTenantID == "" {
		return "generated-tenant-1", nil
	}
	return f.nextTenantID, nil
}
```

Then, in every existing test (`TestBootstrap_CreatesAdmin_OnFreshDeployment`,
`TestBootstrap_NoOp_WhenUsersAlreadyExist`,
`TestBootstrap_NoOp_WhenConfigIncomplete`,
`TestBootstrap_UsesSuppliedPassword_WithoutReturningIt`, and any other
`NewBootstrap(...)`/`BootstrapConfig{...}` call sites in this file —
`grep -n 'NewBootstrap(\|BootstrapConfig{' bootstrap_test.go` to find every
one):

- Add `tenants := &fakeTenantProvisioner{}` and pass it as `NewBootstrap`'s
  5th argument.
- Replace `TenantID: "tenant-1"` (or whatever literal each test used) with
  nothing — remove the field entirely (it no longer exists on
  `BootstrapConfig`). Where a test asserted the created user's `TenantID`
  equals the literal it passed in, change that assertion to
  `tenants.nextTenantID` (or the fake's default, `"generated-tenant-1"`,
  if the test didn't set one) — the user's tenant now comes from the fake's
  return value, not a config literal.
- `TestBootstrap_NoOp_WhenConfigIncomplete` needs no `tenants` assertion
  change beyond the signature update — bootstrap should still no-op before
  ever calling `CreateCompany` when `Email` is empty; add
  `if len(tenants.calledWith) != 0 { t.Error(...) }` to that test to make
  the "step 1 never runs" guarantee explicit, not just implied by the
  existing empty-password assertion.

### Step 2 — new test: saga order (tenant before user)

```go
func TestBootstrap_ProvisionsTenantBeforeCreatingUser(t *testing.T) {
	users := newFakeUserRepository()
	audit := &fakeAuditRepository{}
	hasher := fakeHasher{}
	clock := &fakeClock{now: time.Now()}
	tenants := &fakeTenantProvisioner{nextTenantID: "tenant-xyz"}
	bootstrap := NewBootstrap(users, audit, hasher, clock, tenants)

	_, err := bootstrap.EnsureAdmin(context.Background(), BootstrapConfig{
		Email: "admin@example.com", Password: "",
	}, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tenants.calledWith) != 1 {
		t.Fatalf("expected CreateCompany called exactly once, got %d calls", len(tenants.calledWith))
	}
	user, _, err := users.GetUserByEmail(context.Background(), "admin@example.com")
	if err != nil {
		t.Fatalf("expected admin user to exist: %v", err)
	}
	if user.TenantID != "tenant-xyz" {
		t.Errorf("expected user.TenantID %q (from CreateCompany's return value), got %q", "tenant-xyz", user.TenantID)
	}
}
```

### Step 3 — new test: `CreateCompany` failing stops the saga before any user is created

```go
func TestBootstrap_CreateCompanyFailure_NeverCreatesUser(t *testing.T) {
	users := newFakeUserRepository()
	audit := &fakeAuditRepository{}
	hasher := fakeHasher{}
	clock := &fakeClock{now: time.Now()}
	tenants := &fakeTenantProvisioner{createErr: errors.New("tenant-service unreachable")}
	bootstrap := NewBootstrap(users, audit, hasher, clock, tenants)

	_, err := bootstrap.EnsureAdmin(context.Background(), BootstrapConfig{
		Email: "admin@example.com", Password: "",
	}, slog.Default())
	if err == nil {
		t.Fatal("expected an error when CreateCompany fails")
	}
	if _, _, err := users.GetUserByEmail(context.Background(), "admin@example.com"); err == nil {
		t.Error("expected no admin user to be created when tenant provisioning failed")
	}
}
```

Add `"errors"` to the file's imports if not already present.

### Step 4 — new test: `CreateUser` failing after a successful `CreateCompany` logs the orphan (doesn't panic/swallow the error)

```go
func TestBootstrap_CreateUserFailure_AfterTenantProvisioned_ReturnsOriginalError(t *testing.T) {
	users := newFakeUserRepository()
	users.createErr = errors.New("db write failed") // requires fakeUserRepository to support a settable createErr — add if it doesn't already
	audit := &fakeAuditRepository{}
	hasher := fakeHasher{}
	clock := &fakeClock{now: time.Now()}
	tenants := &fakeTenantProvisioner{nextTenantID: "tenant-orphaned"}
	bootstrap := NewBootstrap(users, audit, hasher, clock, tenants)

	_, err := bootstrap.EnsureAdmin(context.Background(), BootstrapConfig{
		Email: "admin@example.com", Password: "",
	}, slog.Default())
	if err == nil {
		t.Fatal("expected the CreateUser error to propagate")
	}
	// No compensating DeleteCompany call to assert — SOL-002 deliberately
	// doesn't add one (see that doc's "Design" section). This test only
	// proves the failure surfaces correctly and doesn't panic walking a
	// nonexistent compensation path.
}
```

Check `fakeUserRepository`'s current definition
(`grep -n 'type fakeUserRepository' -A 20 *_test.go` in this package) for
whether `CreateUser` already supports injecting an error; add a
`createErr error` field + check if it doesn't.

### Step 5 — new test: `defaultCompanyName` derivation

```go
func TestDefaultCompanyName(t *testing.T) {
	cases := []struct{ email, want string }{
		{"admin@acme.com", "acme.com"},
		{"admin@sub.acme.com", "sub.acme.com"},
		{"not-an-email", "Default Company"},
		{"", "Default Company"},
	}
	for _, c := range cases {
		if got := defaultCompanyName(c.email); got != c.want {
			t.Errorf("defaultCompanyName(%q) = %q, want %q", c.email, got, c.want)
		}
	}
}
```

## Verify

```bash
cd backend-go
go test ./services/auth-service/internal/usecase/... -count=1 -v -run 'TestBootstrap|TestDefaultCompanyName'
```

Expected: every updated existing test plus the 4 new tests pass.

Then, if a local Postgres + `tenant-service` instance is available (per
`10-deployment-infrastructure.md`'s `make dev-up` local stack), run the
integration check SOL-002's own Testing Plan calls for: a full
`EnsureAdmin` against real `tenant-service`, then confirm
`profile.getResolved` succeeds for the created admin — this is the actual
end-to-end regression BUG-002 describes, not just the usecase-level fakes
above.
