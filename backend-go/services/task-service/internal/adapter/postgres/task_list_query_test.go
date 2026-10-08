package postgres

import (
	"reflect"
	"strconv"
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
		{"tenant only", usecase.ListFilter{}, []string{"tenant_id = $1"}, []any{"t", int32(50)}},
		{"project", usecase.ListFilter{ProjectID: "p"}, []string{"project_id = $2::uuid"}, []any{"t", "p", int32(50)}},
		{"types", usecase.ListFilter{TaskTypes: []string{"plan"}}, []string{"task_type = ANY($2::text[])"}, []any{"t", []string{"plan"}, int32(50)}},
		{"requests", usecase.ListFilter{RequestIDs: []string{"r1", "r2"}}, []string{"request_id = ANY($2::uuid[])"}, []any{"t", []string{"r1", "r2"}, int32(50)}},
		{"parent", usecase.ListFilter{ParentID: "x"}, []string{"parent_id = $2::uuid"}, []any{"t", "x", int32(50)}},
		{"all in order", usecase.ListFilter{ProjectID: "p", TaskTypes: []string{"phase"}, RequestIDs: []string{"r"}, ParentID: "x", PageToken: "tok", PageSize: 7},
			[]string{"project_id = $2::uuid", "task_type = ANY($3::text[])", "request_id = ANY($4::uuid[])", "parent_id = $5::uuid", "id > $6::uuid"},
			[]any{"t", "p", []string{"phase"}, []string{"r"}, "x", "tok", int32(7)}},
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
		if !strings.Contains(q, "ORDER BY id LIMIT $"+strconv.Itoa(len(args))) {
			t.Errorf("%s: LIMIT must bind the last arg:\n%s", c.name, q)
		}
	}
}
