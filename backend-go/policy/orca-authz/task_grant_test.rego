package orca.authz.task

import rego.v1

test_owner_can_admin if {
	allow with input as {"level": "owner", "action": "admin"}
}

test_company_can_read if {
	allow with input as {"level": "company", "action": "read"}
}

test_company_cannot_write if {
	not allow with input as {"level": "company", "action": "write"}
}

test_team_can_write if {
	allow with input as {"level": "team", "action": "write"}
}

test_team_cannot_execute if {
	not allow with input as {"level": "team", "action": "execute"}
}

test_unspecified_level_denied if {
	not allow with input as {"level": "unspecified", "action": "read"}
}

test_unknown_action_denied if {
	not allow with input as {"level": "owner", "action": "delete_tenant"}
}

# TASK-TG-03-01: Grant/RevokeGrant/ListGrants require "manage" — only
# owner/admin may write a grant on a task they can already administer.
test_owner_can_manage if {
	allow with input as {"level": "owner", "action": "manage"}
}

test_admin_can_manage if {
	allow with input as {"level": "admin", "action": "manage"}
}

test_user_cannot_manage if {
	not allow with input as {"level": "user", "action": "manage"}
}

test_team_cannot_manage if {
	not allow with input as {"level": "team", "action": "manage"}
}

test_company_cannot_manage if {
	not allow with input as {"level": "company", "action": "manage"}
}
