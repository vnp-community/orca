package wscompat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type fakeToolLister struct{ views []McpToolView }

func (f fakeToolLister) AdminToolViews(context.Context, string) ([]McpToolView, error) {
	return f.views, nil
}

func callToolList(t *testing.T, d McpChannelDeps, id Identity, arg string) (any, error) {
	t.Helper()
	r := NewRegistry()
	registerMcpToolCatalogChannel(r, d)
	var args []json.RawMessage
	if arg != "" {
		args = []json.RawMessage{json.RawMessage(arg)}
	}
	return r.Dispatch(context.Background(), id, "mcp.admin.tool.list", args)
}

func TestMcpAdminToolList(t *testing.T) {
	d := McpChannelDeps{Enabled: true, ToolCatalog: fakeToolLister{views: []McpToolView{
		{Name: "git_status", Namespace: "git", Risk: "read"},
		{Name: "git_commit", Namespace: "git", Risk: "write_reversible"},
		{Name: "admin_listUsers", Namespace: "admin", Risk: "admin"},
	}}}
	admin := Identity{TenantID: "t", UserID: "u", Role: "admin"}

	out, err := callToolList(t, d, admin, `{"namespace":"git"}`)
	if err != nil {
		t.Fatal(err)
	}
	views := out.([]McpToolView)
	if len(views) != 2 || views[0].Name != "git_commit" {
		t.Errorf("filter/sort: %+v", views)
	}
	b, _ := json.Marshal(views[0])
	for _, k := range []string{`"requiredScope"`, `"hardDenied"`, `"effectiveSource"`, `"annotations":{"readOnly"`} {
		if !strings.Contains(string(b), k) {
			t.Errorf("missing %s in %s", k, b)
		}
	}
	if out, _ := callToolList(t, d, admin, `{"risk":"admin"}`); len(out.([]McpToolView)) != 1 {
		t.Error("risk filter")
	}
	if out, _ := callToolList(t, d, admin, ``); len(out.([]McpToolView)) != 3 {
		t.Error("no args = all")
	}

	for _, role := range []string{"user", ""} {
		if _, err := callToolList(t, d, Identity{Role: role}, ``); err == nil || !strings.HasPrefix(err.Error(), "MCP_NOT_ADMIN") {
			t.Errorf("role %q must get MCP_NOT_ADMIN, got %v", role, err)
		}
	}
	d.Enabled = false
	if _, err := callToolList(t, d, admin, ``); err == nil || !strings.HasPrefix(err.Error(), "MCP_DISABLED") {
		t.Errorf("disabled: %v", err)
	}
	d.Enabled, d.ToolCatalog = true, nil
	if _, err := callToolList(t, d, admin, ``); err == nil || !strings.HasPrefix(err.Error(), "MCP_UNAVAILABLE") {
		t.Errorf("nil catalog: %v", err)
	}
}
