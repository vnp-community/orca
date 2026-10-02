# MCP OAuth grant/client management (BE-MCP-SOL-005). mcp-service enforces the
# same rule in Go today (usecase.callerIdentity.requireAdmin plus owner-scoped
# queries); this bundle is the single written statement of it so both stay in
# step when mcp-service moves to common/policy.Evaluator.
#
# input shape: {"action": <string>, "subject": {"user_id", "role"},
#               "resource": {"user_id"}}
# Consumed as data.orca.authz.mcp_oauth.allow.
package orca.authz.mcp_oauth

import rego.v1

default allow := false

# A user may revoke only their own grant.
allow if {
	input.action == "grant.revoke"
	input.resource.user_id == input.subject.user_id
}

# Everything under admin.* (client list/setStatus, listing or revoking other
# users' grants) needs the admin role. An absent role never matches.
allow if {
	startswith(input.action, "admin.")
	input.subject.role == "admin"
}
