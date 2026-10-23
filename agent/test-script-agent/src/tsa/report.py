"""assertion-manifest.json and tsa-report.md, written next to the generated scripts. The report text is Vietnamese (read by QA)."""
from __future__ import annotations

from .conformance import ERROR, INFO, WARNING


def manifest(o) -> dict:
    return {"schema": "tsa-manifest/v1", "feature": o.feature, "specVersion": o.spec_version, "branch": o.branch, "generator": o.generator,
            "assertions": [r for f in o.files for r in f.manifest]}


def _cell(v) -> str:
    return str(v if v is not None else "").replace("|", "\\|").replace("\n", " ")


def markdown(o, bundle) -> str:
    c = o.counts
    lines = [f"# TSA: {o.feature} v{o.spec_version}", "",
             f"- Nhánh: `{o.branch}`; generator: `{o.generator}`; TCIR: trạng thái `{bundle.status}`, baseline `{bundle.baseline}`",
             f"- Test case: đã sinh {c.get('generated', 0)}, cần xem lại {c.get('needs-review', 0)}, lỗi {c.get('failed', 0)}, "
             f"bỏ qua {c['skipped']}",
             f"- Chi phí LLM: {sum(f.cost_usd for f in o.files):.4f} USD; thời gian {o.seconds:.0f} s",
             "- Script chưa được chạy (runtime là bước sau). Dòng có `@exp` lấy từ spec: muốn đổi thì sửa spec, không sửa script.", "",
             "## Tệp", "", "| Tệp | Nhóm | Trạng thái | Số lần sinh | Test case | Lỗi | Cảnh báo |", "| --- | --- | --- | --- | --- | --- | --- |"]
    for f in o.files:
        sev = [x.severity for x in f.findings]
        lines.append(f"| `{f.path}` | {_cell(f.group)} | {f.status} | {f.attempts} | {len(f.cases)} | {sev.count(ERROR)} | "
                     f"{sev.count(WARNING)} |")
    lines += ["", "## Độ phủ kết quả mong đợi", "", "| Expected | Test case | Toán tử | Giá trị spec | Trạng thái | Vị trí |",
              "| --- | --- | --- | --- | --- | --- |"]
    for f in o.files:
        for r in f.manifest:
            where = f"`{r['file']}:{r['line']}`" if r.get("file") else ""
            lines.append(f"| `{r['exp']}` | `{r['case']}` | `{_cell(r['op'])}` | {_cell(r['value'])[:80]} | {r['status']} | {where} |")
    issues = [(f.path, x) for f in o.files for x in f.findings if x.severity != INFO]
    lines += ["", "## Vấn đề", ""]
    if issues:
        lines += ["| Mức | Luật | Vị trí | Nội dung |", "| --- | --- | --- | --- |"]
        lines += [f"| {x.severity} | {x.rule} | `{x.file or p}{':' + str(x.line) if x.line else ''}` | {_cell(x.message)} |" for p, x in issues]
    else:
        lines.append("Không có.")
    gap_rows = [r for f in o.files for r in f.manifest if r.get("status") == "gap"]
    if gap_rows:
        lines += ["", "## Expected chưa assert được (spec còn thiếu)", "",
                  "Các kết quả mong đợi dưới đây chưa được spec định nghĩa đủ để assert; cần BA/QA bổ sung spec rồi sinh lại.", ""]
        lines += [f"- `{r['exp']}` (`{r['case']}`): {_cell(r.get('reason'))}" for r in gap_rows]
    unchecked = sum(1 for f in o.files for x in f.findings if x.rule == "value-unchecked")
    if unchecked:
        lines += ["", f"{unchecked} expected có giá trị dạng văn xuôi: chỉ kiểm được toán tử, QA cần đọc lại các assertion này."]
    lines += ["", "## Test case bỏ qua", ""]
    lines += ([f"- `{s.case}`: {s.reason}" for s in o.skipped] or ["Không có."])
    return "\n".join(lines) + "\n"
