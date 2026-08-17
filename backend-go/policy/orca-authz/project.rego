# project-role/global-admin authorization — project-service.md §9's matrix:
# owner-only actions (UpdateProject/DeleteProject/AddMember/RebindDevServer,
# plus this service's judgment-call extension to the repo-catalog mutation
# RPCs AddRepo/RemoveRepo/ReorderRepos — see project-service's README "Known
# gaps") require the caller's project role to be "owner", or the caller to
# be a global admin; any-membership actions (GetProject/ListRepos/
# ListWorktrees) accept "owner" or "member" (the fuller owner/member/viewer
# model is a documented follow-up — see domain.ProjectRole's doc comment),
# or global admin. Consumed as data.orca.authz.project.allow via
# common/policy.Evaluator.
#
# input shape: {"caller_project_role": <string>, "caller_global_role": <string>, "action": <string>}
# caller_project_role is one of project-service's domain.ProjectRole values
# ("owner"|"member"), or "" when the caller has no membership row for the
# project in question. action is one of "owner_only"|"any_member".
package orca.authz.project

import rego.v1

# action -> the set of caller_project_role values it authorizes, independent
# of caller_global_role — mirrors task_grant.rego's level_actions table
# shape for the same reason: one small lookup table beats a chain of
# per-action rules that can drift out of sync with each other.
action_roles := {
	"owner_only": {"owner"},
	"any_member": {"owner", "member"},
}

default allow := false

# A global admin is authorized for every action regardless of project
# membership — project-service.md §9's "owner role OR global admin" clause.
allow if {
	input.caller_global_role == "admin"
}

allow if {
	roles := action_roles[input.action]
	input.caller_project_role in roles
}
