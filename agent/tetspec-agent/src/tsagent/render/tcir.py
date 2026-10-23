"""TCIR (Test Case Intermediate Representation): the machine-readable form of each test case, for the test-script agent.

One TCIR per TestCase, built only from the test KG. Every expected result keeps a stable id `<TC>.E<n>` so a generated script can tag
the assertion that checks it and a checker can match them one to one. Data fields are value classes, not fixed values: the test script
generates its own data at run time (secret fields carry no example at all).
"""
import collections

SCHEMA_VERSION = "tcir/v1"


class Index:
    def __init__(self, g):
        self.g = g
        self.kids = collections.defaultdict(list)
        for e in g.entities.values():
            for rel, ts in e.rels.items():
                for t in ts:
                    self.kids[(t, rel)].append(e)
        for k in self.kids:
            self.kids[k].sort(key=lambda e: (e.attrs.get("ordinal", 0), e.code))

    def back(self, code, rel, cls):
        return [e for e in self.kids.get((code, rel), []) if e.cls == cls]

    def all(self, codes):
        return [x for x in (self.g.get(c) for c in codes) if x is not None]


def _review(g, code):
    return ((g.meta.get("lifecycle") or {}).get(code) or {}).get("review_status") or "proposed"


def _field(f):
    out = {"name": f.attrs["name"], "valueClass": f.attrs["valueClass"], "sensitive": bool(f.attrs.get("sensitive"))}
    if not out["sensitive"]:
        out["example"] = f.attrs.get("example")
    return out


def test_case(g, tc, ix=None):
    """TCIR dict of one TestCase entity of graph `g`."""
    ix = ix or Index(g)
    cov = ix.all(tc.rel("covers"))
    conds = ix.all(sorted({c for cv in cov for c in cv.rel("realizes")}))
    trs = sorted({tr.code for c in conds for tr in ix.back(c.code, "groups", "TestRequirement")})
    tr_ents = ix.all(trs)
    basis = sorted({g.get(b).attrs["tddCode"] for c in conds for b in c.rel("subject") if g.get(b) is not None})
    steps = ix.back(tc.code, "testCase", "TestStep")
    asserts = ix.back(tc.code, "testCase", "Assertion")
    data = []
    for d in ix.all(tc.rel("needsData")):
        data.append({"code": d.code, "entity": d.attrs["entity"], "lifecycle": d.attrs["lifecycle"], "sensitivity": d.attrs["sensitivity"],
                     "fields": [_field(f) for f in ix.back(d.code, "data", "DataField")]})
    expected = []
    for a in asserts:
        o = g.get(a.rel("oracle")[0]) if a.rel("oracle") else None
        item = {"id": f"{tc.code}.E{a.attrs['ordinal']}", "assertion": a.code, "kind": a.attrs["assertionKind"], "op": a.attrs["operator"],
                "value": a.attrs["expected"]}
        if a.attrs.get("unit"):
            item["unit"] = a.attrs["unit"]
        if a.attrs.get("description"):
            item["description"] = a.attrs["description"]
        if o is not None:
            item["oracle"] = {"code": o.code, "kind": o.attrs["oracleKind"], "expected": o.attrs["expected"],
                              **({"tolerance": o.attrs["tolerance"]} if o.attrs.get("tolerance") else {})}
        expected.append(item)
    return {
        "schemaVersion": SCHEMA_VERSION,
        "id": tc.code,
        "feature": g.feature,
        "specVersion": g.meta.get("version"),
        "objective": tc.attrs["objective"],
        "level": tc.attrs["level"],
        "priority": tc.attrs["priority"],
        "testTypes": sorted({tr.attrs["testType"] for tr in tr_ents}),
        "automatable": bool(tc.attrs.get("automatable")),
        "review": {"derivation": tc.prov.derivation, "status": _review(g, tc.code)},
        "preconditions": list(tc.attrs.get("preconditions") or []),
        "fixtures": [{"code": x.code, "kind": x.attrs["kind"], "setup": x.attrs["setup"], "teardown": x.attrs["teardown"]}
                     for x in ix.all(tc.rel("needsFixture"))],
        "stubs": [{"code": x.code, "target": x.attrs["targetCode"], "behavior": x.attrs["behavior"],
                   **{k: x.attrs[k] for k in ("statusCode", "delayMs", "response") if x.attrs.get(k) is not None}}
                  for x in ix.all(tc.rel("needsStub"))],
        "faultInjections": [{"code": x.code, "mechanism": x.attrs["mechanism"], "trigger": x.attrs["trigger"]}
                            for x in ix.all(tc.rel("injects"))],
        "data": {"policy": "generate", "items": data},
        "steps": [{"id": f"{tc.code}.S{s.attrs['ordinal']}", "kind": s.attrs["stepKind"], "action": s.attrs["action"],
                   **({"input": s.attrs["input"]} if s.attrs.get("input") is not None else {}),
                   **({"target": s.attrs["targetCode"]} if s.attrs.get("targetCode") else {})} for s in steps],
        "expected": expected,
        "trace": {"coverageItems": [c.code for c in cov], "conditions": [c.code for c in conds], "requirements": trs, "basis": basis,
                  "testSet": tc.rel("inSet")},
    }


def bundle(g, codes=None, level=None):
    """TCIR of every TestCase of `g` (or of `codes`, or of one `level`), in code order."""
    ix = Index(g)
    want = set(codes) if codes else None
    tcs = [tc for tc in g.of("TestCase") if (want is None or tc.code in want) and (level is None or tc.attrs["level"] == level)]
    return {"schemaVersion": SCHEMA_VERSION, "feature": g.feature, "specVersion": g.meta.get("version"), "status": g.meta.get("status"),
            "baseline": g.meta.get("baseline"), "testCases": [test_case(g, tc, ix) for tc in tcs]}
