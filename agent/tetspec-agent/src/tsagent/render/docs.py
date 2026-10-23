"""The three ISO/IEC/IEEE 29119-3 md documents built from the test KG (T6.4). Stable order, no timestamps (MECH-07).

All headings, labels and texts here are document content and stay Vietnamese."""
import collections
import json

from . import tcir

LEVEL_NAME = {"L1": "Nghiệp vụ đầu-cuối", "L2": "Nghiệp vụ", "L3": "Cô lập service"}
KIND_TITLE = {"TestRequirements": "Test Requirements", "TestDesign": "Test Design", "TestCases": "Test Cases"}


def cell(v):
    if v is None or v == "" or v == []:
        return ""
    if isinstance(v, (dict, list)):
        v = json.dumps(v, ensure_ascii=False, sort_keys=True)
    return str(v).replace("\\", "\\\\").replace("|", "\\|").replace("\r", "").replace("\n", "<br>")


def table(headers, rows):
    out = ["| " + " | ".join(headers) + " |", "| " + " | ".join("---" for _ in headers) + " |"]
    out += ["| " + " | ".join(cell(c) for c in r) + " |" for r in rows]
    return out


def codes_cell(codes):
    return ", ".join(f"`{c}`" for c in codes)


class View:
    """Read index over a TsoGraph for rendering."""

    def __init__(self, g, pkg, meta):
        self.g, self.pkg, self.meta = g, pkg, meta
        self.kids = collections.defaultdict(list)
        for e in g.entities.values():
            for rel, ts in e.rels.items():
                for t in ts:
                    self.kids[(t, rel)].append(e)
        for k in self.kids:
            self.kids[k].sort(key=lambda e: (e.attrs.get("ordinal", 0), e.code))

    def of(self, cls):
        return list(self.g.of(cls))

    def get(self, code):
        return self.g.get(code)

    def one(self, e, rel):
        return self.g.get(e.rel(rel)[0]) if e is not None and e.rel(rel) else None

    def back(self, code, rel, cls=None):
        return [e for e in self.kids.get((code, rel), []) if cls is None or e.cls == cls]

    def tdd(self, basis_code):
        b = self.g.get(basis_code)
        return b.attrs["tddCode"] if b else basis_code

    def review(self, code):
        lc = self.meta.get("lifecycle", {}).get(code) or {}
        return lc.get("review_status") or "proposed"

    def deriv(self, e):
        return e.prov.derivation + ("" if e.prov.derivation in ("stated", "rule") else f"/{self.review(e.code)}")


def info(v, kind):
    m = v.meta
    rows = [("id", f"TSD-{v.g.feature}-{kind}_v{m.get('version')}"), ("Feature", v.g.feature), ("Phiên bản", m.get("version")),
            ("Trạng thái", m.get("status")), ("Baseline TDD", f"{m.get('primaryDoc') or ''} ({m.get('baseline')})"),
            ("CR của phiên bản này", ", ".join(m.get("changedBy") or []) or "—"),
            ("Ontology", "tso v" + str(m.get("ontologyVersion", "")))]
    return ["## 0. Thông tin tài liệu", ""] + table(["Trường", "Giá trị"], rows) + [""]


def history_section(v, n):
    out = [f"## {n}. Lịch sử phiên bản và phạm vi hồi quy", ""]
    out += table(["Phiên bản", "Trạng thái", "CR", "Thêm", "Sửa", "Bỏ", "Số nút"],
                 [(h["version"], h["status"], ", ".join(h["changedBy"]) or "—", h["added"], h["modified"], h["retired"], h["nodes"])
                  for h in v.meta.get("history", [])])
    reg = v.g.regression
    out += ["", f"Hồi quy ({len(reg)} test case đã baseline, thuộc phần bị ảnh hưởng nhưng không đổi): " +
            (codes_cell(reg) if reg else "không có"), ""]
    return out


# ------------------------------------------------------------------------------------------- 01
def requirements(v):
    g = v.g
    out = ["# Test Requirements: " + g.feature, ""] + info(v, "TestRequirements")
    levels = sorted({tr.attrs["level"] for tr in v.of("TestRequirement")})
    out += ["## 1. Phạm vi và mục tiêu", "",
            f"- Phạm vi: feature {g.feature}; mức test: " + "; ".join(f"{lv} {LEVEL_NAME[lv]}" for lv in levels),
            "- Mục tiêu: mọi yêu cầu trong phạm vi có obligation; mọi điều kiện có oracle, coverage item và test case truy vết được",
            ""]
    out += ["## 2. Test basis", ""]
    docs = sorted(v.pkg.of("Document"), key=lambda n: n.code)
    out += table(["Tài liệu", "Loại", "Phiên bản", "Trạng thái"],
                 [(d.code, d.props.get("docType", ""), d.base.get("version", ""), d.base.get("status", "")) for d in docs])
    bases = v.of("TestBasisItem")
    by_cls = collections.Counter(b.attrs["tddClass"] for b in bases if b.attrs.get("inScope"))
    out += ["", "Nút TDD trong phạm vi được dùng làm căn cứ: " + ", ".join(f"{k} {n}" for k, n in sorted(by_cls.items()))
            + f"; nút ngữ cảnh: {sum(1 for b in bases if not b.attrs.get('inScope'))}.", ""]
    out += ["## 3. Phân tích (tầng A)", ""]
    for ob in v.of("TestObligation"):
        subj = [v.tdd(s) for s in ob.rel("subject")]
        conds = v.back(ob.code, "obligation", "TestCondition")
        out += [f"### {ob.code}", "", f"- Chủ thể: {codes_cell(subj)}; nguồn gốc `{ob.attrs['origin']}`; ưu tiên {ob.attrs['priority']}"
                + (f"; rủi ro {codes_cell(ob.rel('ratedBy'))}" if ob.rel("ratedBy") else ""),
                f"- Câu hỏi: {ob.attrs['question']}", ""]
        rows = []
        for c in conds:
            o = v.one(c, "oracle")
            rows.append((f"`{c.code}`", c.attrs["conditionKind"], c.attrs["statement"], c.attrs["expectedEffect"],
                         f"{o.attrs['oracleKind']}: {o.attrs['expected']}" + (f" (đo: {o.attrs['measurement']})" if o.attrs.get("measurement") else "")
                         if o else "", codes_cell(c.prov.based_on), v.deriv(c)))
        if rows:
            out += table(["Điều kiện", "Loại", "Nội dung", "Kết quả mong đợi", "Oracle", "Căn cứ", "Nguồn gốc"], rows) + [""]
    out += ["## 4. Test Requirements theo mức", ""]
    for i, lv in enumerate(levels, 1):
        out += [f"### 4.{i} {lv}: {LEVEL_NAME[lv]}", ""]
        for tr in (t for t in v.of("TestRequirement") if t.attrs["level"] == lv):
            out += [f"#### {tr.code}", "", tr.attrs["statement"], ""]
            out += table(["Chủ thể", "Loại test", "Ưu tiên", "Phương pháp", "Nguồn gốc"],
                         [(v.tdd(tr.rel("subject")[0]), tr.attrs["testType"], tr.attrs["priority"], tr.attrs["verificationMethod"],
                           v.deriv(tr))])
            out += ["", f"Phạm vi: {tr.attrs['scopeNote']}", "", "Tiêu chí đạt:", ""]
            for pc in v.back(tr.code, "requirement", "PassCriterion"):
                out.append(f"{pc.attrs['ordinal']}. {cell(pc.attrs['text'])} (`{pc.rel('condition')[0]}`)")
            out.append("")
    out += ["## 5. Truy vết", ""]
    rows = []
    for c in v.of("TestCondition"):
        trs = [t.code for t in v.back(c.code, "groups", "TestRequirement")]
        covs = v.back(c.code, "realizes", "CoverageItem")
        tcs = sorted({t.code for cv in covs for t in v.back(cv.code, "covers", "TestCase")})
        rows.append((codes_cell(c.prov.based_on), f"`{c.rel('obligation')[0]}`", f"`{c.code}`", codes_cell(trs),
                     len(covs), codes_cell(tcs)))
    out += table(["Nguồn", "Obligation", "Điều kiện", "TR", "Coverage item", "Test case"], rows) + [""]
    out += ["## 6. Khoảng trống mở (TestGap)", ""]
    gaps = v.of("TestGap")
    out += (table(["Mã", "Thiếu", "Câu hỏi", "Owner", "Hạn", "Chặn"],
                  [(f"`{x.code}`", x.attrs["missing"], x.attrs["askedQuestion"], x.attrs["owner"], x.attrs.get("dueDate"),
                    codes_cell(x.rel("blocks") + x.rel("blocksObligation"))) for x in gaps]) if gaps else ["Không có."]) + [""]
    rep = v.meta.get("report")
    out += ["## 7. Báo cáo kiểm (gate)", ""]
    if rep:
        out += [f"Gate {rep['gate']}: {'đạt' if rep['passed'] else 'không đạt'}" +
                (f" (chặn: {', '.join(rep['blocking'])})" if rep["blocking"] else ""), ""]
        out += table(["Luật", "Mức", "Nội dung", "Đã kiểm", "Vi phạm"],
                     [(r["rule"], r["level"], r["title"], r["checked"], r["violationCount"]) for r in rep["rules"]])
        out.append("")
    out += history_section(v, 8)
    return "\n".join(out)


# ------------------------------------------------------------------------------------------- 02
def design(v):
    g = v.g
    out = ["# Test Design: " + g.feature, ""] + info(v, "TestDesign")
    out += ["## 1. Cách tiếp cận theo mức", ""]
    envs = {e.code: e for e in v.of("TestEnvironment")}
    out += table(["Mức", "Ý nghĩa", "Môi trường", "Thành phần"],
                 [(e.code.rsplit("-", 1)[-1], LEVEL_NAME.get(e.code.rsplit("-", 1)[-1], ""), e.attrs["name"],
                   ", ".join(e.attrs.get("services") or [])) for e in envs.values()]) + [""]
    out += ["## 2. Thiết kế test", ""]
    for td in v.of("TestDesign"):
        tr = v.one(td, "addresses")
        out += [f"### {td.code}", "", f"- TR: `{tr.code}` ({tr.attrs['level']}, {tr.attrs['testType']})",
                f"- Kỹ thuật: {td.attrs['technique']}; tiêu chí phủ: {td.attrs['coverageCriterion']}; mục tiêu {td.attrs['coverageTarget']} %",
                f"- Lý do: {td.attrs['rationale']}", ""]
        rows = []
        for cv in v.back(td.code, "design", "CoverageItem"):
            val = cv.attrs.get("value")
            rows.append((f"`{cv.code}`", f"`{cv.rel('realizes')[0]}`", cv.attrs["coverageKind"], cv.attrs["description"],
                         val.get("value") if isinstance(val, dict) else val, cv.attrs["expected"], f"`{cv.rel('expectedFrom')[0]}`"))
        out += table(["Coverage item", "Điều kiện", "Loại", "Mô tả", "Giá trị", "Kết quả mong đợi", "Oracle"], rows) + [""]
    out += ["## 3. Oracle", ""]
    out += table(["Mã", "Loại", "Kết quả mong đợi", "Cách đo", "Dung sai", "Nguồn"],
                 [(f"`{o.code}`", o.attrs["oracleKind"], o.attrs["expected"], o.attrs.get("measurement"), o.attrs.get("tolerance"),
                   codes_cell([v.tdd(x) for x in o.rel("source")])) for o in v.of("TestOracle")]) + [""]
    out += ["## 4. Lớp tương đương và biên", ""]
    out += table(["Mã", "Tham số", "Loại", "Giá trị"],
                 [(f"`{p.code}`", p.attrs["parameter"], p.attrs["classKind"],
                   "; ".join(f"{x['value']} ({'hợp lệ' if x.get('valid') else 'không hợp lệ'})" if isinstance(x, dict) else str(x)
                             for x in p.attrs.get("values") or [])) for p in v.of("Partition")]) + [""]
    out += ["## 5. Mô hình lỗi và tiêm lỗi", ""]
    rows = []
    for fm in v.of("FaultModel"):
        fis = v.back(fm.code, "realizes", "FaultInjection")
        rows.append((f"`{fm.code}`", fm.attrs["faultKind"], fm.attrs["injectionPoint"], fm.attrs["timing"], fm.attrs["expectedHandling"],
                     "; ".join(f"{fi.attrs['mechanism']} — {fi.attrs['trigger']}" for fi in fis)))
    out += (table(["Mã", "Loại lỗi", "Điểm tiêm", "Thời điểm", "Xử lý mong đợi", "Cách tiêm"], rows) if rows else ["Không có."]) + [""]
    out += ["## 6. Stub", ""]
    stubs = v.of("StubSpec")
    out += (table(["Mã", "Đích", "Hành vi", "HTTP", "Trễ (ms)"],
                  [(f"`{s.code}`", s.attrs["targetCode"], s.attrs["behavior"], s.attrs.get("statusCode"), s.attrs.get("delayMs"))
                   for s in stubs]) if stubs else ["Không có."]) + [""]
    out += ["## 7. Đường đi kịch bản", ""]
    paths = v.of("ScenarioPath")
    out += (table(["Mã", "Loại", "Các bước"], [(f"`{p.code}`", p.attrs["pathKind"], " → ".join(map(str, p.attrs["steps"])))
                                              for p in paths]) if paths else ["Không có."]) + [""]
    out += ["## 8. Tập test", ""]
    out += table(["Thứ tự", "Mã", "Mục đích", "Môi trường", "Điều kiện vào", "Điều kiện ra", "Số TC"],
                 [(s.attrs["ordinal"], f"`{s.code}`", s.attrs["purpose"], codes_cell(s.rel("runsIn")), "; ".join(s.attrs["entryCriteria"]),
                   "; ".join(s.attrs["exitCriteria"]), len(v.back(s.code, "inSet", "TestCase")))
                  for s in sorted(v.of("TestSet"), key=lambda s: s.attrs["ordinal"])]) + [""]
    out += history_section(v, 9)
    return "\n".join(out)


# ------------------------------------------------------------------------------------------- 03
def cases(v):
    g = v.g
    out = ["# Test Cases: " + g.feature, ""] + info(v, "TestCases")
    out += ["## 1. Quy ước", "", "- Bước: given (chuẩn bị), when (hành động), then (quan sát), cleanup (dọn).",
            "- Kết quả mong đợi của assertion lấy từ oracle; dữ liệu nhạy cảm chỉ ghi `secretRef:<tên>`.",
            f"- Mỗi test case kèm một khối TCIR (`{tcir.SCHEMA_VERSION}`) cho agent sinh test script: mã kết quả mong đợi `<TC>.E<n>`, "
            "mã bước `<TC>.S<n>`; dữ liệu chỉ ghi lớp giá trị, test script tự sinh dữ liệu khi chạy.", ""]
    tcs = v.of("TestCase")
    ix = tcir.Index(g)
    out += ["## 2. Test Cases theo mức", ""]
    for i, lv in enumerate(sorted({t.attrs["level"] for t in tcs}), 1):
        out += [f"### 2.{i} {lv}: {LEVEL_NAME[lv]}", ""]
        for tc in (t for t in tcs if t.attrs["level"] == lv):
            out += [f"#### {tc.code}", "", tc.attrs["objective"], ""]
            out += table(["Ưu tiên", "Tự động", "Phủ", "Tập", "Nguồn gốc"],
                         [(tc.attrs["priority"], "có" if tc.attrs.get("automatable") else "không", codes_cell(tc.rel("covers")),
                           codes_cell(tc.rel("inSet")), v.deriv(tc))]) + [""]
            if tc.attrs.get("preconditions"):
                out += ["Tiền điều kiện:", ""] + [f"- {cell(p)}" for p in tc.attrs["preconditions"]] + [""]
            for rel, title in (("needsFixture", "Fixture"), ("needsStub", "Stub"), ("injects", "Tiêm lỗi")):
                items = [v.get(x) for x in tc.rel(rel)]
                if items:
                    desc = {"needsFixture": lambda e: f"{e.attrs['setup']} / dọn: {e.attrs['teardown']}",
                            "needsStub": lambda e: f"{e.attrs['targetCode']} → {e.attrs['behavior']}" + (f" ({e.attrs['statusCode']})" if e.attrs.get("statusCode") else ""),
                            "injects": lambda e: f"{e.attrs['mechanism']} — {e.attrs['trigger']}"}[rel]
                    out += [f"{title}: " + "; ".join(f"`{e.code}` {cell(desc(e))}" for e in items), ""]
            for d in [v.get(x) for x in tc.rel("needsData")]:
                fields = v.back(d.code, "data", "DataField")
                out += [f"Dữ liệu `{d.code}` ({d.attrs['entity']}, {d.attrs['sensitivity']}): " +
                        "; ".join(f"{f.attrs['name']} = {cell(f.attrs['example'])} ({f.attrs['valueClass']})" for f in fields), ""]
            steps = v.back(tc.code, "testCase", "TestStep")
            out += table(["#", "Loại", "Hành động", "Đầu vào", "Đích"],
                         [(s.attrs["ordinal"], s.attrs["stepKind"], s.attrs["action"], s.attrs.get("input"), s.attrs.get("targetCode"))
                          for s in steps]) + [""]
            asserts = v.back(tc.code, "testCase", "Assertion")
            out += table(["#", "Loại", "Toán tử", "Mong đợi", "Đơn vị", "Oracle"],
                         [(a.attrs["ordinal"], a.attrs["assertionKind"], a.attrs["operator"], a.attrs["expected"], a.attrs.get("unit"),
                           codes_cell(a.rel("oracle"))) for a in asserts]) + [""]
            out += ["TCIR:", "", "```json", json.dumps(tcir.test_case(g, tc, ix, v.pkg), ensure_ascii=False, indent=1), "```", ""]
    out += history_section(v, 3)
    return "\n".join(out)


BUILDERS = {"TestRequirements": ("01-test-requirements.md", requirements), "TestDesign": ("02-test-design.md", design),
            "TestCases": ("03-test-cases.md", cases)}
