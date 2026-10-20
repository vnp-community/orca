"""Extended profile seed rules (T8.3, ONTOLOGY-TEST-TSO section 7.2): run after the Core rules when the package declares `+ext-N`.

Relation names come from `profiles.rel` (default per ONTOLOGY-TDD section 5, pending OQ-O1). Each in-scope module 04 node is a subject.

Vietnamese strings in this module are deliberate: rule templates become the content of the (Vietnamese) test-spec documents.
"""
from .profiles import rel
from .seed_core import label, text

TIMING = {"noEffect": "before", "none": "before", "partial": "during", "committed": "afterCommit", "unknown": "afterCommit"}


def lst(v):
    if v is None or v == "":
        return []
    return v if isinstance(v, list) else [v]


def _s(v):
    if isinstance(v, dict):
        return "; ".join(f"{k}={x}" for k, x in sorted(v.items()))
    return str(v)


def run(sd):
    P = sd.pkg
    handlers = {
        "DesignObligation": obligation, "Operation": operation, "InteractionFlow": flow, "DecisionTable": decision_table,
        "Invariant": invariant, "ConcurrencyPolicy": concurrency, "ErrorContract": error_contract, "TimeoutBudget": timeout_budget,
        "IdempotencyPolicy": idempotency, "Transaction": transaction, "Saga": saga, "Migration": migration,
        "ContractCompatibility": compatibility, "PermissionDesign": permission, "SecretHandling": secret, "AuditDesign": audit,
        "LoadProfile": load_profile, "RecoveryPlan": recovery, "ObservabilityDesign": observability, "RolloutPlan": rollout,
    }
    for cls in sorted(handlers):
        for n in P.of(cls):
            if n.in_scope:
                handlers[cls](sd, n)
    for g in P.of("DesignGap"):
        if g.in_scope:
            design_gap(sd, g)


def obligation(sd, n):
    if str(n.props.get("applicability", "applicable")) != "applicable":
        return
    P = sd.pkg
    targets = P.targets(n.code, rel("obligationFor"))
    prio = min((x.props.get("priority") and {"must": "P1", "should": "P2"}.get(str(x.props["priority"]).lower(), "P2") or "P2"
                for x in targets), default="P2")
    ob = sd.obligation(n, "design", n.props.get("question") or label(n), prio, extra_subjects=targets, origin="designObligation")
    res = P.sources(n.code, rel("resolves"), "ObligationResolution")
    resolved = next((r for r in res if str(r.props.get("status", "")).lower() in ("resolved", "addressed", "done")), None)
    for kind in lst(n.props.get("requiredChecks")) or ["functional"]:
        exp = resolved.props.get("explanation") if resolved else None
        orc = sd.oracle(resolved or n, "reference", exp or f"Thiết kế trả lời: {n.props.get('question') or label(n)}", key=f"obl-{kind}")
        sd.condition(ob, n, kind if kind in KINDS else "functional", f"obl-{kind}", n.props.get("question") or label(n),
                     exp or "Thiết kế đáp ứng nghĩa vụ", orc, based=[t.code for t in targets],
                     missing=None if exp else f"cách thiết kế giải nghĩa vụ {n.code} ({kind})")


KINDS = {"functional", "inputValidation", "stateTransition", "guardViolation", "invariant", "decisionRule", "errorHandling",
         "idempotency", "concurrency", "timing", "atomicity", "compensation", "migration", "compatibility", "access",
         "secretExposure", "audit", "performance", "resilience", "observability", "rollout"}


def design_gap(sd, g):
    P = sd.pkg
    for ob_node in P.targets(g.code, rel("gapOf"), "DesignObligation"):
        obs = sd.obligations.get(ob_node.code, [])
        sd.gap(ob_node, g.code, g.props.get("missingKnowledge") or label(g),
               f"Bổ sung: {g.props.get('missingKnowledge') or label(g)}?", g.props.get("owner") or g.base.get("owner") or "Tech Lead",
               blocks=list(sd.by_subject.get(ob_node.code, [])), blocks_ob=obs[0] if obs else None, based=[g.code])


def operation(sd, n):
    p = n.props
    reqs = sd.pkg.targets(n.code, rel("operationFor"))
    ob = sd.obligation(n, "operation", f"Thao tác {label(n)} đúng hợp đồng", "P1", extra_subjects=reqs)
    for i, post in enumerate(lst(p.get("postconditions")), 1):
        orc = sd.oracle(n, "postcondition", _s(post), key=f"post-{i}")
        sd.condition(ob, n, "functional", f"post-{i}", f"{label(n)} thành công", _s(post), orc,
                     pre="; ".join(_s(x) for x in lst(p.get("preconditions"))) or None, evidence=[(n.code, _s(post))])
    for i, pre in enumerate(lst(p.get("preconditions")), 1):
        orc = sd.oracle(n, "postcondition", f"Không thực hiện khi không thỏa: {_s(pre)}", key=f"pre-{i}")
        sd.condition(ob, n, "guardViolation", f"pre-{i}", f"Gọi {label(n)} khi không thỏa: {_s(pre)}",
                     "Không thực hiện, không có hiệu ứng phụ", orc, evidence=[(n.code, _s(pre))])
    for i, err in enumerate(lst(p.get("errorOutcomes")), 1):
        orc = sd.oracle(n, "errorContract", _s(err), key=f"err-{i}")
        sd.condition(ob, n, "errorHandling", f"err-{i}", f"{label(n)} gặp: {_s(err).split(':')[0]}", _s(err), orc,
                     evidence=[(n.code, _s(err))])


def flow(sd, n):
    steps = [label(s) for s in sorted(sd.pkg.targets(n.code, rel("hasStep")), key=lambda s: s.props.get("ordering", 0))] or [label(n)]
    ob = sd.obligation(n, "flow", f"Luồng {label(n)} đúng", "P1")
    pm = sd.path(n, "main", "main", steps)
    orc = sd.oracle(n, "postcondition", "; ".join(_s(x) for x in lst(n.props.get("successConditions"))) or steps[-1], key="main")
    sd.condition(ob, n, "functional", "main", " → ".join(steps), sd.g.get(orc).attrs["expected"], orc, path=pm)
    for i, fp in enumerate(lst(n.props.get("failurePaths")), 1):
        pf = sd.path(n, f"fail-{i}", "failure", steps[:1] + [_s(fp)])
        o2 = sd.oracle(n, "postcondition", _s(fp), key=f"fail-{i}")
        sd.condition(ob, n, "errorHandling", f"fail-{i}", f"Luồng lỗi: {_s(fp)}", _s(fp), o2, path=pf)


def decision_table(sd, n):
    ob = sd.obligation(n, "decision", f"Bảng quyết định {label(n)} đúng từng dòng", "P1")
    rows = lst(n.props.get("rows"))
    for i, row in enumerate(rows, 1):
        orc = sd.oracle(n, "decisionRow", _s(row), key=f"row-{i}")
        sd.condition(ob, n, "decisionRule", f"row-{i}", f"Đầu vào khớp dòng {i}: {_s(row)}", _s(row), orc)
    if str(n.props.get("completenessScope", "")).lower() not in ("complete", "đầy đủ"):
        orc = sd.oracle(n, "decisionRow", "Xử lý mặc định khi không khớp dòng nào", key="nomatch")
        sd.condition(ob, n, "decisionRule", "nomatch", "Đầu vào không khớp dòng nào", "Xử lý mặc định", orc,
                     missing=f"hành vi khi không khớp dòng nào của {n.code}")


def invariant(sd, n):
    ob = sd.obligation(n, "invariant", f"Bất biến giữ: {n.props.get('predicate') or label(n)}", "P0")
    pred = n.props.get("predicate") or label(n)
    for i, ep in enumerate(lst(n.props.get("enforcementPoints")) or ["chung"], 1):
        orc = sd.oracle(n, "invariant", pred, key=f"ep-{i}")
        fm = sd.fault(n, f"ep-{i}", "violationAttempt", str(ep), "during", n.props.get("failureBehavior") or "Bất biến được giữ")
        sd.condition(ob, n, "invariant", f"ep-{i}", f"Cố vi phạm tại điểm thực thi: {ep}", n.props.get("failureBehavior") or pred,
                     orc, fault=fm, evidence=[(n.code, pred)])


def concurrency(sd, n):
    ob = sd.obligation(n, "concurrency", f"Chính sách đồng thời {label(n)}", "P0")
    for i, res in enumerate(lst(n.props.get("sharedResources")) or [label(n)], 1):
        orc = sd.oracle(n, "invariant", n.props.get("conflictHandling") or "Không mất cập nhật, không trùng", key=f"res-{i}")
        sd.condition(ob, n, "concurrency", f"res-{i}", f"Hai tác nhân cùng ghi {res} ({n.props.get('isolation') or ''})".strip(),
                     n.props.get("conflictHandling") or "Không mất cập nhật, không trùng", orc,
                     missing=None if n.props.get("conflictHandling") else f"cách xử lý xung đột trên {res}")


def error_contract(sd, n):
    p = n.props
    owners = sd.pkg.targets(n.code, rel("errorOf"))
    ob = sd.obligation(n, "error", f"Hợp đồng lỗi {label(n)}", "P1", extra_subjects=owners)
    timing = TIMING.get(str(p.get("sideEffectState", "")), "before")
    handling = f"{p.get('classification', '')}; retryable={p.get('retryable')}; hiệu ứng phụ: {p.get('sideEffectState', '?')}"
    fm = sd.fault(n, "err", str(p.get("classification") or "error"), owners[0].code if owners else n.code, timing, handling)
    orc = sd.oracle(n, "errorContract", handling, key="err")
    sd.condition(ob, n, "errorHandling", "err", f"Xảy ra: {p.get('condition') or label(n)}", handling, orc, fault=fm,
                 based=[o.code for o in owners])


def timeout_budget(sd, n):
    p = n.props
    calls = p.get("callBudgets") or {}
    total = sum(v for v in calls.values() if isinstance(v, (int, float))) if isinstance(calls, dict) else None
    dl = p.get("endToEndDeadline")
    ob = sd.obligation(n, "budget", f"Ngân sách thời gian {label(n)}", "P1", extra_subjects=sd.pkg.targets(n.code, rel("budgetFor")))
    st = f"Trường hợp xấu nhất: tổng ngân sách lời gọi {total} ms (mỗi lời gọi lấy mức tối đa) so với hạn {dl} ms"
    orc = sd.oracle(n, "threshold", f"≤ {dl} ms", key=f"worst-{n.code}",
                    measurement="Cộng ngân sách tối đa của từng lời gọi (trường hợp xấu nhất), không cộng p95")
    sd.condition(ob, n, "timing", f"worst-{n.code}", st, f"≤ {dl} ms; quá hạn thì: {p.get('cancellation') or '?'}", orc,
                 missing=None if dl else f"hạn đầu-cuối của {n.code}")


def idempotency(sd, n):
    p = n.props
    ob = sd.obligation(n, "idempotency", f"Chính sách idempotency {label(n)}", "P0", extra_subjects=sd.pkg.targets(n.code, rel("idempotencyOf")))
    orc = sd.oracle(n, "invariant", f"Khóa {p.get('keyDerivation', '?')} không tạo hiệu ứng lặp", key="idem")
    cases = [("replay", "idempotency", "Gửi lại cùng khóa, cùng payload", "Trả kết quả lần đầu, không tạo hiệu ứng lặp"),
             ("payload-mismatch", "idempotency", "Cùng khóa, khác payload", f"Bị từ chối (đối chiếu {p.get('payloadFingerprint', 'dấu vân payload')})"),
             ("concurrent-dup", "concurrency", "Hai yêu cầu cùng khóa gửi đồng thời", p.get("concurrentDuplicateBehavior") or "Chỉ một hiệu ứng"),
             ("after-retention", "idempotency", f"Gửi lại sau thời hạn giữ khóa ({p.get('retention', '?')})", "Xử lý như yêu cầu mới theo chính sách")]
    for key, kind, st, exp in cases:
        sd.condition(ob, n, kind, key, st, exp, orc, missing=f"thời hạn giữ khóa của {n.code}" if key == "after-retention" and not p.get("retention") else None)


def transaction(sd, n):
    p = n.props
    ob = sd.obligation(n, "atomicity", f"Giao dịch {label(n)} nguyên tử", "P0")
    for key, timing, st in (("before-commit", "before", f"Lỗi trước điểm commit ({p.get('commitPoint', '?')})"),
                            ("after-commit", "afterCommit", f"Lỗi ngay sau điểm commit ({p.get('commitPoint', '?')})")):
        fm = sd.fault(n, key, "processCrash", n.code, timing, p.get("rollbackBehavior") or "Không ghi dở")
        exp = (p.get("rollbackBehavior") or "Không có thay đổi nào") if key == "before-commit" else "Toàn bộ thay đổi còn, không lặp khi chạy lại"
        orc = sd.oracle(n, "invariant", exp, key=key)
        sd.condition(ob, n, "atomicity", key, st, exp, orc, fault=fm)


def saga(sd, n):
    ob = sd.obligation(n, "saga", f"Saga {label(n)} bù trừ đúng", "P0")
    for s in sd.pkg.targets(n.code, rel("hasStep"), "SagaStep"):
        q = s.props
        orc = sd.oracle(s, "postcondition", q.get("compensation") or "Bù trừ bước", key="comp")
        sd.condition(ob, s, "compensation", "comp", f"Bước sau {label(s)} lỗi", q.get("compensation") or "Bù trừ bước", orc,
                     based=[n.code], missing=None if q.get("compensation") else f"cách bù trừ {s.code}")
        if q.get("irreversibility"):
            o2 = sd.oracle(s, "humanJudgment", f"Xử lý tay: {q['irreversibility']}", key="manual")
            sd.condition(ob, s, "compensation", "manual", f"{label(s)} không đảo ngược được", f"Xử lý tay: {q['irreversibility']}", o2,
                         based=[n.code])


def migration(sd, n):
    p = n.props
    ob = sd.obligation(n, "migration", f"Migration {label(n)}", "P1")
    for key, st, exp in (("forward", "Chạy migration tiến", "; ".join(_s(x) for x in lst(p.get("validationQueries"))) or "Dữ liệu đúng sau migrate"),
                         ("rollback", "Rollback hoặc khôi phục", _s(p.get("rollbackOrRecovery") or "Khôi phục trạng thái trước")),
                         ("rerun", "Chạy lại migration đã chạy dở", "Không lỗi, không nhân đôi dữ liệu"),
                         ("compat", f"Phiên bản cũ và mới cùng chạy trong cửa sổ {p.get('compatibilityWindow', '?')}", "Cả hai phiên bản hoạt động")):
        orc = sd.oracle(n, "postcondition", exp, key=key)
        sd.condition(ob, n, "migration", key, st, exp, orc)


def compatibility(sd, n):
    ob = sd.obligation(n, "compat", f"Tương thích hợp đồng {label(n)}", "P1", extra_subjects=sd.pkg.targets(n.code, rel("compatibilityOf")))
    for i, v in enumerate(lst(n.props.get("consumerVersions")) or ["hiện hành"], 1):
        orc = sd.oracle(n, "schema", f"Consumer {v} đọc được ({n.props.get('compatibilityMode', '?')})", key=f"v-{i}")
        sd.condition(ob, n, "compatibility", f"v-{i}", f"Consumer phiên bản {v}", sd.g.get(orc).attrs["expected"], orc)


def permission(sd, n):
    P = sd.pkg
    points = {e.code: e for e in P.targets(n.code, rel("protects"))}
    for ep in lst(n.props.get("enforcementPoints")):
        if P.get(str(ep)) is not None:
            points[str(ep)] = P.get(str(ep))
    ob = sd.obligation(n, "access", f"Phân quyền {label(n)}", "P0", extra_subjects=list(points.values()))
    pred = n.props.get("decisionPredicate") or label(n)
    for code in sorted(points) or [n.code]:
        orc = sd.oracle(n, "reference", f"Cho phép khi {pred}; từ chối: {n.props.get('denyBehavior', '?')}", key=f"acl-{code}")
        sd.condition(ob, n, "access", f"allow-{code}", f"{n.props.get('subject', 'Chủ thể')} hợp lệ gọi {code} ({pred})", "Được phục vụ",
                     orc, based=[code] if code in P.nodes else ())
        sd.condition(ob, n, "access", f"deny-{code}", f"Người khác gọi {code} (không thỏa {pred})",
                     str(n.props.get("denyBehavior") or "Bị từ chối"), orc, based=[code] if code in P.nodes else (),
                     missing=None if n.props.get("denyBehavior") else f"hành vi từ chối của {code}")


def secret(sd, n):
    ob = sd.obligation(n, "secret", f"Bí mật {label(n)} không lộ", "P0")
    for ch in ("log", "error", "event"):
        orc = sd.oracle(n, "reference", f"Không có {n.props.get('secretKind', 'bí mật')} trong {ch}", key=ch)
        sd.condition(ob, n, "secretExposure", ch, f"Kích hoạt đường {ch} khi dùng {n.props.get('reference', label(n))}",
                     sd.g.get(orc).attrs["expected"], orc)


def audit(sd, n):
    ob = sd.obligation(n, "audit", f"Audit {label(n)}", "P1", extra_subjects=sd.pkg.targets(n.code, rel("auditOf")))
    for i, ev in enumerate(lst(n.props.get("events")) or [label(n)], 1):
        orc = sd.oracle(n, "reference", f"Ghi audit {ev} kèm {n.props.get('actorAttribution', 'tác nhân')}", key=f"ev-{i}")
        sd.condition(ob, n, "audit", f"ev-{i}", f"Sự kiện {ev} xảy ra", sd.g.get(orc).attrs["expected"], orc)


def load_profile(sd, n):
    p = n.props
    nfrs = sd.pkg.targets(n.code, rel("loadFor"))
    ob = sd.obligation(n, "load", f"Tải {label(n)}", "P1", extra_subjects=nfrs)
    wl = "; ".join(f"{k}={_s(p[k])}" for k in ("workloadMix", "arrivalPattern", "concurrency", "duration") if p.get(k))
    for x in nfrs or [n]:
        orc = sd.oracle(x, "threshold", text(x), key=f"load-{n.code}", measurement=wl or None)
        sd.condition(ob, n, "performance", f"load-{x.code}", f"Chạy tải {label(n)}", text(x), orc, based=[x.code],
                     missing=None if wl else f"workload của {n.code}")


def recovery(sd, n):
    ob = sd.obligation(n, "recovery", f"Phục hồi {label(n)}", "P1")
    for k in ("RTO", "RPO"):
        if n.props.get(k) is None:
            continue
        orc = sd.oracle(n, "threshold", f"{k} ≤ {n.props[k]}", key=k.lower(), measurement=f"Đo từ lúc sự cố ({n.props.get('failureScope', '?')}) tới khi phục hồi")
        sd.condition(ob, n, "resilience", k.lower(), f"Sự cố {n.props.get('failureScope', '')}".strip(), f"{k} ≤ {n.props[k]}", orc)


def observability(sd, n):
    ob = sd.obligation(n, "obs", f"Quan sát {label(n)}", "P2", extra_subjects=sd.pkg.targets(n.code, rel("observes")))
    for i, th in enumerate(lst(n.props.get("thresholds")), 1):
        orc = sd.oracle(n, "reference", f"Vượt {_s(th)} thì {_s(n.props.get('actions') or 'cảnh báo')}", key=f"th-{i}")
        sd.condition(ob, n, "observability", f"th-{i}", f"Tín hiệu vượt ngưỡng {_s(th)}", sd.g.get(orc).attrs["expected"], orc)


def rollout(sd, n):
    ob = sd.obligation(n, "rollout", f"Triển khai {label(n)}", "P2")
    for i, ac in enumerate(lst(n.props.get("abortConditions")), 1):
        orc = sd.oracle(n, "reference", f"Dừng triển khai khi {_s(ac)}", key=f"abort-{i}")
        sd.condition(ob, n, "rollout", f"abort-{i}", f"Điều kiện dừng xảy ra: {_s(ac)}", sd.g.get(orc).attrs["expected"], orc)
