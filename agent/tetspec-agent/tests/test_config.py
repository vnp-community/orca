import pytest

from tdd_agent.config import load_config
from tdd_agent.errors import ConfigError


def test_defaults_are_claude_code_sonnet_medium_and_unlimited_budget():
    cfg = load_config(env={})
    assert cfg.get("llm.default") == "claude_code"
    assert cfg.get("llm.claude_code.model") == "claude-sonnet-5-5"
    assert cfg.get("llm.claude_code.effort") == "medium"
    assert cfg.get("llm.budget") == {"max_calls_per_run": 0, "max_wall_seconds": 0, "max_runs_per_day": 0}
    assert cfg.get("obs.log_payloads") == "none" and cfg.get("mcp.write") is False and cfg.get("mcp.approvals") is False


def test_precedence_flag_over_env_over_file_over_default(tmp_path):
    f = tmp_path / "tdd-agent.toml"
    f.write_text('[llm]\ndefault = "codex_cli"\n[llm.claude_code]\neffort = "high"\n')
    assert load_config(f, env={}).get("llm.default") == "codex_cli"
    env = {"TDD_AGENT__LLM__DEFAULT": "claude_code"}
    cfg = load_config(f, env=env)
    assert cfg.get("llm.default") == "claude_code" and cfg.get("llm.claude_code.effort") == "high"
    cfg = load_config(f, overrides={"llm": {"default": "codex_cli"}}, env=env)
    assert cfg.get("llm.default") == "codex_cli"
    assert cfg.source["llm.default"] == "flag"


def test_env_types_and_budget_values():
    cfg = load_config(env={"TDD_AGENT__LLM__BUDGET__MAX_CALLS_PER_RUN": "25", "TDD_AGENT__MCP__WRITE": "true"})
    assert cfg.get("llm.budget.max_calls_per_run") == 25 and cfg.get("mcp.write") is True


@pytest.mark.parametrize("env", [
    {"TDD_AGENT__LLM__NOPE": "x"}, {"TDD_AGENT__NOSECTION__X": "x"}, {"TDD_AGENT__LLM__BUDGET__MAX_CALLS_PER_RUN": "-1"},
    {"TDD_AGENT__OBS__LOG_PAYLOADS": "everything"}, {"TDD_AGENT__LLM__CLAUDE_CODE__MAX_ATTEMPTS": "9"},
    {"TDD_AGENT__MCP__WRITE": "maybe"},
])
def test_invalid_configuration_fails_early(env):
    with pytest.raises(ConfigError):
        load_config(env=env)


def test_missing_explicit_file_and_unknown_key_in_file(tmp_path):
    with pytest.raises(ConfigError) as e:
        load_config(tmp_path / "missing.toml", env={})
    assert e.value.code == "CONFIG_FILE_MISSING"
    bad = tmp_path / "bad.toml"; bad.write_text("[llm]\nprovider = 'x'\n")
    with pytest.raises(ConfigError) as e:
        load_config(bad, env={})
    assert e.value.code == "CONFIG_UNKNOWN_KEY"


def test_approver_allowlist_validation(tmp_path):
    ok = tmp_path / "ok.toml"
    ok.write_text('[[approvals.actors]]\nid="alice"\nkind="human"\nroles=["BA","SA"]\n[[approvals.actors]]\nid="claude"\nkind="coding_agent"\nroles=["SA/Dev"]\n')
    cfg = load_config(ok, env={})
    assert [a.id for a in cfg.actors()] == ["alice", "claude"]
    for body in ['[[approvals.actors]]\nid="x"\nkind="robot"\nroles=["BA"]\n',
                 '[[approvals.actors]]\nid="x"\nkind="human"\nroles=["CEO"]\n',
                 '[[approvals.actors]]\nid="x"\nkind="human"\nroles=["BA"]\n[[approvals.actors]]\nid="x"\nkind="human"\nroles=["BA"]\n',
                 '[mcp]\napprovals=true\n']:
        f = tmp_path / "b.toml"; f.write_text(body)
        with pytest.raises(ConfigError):
            load_config(f, env={})


def test_secrets_are_masked_in_redacted_view(tmp_path):
    f = tmp_path / "t.toml"; f.write_text('[mcp]\ntoken_env="TDD_AGENT_MCP_TOKEN"\n')
    red = load_config(f, env={}).redacted()
    assert red["mcp"]["token_env"] == "TDD_AGENT_MCP_TOKEN"


def test_store_url_resolves_relative_paths(tmp_path):
    f = tmp_path / "tdd-agent.toml"; f.write_text("")
    cfg = load_config(f, env={})
    assert cfg.store_url("knowledge") == f"sqlite:{tmp_path.resolve()}/.tdd-agent/knowledge.db"
    assert load_config(env={"TDD_AGENT__STORE__KNOWLEDGE": "memory:"}).store_url("knowledge") == "memory:"
