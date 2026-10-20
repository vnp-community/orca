"""LLM fill of layer C (T6.3): steps, data, fixtures, assertion descriptions, fault injection mechanisms. The expected result of an
assertion comes from the oracle and the LLM cannot change it. New steps replace all skeleton steps of the TC; items with secret patterns are dropped (TSO-018)."""
import os

from ..analyze import codes
from ..stepfill import StepFiller
from ..validate.secrets import SECRET_REF, find_secrets, walk_strings

CHUNK = 15


class SpecFiller(StepFiller):
    step = "specification"
    schema = "specification"
    prompt_dir = os.path.join(os.path.dirname(__file__), "prompts")

    def children(self, cls, rel, tc):
        return sorted((e for e in self.g.of(cls) if e.rel(rel) == [tc]), key=lambda e: (e.attrs.get("ordinal", 0), e.code))

    def tc_view(self, tc):
        g = self.g
        t = g.get(tc)
        cov = g.get(t.rel("covers")[0])
        cond = g.get(cov.rel("realizes")[0])
        v = {"seedCode": tc, "objective": t.attrs["objective"], "level": t.attrs["level"],
             "condition": {"statement": cond.attrs["statement"], "expectedEffect": cond.attrs["expectedEffect"],
                           "preState": cond.attrs.get("preState"), "trigger": cond.attrs.get("trigger")},
             "coverage": {"description": cov.attrs["description"], "value": cov.attrs.get("value")},
             "steps": [{k: s.attrs.get(k) for k in ("stepKind", "action", "input", "targetCode") if s.attrs.get(k) is not None}
                       for s in self.children("TestStep", "testCase", tc)],
             "assertions": [{"seedCode": a.code, "expected": a.attrs["expected"], "assertionKind": a.attrs["assertionKind"]}
                            for a in self.children("Assertion", "testCase", tc)],
             "data": [{"seedCode": f.code, "name": f.attrs["name"], "example": f.attrs["example"], "sensitive": f.attrs["sensitive"]}
                      for d in t.rel("needsData") for f in self.children("DataField", "data", d)],
             "fixtures": [{"seedCode": x, **{k: g.get(x).attrs[k] for k in ("setup", "teardown")}} for x in t.rel("needsFixture")],
             "faultInjections": [{"seedCode": x, **{k: g.get(x).attrs[k] for k in ("mechanism", "trigger")}} for x in t.rel("injects")],
             "stubs": [{k: g.get(x).attrs.get(k) for k in ("targetCode", "behavior", "statusCode")} for x in t.rel("needsStub")]}
        return v

    def groups(self):
        g = self.g
        by_set = {}
        for tc in g.of("TestCase"):
            by_set.setdefault(tc.rel("inSet")[0] if tc.rel("inSet") else "-", []).append(tc.code)
        for ts in sorted(by_set):
            tcs = sorted(by_set[ts])
            for i in range(0, len(tcs), CHUNK):
                part = tcs[i:i + CHUNK]
                views = [self.tc_view(t) for t in part]
                allowed = set()
                for t in part:
                    e = g.get(t)
                    allowed |= set(e.prov.based_on)
                    for s in self.children("TestStep", "testCase", t):
                        if s.attrs.get("targetCode"):
                            allowed.add(s.attrs["targetCode"])
                    allowed |= {x["seedCode"] for v in views for k in ("assertions", "data", "fixtures", "faultInjections") for x in v[k]}
                pack = {"testSet": ts, "testcases": views, "allowedCodes": sorted(allowed | set(part))}
                yield f"{ts}#{i // CHUNK}", pack, set(part)

    def problems(self, out, pack, seeds):
        allowed = set(pack["allowedCodes"])
        views = {v["seedCode"]: v for v in pack["testcases"]}
        keep, probs = {"testcases": []}, []
        for i, t in enumerate(out.get("testcases", [])):
            p = []
            v = views.get(t["seedCode"])
            if v is None:
                probs.append(f"testcases[{i}].seedCode {t['seedCode']} is not in the pack")
                continue
            if not any(s["stepKind"] == "when" for s in t["steps"]):
                p.append(f"testcases[{i}] has no when step")
            bad = [s["targetCode"] for s in t["steps"] if s.get("targetCode") and s["targetCode"] not in allowed]
            if bad:
                p.append(f"testcases[{i}].steps targetCode not on the allow list: {bad}")
            own = {k: {x["seedCode"] for x in v[k]} for k in ("assertions", "data", "fixtures", "faultInjections")}
            for k in own:
                stray = [x["seedCode"] for x in t.get(k, []) if x["seedCode"] not in own[k]]
                if stray:
                    p.append(f"testcases[{i}].{k} has seedCodes outside the TC: {stray}")
            sens = {x["seedCode"] for x in v["data"] if x["sensitive"]}
            for x in t.get("data", []):
                if x["seedCode"] in sens and not SECRET_REF.match(x["example"]):
                    p.append(f"testcases[{i}].data {x['seedCode']}: sensitive fields must use secretRef:<name>")
            hits = sorted({h for s in walk_strings(t) for h in find_secrets(s)})
            if hits:
                p.append(f"testcases[{i}] contains secret patterns: {hits}")
            if p:
                probs += p
            else:
                keep["testcases"].append(t)
        return keep, probs

    def apply(self, key, out, ph):
        g = self.g
        for t in out.get("testcases", []):
            tc = g.get(t["seedCode"])
            tc.attrs["objective"] = t["objective"]
            tc.attrs["preconditions"] = list(t.get("preconditions") or [])
            self.mark(tc, ph, 0.75)
            old = self.children("TestStep", "testCase", tc.code)
            for s in old:
                del g.entities[s.code]
            for i, s in enumerate(t["steps"], 1):
                from ..model import Provenance, TsoEntity
                e = TsoEntity("TestStep", codes.step(tc.code, i),
                              {k: v for k, v in {"ordinal": i, "stepKind": s["stepKind"], "action": s["action"],
                                                 "input": s.get("input"), "targetCode": s.get("targetCode")}.items() if v is not None},
                              {"testCase": [tc.code]}, Provenance("llm", list(tc.prov.based_on), run_id=self.run_id))
                g.add(e)
                self.mark(e, ph, 0.75)
            for k, fields in (("assertions", ("description",)), ("data", ("example",)), ("fixtures", ("setup", "teardown")),
                              ("faultInjections", ("mechanism", "trigger"))):
                for x in t.get(k, []):
                    e = g.get(x["seedCode"])
                    for f in fields:
                        if x.get(f):
                            e.attrs[f] = x[f]
                    self.mark(e, ph, 0.75)
            self.res.filled.append(tc.code)
