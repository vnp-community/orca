package orca.authz.request_test

import rego.v1
import data.orca.authz.request.allow

test_owner_triage if {
	allow with input as {"action": "triage", "caller_project_role": "owner", "is_reporter": false, "actor_type": "user"}
}

test_reporter_triage if {
	allow with input as {"action": "triage", "caller_project_role": "", "is_reporter": true, "actor_type": "user"}
}

test_member_triage_denied if {
	not allow with input as {"action": "triage", "caller_project_role": "member", "is_reporter": false, "actor_type": "user"}
}

test_stranger_triage_denied if {
	not allow with input as {"action": "triage", "caller_project_role": "", "is_reporter": false, "actor_type": "user"}
}

test_agent_classify_allowed_for_owner if {
	allow with input as {"action": "triage", "rpc": "ClassifyRequest", "caller_project_role": "owner", "is_reporter": false, "actor_type": "agent"}
}

test_agent_classify_denied_for_stranger if {
	not allow with input as {"action": "triage", "rpc": "ClassifyRequest", "caller_project_role": "", "is_reporter": false, "actor_type": "agent"}
}

test_agent_approve_denied if {
	not allow with input as {"action": "decide", "rpc": "Approve", "caller_project_role": "owner", "is_reporter": false, "actor_type": "agent"}
}

test_agent_denied_plan_even_admin if {
	not allow with input as {"action": "plan", "rpc": "GeneratePlan", "caller_global_role": "admin", "caller_project_role": "", "is_reporter": false, "actor_type": "agent"}
}

test_empty_roles_denied if {
	not allow with input as {"action": "read", "caller_global_role": "", "caller_project_role": "", "is_reporter": false, "actor_type": "user"}
}

test_bogus_action_denied if {
	not allow with input as {"action": "bogus", "caller_project_role": "owner", "is_reporter": false, "actor_type": "user"}
}

test_decide_user_allowed_agent_denied if {
	allow with input as {"action": "decide", "caller_project_role": "owner", "is_reporter": false, "actor_type": "user"}
	not allow with input as {"action": "decide", "caller_project_role": "owner", "is_reporter": false, "actor_type": "agent"}
}

test_authenticated_frontdoor if {
	allow with input as {"action": "authenticated", "caller_project_role": "", "is_reporter": false, "actor_type": "user"}
	not allow with input as {"action": "authenticated", "caller_project_role": "", "is_reporter": false, "actor_type": "agent"}
}
