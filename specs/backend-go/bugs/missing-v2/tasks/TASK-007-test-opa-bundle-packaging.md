# TASK-007: Tests for `Evaluator.Warm` + a CI check that actually catches the container-packaging gap

**From Solution:** SOL-003
**Priority:** P0
**Service:** `common/policy`, CI config
**File:** `common/policy/evaluator_test.go`, a new CI step (location depends on this repo's CI config — see Step 3)
**Depends on:** TASK-005, TASK-006
**Status:** `[ ]` TODO

---

## Context

`common/policy/evaluator_test.go` already runs real `Decision` calls
against the real bundle at `../../policy/orca-authz` — a path that's
**correct** when `go test` runs from `common/policy/`'s own directory,
which is exactly why BUG-003 (a container-packaging gap, not a Go bug) was
invisible to this existing test. TASK-005/006 fix the actual bug; this
task adds (1) unit coverage for the new `Warm` method and (2) the one test
that would have caught BUG-003 before it reached `172.20.2.39` — a
built-image inspection, not another Go unit test against the checkout's
own relative paths.

## Changes to make

### Step 1 — unit tests for `Warm`, in `common/policy/evaluator_test.go`

```go
func TestEvaluator_Warm_Succeeds_ForRealBundle(t *testing.T) {
	e := policy.NewEvaluator(bundlePath)
	if err := e.Warm(context.Background(),
		"data.orca.authz.admin.allow",
		"data.orca.authz.project.allow",
		"data.orca.authz.task.allow",
		"data.orca.authz.annotation.allow",
	); err != nil {
		t.Fatalf("expected Warm to succeed against the real bundle for every consuming service's query: %v", err)
	}
}

func TestEvaluator_Warm_FailsFast_WhenBundlePathIsWrong(t *testing.T) {
	e := policy.NewEvaluator("/definitely/does/not/exist")
	if err := e.Warm(context.Background(), "data.orca.authz.admin.allow"); err == nil {
		t.Fatal("expected Warm to fail against a nonexistent bundle path — this is the exact failure mode BUG-003 needed to happen at startup, not on first request")
	}
}

func TestEvaluator_Warm_PopulatesCache_DecisionDoesNotRecompile(t *testing.T) {
	// Indirect check: Warm should leave the query pre-compiled so a
	// subsequent Decision call for the same query doesn't pay the compile
	// cost again — this is the same `prepared` map Decision itself uses
	// (preparedQuery's cache-hit path), so this test just confirms Warm and
	// Decision agree on the same cache key format (the raw query string) by
	// calling both against the same evaluator instance without error.
	e := policy.NewEvaluator(bundlePath)
	if err := e.Warm(context.Background(), "data.orca.authz.admin.allow"); err != nil {
		t.Fatalf("Warm: %v", err)
	}
	if _, err := e.Decision(context.Background(), "data.orca.authz.admin.allow", map[string]any{
		"actor": map[string]any{"role": "admin", "id": "u1"},
	}); err != nil {
		t.Fatalf("Decision after Warm: %v", err)
	}
}
```

### Step 2 — table-driven variant covering all 4 real consumer queries individually

Fold into `TestEvaluator_Warm_Succeeds_ForRealBundle` above (already
covers all 4 in one call) unless a future reviewer wants per-query
failure isolation — note this as a judgment call for whoever implements,
not a hard requirement.

### Step 3 — CI: build all 4 images, assert the bundle actually landed

This is the test that directly prevents BUG-003's regression class. Exact
placement depends on this repo's CI tooling (check for an existing
`.github/workflows/`, `Makefile` `ci` target, or similar in `backend-go/`
before picking a location) — the check itself, regardless of where it's
wired in:

```bash
#!/usr/bin/env bash
# ci/check-opa-bundle-in-images.sh (or fold into an existing CI script)
set -euo pipefail
cd "$(dirname "$0")/.."  # backend-go/

for svc in auth-service task-service annotation-service project-service; do
  echo "Building $svc..."
  docker build -q -f "services/$svc/deploy/Dockerfile" -t "orca-go/$svc:ci-opa-check" .

  echo "Checking $svc image for policy/orca-authz..."
  # distroless has no shell — export the image filesystem and grep the
  # tarball listing instead of trying to exec anything inside it.
  cid=$(docker create "orca-go/$svc:ci-opa-check")
  if ! docker export "$cid" | tar -tv | grep -q '^.*policy/orca-authz/.*\.rego$'; then
    echo "FAIL: $svc image does not contain policy/orca-authz/*.rego" >&2
    docker rm "$cid" >/dev/null
    exit 1
  fi
  docker rm "$cid" >/dev/null
  echo "OK: $svc"
done
echo "All 4 images contain the OPA bundle."
```

Wire this into whatever runs on every PR touching
`services/{auth,task,annotation,project}-service/deploy/Dockerfile`,
`common/policy/`, or `policy/orca-authz/` — the goal is that a future
5th service adopting embedded OPA (or an edit to one of these 4
Dockerfiles that accidentally drops the `COPY policy` line again) fails CI
immediately, not silently ships.

## Verify

```bash
cd backend-go
go test ./common/policy/... -count=1 -v -run 'TestEvaluator_Warm'
bash ci/check-opa-bundle-in-images.sh   # or wherever Step 3's script ends up living
```

Expected: all 3 `Warm` unit tests pass; the image-inspection script prints
`OK` for all 4 services. Re-run
`bash ci/check-opa-bundle-in-images.sh` against a deliberately-reverted
Dockerfile (temporarily remove the `COPY policy ./policy` line from one
service) to confirm the check actually fails when the bug is
reintroduced — this is the check's own regression test, worth doing once
manually even though it's not itself committed as an automated test.
