# MCP tool-call gate (BE-MCP-SOL-012). Tenant policies arrive through
# input.tenant_policies (DB + RLS), never through this bundle, so this file is
# static: hard-deny and risk defaults are only changeable by code review.
package orca.authz.mcp

import rego.v1

risk_default := {
	"read": "allow",
	"write_reversible": "allow",
	"exec": "require_approval",
	"destructive": "require_approval",
	"admin": "deny",
}

strictness := {"allow": 0, "require_approval": 1, "deny": 2}

hard_deny_prefixes := {"credentials.", "auth.", "mcp."}

hard_deny_channels := {
	"admin.createUser",
	"admin.updateUserRole",
	"admin.deactivateUser",
	"admin.reactivateUser",
	"admin.forceRevokeSession",
	"admin.forceRevokeAllSessions",
	"admin.createPolicy",
	"admin.updatePolicy",
	"admin.deletePolicy",
	"admin.getPolicy",
	"admin.listPolicies",
	"aiProvider.writeCredential",
	"devServer.agentTokens.create",
	"devServer.agentTokens.revoke",
	"devServer.agentTokens.list",
	"team.create",
	"team.addMember",
	"team.removeMember",
	"profile.createCompany",
	"profile.updateCompany",
	"profile.updateUser",
}

hard_deny_patterns := {`\.start(Cli)?AuthLogin$`}

default hard_denied := false

hard_denied if {
	some prefix in hard_deny_prefixes
	startswith(input.tool.channel, prefix)
}

hard_denied if input.tool.channel in hard_deny_channels

hard_denied if {
	some pattern in hard_deny_patterns
	regex.match(pattern, input.tool.channel)
}

channel_known if input.tool.channel != ""

tenant_enabled if input.settings.enabled == true

client_allowed if input.client.status == "allowed"

scope_ok if input.tool.required_scope in input.token.scopes

user_is_admin if input.user.role == "admin"

depth_ok if input.session.depth < input.settings.max_depth

killswitch_clear if input.killswitch.active == false

session_clean if input.session.untrusted_read == false

tool_closed_world if input.tool.open_world == false

tool_not_spawning if input.tool.spawns_process == false

has_dim(m) if m.tool

has_dim(m) if m.namespace

has_dim(m) if m.risk

has_dim(m) if m.clientId

has_dim(m) if m.roles

match_tool(m) if not m.tool

match_tool(m) if m.tool == input.tool.name

match_namespace(m) if not m.namespace

match_namespace(m) if m.namespace == input.tool.namespace

match_risk(m) if not m.risk

match_risk(m) if m.risk == input.tool.risk

match_client(m) if not m.clientId

match_client(m) if m.clientId == input.client.id

match_roles(m) if not m.roles

match_roles(m) if input.user.role in m.roles

policy_matches(m) if {
	has_dim(m)
	match_tool(m)
	match_namespace(m)
	match_risk(m)
	match_client(m)
	match_roles(m)
}

matched := [p |
	some p in input.tenant_policies
	policy_matches(p.match)
]

# Strictest wins: deny > require_approval > allow, regardless of specificity.
policy_decision := d if {
	count(matched) > 0
	rank := max([object.get(strictness, p.decision, 2) | some p in matched])
	some d, r in strictness
	r == rank
}

base := object.get(risk_default, input.tool.risk, "deny")

pre := policy_decision if count(matched) > 0

pre := base if count(matched) == 0

exact_tool_allow if {
	some p in matched
	p.match.tool == input.tool.name
	p.decision == "allow"
}

# exec/destructive may only be relaxed to allow by an exact-tool policy.
floor_exact_applies if {
	pre == "allow"
	input.tool.risk in {"exec", "destructive"}
	not exact_tool_allow
}

step1 := "require_approval" if floor_exact_applies

step1 := pre if not floor_exact_applies

# Open-world tool after an untrusted read in the same (user, client) window.
floor_openworld_applies if {
	step1 == "allow"
	not tool_closed_world
	not session_clean
}

step2 := "require_approval" if floor_openworld_applies

step2 := step1 if not floor_openworld_applies

decision_source := "tenant_policy" if count(matched) > 0

decision_source := "default" if count(matched) == 0

policy_reasons := [sprintf("policy:%s:%s", [p.id, p.decision]) | some p in matched] if count(matched) > 0

policy_reasons := [sprintf("risk_default:%s=%s", [input.tool.risk, base])] if count(matched) == 0

floor_reasons := array.concat(
	[r | floor_exact_applies; r := "risk_floor:exact_tool_policy_required"],
	[r | floor_openworld_applies; r := "open_world_after_untrusted_read"],
)

default decision := {"decision": "deny", "source": "default", "reasons": ["policy_undefined"]}

decision := {"decision": "deny", "source": "kill_switch", "reasons": ["kill_switch_active"]} if {
	not killswitch_clear
} else := {"decision": "deny", "source": "hard_deny", "reasons": ["hard_deny"]} if {
	hard_denied
} else := {"decision": "deny", "source": "tool_descriptor", "reasons": ["tool_descriptor_incomplete"]} if {
	not channel_known
} else := {"decision": "deny", "source": "tenant_settings", "reasons": ["mcp_disabled_for_tenant"]} if {
	not tenant_enabled
} else := {"decision": "deny", "source": "client", "reasons": ["client_not_allowed"]} if {
	not client_allowed
} else := {"decision": "deny", "source": "scope", "reasons": ["scope_missing"]} if {
	not scope_ok
} else := {"decision": "deny", "source": "role", "reasons": ["admin_role_required"]} if {
	input.tool.risk == "admin"
	not user_is_admin
} else := {"decision": "deny", "source": "recursion", "reasons": ["depth_limit"]} if {
	not tool_not_spawning
	not depth_ok
} else := {"decision": step2, "source": decision_source, "reasons": array.concat(policy_reasons, floor_reasons)}
