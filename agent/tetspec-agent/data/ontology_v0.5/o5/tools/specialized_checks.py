"""Typed semantic checks for specialist TDD design contracts."""
from __future__ import annotations

from typing import Any
from quality5 import execute, known


def _props(node):
    return node.get("props", {})


def check_transaction(graph):
    errors = []
    for node in graph.get("elements", []):
        p = _props(node)
        if node.get("class") == "Transaction":
            for field in ("boundary", "isolation", "consistency"):
                if not known(p.get(field)): errors.append(f"{node.get('id')}:{field}")
        if node.get("class") == "Operation" and p.get("transactional"):
            for field in ("timeout", "retry", "idempotency", "failure_policy"):
                if not known(p.get(field)): errors.append(f"{node.get('id')}:{field}")
    return errors


def check_migration(graph):
    errors = []
    for node in graph.get("elements", []):
        if node.get("class") != "Migration": continue
        p = _props(node)
        for field in ("stages", "backfill", "lock", "rollback"):
            if not known(p.get(field)): errors.append(f"{node.get('id')}:{field}")
        stages = p.get("stages")
        if isinstance(stages, list):
            ids = {str(x.get("id")) for x in stages if isinstance(x, dict)}
            for stage in stages:
                for dep in stage.get("depends_on", []) if isinstance(stage, dict) else []:
                    if str(dep) not in ids: errors.append(f"{node.get('id')}:dangling_stage:{dep}")
            visiting, visited = set(), set()
            def visit(ident):
                if ident in visiting: return True
                if ident in visited: return False
                visiting.add(ident)
                item = next((x for x in stages if isinstance(x, dict) and str(x.get("id")) == ident), {})
                cycle = any(visit(str(dep)) for dep in item.get("depends_on", []))
                visiting.remove(ident); visited.add(ident)
                return cycle
            if any(visit(str(x.get("id"))) for x in stages if isinstance(x, dict)):
                errors.append(f"{node.get('id')}:stage_cycle")
        if isinstance(p.get("backfill"), dict) and p["backfill"].get("strategy") == "none":
            errors.append(f"{node.get('id')}:backfill_strategy")
        if isinstance(p.get("backfill"), dict) and p["backfill"].get("interrupted") and not p["backfill"].get("resume_from"):
            errors.append(f"{node.get('id')}:backfill_resume")
        if not known(p.get("compatibility")):
            errors.append(f"{node.get('id')}:compatibility")
    return errors


def check_operations(graph):
    errors = []
    for node in graph.get("elements", []):
        if node.get("class") != "Operation": continue
        p = _props(node)
        if p.get("method") in {"POST", "PUT", "PATCH", "DELETE"}:
            for field in ("idempotency", "concurrency"):
                if not known(p.get(field)): errors.append(f"{node.get('id')}:{field}")
    return errors


def check_failure_consistency(graph):
    """Check retry/deadline/fallback semantics rather than class presence."""
    errors = []
    for node in graph.get("elements", []):
        if node.get("class") not in {"Operation", "Event", "Flow"}:
            continue
        p = _props(node)
        retry = p.get("retry")
        timeout = p.get("timeout") or p.get("deadline")
        if retry and isinstance(retry, dict):
            attempts = retry.get("max_attempts")
            if not isinstance(attempts, int) or attempts < 1:
                errors.append(f"{node.get('id')}:retry.max_attempts")
            if attempts and attempts > 1 and not known(p.get("idempotency")):
                errors.append(f"{node.get('id')}:retry_requires_idempotency")
        if timeout and isinstance(timeout, dict) and known(p.get("timeout_budget")):
            total = timeout.get("total", timeout.get("value"))
            budget = p.get("timeout_budget", {}).get("total") if isinstance(p.get("timeout_budget"), dict) else None
            if isinstance(total, (int, float)) and isinstance(budget, (int, float)) and total > budget:
                errors.append(f"{node.get('id')}:deadline_exceeds_budget")
        if p.get("failure_policy") and isinstance(p.get("failure_policy"), dict):
            policy = p["failure_policy"]
            if policy.get("fallback") is None and policy.get("dead_letter") is None and policy.get("failure_modes"):
                errors.append(f"{node.get('id')}:failure_policy_no_terminal_action")
    return errors


def check_security(graph):
    """Check permission/trust-boundary evidence for protected operations."""
    errors = []
    for node in graph.get("elements", []):
        if node.get("class") not in {"Operation", "Interface", "Event"}:
            continue
        p = _props(node)
        protected = bool(p.get("requires_role") or p.get("requires_scope") or p.get("auth") or p.get("trust_boundary"))
        if protected:
            for field in ("auth", "requires_role", "requires_scope", "trust_boundary"):
                if field in p and p.get(field) in ("unknown", "pending", ""):
                    errors.append(f"{node.get('id')}:{field}")
            if not p.get("authorization_decision") and not p.get("permission_matrix"):
                errors.append(f"{node.get('id')}:permission_evidence")
    return errors


def check_permission(graph):
    errors = []
    for node in graph.get("elements", []):
        if node.get("class") not in {"Operation", "Interface", "Event"}: continue
        p = _props(node)
        if p.get("requires_role") or p.get("requires_scope") or p.get("auth"):
            matrix = p.get("permission_matrix") or p.get("authorization_decision")
            if not matrix: errors.append(f"{node.get('id')}:permission_matrix")
            elif isinstance(matrix, list) and any(not isinstance(row, dict) or not row.get("allow") and not row.get("deny") for row in matrix):
                errors.append(f"{node.get('id')}:permission_row")
    return errors


def check_operations_and_data(graph):
    return check_transaction(graph) + check_migration(graph) + check_operations(graph)


def check_behavior(graph):
    errors = []
    for node in graph.get("elements", []):
        if node.get("class") != "StateMachine": continue
        p = _props(node)
        states = p.get("states", []); transitions = p.get("transitions", [])
        state_ids = {str(x.get("id", x)) if isinstance(x, dict) else str(x) for x in states}
        if not state_ids: errors.append(f"{node.get('id')}:states")
        if not isinstance(transitions, list): errors.append(f"{node.get('id')}:transitions")
        for transition in transitions if isinstance(transitions, list) else []:
            if not isinstance(transition, dict) or str(transition.get("from")) not in state_ids or str(transition.get("to")) not in state_ids:
                errors.append(f"{node.get('id')}:dangling_transition")
            elif not transition.get("event") and not transition.get("condition"):
                errors.append(f"{node.get('id')}:transition_trigger")
        if p.get("initial_state") and str(p["initial_state"]) not in state_ids:
            errors.append(f"{node.get('id')}:initial_state")
    return errors


def check_behavior_contracts(graph):
    """Validate supported finite behavior structures; text-only gaps stay explicit."""
    errors = []
    for node in graph.get("elements", []):
        cls, p = node.get("class"), _props(node)
        if cls == "Flow":
            if not known(p.get("trigger")) or not isinstance(p.get("steps"), list) or not p.get("steps"):
                errors.append(f"{node.get('id')}:flow_trigger_steps")
            if p.get("error_paths") is None and p.get("may_fail_with") is None:
                errors.append(f"{node.get('id')}:flow_error_paths")
        elif cls == "Algorithm":
            ast = p.get("expression_ast") or p.get("ast")
            if not isinstance(ast, (dict, list)):
                errors.append(f"{node.get('id')}:typed_expression_ast")
        elif cls == "DecisionTable":
            if not isinstance(p.get("columns"), list) or not isinstance(p.get("rows"), list) or not p.get("rows"):
                errors.append(f"{node.get('id')}:decision_table_rows")
        elif cls == "Condition" and not (p.get("expression_ast") or p.get("expression")):
            errors.append(f"{node.get('id')}:condition_expression")
    return errors


def check_commit_emit(graph):
    errors = []
    for node in graph.get("elements", []):
        if node.get("class") not in {"Operation", "Event", "Flow"}: continue
        p = _props(node)
        if p.get("transactional") and p.get("emits_event") and not p.get("emit_after_commit"):
            errors.append(f"{node.get('id')}:emit_after_commit")
        if p.get("saga") and not p.get("compensation"):
            errors.append(f"{node.get('id')}:saga_compensation")
    return errors


def check_operations_and_recovery(graph):
    errors = []
    for node in graph.get("elements", []):
        p = _props(node); cls = node.get("class")
        requirements = {
            "LoadProfile": ("average", "peak", "payload", "growth"),
            "CapacityModel": ("resources", "headroom", "scaling"),
            "HighAvailabilityTopology": ("replicas", "failure_domains", "failover"),
            "RecoveryObjective": ("rto", "rpo", "recovery_procedure"),
        }.get(cls, ())
        for field in requirements:
            if not known(p.get(field)): errors.append(f"{node.get('id')}:{field}")
        if cls == "LoadProfile":
            for field in ("average", "peak"):
                value = p.get(field)
                if isinstance(value, dict) and value.get("value") is not None and not value.get("unit"): errors.append(f"{node.get('id')}:{field}.unit")
            if isinstance(p.get("average"), (int, float)) and isinstance(p.get("peak"), (int, float)) and p["peak"] < p["average"]:
                errors.append(f"{node.get('id')}:peak_below_average")
        if cls == "HighAvailabilityTopology" and isinstance(p.get("failure_domains"), list) and len(p["failure_domains"]) < 2:
            errors.append(f"{node.get('id')}:failure_domain_redundancy")
    return errors


def check_compatibility_and_decimal(graph):
    errors = []
    for node in graph.get("elements", []):
        p = _props(node)
        if node.get("class") == "ContractCompatibility":
            for field in ("versioning", "compatibility", "consumers"):
                if not known(p.get(field)): errors.append(f"{node.get('id')}:{field}")
        if node.get("class") == "LedgerInvariant":
            scale = p.get("scale")
            if isinstance(scale, dict) and scale.get("value") is not None and (not scale.get("unit") or int(scale.get("value", -1)) < 0): errors.append(f"{node.get('id')}:scale")
            if not p.get("rounding") and p.get("currency"): errors.append(f"{node.get('id')}:rounding")
    return errors


def evaluate_specialists(graph):
    inputs = {"graph": graph}
    return {
        "transaction": execute("SPECIAL-TRANSACTION", inputs, lambda _: check_transaction(graph)),
        "migration": execute("SPECIAL-MIGRATION", inputs, lambda _: check_migration(graph)),
        "operation": execute("SPECIAL-OPERATION", inputs, lambda _: check_operations(graph)),
        "behavior": execute("SPECIAL-BEHAVIOR", inputs, lambda _: check_behavior(graph)),
        "behavior_contract": execute("SPECIAL-BEHAVIOR-CONTRACT", inputs, lambda _: check_behavior_contracts(graph)),
        "commit_emit": execute("SPECIAL-COMMIT-EMIT", inputs, lambda _: check_commit_emit(graph)),
        "permission": execute("SPECIAL-PERMISSION", inputs, lambda _: check_permission(graph)),
        "recovery": execute("SPECIAL-RECOVERY", inputs, lambda _: check_operations_and_recovery(graph)),
        "failure": execute("SPECIAL-FAILURE", inputs, lambda _: check_failure_consistency(graph)),
        "security": execute("SPECIAL-SECURITY", inputs, lambda _: check_security(graph)),
        "compatibility": execute("SPECIAL-COMPATIBILITY", inputs, lambda _: check_compatibility_and_decimal(graph)),
    }
