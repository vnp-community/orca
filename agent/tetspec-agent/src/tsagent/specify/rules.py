"""Layer C specification rules (T6.3, FR-007): a TestCase per CoverageItem, given/when/then steps, Assertions taking the result from the oracle,
data from Partitions, Fixtures from the pre-state, StubSpecs for the dependencies of L3 TCs, a FaultInjection for every FaultModel,
TestSets by (level, subject), TestEnvironments by level.

Vietnamese strings in this module are deliberate: they become the content of the (Vietnamese) test-spec documents.
"""
import re

from ..analyze import codes
from ..model import Provenance, TsoEntity

ASSERTION_KIND = {"errorContract": "errorCode", "threshold": "threshold", "stateMachine": "state", "invariant": "count",
                  "postcondition": "state", "schema": "status", "acceptanceCriterion": "value", "reference": "value",
                  "decisionRow": "value", "humanJudgment": "value"}
SIDE_EFFECT_KINDS = {"record", "count", "event", "state", "absence"}
UNIT_RE = re.compile(r"(\d[\d.,]*)\s*(ms|s|giây|phút|%|rps|request/giây|KB|MB)\b")
HTTP_RE = re.compile(r"HTTP\s*(\d{3})")
SENSITIVE_RE = re.compile(r"(?i)authori|api[ -]?key|password|token|secret|pin\b|otp")
ENV = {"L1": ("Môi trường tích hợp đầu-cuối", ["toàn bộ service của feature", "CSDL", "hàng đợi", "stub đối tác ngoài"]),
       "L2": ("Môi trường service", ["service đang kiểm", "CSDL thật", "stub phụ thuộc ngoài"]),
       "L3": ("Môi trường service cô lập", ["service đang kiểm", "stub mọi phụ thuộc", "đồng hồ giả"])}
STUB_BEHAVIOR = (("unknownOutcome", "timeout"), ("timeout", "timeout"), ("transient", "errorTransient"), ("5xx", "errorTransient"),
                 ("503", "errorTransient"), ("429", "errorTransient"), ("permanent", "errorPermanent"), ("4xx", "errorPermanent"),
                 ("40", "errorPermanent"), ("schema", "malformedPayload"), ("dependencyFailure", "errorTransient"))
INJECTION = {"processCrash": "Dừng cưỡng bức tiến trình (SIGKILL) tại điểm tiêm, rồi khởi động lại",
             "poisonMessage": "Đưa một message hỏng vào kênh",
             "dependencyFailure": "Stub phụ thuộc trả lỗi hoặc không phản hồi"}


def stub_behavior(fault_kind, trigger=""):
    s = f"{fault_kind} {trigger}"
    for k, b in STUB_BEHAVIOR:
        if k.lower() in s.lower():
            return b
    return "errorTransient"


def assertion_kind(oracle):
    kind = ASSERTION_KIND.get(oracle.attrs["oracleKind"], "value")
    if HTTP_RE.search(oracle.attrs.get("expected", "")):
        return "status"
    return kind


class Specifier:
    def __init__(self, g, pkg, run_id=""):
        self.g, self.pkg, self.f, self.run_id = g, pkg, g.feature, run_id
        self.created = []

    def _ent(self, cls, code, attrs, based_on, rels=None):
        if self.g.get(code) is not None:
            e = self.g.get(code)
            for k, v in (rels or {}).items():
                e.link(k, *(v if isinstance(v, (list, tuple)) else [v]))
            return e
        e = TsoEntity(cls, code, {k: v for k, v in attrs.items() if v is not None}, {},
                      Provenance("rule", sorted({b for b in based_on if b in self.pkg.nodes}), confidence=0.7, run_id=self.run_id))
        for k, v in (rels or {}).items():
            e.link(k, *(v if isinstance(v, (list, tuple)) else [v]))
        self.g.add(e)
        self.created.append(code)
        return e

    def run(self):
        g = self.g
        for fm in g.of("FaultModel"):
            self.injection(fm)
        sets = {}
        for cov in g.of("CoverageItem"):
            td = g.get(cov.rel("design")[0])
            tr = g.get(td.rel("addresses")[0])
            tc = self.testcase(cov, tr)
            subj = g.get(tr.rel("subject")[0]).attrs["tddCode"]
            sets.setdefault((tr.attrs["level"], subj), []).append(tc.code)
        for i, (lvl, subj) in enumerate(sorted(sets), 1):
            env = codes.environment(self.f, lvl)
            name, services = ENV[lvl]
            self._ent("TestEnvironment", env, {"name": f"{name} ({lvl})", "services": services, "secretRef": ["secretRef:api-key-test"]},
                      [subj])
            ts = codes.test_set(self.f, lvl, subj)
            self._ent("TestSet", ts, {"purpose": f"Các test case mức {lvl} của {subj}", "ordinal": i,
                                      "entryCriteria": [f"{name} sẵn sàng", "Dữ liệu và stub của tập đã nạp"],
                                      "exitCriteria": ["100% test case của tập được chạy", "0 test case P0/P1 thất bại chưa xử lý"]},
                      [subj], {"runsIn": env})
            for tc in sets[(lvl, subj)]:
                g.get(tc).link("inSet", ts)
        return self

    def injection(self, fm):
        a = fm.attrs
        mech = INJECTION.get(a["faultKind"]) or f"Stub {a['injectionPoint']} tạo lỗi {a['faultKind']}"
        when = {"before": "trước khi thao tác chính ghi dữ liệu", "during": "trong khi thao tác chính đang chạy",
                "afterCommit": "ngay sau khi giao dịch đã commit, trước khi trả kết quả", "beforeAck": "trước khi gửi xác nhận"}
        self._ent("FaultInjection", codes.fault_injection(fm.code),
                  {"mechanism": mech, "trigger": f"Tại {a['injectionPoint']}, {when.get(a['timing'], a['timing'])}"},
                  fm.prov.based_on, {"realizes": fm.code})

    def target(self, cond):
        """Code of the TDD node called by the `when` step: the Endpoint for Field/Response, the Scenario/Integration/Module otherwise."""
        subj = self.g.get(cond.rel("subject")[0]).attrs["tddCode"]
        n = self.pkg.get(subj)
        if n is not None and n.cls in ("Field", "Response"):
            for b in cond.prov.based_on:
                m = self.pkg.get(b)
                if m is not None and m.cls == "Endpoint":
                    return b
        if n is not None and n.cls in ("AcceptanceCriterion",):
            for b in cond.prov.based_on:
                m = self.pkg.get(b)
                if m is not None and m.cls in ("FunctionalRequirement", "NonFunctionalRequirement"):
                    eps = self.pkg.sources(b, "implements", "Endpoint")
                    return eps[0].code if eps else b
        return subj

    def testcase(self, cov, tr):
        g = self.g
        cond = g.get(cov.rel("realizes")[0])
        orc = g.get(cov.rel("expectedFrom")[0])
        lvl = tr.attrs["level"]
        key = cov.code.rsplit("-", 1)[-1]
        code = codes.testcase(lvl, self.f, cond.code, key)
        value = cov.attrs.get("value")
        val, valid = (value.get("value"), value.get("valid")) if isinstance(value, dict) else (value, None)
        objective = f"Chứng minh: {cond.attrs['statement']}" + (f" (giá trị {val})" if val is not None else "")
        based = sorted(set(cond.prov.based_on))
        tc = self._ent("TestCase", code, {"objective": objective, "level": lvl, "priority": tr.attrs["priority"],
                                          "automatable": tr.attrs["verificationMethod"] != "inspection", "preconditions": []},
                       based, {"covers": cov.code})
        target = self.target(cond)
        steps = []
        if cond.attrs.get("preState"):
            steps.append(("given", f"Chuẩn bị: {cond.attrs['preState']}", None))
        else:
            steps.append(("given", "Hệ thống ở trạng thái sẵn sàng, dữ liệu nền đã nạp", None))
        fm = g.get(cond.rel("faultModel")[0]) if cond.rel("faultModel") else None
        if fm is not None:
            steps.append(("given", f"Bật tiêm lỗi: {fm.attrs['faultKind']} tại {fm.attrs['injectionPoint']} ({fm.attrs['timing']})", None))
        action = cond.attrs.get("trigger") or cond.attrs["statement"]
        inp = None
        if val is not None:
            part = g.get(cond.rel("partition")[0]) if cond.rel("partition") else None
            inp = {part.attrs["parameter"] if part else "value": val}
            action = f"Gửi yêu cầu tới {target} với {part.attrs['parameter'] if part else 'giá trị'} = {val}"
        steps.append(("when", action, inp))
        steps.append(("then", f"Quan sát kết quả: {cov.attrs['expected']}", None))
        for i, (kind, act, inp_) in enumerate(steps, 1):
            self._ent("TestStep", codes.step(code, i), {"ordinal": i, "stepKind": kind, "action": act, "input": inp_,
                                                         "targetCode": target if kind == "when" else None},
                      based, {"testCase": code})
        exp = orc.attrs["expected"] if valid is not True else cov.attrs["expected"]
        unit = (UNIT_RE.search(exp) or [None, None, None])[2] if orc.attrs["oracleKind"] == "threshold" else None
        op = "<=" if orc.attrs["oracleKind"] == "threshold" else "=="
        ak = assertion_kind(orc) if valid is not True else "status"
        self._ent("Assertion", codes.assertion(code, 1), {"ordinal": 1, "assertionKind": ak, "operator": op, "expected": exp,
                                                          "unit": unit, "description": f"Kết quả khớp oracle {orc.code}"},
                  based, {"testCase": code, "oracle": orc.code})
        if fm is not None and fm.attrs["timing"] == "afterCommit":
            self._ent("Assertion", codes.assertion(code, 2), {"ordinal": 2, "assertionKind": "count", "operator": "==",
                                                              "expected": f"Hiệu ứng phụ sau commit đúng một lần: {orc.attrs['expected']}",
                                                              "description": "Không trùng, không mất hiệu ứng phụ sau commit"},
                      based, {"testCase": code, "oracle": orc.code})
            tc.link("injects", codes.fault_injection(fm.code))
        elif fm is not None:
            tc.link("injects", codes.fault_injection(fm.code))
        if val is not None:
            part = g.get(cond.rel("partition")[0]) if cond.rel("partition") else None
            pname = part.attrs["parameter"] if part else "value"
            sensitive = bool(SENSITIVE_RE.search(pname))
            data = codes.test_data(code)
            self._ent("TestData", data, {"entity": target, "lifecycle": "tạo trong test, xóa ở cleanup",
                                         "sensitivity": "secret" if sensitive else "none"}, based)
            self._ent("DataField", codes.data_field(code, pname),
                      {"name": pname, "valueClass": {True: "hợp lệ", False: "không hợp lệ"}.get(valid, "chưa xác định"),
                       "example": f"secretRef:{codes.norm(pname).lower()}-test" if sensitive and valid else str(val),
                       "sensitive": sensitive}, based, {"data": data})
            tc.link("needsData", data)
        if cond.attrs.get("preState"):
            fx = codes.fixture(self.f, target, codes.short_hash(cond.attrs["preState"]))
            self._ent("Fixture", fx, {"kind": "state", "setup": f"Đưa hệ thống về: {cond.attrs['preState']}",
                                      "teardown": "Xóa dữ liệu do fixture tạo"}, based)
            tc.link("needsFixture", fx)
        if lvl == "L3":
            for dep, behavior, status in self.stubs(cond, fm):
                st = codes.stub(self.f, dep, behavior, status)
                self._ent("StubSpec", st, {"targetCode": dep, "behavior": behavior, "statusCode": status,
                                           "delayMs": None}, [dep])
                tc.link("needsStub", st)
        return tc

    def stubs(self, cond, fm):
        """(dependency code, behaviour, HTTP status) for L3 TCs: dependencies of the subject (dependsOn, outbound integratesVia)."""
        out = []
        subj = self.g.get(cond.rel("subject")[0]).attrs["tddCode"]
        ip = self.pkg.get(fm.attrs["injectionPoint"]) if fm is not None else None
        if ip is not None and ip.cls in ("Integration", "Module", "Channel", "ExternalParty") and fm.attrs["faultKind"] != "processCrash":
            st = HTTP_RE.search(" ".join(str(self.pkg.get(b).props.get("trigger", "")) for b in cond.prov.based_on
                                         if self.pkg.get(b) is not None))
            out.append((fm.attrs["injectionPoint"], stub_behavior(fm.attrs["faultKind"], st.group(0) if st else ""),
                        int(st.group(1)) if st else None))
        n = self.pkg.get(subj)
        mods = [subj] if n is not None and n.cls == "Module" else []
        for m in mods:
            for d in self.pkg.targets(m, "dependsOn", "Module"):
                if all(d.code != x[0] for x in out):
                    out.append((d.code, "success", None))
            for i in self.pkg.targets(m, "integratesVia", "Integration"):
                if str(i.props.get("direction", "outbound")) != "inbound" and all(i.code != x[0] for x in out):
                    out.append((i.code, "success", None))
        return out


C_CLASSES = ("TestCase", "TestStep", "Assertion", "TestData", "DataField", "Fixture", "StubSpec", "FaultInjection",
             "TestEnvironment", "TestSet")


def specify(g, pkg, run_id=""):
    return Specifier(g, pkg, run_id).run()
