# TASK-006: Copy the `orca-authz` bundle into all 4 embedded-OPA service images, default `OPA_BUNDLE_PATH` to the container-correct absolute path

**From Solution:** SOL-003
**Priority:** P0
**Service:** `auth-service`, `task-service`, `annotation-service`, `project-service` (identical change × 4)
**File:** `services/{auth,task,annotation,project}-service/deploy/Dockerfile`, `services/{auth,task,annotation,project}-service/internal/config/config.go`
**Depends on:** none (pairs with TASK-005 for one PR)
**Status:** `[ ]` TODO

---

## Context

This is the actual root cause of BUG-003: `policy/orca-authz/` (the Rego
bundle, at the `backend-go/` repo root) is never copied into any of the 4
services' final distroless image stages — confirmed by diffing all 4
Dockerfiles, byte-identical except for the service name. `OPABundlePath`
defaults to a relative path (`../../policy/orca-authz`) correct only when
run from the service's own directory in a checkout, never inside the
container (no `WORKDIR` is even set in the final stage).

## Changes to make

Apply the identical change to all 4 `deploy/Dockerfile`s — shown here for
`project-service`; substitute the service name for the other 3:

```dockerfile
# services/project-service/deploy/Dockerfile
FROM golang:1.23-bookworm AS build
WORKDIR /src
COPY go.work go.work.sum* ./
COPY common ./common
COPY proto ./proto
COPY policy ./policy                                            # NEW
COPY services/project-service ./services/project-service
WORKDIR /src/services/project-service
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -X main.version=${VERSION:-dev}" \
    -o /out/project-service ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/project-service /project-service
COPY --from=build /src/policy/orca-authz /policy/orca-authz      # NEW
COPY services/project-service/migrations /migrations
USER nonroot:nonroot
ENTRYPOINT ["/project-service"]
```

Then, in each service's `internal/config/config.go`, change the
`OPABundlePath` default from the relative dev-checkout path to the
absolute in-container path the Dockerfile change above now guarantees:

```go
// BEFORE (all 4 services, identical):
OPABundlePath: commonconfig.StringEnv("OPA_BUNDLE_PATH", "../../policy/orca-authz"),

// AFTER:
OPABundlePath: commonconfig.StringEnv("OPA_BUNDLE_PATH", "/policy/orca-authz"),
```

Local dev / direct `go run`/`go test` from a service's own directory now
needs `OPA_BUNDLE_PATH=../../policy/orca-authz` set explicitly (e.g. in
`docker-compose.yml`'s dev environment block, or a local `.env`) — the
compiled-in default now targets where the binary actually runs in every
deployed environment, matching `10-deployment-infrastructure.md`'s
Kubernetes-topology-is-the-target framing; check whether
`docker-compose.yml`/any local-dev env file already sets `OPA_BUNDLE_PATH`
and update it to the relative path if so, or add it if this change would
otherwise break local dev's own container (local dev's compose build might
use the SAME Dockerfile, in which case `/policy/orca-authz` is already
correct there too and no override is needed — verify which is true before
assuming an override is required).

## Verify

```bash
cd backend-go
for svc in auth-service task-service annotation-service project-service; do
  docker build -f services/$svc/deploy/Dockerfile -t orca-go/$svc:opa-bundle-check .
  echo "=== $svc ==="
  docker run --rm --entrypoint /bin/true orca-go/$svc:opa-bundle-check 2>&1 || true
  # distroless has no shell to `ls` with — verify the file landed via a
  # multi-stage COPY --from inspection instead:
  docker run --rm --entrypoint "" gcr.io/distroless/static-debian12:nonroot true 2>&1 || true
done
```

Distroless images have no shell, so `ls`-style manual inspection doesn't
work directly — use `docker cp` from a stopped container, or (simpler)
`docker build --target build` against the intermediate `build` stage
(which DOES have a shell) and `ls /src/policy/orca-authz` there to confirm
the bundle reached the build context, then trust the final `COPY --from=build`
line for the rest. TASK-007 adds a proper automated version of this check
suitable for CI.

After building, re-run each service's own `go test` to confirm nothing
else broke:

```bash
go test ./services/auth-service/... ./services/task-service/... ./services/annotation-service/... ./services/project-service/... -count=1
```
