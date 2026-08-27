# admin-action checks — replaces auth-service's requireAdminActor inline
# `role == "admin"` check (execution-plan.md Epic E). Consumed as
# data.orca.authz.admin.allow via common/policy.Evaluator.
#
# input shape: {"actor": {"id": <string>, "role": <string>}}
package orca.authz.admin

import rego.v1

default allow := false

allow if {
	input.actor.role == "admin"
}
