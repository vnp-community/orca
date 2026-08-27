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
