import json

import pytest

from helpers import calls, claude_envelope, make_cli
from tdd_agent.config import load_config
from tdd_agent.errors import ConfigError
from tdd_agent.llm.base import LlmRequest, ProviderFailure
from tdd_agent.llm.claude_code import ClaudeCodeProvider
from tdd_agent.llm.codex_cli import CodexCliProvider
from tdd_agent.llm.fake import FakeProvider
from tdd_agent.llm.gateway import LlmGateway
from tdd_agent.llm.registry import available, create_provider, register_provider
from tdd_agent.obs import Observer

SCHEMA = {"type": "object", "properties": {"answer": {"type": "integer"}}, "required": ["answer"], "additionalProperties": False}


def claude(tmp_path, scenario, **settings):
    cli = make_cli(tmp_path, "claude", scenario)
    return ClaudeCodeProvider({"command": str(cli), "model": "claude-sonnet-5-5", "effort": "medium", "timeout_seconds": 5, "max_attempts": 2, **settings})


def req(**kw):
    return LlmRequest(role=kw.pop("role", "extractor"), prompt=kw.pop("prompt", "2+2"), output_schema=kw.pop("schema", SCHEMA), **kw)


def test_claude_command_shape_and_success(tmp_path):
    p = claude(tmp_path, [{"stdout": claude_envelope(structured={"answer": 4}, result='{"answer":4}')}])
    r = p.complete(req(prompt="what is 2+2?"))
    assert r.ok and r.parsed == {"answer": 4} and r.model == "claude-sonnet-5-5" and r.effort == "medium"
    assert r.usage == {"input_tokens": 3, "output_tokens": 5} and r.cost_usd == 0.01 and r.evidence_grade == "real"
    [call] = calls(tmp_path)
    argv = call["argv"]
    assert call["stdin"] == "what is 2+2?"
    assert argv[argv.index("--model") + 1] == "claude-sonnet-5-5" and argv[argv.index("--effort") + 1] == "medium"
    assert argv[argv.index("--tools") + 1] == "" and argv[argv.index("--setting-sources") + 1] == ""
    assert json.loads(argv[argv.index("--json-schema") + 1]) == SCHEMA
    for flag in ("-p", "--no-session-persistence", "--disable-slash-commands", "--strict-mcp-config", "--no-chrome"):
        assert flag in argv
    assert "--bare" not in argv, "--bare disables the subscription login and must never be used"


def test_claude_text_json_fallback_and_code_fence(tmp_path):
    p = claude(tmp_path, [{"stdout": claude_envelope(result='```json\n{"answer": 7}\n```')}])
    assert p.complete(req()).parsed == {"answer": 7}


def test_schema_invalid_then_success_records_both_attempts(tmp_path):
    p = claude(tmp_path, [{"stdout": claude_envelope(structured={"answer": "four"})}, {"stdout": claude_envelope(structured={"answer": 4})}])
    r = p.complete(req())
    assert r.ok and len(r.attempts) == 2 and r.attempts[0]["error"] == "OUTPUT_SCHEMA_INVALID" and r.attempts[1]["ok"]
    assert r.usage["input_tokens"] == 6, "usage accumulates across attempts"


def test_schema_invalid_every_attempt_is_blocked_without_repair(tmp_path):
    p = claude(tmp_path, [{"stdout": claude_envelope(structured={"wrong": 1})}])
    r = p.complete(req())
    assert r.status == "blocked" and r.blockers == ["OUTPUT_SCHEMA_INVALID"] and r.parsed is None and len(r.attempts) == 2


def test_non_json_stdout_is_bad_output_and_retried(tmp_path):
    p = claude(tmp_path, [{"stdout": "hello"}])
    r = p.complete(req())
    assert r.blockers == ["PROVIDER_BAD_OUTPUT"] and len(calls(tmp_path)) == 2


def test_rate_limit_is_not_retried(tmp_path):
    p = claude(tmp_path, [{"stdout": claude_envelope(result="You've hit your usage limit", is_error=True, api_error_status=429)}])
    r = p.complete(req())
    assert r.blockers == ["PROVIDER_RATE_LIMITED"] and len(calls(tmp_path)) == 1


def test_unsupported_option_and_auth(tmp_path):
    p = claude(tmp_path, [{"stderr": "error: unknown option '--effort'", "exit": 1}])
    assert p.complete(req()).blockers == ["PROVIDER_OPTION_UNSUPPORTED"] and len(calls(tmp_path)) == 1
    q = claude(tmp_path / "b", [{"stdout": claude_envelope(result="Not logged in · Please run /login", is_error=True)}])
    assert q.complete(req()).blockers == ["PROVIDER_AUTH_REQUIRED"]


def test_timeout_is_retried_then_blocked(tmp_path):
    p = claude(tmp_path, [{"sleep": 3, "stdout": claude_envelope(structured={"answer": 1})}], timeout_seconds=1)
    r = p.complete(req())
    assert r.status == "blocked" and r.blockers == ["PROVIDER_TIMEOUT"] and len(r.attempts) == 2


def test_missing_binary_and_non_read_only_tools(tmp_path):
    r = ClaudeCodeProvider({"command": str(tmp_path / "nope")}).complete(req())
    assert r.blockers == ["PROVIDER_UNAVAILABLE"] and len(r.attempts) == 1
    p = claude(tmp_path, [{"stdout": claude_envelope(structured={"answer": 1})}])
    assert p.complete(req(allowed_tools=("Bash",))).blockers == ["PROVIDER_OPTION_UNSUPPORTED"]
    assert p.complete(req(allowed_tools=("Read", "Grep"))).ok
    assert calls(tmp_path)[-1]["argv"][calls(tmp_path)[-1]["argv"].index("--tools") + 1] == "Read,Grep"


def test_describe_has_no_secrets_and_reports_version(tmp_path):
    d = claude(tmp_path, [{}]).describe()
    assert d["cli_version"] == "fake-cli 1.0" and d["model"] == "claude-sonnet-5-5" and d["effort"] == "medium"


def codex(tmp_path, scenario, **settings):
    cli = make_cli(tmp_path, "codex", scenario)
    return CodexCliProvider({"command": str(cli), "model": "gpt-x", "timeout_seconds": 5, "max_attempts": 2, **settings})


def test_codex_command_shape_and_success(tmp_path):
    p = codex(tmp_path, [{"output_file": '{"answer": 3}'}])
    r = p.complete(req(system="be brief"))
    assert r.ok and r.parsed == {"answer": 3} and r.model == "gpt-x"
    argv = calls(tmp_path)[0]["argv"]
    assert argv[:1] == ["exec"] and argv[argv.index("--model") + 1] == "gpt-x"
    assert argv[argv.index("--sandbox") + 1] == "read-only" and "--ephemeral" in argv and "--output-schema" in argv and argv[-1] == "-"
    assert calls(tmp_path)[0]["stdin"].startswith("be brief")


def test_codex_failures(tmp_path):
    assert codex(tmp_path, [{"stderr": "429 Too Many Requests", "exit": 1}]).complete(req()).blockers == ["PROVIDER_RATE_LIMITED"]
    assert codex(tmp_path / "b", [{"exit": 0}]).complete(req()).blockers == ["PROVIDER_BAD_OUTPUT"]
    assert codex(tmp_path / "c", [{"output_file": "{}"}]).complete(req(allowed_tools=("Read",))).blockers == ["PROVIDER_OPTION_UNSUPPORTED"]


def test_registry():
    assert {"claude_code", "codex_cli"} <= set(available())
    with pytest.raises(ConfigError) as e:
        create_provider("nope")
    assert e.value.code == "PROVIDER_UNKNOWN"
    register_provider("custom_test", lambda s: FakeProvider({"*": {"answer": 1}}, s))
    assert create_provider("custom_test").name == "fake"


# ----------------------------------------------------------------- gateway
def gw(config=None, providers=None, mode="none"):
    cfg = config or load_config(env={})
    ob = Observer(Observer.in_memory().store, payload_mode=mode)
    return LlmGateway(cfg, ob, providers or {"claude_code": FakeProvider({"*": {"answer": 4}})}), ob


def test_gateway_default_provider_is_claude_code_and_audit_has_no_payload():
    g, ob = gw()
    assert g.role_map == {"author": "claude_code", "critic": "claude_code", "extractor": "claude_code"}
    r = g.call_json("extractor", "secret prompt text", SCHEMA)
    assert r.ok and r.evidence_grade == "fixture"
    [rec] = ob.store.list_llm_calls()
    assert rec["provider"] == "fake" and rec["payload_mode"] == "none" and "payload" not in rec and rec["prompt_hash"]
    assert "secret prompt text" not in json.dumps(rec)
    spans = ob.store.list_spans()
    assert [s["kind"] for s in spans] == ["llm_call"] and spans[0]["status"] == "ok"


def test_gateway_full_payload_is_redacted_and_opt_in():
    g, ob = gw(mode="full")
    g.call_json("extractor", "contact a.b@example.com with token=abcdef123456", SCHEMA)
    rec = ob.store.list_llm_calls()[0]
    assert rec["payload_mode"] == "full" and "a.b@example.com" not in json.dumps(rec) and "abcdef123456" not in json.dumps(rec)
    assert rec["output_hash"]


def test_gateway_role_mapping_and_distinct_critic():
    cfg = load_config(env={"TDD_AGENT__LLM__ROLES__CRITIC": "codex_cli"})
    a, c = FakeProvider({"*": {"answer": 1}}), FakeProvider({"*": {"answer": 2}})
    g = LlmGateway(cfg, Observer.in_memory(), {"claude_code": a, "codex_cli": c})
    assert g.call_json("author", "x", SCHEMA).parsed == {"answer": 1} and g.call_json("critic", "x", SCHEMA).parsed == {"answer": 2}
    same = load_config(env={"TDD_AGENT__LLM__REQUIRE_DISTINCT_CRITIC": "true"})
    with pytest.raises(ConfigError) as e:
        LlmGateway(same, Observer.in_memory(), {"claude_code": FakeProvider({"*": {}})})
    assert e.value.code == "CRITIC_NOT_DISTINCT"
    cfg2 = load_config(env={"TDD_AGENT__LLM__REQUIRE_DISTINCT_CRITIC": "true", "TDD_AGENT__LLM__ROLES__CRITIC": "codex_cli"})
    LlmGateway(cfg2, Observer.in_memory(), {"claude_code": FakeProvider({}, {"model": "a"}), "codex_cli": FakeProvider({}, {"model": "b"})})


def test_unknown_provider_fails_at_construction_not_at_first_call():
    with pytest.raises(ConfigError):
        LlmGateway(load_config(env={"TDD_AGENT__LLM__DEFAULT": "ghost"}), Observer.in_memory())


def test_budget_defaults_to_unlimited_and_enforces_when_configured():
    g, ob = gw()
    from tdd_agent.obs import bind
    with bind(run_id="r-unl"):
        assert all(g.call_json("extractor", f"p{i}", SCHEMA).ok for i in range(30))
    cfg = load_config(env={"TDD_AGENT__LLM__BUDGET__MAX_CALLS_PER_RUN": "2"})
    provider = FakeProvider({"*": {"answer": 1}})
    g2, ob2 = gw(cfg, {"claude_code": provider})
    with bind(run_id="r1"):
        assert g2.call_json("extractor", "a", SCHEMA).ok and g2.call_json("extractor", "b", SCHEMA).ok
        third = g2.call_json("extractor", "c", SCHEMA)
    assert third.status == "blocked" and third.blockers == ["BUDGET_EXCEEDED"] and len(provider.calls) == 2
    assert [s["status"] for s in ob2.store.list_spans()][-1] == "blocked"


def test_provider_failure_becomes_blocked_span_and_metric():
    g, ob = gw(providers={"claude_code": FakeProvider({"*": ProviderFailure("PROVIDER_RATE_LIMITED", retryable=False)})})
    r = g.call_json("extractor", "x", SCHEMA)
    assert r.blockers == ["PROVIDER_RATE_LIMITED"]
    assert ob.store.list_spans()[0]["blockers"] == ["PROVIDER_RATE_LIMITED"]
    assert any(m["labels"].get("status") == "blocked" for m in ob.store.list_metrics("llm_calls_total"))


def test_run_quota_per_day():
    from tdd_agent.store.base import RunRecord, now_iso
    cfg = load_config(env={"TDD_AGENT__LLM__BUDGET__MAX_RUNS_PER_DAY": "1"})
    g, ob = gw(cfg)
    assert g.check_run_quota() is None
    ts = now_iso()
    ob.store.create_run(RunRecord("r", "generation", "running", "intake", {}, {}, ts, ts, "t"))
    assert g.check_run_quota() == "BUDGET_EXCEEDED"


REAL_REPORT = ("OpenAI Codex v0.0.0\n--------\nworkdir: /tmp/x\nmodel: gpt-5.6-terra\nprovider: openai\napproval: never\nsandbox: read-only\n--------\nuser\nReturn it.\n\n"
               "warning: Code Mode is unavailable because failed to spawn code-mode host\ncodex\n{\"answer\":3}\ntokens used\n13,212\n")


def test_codex_ignores_the_users_own_configuration_by_default_and_reports_the_model_and_tokens_it_printed(tmp_path):
    p = codex(tmp_path, [{"output_file": '{"answer": 3}', "stderr": REAL_REPORT}], model="")
    r = p.complete(req())
    argv = calls(tmp_path)[0]["argv"]
    assert r.ok and "--ignore-user-config" in argv and "--ignore-rules" in argv and "--model" not in argv
    assert r.model == "gpt-5.6-terra" and r.usage == {"total_tokens": 13212} and r.evidence_grade == "real"
    assert p.describe()["model"] == "gpt-5.6-terra" and p.describe()["isolate"] is True
    kept = codex(tmp_path / "k", [{"output_file": '{"answer": 3}'}], isolate=False)
    kept.complete(req())
    assert "--ignore-user-config" not in calls(tmp_path / "k")[0]["argv"]


def test_a_model_the_account_cannot_use_is_named_instead_of_retried(tmp_path):
    msg = "ERROR: {\"detail\":\"The 'gpt-5.5-codex' model is not supported when using Codex with a ChatGPT account.\"}"
    r = codex(tmp_path, [{"stderr": msg, "exit": 1}]).complete(req())
    assert r.blockers == ["PROVIDER_MODEL_UNSUPPORTED"] and len(calls(tmp_path)) == 1 and "gpt-5.5-codex" in r.attempts[0]["message"]


def test_every_output_schema_of_a_skill_that_uses_the_model_satisfies_the_strict_rule_of_openai():
    """Codex passes the schema to a provider that refuses open objects: every object lists its properties, requires all of them and forbids others."""
    import importlib.util
    from tdd_agent.core import o5_path
    from tdd_agent.skills import PACKAGE_ROOT
    o5_path.o5_root()
    offenders = []

    def walk(node, where):
        if isinstance(node, dict):
            if node.get("type") == "object":
                props = node.get("properties")
                if props is None or node.get("additionalProperties") is not False or sorted(node.get("required", [])) != sorted(props):
                    offenders.append(where)
            for k, v in node.items():
                walk(v, f"{where}.{k}")
        elif isinstance(node, list):
            for i, v in enumerate(node):
                walk(v, f"{where}[{i}]")
    checked = []
    for path in sorted(PACKAGE_ROOT.glob("*/*/skill.py")):
        if path.parent.parent.name == "logic":
            continue
        spec = importlib.util.spec_from_file_location(f"strict_{path.parent.name.replace('-', '_')}", path)
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        schema = getattr(module, "SCHEMA", None) or (module.output_schema() if hasattr(module, "output_schema") else None)
        if schema is not None:
            checked.append(path.parent.name)
            walk(schema, path.parent.name)
    assert {"extract-facts-from-prose", "critique-spec", "author-spec-prose", "propose-srs-edits-from-cr", "parse-edit-intent"} <= set(checked)
    assert offenders == [], f"open or lax objects in the schema sent to the model: {offenders}"
