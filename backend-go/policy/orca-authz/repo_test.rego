package orca.authz.repo

import rego.v1

test_owner_allowed_regardless_of_repo_role if {
	allow with input as {"caller_project_role": "owner", "caller_repo_role": "", "caller_global_role": "", "action": "repo_admin_only"}
}

test_member_with_no_repo_grant_denied_for_admin_only if {
	not allow with input as {"caller_project_role": "member", "caller_repo_role": "", "caller_global_role": "", "action": "repo_admin_only"}
}

test_member_with_developer_grant_denied_for_admin_only if {
	not allow with input as {"caller_project_role": "member", "caller_repo_role": "developer", "caller_global_role": "", "action": "repo_admin_only"}
}

test_member_with_admin_grant_allowed_for_admin_only if {
	allow with input as {"caller_project_role": "member", "caller_repo_role": "admin", "caller_global_role": "", "action": "repo_admin_only"}
}

test_member_with_developer_grant_denied_for_lead_or_admin if {
	not allow with input as {"caller_project_role": "member", "caller_repo_role": "developer", "caller_global_role": "", "action": "repo_lead_or_admin"}
}

test_member_with_lead_grant_allowed_for_lead_or_admin if {
	allow with input as {"caller_project_role": "member", "caller_repo_role": "lead", "caller_global_role": "", "action": "repo_lead_or_admin"}
}

test_member_with_admin_grant_allowed_for_lead_or_admin if {
	allow with input as {"caller_project_role": "member", "caller_repo_role": "admin", "caller_global_role": "", "action": "repo_lead_or_admin"}
}

test_member_with_developer_grant_allowed_for_any_functional_role if {
	allow with input as {"caller_project_role": "member", "caller_repo_role": "developer", "caller_global_role": "", "action": "repo_any_functional_role"}
}

test_member_with_no_repo_grant_denied_for_any_functional_role if {
	not allow with input as {"caller_project_role": "member", "caller_repo_role": "", "caller_global_role": "", "action": "repo_any_functional_role"}
}

test_global_admin_allowed_regardless_of_project_or_repo_role if {
	allow with input as {"caller_project_role": "", "caller_repo_role": "", "caller_global_role": "admin", "action": "repo_admin_only"}
}

test_unknown_action_denied_for_non_owner if {
	not allow with input as {"caller_project_role": "member", "caller_repo_role": "admin", "caller_global_role": "", "action": "bogus"}
}

test_missing_input_denied if {
	not allow with input as {}
}
