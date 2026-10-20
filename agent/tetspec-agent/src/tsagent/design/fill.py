"""LLM fill of layer B (T6.2): statement, scopeNote, PassCriterion wording, CoverageItem descriptions and values."""
import os

from ..analyze import codes
from ..analyze.context import node_view
from ..stepfill import StepFiller
from .rules import values_of


class DesignFiller(StepFiller):
    step = "design"
    schema = "design"
    prompt_dir = os.path.join(os.path.dirname(__file__), "prompts")

    def groups(self):
        g = self.g
        by_subject = {}
        for tr in g.of("TestRequirement"):
            subj = g.get(tr.rel("subject")[0]).attrs["tddCode"]
            by_subject.setdefault(subj, []).append(tr)
        for subj in sorted(by_subject):
            trs = by_subject[subj]
            pack = {"subject": node_view(self.pkg.get(subj)) if self.pkg.get(subj) else {"code": subj},
                    "requirements": [self.tr_view(tr) for tr in trs]}
            allowed = {subj} | {c for tr in trs for c in tr.rel("groups")} | {x["code"] for tr in pack["requirements"] for x in tr["coverage"]}
            allowed |= {b for tr in trs for b in tr.prov.based_on}
            pack["allowedCodes"] = sorted(allowed)
            yield subj, pack, {tr.code for tr in trs}

    def tr_view(self, tr):
        g = self.g
        conds = []
        for c in tr.rel("groups"):
            ce = g.get(c)
            o = g.get(ce.rel("oracle")[0]).attrs
            v = {"code": c, "conditionKind": ce.attrs["conditionKind"], "statement": ce.attrs["statement"],
                 "expectedEffect": ce.attrs["expectedEffect"], "oracle": {k: o[k] for k in ("oracleKind", "expected", "measurement") if o.get(k)}}
            if ce.rel("partition"):
                v["partitionValues"] = [x[0] for x in values_of(g.get(ce.rel("partition")[0]))]
            conds.append(v)
        cov = [{"code": e.code, "condition": e.rel("realizes")[0], "description": e.attrs["description"], "value": e.attrs.get("value")}
               for e in g.of("CoverageItem") if g.get(e.rel("design")[0]).rel("addresses")[0] == tr.code]
        return {"seedCode": tr.code, "level": tr.attrs["level"], "testType": tr.attrs["testType"], "statement": tr.attrs["statement"],
                "scopeNote": tr.attrs["scopeNote"], "conditions": conds, "coverage": cov}

    def problems(self, out, pack, seeds):
        keep, probs = {"requirements": []}, []
        trs = {t["seedCode"]: t for t in pack["requirements"]}
        for i, r in enumerate(out.get("requirements", [])):
            p = []
            t = trs.get(r["seedCode"])
            if t is None:
                probs.append(f"requirements[{i}].seedCode {r['seedCode']} is not in the pack")
                continue
            conds = {c["code"]: c for c in t["conditions"]}
            got = [pc["conditionCode"] for pc in r["passCriteria"]]
            if sorted(got) != sorted(conds):
                p.append(f"requirements[{i}].passCriteria must have exactly one entry per condition {sorted(conds)}, got {got}")
            subj_label = str(pack["subject"].get("label", ""))
            if subj_label and r["statement"].strip().lower() == subj_label.strip().lower():
                p.append(f"requirements[{i}].statement copies the name of the source requirement")
            cov = {x["code"]: x for x in t["coverage"]}
            for j, cv in enumerate(r.get("coverage", [])):
                if cv["seedCode"] not in cov:
                    p.append(f"requirements[{i}].coverage[{j}].seedCode {cv['seedCode']} does not exist")
                    continue
                cond = conds.get(cov[cv["seedCode"]]["condition"], {})
                allowed_vals = cond.get("partitionValues")
                if "value" in cv and allowed_vals is not None and cv["value"] not in allowed_vals \
                        and cv["value"] != (cov[cv["seedCode"]].get("value") or {}).get("value"):
                    p.append(f"requirements[{i}].coverage[{j}].value {cv['value']!r} is outside the Partition {allowed_vals}")
            if p:
                probs += p
            else:
                keep["requirements"].append(r)
        return keep, probs

    def apply(self, subj, out, ph):
        g = self.g
        for r in out.get("requirements", []):
            tr = g.get(r["seedCode"])
            tr.attrs["statement"], tr.attrs["scopeNote"] = r["statement"], r["scopeNote"]
            self.mark(tr, ph, 0.8, [(subj, r["statement"][:200])])
            order = {c: i for i, c in enumerate(tr.rel("groups"), 1)}
            for pc in r["passCriteria"]:
                e = g.get(codes.pass_criterion(tr.code, order[pc["conditionCode"]]))
                if e is not None and e.rel("condition") == [pc["conditionCode"]]:
                    e.attrs["text"] = pc["text"]
                    self.mark(e, ph, 0.8, [(b, "") for b in e.prov.based_on[:1]])
            for cv in r.get("coverage", []):
                e = g.get(cv["seedCode"])
                if cv.get("description"):
                    e.attrs["description"] = cv["description"]
                if "value" in cv and cv["value"] is not None:
                    old = e.attrs.get("value")
                    e.attrs["value"] = {**old, "value": cv["value"]} if isinstance(old, dict) else cv["value"]
                self.mark(e, ph, 0.7, [(b, "") for b in e.prov.based_on[:1]])
            self.res.filled.append(tr.code)
