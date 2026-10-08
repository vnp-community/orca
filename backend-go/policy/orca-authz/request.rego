# Request-level authorization (CR-REQ-035 2.3). Consumed as data.orca.authz.request.allow by
# request-service through common/policy.Evaluator, same shape as project.rego.
#
# input: {action, rpc, caller_global_role, caller_project_role, is_reporter, actor_type}
#   action              group of the RPC from request-service's rpc_catalog (read, create, ...)
#   rpc                 short RPC name, used only to bound what an agent may call
#   caller_project_role "owner" | "member" | "" (project-service membership)
#   is_reporter         caller is requests.reporter_id of the target Request
#   actor_type          "user" | "agent" | "system"
#
# Boundaries: "decide" and "authenticated" are front doors only (the approver set depends on data
# and stays in Go, CR-REQ-010); "admin" has no project role, so only a global admin passes;
# "internal" never reaches this file (internalcaller.Guard handles it).
package orca.authz.request

import rego.v1

# group -> project roles allowed. "reporter" is the is_reporter flag, not a project role.
group_roles := {
	"read": {"owner", "member", "reporter"},
	"create": {"owner", "member"},
	"triage": {"owner", "reporter"},
	"analyze": {"owner", "reporter"},
	"plan": {"owner", "reporter"},
	"execute": {"owner"},
	"lifecycle": {"owner", "reporter"},
}

# group -> RPCs an agent (MCP session) may call. Nothing else is allowed for an agent, even when
# the user behind it is a global admin: the gates exist so that a person controls the agent.
agent_rpcs := {
	"read": {
		"GetRequest", "ListRequests", "ListBacklog", "ListRequestTypeHistory", "ListRequestLinks",
		"ListSolutions", "GetApproval", "ListApprovals",
	},
	"create": {"CreateRequest", "SpawnChildRequest"},
	"triage": {"ClassifyRequest", "ChangeRequestType"},
	"analyze": {"GenerateSolution"},
	"lifecycle": {"ReturnToBacklog", "ReopenRequest"},
	"authenticated": {"GetRequestFlow", "GetRequestFlowSettings", "ListPendingForUser"},
}

caller_roles contains role if {
	role := input.caller_project_role
	role != ""
}

caller_roles contains "reporter" if input.is_reporter == true

default allow := false

agent_allowed if input.rpc in agent_rpcs[input.action]

# A missing actor_type counts as a person, like tenant.ActorType on the Go side.
is_agent if input.actor_type == "agent"

human if not is_agent

allow if {
	human
	input.caller_global_role == "admin"
}

allow if {
	human
	some role in caller_roles
	role in group_roles[input.action]
}

allow if {
	is_agent
	agent_allowed
	input.caller_global_role == "admin"
}

allow if {
	is_agent
	agent_allowed
	some role in caller_roles
	role in group_roles[input.action]
}

# Front doors: any authenticated person; the Go side decides who may really approve.
allow if {
	human
	input.action in {"decide", "authenticated"}
}

# Agents may only reach the read-only "authenticated" RPCs listed in agent_rpcs.
allow if {
	is_agent
	input.action == "authenticated"
	agent_allowed
}
