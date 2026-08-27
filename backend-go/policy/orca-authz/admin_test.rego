package orca.authz.admin

import rego.v1

test_admin_allowed if {
	allow with input as {"actor": {"role": "admin", "id": "u1"}}
}

test_non_admin_denied if {
	not allow with input as {"actor": {"role": "user", "id": "u1"}}
}

test_missing_actor_denied if {
	not allow with input as {}
}
