import json

import pytest

from tdd_agent.cli import main, parse_source
from tdd_agent.errors import TddAgentError


def run_cli(capsys, tmp_path, *argv):
    base = ["--set", f"store.knowledge=sqlite:{tmp_path}/k.db", "--set", f"store.operational=sqlite:{tmp_path}/o.db", "--set", f"paths.work_dir={tmp_path}/work",
            "--set", f"obs.log_file={tmp_path}/log.jsonl"]
    code = main(base + list(argv))
    out = capsys.readouterr().out
    try:
        return code, json.loads(out[out.index("{") if out.lstrip().startswith("{") or "\n{" in out else out.index("["):]) if out.strip() else None
    except ValueError:
        return code, out


@pytest.fixture()
def docs(tmp_path):
    (tmp_path / "urd.md").write_text("# URD\n| ID | Statement |\n|---|---|\n| REQ-1 | Evidence kept. |\n")
    (tmp_path / "srs.md").write_text("# SRS\n| ID | Statement | Acceptance |\n|---|---|---|\n| FR-1 | The system MUST store files. | AC-1 |\n")
    (tmp_path / "sad.md").write_text("# SAD\n| ID | Decision | Rationale | Alternatives | Tradeoffs |\n|---|---|---|---|---|\n| ADR-1 | Kafka | log | RabbitMQ | cost |\n")
    (tmp_path / "request.json").write_text(json.dumps({"mode": "initial", "params": {"tdd_id": "TDD-CLI", "prose": False}}))
    return tmp_path


def test_parse_source_and_errors(tmp_path):
    (tmp_path / "a.md").write_text("hello")
    d = parse_source(f"SRS@2:srs={tmp_path}/a.md")
    assert (d.id, d.revision, d.role, d.text) == ("SRS", "2", "SRS", "hello")
    with pytest.raises(TddAgentError) as e:
        parse_source("nonsense")
    assert e.value.code == "SOURCE_SPEC_INVALID"


def test_config_command_masks_and_reports_fingerprint(capsys, tmp_path):
    code, out = run_cli(capsys, tmp_path, "config")
    assert code == 0 and out["config"]["llm"]["default"] == "claude_code" and out["config"]["llm"]["claude_code"]["model"] == "claude-sonnet-5-5" and len(out["fingerprint"]) == 64


def test_invalid_configuration_is_a_usage_error(capsys, tmp_path):
    code, out = run_cli(capsys, tmp_path, "--set", "llm.default=ghost", "run", "list")
    assert code == 2 and out["error"]["code"] == "PROVIDER_UNKNOWN"


def test_generation_run_through_the_cli_then_inspect(capsys, docs):
    t = docs
    code, out = run_cli(capsys, t, "run", "start", "tdd-generation", "--request", str(t / "request.json"), "--run-id", "cli1",
                        "--source", f"URD@1:URD={t}/urd.md", "--source", f"SRS@1:SRS={t}/srs.md", "--source", f"SAD@1:SAD={t}/sad.md")
    assert code == 0 and out["status"] == "completed" and out["flow"] == "tdd-generation", out
    code, out = run_cli(capsys, t, "kg", "stats")
    assert code == 0 and out["elements"] == 3 and out["by_class"]["Decision"] == 1
    code, out = run_cli(capsys, t, "run", "list")
    assert [r["run_id"] for r in out] == ["cli1"]
    code, out = run_cli(capsys, t, "run", "show", "cli1")
    assert out["tasks"]["G3"]["status"] == "completed" and out["transitions"][-1]["to"] == "tdd_realized"
    code, out = run_cli(capsys, t, "review", "--list")
    assert {e["id"] for e in out} == {"REQ-1", "FR-1", "ADR-1"}
    code, out = run_cli(capsys, t, "review", "--actor", "nobody", "--role", "BA", "--rationale", "x", "--all-pending")
    assert code == 1 and out["error"]["code"] == "REVIEW_NOT_PERMITTED"
    code, out = run_cli(capsys, t, "metrics")
    assert code == 0 and "runs_total" in out["metrics"]


def test_approval_commands_fail_cleanly(capsys, docs):
    t = docs
    run_cli(capsys, t, "run", "start", "tdd-generation", "--request", str(t / "request.json"), "--run-id", "cli2", "--source", f"URD@1:URD={t}/urd.md",
            "--source", f"SRS@1:SRS={t}/srs.md", "--source", f"SAD@1:SAD={t}/sad.md")
    code, out = run_cli(capsys, t, "decision", "cli2", "G3", "--actor", "coding-agent")
    assert code == 1 and out["error"]["code"] == "NO_PENDING_DECISION"
    code, out = run_cli(capsys, t, "approve", "cli2", "G3", "--actor", "coding-agent", "--role", "SA", "--rationale", "x", "--hash", "abc")
    assert code == 1 and out["code"] in ("GATE_NOT_APPROVABLE", "ACTOR_NOT_ALLOWLISTED")


def test_skills_commands(capsys, tmp_path):
    code, out = run_cli(capsys, tmp_path, "skills", "list")
    assert code == 0 and len(out) == 23 and {s["tier"] for s in out} == {"logic", "hybrid", "llm"}
    code, out = run_cli(capsys, tmp_path, "skills", "test", "--tiers", "logic")
    assert code == 0 and out["failed"] == 0 and out["passed"] >= 9
    code, out = run_cli(capsys, tmp_path, "skills", "demotion-report")
    assert code == 0 and out["calls_total"] == 0 and out["candidates"] == []
