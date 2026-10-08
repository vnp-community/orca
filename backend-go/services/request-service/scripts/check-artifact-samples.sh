#!/usr/bin/env bash
# The canonical-JSON golden cases must stay byte-identical in request-service and task-service,
# because both services hash task specs and the digests have to agree (CR-REQ-027).
# Run from backend-go/services/request-service; fails when the copies differ or task-service has none yet.
set -euo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
mine="$here/testdata/artifacts/task"
theirs="$here/../task-service/testdata/artifacts/task"
if [ ! -d "$theirs" ]; then
  echo "task-service has no testdata/artifacts/task yet: $theirs" >&2
  exit 1
fi
diff -r "$mine" "$theirs"
echo "artifact samples identical"
