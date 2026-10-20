"""Review and baseline (T7.2, FR-011, SCN-03): prioritised pending list, review by code or filter, auto-approval policy,
baseline through gate GS + a Baselined primary TDD Document + dropped nodes with a CR (TSO-020, 021, B18)."""
import collections

from ..config import check_feature
from ..errors import HandEditedError, NotFound, ValidationFailed
from ..render import render
from ..store import lifecycle, ops, source
from ..store import tso as store_tso
from ..validate.tso_rules import Ctx, validate

VERSIONING_RULES = ("T16", "T19", "T20", "R17")


def _draft_graph(db, feature):
    d = store_tso.open_draft(db, feature)
    if d is None:
        raise NotFound(f"{feature}: no Draft version")
    return store_tso.load_spec(db, feature, d["version"])


def pending_review(db, feature, classes=None, derivation=None, origin=None, layer=None, min_confidence=None, max_confidence=None,
                   statuses=("proposed", "needsInfo")):
    """Nodes awaiting review, by priority: LLM proposals (`llmProposal`), then `llm`, then rules; within a group by increasing confidence."""
    check_feature(feature)
    rows = lifecycle.pending(db, feature, derivation=derivation, classes=classes, layer=layer, review_status=statuses)
    g = _draft_graph(db, feature) if rows else None
    proposal_obs = {e.code for e in g.of("TestObligation") if e.attrs.get("origin") == "llmProposal"} if g else set()
    out = []
    for r in rows:
        e = g.get(r["code"])
        org = r.get("origin")
        if e is not None and e.cls == "TestCondition" and set(e.rel("obligation")) & proposal_obs:
            org = "llmProposal"
        if origin and org != origin:
            continue
        if min_confidence is not None and r["confidence"] < min_confidence:
            continue
        if max_confidence is not None and r["confidence"] > max_confidence:
            continue
        out.append({"code": r["code"], "class": r["cls"], "layer": r["layer"], "derivation": r["derivation"],
                    "confidence": r["confidence"], "reviewStatus": r["review_status"], "origin": org,
                    "summary": _summary(e)})
    rank = {"llmProposal": 0}
    out.sort(key=lambda x: (rank.get(x["origin"], 1 if x["derivation"] == "llm" else 2), x["confidence"], x["code"]))
    return out


def _summary(e):
    if e is None:
        return ""
    for k in ("statement", "question", "objective", "missing", "expected", "text", "description", "action"):
        if e.attrs.get(k):
            return str(e.attrs[k])[:200]
    return ""


def pending_summary(db, feature, limit=50):
    """Pending-review summary (tool `list_pending_review`): counts by class, gap-blocked nodes, unreviewed LLM nodes; the list is cut at `limit`."""
    items = pending_review(db, feature) if store_tso.open_draft(db, feature) else []
    g = _draft_graph(db, feature) if items else None
    blocked = []
    if g is not None:
        review_st = {c: (lc or {}).get("review_status") for c, lc in g.meta["lifecycle"].items()}
        for gp in g.of("TestGap"):
            if review_st.get(gp.code) not in ("approved", "rejected"):
                blocked += [{"code": c, "gap": gp.code, "missing": gp.attrs["missing"][:160]} for c in gp.rel("blocks")]
    inferred = [i for i in items if i["derivation"] == "llm"]
    slim = [{k: i[k] for k in ("code", "class", "derivation", "confidence", "reviewStatus", "origin")} for i in items[:limit]]
    return {"total": len(items), "byClass": dict(sorted(collections.Counter(i["class"] for i in items).items())),
            "pendingConfirmationCount": len(blocked), "inferredCount": len(inferred),
            "pendingConfirmation": blocked[:limit], "inferred": [i["code"] for i in inferred[:limit]], "items": slim}


def review(db, feature, status, by, note="", codes=None, **filters):
    """approve / reject / needsInfo by code or filter (classes, derivation, layer, min_confidence)."""
    return lifecycle.set_review(db, feature, status, by, note, codes=codes, **filters)


def auto_approve(db, feature, policy):
    """Policy `TSAGENT_AUTO_APPROVE`: approve `llm` nodes of allowed classes above the confidence threshold; reviewer=auto:<policy>."""
    if policy is None or not policy.enabled:
        return {"updated": 0}
    tag = f"auto:classes={','.join(policy.classes)};min_confidence={policy.min_confidence}"
    return lifecycle.set_review(db, feature, "approved", tag, "auto-approved by policy", classes=list(policy.classes),
                                derivation="llm", min_confidence=policy.min_confidence, only_proposed=True)


def gate_report(db, cfg, feature):
    g = _draft_graph(db, feature)
    pkg = source.load_source(db, feature, g.meta["baseline"])
    rv = {c: (lc or {}).get("review_status") for c, lc in g.meta["lifecycle"].items()}
    problems = [{"code": p.get("feature") or feature, "message": f"{p['rule']}: {p['message']}"}
                for p in ops.check(db, feature) if p["rule"] in VERSIONING_RULES]
    rep = validate(Ctx(g, pkg, g.meta["baseline"], auto_approve=cfg.auto_approve, review=rv,
                       extra={"versioning_problems": problems}), "GS")
    return rep, pkg, g


def baseline_spec(db, cfg, feature, by, retired_by=None):
    """Apply the auto-approval policy, check GS and the primary TDD Document, then baseline in one transaction; re-render if documents exist."""
    check_feature(feature)
    auto = auto_approve(db, feature, cfg.auto_approve)
    rep, pkg, g = gate_report(db, cfg, feature)
    primary = pkg.get(pkg.meta.get("primaryDoc") or "")
    errors = []
    if primary is None:
        errors.append("cannot determine the primary TDD Document of the source baseline")
    elif str(primary.base.get("status", "")) != "Baselined":
        errors.append(f"primary TDD Document {primary.code} is not Baselined (currently {primary.base.get('status')})")
    if errors or not rep.passed:
        d = rep.to_dict()
        d["errors"] = errors
        raise ValidationFailed(d, "baseline")
    res = lifecycle.baseline(db, feature, by, retired_by=retired_by)
    res["autoApproved"] = auto["updated"]
    if db.one("SELECT 1 FROM rendered_file WHERE feature = ?", (feature,)):
        try:
            res["files"] = render(db, cfg, feature)["files"]
        except HandEditedError as e:
            res["renderError"] = str(e)
    return res
