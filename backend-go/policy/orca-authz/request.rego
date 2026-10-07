package orca.authz.request

import rego.v1

# input shape:
# {
#   "action": <string>,             # read, create, triage, solution, decide, execute, admin, lifecycle, plan, authenticated
#   "rpc": <string>,                # The specific gRPC method name being called
#   "caller_global_role": <string>, # "admin" or ""
#   "caller_project_role": <string>,# "owner", "member", or ""
#   "is_reporter": <boolean>,       # true if the caller is the reporter of the request
#   "actor_type": <string>          # "user", "agent", "system"
# }
# Note: "decide" and "authenticated" actions are just front-doors. Actual approvers
# are evaluated in Go because they depend on data states. The "admin" action only applies
# to global admins. "internal" action is never checked here (it's handled by Guard).

# group_roles maps an action to the set of caller_roles allowed to perform it.
group_roles := {
	"read": {"owner", "member", "reporter"},
	"create": {"owner", "member"},
	"triage": {"owner", "reporter"},
	"solution": {"owner", "member"},
	"decide": {"owner"},
	"execute": {"owner", "member"},
	"admin": {},
	"lifecycle": {"owner"},
	"plan": {"owner", "member"},
	"authenticated": {"owner", "member", "reporter", "stranger"},
}

# agent_rpcs maps an action to the set of RPCs an agent is allowed to call for that action.
agent_rpcs := {
	"read": {
		"GetRequest", "ListRequests", "ListRequestTimeline", 
		"GetSolution", "ListSolutions", "ListRequestLinks", "CheckSecretScanInfo",
	},
	"create": {
		"CreateRequest", "SpawnChildRequest",
	},
	"triage": {
		"ClassifyRequest",
	},
	"solution": {
		"ProposeSolution", "UpdateSolution", "MarkSolutionApproved",
	},
	"execute": {
		"RecordExecutionPlan", "RecordExecutionResult",
	},
}

# Build the set of roles the caller holds.
caller_roles contains role if {
	role := input.caller_project_role
	role != ""
}

caller_roles contains "reporter" if {
	input.is_reporter == true
}

caller_roles contains "stranger" if {
	input.caller_project_role == ""
	input.is_reporter == false
}

default allow := false

# Rule 1: Global admin is allowed for all actions EXCEPT when actor_type is agent 
# and the action forbids agents (decide, execute, admin, lifecycle, plan)
allow if {
	input.caller_global_role == "admin"
	input.actor_type != "agent"
}

# Rule 2: Global admin using agent is restricted to agent_rpcs for the allowed actions.
allow if {
	input.caller_global_role == "admin"
	input.actor_type == "agent"
	agent_allowed(input.action, input.rpc)
}

# Rule 3: Regular user check against group_roles.
allow if {
	input.actor_type != "agent"
	roles := group_roles[input.action]
	count(caller_roles & roles) > 0
}

# Rule 4: Agent check against group_roles and agent_rpcs.
allow if {
	input.actor_type == "agent"
	roles := group_roles[input.action]
	count(caller_roles & roles) > 0
	agent_allowed(input.action, input.rpc)
}

# Helper to check if agent is allowed to call this RPC for this action.
agent_allowed(action, rpc) if {
	allowed_rpcs := agent_rpcs[action]
	rpc in allowed_rpcs
}
