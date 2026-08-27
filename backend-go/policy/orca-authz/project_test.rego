package orca.authz.project

import rego.v1

test_owner_allowed_for_owner_only if {
	allow with input as {"caller_project_role": "owner", "caller_global_role": "", "action": "owner_only"}
}

test_member_denied_for_owner_only if {
	not allow with input as {"caller_project_role": "member", "caller_global_role": "", "action": "owner_only"}
}

test_no_membership_denied_for_owner_only if {
	not allow with input as {"caller_project_role": "", "caller_global_role": "", "action": "owner_only"}
}

test_global_admin_allowed_for_owner_only_regardless_of_project_role if {
	allow with input as {"caller_project_role": "", "caller_global_role": "admin", "action": "owner_only"}
}

test_owner_allowed_for_any_member if {
	allow with input as {"caller_project_role": "owner", "caller_global_role": "", "action": "any_member"}
}

test_member_allowed_for_any_member if {
	allow with input as {"caller_project_role": "member", "caller_global_role": "", "action": "any_member"}
}

test_no_membership_denied_for_any_member if {
	not allow with input as {"caller_project_role": "", "caller_global_role": "", "action": "any_member"}
}

test_global_admin_allowed_for_any_member_regardless_of_project_role if {
	allow with input as {"caller_project_role": "", "caller_global_role": "admin", "action": "any_member"}
}

test_unknown_action_denied if {
	not allow with input as {"caller_project_role": "owner", "caller_global_role": "", "action": "bogus"}
}

test_missing_input_denied if {
	not allow with input as {}
}
