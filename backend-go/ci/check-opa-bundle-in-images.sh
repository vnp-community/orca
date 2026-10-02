#!/usr/bin/env bash
# ci/check-opa-bundle-in-images.sh (or fold into an existing CI script)
set -euo pipefail
cd "$(dirname "$0")/.."  # backend-go/

svcs=(auth-service task-service annotation-service project-service mcp-service)
for svc in "${svcs[@]}"; do
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
echo "All ${#svcs[@]} images contain the OPA bundle."
