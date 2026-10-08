package mysql

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

func TestBuildListQuery(t *testing.T) {
	cases := []struct {
		name      string
		f         usecase.ListFilter
		wantConds []string
		wantArgs  []any
	}{
		{"tenant only", usecase.ListFilter{}, []string{"tenant_id = ?"}, []any{"t", int32(50)}},
		{"project", usecase.ListFilter{ProjectID: "p"}, []string{"project_id = ?"}, []any{"t", "p", int32(50)}},
		{"types", usecase.ListFilter{TaskTypes: []string{"plan", "phase"}}, []string{"task_type IN (?,?)"}, []any{"t", "plan", "phase", int32(50)}},
		{"requests", usecase.ListFilter{RequestIDs: []string{"r1", "r2", "r3"}}, []string{"request_id IN (?,?,?)"}, []any{"t", "r1", "r2", "r3", int32(50)}},
		{"parent", usecase.ListFilter{ParentID: "x"}, []string{"parent_id = ?"}, []any{"t", "x", int32(50)}},
		{"all in order", usecase.ListFilter{ProjectID: "p", TaskTypes: []string{"phase"}, RequestIDs: []string{"r"}, ParentID: "x", PageToken: "tok", PageSize: 7},
			[]string{"project_id = ? AND task_type IN (?) AND request_id IN (?) AND parent_id = ? AND id > ?"},
			[]any{"t", "p", "phase", "r", "x", "tok", int32(7)}},
	}
	for _, c := range cases {
		q, args := buildListQuery("t", c.f)
		for _, cond := range c.wantConds {
			if !strings.Contains(q, cond) {
				t.Errorf("%s: query lacks %q:\n%s", c.name, cond, q)
			}
		}
		if !reflect.DeepEqual(args, c.wantArgs) {
			t.Errorf("%s: args %#v, want %#v", c.name, args, c.wantArgs)
		}
		if strings.Count(q, "?") != len(args) {
			t.Errorf("%s: %d placeholders for %d args:\n%s", c.name, strings.Count(q, "?"), len(args), q)
		}
		if !strings.HasSuffix(q, "ORDER BY id LIMIT ?") {
			t.Errorf("%s: unexpected tail:\n%s", c.name, q)
		}
	}
}
