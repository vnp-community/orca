package orca.authz.annotation

import rego.v1

test_author_can_edit if {
	allow with input as {"actor_id": "u1", "author_id": "u1", "actor_role": "user"}
}

test_admin_override if {
	allow with input as {"actor_id": "u2", "author_id": "u1", "actor_role": "admin"}
}

test_non_author_non_admin_denied if {
	not allow with input as {"actor_id": "u2", "author_id": "u1", "actor_role": "user"}
}
