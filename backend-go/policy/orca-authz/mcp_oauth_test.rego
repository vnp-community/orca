package orca.authz.mcp_oauth

import rego.v1

test_user_revokes_own_grant if {
	allow with input as {"action": "grant.revoke", "subject": {"user_id": "u1", "role": "user"}, "resource": {"user_id": "u1"}}
}

test_user_cannot_revoke_others_grant if {
	not allow with input as {"action": "grant.revoke", "subject": {"user_id": "u1", "role": "user"}, "resource": {"user_id": "u2"}}
}

test_admin_actions_need_admin_role if {
	allow with input as {"action": "admin.client.setStatus", "subject": {"user_id": "u1", "role": "admin"}}
	allow with input as {"action": "admin.grant.revoke", "subject": {"user_id": "u1", "role": "admin"}}
	not allow with input as {"action": "admin.client.list", "subject": {"user_id": "u1", "role": "user"}}
}

test_missing_role_fails_closed if {
	not allow with input as {"action": "admin.client.list", "subject": {"user_id": "u1"}}
}

test_unknown_action_denied_even_for_admin if {
	not allow with input as {"action": "tenant.delete", "subject": {"user_id": "u1", "role": "admin"}}
}

test_user_with_matching_id_cannot_use_admin_prefix if {
	not allow with input as {"action": "admin.grant.revoke", "subject": {"user_id": "u1", "role": "user"}, "resource": {"user_id": "u1"}}
}
