"""Integration T1.1: mandatory LLM mode, key/model checks (TC-21..24)."""
import pytest

from testspec_agent.conftest import make_cfg
from tsagent.config import Config
from tsagent.errors import ConfigError
from tsagent.llm import LLMKeyMissing, LLMModelMissing, LLMNotConfigured, check_llm, make_provider

pytestmark = pytest.mark.unit


def test_config_builds_without_llm_mode(monkeypatch):
    monkeypatch.delenv("TSAGENT_LLM", raising=False)
    c = Config()
    assert c.llm_provider == "" and not c.llm_enabled_for("analysis")


def test_unknown_mode_still_rejected(monkeypatch):
    monkeypatch.setenv("TSAGENT_LLM", "gpt")
    with pytest.raises(ConfigError, match="not supported"):
        Config()


def test_orchestrator_refuses_unset_mode(tmp_path, tmp_db):
    from tsagent.orchestrator import Orchestrator
    cfg = make_cfg(tmp_path, llm_provider="")
    with pytest.raises(LLMNotConfigured) as e:
        Orchestrator(tmp_db, cfg)
    assert e.value.code == "LLM_NOT_CONFIGURED"
    for mode in ("claude-cli", "anthropic", "openai", "none"):
        assert mode in str(e.value)


@pytest.mark.parametrize("mode,key,model_attr,model_env", [
    ("anthropic", "ANTHROPIC_API_KEY", "anthropic_model", "ANTHROPIC_MODEL"),
    ("openai", "OPENAI_API_KEY", "openai_model", "OPENAI_MODEL"),
])
def test_api_provider_needs_key_and_model(tmp_path, monkeypatch, mode, key, model_attr, model_env):
    monkeypatch.delenv(key, raising=False)
    cfg = make_cfg(tmp_path, llm_provider=mode, **{model_attr: "m-1"})
    with pytest.raises(LLMKeyMissing, match=key) as e:
        check_llm(cfg)
    assert e.value.code == "LLM_KEY_MISSING"
    monkeypatch.setenv(key, "sk-test")
    setattr(cfg, model_attr, "")
    with pytest.raises(LLMModelMissing, match=model_env) as e:
        make_provider(cfg)
    assert e.value.code == "LLM_MODEL_MISSING"
    setattr(cfg, model_attr, "m-1")
    assert check_llm(cfg) == {"llm": mode}
    assert make_provider(cfg) is not None


def test_claude_cli_and_none_unchanged(tmp_path):
    assert check_llm(make_cfg(tmp_path, llm_provider="claude-cli")) == {"llm": "claude-cli"}
    assert make_provider(make_cfg(tmp_path, llm_provider="claude-cli")) is not None
    assert make_provider(make_cfg(tmp_path, llm_provider="none")) is None


def test_cli_gen_without_mode_exits_2(tmp_path, monkeypatch, capsys):
    from tsagent.cli import main
    monkeypatch.delenv("TSAGENT_LLM", raising=False)
    monkeypatch.setenv("TSAGENT_DB", str(tmp_path / "t.db"))
    monkeypatch.setenv("TSAGENT_SOURCE_ROOTS", str(tmp_path))
    src = tmp_path / "x.md"
    src.write_text("# x\n", encoding="utf-8")
    assert main(["gen", "--feature", "F1", "--src", str(src)]) == 2
    err = capsys.readouterr().err
    assert "LLM_NOT_CONFIGURED" in err and "claude-cli" in err
