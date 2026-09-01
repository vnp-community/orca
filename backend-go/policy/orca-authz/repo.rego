# repo-level functional-role authorization — layered on top of
# project.rego's owner/member tier. project.rego decides who's in a project
# at all; this decides what a project member can do on one specific repo
# within it (a "developer" grant on repo X doesn't carry over to repo Y in
# the same project). Consumed as data.orca.authz.repo.allow via
# common/policy.Evaluator.
#
# input shape: {"caller_project_role": <string>, "caller_repo_role": <string>,
#               "caller_global_role": <string>, "action": <string>}
# caller_project_role is one of project-service's domain.ProjectRole values
# ("owner"|"member"), or "" when the caller has no project membership row at
# all. caller_repo_role is one of domain.RepoRole's values
# ("developer"|"lead"|"admin"), or "" when the caller has no repo_members
# grant for this specific repo. action is one of "repo_admin_only"|
# "repo_lead_or_admin"|"repo_any_functional_role".
package orca.authz.repo

import rego.v1

# action -> the set of caller_repo_role values it authorizes for a non-owner
# project member — mirrors project.rego's action_roles table shape.
action_roles := {
	"repo_admin_only": {"admin"},
	"repo_lead_or_admin": {"lead", "admin"},
	"repo_any_functional_role": {"developer", "lead", "admin"},
}

default allow := false

# A global admin is authorized for every action regardless of any role.
allow if {
	input.caller_global_role == "admin"
}

# A project owner is always authorized on their own project's repos,
# regardless of caller_repo_role (including no repo_members grant at all —
# repo_members is opt-in, an owner's access never depends on holding one).
allow if {
	input.caller_project_role == "owner"
}

# A non-owner project member needs a repo_members grant matching the
# action's tier — caller_project_role alone (e.g. plain "member") is never
# sufficient by itself, only caller_repo_role is checked here.
allow if {
	roles := action_roles[input.action]
	input.caller_repo_role in roles
}
