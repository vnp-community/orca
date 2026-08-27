# annotation author-only edit/delete, admin override — annotation-service's
# UpdateAnnotation/DeleteAnnotation enforce tenant isolation only today
# (README "Known gaps"); this is the OPA policy decision the design doc's
# §9 assigns to the gateway/usecase boundary instead of an inline check
# (execution-plan.md Epic E). Consumed as data.orca.authz.annotation.allow
# via common/policy.Evaluator.
#
# input shape: {"actor_id": <string>, "author_id": <string>, "actor_role": <string>}
package orca.authz.annotation

import rego.v1

default allow := false

allow if {
	input.actor_id == input.author_id
}

allow if {
	input.actor_role == "admin"
}
