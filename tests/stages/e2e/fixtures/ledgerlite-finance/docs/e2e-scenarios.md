# LedgerLite e2e scenarios

Three open work items for ORCA's e2e to run as Requests against a throwaway copy of this fixture. None of them is implemented in the code or the docs: the code is the clean baseline, and the bug exists only after you apply the patch in scenario 1.

All commands below were run against this fixture while it was written: each definition-of-done block passes on a reference implementation and fails on the untouched baseline.

## 0. Prepare the throwaway repo

```bash
FIXTURE=<orca>/tests/stages/e2e/fixtures/ledgerlite-finance
WORK="$(mktemp -d)/ledgerlite" && mkdir -p "$WORK" && cp -R "$FIXTURE/." "$WORK/" && cd "$WORK"
tdd-agent generate                 # design knowledge (graph), see README
tdd-agent mcp enable --read-only   # writes .mcp.json; must exist BEFORE the commit so task worktrees get it
git init -q -b main && git add -A && git commit -q -m "LedgerLite baseline"
export GOWORK=off          # only needed if a go.work exists in a parent directory
go test ./... -count=1     # baseline is green
```

The two `tdd-agent` lines come before the commit on purpose (same order as the README). Skip them only for Requests that do not need the knowledge graph.

| # | Title | Type | Size | Engine | Needs knowledge graph |
| --- | --- | --- | --- | --- | --- |
| 1 | Daily limit does not reset at UTC midnight | bug | S | native | optional (BR-003, ADR-005) |
| 2 | Add account statement endpoint | task | S (M accepted) | native | optional (REQ-007, FR-005) |
| 3 | Charge a fee on transfers | change_request | L (M accepted) | OpenSpec | yes (touches SRS and SAD) |

---

## 1. Bug: daily limit does not reset at UTC midnight

### 1.1 Inject the bug (setup step, then commit it so the Request starts from the buggy tree)

The regression is a single line in `internal/ledger/service.go`: the limit window becomes "the last 24 hours" instead of "the current UTC calendar day". The patch applies cleanly to the pristine fixture and was checked with `git apply --check`.

```bash
git apply <<'PATCH'
diff --git a/internal/ledger/service.go b/internal/ledger/service.go
index b14b6b1..11122dd 100644
--- a/internal/ledger/service.go
+++ b/internal/ledger/service.go
@@ -127,7 +127,7 @@ func (s *Service) transferLocked(req TransferRequest) (Transfer, bool, error) {
 
 // sentToday sums what the account sent during the UTC calendar day of now (ADR-005).
 func (s *Service) sentToday(accountID string, now time.Time) int64 {
-	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
+	dayStart := now.Add(-24 * time.Hour)
 	var sent int64
 	for _, e := range s.store.EntriesSince(accountID, dayStart) {
 		if e.Delta < 0 {
PATCH
git commit -qam "Use a rolling 24h window for the daily limit"
BUG_COMMIT="$(git rev-parse HEAD)"   # the tree a Request starts from
```

### 1.2 Request to create

- Title: `Daily limit does not reset at UTC midnight`
- Expected type: `bug`
- Expected size: `S`
- Expected urgency: `normal`

Body (paste as is):

```text
Support reports that accounts stay blocked after midnight UTC.

Steps to reproduce
1. Take an account with daily_limit 1000 and a balance of at least 2000.
2. At 09:00 UTC send transfers that total exactly 1000. They are accepted.
3. At 00:01 UTC on the next day send a transfer of 1000.

Actual: the transfer is rejected with 422 daily_limit_exceeded. The same transfer is accepted only after 09:00 UTC, 24 hours after the first one.

Expected: the daily limit window is the UTC calendar day (SRS BR-003, SAD ADR-005), so the transfer on the new day is accepted.

Evidence: `go test ./internal/ledger -run TestDailyLimitIsInclusiveAndResetsAtUTCMidnight -count=1` fails with "a new UTC day resets the window: daily transfer limit exceeded".
```

### 1.3 Expected fix

In `Service.sentToday` (`internal/ledger/service.go`) the window must start at 00:00:00 UTC of the day of `now`:
`dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)`. Add a regression test for the boundary just before and just after midnight (23:59 then 00:01). No change to the SRS or SAD is needed: they already describe the correct behaviour.

Knowledge an analysis should cite when the graph is available: BR-003, ADR-005, SVC-002, FR-003.

### 1.4 Definition of done (run from the repo root)

Before the fix the first test command fails; after the fix everything below exits 0.

```bash
set -euo pipefail
export GOWORK=off
go vet ./... && go test -race -count=1 ./...
go test ./internal/ledger -run TestDailyLimitIsInclusiveAndResetsAtUTCMidnight -count=1
# three DailyLimit tests exist in the baseline; the fix must add at least one regression test
test "$(go test -count=1 -v -run DailyLimit ./... | grep -c -- '--- PASS')" -ge 4
# the change stays inside the ledger package (BUG_COMMIT from step 1.1; committed or uncommitted work both count)
CHANGED="$(git diff --name-only "$BUG_COMMIT")"   # fails loudly if BUG_COMMIT is unset or wrong
! printf '%s\n' "$CHANGED" | grep -qv -e '^internal/ledger/' -e '^$'
```

---

## 2. Task: add an account statement endpoint

### 2.1 Request to create

- Title: `Add account statement endpoint`
- Expected type: `task`
- Expected size: `S` (a reviewer may reasonably say `M`)
- Expected urgency: `normal`

Body (paste as is):

```text
Customers need to see what happened on an account (URD REQ-007, planned for after v1.0).

Add GET /accounts/{id}/statement with optional query parameters from and to, both UTC dates in the form YYYY-MM-DD. from is inclusive (00:00:00 UTC of that day) and to is exclusive (00:00:00 UTC of that day). If a parameter is missing that side of the range is open.

200 response:
{
  "account_id": "acc_1",
  "opening_balance": 0,
  "closing_balance": 4100,
  "lines": [ {"seq": 2, "transfer_id": "txn_1", "at": "2026-03-10T09:00:00Z", "delta": 5000, "balance_after": 5000} ]
}

Rules
- lines are the account's journal entries in the range, in seq order; balance_after is the running balance after that line.
- opening_balance is the balance just before the first moment of the range; closing_balance is opening_balance plus the sum of the deltas in the range. With no filters, closing_balance equals the current balance of the account.
- 404 account_not_found for an unknown account.
- 400 invalid_range when a date cannot be parsed or when from is not before to.
- Do not change how the journal is written (it stays append-only) and add no dependencies.

Add unit tests in internal/ledger and HTTP tests in internal/httpapi, with names starting with TestStatement.
```

### 2.2 Acceptance criteria

1. The endpoint behaves exactly as above (checked by the black-box script in 2.3).
2. New tests named `TestStatement...` exist and pass; the existing suite stays green under `-race`.
3. `go.mod` still has no `require` directive.
4. Expected, not enforced by the script: `docs/SRS.md` gains a functional requirement row (FR-008) and an interface row (API-007) in the same table format, so `tdd-agent generate` keeps working.

Knowledge an analysis should cite when the graph is available: REQ-007 (no FR realises it yet), FR-005, API-005, ENT-003, FR-002.

### 2.3 Definition of done (run from the repo root)

Fails on the baseline (no statement tests, no endpoint), exits 0 on a correct implementation.

```bash
set -euo pipefail
export GOWORK=off
go vet ./... && go test -race -count=1 ./...
go test -count=1 -v -run Statement ./... | grep -q -- '--- PASS: TestStatement'
! grep -q '^require' go.mod
# Build and start ledgerd on a free port; $BASE is its URL. Stops it on exit.
BIN="$(mktemp -d)/ledgerd"
go build -o "$BIN" ./cmd/ledgerd
PORT="$(python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1])')"
"$BIN" -addr "127.0.0.1:$PORT" ${LEDGERD_FLAGS:-} >/dev/null &
LEDGERD_PID=$!
trap 'kill $LEDGERD_PID 2>/dev/null' EXIT
BASE="http://127.0.0.1:$PORT"
for _ in $(seq 50); do python3 -c "import urllib.request,sys;urllib.request.urlopen('$BASE/healthz')" 2>/dev/null && break; sleep 0.1; done
export BASE
python3 - <<'PY'
import json, os, urllib.request, urllib.error
BASE = os.environ["BASE"]
def call(method, path, body=None, key=None):
    req = urllib.request.Request(BASE + path, method=method, data=json.dumps(body).encode() if body is not None else None)
    if key: req.add_header("Idempotency-Key", key)
    try:
        with urllib.request.urlopen(req) as r: return r.status, json.load(r)
    except urllib.error.HTTPError as e: return e.code, json.load(e)
_, a = call("POST", "/accounts", {"owner": "a", "initial_deposit": 5000})
_, b = call("POST", "/accounts", {"owner": "b"})
assert call("POST", "/transfers", {"from": a["id"], "to": b["id"], "amount": 1200}, "st-1")[0] == 201
assert call("POST", "/transfers", {"from": b["id"], "to": a["id"], "amount": 300}, "st-2")[0] == 201
st, s = call("GET", f"/accounts/{a['id']}/statement")
assert st == 200, (st, s)
assert [l["delta"] for l in s["lines"]] == [5000, -1200, 300], s
assert [l["balance_after"] for l in s["lines"]] == [5000, 3800, 4100], s
assert s["opening_balance"] == 0 and s["closing_balance"] == 4100, s
assert all(k in s["lines"][0] for k in ("seq", "transfer_id", "at", "delta", "balance_after")), s
_, acc = call("GET", f"/accounts/{a['id']}")
assert s["closing_balance"] == acc["balance"], (s, acc)
st, s = call("GET", f"/accounts/{a['id']}/statement?from=2099-01-01")          # nothing on or after from
assert st == 200 and s["lines"] == [] and s["opening_balance"] == 4100 and s["closing_balance"] == 4100, s
st, s = call("GET", f"/accounts/{a['id']}/statement?to=2000-01-02")             # nothing before to
assert st == 200 and s["lines"] == [] and s["opening_balance"] == 0 and s["closing_balance"] == 0, s
st, s = call("GET", f"/accounts/{a['id']}/statement?from=2000-01-01&to=2099-01-01")
assert st == 200 and len(s["lines"]) == 3, s
for q in ("from=2026-01-02&to=2026-01-01", "from=2026-01-01&to=2026-01-01", "from=not-a-date", "to=31-12-2026"):
    st, e = call("GET", f"/accounts/{a['id']}/statement?{q}")
    assert st == 400 and e["error"]["code"] == "invalid_range", (q, st, e)
st, e = call("GET", "/accounts/nope/statement")
assert st == 404 and e["error"]["code"] == "account_not_found", (st, e)
print("statement black-box checks passed")
PY
```

Optional documentation check: `grep -Eq '^\| FR-008 ' docs/SRS.md && grep -Eq '^\| API-007 ' docs/SRS.md`.

---

## 3. Change request: charge a fee on transfers

### 3.1 Request to create

- Title: `Charge a fee on transfers`
- Expected type: `change_request`
- Expected size: `L` (`M` accepted)
- Expected urgency: `normal`
- Expected engine: OpenSpec (change_request runs the OpenSpec flow, so the knowledge graph and the tdd-agent skills matter here)

Body (paste as is):

```text
Finance wants to earn a fee on every customer transfer.

Fee rule
- fee = flat fee + percentage fee, where the percentage fee is amount * bps / 10000 rounded half up to a whole minor unit.
- The flat fee and the basis points are set when the server starts with new flags -fee-flat and -fee-bps. Both default to 0; with both 0 the service behaves exactly as today.
- The sender pays the fee on top of the amount. The fee is credited to a new system account with id acc_fees.
- Opening deposits (transfers from acc_funding) never carry a fee.

Effects
- A transfer with a fee posts three journal entries: sender -(amount+fee), receiver +amount, acc_fees +fee. They still sum to zero. A transfer with fee 0 posts two entries as today.
- The insufficient-funds rule includes the fee. The daily limit counts only the amount, not the fee.
- The transfer resource gains an integer field fee (POST /transfers and GET /transfers/{id}). The fee is stored with the transfer, so an idempotent replay returns the same fee even if the flags changed.

Documentation
- Update docs/SRS.md: the transfer requirement, BR-001 (entries per transfer), new business rules for who pays the fee and for rounding, the Transfer entity and the POST /transfers interface.
- Update docs/SAD.md: record the decision to model fees as a third journal leg to a system account (new ADR) and update the ledger service component and the data model.

Acceptance example with -fee-flat 25 -fee-bps 100: a transfer of 10000 has fee 125 (sender pays 10125); a transfer of 150 has fee 27; a transfer of 149 has fee 26.
```

### 3.2 Why it is a change request

It changes agreed behaviour and two design documents: BR-001 and FR-003 change meaning, ENT-002 and API-003 gain a field, a new decision (ADR-007 expected) is needed, and the graph must be regenerated afterwards.

Knowledge an analysis or solution should cite when the graph is available: FR-003, BR-001, BR-002, BR-003, ENT-002, API-003, ADR-004, SVC-002, SVC-003.

### 3.3 Definition of done (run from the repo root)

Fails on the baseline, exits 0 on a correct implementation.

```bash
set -euo pipefail
export GOWORK=off
go vet ./... && go test -race -count=1 ./...
go test -count=1 -v -run Fee ./... | grep -q -- '--- PASS: Test.*Fee'
! grep -q '^require' go.mod
LEDGERD_FLAGS="-fee-flat 25 -fee-bps 100"
# Build and start ledgerd on a free port; $BASE is its URL. Stops it on exit.
BIN="$(mktemp -d)/ledgerd"
go build -o "$BIN" ./cmd/ledgerd
PORT="$(python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1])')"
"$BIN" -addr "127.0.0.1:$PORT" ${LEDGERD_FLAGS:-} >/dev/null &
LEDGERD_PID=$!
trap 'kill $LEDGERD_PID 2>/dev/null' EXIT
BASE="http://127.0.0.1:$PORT"
for _ in $(seq 50); do python3 -c "import urllib.request,sys;urllib.request.urlopen('$BASE/healthz')" 2>/dev/null && break; sleep 0.1; done
export BASE
python3 - <<'PY'
import json, os, urllib.request, urllib.error
BASE = os.environ["BASE"]
def call(method, path, body=None, key=None):
    req = urllib.request.Request(BASE + path, method=method, data=json.dumps(body).encode() if body is not None else None)
    if key: req.add_header("Idempotency-Key", key)
    try:
        with urllib.request.urlopen(req) as r: return r.status, json.load(r), r.headers
    except urllib.error.HTTPError as e: return e.code, json.load(e), e.headers
def balance(i): return call("GET", f"/accounts/{i}")[1]["balance"]
# server runs with -fee-flat 25 -fee-bps 100
_, a, _ = call("POST", "/accounts", {"owner": "a", "initial_deposit": 100000})
_, b, _ = call("POST", "/accounts", {"owner": "b"})
assert balance(a["id"]) == 100000, "opening deposits must not be charged a fee"
st, t, _ = call("POST", "/transfers", {"from": a["id"], "to": b["id"], "amount": 10000}, "fee-1")
assert st == 201 and t["fee"] == 125 and t["amount"] == 10000, (st, t)          # 25 + 1% of 10000
assert balance(a["id"]) == 100000 - 10125 and balance(b["id"]) == 10000 and balance("acc_fees") == 125
st, t, _ = call("POST", "/transfers", {"from": a["id"], "to": b["id"], "amount": 150}, "fee-2")
assert st == 201 and t["fee"] == 27, (st, t)                                    # 25 + round-half-up(1.5) = 27
st, t, _ = call("POST", "/transfers", {"from": a["id"], "to": b["id"], "amount": 149}, "fee-3")
assert st == 201 and t["fee"] == 26, (st, t)                                    # 25 + round-half-up(1.49) = 26
st, again, h = call("POST", "/transfers", {"from": a["id"], "to": b["id"], "amount": 10000}, "fee-1")
assert st == 200 and h.get("Idempotent-Replayed") == "true" and again["fee"] == 125, (st, again)
assert balance("acc_fees") == 125 + 27 + 26, "a replay must not charge the fee again"
_, c, _ = call("POST", "/accounts", {"owner": "c", "initial_deposit": 10000})
st, e, _ = call("POST", "/transfers", {"from": c["id"], "to": b["id"], "amount": 9900}, "fee-4")   # 9900 + 124 > 10000
assert st == 422 and e["error"]["code"] == "insufficient_funds", (st, e)
assert balance(c["id"]) == 10000 and balance("acc_fees") == 178, "a rejected transfer must not move money or fees"
_, d, _ = call("POST", "/accounts", {"owner": "d", "initial_deposit": 1000, "daily_limit": 500})
st, t, _ = call("POST", "/transfers", {"from": d["id"], "to": b["id"], "amount": 500}, "fee-5")
assert st == 201, (st, t)                                                          # the daily limit counts the amount, not the fee
j = call("GET", "/journal")[1]["entries"]
assert sum(x["delta"] for x in j) == 0, "journal must still sum to zero"
by_transfer = {}
for x in j: by_transfer.setdefault(x["transfer_id"], []).append(x)
fee_tx = [v for v in by_transfer.values() if any(x["account_id"] == "acc_fees" for x in v)]
assert fee_tx and all(len(v) == 3 for v in fee_tx), "a fee-bearing transfer posts exactly three entries"
print("fee black-box checks passed")
PY
# documents updated (a new ADR and at least one new FR or BR row)
grep -Eq '^\| ADR-00[7-9] ' docs/SAD.md
grep -Eq '^\| (FR-00[89]|FR-01[0-9]|BR-00[7-9]|BR-01[0-9]) ' docs/SRS.md
```

Knowledge check (needs `tdd-agent` on PATH; offline): `tdd-agent generate --force` exits 0 and `tdd-agent kg stats` reports more than 67 elements, proving the edited documents still parse.
