"""Standard reader-facing TDD: the eleven parts of design v0.6 plus an appendix, every element anchored, missing information marked (never invented), no KG payload, no knowledge-graph-only codes."""
from __future__ import annotations

import re

import pytest

from tdd_agent.errors import ConfigError
from tdd_agent.render import RenderInfo, get_writer, render_standard
from tdd_agent.render.template import CHAPTERS

PROV = lambda doc, line, quote="q": [{"source_id": doc, "source_version": "1", "block": f"line:{line}", "kind": "extracted", "quote": quote, "source_hash": "h"}]


def el(id, cls, name=None, owner="SRS", line=1, **props):
    return {"id": id, "class": cls, "name": name or id, "owner_doc": owner, "maturity": "sketch", "review_status": "needs_review", "origin": "upstream",
            "props": props, "provenance": PROV(owner, line)}


def graph():
    return {"version": "T-v1", "elements": [
        el("FR-A-01", "Requirement", "Hệ thống phải gửi biên nhận", line=10, acceptance="Biên nhận gửi trong 5 giây"),
        el("FR-A-02", "Requirement", "Hệ thống phải lưu lịch sử", line=11),
        el("BRL-01", "BusinessRule", "Biên nhận gửi một lần", line=20, source_columns={"Mã": "BRL-01", "Nội dung": "Biên nhận gửi một lần", "FR": "FR-A-01"}),
        el("ENT-01", "Entity", "Receipt", line=30, upstream_definition={"m_t": "Biên nhận", "nh_danh": "receipt_id"}, source_columns={"Đọc": "API-P-01", "Ghi": "API-P-02"}),
        el("ENT-01:attribute:receipt_id", "Attribute", "receipt_id", owner="URD", line=5, entity="ENT-01", level="logical", physical_type="unknown"),
        el("ENT-01:attribute:sent_at", "Attribute", "sent_at", owner="URD", line=5, entity="ENT-01", level="logical", physical_type="timestamptz"),
        el("SM-ENT-01", "StateMachine", "ENT-01 lifecycle", line=40, owner_entity="ENT-01", states=["New", "Sent"]),
        el("SM-ENT-01:New", "State", "New", line=41, machine="SM-ENT-01"),
        el("SM-ENT-01:Sent", "State", "Sent", line=41, machine="SM-ENT-01"),
        el("TR-SM-ENT-01:New->Sent:aa", "Transition", "New → Sent", line=42, **{"from": "New", "to": "Sent", "event": "gửi", "guard": "email hợp lệ"}),
        el("ERR-timeout", "ErrorCode", "timeout", line=50, http_status=504, meaning="Quá 30 giây"),
        el("NFR-PE-01", "QualityTarget", "Gửi biên nhận", line=60, source_section="SRS > 6. Yêu cầu phi chức năng > 6.1. Hiệu năng (PE)", ch_ti_u_o_c="p95 ≤ 2 giây"),
        el("NFR-SE-01", "QualityTarget", "Phiên đăng nhập", line=61, source_section="SRS > 6. Yêu cầu phi chức năng > 6.5. Bảo mật (SE)"),
        el("ADR-01", "Decision", "Dùng Go", owner="SAD", line=70, chosen="Go cho backend"),
        el("FACT-0123456789", "Constraint", "Quy mô thiết kế", line=12, statement="500 dự án, 1 triệu Usage", evidence_span="Quy mô thiết kế: 500 dự án"),
        el("FR-A-03", "Requirement", "Hệ thống phải xuất báo cáo", line=12, source_section="SRS > 5. Yêu cầu chức năng > 5.1. Quản trị (FR-A)"),
    ], "edges": [
        {"src": "FR-A-01", "rel": "traces_to", "dst": "BRL-01", "provenance": PROV("SRS", 20)},
        {"src": "FR-A-01", "rel": "traces_to", "dst": "ENT-01", "provenance": PROV("SRS", 30)},
        {"src": "FR-A-01", "rel": "satisfies", "dst": "NFR-PE-01", "provenance": PROV("SRS", 60)},
    ], "gaps": [
        {"id": "GAP-API-P-01-UNDEFINED", "need": "definition", "target": "API-P-01", "referenced_by": "FR-A-01", "reason": "API-P-01 is referenced by FR-A-01 but none of the supplied documents defines it", "status": "missing"},
    ], "conflicts": []}


SRS = "# SRS\n## 1. Giới thiệu\n### 1.2. Phạm vi\n**Ngoài phạm vi v1.0:**\n- Đa khách hàng\n- Tự sửa mã\n\n## 9. Triển khai và hạ tầng\n" + "\n" * 8 + "Nội dung\n"


def visible(text: str) -> str:
    """The text as a reader sees it: without the invisible anchors and link targets."""
    return re.sub(r"\]\(#[^)]*\)", "]", re.sub(r'<a id="[^"]*"></a>', "", text))


def render(g=None, **kw):
    info = RenderInfo(tdd_id="TDD-X", revision="1", snapshot_id="snap-000001", snapshot_hash="abc123", readiness="blocked", source_texts={("SRS", "1"): SRS}, **kw)
    return render_standard(g or graph(), info)


def test_the_document_has_the_eleven_parts_of_the_design_in_order_plus_an_appendix():
    text, report = render()
    heads = re.findall(r"^## (\d+|A)\. (.+)$", text, re.M)
    assert [h[1] for h in heads] == [c.vi for c in CHAPTERS] and [h[0] for h in heads] == [c.number for c in CHAPTERS]
    assert [c.number for c in CHAPTERS] == [str(i) for i in range(1, 12)] + ["A"]
    assert report["ok"] and report["chapters"] == 12
    for needle in ("Phạm vi và nguồn", "Phân rã hệ thống", "Hành vi", "Hợp đồng giao diện", "Dữ liệu", "Giao dịch và khả năng chịu lỗi", "Di trú dữ liệu", "Chất lượng và vận hành", "Bảo mật", "Quyết định, giả định và thông tin còn thiếu", "Kiểm chứng và truy vết"):
        assert needle in [h[1] for h in heads]


def test_every_element_has_an_anchor_and_every_row_names_its_source_or_rule():
    g = graph()
    text, report = render(g)
    assert report["anchored"] == len(g["elements"]) and not report["missing_elements"] and report["rows_without_source"] == 0
    for e in g["elements"]:
        assert f'<a id="{e["id"].replace("->", "-_")}"></a>' in text or f'id="{e["id"]}"' in text
    line, _ = render(source_detail="line")
    assert "SRS@1 d.10" in line, "audit detail: document, revision, line"


def test_the_source_of_a_row_is_the_section_of_the_source_document():
    text, _ = render()
    assert "SRS §5.1" in text, "from the source section of the element"
    assert "SRS §9" in text, "no section recorded: the nearest numbered heading above the quoted line"


def test_rendering_is_deterministic_and_states_the_snapshot_without_a_timestamp():
    a, ra = render()
    b, _ = render()
    assert a == b and ra["deterministic"] and "abc123" in a and "snap-000001" in a


def test_no_knowledge_graph_payload_is_embedded():
    text, _ = render()
    assert "Canonical" not in text and "zlib" not in text and "round-trip" not in text and "Value [" not in text


def test_codes_that_only_the_knowledge_graph_knows_are_never_shown_to_the_reader():
    text, _ = render()
    v = visible(text)
    assert "FACT-" not in v and "TR-SM" not in v and ":attribute:" not in v and "SM-ENT" not in v and "GAP-" not in v
    assert "Quy mô thiết kế" in v and "FR-A-01" in v and "ENT-01" in v, "the name of a prose fact and the codes of the source documents stay"
    assert '<a id="FACT-0123456789"></a>' in text, "the element is still anchored (invisibly)"


def test_missing_information_is_marked_not_invented():
    text, _ = render()
    assert "**THIẾU**" in text and "timestamptz" in text, "a type that the data has is shown as is"
    for what in ("Ranh giới tin cậy và ma trận phân quyền", "Mô hình mối đe doạ", "Kiến trúc triển khai", "Test case, kịch bản kiểm thử", "Danh mục rủi ro", "Schema trước/sau"):
        assert f"[THIẾU] {what}" in text


def test_non_goals_are_a_question_for_the_ba_and_quote_the_source_passage_verbatim():
    text, _ = render()
    sec = text.split("### 1.3 Phi mục tiêu", 1)[1].split("### 1.4 ", 1)[0]
    assert "[THIẾU] Phi mục tiêu" in sec and "BA / chủ sản phẩm" in sec
    assert "Đa khách hàng" in sec and "Tự sửa mã" in sec and "SRS §1.2, dòng 4" in sec
    none, _ = render_standard(graph(), RenderInfo(tdd_id="TDD-X", snapshot_id="s", snapshot_hash="h", source_texts={("SRS", "1"): "# SRS\n"}))
    assert "không có mục 'ngoài phạm vi'" in none, "no passage in the sources: say so, do not infer non-goals"


def test_traceability_comes_from_edges_and_marks_undefined_references():
    text, _ = render()
    sec = text.split("### 11.3 ", 1)[1].split("### 11.4 ", 1)[0]
    assert "[BRL-01](#BRL-01)" in sec and "[ENT-01](#ENT-01)" in sec and "[NFR-PE-01](#NFR-PE-01)" in sec
    assert "API-P-01 *" in sec, "an id referenced but undefined is flagged"


def test_quality_targets_are_placed_by_the_heading_of_their_source_group():
    text, _ = render()
    assert "NFR-PE-01" in text.split("## 8. ", 1)[1].split("## 9. ", 1)[0]
    assert "NFR-SE-01" in text.split("## 9. ", 1)[1].split("## 10. ", 1)[0]
    assert "500 dự án" in text.split("## 8. ", 1)[1].split("## 9. ", 1)[0], "a prose constraint about scale goes to quality"


def test_acceptance_criteria_are_quoted_and_requirements_without_them_are_listed():
    text, _ = render()
    sec = text.split("### 11.1 ", 1)[1].split("### 11.2 ", 1)[0]
    assert "Biên nhận gửi trong 5 giây" in sec and "1/5 yêu cầu có tiêu chí" in sec and "FR-A-02" in sec


def test_the_front_status_table_and_the_missing_summary_agree_with_the_body():
    text, report = render()
    assert report["missing_blocks"] == text.count("> **[THIẾU]")
    summary = text.split("### 10.7 ", 1)[1].split('<a id="ch-11">', 1)[0]
    assert summary.count("\n| ") - 1 == report["missing_blocks"], "one summary row per MISSING block"
    assert "chưa sẵn sàng" in text.split("### 0.2 ", 1)[0], "readiness is explained in words"


def test_unknown_output_format_is_a_coded_configuration_error():
    with pytest.raises(ConfigError) as exc:
        get_writer("pdf")
    assert exc.value.code == "RENDER_FORMAT_UNKNOWN"
    assert get_writer("md").extension == ".md"


def test_english_titles_are_available_by_configuration():
    text, _ = render(language="en")
    assert "## 5. Data" in text and "Technical Design Document" in text


def test_business_objectives_of_the_urd_fill_the_goals_of_part_one_and_acceptance_ids_are_not_facts():
    from tdd_agent.skills.lib import facts, text as T
    urd = "# URD\n## 3.2. Mục tiêu kinh doanh\n| Mã | Mục tiêu | Chỉ số |\n|---|---|---|\n| BO-01 | Tập trung vòng đời kiểm thử vào một nền tảng | 100% dự án áp dụng |\n"
    out = facts.structured_facts(urd, "URD", "1", T.segment(urd))
    assert [(e["id"], e["class"]) for e in out["elements"]] == [("BO-01", "Requirement")] and out["gaps"] == []
    srs = "# SRS\n| Mã | Chấp nhận |\n|---|---|\n| AC-53.2 | Bug đổi trạng thái |\n"
    assert facts.structured_facts(srs, "SRS", "1", T.segment(srs)) == {"elements": [], "edges": [], "gaps": []}, "no element and no GAP-AC-…-CLASS"
    own = facts.structured_facts("# X\n| Mã | Nội dung |\n|---|---|\n| ZZ-1 | something |\n", "X", "1", T.segment("# X\n| Mã | Nội dung |\n|---|---|\n| ZZ-1 | something |\n"), {"ZZ": "Constraint"})
    assert [e["class"] for e in own["elements"]] == ["Constraint"], "[project.id_classes] extends the table"
