# TASK-021: Regression test for `accounts.select*`'s null-accountId relay fix

**From Solution:** SOL-010
**Priority:** High
**Service:** `api-gateway`
**File:** `internal/adapter/wscompat/channels_accounts_test.go` (existing file, extended)
**Depends on:** TASK-020
**Status:** `[x]` DONE — added `TestAccountsChannels_NullAccountId_RelaysAsJSONNull`, covering both `accounts.selectClaude` and `accounts.selectCodex`. **Proven to actually regression-test the bug**: reverted TASK-020's fix locally, confirmed the new test fails (`relayed accountId = "", want JSON null`) against the pre-fix code, then restored the fix. All 12 tests in the file (11 pre-existing + 1 new) pass; `go test ./services/api-gateway/internal/adapter/wscompat/... -count=1` clean.

---

## Context

`channels_accounts_test.go` already had thorough coverage (`TestAccountsChannels_RelaySuccess`, subscribe/resolve tests) but nothing exercised a `null` `accountId` — the exact shape the frontend sends for "System default" and the exact shape that was silently corrupted before TASK-020's fix.

## Changes made

`internal/adapter/wscompat/channels_accounts_test.go` — added, right before `TestAccountsChannels_MissingConnectionID_FailsFastWithoutCallingRelay`:

```go
func TestAccountsChannels_NullAccountId_RelaysAsJSONNull(t *testing.T) {
	cases := []string{"accounts.selectClaude", "accounts.selectCodex"}
	for _, channel := range cases {
		t.Run(channel, func(t *testing.T) {
			var gotReq *infrafleetv1.RelayByDevServerRequest
			fake := &fakeAccountsRelayClient{
				relayByDevServerFunc: func(_ context.Context, in *infrafleetv1.RelayByDevServerRequest) (*infrafleetv1.RelayResponse, error) {
					gotReq = in
					return &infrafleetv1.RelayResponse{ResultJson: `{"ok":true}`}, nil
				},
			}
			r := NewRegistry()
			registerAccountsChannels(r, fake)

			args := argsJSON(t, map[string]any{"accountId": nil, "connectionId": "ds-1"})
			if _, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, channel, args); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var params map[string]any
			if err := json.Unmarshal([]byte(gotReq.GetParamsJson()), &params); err != nil {
				t.Fatalf("params_json not valid JSON: %v", err)
			}
			accountID, present := params["accountId"]
			if !present {
				t.Fatal("expected params to have an \"accountId\" key at all")
			}
			if accountID != nil {
				t.Errorf(`relayed accountId = %#v, want JSON null`, accountID)
			}
		})
	}
}
```

Uses the file's existing `fakeAccountsRelayClient`/`argsJSON` helpers — no new test infrastructure needed.

## Verify

```bash
cd backend-go
go test ./services/api-gateway/internal/adapter/wscompat/... -run TestAccounts -count=1 -v
```

Expected: all 12 tests pass (11 pre-existing + this one).
