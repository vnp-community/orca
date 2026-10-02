package mcppolicy

import (
	"context"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"testing"

	"github.com/stablyai/orca-go/common/policy"
)

// The Go fuse and the Rego hard-deny prefixes must never drift apart.
func TestNeverDispatchPrefixesMatchRego(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	bundle := filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "policy", "orca-authz")
	v, err := policy.NewEvaluator(bundle).Value(context.Background(), "data.orca.authz.mcp.hard_deny_prefixes", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	var fromRego []string
	for _, x := range v.([]any) {
		fromRego = append(fromRego, x.(string))
	}
	sort.Strings(fromRego)
	goSide := append([]string(nil), NeverDispatchPrefixes...)
	sort.Strings(goSide)
	if !reflect.DeepEqual(fromRego, goSide) {
		t.Fatalf("rego=%v go=%v", fromRego, goSide)
	}
}

func TestNeverDispatch(t *testing.T) {
	for ch, want := range map[string]bool{"mcp.approval.decide": true, "credentials.get": true, "auth.login": true, "task.create": false, "task.mcp.note": false, "mcpx.y": false} {
		if NeverDispatch(ch) != want {
			t.Errorf("%s", ch)
		}
	}
}
