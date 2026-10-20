"""Compare v2 layer A (and B, C when present) with the v1 baseline on MiniPay CR-001 (T5.6, TST-21). The report itself is Vietnamese.

python scripts/tsagent/compare_baseline.py --mode none --out COMPARE-A.md
python scripts/tsagent/compare_baseline.py --mode claude-cli --cap 3 --only MECH-RunClaim,NFR-011,SCN-ScheduledRun --out ...
"""
import argparse
import collections
import datetime
import os
import pathlib
import sys
import tempfile

ROOT = pathlib.Path(__file__).resolve().parents[2]  # scripts/tsagent/x.py -> repo root
sys.path.insert(0, str(ROOT / "src"))

from tsagent.analyze import analyze  # noqa: E402
from tsagent.config import Config  # noqa: E402
from tsagent.design import run_design_step  # noqa: E402
from tsagent.ingest import ingest  # noqa: E402
from tsagent.llm import make_client  # noqa: E402
from tsagent.specify import PRE_REVIEW, run_specify_step  # noqa: E402
from tsagent.validate.tso_rules import Ctx, validate  # noqa: E402

FIX = ROOT / "tests" / "testspec_agent" / "fixtures" / "minipay"
DOCS = ("cr001-slice.tdd-kg.json", "solution.md", "supplement.md")


def conds(g, pred):
    return [c.code for c in g.of("TestCondition") if pred(c)]


def fault_timing(g, c):
    return g.get(c.rel("faultModel")[0]).attrs["timing"] if c.rel("faultModel") else None


def oracle(g, c):
    return g.get(c.rel("oracle")[0]).attrs if c.rel("oracle") else {}


def subject(g, c):
    return g.get(c.rel("subject")[0]).attrs["tddCode"]


def gap_rows(g, pkg):
    """(G, v1 gap, v2 layer A result, evidence codes)."""
    a = lambda pred: conds(g, pred)  # noqa: E731
    g1 = a(lambda c: c.attrs["conditionKind"] == "concurrency")
    g2 = a(lambda c: fault_timing(g, c) == "afterCommit" or "khởi động lại" in c.attrs["statement"].lower())
    g3 = a(lambda c: c.attrs["conditionKind"] == "timing" and c.attrs["key"].startswith("worst-"))
    g4 = a(lambda c: subject(g, c) == "MECH-SchedulerThrottle")
    g5 = a(lambda c: subject(g, c) in ("NFR-010", "NFR-013") and c.attrs["key"] == "target"
           and any(x in (oracle(g, c).get("measurement") or "") for x in ("99%", "100 lần")))
    g13 = [x.code for x in g.of("TestGap") if "chưa xác nhận" in x.attrs["missing"]]
    g14 = sorted({subject(g, g.get(c)).split("-")[1] for c in a(lambda c: c.attrs["conditionKind"] == "access")})
    eps = sorted(n.code for n in pkg.of("Endpoint") if n.in_scope)
    later = "tầng B (Giai đoạn 6)"
    return [
        ("G1", "trùng đồng thời", f"có, {len(g1)} điều kiện `concurrency`", g1[:4]),
        ("G2", "chết giữa chừng", f"có, {len(g2)} điều kiện (FaultModel `afterCommit` hoặc khởi động lại)", g2[:4]),
        ("G3", "ngân sách xấu nhất", f"có, {len(g3)} điều kiện `timing` trường hợp xấu nhất", g3[:4]),
        ("G4", "throttle 20 lần/chu kỳ, 10 rps", f"có, {len(g4)} điều kiện trên MECH-SchedulerThrottle", g4[:4]),
        ("G5", "NFR mất 99 %, 100 lần cùng lúc", f"có, measurement giữ nguyên văn ({len(g5)}/2)", g5),
        ("G6", "test NFR vô nghĩa (+1 cho chỉ tiêu 0/100 %)", f"chưa đo được ở tầng A; kiểm bằng TSO-016 ở {later}", []),
        ("G7", "không có L3 cho MOD-Notify, INT-02, INT-03",
         "các nút này là `context` trong lát cắt CR-001 nên không là chủ thể; mức L3 do tầng B quyết định", []),
        ("G8", "chỉ một hành trình L1", f"ScenarioPath main + failure cho mọi Scenario có ≥ 2 Module; gom L1 ở {later}",
         [x.code for x in g.of("ScenarioPath")]),
        ("G9", "không có hồi quy", "chọn hồi quy ở Giai đoạn 7 (`tso_regression`)", []),
        ("G10", "BR-10 (UTC+7) không có", "BR-10 chỉ có trong srs.md, không thuộc đầu vào; KG TDD chưa có lớp BusinessRule "
                                         "(cần profile Extended: DecisionTable/Invariant)", []),
        ("G11", "TR chép tên FR, tiêu chí dồn một ô", f"tầng A tách điều kiện theo AC/Field/Response; tiêu chí đánh số ở {later}", []),
        ("G12", "TR chồng lấn", f"`scopeNote` ở {later}", []),
        ("G13", "pendingConfirmation lan 123 nút", f"{len(g13)} TestGap từ giả định chưa xác nhận (chỉ theo `assumes`/`blockedBy` trực tiếp)",
         g13[:4]),
        ("G14", "phân quyền từng endpoint", f"có điều kiện `access` cho {len(g14)}/{len(eps)} endpoint trong phạm vi", g14),
    ]


def main(argv=None):
    ap = argparse.ArgumentParser()
    ap.add_argument("--mode", default="none", choices=["none", "claude-cli", "anthropic", "openai"])
    ap.add_argument("--cap", type=int, default=200)
    ap.add_argument("--only", default="", help="call the LLM only for these subjects (comma separated)")
    ap.add_argument("--out", required=True)
    ap.add_argument("--layers", default="a", choices=["a", "abc"], help="a: layer A only; abc: add layers B, C (COMPARE-BC)")
    a = ap.parse_args(argv)
    tmp = pathlib.Path(tempfile.mkdtemp(prefix="tsagent-compare-"))
    cfg = Config()
    cfg.db, cfg.cache_dir, cfg.inbox_dir = str(tmp / "db"), os.environ.get("TSAGENT_CACHE_DIR", str(tmp / "cache")), str(tmp / "inbox")
    cfg.source_roots = (str(FIX),)
    cfg.llm_provider = a.mode
    cfg.llm_caps = {"analysis": a.cap, "design": 0, "specification": 0}
    pkg, _ = ingest([str(FIX / d) for d in DOCS], "SCHEDULED-TRANSFER", cfg)
    only = set(a.only.split(",")) if a.only else None
    g, fr, rep, _ = analyze(pkg, "cmp", make_client(cfg), cfg, only=only)
    if a.layers == "abc":
        return write_bc(a, cfg, pkg, g)
    kinds = collections.Counter(c.attrs["conditionKind"] for c in g.of("TestCondition"))
    cls = collections.Counter(e.cls for e in g.entities.values())
    llm = sum(1 for e in g.entities.values() if e.prov.derivation == "llm")
    lines = [f"## Chế độ `{a.mode}`" + (f" (chỉ {a.only}, trần {a.cap})" if a.only else ""), "",
             f"Chạy {datetime.date.today().isoformat()}, đầu vào {', '.join(DOCS)}. Gate GA: {rep.summary()}.", "",
             "| Lớp | Số nút |", "| --- | --- |"] + [f"| {k} | {v} |" for k, v in sorted(cls.items())] + \
            ["", "| conditionKind | Số điều kiện |", "| --- | --- |"] + [f"| {k} | {v} |" for k, v in sorted(kinds.items())] + \
            ["", f"Lượt gọi LLM: {fr.calls}; nút `derivation=llm`: {llm}; điều kiện LLM điền: {len(fr.filled)}; "
                 f"đề xuất thêm: {len(fr.proposed)}; mục bị loại: {sum(len(x['problems']) for x in fr.rejected)}.", "",
             "| # | Khoảng trống v1 | Tầng A v2 | Minh chứng |", "| --- | --- | --- | --- |"]
    for gid, gap, res, ev in gap_rows(g, pkg):
        lines.append(f"| {gid} | {gap} | {res} | {', '.join(f'`{x}`' for x in ev) or '—'} |")
    out = pathlib.Path(a.out)
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text("\n".join(lines) + "\n", encoding="utf-8")
    print(out)



def tcs_for(g, pred):
    """Test cases covering the conditions that satisfy `pred`."""
    out = []
    for tc in g.of("TestCase"):
        for cv in tc.rel("covers"):
            c = g.get(g.get(cv).rel("realizes")[0])
            if pred(c):
                out.append(tc.code)
                break
    return sorted(set(out))


def bc_metrics(g, pkg):
    trs = list(g.of("TestRequirement"))
    tcs = list(g.of("TestCase"))
    pcs = collections.defaultdict(list)
    for pc in g.of("PassCriterion"):
        pcs[pc.rel("requirement")[0]].append(pc)
    numbered = sum(1 for tr in trs if len(pcs[tr.code]) == len(tr.rel("groups")))
    copied = 0
    for tr in trs:
        n = pkg.get(g.get(tr.rel("subject")[0]).attrs["tddCode"])
        if n is not None and tr.attrs["statement"].strip().lower() in (str(n.base.get("label", "")).lower(), str(n.props.get("text", "")).lower()):
            copied += 1
    per_subject = collections.Counter(tr.rel("subject")[0] for tr in trs)
    scoped = sum(1 for tr in trs if per_subject[tr.rel("subject")[0]] < 2 or tr.attrs.get("scopeNote"))
    asserts = list(g.of("Assertion"))
    rep = validate(Ctx(g, pkg, "cmp"), "GB", ["TSO-016"])
    return {"tr": len(trs), "trByLevel": collections.Counter(tr.attrs["level"] for tr in trs), "tc": len(tcs),
            "tcByLevel": collections.Counter(tc.attrs["level"] for tc in tcs), "numbered": numbered, "copied": copied,
            "scoped": scoped, "asserts": len(asserts), "assertsWithOracle": sum(1 for x in asserts if x.rel("oracle")),
            "meaninglessNfr": len(rep.result("TSO-016").violations), "td": len(list(g.of("TestDesign"))),
            "cov": len(list(g.of("CoverageItem")))}


def bc_rows(g, pkg):
    subj = lambda c: g.get(c.rel("subject")[0]).attrs["tddCode"]  # noqa: E731
    fm = lambda c: g.get(c.rel("faultModel")[0]).attrs["timing"] if c.rel("faultModel") else None  # noqa: E731
    m = bc_metrics(g, pkg)
    l1 = [tc.code for tc in g.of("TestCase") if tc.attrs["level"] == "L1"]
    l3 = [tc.code for tc in g.of("TestCase") if tc.attrs["level"] == "L3"]
    acc = tcs_for(g, lambda c: c.attrs["conditionKind"] == "access")
    eps = {b for tc in acc for b in g.get(g.get(g.get(tc).rel("covers")[0]).rel("realizes")[0]).prov.based_on
           if pkg.get(b) is not None and pkg.get(b).cls == "Endpoint"}
    return [
        ("G1", tcs_for(g, lambda c: c.attrs["conditionKind"] == "concurrency"), ""),
        ("G2", tcs_for(g, lambda c: fm(c) == "afterCommit" or "khởi động lại" in c.attrs["statement"].lower()), ""),
        ("G3", tcs_for(g, lambda c: c.attrs["conditionKind"] == "timing" and c.attrs["key"].startswith("worst-")), ""),
        ("G4", tcs_for(g, lambda c: subj(c) == "MECH-SchedulerThrottle"), ""),
        ("G5", tcs_for(g, lambda c: subj(c) in ("NFR-010", "NFR-013")), ""),
        ("G6", [], f"{m['meaninglessNfr']} test NFR vô nghĩa (TSO-016)"),
        ("G7", l3, "MOD-Notify, INT-02, INT-03 là ngữ cảnh của lát cắt CR-001; L3 sinh cho Mechanism và phụ thuộc trong phạm vi"),
        ("G8", l1, "L1 cho luồng chính, luồng lỗi và ngân sách của SCN-ScheduledRun"),
        ("G9", [], "chọn hồi quy ở Giai đoạn 7 (`tso_regression`)"),
        ("G10", [], "BR-10 chỉ có trong srs.md, không thuộc đầu vào; cần profile Extended"),
        ("G11", [], f"{m['numbered']}/{m['tr']} TR có tiêu chí đánh số theo điều kiện; {m['copied']} TR chép tên yêu cầu"),
        ("G12", [], f"{m['scoped']}/{m['tr']} TR có scopeNote khi chủ thể có nhiều TR"),
        ("G13", [], f"{sum(1 for x in g.of('TestGap') if 'chưa xác nhận' in x.attrs['missing'])} TestGap từ giả định chưa xác nhận"),
        ("G14", acc, f"phân quyền cho {len(eps)} endpoint"),
    ]


def write_bc(a, cfg, pkg, g):
    client = make_client(cfg)
    run_design_step(g, pkg, "cmp", client, cfg)
    _, _, rep = run_specify_step(g, pkg, "cmp", client, cfg)
    m = bc_metrics(g, pkg)
    lv = lambda c: ", ".join(f"{k}: {v}" for k, v in sorted(c.items()))  # noqa: E731
    lines = [f"## Chế độ `{a.mode}`", "", f"Gate trước duyệt ({', '.join(PRE_REVIEW[:1])}…): {rep.summary()}.", "",
             "| Chỉ số | v1 (`none`) | v2 |", "| --- | --- | --- |",
             f"| Test Requirement | 25 (L1: 1, L2: 23, L3: 1) | {m['tr']} ({lv(m['trByLevel'])}) |",
             f"| Test Design | 40 | {m['td']} |", f"| Coverage item | 183 | {m['cov']} |",
             f"| Test Case | 183 (L1: 2, L2: 158, L3: 23) | {m['tc']} ({lv(m['tcByLevel'])}) |",
             f"| TR có tiêu chí đánh số theo điều kiện | 0 | {m['numbered']}/{m['tr']} |",
             f"| TR chép tên yêu cầu | phần lớn | {m['copied']} |",
             f"| Assertion có oracle | không có khái niệm | {m['assertsWithOracle']}/{m['asserts']} |",
             f"| Test NFR vô nghĩa | có (101 %, ngưỡng 0, 6 chu kỳ) | {m['meaninglessNfr']} |", "",
             "| # | Test case v2 | Ghi chú |", "| --- | --- | --- |"]
    for gid, tcs, note in bc_rows(g, pkg):
        shown = ", ".join(f"`{x}`" for x in tcs[:3]) + (f" … ({len(tcs)})" if len(tcs) > 3 else "")
        lines.append(f"| {gid} | {shown or '—'} | {note} |")
    out = pathlib.Path(a.out)
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text("\n".join(lines) + "\n", encoding="utf-8")
    print(out)


if __name__ == "__main__":
    main()
