package orca.authz.request_test

import rego.v1

import data.orca.authz.request

# CR-REQ-044: RetryPlanGeneration is in the "plan" group. The reporter, an owner and an admin reach it (request-service
# checks again who may plan); an agent never does, because it is absent from agent_rpcs.
test_plan_owners_reach_retry_plan_generation if {
	every role in ["admin", "owner", "reporter"] {
		request.allow with input as object.union(callers[role], {"action": "plan", "rpc": "RetryPlanGeneration", "actor_type": "user"})
	}
}

test_member_and_stranger_do_not_reach_retry_plan_generation if {
	every role in ["member", "stranger"] {
		not request.allow with input as object.union(callers[role], {"action": "plan", "rpc": "RetryPlanGeneration", "actor_type": "user"})
	}
}

test_agent_never_reaches_retry_plan_generation if {
	every role in ["admin", "owner", "reporter", "member"] {
		not request.allow with input as object.union(callers[role], {"action": "plan", "rpc": "RetryPlanGeneration", "actor_type": "agent"})
	}
}

test_retry_plan_generation_is_not_an_agent_rpc if {
	every _, rpcs in request.agent_rpcs {
		not "RetryPlanGeneration" in rpcs
	}
}
