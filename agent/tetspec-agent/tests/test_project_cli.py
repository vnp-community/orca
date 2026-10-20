"""Project mode: init / generate / status inside a project folder, with a local SQLite knowledge graph."""
from __future__ import annotations

import json
import os

import pytest

from tdd_agent.cli import main
from tdd_agent.project import find_project_root

URD = "# URD\n| ID | Statement |\n|---|---|\n| REQ-1 | Evidence kept. |\n"
SRS = "# SRS\n## 5. Requirements\n### 5.1. Storage\n| ID | Statement | Acceptance |\n|---|---|---|\n| FR-1 | The system MUST store files. | AC-1 |\n"
SAD = "# SAD\n| ID | Decision | Rationale | Alternatives | Tradeoffs |\n|---|---|---|---|---|\n| ADR-1 | Kafka | log | RabbitMQ | cost |\n"


@pytest.fixture()
def project(tmp_path, monkeypatch):
    root = tmp_path / "P"
    (root / "docs").mkdir(parents=True)
    for name, text in (("URD", URD), ("SRS", SRS), ("SAD", SAD)):
        (root / "docs" / f"{name}.md").write_text(text, encoding="utf-8")
    monkeypatch.chdir(root)
    return root


def cli(capsys, *argv):
    code = main(list(argv))
    cap = capsys.readouterr()
    out = cap.out
    return code, (json.loads(out[out.index("{"):]) if "{" in out else None), cap.err


SOURCES = ("--urd", "docs/URD.md", "--srs", "docs/SRS.md", "--sad", "docs/SAD.md")


def init(capsys, *extra):
    return cli(capsys, "init", "--name", "P", *SOURCES, "--no-prose", *extra)


def scaffold(capsys):
    return init(capsys, "--no-generate")


def test_init_sets_the_project_up_and_builds_the_knowledge_graph_and_the_tdd(project, capsys):
    code, out, err = init(capsys)
    assert code == 0 and out["created"] is True and out["tdd_id"] == "TDD-P" and out["sources"]["srs"] == "docs/SRS.md"
    assert out["generate"]["status"] == "completed" and out["generate"]["revision"] == "1" and out["generate"]["kg"]["elements"] == 3
    assert (project / ".tdd-agent" / "config.toml").is_file() and (project / ".tdd-agent" / "knowledge.db").is_file()
    assert "## 1. Phạm vi và nguồn" in (project / "docs" / "tdd" / "TDD-P.md").read_text(encoding="utf-8"), "no separate generate step"
    assert "*.db" in (project / ".tdd-agent" / ".gitignore").read_text(), "the local knowledge graph is not meant for version control"
    assert "prose = false" in (project / ".tdd-agent" / "config.toml").read_text(), "--no-prose is remembered"
    assert "TDD written: docs/tdd/TDD-P.md" in err


def test_init_again_keeps_the_configuration_and_only_refreshes_when_the_documents_changed(project, capsys):
    init(capsys)
    config = (project / ".tdd-agent" / "config.toml").read_text()
    code, out, err = init(capsys)
    assert code == 0 and out["created"] is False and out["generate"]["status"] == "up_to_date" and "already initialized" in err
    assert (project / ".tdd-agent" / "config.toml").read_text() == config
    (project / "docs" / "SRS.md").write_text(SRS.replace("MUST store files", "MUST store files for 5 years"), encoding="utf-8")
    code, out, _ = init(capsys)
    assert out["generate"]["status"] == "completed" and out["generate"]["revision"] == "2"
    code, out, _ = init(capsys, "--force")
    assert out["created"] is True, "--force rewrites the configuration"


def test_init_without_documents_or_with_no_generate_only_scaffolds(project, capsys):
    code, out, err = cli(capsys, "init", "--name", "P")
    assert code == 0 and out["generate"]["status"] == "skipped" and "pass --urd/--srs/--sad" in err
    assert not (project / "docs" / "tdd" / "TDD-P.md").exists() and (project / ".tdd-agent" / "config.toml").is_file()
    code, out, _ = scaffold(capsys)
    assert out["generate"]["reason"] == "--no-generate" and not (project / ".tdd-agent" / "knowledge.db").exists()
    code, out, _ = cli(capsys, "generate", "--no-prose")
    assert code == 2 and out["error"]["code"] == "SOURCES_MISSING", "the configuration of a scaffold without --urd/--srs/--sad has no documents"


def test_other_commands_need_an_initialized_project(project, capsys):
    code, out, _ = cli(capsys, "generate")
    assert code == 2 and out["error"]["code"] == "PROJECT_NOT_INITIALIZED"
    code, out, _ = cli(capsys, "status")
    assert code == 2 and out["error"]["code"] == "PROJECT_NOT_INITIALIZED"


def test_generate_refreshes_the_tdd_and_the_knowledge_graph_after_the_documents_changed(project, capsys):
    scaffold(capsys)
    code, out, err = cli(capsys, "generate", "--no-prose")
    assert code == 0 and out["status"] == "completed" and out["tdd_id"] == "TDD-P" and out["revision"] == "1"
    doc = project / "docs" / "tdd" / "TDD-P.md"
    text = doc.read_text(encoding="utf-8")
    assert out["document"] == "docs/tdd/TDD-P.md" and "## 1. Phạm vi và nguồn" in text and "SRS §5.1" in text
    assert (project / ".tdd-agent" / "knowledge.db").is_file() and out["kg"]["elements"] == 3
    assert out["checks"]["ok"] is True and "TDD written: docs/tdd/TDD-P.md" in err, "JSON on stdout, a short summary for people on stderr"


def test_a_second_generate_with_unchanged_sources_does_nothing_and_a_changed_source_updates_the_same_document(project, capsys):
    init(capsys)
    code, out, err = cli(capsys, "generate", "--no-prose")
    assert code == 0 and out["status"] == "up_to_date" and "up to date" in err
    (project / "docs" / "SRS.md").write_text(SRS.replace("MUST store files", "MUST store files for 5 years"), encoding="utf-8")
    code, out, _ = cli(capsys, "status")
    assert {s["role"]: s["changed_since_last_generate"] for s in out["sources"]} == {"URD": False, "SRS": True, "SAD": False}
    code, out, _ = cli(capsys, "generate", "--no-prose")
    assert out["status"] == "completed" and out["revision"] == "2" and out["sources"]["SRS"]["revision"] == "2" and out["sources"]["URD"]["revision"] == "1"
    assert "for 5 years" in (project / "docs" / "tdd" / "TDD-P.md").read_text(encoding="utf-8"), "the same file, updated"
    code, out, _ = cli(capsys, "generate", "--no-prose", "--force")
    assert out["status"] == "completed" and out["revision"] == "3"


def test_the_project_is_found_from_a_sub_folder(project, capsys):
    scaffold(capsys)
    sub = project / "docs" / "deep"
    sub.mkdir()
    os.chdir(sub)
    assert find_project_root(sub) == project.resolve()
    code, out, _ = cli(capsys, "generate", "--no-prose")
    assert code == 0 and out["document"] == "docs/tdd/TDD-P.md" and (project / "docs" / "tdd" / "TDD-P.md").is_file()
    assert not (sub / ".tdd-agent").exists(), "state stays in the project root"


def test_flags_override_the_configured_documents_and_output_folder(project, capsys):
    scaffold(capsys)
    (project / "other").mkdir()
    (project / "other" / "SRS2.md").write_text(SRS.replace("FR-1", "FR-9"), encoding="utf-8")
    code, out, _ = cli(capsys, "generate", "--no-prose", "--srs", "other/SRS2.md", "--out", "build/tdd", "--id", "TDD-ALT")
    assert code == 0 and out["document"] == "build/tdd/TDD-ALT.md" and "FR-9" in (project / "build" / "tdd" / "TDD-ALT.md").read_text(encoding="utf-8")


def test_missing_documents_are_reported_clearly(project, capsys):
    (project / "docs" / "SAD.md").unlink()
    code, out, _ = init(capsys)
    assert code == 2 and out["error"]["code"] == "SOURCE_FILE_MISSING" and "SAD" in out["error"]["message"]
    assert (project / ".tdd-agent" / "config.toml").is_file(), "the project exists; only the first build failed"


def test_status_reports_the_knowledge_graph_and_the_last_document(project, capsys):
    init(capsys)
    code, out, err = cli(capsys, "status")
    assert code == 0 and out["knowledge"]["elements"] == 3 and out["last_generate"]["document"] == "docs/tdd/TDD-P.md" and out["needs_review"] == 3
    assert [d["id"] for d in out["documents"]] == ["TDD-P"] and "Knowledge graph: 3 elements" in err
    code, out, _ = cli(capsys, "kg", "stats")
    assert out["elements"] == 3, "the other commands use the project's knowledge graph too"


def test_mcp_enable_exposes_read_and_write_tools_by_default_and_read_only_on_request(project, capsys):
    init(capsys)
    code, out, _ = cli(capsys, "mcp", "enable")
    assert code == 0 and out["policy"] == {"write": True, "approvals": False, "client_actor": None} and "write: run_start" in out["tools"]
    assert "write = true" in (project / ".tdd-agent" / "config.toml").read_text()
    code, out, _ = cli(capsys, "mcp", "enable", "--read-only")
    assert out["policy"]["write"] is False and "(read-only)" in out["tools"]
    assert "write = false" in (project / ".tdd-agent" / "config.toml").read_text()
    code, out, _ = cli(capsys, "mcp", "enable", "--approvals")
    assert code == 2 and out["error"]["code"] == "ARGUMENT_INVALID", "approving through MCP stays opt-in and needs a registered client identity"


def test_change_followed_by_a_file_is_the_one_step_command_and_the_actions_are_left_alone():
    from tdd_agent.cli import _shorthand
    assert _shorthand(["-C", "p", "change", "cr.md", "--yes"]) == ["-C", "p", "change", "run", "cr.md", "--yes"]
    assert _shorthand(["change", "status"]) == ["change", "status"] and _shorthand(["change", "--help"]) == ["change", "--help"]
    assert _shorthand(["--set", "a.b=c", "status"]) == ["--set", "a.b=c", "status"]


def test_doctor_reports_each_check_with_its_fix_and_human_output_is_the_default_in_a_terminal(project, capsys, monkeypatch):
    scaffold(capsys)
    monkeypatch.setattr("shutil.which", lambda cmd: None)
    code, out, err = cli(capsys, "doctor", "--json")
    assert code == 1 and out["ok"] is False
    failed = {c["name"]: c for c in out["checks"] if not c["ok"]}
    assert "openspec" in failed and "install" in failed["openspec"]["fix"] and "baseline" in failed and "tdd-agent init" in failed["baseline"]["fix"]
    code = main(["--human", "doctor"])
    printed = capsys.readouterr()
    assert code == 1 and "[FAIL] openspec" in printed.out and "{" not in printed.out, "readable text on stdout, no JSON"


def test_init_can_choose_codex_as_the_agent_and_config_shows_who_does_what(project, capsys):
    code, out, err = init(capsys, "--no-generate", "--agent", "codex")
    assert code == 0 and 'default = "codex_cli"' in (project / ".tdd-agent" / "config.toml").read_text()
    code, out, _ = cli(capsys, "config")
    assert out["effective"]["agents"] == {"author": "codex_cli", "critic": "codex_cli", "extractor": "codex_cli"} and out["effective"]["approvals_mode"] == "auto"
    other = project.parent / "Q"
    (other / "docs").mkdir(parents=True)
    code, out, _ = cli(capsys, "-C", str(other), "init", "--no-generate", "--name", "Q")
    assert 'default = "claude_code"' in (other / ".tdd-agent" / "config.toml").read_text(), "claude stays the default"


def test_kg_search_reindex_and_index_check_from_the_command_line(project, capsys):
    init(capsys)
    code, out, err = cli(capsys, "--json", "kg", "index-check")
    assert code == 0 and out["ok"] is True and out["rows"] >= 3
    code, out, err = cli(capsys, "--json", "kg", "reindex")
    assert code == 0 and out["ok"] is True
    capsys.readouterr()
    code = main(["--json", "kg", "search", "fr-1"])
    hits = json.loads(capsys.readouterr().out)
    assert code == 0 and hits[0]["id"] == "FR-1" and hits[0]["class"] == "Requirement"
