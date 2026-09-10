package orca.authz.tenant

import rego.v1

test_admin_allowed_for_company_edit if {
	allow with input as {"caller_role": "admin", "action": "company_edit", "same_department": false}
}

test_lead_denied_for_company_edit if {
	not allow with input as {"caller_role": "lead", "action": "company_edit", "same_department": false}
}

test_user_denied_for_company_edit if {
	not allow with input as {"caller_role": "user", "action": "company_edit", "same_department": false}
}

test_empty_role_denied_for_company_edit if {
	not allow with input as {"caller_role": "", "action": "company_edit", "same_department": false}
}

test_admin_allowed_for_department_edit if {
	allow with input as {"caller_role": "admin", "action": "department_edit", "same_department": false}
}

test_lead_allowed_for_department_edit_same_department if {
	allow with input as {"caller_role": "lead", "action": "department_edit", "same_department": true}
}

test_lead_denied_for_department_edit_different_department if {
	not allow with input as {"caller_role": "lead", "action": "department_edit", "same_department": false}
}

test_user_denied_for_department_edit if {
	not allow with input as {"caller_role": "user", "action": "department_edit", "same_department": true}
}

test_empty_role_denied_for_department_edit if {
	not allow with input as {"caller_role": "", "action": "department_edit", "same_department": true}
}

test_unknown_action_denied if {
	not allow with input as {"caller_role": "admin", "action": "bogus", "same_department": false}
}

test_missing_input_denied if {
	not allow with input as {}
}
