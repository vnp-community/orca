import io
import json
import logging

from tdd_agent.obs import Observer, Redactor, bind, configure_logging, log_event, get_logger, render_tree
from tdd_agent.obs.tracing import Tracer
from tdd_agent.store.memory import MemoryOperationalStore


def test_json_log_lines_carry_correlation_ids_and_redact_secrets(tmp_path):
    path = tmp_path / "logs" / "x.jsonl"
    configure_logging("INFO", path, False)
    log = get_logger("t")
    with bind(run_id="run-9", trace_id="tr", agent="impact", gate="G2", step="scan"):
        log_event(log, "thing_happened", token="supersecretvalue", note="mail me at a@b.com", n=3)
    line = json.loads(path.read_text().splitlines()[-1])
    assert line["event"] == "thing_happened" and line["run_id"] == "run-9" and line["gate"] == "G2" and line["agent"] == "impact"
    assert line["token"] == "***" and "a@b.com" not in line["note"] and line["n"] == 3 and line["level"] == "INFO"
    configure_logging("INFO", None, False)


def test_spans_nest_persist_and_render_including_blocked():
    ob = Observer.in_memory()
    with bind(run_id="r1"):
        with ob.span("run:generation", "run") as run:
            with ob.span("G2", "gate") as gate:
                with ob.span("scan", "step") as step:
                    step.set(count=3)
                gate.block("SOURCE_ROLE_MISSING:SAD")
    spans = ob.store.list_spans(run_id="r1")
    assert {s["trace_id"] for s in spans} == {spans[0]["trace_id"]} and len(spans) == 3
    by_name = {s["name"]: s for s in spans}
    assert by_name["G2"]["parent_id"] == by_name["run:generation"]["span_id"] and by_name["scan"]["parent_id"] == by_name["G2"]["span_id"]
    assert by_name["G2"]["status"] == "blocked" and by_name["G2"]["blockers"] == ["SOURCE_ROLE_MISSING:SAD"]
    assert by_name["scan"]["attributes"]["count"] == 3 and all(s["duration_ms"] >= 0 for s in spans)
    tree = render_tree(spans)
    assert "gate" in tree and "[blocked]" in tree and "blockers=SOURCE_ROLE_MISSING:SAD" in tree
    assert tree.splitlines()[1].startswith("  ") and tree.splitlines()[2].startswith("    ")


def test_exception_marks_span_error_and_reraises():
    ob = Observer.in_memory()
    try:
        with ob.span("boom", "step"):
            raise ValueError("kaput")
    except ValueError:
        pass
    [s] = ob.store.list_spans()
    assert s["status"] == "error" and s["attributes"]["error_code"] == "ValueError"


def test_missing_opentelemetry_does_not_fail_runs(monkeypatch):
    import builtins
    real = builtins.__import__

    def fake_import(name, *a, **k):
        if name.startswith("opentelemetry"):
            raise ImportError(name)
        return real(name, *a, **k)

    monkeypatch.setattr(builtins, "__import__", fake_import)
    tracer = Tracer(MemoryOperationalStore(), otlp_endpoint="http://localhost:4318/v1/traces")
    with tracer.span("still works", "step"):
        pass
    assert tracer.store.list_spans()[0]["status"] == "ok"


def test_redactor_masks_keys_and_patterns():
    r = Redactor([r"internal-\d+"])
    out = r.data({"api_key": "abc", "token_env": "TDD_AGENT_MCP_TOKEN", "text": "Bearer abcdefghijklmnop and internal-42", "nested": [{"password": "p"}]})
    assert out["api_key"] == "***" and out["token_env"] == "TDD_AGENT_MCP_TOKEN"
    assert "abcdefghijklmnop" not in out["text"] and "internal-42" not in out["text"] and out["nested"][0]["password"] == "***"


def test_metrics_llm_share_per_agent():
    ob = Observer.in_memory()
    for _ in range(3): ob.metrics.skill_resolution("impact", "scan-inventory-overlap", "rule")
    ob.metrics.skill_resolution("impact", "judge-design-coverage", "llm")
    assert ob.metrics.llm_share()["impact"]["llm_share"] == 0.25
    ob.metrics.blocked("G2", "SAD_READONLY"); ob.metrics.blocked("G2", "SAD_READONLY")
    assert {m["labels"]["code"]: m["value"] for m in ob.store.list_metrics("blocked_total")} == {"SAD_READONLY": 2.0}
