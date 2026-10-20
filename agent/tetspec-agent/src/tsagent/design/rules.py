"""Layer B design rules (T6.1, FR-006): technique by `conditionKind`, TRs grouped by (subject, level, testType), numbered
PassCriteria, TestDesign per (TR, technique), a CoverageItem per condition (a Partition yields one item per value).

Vietnamese strings in this module are deliberate: they become the content of the (Vietnamese) test-spec documents.
"""
from ..analyze import codes
from ..model import Evidence, Provenance, TsoEntity

TECHNIQUE = {
    "functional": "useCase", "inputValidation": "equivalencePartitioning", "stateTransition": "stateTransition",
    "guardViolation": "stateTransition", "invariant": "errorGuessing", "decisionRule": "decisionTable",
    "errorHandling": "faultInjection", "idempotency": "errorGuessing", "concurrency": "faultInjection",
    "timing": "boundaryValue", "atomicity": "faultInjection", "compensation": "faultInjection", "migration": "scenario",
    "compatibility": "scenario", "access": "accessMatrix", "secretExposure": "errorGuessing", "audit": "scenario",
    "performance": "loadModel", "resilience": "faultInjection", "observability": "faultInjection", "rollout": "scenario",
}
# acceptable techniques for each kind (TSO-010)
ALLOWED = {k: {v} for k, v in TECHNIQUE.items()}
ALLOWED["functional"] |= {"scenario", "equivalencePartitioning"}
ALLOWED["inputValidation"] |= {"boundaryValue"}
ALLOWED["invariant"] |= {"faultInjection"}
ALLOWED["idempotency"] |= {"faultInjection"}
ALLOWED["timing"] |= {"loadModel"}
ALLOWED["performance"] |= {"boundaryValue", "analysis"}
TEST_TYPE = {
    "functional": "functional", "inputValidation": "functional", "stateTransition": "functional", "guardViolation": "functional",
    "decisionRule": "functional", "invariant": "functional", "audit": "functional",
    "errorHandling": "reliability", "idempotency": "reliability", "concurrency": "reliability", "atomicity": "reliability",
    "compensation": "reliability", "resilience": "reliability",
    "timing": "performance", "performance": "performance",
    "access": "security", "secretExposure": "security",
    "compatibility": "compatibility", "migration": "compatibility",
    "observability": "maintainability", "rollout": "maintainability",
}
L3_CLASSES = {"Integration", "ErrorMapping", "CircuitBreaker", "PartnerLimit", "Channel", "SecurityScheme", "Mechanism",
              "TimeoutPolicy", "RetryPolicy"}
L3_KINDS = {"errorHandling", "timing", "idempotency", "concurrency", "resilience"}
TYPE_VN = {"functional": "chức năng", "reliability": "độ tin cậy", "performance": "hiệu năng", "security": "bảo mật",
           "compatibility": "tương thích", "maintainability": "khả năng bảo trì", "usability": "khả dụng"}
CRITERION = {
    "equivalencePartitioning": "Mỗi lớp tương đương một coverage item",
    "boundaryValue": "Mỗi biên hai giá trị (trong và ngoài biên)",
    "stateTransition": "Mỗi chuyển trạng thái và mỗi guard bị vi phạm",
    "useCase": "Mỗi tiêu chí chấp nhận một coverage item",
    "scenario": "Mỗi đường đi (chính, lỗi) một coverage item",
    "faultInjection": "Mỗi mô hình lỗi được tiêm ít nhất một lần",
    "errorGuessing": "Mỗi tình huống lỗi đoán trước một coverage item",
    "accessMatrix": "Mỗi điểm thực thi quyền × {cho phép, từ chối}",
    "loadModel": "Mỗi chỉ tiêu tải đo theo workload khai báo",
    "decisionTable": "Mỗi dòng bảng quyết định",
}


def technique_for(cond, g):
    kind = cond.attrs["conditionKind"]
    if kind == "inputValidation" and cond.rel("partition"):
        part = g.get(cond.rel("partition")[0])
        if part is not None and part.attrs.get("classKind") == "boundary":
            return "boundaryValue"
    if kind in ("functional", "errorHandling") and cond.rel("path"):
        return "scenario"
    return TECHNIQUE.get(kind, "errorGuessing")


def level_for(cond, g, pkg):
    subj = g.get(cond.rel("subject")[0]).attrs
    if cond.rel("path"):
        return "L1"
    obl = g.get(cond.rel("obligation")[0])
    osubj = g.get(codes.basis(g.feature, codes.primary_subject(g, obl))).attrs
    if osubj["tddClass"] == "Scenario" and len(obl.rel("subject")) >= 3:
        return "L1"
    if subj["tddClass"] in L3_CLASSES or osubj["tddClass"] in L3_CLASSES:
        return "L3"
    if osubj["tddClass"] == "Module" and cond.attrs["conditionKind"] in L3_KINDS:
        return "L3"
    return "L2"


def values_of(part):
    """Partition values normalised to [(value, valid)]."""
    out = []
    for v in part.attrs.get("values") or []:
        if isinstance(v, dict) and "value" in v:
            out.append((v["value"], bool(v.get("valid", part.attrs.get("valid")))))
        else:
            out.append((v, bool(part.attrs.get("valid"))))
    return out


class Designer:
    def __init__(self, g, pkg, run_id=""):
        self.g, self.pkg, self.f, self.run_id = g, pkg, g.feature, run_id
        self.created = []

    def _ent(self, cls, code, attrs, based_on, rels, derivation="rule", evidence=()):
        e = TsoEntity(cls, code, {k: v for k, v in attrs.items() if v is not None}, {},
                      Provenance(derivation, sorted(set(based_on)), list(evidence), confidence=0.7, run_id=self.run_id))
        for k, v in rels.items():
            e.link(k, *(v if isinstance(v, (list, tuple)) else [v]))
        self.g.add(e)
        self.created.append(code)
        return e

    def run(self):
        g = self.g
        groups = {}
        for c in g.of("TestCondition"):
            obl = g.get(c.rel("obligation")[0])
            subj = codes.primary_subject(g, obl)
            key = (subj, level_for(c, g, self.pkg), TEST_TYPE.get(c.attrs["conditionKind"], "functional"))
            groups.setdefault(key, []).append(c)
        per_subject = {}
        for (subj, lvl, tt) in groups:
            per_subject.setdefault(subj, []).append((lvl, tt))
        td_names = {}
        for (subj, lvl, tt), cs in groups.items():
            for c in cs:
                td_names.setdefault((subj, lvl, technique_for(c, g)), set()).add(tt)
        for (subj, lvl, tt) in sorted(groups):
            self.requirement(subj, lvl, tt, sorted(groups[(subj, lvl, tt)], key=lambda c: c.code), per_subject[subj], td_names)
        return self

    def requirement(self, subj, lvl, tt, conds, siblings, td_names):
        g = self.g
        node = self.pkg.get(subj)
        name = node.base.get("label") or node.code if node else subj
        obls = sorted({c.rel("obligation")[0] for c in conds})
        prio = min(g.get(o).attrs["priority"] for o in obls)
        kinds = sorted({c.attrs["conditionKind"] for c in conds})
        verification = "benchmark" if all(k == "performance" for k in kinds) else "test"
        code = codes.requirement(self.f, lvl, subj, tt)
        others = [f"{x[0]}/{x[1]}" for x in sorted(siblings) if x != (lvl, tt)]
        stmt = (f"Kiểm {len(conds)} điều kiện {TYPE_VN.get(tt, tt)} ({', '.join(kinds)}) của {subj} ở mức {lvl}: "
                + "; ".join(c.attrs["statement"][:80] for c in conds[:3]) + ("; …" if len(conds) > 3 else ""))
        scope = (f"Chỉ phần {TYPE_VN.get(tt, tt)} mức {lvl} của {subj} ({name})"
                 + (f"; phần khác của cùng chủ thể nằm ở TR {', '.join(others)}" if others else ""))
        based = sorted({b for c in conds for b in c.prov.based_on})
        basis = g.get(conds[0].rel("subject")[0])
        subj_basis = codes.basis(self.f, subj)
        tr = self._ent("TestRequirement", code, {"statement": stmt, "level": lvl, "testType": tt, "priority": prio,
                                                 "verificationMethod": verification, "scopeNote": scope},
                       based, {"subject": subj_basis if g.get(subj_basis) else basis.code, "groups": [c.code for c in conds]},
                       evidence=[Evidence(subj, name)] if subj in self.pkg.nodes else ())
        for i, c in enumerate(conds, 1):
            self._ent("PassCriterion", codes.pass_criterion(code, i),
                      {"ordinal": i, "text": c.attrs["expectedEffect"] if c.attrs["statement"] == c.attrs["expectedEffect"]
                       else f"{c.attrs['statement']} → {c.attrs['expectedEffect']}"},
                      c.prov.based_on, {"requirement": code, "condition": c.code})
        by_tech = {}
        for c in conds:
            by_tech.setdefault(technique_for(c, g), []).append(c)
        for tech in sorted(by_tech):
            name_t = tech if len(td_names.get((subj, lvl, tech), ())) <= 1 else f"{tech}-{tt}"
            td = codes.design(self.f, lvl, subj, name_t)
            self._ent("TestDesign", td, {"technique": tech, "coverageCriterion": CRITERION.get(tech, "Mỗi điều kiện một coverage item"),
                                         "coverageTarget": 100,
                                         "rationale": f"{tech} phù hợp {', '.join(sorted({c.attrs['conditionKind'] for c in by_tech[tech]}))}"},
                      based, {"addresses": tr.code})
            for c in by_tech[tech]:
                self.coverage(td, lvl, c)
        return tr

    def coverage(self, td, lvl, c):
        g = self.g
        orc = g.get(c.rel("oracle")[0])
        part = g.get(c.rel("partition")[0]) if c.rel("partition") else None
        if part is not None and values_of(part):
            for i, (v, valid) in enumerate(values_of(part)):
                key = f"v{i}"
                exp = orc.attrs["expected"] if not valid else f"{part.attrs['parameter']} = {v} được chấp nhận"
                self._ent("CoverageItem", codes.coverage(self.f, lvl, c.code, key),
                          {"coverageKind": "partitionValue" if valid is False else "validBoundary" if part.attrs.get("classKind") == "boundary" else "validClass",
                           "description": f"{part.attrs['parameter']} = {v} ({'hợp lệ' if valid else 'không hợp lệ'})",
                           "expected": exp, "value": {"value": v, "valid": valid}},
                          c.prov.based_on, {"design": td, "realizes": c.code, "expectedFrom": orc.code})
            return
        kind = "path" if c.rel("path") else "faultModel" if c.rel("faultModel") else "condition"
        self._ent("CoverageItem", codes.coverage(self.f, lvl, c.code, "c"),
                  {"coverageKind": kind, "description": c.attrs["statement"], "expected": orc.attrs["expected"]},
                  c.prov.based_on, {"design": td, "realizes": c.code, "expectedFrom": orc.code})


B_CLASSES = ("TestRequirement", "PassCriterion", "TestDesign", "CoverageItem")


def design(g, pkg, run_id=""):
    """Add layer B to `g` (which already has layer A). Returns the Designer (`.created`)."""
    return Designer(g, pkg, run_id).run()
