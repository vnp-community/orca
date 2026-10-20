"""T2.1: the [testspec] configuration section and its exclusion from the fingerprint."""
import pytest

from tdd_agent.config import load_config
from tdd_agent.errors import ConfigError


def test_defaults_have_no_llm_mode():
    ts = load_config(env={}).section("testspec")
    assert ts == {"enabled": True, "llm": "", "parallel": 4, "group_retries": 2, "specs_dir": "docs/testspec", "kg_mcp": "",
                  "kg_mcp_token_env": "TDD_AGENT_MCP_TOKEN", "context_mode": "none", "context_path": ""}


def test_file_and_env_override(tmp_path):
    f = tmp_path / "tdd-agent.toml"
    f.write_text('[testspec]\nllm = "claude-cli"\nparallel = 2\n')
    cfg = load_config(f, env={"TDD_AGENT__TESTSPEC__GROUP_RETRIES": "5", "TDD_AGENT__TESTSPEC__ENABLED": "false"})
    ts = cfg.section("testspec")
    assert ts["llm"] == "claude-cli" and ts["parallel"] == 2 and ts["group_retries"] == 5 and ts["enabled"] is False


@pytest.mark.parametrize("patch", [
    {"llm": "gpt"}, {"parallel": 0}, {"group_retries": -1}, {"specs_dir": "/abs"}, {"specs_dir": "../out"}, {"specs_dir": ""},
    {"context_mode": "both"}, {"context_path": "/abs"}, {"context_path": "../x"},
])
def test_invalid_values_rejected(patch):
    with pytest.raises(ConfigError) as e:
        load_config(env={}, overrides={"testspec": patch})
    assert e.value.code == "CONFIG_INVALID"


def test_unknown_key_rejected():
    with pytest.raises(ConfigError) as e:
        load_config(env={}, overrides={"testspec": {"model": "x"}})
    assert e.value.code == "CONFIG_UNKNOWN_KEY"


def test_testspec_section_not_in_fingerprint():
    base = load_config(env={}).fingerprint()
    assert load_config(env={}, overrides={"testspec": {"group_retries": 7, "llm": "none", "parallel": 1}}).fingerprint() == base
    assert load_config(env={}, overrides={"render": {"language": "en"}}).fingerprint() != base
