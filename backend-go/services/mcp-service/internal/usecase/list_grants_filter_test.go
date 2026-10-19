package usecase

import "testing"

// TASK-E2E-017: an admin filter that cannot be a uuid names no user, so it lists nothing instead of failing the query.
func TestListGrants_MalformedUserFilterIsEmptyNotError(t *testing.T) {
	h := newAuthzHarness(t)
	got, err := h.list.Execute(as(tenantA, userBob, "admin"), ListGrantsInput{AllUsers: true, UserID: "not-a-uuid"})
	if err != nil || len(got) != 0 {
		t.Fatalf("Execute = %v, %v; want empty, nil", got, err)
	}
}
