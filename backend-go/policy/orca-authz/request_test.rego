package orca.authz.request_test

import rego.v1

import data.orca.authz.request

# Roles of the matrix; "stranger" has neither a project role nor the reporter flag.
callers := {
	"admin": {"caller_global_role": "admin", "caller_project_role": "", "is_reporter": false},
	"owner": {"caller_global_role": "", "caller_project_role": "owner", "is_reporter": false},
	"member": {"caller_global_role": "", "caller_project_role": "member", "is_reporter": false},
	"reporter": {"caller_global_role": "", "caller_project_role": "", "is_reporter": true},
	"stranger": {"caller_global_role": "", "caller_project_role": "", "is_reporter": false},
}

# Expected matrix (design table of TASK-REQ-035-03): group -> roles that pass for a person.
want := {
	"read": {"admin", "owner", "member", "reporter"},
	"create": {"admin", "owner", "member"},
	"triage": {"admin", "owner", "reporter"},
	"analyze": {"admin", "owner", "reporter"},
	"plan": {"admin", "owner", "reporter"},
	"execute": {"admin", "owner"},
	"lifecycle": {"admin", "owner", "reporter"},
	"admin": {"admin"},
	"decide": {"admin", "owner", "member", "reporter", "stranger"},
	"authenticated": {"admin", "owner", "member", "reporter", "stranger"},
}

person_input(group, role) := object.union(callers[role], {"action": group, "rpc": "Any", "actor_type": "user"})

test_user_matrix_every_group_and_role if {
	bad := {sprintf("%s/%s", [group, role]) |
		some group, roles in want
		some role, _ in callers
		request.allow with input as person_input(group, role)
		not role in roles
	}
	missing := {sprintf("%s/%s", [group, role]) |
		some group, roles in want
		some role in roles
		not request.allow with input as person_input(group, role)
	}
	count(bad) == 0
	count(missing) == 0
}

agent_input(group, rpc, role) := object.union(callers[role], {"action": group, "rpc": rpc, "actor_type": "agent"})

test_agent_listed_rpc_allowed_for_owner if {
	every group, rpcs in request.agent_rpcs {
		every rpc in rpcs {
			# Groups that need a project role are checked as owner; "authenticated" needs none.
			request.allow with input as agent_input(group, rpc, "owner")
		}
	}
}

test_agent_listed_rpc_allowed_for_admin_user if {
	every group, rpcs in request.agent_rpcs {
		every rpc in rpcs {
			request.allow with input as agent_input(group, rpc, "admin")
		}
	}
}

test_agent_listed_rpc_denied_for_stranger_except_front_door if {
	allowed := {sprintf("%s/%s", [group, rpc]) |
		some group, rpcs in request.agent_rpcs
		some rpc in rpcs
		request.allow with input as agent_input(group, rpc, "stranger")
	}
	every key in allowed {
		startswith(key, "authenticated/")
	}
}

test_agent_unlisted_rpc_denied_even_for_admin if {
	every case in [
		{"action": "execute", "rpc": "StartPhase"},
		{"action": "decide", "rpc": "Approve"},
		{"action": "decide", "rpc": "Reject"},
		{"action": "admin", "rpc": "SetRequestFlowSettings"},
		{"action": "plan", "rpc": "GeneratePlan"},
		{"action": "lifecycle", "rpc": "CancelRequest"},
		{"action": "admin", "rpc": "EraseRequest"},
		{"action": "admin", "rpc": "ExportRequest"},
		{"action": "triage", "rpc": "ConfirmRequestType"},
	] {
		not request.allow with input as object.union(callers.admin, {"action": case.action, "rpc": case.rpc, "actor_type": "agent"})
		not request.allow with input as object.union(callers.owner, {"action": case.action, "rpc": case.rpc, "actor_type": "agent"})
	}
}

test_agent_listed_rpc_in_wrong_group_denied if {
	not request.allow with input as agent_input("execute", "CreateRequest", "owner")
	not request.allow with input as agent_input("read", "GenerateSolution", "owner")
}

test_agent_cannot_use_decide_front_door if {
	not request.allow with input as agent_input("decide", "Approve", "stranger")
	not request.allow with input as agent_input("decide", "Reject", "admin")
}

test_empty_roles_denied_for_every_role_group if {
	every group in ["read", "create", "triage", "analyze", "plan", "execute", "lifecycle", "admin"] {
		not request.allow with input as person_input(group, "stranger")
	}
}

test_unknown_action_denied if {
	not request.allow with input as person_input("bogus", "stranger")
	not request.allow with input as person_input("bogus", "owner")
	not request.allow with input as agent_input("bogus", "GetRequest", "owner")
}

test_decide_front_door_for_person_without_role if {
	request.allow with input as person_input("decide", "stranger")
	not request.allow with input as agent_input("decide", "Approve", "owner")
}

test_authenticated_front_door_person_and_listed_agent if {
	request.allow with input as person_input("authenticated", "stranger")
	request.allow with input as agent_input("authenticated", "ListPendingForUser", "stranger")
	not request.allow with input as agent_input("authenticated", "SomethingElse", "stranger")
}

test_missing_actor_type_is_not_agent_but_roles_still_needed if {
	not request.allow with input as {"action": "read", "rpc": "GetRequest", "caller_global_role": "", "caller_project_role": "", "is_reporter": false}
	request.allow with input as {"action": "read", "rpc": "GetRequest", "caller_global_role": "", "caller_project_role": "member", "is_reporter": false}
}

test_system_actor_follows_person_rules if {
	not request.allow with input as object.union(callers.stranger, {"action": "read", "rpc": "GetRequest", "actor_type": "system"})
}
