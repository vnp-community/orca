# TASK-PRF-01-08: Wire OPA evaluator + audit publisher into `cmd/server/main.go`, close out test coverage

**From Solution:** SOL-PRF-01
**Priority:** P1 — last task in this set; the package doesn't build end-to-end until this lands
**Service:** `tenant-service`
**File:** `backend-go/services/tenant-service/cmd/server/main.go`
**Depends on:** TASK-PRF-01-03, TASK-PRF-01-04, TASK-PRF-01-05, TASK-PRF-01-06, TASK-PRF-01-07
**Status:** `[ ]` TODO

---

## Context

TASK-PRF-01-05/06 changed `NewUpdateCompany`/`NewUpdateDepartment`/
`NewCreateDepartment`'s constructor signatures to take `opa OPAClient` and
`audit AuditPublisher`. Nothing constructs an OPA evaluator or wires the
audit publisher in `main.go` yet — this task does both, following
`project-service/cmd/server/main.go`'s existing `opa :=
projectopaclient.New(policy.NewEvaluator(cfg.OPABundlePath))` pattern
exactly, and reuses the already-constructed `pub *commoneventbus.Publisher`
this file's NATS block builds for `PublishProfileInvalidated`.

## Changes to make

Add `OPABundlePath` to `backend-go/services/tenant-service/internal/config/config.go`
(tenant-service's `Config` has no such field today, unlike project-service's):

```go
type Config struct {
	commonconfig.Base
	NATSURL string
	// OPABundlePath points requireCompanyAdmin/requireDepartmentAccess's OPA
	// client at policy/orca-authz — same convention as project-service's own
	// OPABundlePath, for identical override behavior in every service.
	OPABundlePath string
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("tenant-service")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Base:          base,
		NATSURL:       commonconfig.StringEnv("NATS_URL", "nats://localhost:4222"),
		OPABundlePath: commonconfig.StringEnv("OPA_BUNDLE_PATH", "../../policy/orca-authz"),
	}, nil
}
```

In `backend-go/services/tenant-service/cmd/server/main.go`:

1. Add imports:
```go
"github.com/stablyai/orca-go/common/policy"
tenantopaclient "github.com/stablyai/orca-go/services/tenant-service/internal/adapter/opaclient"
```

2. Construct the OPA client (near the top of `run()`, alongside `companies :=
   tenantpostgres.NewCompanyRepository(pool)` etc.):
```go
opa := tenantopaclient.New(policy.NewEvaluator(cfg.OPABundlePath))
```

3. Build `auditPublisher usecase.AuditPublisher` from the already-constructed
   `pub` inside the existing `if err != nil { ... } else { ... }` NATS block
   (the block that currently only sets `invalidationPublisher`):
```go
} else {
	defer func() { _ = closeBus() }()
	if err := pub.EnsureStream(ctx, tenanteventbus.StreamName, []string{"orca.tenant.>"}); err != nil {
		logger.WarnContext(ctx, "failed to ensure jetstream stream", slog.Any("error", err))
	} else {
		invalidationPublisher = tenanteventbus.New(pub)
		auditPublisher = tenanteventbus.New(pub) // NEW — same Publisher implements both ports
		healthSrv.Register("nats", func() error { return nil })

		invalidationConsumer := tenanteventbus.NewConsumer(cons, profileCache)
		consumerWG.Add(1)
		go func() {
			defer consumerWG.Done()
			invalidationConsumer.Run(ctx, logger)
		}()
	}
}
```
Declare `var auditPublisher usecase.AuditPublisher` alongside the existing
`var invalidationPublisher usecase.CacheInvalidationPublisher` — a nil
`auditPublisher` (NATS unreachable at startup) is valid, same nil-safe
convention `CacheInvalidationPublisher` already establishes.

4. Update the three changed constructor calls:
```go
createDepartmentUC := usecase.NewCreateDepartment(companies, departments, opa, auditPublisher)
updateCompanyUC := usecase.NewUpdateCompany(companies, profiles, profileCache, invalidationPublisher, opa, auditPublisher)
updateDepartmentUC := usecase.NewUpdateDepartment(departments, profiles, profileCache, invalidationPublisher, opa, auditPublisher)
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./...
go vet ./services/tenant-service/...
```

Run the full test suite for the changed package and confirm every test case
from TASK-PRF-01-01 through -07's Verify sections passes together (some were
only `go vet`-checked in isolation until this task's constructor wiring
compiles):

```bash
go test ./services/tenant-service/... -v
opa test policy/orca-authz/ -v
```

Expected: clean build, all tenant-service tests green, `opa test` green.
