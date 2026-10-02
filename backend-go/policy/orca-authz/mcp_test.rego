package orca.authz.mcp_test

import data.orca.authz.mcp
import rego.v1

# base builds a fully-populated, allowed input; tests override pieces.
base_in := {
	"user": {"id": "u1", "role": "user"},
	"client": {"id": "c1", "status": "allowed"},
	"token": {"scopes": ["orca:read", "orca:write", "orca:exec", "orca:admin"]},
	"settings": {"enabled": true, "max_depth": 1},
	"killswitch": {"active": false},
	"session": {"depth": 0, "untrusted_read": false},
	"tenant_policies": [],
	"tool": {
		"name": "task_create", "channel": "task.create", "namespace": "task", "risk": "write_reversible",
		"required_scope": "orca:write", "open_world": false, "spawns_process": false,
	},
}

with_tool(t) := object.union(base_in, {"tool": object.union(base_in.tool, t)})

dec(i) := d if {
	d := mcp.decision with input as i
}

test_read_allow if dec(with_tool({"risk": "read", "required_scope": "orca:read"})).decision == "allow"

test_write_allow if dec(base_in).decision == "allow"

test_exec_requires_approval if dec(with_tool({"risk": "exec", "required_scope": "orca:exec"})).decision == "require_approval"

test_destructive_requires_approval if dec(with_tool({"risk": "destructive", "required_scope": "orca:exec"})).decision == "require_approval"

test_admin_risk_denied_for_user if {
	d := dec(with_tool({"risk": "admin", "required_scope": "orca:admin"}))
	d.decision == "deny"
	d.source == "role"
}

test_admin_risk_default_deny_for_admin_user if {
	i := object.union(with_tool({"risk": "admin", "required_scope": "orca:admin"}), {"user": {"id": "a", "role": "admin"}})
	dec(i).decision == "deny"
}

test_admin_risk_policy_allow_for_admin if {
	i := object.union(with_tool({"risk": "admin", "required_scope": "orca:admin"}), {
		"user": {"id": "a", "role": "admin"},
		"tenant_policies": [{"id": "p1", "decision": "allow", "match": {"tool": "task_create"}}],
	})
	dec(i).decision == "allow"
}

test_missing_scope_denied if {
	d := dec(object.union(base_in, {"token": {"scopes": ["orca:read"]}}))
	d.decision == "deny"
	d.source == "scope"
}

test_missing_token_denied if dec(object.remove(base_in, ["token"])).decision == "deny"

test_tenant_disabled_denied if {
	d := dec(object.union(base_in, {"settings": {"enabled": false, "max_depth": 1}}))
	d.source == "tenant_settings"
}

test_missing_settings_denied if dec(object.remove(base_in, ["settings"])).decision == "deny"

test_client_blocked_denied if dec(object.union(base_in, {"client": {"id": "c1", "status": "blocked"}})).source == "client"

test_client_pending_denied if dec(object.union(base_in, {"client": {"id": "c1", "status": "pending"}})).decision == "deny"

test_killswitch_denied if {
	d := dec(object.union(base_in, {"killswitch": {"active": true}}))
	d.source == "kill_switch"
}

test_missing_killswitch_denied if dec(object.remove(base_in, ["killswitch"])).decision == "deny"

test_empty_input_denied if dec({}).decision == "deny"

test_missing_risk_denied if dec(object.remove(base_in, ["tool"])).decision == "deny"

test_missing_risk_field_denied if {
	i := object.union(object.remove(base_in, ["tool"]), {"tool": object.remove(base_in.tool, ["risk"])})
	dec(i).decision == "deny"
}

test_unknown_risk_denied if dec(with_tool({"risk": "weird"})).decision == "deny"

test_missing_channel_denied if {
	i := object.union(object.remove(base_in, ["tool"]), {"tool": object.remove(base_in.tool, ["channel"])})
	dec(i).source == "tool_descriptor"
}

test_hard_deny_credentials_even_with_allow_policy if {
	i := object.union(with_tool({"name": "credentials_get", "channel": "credentials.get", "namespace": "credentials", "risk": "read", "required_scope": "orca:read"}), {
		"tenant_policies": [{"id": "p1", "decision": "allow", "match": {"namespace": "credentials"}}],
	})
	d := dec(i)
	d.decision == "deny"
	d.source == "hard_deny"
}

test_hard_deny_mcp_prefix if dec(with_tool({"channel": "mcp.approval.decide", "risk": "read", "required_scope": "orca:read"})).source == "hard_deny"

test_hard_deny_every_channel if {
	every ch in mcp.hard_deny_channels {
		dec(with_tool({"channel": ch})).source == "hard_deny"
	}
}

test_hard_deny_every_prefix if {
	every p in mcp.hard_deny_prefixes {
		dec(with_tool({"channel": concat("", [p, "anything"])})).source == "hard_deny"
	}
}

test_hard_deny_start_auth_login if dec(with_tool({"channel": "github.startAuthLogin"})).source == "hard_deny"

test_hard_deny_not_substring if dec(with_tool({"channel": "task.mcp.note"})).decision == "allow"

test_policy_tightens_read_to_deny if {
	i := object.union(with_tool({"risk": "read", "required_scope": "orca:read"}), {"tenant_policies": [{"id": "p1", "decision": "deny", "match": {"risk": "read"}}]})
	dec(i).decision == "deny"
}

test_policy_relaxes_write_to_approval if {
	i := object.union(base_in, {"tenant_policies": [{"id": "p1", "decision": "require_approval", "match": {"namespace": "task"}}]})
	dec(i).decision == "require_approval"
}

test_deny_beats_allow_any_specificity if {
	i := object.union(base_in, {"tenant_policies": [
		{"id": "p1", "decision": "allow", "match": {"tool": "task_create"}},
		{"id": "p2", "decision": "deny", "match": {"namespace": "task"}},
	]})
	dec(i).decision == "deny"
}

test_approval_beats_allow if {
	i := object.union(base_in, {"tenant_policies": [
		{"id": "p1", "decision": "allow", "match": {"tool": "task_create"}},
		{"id": "p2", "decision": "require_approval", "match": {"risk": "write_reversible"}},
	]})
	dec(i).decision == "require_approval"
}

test_other_client_policy_ignored if {
	i := object.union(base_in, {"tenant_policies": [{"id": "p1", "decision": "deny", "match": {"clientId": "other"}}]})
	dec(i).decision == "allow"
}

test_roles_dimension if {
	i := object.union(base_in, {"tenant_policies": [{"id": "p1", "decision": "deny", "match": {"roles": ["user"]}}]})
	dec(i).decision == "deny"
}

test_roles_dimension_other_role_ignored if {
	i := object.union(base_in, {"tenant_policies": [{"id": "p1", "decision": "deny", "match": {"roles": ["admin"]}}]})
	dec(i).decision == "allow"
}

test_empty_match_never_matches if {
	i := object.union(base_in, {"tenant_policies": [{"id": "p1", "decision": "deny", "match": {}}]})
	dec(i).decision == "allow"
}

test_unknown_policy_decision_is_deny if {
	i := object.union(base_in, {"tenant_policies": [{"id": "p1", "decision": "maybe", "match": {"tool": "task_create"}}]})
	dec(i).decision == "deny"
}

test_wildcard_exec_allow_clamped if {
	i := object.union(with_tool({"risk": "exec", "required_scope": "orca:exec"}), {"tenant_policies": [{"id": "p1", "decision": "allow", "match": {"risk": "exec"}}]})
	d := dec(i)
	d.decision == "require_approval"
	"risk_floor:exact_tool_policy_required" in d.reasons
}

test_exact_tool_exec_allow_kept if {
	i := object.union(with_tool({"risk": "exec", "required_scope": "orca:exec"}), {"tenant_policies": [{"id": "p1", "decision": "allow", "match": {"tool": "task_create"}}]})
	dec(i).decision == "allow"
}

test_open_world_clean_session_allowed if dec(with_tool({"open_world": true})).decision == "allow"

test_open_world_tainted_requires_approval if {
	i := object.union(with_tool({"open_world": true}), {"session": {"depth": 0, "untrusted_read": true}})
	d := dec(i)
	d.decision == "require_approval"
	"open_world_after_untrusted_read" in d.reasons
}

test_closed_world_tainted_allowed if {
	i := object.union(base_in, {"session": {"depth": 0, "untrusted_read": true}})
	dec(i).decision == "allow"
}

test_open_world_deny_stays_deny if {
	i := object.union(with_tool({"open_world": true}), {
		"session": {"depth": 0, "untrusted_read": true},
		"tenant_policies": [{"id": "p1", "decision": "deny", "match": {"tool": "task_create"}}],
	})
	dec(i).decision == "deny"
}

test_missing_open_world_flag_treated_open if {
	i := object.union(object.remove(base_in, ["tool"]), {
		"tool": object.remove(base_in.tool, ["open_world"]),
		"session": {"depth": 0, "untrusted_read": true},
	})
	dec(i).decision == "require_approval"
}

test_spawn_denied_at_max_depth if {
	i := object.union(with_tool({"spawns_process": true}), {"session": {"depth": 1, "untrusted_read": false}})
	dec(i).source == "recursion"
}

test_spawn_allowed_below_max_depth if dec(with_tool({"spawns_process": true})).decision == "allow"

test_non_spawning_unaffected_by_depth if {
	i := object.union(base_in, {"session": {"depth": 5, "untrusted_read": false}})
	dec(i).decision == "allow"
}

test_hard_denied_flag_for_catalog if {
	mcp.hard_denied with input as {"tool": {"channel": "team.create"}}
	not mcp.hard_denied with input as {"tool": {"channel": "task.create"}}
}

test_empty_channel_denied if dec(with_tool({"channel": ""})).source == "tool_descriptor"
