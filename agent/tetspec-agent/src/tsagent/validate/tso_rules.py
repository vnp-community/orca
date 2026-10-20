"""TSO-* rules (ONTOLOGY-TEST-TSO section 9) on an in-memory TsoGraph + the source package. Each rule: fn(ctx, RuleResult).

Rule titles stay Vietnamese: they are printed in the "Báo cáo kiểm" section of the test-spec documents."""
import collections
import re
from dataclasses import dataclass, field

from ..model import TsoGraph
from .report import BLOCK, BLOCK_BASELINE, WARN, Report, RuleResult

REQUIREMENT_CLASSES = ("FunctionalRequirement", "NonFunctionalRequirement", "UserRequirement")
SOURCE_REF_ATTRS = {"TestBasisItem": ("tddCode",), "TestStep": ("targetCode",), "StubSpec": ("targetCode",)}


@dataclass
class Ctx:
    graph: TsoGraph
    pkg: object                    # SourcePackage of the baseline in use
    baseline_id: str = ""
    expected_seed: set = None      # mandatory seed condition + obligation codes (TSO-005); None means computed
    auto_approve: object = None    # AutoApprovePolicy (TSO-020)
    review: dict = field(default_factory=dict)  # code -> reviewStatus (TSO-020, when checking against the DB)
    extra: dict = field(default_factory=dict)

    def seeds(self):
        if self.expected_seed is None:
            from ..analyze.seed_core import seed
            s = seed(self.pkg, self.baseline_id)
            self.expected_seed = {e.code for e in s.g.of("TestCondition", "TestObligation")}
        return self.expected_seed


RULES = {}


def rule(rid, level, title):
    def deco(fn):
        RULES[rid] = (level, title, fn)
        return fn
    return deco


# ------------------------------------------------------------------------------------------------- every layer
@rule("TSO-001", BLOCK, "Mọi nút có basedOn tới nút TDD tồn tại ở baseline")
def tso_001(ctx, r):
    for e in ctx.graph.entities.values():
        r.checked += 1
        ok = [b for b in e.prov.based_on if b in ctx.pkg.nodes]
        if not ok:
            r.add(e.code, f"{e.cls} has no valid basedOn (has: {e.prov.based_on or 'empty'})")
        elif len(ok) != len(e.prov.based_on):
            r.add(e.code, f"basedOn points to codes missing from the baseline: {sorted(set(e.prov.based_on) - set(ok))}")


@rule("TSO-002", BLOCK, "Nút derivation=llm có model, promptHash, evidence")
def tso_002(ctx, r):
    for e in ctx.graph.entities.values():
        if e.prov.derivation != "llm":
            continue
        r.checked += 1
        miss = [k for k, v in (("model", e.prov.model), ("promptHash", e.prov.prompt_hash), ("evidence", e.prov.evidence)) if not v]
        if miss:
            r.add(e.code, f"missing {', '.join(miss)}")


@rule("TSO-003", BLOCK, "Mọi mã tham chiếu thuộc danh sách trắng")
def tso_003(ctx, r):
    for e in ctx.graph.entities.values():
        r.checked += 1
        bad = [ev.source_code for ev in e.prov.evidence if ev.source_code not in ctx.pkg.nodes]
        for a in SOURCE_REF_ATTRS.get(e.cls, ()):
            v = e.attrs.get(a)
            if v and v not in ctx.pkg.nodes:
                bad.append(v)
        for name, ts in e.rels.items():
            bad += [f"{name}->{t}" for t in ts if t not in ctx.graph.entities]
        if bad:
            r.add(e.code, f"codes not on the allow list: {sorted(set(bad))}")


# ------------------------------------------------------------------------------------------------- layer A
def _subject_codes(g, e):
    return {g.get(s).attrs.get("tddCode") for s in e.rel("subject") if g.get(s)}


@rule("TSO-004", BLOCK, "Mọi Requirement trong phạm vi có TestObligation")
def tso_004(ctx, r):
    covered = set()
    for ob in ctx.graph.of("TestObligation"):
        covered |= _subject_codes(ctx.graph, ob)
    for n in ctx.pkg.of(*REQUIREMENT_CLASSES):
        if not n.in_scope:
            continue
        r.checked += 1
        if n.code not in covered:
            r.add(n.code, f"{n.cls} {n.code} has no TestObligation")
    for n in ctx.pkg.of("DesignObligation"):
        if n.in_scope and str(n.props.get("applicability", "applicable")) == "applicable":
            r.checked += 1
            if n.code not in covered:
                r.add(n.code, f"DesignObligation {n.code} has no TestObligation")


@rule("TSO-005", BLOCK, "Nút TDD có luật seed có đủ TestCondition seed")
def tso_005(ctx, r):
    for code in sorted(ctx.seeds()):
        r.checked += 1
        if code not in ctx.graph.entities:
            r.add(code, "mandatory seed node is missing")


@rule("TSO-006", BLOCK, "Mọi TestCondition có oracle; oracle threshold có measurement")
def tso_006(ctx, r):
    # missing measurement with a TestGap already blocking the condition: TSO-007 blocks it at baseline (ask the owner), the analysis gate does not
    gapped = {c for gp in ctx.graph.of("TestGap") for c in gp.rel("blocks")}
    for c in ctx.graph.of("TestCondition"):
        r.checked += 1
        o = ctx.graph.get(c.rel("oracle")[0]) if c.rel("oracle") else None
        if o is None:
            r.add(c.code, "no oracle")
        elif o.attrs.get("oracleKind") == "threshold" and not o.attrs.get("measurement") and c.code not in gapped:
            r.add(c.code, f"threshold oracle {o.code} has no measurement")


@rule("TSO-007", BLOCK_BASELINE, "TestGap có owner; điều kiện bị gap chặn không được baseline")
def tso_007(ctx, r):
    for gp in ctx.graph.of("TestGap"):
        r.checked += 1
        if not str(gp.attrs.get("owner") or "").strip():
            r.add(gp.code, "TestGap has no owner")
        if ctx.review.get(gp.code) in ("approved", "rejected"):
            continue  # the gap was closed by a reviewer (risk accepted or rejected): it no longer blocks (DEVIATIONS D14)
        for c in gp.rel("blocks"):
            r.add(c, f"blocked by {gp.code}: {gp.attrs.get('missing', '')[:120]}")


# ------------------------------------------------------------------------------------------------- layer B
@rule("TSO-008", BLOCK, "Mọi TestCondition được ít nhất một CoverageItem hiện thực")
def tso_008(ctx, r):
    realized = {c for e in ctx.graph.of("CoverageItem") for c in e.rel("realizes")}
    for c in ctx.graph.of("TestCondition"):
        r.checked += 1
        if c.code not in realized:
            r.add(c.code, "no CoverageItem")


@rule("TSO-009", BLOCK, "TR có passCriteria đánh số trỏ tới điều kiện; scopeNote khi cùng chủ thể có nhiều TR")
def tso_009(ctx, r):
    g = ctx.graph
    pcs = collections.defaultdict(list)
    for pc in g.of("PassCriterion"):
        pcs[pc.rel("requirement")[0] if pc.rel("requirement") else None].append(pc)
    per_subject = collections.Counter(tr.rel("subject")[0] for tr in g.of("TestRequirement") if tr.rel("subject"))
    for tr in g.of("TestRequirement"):
        r.checked += 1
        items = sorted(pcs.get(tr.code, []), key=lambda x: x.attrs.get("ordinal", 0))
        if not items:
            r.add(tr.code, "no PassCriterion")
            continue
        if [x.attrs.get("ordinal") for x in items] != list(range(1, len(items) + 1)):
            r.add(tr.code, "PassCriteria are not numbered contiguously from 1")
        stray = [x.code for x in items if not x.rel("condition") or x.rel("condition")[0] not in tr.rel("groups")]
        if stray:
            r.add(tr.code, f"PassCriterion points outside the TR's conditions: {stray}")
        missing = set(tr.rel("groups")) - {x.rel("condition")[0] for x in items if x.rel("condition")}
        if missing:
            r.add(tr.code, f"conditions without a criterion: {sorted(missing)}")
        if per_subject[tr.rel("subject")[0]] > 1 and not str(tr.attrs.get("scopeNote") or "").strip():
            r.add(tr.code, "missing scopeNote while the subject has several TRs")
        subj = g.get(tr.rel("subject")[0])
        n = ctx.pkg.get(subj.attrs["tddCode"]) if subj else None
        if n is not None and str(tr.attrs.get("statement", "")).strip().lower() in (
                str(n.base.get("label", "")).strip().lower(), str(n.props.get("text", "")).strip().lower()):
            r.add(tr.code, "statement copies the name of the source requirement")


@rule("TSO-010", WARN, "Kỹ thuật phù hợp conditionKind")
def tso_010(ctx, r):
    from ..design.rules import ALLOWED
    g = ctx.graph
    for cov in g.of("CoverageItem"):
        td = g.get(cov.rel("design")[0]) if cov.rel("design") else None
        c = g.get(cov.rel("realizes")[0]) if cov.rel("realizes") else None
        if td is None or c is None:
            continue
        r.checked += 1
        kind, tech = c.attrs["conditionKind"], td.attrs["technique"]
        if tech not in ALLOWED.get(kind, {tech}) and not (tech == "scenario" and c.rel("path")):
            r.add(cov.code, f"technique {tech} does not fit {kind}")


@rule("TSO-016", BLOCK, "Điều kiện performance có workload; không có test vượt ngưỡng + 1 cho chỉ tiêu 0 hoặc 100 %")
def tso_016(ctx, r):
    g = ctx.graph
    gapped = {c for gp in g.of("TestGap") for c in gp.rel("blocks")}
    for c in g.of("TestCondition"):
        if c.attrs["conditionKind"] != "performance":
            continue
        r.checked += 1
        o = g.get(c.rel("oracle")[0]) if c.rel("oracle") else None
        if (o is None or not str(o.attrs.get("measurement") or "").strip()) and c.code not in gapped:
            r.add(c.code, "performance condition has no workload/measurement")
    for cov in g.of("CoverageItem"):
        c = g.get(cov.rel("realizes")[0]) if cov.rel("realizes") else None
        if c is None:
            continue
        for b in c.prov.based_on:
            n = ctx.pkg.get(b)
            if n is None or n.cls != "NonFunctionalRequirement":
                continue
            tv = n.props.get("targetValue")
            unit = str(n.props.get("targetUnit", ""))
            val = cov.attrs.get("value")
            val = val.get("value") if isinstance(val, dict) else val
            if tv in (0, 100) and (tv == 0 or "%" in unit or "percent" in unit) and isinstance(val, (int, float)) and val > tv:
                r.checked += 1
                r.add(cov.code, f"an over-threshold test {val} for target {tv}{unit} is meaningless")


# ------------------------------------------------------------------------------------------------- layer C
def _children(g, cls, rel):
    out = collections.defaultdict(list)
    for e in g.of(cls):
        for t in e.rel(rel):
            out[t].append(e)
    return out


NUM_RE = re.compile(r"\d+(?:[.,]\d+)?")


@rule("TSO-011", BLOCK, "Mọi CoverageItem có TestCase phủ; mọi TestCase có bước when và Assertion")
def tso_011(ctx, r):
    g = ctx.graph
    covered = {c for tc in g.of("TestCase") for c in tc.rel("covers")}
    for cov in g.of("CoverageItem"):
        r.checked += 1
        if cov.code not in covered:
            r.add(cov.code, "no covering TestCase")
    steps, asserts = _children(g, "TestStep", "testCase"), _children(g, "Assertion", "testCase")
    for tc in g.of("TestCase"):
        r.checked += 1
        if not any(s.attrs.get("stepKind") == "when" for s in steps.get(tc.code, [])):
            r.add(tc.code, "no when step")
        if not asserts.get(tc.code):
            r.add(tc.code, "no Assertion")


@rule("TSO-012", BLOCK, "Assertion.expected khớp oracle (đơn vị, ngưỡng, mã lỗi)")
def tso_012(ctx, r):
    g = ctx.graph
    for a in g.of("Assertion"):
        r.checked += 1
        o = g.get(a.rel("oracle")[0]) if a.rel("oracle") else None
        if o is None:
            r.add(a.code, "Assertion has no oracle")
            continue
        tc = g.get(a.rel("testCase")[0]) if a.rel("testCase") else None
        cov = g.get(tc.rel("covers")[0]) if tc is not None and tc.rel("covers") else None
        expected_src = [o.attrs.get("expected", "")]
        if cov is not None:
            expected_src.append(cov.attrs.get("expected", ""))
        nums_a = set(NUM_RE.findall(a.attrs.get("expected", "")))
        if not any(set(NUM_RE.findall(s)) <= nums_a for s in expected_src):
            r.add(a.code, f"expected '{a.attrs.get('expected', '')[:80]}' does not carry the figures/error codes of oracle {o.code}")


@rule("TSO-013", BLOCK, "FaultModel có FaultInjection; timing=afterCommit có assertion về hiệu ứng phụ")
def tso_013(ctx, r):
    from ..specify.rules import SIDE_EFFECT_KINDS
    g = ctx.graph
    inj = _children(g, "FaultInjection", "realizes")
    asserts = _children(g, "Assertion", "testCase")
    for fm in g.of("FaultModel"):
        r.checked += 1
        fis = inj.get(fm.code, [])
        if not fis:
            r.add(fm.code, "no FaultInjection")
            continue
        if fm.attrs.get("timing") == "afterCommit":
            tcs = [tc for tc in g.of("TestCase") if set(tc.rel("injects")) & {x.code for x in fis}]
            if not tcs:
                r.add(fm.code, "no TestCase injecting the afterCommit fault")
            for tc in tcs:
                if not any(a.attrs.get("assertionKind") in SIDE_EFFECT_KINDS for a in asserts.get(tc.code, [])):
                    r.add(tc.code, f"injects {fm.code} afterCommit but has no side-effect assertion")


@rule("TSO-014", BLOCK, "Có idempotency thì phủ đủ: gửi lại, khác payload, trùng đồng thời")
def tso_014(ctx, r):
    g = ctx.graph
    by_subject = collections.defaultdict(set)
    for c in g.of("TestCondition"):
        by_subject[g.get(c.rel("subject")[0]).attrs["tddCode"]].add(c.attrs.get("key"))
    covered = {c for tc in g.of("TestCase") for cov in tc.rel("covers") for c in (g.get(cov).rel("realizes") if g.get(cov) else [])}
    for n in ctx.pkg.nodes.values():
        if not n.in_scope or not (n.props.get("idempotent") is True or n.cls == "IdempotencyPolicy"):
            continue
        if n.cls == "Endpoint" and str(n.props.get("method", "")).upper() in ("GET", "HEAD", "OPTIONS"):
            continue
        r.checked += 1
        need = {"replay", "payload-mismatch", "concurrent-dup"} | ({"after-retention"} if n.cls == "IdempotencyPolicy" else set())
        missing = need - by_subject.get(n.code, set())
        if missing:
            r.add(n.code, f"missing idempotency conditions: {sorted(missing)}")
        conds = [c for c in g.of("TestCondition") if g.get(c.rel("subject")[0]).attrs["tddCode"] == n.code
                 and c.attrs.get("key") in ("replay", "payload-mismatch", "concurrent-dup")]
        if any(True for _ in g.of("TestCase")):
            unc = [c.code for c in conds if c.code not in covered]
            if unc:
                r.add(n.code, f"idempotency conditions without a TestCase: {unc}")


@rule("TSO-015", BLOCK, "Điều kiện timing dùng trường hợp xấu nhất, không cộng p95 thành phần")
def tso_015(ctx, r):
    g = ctx.graph
    for c in g.of("TestCondition"):
        if c.attrs["conditionKind"] != "timing" or not str(c.attrs.get("key", "")).startswith("worst-"):
            continue
        r.checked += 1
        o = g.get(c.rel("oracle")[0]) if c.rel("oracle") else None
        m = str(o.attrs.get("measurement") or "") if o else ""
        if not re.search(r"xấu nhất|worst", m + c.attrs["statement"], re.I):
            r.add(c.code, "does not state how the worst case is computed")
        if re.search(r"p9\d\s*\+|cộng\s+p9\d", m + c.attrs["statement"], re.I) and not re.search(r"không cộng p9\d", m, re.I):
            r.add(c.code, "adds component percentiles instead of using the worst case")


@rule("TSO-017", WARN, "Mỗi điểm thực thi quyền có test cho phép và từ chối")
def tso_017(ctx, r):
    g = ctx.graph
    points = collections.defaultdict(lambda: {"allow": False, "deny": False})
    for c in g.of("TestCondition"):
        subj = g.get(c.rel("subject")[0]).attrs["tddCode"]
        n = ctx.pkg.get(subj)
        if n is None or n.cls == "DesignObligation":  # a design obligation is not an access enforcement point
            continue
        if c.attrs["conditionKind"] == "access":
            ep = next((b for b in c.prov.based_on if ctx.pkg.get(b) is not None and ctx.pkg.get(b).cls == "Endpoint"), subj)
            points[ep]["allow" if str(c.attrs.get("key", "")).startswith("allow") else "deny"] = True
        elif n.cls == "Response" and int(n.props.get("statusCode", 0) or 0) < 400:
            ep = next((b for b in c.prov.based_on if ctx.pkg.get(b) is not None and ctx.pkg.get(b).cls == "Endpoint"), None)
            if ep:
                points[ep]["allow"] = True
    # subjects that are not endpoints (FR, Scenario... when the LLM proposes access conditions): the "allow" case is a successful
    # functional condition of that subject itself
    for c in g.of("TestCondition"):
        if c.attrs["conditionKind"] != "functional":
            continue
        for p in {g.get(c.rel("subject")[0]).attrs["tddCode"], *c.prov.based_on}:
            if p in points:
                points[p]["allow"] = True
    for p, v in sorted(points.items()):
        if not v["deny"] and not any(c.attrs["conditionKind"] == "access" for c in g.of("TestCondition")
                                     if p in c.prov.based_on or g.get(c.rel("subject")[0]).attrs["tddCode"] == p):
            continue
        r.checked += 1
        if not (v["allow"] and v["deny"]):
            r.add(p, f"missing the {'allow' if not v['allow'] else 'deny'} test")


@rule("TSO-018", BLOCK, "Không có giá trị bí mật trong dữ liệu, bước, assertion")
def tso_018(ctx, r):
    from .secrets import SECRET_REF, scan_entity
    for e in ctx.graph.entities.values():
        if e.cls not in ("TestStep", "Assertion", "DataField", "TestData", "Fixture", "StubSpec", "TestCase", "TestEnvironment"):
            continue
        r.checked += 1
        hits = scan_entity(e)
        if hits:
            r.add(e.code, f"secret patterns: {hits}")
        if e.cls == "DataField" and e.attrs.get("sensitive") and e.attrs.get("valueClass") == "hợp lệ" \
                and not SECRET_REF.match(str(e.attrs.get("example", ""))):
            r.add(e.code, "sensitive field does not use secretRef")


@rule("TSO-019", BLOCK, "TR mức L1 đi qua ScenarioPath có ít nhất hai Module")
def tso_019(ctx, r):
    g = ctx.graph
    for tr in g.of("TestRequirement"):
        if tr.attrs.get("level") != "L1":
            continue
        r.checked += 1
        ok = False
        for c in tr.rel("groups"):
            ce = g.get(c)
            obl = g.get(ce.rel("obligation")[0]) if ce else None
            for path_code in (ce.rel("path") if ce else []):
                sc = g.get(g.get(path_code).rel("scenario")[0]).attrs["tddCode"] if g.get(path_code).rel("scenario") else None
                if sc and len(ctx.pkg.participants(sc)) >= 2:
                    ok = True
            if obl is not None and len(obl.rel("subject")) >= 3:
                ok = True
        if not ok:
            r.add(tr.code, "L1 TR does not go through a flow with at least two Modules")


@rule("TSO-020", BLOCK_BASELINE, "Nút llm chỉ baseline khi đã duyệt, trừ loại nút và ngưỡng policy cho phép")
def tso_020(ctx, r):
    pol = ctx.auto_approve
    for e in ctx.graph.entities.values():
        if e.prov.derivation != "llm":
            continue
        r.checked += 1
        st = ctx.review.get(e.code, "proposed")
        if st == "approved":
            continue
        if pol is not None and pol.enabled and e.cls in pol.classes and e.prov.confidence >= pol.min_confidence:
            continue
        r.add(e.code, f"llm node not reviewed ({st})")


@rule("TSO-021", BLOCK, "Phiên bản hóa: Baselined bất biến; supersedes thẳng; Retired có retiredBy")
def tso_021(ctx, r):
    probs = ctx.extra.get("versioning_problems", [])
    r.checked = len(ctx.extra.get("versioning_checked", [])) or (1 if "versioning_problems" in ctx.extra else 0)
    for p in probs:
        r.add(p.get("code", p.get("feature", "-")), p.get("message", str(p)))


GATES = {"GA": [f"TSO-{i:03d}" for i in range(1, 8)],
         "GB": [f"TSO-{i:03d}" for i in range(1, 11)] + ["TSO-016"],
         "GS": [f"TSO-{i:03d}" for i in range(1, 22)]}


def validate(ctx, gate="GA", rules=None):
    rep = Report(gate)
    rep.counts = dict(collections.Counter(e.cls for e in ctx.graph.entities.values()))
    for rid in rules or GATES[gate]:
        if rid not in RULES:
            continue
        level, title, fn = RULES[rid]
        res = RuleResult(rid, level, title)
        fn(ctx, res)
        rep.results.append(res)
    return rep


def text_has(pattern, *texts):
    return any(re.search(pattern, str(t or ""), re.I) for t in texts)


__all__ = ["Ctx", "GATES", "RULES", "validate", "WARN"]
