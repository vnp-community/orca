"""Free-form edits of one section of the TDD: "sửa mục 1.3 …"."""
import pytest

from harness import make
from test_agents_impact_edit import edit_graph, edit
from tdd_agent.agents.flows import default_flows
from tdd_agent.agents.promptedit import undo_last_edit
from tdd_agent.store.base import SourceDoc

DOC = ("# TDD\n\n## 1. Phạm vi\n\n### 1.3 Phi mục tiêu\n\n> **[THIẾU] Phi mục tiêu.** chưa có.\n\n### 1.4 Ràng buộc\n\nx\n\n## 2. Phân rã\n\n### 2.4 Phân bổ triển khai\n\n> **[THIẾU] Triển khai.** chưa có.\n")


def runtime(tmp_path, answer):
    def handler(req, n):
        if req.metadata.get("skill") == "write-section":
            return answer(req) if callable(answer) else answer
        raise AssertionError(req.metadata.get("skill"))
    o, know, ops, provider = make(default_flows(), handler, tmp_path)
    know.commit_graph(None, edit_graph(), actor="seed")
    know.register_source(SourceDoc.make("TDD-X", "1", "TDD", DOC))
    return o, know, ops, provider


def latest(know):
    return max((d for d in know.list_sources("TDD") if d.id == "TDD-X"), key=lambda d: int(d.revision))


def test_a_section_is_written_from_the_request_marked_as_a_persons_text_and_the_missing_notice_goes(tmp_path):
    o, know, ops, _ = runtime(tmp_path, {"mode": "append", "markdown": "- Không có ứng dụng native.\n- Không có chế độ offline.", "summary": "Hai phi mục tiêu."})
    run = edit(o, "Trong mục 1.3 Phi mục tiêu, thêm: không có ứng dụng native; không có chế độ offline.")
    assert run.status == "completed", run.result
    out = ops.get_task("e1", "P1")["output"]
    assert out["status"] == "committed" and out["section_edit"]["note_id"] == "NOTE-TDD-X-1-3" and out["section_edit"]["previous"] is None
    note = know.get_element("NOTE-TDD-X-1-3")
    assert note["class"] == "Section" and note["props"]["section"] == "1.3" and note["provenance"][0]["kind"] == "prompt"
    text = latest(know).text
    section = text.split("### 1.3 Phi mục tiêu", 1)[1].split("### 1.4", 1)[0]
    assert "Nội dung do người dùng chỉnh" in section and "- Không có ứng dụng native." in section and "THIẾU" not in section, section
    assert "NOTE-TDD-X-1-3" in text, "anchored, so the document check passes"


def test_adding_keeps_what_the_person_wrote_before_and_rewriting_replaces_it(tmp_path):
    answers = iter([{"mode": "append", "markdown": "- một", "summary": "s"}, {"mode": "append", "markdown": "- hai", "summary": "s"}, {"mode": "replace", "markdown": "- một và hai", "summary": "s"}])
    o, know, ops, _ = runtime(tmp_path, lambda req: next(answers))
    edit(o, "Mục 1.3: thêm một", run="e1"); edit(o, "Mục 1.3: thêm hai", run="e2")
    section = lambda: latest(know).text.split("### 1.3 Phi mục tiêu", 1)[1].split("### 1.4", 1)[0]
    assert "- một" in section() and "- hai" in section()
    edit(o, "Viết lại mục 1.3 cho gọn", run="e3")
    assert "- một và hai" in section() and "- hai\n" not in section().replace("- một và hai", "")


def test_the_person_s_verb_decides_between_adding_and_rewriting():
    import importlib.util
    spec = importlib.util.spec_from_file_location("write_section", "src/tdd_agent/skills/llm/write-section/skill.py")
    m = importlib.util.module_from_spec(spec); spec.loader.exec_module(m)
    assert m.decide_mode("Bổ sung vào mục 1.3 ba ý", "replace") == "append" and m.decide_mode("Viết lại mục 1.3 cho ngắn gọn", "append") == "replace"
    assert m.decide_mode("Mục 2.4: tất cả service chạy trên Kubernetes", "append") == "append" and m.decide_mode("Mục 2.4: x", "replace") == "replace", "no verb: the model decides"


def test_numbers_and_ids_the_request_did_not_give_are_refused(tmp_path):
    o, know, ops, _ = runtime(tmp_path, {"mode": "append", "markdown": "- SLA 99,95% mỗi tháng.", "summary": "s"})
    before = know.head().id
    run = edit(o, "Trong mục 2.4, ghi mức SLA phù hợp.")
    assert run.status == "blocked" and run.result["blockers"][0].startswith("SECTION_NUMBER_NOT_IN_REQUEST") and know.head().id == before
    o2, know2, _, _ = runtime(tmp_path / "b", {"mode": "append", "markdown": "- Xem FR-XYZ-01.", "summary": "s"})
    assert edit(o2, "Trong mục 2.4, ghi một tham chiếu.").result["blockers"][0].startswith("SECTION_ID_NOT_IN_REQUEST")
    o3, know3, _, _ = runtime(tmp_path / "c", {"mode": "append", "markdown": "- Chạy 3 broker.", "summary": "s"})
    assert edit(o3, "Trong mục 2.4, ghi: Kafka chạy 3 broker.").status == "completed", "a number the person gave is fine"


def test_an_unknown_section_a_missing_permission_and_an_element_id_are_not_section_edits(tmp_path):
    o, know, ops, _ = runtime(tmp_path, {"mode": "append", "markdown": "- x", "summary": "s"})
    run = edit(o, "Sửa mục 9.9 thành: không có gì.")
    out = ops.get_task("e1", "P1")["output"]
    assert out["status"] == "clarification_required" and out["reason"] == "SECTION_NOT_FOUND" and "9.9" in out["question"]
    denied = edit(o, "Trong mục 1.3, thêm: x.", auth={"scope": ["*"], "operations": ["add"]}, run="e2")
    assert denied.result["blockers"] == ["UNAUTHORIZED"]
    scoped = edit(o, "Trong mục 1.3, thêm: x.", auth={"scope": ["API-I-01"], "operations": ["modify"]}, run="e3")
    assert scoped.result["blockers"] == ["UNAUTHORIZED"], "a scope that does not name the section's note"


def test_undo_restores_the_earlier_text_and_refuses_when_the_graph_moved_on(tmp_path):
    answers = iter([{"mode": "append", "markdown": "- a", "summary": "s"}, {"mode": "append", "markdown": "- b", "summary": "s"}])
    o, know, ops, _ = runtime(tmp_path, lambda req: next(answers))
    edit(o, "Mục 1.3: thêm a", run="e1"); edit(o, "Mục 1.3: thêm b", run="e2")
    assert undo_last_edit(know, ops.get_task("e1", "P1")["output"], actor="u")["status"] == "conflict"
    assert undo_last_edit(know, ops.get_task("e2", "P1")["output"], actor="u")["status"] == "undone"
    assert know.get_element("NOTE-TDD-X-1-3")["props"]["markdown"] == "- a"
    assert undo_last_edit(know, ops.get_task("e2", "P1")["output"], actor="u")["status"] == "conflict"


def test_a_note_for_a_section_the_layout_no_longer_has_is_kept_in_the_appendix(tmp_path):
    from tdd_agent.render import RenderInfo, render_standard
    o, know, ops, _ = runtime(tmp_path, {"mode": "append", "markdown": "- x", "summary": "s"})
    edit(o, "Trong mục 1.3, thêm: x.")
    graph = know.materialize()
    graph["elements"] = [dict(e, props=dict(e["props"], section="7.9")) if e["id"] == "NOTE-TDD-X-1-3" else e for e in graph["elements"]]
    text, report = render_standard({"version": "v", "elements": graph["elements"], "edges": []}, RenderInfo(tdd_id="TDD-X", revision="2", snapshot_id="s", snapshot_hash="h"), "md")
    assert "Mục 7.9:" in text and report["ok"], report
