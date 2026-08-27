# tenant-service RBAC — company_edit is admin-only; department_edit is admin
# OR a lead of the same department (same_department precomputed by the
# caller — see tenant-service's authorization.go doc comment for why OPA
# doesn't do its own department lookup, mirroring task_grant.rego's
# BFS-precomputed-input pattern).
#
# input shape: {"caller_role": <string>, "action": <string>, "same_department": <bool>}
# caller_role is "admin" | "lead" | "user" | "" (no role claim yet — see
# common/tenant.Role's doc comment for the known upstream gap).
package orca.authz.tenant

import rego.v1

default allow := false

allow if {
	input.action == "company_edit"
	input.caller_role == "admin"
}

allow if {
	input.action == "department_edit"
	input.caller_role == "admin"
}

allow if {
	input.action == "department_edit"
	input.caller_role == "lead"
	input.same_department == true
}
