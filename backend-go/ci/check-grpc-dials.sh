#!/usr/bin/env bash
# Every service-to-service gRPC connection must be dialed with grpcmw.NewClient so the caller's identity
# (tenant, user, role, actor type) travels with each call. A bare grpc.NewClient drops it and the callee
# answers NO_TENANT / NO_USER far from where the dial was written.
#
# Allowed bare dials, each with a reason:
#   common/grpcmw            defines the wrapper.
#   services/api-gateway     attaches identity explicitly per call (gatewaygrpc.AttachIdentity) from the session.
#   *contracttest*, *_test   in-memory test connections.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

hits="$(grep -rnE --include='*.go' 'grpc\.(NewClient|Dial|DialContext)\(' common services cmd 2>/dev/null \
  | grep -vE '_test\.go:|/contracttest/|^common/grpcmw/|^services/api-gateway/|/proto/gen/' \
  | grep -vE '^[^:]+:[0-9]+:\s*//' || true)"

if [ -n "$hits" ]; then
  echo "Bare gRPC dials found; use grpcmw.NewClient (github.com/stablyai/orca-go/common/grpcmw):" >&2
  echo "$hits" >&2
  exit 1
fi
echo "ok: every service dials through grpcmw.NewClient"
