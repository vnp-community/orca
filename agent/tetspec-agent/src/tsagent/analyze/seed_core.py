"""Core profile seed rules (ONTOLOGY-TEST-TSO section 7.1, MECH-10): build the mandatory layer A nodes from the TDD KG, without an LLM.

The content is a rule template taken from the source text (mode `none` uses it as is). A condition lacking the information to write its
expected result is blocked by a "content" TestGap; when the LLM fill step provides grounded content that gap is dropped (`content_gaps`).

Subjects are in-scope nodes (`testScope` other than `context`). Additions to table 7.1 (DEVIATIONS D12): Mechanism by
`mechanismKind`, an access check for each Endpoint with authentication info, UserRequirement traced to FRs.

Vietnamese strings in this module are deliberate: rule templates become the content of the (Vietnamese) test-spec documents,
and the keyword patterns match Vietnamese source documents.
"""
import re

from ..model import Evidence, Provenance, TsoEntity, TsoGraph
from . import codes

PRIORITY = {"must": "P1", "should": "P2", "could": "P3", "won't": "P3", "wont": "P3"}
BUMP = {"P3": "P2", "P2": "P1", "P1": "P0", "P0": "P0"}
LEVELS = {"Low": 0, "Medium": 1, "High": 2}
NFR_CATEGORY = {"performance_efficiency": "performance", "performance": "performance", "reliability": "resilience",
                "security": "access", "maintainability": "observability", "functional_suitability": "functional",
                "compatibility": "compatibility", "portability": "compatibility", "usability": "functional"}
NFR_KEYWORDS = (
    ("performance", (r"\bp9\d\b", r"\brps\b", r"\btps\b", r"thông lượng", r"throughput")),
    ("concurrency", (r"đồng thời", r"cùng quét", r"concurren", r"hai scheduler", r"hai instance")),
    ("idempotency", (r"trùng", r"duplicate", r"idempot")),
    ("resilience", (r"phục hồi", r"khởi động lại", r"recover", r"restart", r"chạy bù", r"\brto\b", r"\brpo\b")),
    ("observability", (r"truy vết", r"correlation", r"\btrace", r"giám sát", r"cảnh báo")),
    ("timing", (r"đúng giờ", r"trong vòng", r"kể từ", r"trễ")),
    ("access", (r"phân quyền", r"xác thực", r"authori", r"authenti")),
)
MEASURE_RE = re.compile(r"\d+\s*%|\bp\d{2}\b|trong \d+|mỗi |per |/s\b|cửa sổ|window|lần chạy|cùng lúc", re.I)
MECH_KIND = {"concurrency": "concurrency", "rate-limit": "performance", "ratelimit": "performance", "throttle": "performance",
             "resilience": "resilience", "availability": "resilience", "idempotency": "idempotency",
             "observability": "observability", "tracing": "observability", "security": "access"}
ACCESS_RULE_RE = re.compile(r"phân quyền|quyền (xem|truy cập|hạn)|thành viên|vai trò|\bscope\b|api token|authori[sz]|permission|\brbac\b|\brole\b", re.I)
AUTH_RE = re.compile(r"authori|api[ -]?key|x-api-key|bearer|token", re.I)
CONTEXT_OWNER = "QA Lead"
SAFE_METHODS = ("GET", "HEAD", "OPTIONS")


def label(n):
    if n.cls == "Endpoint" and n.props.get("method") and n.props.get("path") and not n.base.get("label"):
        return f"{n.props['method']} {n.props['path']}"
    return str(n.base.get("label") or n.props.get("name") or n.props.get("text") or n.props.get("statement") or n.code)


def text(n):
    return str(n.props.get("statement") or n.props.get("text") or n.base.get("description") or label(n))


def prio(n):
    return PRIORITY.get(str(n.props.get("priority", "")).strip().lower(), "P2")


def unconfirmed(n):
    st = str(n.props.get("confirmStatus") or n.base.get("status") or "").lower()
    return st not in ("confirmed", "closed", "resolved", "answered", "baselined")


class Seeder:
    def __init__(self, pkg, baseline_id, run_id=""):
        self.pkg, self.f, self.bid, self.run_id = pkg, pkg.feature, baseline_id, run_id
        self.g = TsoGraph(pkg.feature)
        self.content_gaps = {}  # condition code -> TestGap code for missing content
        self.by_subject = {}    # subject TDD node code -> [condition codes]
        self.obligations = {}   # subject TDD node code -> [obligation codes]

    # ------------------------------------------------------------------ node builders
    def _ent(self, cls, code, attrs, based_on, rels=None, evidence=(), derivation="rule", confidence=0.7):
        attrs = {k: v for k, v in attrs.items() if v is not None}
        based = sorted({b for b in based_on if b in self.pkg.nodes})
        ev = [Evidence(c, q[:600]) for c, q in evidence if c in self.pkg.nodes]
        e = TsoEntity(cls, code, attrs, {}, Provenance(derivation, based, ev, confidence=confidence, run_id=self.run_id))
        for k, v in (rels or {}).items():
            e.link(k, *(v if isinstance(v, (list, tuple)) else [v]))
        if code in self.g.entities:
            return self.g.entities[code]
        return self.g.add(e)

    def basis(self, n):
        n = self.pkg.get(n) if isinstance(n, str) else n
        code = codes.basis(self.f, n.code)
        return self._ent("TestBasisItem", code, {"tddCode": n.code, "tddClass": n.cls, "baselineId": self.bid,
                                                 "inScope": n.in_scope}, [n.code], derivation="stated", confidence=1.0).code

    def obligation(self, subject, group, question, priority, extra_subjects=(), origin="seedRule"):
        code = codes.obligation(self.f, subject.code, group)
        subs = [self.basis(subject)] + [self.basis(x) for x in extra_subjects]
        self._ent("TestObligation", code, {"question": question, "origin": origin, "applicability": "applicable",
                                           "priority": priority}, [subject.code, *[x.code for x in extra_subjects]],
                  {"subject": subs}, evidence=[(subject.code, text(subject))])
        self.obligations.setdefault(subject.code, [])
        if code not in self.obligations[subject.code]:
            self.obligations[subject.code].append(code)
        return code

    def oracle(self, source, kind, expected, key=None, measurement=None, tolerance=None, based=()):
        code = codes.oracle(self.f, source.code, key)
        return self._ent("TestOracle", code, {"oracleKind": kind, "expected": expected, "measurement": measurement,
                                              "tolerance": tolerance}, [source.code, *based], {"source": self.basis(source)},
                         evidence=[(source.code, text(source))]).code

    def fault(self, subject, key, kind, point, timing, handling, based=()):
        code = codes.fault_model(self.f, subject.code, key)
        return self._ent("FaultModel", code, {"faultKind": kind, "injectionPoint": point, "timing": timing,
                                              "expectedHandling": handling}, [subject.code, *based]).code

    def partition(self, subject, key, parameter, kind, values, valid):
        """`values`: [{"value", "valid"}]; `valid` of a Partition is the target class (invalid class for input-check conditions)."""
        code = codes.partition(self.f, subject.code, key)
        return self._ent("Partition", code, {"parameter": parameter, "classKind": kind, "values": values, "valid": valid},
                         [subject.code]).code

    def path(self, subject, key, kind, steps):
        code = codes.path(self.f, subject.code, key)
        return self._ent("ScenarioPath", code, {"pathKind": kind, "steps": steps}, [subject.code],
                         {"scenario": self.basis(subject)}).code

    def condition(self, ob, subject, kind, key, statement, effect, oracle, based=(), pre=None, trigger=None, fault=None,
                  part=None, path=None, evidence=None, derivation="rule", missing=None, owner=None):
        key = None if key == subject.code else key  # the subject's only condition: do not repeat the code in the key
        code = codes.condition(self.f, subject.code, kind, key)
        ev = evidence if evidence is not None else [(subject.code, text(subject))]
        self._ent("TestCondition", code, {"conditionKind": kind, "statement": statement, "expectedEffect": effect,
                                          "preState": pre, "trigger": trigger, "key": str(key or kind)},
                  [subject.code, *based], {"obligation": ob, "subject": self.basis(subject), "oracle": oracle,
                                           "faultModel": fault, "partition": part, "path": path},
                  evidence=ev, derivation=derivation, confidence=0.9 if derivation == "stated" else 0.6)
        lst = self.by_subject.setdefault(subject.code, [])
        if code not in lst:
            lst.append(code)
        if missing:
            g = self.gap(subject, "-".join(["content", kind] + ([str(key)] if key else [])), missing, f"Bổ sung vào TDD: {missing}?",
                         owner or subject.base.get("owner") or CONTEXT_OWNER, blocks=[code])
            self.content_gaps[code] = g
        return code

    def gap(self, subject, key, missing, question, owner, blocks=(), blocks_ob=None, due=None, based=()):
        code = codes.gap(self.f, subject.code, key)
        e = self._ent("TestGap", code, {"missing": missing, "owner": owner, "askedQuestion": question, "dueDate": due},
                      [subject.code, *based], {"blocks": list(blocks), "blocksObligation": blocks_ob})
        e.link("blocks", *blocks)
        return code

    # ------------------------------------------------------------------ rules
    def run(self):
        P = self.pkg
        for n in P.of("FunctionalRequirement"):
            if n.in_scope:
                self.rule_fr(n)
        for n in P.of("UserRequirement"):
            if n.in_scope:
                self.rule_ur(n)
        for n in P.of("NonFunctionalRequirement"):
            if n.in_scope:
                self.rule_nfr(n)
        for n in P.of("Endpoint"):
            if n.in_scope:
                self.rule_endpoint(n)
        for n in P.of("StateMachine"):
            if n.in_scope:
                self.rule_sm(n)
        for n in P.of("Module"):
            if n.in_scope:
                self.rule_module(n)
        for n in P.of("Integration"):
            if n.in_scope:
                self.rule_integration(n)
        for n in P.of("Scenario"):
            if n.in_scope:
                self.rule_scenario(n)
        for n in P.of("Mechanism"):
            if n.in_scope:
                self.rule_mechanism(n)
        from .profiles import profile_of
        if profile_of(P) == "extended":
            from . import seed_ext
            seed_ext.run(self)
        self.rule_risk()
        self.rule_assumptions()
        return self

    def rule_fr(self, fr):
        ob = self.obligation(fr, "func", f"{fr.code} được đáp ứng: {text(fr)}", prio(fr))
        acs = self.pkg.targets(fr.code, "hasAcceptance", "AcceptanceCriterion")
        if not acs:
            self.gap(fr, "no-ac", f"{fr.code} chưa có tiêu chí chấp nhận", f"Tiêu chí chấp nhận nào chứng minh {fr.code} đạt?",
                     fr.base.get("owner") or "Business Analyst", blocks_ob=ob)
            if ACCESS_RULE_RE.search(text(fr)):
                self.rule_fr_access(fr, ob)
            return
        for ac in acs:
            st = text(ac)
            orc = self.oracle(ac, "acceptanceCriterion", st)
            self.condition(ob, ac, "functional", ac.code, st, st, orc, based=[fr.code], evidence=[(ac.code, st)],
                           derivation="stated")

    def rule_fr_access(self, fr, ob):
        """FR without acceptance criteria that states an access rule: seed the allow and deny conditions (TSO-017) so the
        pair does not depend on how the LLM names the conditions it proposes."""
        orc = self.oracle(fr, "reference", text(fr), key="access")
        self.condition(ob, fr, "access", "allow", f"Chủ thể thỏa quy tắc {fr.code} được phục vụ: {text(fr)}", "Được phục vụ", orc)
        self.condition(ob, fr, "access", "deny", f"Chủ thể không thỏa quy tắc {fr.code} bị từ chối: {text(fr)}",
                       "Bị từ chối", orc, missing=f"hành vi từ chối của {fr.code}")

    def rule_ur(self, ur):
        ob = self.obligation(ur, "trace", f"{ur.code} được đáp ứng qua các FR truy vết: {text(ur)}", prio(ur))
        frs = self.pkg.targets(ur.code, "tracesTo", "FunctionalRequirement")
        if not frs:
            self.gap(ur, "no-fr", f"{ur.code} chưa truy vết tới FR nào", f"FR nào hiện thực {ur.code}?",
                     ur.base.get("owner") or "Business Analyst", blocks_ob=ob)
            return
        exp = "Các FR truy vết đều đạt: " + ", ".join(x.code for x in frs)
        orc = self.oracle(ur, "reference", exp, key="trace", based=[x.code for x in frs])
        self.condition(ob, ur, "functional", "trace", f"{text(ur)} (qua {', '.join(x.code for x in frs)})", exp, orc,
                       based=[x.code for x in frs])

    @staticmethod
    def nfr_kind(n):
        cat = str(n.props.get("category", "")).lower()
        if cat in NFR_CATEGORY:
            return NFR_CATEGORY[cat]
        t = text(n).lower()
        for kind, pats in NFR_KEYWORDS:
            if any(re.search(p, t) for p in pats):
                return kind
        return "performance"

    def rule_nfr(self, nfr):
        kind = self.nfr_kind(nfr)
        ob = self.obligation(nfr, "nfr", f"{nfr.code} đạt chỉ tiêu: {text(nfr)}", prio(nfr))
        tv, tu = nfr.props.get("targetValue"), nfr.props.get("targetUnit")
        expected = f"{text(nfr)} (chỉ tiêu {tv} {tu or ''})".strip() if tv is not None else text(nfr)
        measurement = nfr.props.get("condition") or nfr.props.get("measurement") or (text(nfr) if MEASURE_RE.search(text(nfr)) else None)
        orc = self.oracle(nfr, "threshold", expected, measurement=measurement)
        miss = None if measurement else f"cách đo chỉ tiêu {nfr.code} (phân vị, cửa sổ đo, tải)"
        self.condition(ob, nfr, kind, "target", text(nfr), expected, orc, missing=miss)
        for ac in self.pkg.targets(nfr.code, "hasAcceptance", "AcceptanceCriterion"):
            st = text(ac)
            m2 = (st if MEASURE_RE.search(st) else measurement) if kind == "performance" else None
            o2 = self.oracle(ac, "acceptanceCriterion", st, measurement=m2)
            self.condition(ob, ac, kind, ac.code, st, st, o2, based=[nfr.code], evidence=[(ac.code, st)], derivation="stated",
                           missing=f"tải và cách đo cho {ac.code}" if kind == "performance" and not m2 else None)

    def rule_endpoint(self, ep):
        P = self.pkg
        fields = P.targets(ep.code, "hasField", "Field")
        responses = sorted(P.targets(ep.code, "hasResponse", "Response"), key=lambda r: (int(r.props.get("statusCode", 0) or 0), r.code))
        if not fields and not responses:
            return
        ob = self.obligation(ep, "contract", f"{label(ep)} xử lý đúng đầu vào và trả đúng hợp đồng phản hồi",
                             min((prio(x) for x in P.targets(ep.code, "implements")), default="P2"))
        val = [r for r in responses if int(r.props.get("statusCode", 0) or 0) in (400, 422)]
        for fl in fields:
            self._field_conditions(ob, ep, fl, val)
        statuses = set()
        for r in responses:
            s = int(r.props.get("statusCode", 0) or 0)
            statuses.add(s)
            kind = "functional" if s < 400 else "access" if s in (401, 403) else "errorHandling" if s >= 500 else "inputValidation"
            cond = r.props.get("condition") or label(r)
            exp = f"HTTP {s}: {cond}"
            orc = self.oracle(r, "postcondition" if s < 400 else "errorContract", exp)
            self.condition(ob, r, kind, r.code, f"{label(ep)} khi {cond}", exp, orc, based=[ep.code],
                           evidence=[(r.code, cond)], derivation="stated")
        if ep.props.get("idempotent") is True and str(ep.props.get("method", "")).upper() not in SAFE_METHODS:
            conflict = next((r for r in responses if int(r.props.get("statusCode", 0) or 0) in (409, 422)
                             and "idempot" in " ".join(str(v) for v in [label(r), *r.props.values()]).lower()), None)
            self._idempotency(ob, ep, conflict)
        auth = [fl for fl in fields if AUTH_RE.search(str(fl.props.get("name", "")) + " " + label(fl))] + \
               [h for h in P.targets(ep.code, "requiresHeader") if AUTH_RE.search(label(h) + " " + str(h.props.get("name", "")))]
        if auth and not statuses & {401, 403}:
            orc = self.oracle(ep, "reference", "Thiếu hoặc sai thông tin xác thực bị từ chối; hợp lệ thì được phục vụ", key="access")
            self.condition(ob, ep, "access", "authn", f"Gọi {label(ep)} thiếu hoặc sai {label(auth[0])}",
                           "Bị từ chối, không lộ dữ liệu", orc, based=[a.code for a in auth],
                           missing=f"mã lỗi của {label(ep)} khi thiếu hoặc sai xác thực")

    @staticmethod
    def _validation_response(fl, val, case):
        """Response 400/422 for one invalid case of a field: prefer a Response naming the field and the right kind of violation
        (e.g. `amount > 50.000.000` -> 422 LIMIT_EXCEEDED for the above-upper-bound case); otherwise the first 400 Response."""
        name = str(fl.props.get("name") or label(fl)).split(" (")[0].strip().lower()
        words = {"missing": r"thiếu|bắt buộc|missing|required", "min": r"<|nhỏ|dưới|không nguyên dương|âm|ngoài",
                 "max": r">|vượt|quá|lớn hơn|ngoài", "minLength": r"ngắn|độ dài|ngoài", "maxLength": r"dài|độ dài|ngoài",
                 "enum": r"sai|ngoài|không hợp lệ|enum", "format": r"định dạng|parse|format|sai"}.get(case, "")
        def cond(r):
            return str(r.props.get("condition", "")).lower()
        name_re = re.compile(r"(?<![\w-])" + re.escape(name) + r"(?![\w-])") if name else None

        def clause_hit(r):  # field name and violation kind in the same clause
            return bool(name_re and words) and any(name_re.search(c) and re.search(words, c)
                                                  for c in re.split(r",|;| hoặc ", cond(r)))
        hit = [r for r in val if clause_hit(r)]
        if not hit and case == "missing":
            hit = [r for r in val if words and re.search(words, cond(r))]
        if hit:
            return sorted(hit, key=lambda r: (int(r.props.get("statusCode", 0) or 0), r.code))[0]
        bad = [r for r in val if int(r.props.get("statusCode", 0) or 0) == 400]
        return (bad or val or [None])[0]

    def _field_conditions(self, ob, ep, fl, val):
        p = fl.props
        name = str(p.get("name") or label(fl))
        cases = []
        if p.get("required") is True:
            cases.append(("missing", "missing", [{"value": "<thiếu>", "valid": False}], f"Thiếu trường bắt buộc {name}"))
        step = 1 if isinstance(p.get("min", p.get("max", 0)), int) else 0.01
        if isinstance(p.get("min"), (int, float)):
            cases.append(("min", "boundary", [{"value": p["min"] - step, "valid": False}, {"value": p["min"], "valid": True}], f"{name} dưới cận dưới {p['min']}"))
        if isinstance(p.get("max"), (int, float)):
            cases.append(("max", "boundary", [{"value": p["max"], "valid": True}, {"value": p["max"] + step, "valid": False}], f"{name} vượt cận trên {p['max']}"))
        for k in ("minLength", "maxLength"):
            if isinstance(p.get(k), int):
                v = p[k] - 1 if k == "minLength" else p[k] + 1
                cases.append((k, "boundary", [{"value": {"length": p[k]}, "valid": True}, {"value": {"length": v}, "valid": False}], f"{name} có độ dài {v} (giới hạn {k}={p[k]})"))
        if p.get("enum"):
            vals = p["enum"] if isinstance(p["enum"], list) else [p["enum"]]
            cases.append(("enum", "enum", [{"value": x, "valid": True} for x in vals] + [{"value": "<ngoài tập>", "valid": False}], f"{name} nhận giá trị ngoài {', '.join(map(str, vals))}"))
        if p.get("format") or p.get("pattern"):
            cases.append(("format", "format", [{"value": "<sai định dạng>", "valid": False}], f"{name} sai định dạng {p.get('format') or p.get('pattern')}"))
        for key, kind, values, st in cases:
            r = self._validation_response(fl, val, key)
            if key == "missing" and AUTH_RE.search(name):
                r = next((x for x in self.pkg.targets(ep.code, "hasResponse", "Response")
                          if int(x.props.get("statusCode", 0) or 0) == 401), r)
            if r is not None:
                exp = f"HTTP {r.props.get('statusCode')}: {r.props.get('condition') or label(r)}"
                orc = self.oracle(r, "schema", exp, key="validation", based=[ep.code])
            else:
                exp = "Yêu cầu bị từ chối với lỗi kiểm tra đầu vào"
                orc = self.oracle(ep, "schema", exp, key="validation")
            missing = None if r is not None else f"mã lỗi khi đầu vào của {label(ep)} không hợp lệ"
            part = self.partition(fl, key, name, kind, values, False)
            self.condition(ob, fl, "inputValidation", key, f"{label(ep)}: {st}", exp, orc, based=[ep.code], part=part,
                           evidence=[(fl.code, f"{name}: " + ", ".join(f"{k}={v}" for k, v in sorted(p.items())
                                                                         if k not in ("sourceFile", "sourceLine")))],
                           missing=missing)

    def _idempotency(self, ob, n, conflict=None):
        """The three mandatory idempotency conditions (TSO-014): replay, same key with another payload, concurrent duplicate."""
        orc = self.oracle(n, "invariant", "Cùng khóa idempotency không tạo hiệu ứng lặp", key="idem")
        o2 = self.oracle(conflict, "errorContract", f"HTTP {conflict.props.get('statusCode')}: {conflict.props.get('condition') or label(conflict)}",
                         key="idem-conflict", based=[n.code]) if conflict is not None else orc
        for key, kind, st, exp, oc in (
                ("replay", "idempotency", f"Gửi lại cùng yêu cầu (cùng khóa idempotency) tới {label(n)}",
                 "Trả kết quả lần đầu, không tạo hiệu ứng lặp", orc),
                ("payload-mismatch", "idempotency", f"Cùng khóa idempotency, khác payload tới {label(n)}",
                 self.g.get(o2).attrs["expected"] if conflict is not None else "Bị từ chối theo chính sách xung đột", o2),
                ("concurrent-dup", "concurrency", f"Hai yêu cầu cùng khóa idempotency gửi đồng thời tới {label(n)}",
                 "Chỉ một hiệu ứng; yêu cầu còn lại nhận kết quả của yêu cầu thắng", orc)):
            self.condition(ob, n, kind, key, st, exp, oc,
                           missing="chính sách khi cùng khóa khác payload" if key == "payload-mismatch" and conflict is None else None)

    def rule_sm(self, sm):
        P = self.pkg
        trs = {t.code: t for t in P.targets(sm.code, "hasTransition", "Transition")}
        trs.update({t.code: t for t in P.sources(sm.code, "inMachine", "Transition")})
        if not trs:
            return
        governs = ", ".join(label(x) for x in P.targets(sm.code, "governs"))
        ob = self.obligation(sm, "states", f"Vòng đời {label(sm)}{' của ' + governs if governs else ''} chuyển đúng trạng thái",
                             "P1")
        for code in sorted(trs):
            t = trs[code]
            p = t.props
            fr, to, trig = p.get("from", "?"), p.get("to", "?"), p.get("trigger") or "sự kiện"
            eff = f"Trạng thái {to}" + (f"; {p['effect']}" if p.get("effect") else "")
            orc = self.oracle(t, "stateMachine", f"{fr} → {to}" + (f"; {p['effect']}" if p.get("effect") else ""))
            self.condition(ob, t, "stateTransition", t.code, f"{fr} --{trig}--> {to}", eff, orc, based=[sm.code],
                           pre=f"Trạng thái {fr}" + (f", {p['guard']}" if p.get("guard") else ""), trigger=trig,
                           evidence=[(t.code, f"{fr} -> {to}: {trig}")], derivation="stated")
            if p.get("guard"):
                o2 = self.oracle(t, "stateMachine", f"Không chuyển sang {to}; giữ {fr}", key="guard")
                self.condition(ob, t, "guardViolation", "guard", f"{trig} khi không thỏa điều kiện: {p['guard']}",
                               f"Không chuyển sang {to}; giữ {fr}", o2, based=[sm.code], pre=f"Trạng thái {fr}, không thỏa {p['guard']}",
                               trigger=trig, evidence=[(t.code, f"guard: {p['guard']}")])

    def rule_module(self, m):
        deps = self.pkg.targets(m.code, "dependsOn", "Module")
        if not deps:
            return
        ob = self.obligation(m, "deps", f"{label(m)} xử lý đúng khi phụ thuộc lỗi", "P2")
        for d in deps:
            orc = self.oracle(d, "errorContract", f"{label(m)} xử lý lỗi của {label(d)} theo thiết kế", key=f"dep-{m.code}",
                              based=[m.code])
            fm = self.fault(m, f"dep-{d.code}", "dependencyFailure", d.code, "during", f"{label(m)} xử lý khi {label(d)} lỗi",
                            based=[d.code])
            self.condition(ob, m, "errorHandling", f"dep-{d.code}", f"{label(d)} lỗi hoặc không phản hồi khi {label(m)} gọi",
                           f"{label(m)} xử lý lỗi của {label(d)} theo thiết kế", orc, based=[d.code], fault=fm,
                           missing=f"hành vi của {label(m)} khi {label(d)} lỗi")

    @staticmethod
    def worst_case(P, i):
        tos = P.targets(i.code, "hasTimeoutPolicy", "TimeoutPolicy")
        if not tos:
            return None
        to = tos[0].props
        per = to.get("totalMs") or ((to.get("connectMs") or 0) + (to.get("readMs") or 0)) or None
        if not per:
            return None
        rts = P.targets(i.code, "hasRetryPolicy", "RetryPolicy")
        rt = rts[0].props if rts else {}
        att = int(rt.get("maxAttempts") or 1)
        back = 0
        if rt.get("backoffType") not in (None, "none"):
            base = rt.get("initialBackoffMs") or rt.get("backoffMs") or 0
            mult = 2 if rt.get("backoffType") == "exponential" else 1
            back = sum(base * (mult ** k) for k in range(att - 1))
            back = min(back, rt.get("maxBackoffMs") or back) if rt.get("maxBackoffMs") else back
        return {"per": per, "attempts": att, "backoff": back, "total": per * att + back,
                "based": [tos[0].code] + ([rts[0].code] if rts else [])}

    def _timing_worst(self, ob, subject, i, budget, budget_src):
        wc = self.worst_case(self.pkg, i)
        if not wc:
            return
        st = (f"Trường hợp xấu nhất khi gọi {label(i)}: {wc['attempts']} lần × {wc['per']} ms + backoff {wc['backoff']} ms "
              f"= {wc['total']} ms")
        exp = f"{wc['total']} ms ≤ {budget} ms (ngân sách của {budget_src.code})" if budget else None
        orc = self.oracle(subject, "threshold", exp or f"{wc['total']} ms", key=f"worst-{i.code}",
                          measurement="Cộng thời gian trường hợp xấu nhất (timeout mỗi lần × số lần + backoff), không cộng p95",
                          based=[i.code, *wc["based"]])
        self.condition(ob, subject, "timing", f"worst-{i.code}", st, exp or st, orc, based=[i.code, *wc["based"]],
                       evidence=[(wc["based"][0], st)],
                       missing=None if budget else f"ngân sách thời gian của luồng gọi {label(i)}")

    def rule_integration(self, i):
        P = self.pkg
        p = i.props
        ob = self.obligation(i, "resilience", f"Tích hợp {label(i)} xử lý đúng lỗi, thời gian, giới hạn", "P1")
        for em in P.targets(i.code, "hasErrorMapping", "ErrorMapping"):
            q = em.props
            cat = str(q.get("errorCategory", ""))
            trig = q.get("trigger") or label(em)
            timing = "afterCommit" if cat == "unknownOutcome" else "before"
            act = q.get("action")
            fm = self.fault(i, f"err-{em.code}", cat or str(trig), i.code, timing, act or "chưa mô tả", based=[em.code])
            orc = self.oracle(em, "errorContract", act or f"Xử lý {trig}")
            self.condition(ob, em, "errorHandling", em.code, f"{label(i)} trả {trig}", act or f"Xử lý {trig}", orc,
                           based=[i.code], fault=fm, evidence=[(em.code, f"{trig}: {act or ''}")],
                           missing=None if act else f"hành động khi {label(i)} trả {trig}")
        budgets = [s for s in P.sources(i.code, "invokes", "Scenario") if s.props.get("responseBudgetMs")]
        self._timing_worst(ob, i, i, budgets[0].props["responseBudgetMs"] if budgets else None, budgets[0] if budgets else i)
        for cb in P.targets(i.code, "protectedBy", "CircuitBreaker"):
            q = cb.props
            thr = f"{q.get('failureRatePercent', '?')}% lỗi trong tối thiểu {q.get('minimumCalls', '?')} lượt"
            for key, st, exp in (("open", f"Lỗi đạt {thr}", f"Breaker mở {q.get('openDurationSec', '?')} giây, chặn lời gọi tới {label(i)}"),
                                 ("halfopen", f"Hết {q.get('openDurationSec', '?')} giây mở", "Breaker nửa mở, cho một lượt thử"),
                                 ("close", "Lượt thử khi nửa mở thành công", "Breaker đóng, lời gọi đi bình thường")):
                orc = self.oracle(cb, "stateMachine", exp, key=key)
                self.condition(ob, cb, "stateTransition", key, st, exp, orc, based=[i.code], evidence=[(cb.code, text(cb))])
        if p.get("idempotent") is True:
            self._idempotency(ob, i)
        for lim in P.targets(i.code, "boundedBy", "PartnerLimit"):
            q = lim.props
            for attr, unit, kind in (("requestsPerSecond", "request/giây", "performance"), ("maxPayloadKB", "KB", "inputValidation")):
                if q.get(attr) is None:
                    continue
                orc = self.oracle(lim, "threshold", f"{attr} ≤ {q[attr]} {unit}", key=attr,
                                  measurement=f"Đếm {unit} trong cửa sổ 1 giây" if attr == "requestsPerSecond" else "Kích thước thân yêu cầu")
                self.condition(ob, lim, kind, f"{attr}-at", f"{label(i)} ở đúng giới hạn {attr}={q[attr]} {unit}",
                               "Được phục vụ bình thường", orc, based=[i.code], evidence=[(lim.code, f"{attr}={q[attr]}")])
                self.condition(ob, lim, kind, f"{attr}-over", f"{label(i)} vượt giới hạn {attr}={q[attr]} {unit}",
                               "Bị giới hạn hoặc từ chối theo hợp đồng đối tác", orc, based=[i.code],
                               evidence=[(lim.code, f"{attr}={q[attr]}")], missing=f"hành vi khi vượt {attr} của {label(i)}")
        for ch in P.targets(i.code, "usesChannel", "Channel"):
            q = ch.props
            if str(q.get("deliverySemantics", "")).lower() == "atleastonce":
                orc = self.oracle(ch, "invariant", "Consumer khử trùng, không xử lý một message hai lần", key="dup")
                self.condition(ob, ch, "idempotency", "dup", f"{label(ch)} giao trùng một message",
                               "Không xử lý hai lần", orc, based=[i.code], evidence=[(ch.code, "deliverySemantics=atLeastOnce")])
            if q.get("dlqName"):
                fm = self.fault(ch, "poison", "poisonMessage", ch.code, "during", f"Message lỗi vào {q['dlqName']}")
                orc = self.oracle(ch, "errorContract", f"Message lỗi vào {q['dlqName']}", key="dlq")
                self.condition(ob, ch, "errorHandling", "dlq", f"Consumer của {label(ch)} gặp message hỏng",
                               f"Message vào {q['dlqName']}, không chặn message sau", orc, based=[i.code], fault=fm,
                               evidence=[(ch.code, f"dlqName={q['dlqName']}")])
        if str(p.get("direction", "")).lower() == "inbound":
            for sec in P.targets(i.code, "securedBy", "SecurityScheme"):
                orc = self.oracle(sec, "reference", f"Chỉ yêu cầu xác thực đúng {label(sec)} được phục vụ", key="access")
                self.condition(ob, sec, "access", "allow", f"Gọi {label(i)} với thông tin {label(sec)} hợp lệ", "Được phục vụ",
                               orc, based=[i.code])
                self.condition(ob, sec, "access", "deny", f"Gọi {label(i)} thiếu hoặc sai {label(sec)}",
                               "Bị từ chối, không lộ dữ liệu", orc, based=[i.code], missing=f"mã lỗi khi sai xác thực {label(i)}")

    def rule_scenario(self, sc):
        P = self.pkg
        mods = P.participants(sc.code)
        if len(mods) < 2:
            return
        steps = sc.props.get("steps") or [label(sc)]
        steps = steps if isinstance(steps, list) else [str(steps)]
        ob = self.obligation(sc, "flow", f"Luồng {label(sc)} chạy đúng đầu-cuối qua {', '.join(m.code for m in mods)}", "P1",
                             extra_subjects=mods)
        pm = self.path(sc, "main", "main", steps)
        orc = self.oracle(sc, "postcondition", steps[-1], key="main")
        self.condition(ob, sc, "functional", "main", f"Luồng chính {label(sc)}: " + " → ".join(steps), steps[-1], orc,
                       path=pm, based=[m.code for m in mods], evidence=[(sc.code, " -> ".join(steps))])
        budget = sc.props.get("responseBudgetMs")
        for i in P.targets(sc.code, "invokes", "Integration"):
            fbs = P.targets(i.code, "hasFallback")
            fb = fbs[0] if fbs else None
            pf = self.path(sc, f"fail-{i.code}", "failure", steps[:max(1, len(steps) - 1)] + [f"{label(i)} lỗi"])
            exp = text(fb) if fb else None
            o2 = self.oracle(fb or i, "errorContract", exp or f"Luồng xử lý lỗi của {label(i)}", key=f"fail-{sc.code}")
            self.condition(ob, sc, "errorHandling", f"fail-{i.code}", f"{label(i)} lỗi trong luồng {label(sc)}",
                           exp or f"Luồng xử lý lỗi của {label(i)}", o2, path=pf, based=[i.code] + ([fb.code] if fb else []),
                           missing=None if fb else f"hành vi luồng {label(sc)} khi {label(i)} lỗi")
            self._timing_worst(ob, sc, i, budget, sc)
        if budget:
            o3 = self.oracle(sc, "threshold", f"≤ {budget} ms", key="budget",
                             measurement="Đo từ bước đầu tới bước cuối mỗi lần chạy; gồm trường hợp xấu nhất của lời gọi ngoài")
            self.condition(ob, sc, "timing", "budget", f"Luồng {label(sc)} hoàn tất trong {budget} ms", f"≤ {budget} ms", o3,
                           evidence=[(sc.code, f"responseBudgetMs={budget}")])

    def rule_mechanism(self, m):
        kind = MECH_KIND.get(str(m.props.get("mechanismKind", "")).lower(), "invariant")
        sat = self.pkg.sources(m.code, "satisfiedBy")
        ob = self.obligation(m, "mechanism", f"Cơ chế {label(m)} hoạt động đúng", min((prio(x) for x in sat), default="P2"))
        based = [x.code for x in sat]
        if kind == "concurrency":
            orc = self.oracle(m, "invariant", "Chỉ một bên thành công; không có hiệu ứng trùng", key="parallel")
            self.condition(ob, m, "concurrency", "parallel", f"Hai tiến trình cùng thực hiện: {label(m)}",
                           "Chỉ một bên thành công; không có hiệu ứng trùng", orc, based=based)
            fm = self.fault(m, "crash-mid", "processCrash", m.code, "afterCommit", "Phục hồi không tạo hiệu ứng trùng hoặc mất")
            o2 = self.oracle(m, "invariant", "Sau khởi động lại không trùng, không mất", key="crash")
            self.condition(ob, m, "resilience", "crash-mid", f"Tiến trình chết giữa chừng sau bước của {label(m)}",
                           "Sau khởi động lại không trùng, không mất", o2, fault=fm, based=based,
                           missing=f"cách phục hồi khi tiến trình chết giữa chừng ({label(m)})")
        elif kind == "performance":
            orc = self.oracle(m, "threshold", label(m), key="limit", measurement="Đếm theo chu kỳ hoặc theo giây như mô tả cơ chế")
            self.condition(ob, m, "performance", "at-limit", f"Tải đúng giới hạn: {label(m)}", "Xử lý đủ, không vượt giới hạn",
                           orc, based=based)
            self.condition(ob, m, "performance", "over-limit", f"Tải vượt giới hạn: {label(m)}", "Phần vượt bị hoãn hoặc giới hạn",
                           orc, based=based, missing=f"hành vi khi vượt giới hạn ({label(m)})")
        elif kind == "access":
            orc = self.oracle(m, "reference", label(m), key="access")
            self.condition(ob, m, "access", "allow", f"Yêu cầu hợp lệ qua cơ chế: {label(m)}", "Được phục vụ", orc, based=based)
            self.condition(ob, m, "access", "deny", f"Yêu cầu thiếu hoặc sai thông tin qua cơ chế: {label(m)}",
                           "Bị từ chối, không lộ dữ liệu", orc, based=based, missing=f"mã lỗi khi bị từ chối ({label(m)})")
        else:
            orc = self.oracle(m, "postcondition" if kind == "resilience" else "invariant", label(m), key="holds")
            self.condition(ob, m, kind, "holds", label(m), label(m), orc, based=based)

    def rule_risk(self):
        P = self.pkg
        for subj in sorted(self.obligations):
            risks = {r.code: r for r in P.targets(subj, "hasRisk", "Risk")}
            risks.update({r.code: r for r in P.sources(subj, "mitigatedBy", "Risk")})
            for rel in ("allocatedTo", "satisfiedBy"):
                for x in P.targets(subj, rel):
                    risks.update({r.code: r for r in P.targets(x.code, "hasRisk", "Risk")})
            if not risks:
                continue
            rs = [risks[c] for c in sorted(risks)]
            lk = max((r.props.get("likelihood", "Medium") for r in rs), key=lambda v: LEVELS.get(v, 1))
            im = max((r.props.get("impact", "Medium") for r in rs), key=lambda v: LEVELS.get(v, 1))
            base = min(self.g.get(o).attrs["priority"] for o in self.obligations[subj])
            pr = BUMP[base] if im == "High" else base
            rc = codes.risk(self.f, subj)
            self._ent("RiskRating", rc, {"likelihood": lk if lk in LEVELS else "Medium", "impact": im if im in LEVELS else "Medium",
                                         "priority": pr, "rationale": "; ".join(f"{r.code}: {label(r)}" for r in rs)},
                      [subj, *risks])
            for o in self.obligations[subj]:
                ob = self.g.get(o)
                ob.link("ratedBy", rc)
                if im == "High":
                    ob.attrs["priority"] = BUMP[ob.attrs["priority"]] if ob.attrs["priority"] == base else ob.attrs["priority"]

    def rule_assumptions(self):
        """TestGap only for subjects that declare `assumes`/`blockedBy` directly (not spread through shared nodes, G13)."""
        P = self.pkg
        subjects = set(self.by_subject) | set(self.obligations)
        for subj in sorted(subjects):
            for rel, cls in (("assumes", "Assumption"), ("blockedBy", "OpenQuestion")):
                for a in P.targets(subj, rel, cls):
                    if not unconfirmed(a):
                        continue
                    blocks = list(self.by_subject.get(subj, []))
                    obs = self.obligations.get(subj, [])
                    self.gap(P.get(subj), a.code, f"{a.code} chưa xác nhận: {text(a)}", f"Xác nhận {a.code}: {text(a)}?",
                             a.base.get("owner") or CONTEXT_OWNER, blocks=blocks, blocks_ob=obs[0] if obs else None,
                             due=a.props.get("dueDate"), based=[a.code])


def seed(pkg, baseline_id, run_id=""):
    """Return the Seeder after running: `.g` (layer A TsoGraph), `.content_gaps`, `.by_subject`, `.obligations`."""
    return Seeder(pkg, baseline_id, run_id).run()
