# task-grant final decision — task-service.ResolvePermission's BFS ancestor
# walk (internal/domain/grant_resolution.go) computes the resolved
# GrantLevel; this policy is the "does this level authorize this action"
# decision the design doc's domain-computes/OPA-decides split assigns to
# OPA, not to the graph walk itself (execution-plan.md Epic E).
# Consumed as data.orca.authz.task.allow via common/policy.Evaluator.
#
# input shape: {"level": <string>, "action": <string>, "tenant_id": <string>}
# level is one of task-service's domain.GrantLevel values
# ("owner"|"admin"|"user"|"team"|"company"), lowercased; "unspecified"
# (grant_resolution.go's not-found sentinel) matches no row below and is
# always denied.
package orca.authz.task

import rego.v1

# First-cut permission matrix (Epic E's "first policies" — this is a
# starting point, not a final product taxonomy). Maps a resolved
# GrantLevel to the set of actions it authorizes. "read" is allowed at
# every known level, including the weakest (company-wide) grant, since a
# grant that exists at all implies at least visibility;
# GrantLevel's own priority order (owner > admin > user > team > company,
# per grant_resolution.go) is mirrored here as a widening/narrowing action
# set, not re-derived.
level_actions := {
	# "manage" (added by TASK-TG-03-01) is the action Grant/RevokeGrant/
	# ListGrants require — only owner/admin may write a grant on a task
	# they can already administer; a plain "user"-level grantee still
	# cannot re-grant access to others.
	"owner": {"read", "write", "execute", "admin", "manage"},
	"admin": {"read", "write", "execute", "admin", "manage"},
	"user": {"read", "write", "execute"},
	"team": {"read", "write"},
	"company": {"read"},
}

default allow := false

allow if {
	actions := level_actions[input.level]
	input.action in actions
}
