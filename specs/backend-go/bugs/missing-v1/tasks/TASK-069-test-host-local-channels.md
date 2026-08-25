# TASK-069: Test `host.*` local-answer channels

**From Solution:** SOL-011 (Test plan — "Shippable now")
**Priority:** P2
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_test.go`
**Depends on:** TASK-068
**Status:** `[x]` DONE (verified) — test added in the new
`channels_emulator_folderworkspace_host_test.go` file (not
`channels_test.go` — same isolation reason as TASK-047). `go test
./internal/adapter/wscompat/... -run TestRegisterHostChannels -v` — all
4 subtests pass.

---

## Context

Table-driven test asserting all 4 `host.*` channels return the honest
`false`/`[]` shape (not an error, not `notImplementedHandler`'s generic
message) with zero downstream calls — there is nothing to call downstream
today, so this test only needs to assert the response shape and that
dispatch resolves to a real registered handler.

---

## Changes to make

**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_test.go`

```go
func TestRegisterHostChannels_HonestLocalAnswers(t *testing.T) {
	r := NewRegistry()
	registerHostChannels(r)

	tests := []struct {
		channel string
		want    any
	}{
		{"host.wsl.isAvailable", map[string]bool{"available": false}},
		{"host.wsl.listDistros", []string{}},
		{"host.pwsh.isAvailable", map[string]bool{"available": false}},
		{"host.gitBash.isAvailable", map[string]bool{"available": false}},
	}

	for _, tt := range tests {
		t.Run(tt.channel, func(t *testing.T) {
			result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, tt.channel, nil)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if !reflect.DeepEqual(result, tt.want) {
				t.Errorf("channel %q: got %#v, want %#v", tt.channel, result, tt.want)
			}
		})
	}
}
```

Ensure the test file's imports include `"reflect"` (add if missing).

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go test ./internal/adapter/wscompat/... -run TestRegisterHostChannels -v
go vet ./internal/adapter/wscompat/...
```

Expected: all 4 subtests pass.
