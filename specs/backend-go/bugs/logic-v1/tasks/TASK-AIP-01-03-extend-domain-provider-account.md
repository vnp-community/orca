# TASK-AIP-01-03: Add registration-metadata fields to `domain.ProviderAccount`

**From Solution:** SOL-AIP-01
**Priority:** P1
**Service:** `ai-provider-service`
**File:** `backend-go/services/ai-provider-service/internal/domain/provider_account.go`
**Depends on:** TASK-AIP-01-01
**Status:** `[x] DONE — ProviderAccount + NewProviderAccount extended, ErrQuotaLimitTooLow added; TestNewProviderAccount_QuotaLimitTooLow passes.`

---

## Context

`domain.ProviderAccount` (`provider_account.go:142-155`) has no
`Label`/`ModelHint`/`BaseURL`/`QuotaLimitDay`/`Models`/`IsDefault`/
`LastHealthCheckAt`/`CreatedBy` fields, even though the Postgres columns
for the first three already exist (`0002_dev_server_id.up.sql`) and the
rest land in `TASK-AIP-01-01`'s migration. `NewProviderAccount`'s
signature must grow to accept them, and a new quota-floor validation must
be added.

## Changes to make

In `backend-go/services/ai-provider-service/internal/domain/provider_account.go`,
extend the struct (after `DevServerID`):

```go
type ProviderAccount struct {
	ID                 string
	TenantID           string
	ProviderType       ProviderType
	Status             AccountStatus
	CredentialRef      string
	Scope              AccountScope
	UserID             string
	ProjectID          string
	DevServerID        string
	Label              string     // NEW — "name" in BL-AIP-01's terms
	ModelHint          string     // NEW
	BaseURL            string     // NEW
	QuotaLimitDay      int        // NEW — 0 = unlimited
	Models             []string   // NEW — allowed-model allow-list (BUG-AIP-02 dependency)
	IsDefault          bool       // NEW
	LastHealthCheckAt  *time.Time // NEW — written by SOL-AIP-03's health-check job
	CreatedBy          string     // NEW
	RotationGraceUntil *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
```

Add the quota-floor error and validation:

```go
// ErrQuotaLimitTooLow — BL-AIP-01's field-level rule: quota_limit_day must
// be either 0 (unlimited) or >= 1000; anything in between is almost always
// a units mistake (per-request vs. per-day) worth catching at write time.
var ErrQuotaLimitTooLow = errors.New("domain: quota_limit_day must be 0 (unlimited) or >= 1000")
```

Extend `NewProviderAccount`'s signature to accept the new fields and
validate the quota floor:

```go
func NewProviderAccount(
	id, tenantID string,
	providerType ProviderType,
	status AccountStatus,
	credentialRef string,
	scope AccountScope,
	userID, projectID string,
	devServerID string,
	label, modelHint, baseURL string,
	quotaLimitDay int,
	models []string,
	isDefault bool,
	lastHealthCheckAt *time.Time,
	createdBy string,
	rotationGraceUntil *time.Time,
	createdAt, updatedAt time.Time,
) (ProviderAccount, error) {
	if tenantID == "" {
		return ProviderAccount{}, ErrEmptyTenantID
	}
	if !providerType.Valid() {
		return ProviderAccount{}, ErrInvalidProviderType
	}
	if !status.Valid() {
		return ProviderAccount{}, ErrInvalidStatus
	}
	if !scope.Valid() {
		return ProviderAccount{}, ErrInvalidScope
	}
	switch scope {
	case ScopeUser:
		if userID == "" || projectID != "" {
			return ProviderAccount{}, ErrInvalidScopeRef
		}
	case ScopeProject:
		if projectID == "" || userID != "" {
			return ProviderAccount{}, ErrInvalidScopeRef
		}
	case ScopeServer:
		if userID != "" || projectID != "" {
			return ProviderAccount{}, ErrInvalidScopeRef
		}
	}
	if quotaLimitDay != 0 && quotaLimitDay < 1000 {
		return ProviderAccount{}, ErrQuotaLimitTooLow
	}
	return ProviderAccount{
		ID: id, TenantID: tenantID, ProviderType: providerType, Status: status,
		CredentialRef: credentialRef, Scope: scope, UserID: userID, ProjectID: projectID,
		DevServerID: devServerID, Label: label, ModelHint: modelHint, BaseURL: baseURL,
		QuotaLimitDay: quotaLimitDay, Models: models, IsDefault: isDefault,
		LastHealthCheckAt: lastHealthCheckAt, CreatedBy: createdBy,
		RotationGraceUntil: rotationGraceUntil, CreatedAt: createdAt, UpdatedAt: updatedAt,
	}, nil
}
```

Note: this signature change breaks every existing caller of
`NewProviderAccount` (`create_account.go`, and any test helpers/fakes) —
those callers are fixed in `TASK-AIP-01-06` and are expected to fail to
compile until that task lands; do not add a second, parallel constructor
just to avoid the temporary breakage.

## Verify

```bash
cd /opt/repos/orca/backend-go
go vet ./services/ai-provider-service/internal/domain/...
go test ./services/ai-provider-service/internal/domain/... -run TestNewProviderAccount
```

Add `TestNewProviderAccount_QuotaLimitTooLow` (table-driven: 1, 500, 999 →
`ErrQuotaLimitTooLow`; 0, 1000, 50000 → no error) to
`provider_account_test.go`. Expect `go build
./services/ai-provider-service/...` to fail at this point — callers aren't
fixed until `TASK-AIP-01-06`; that's expected and resolved there.
