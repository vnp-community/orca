"""T4.1, T4.3: tsagent configuration from [testspec], LLM checks, the detached child-run command."""
from pathlib import Path

import pytest

from tdd_agent.agents.testspec import gate, runner
from tdd_agent.config import load_config
from tdd_agent.errors import TddAgentError


def cfg_for(tmp_path, **ts):
    return load_config(overrides={"paths": {"work_dir": str(tmp_path / "w"), "repo_root": str(tmp_path / "r")}, "testspec": ts}, env={})


def test_tsagent_config_comes_from_testspec_section(tmp_path, monkeypatch):
    monkeypatch.setenv("TSAGENT_LLM", "openai")             # ignored in the workflow
    monkeypatch.setenv("TSAGENT_LLM_CAP_ANALYSIS", "7")      # other TSAGENT_* variables still apply
    c = runner.tsagent_config(cfg_for(tmp_path, llm="none", parallel=3, group_retries=5, specs_dir="out/ts"))
    assert (c.llm_provider, c.llm_parallel, c.llm_group_retries, c.llm_caps["analysis"]) == ("none", 3, 5, 7)
    assert c.db == str(tmp_path / "w" / "tsagent.db") and c.specs_root == str(tmp_path / "r" / "out" / "ts")
    assert c.source_roots == (str(tmp_path / "w" / "runs"),) and c.cache_dir.startswith(str(tmp_path / "w" / "tsagent"))


@pytest.mark.parametrize("llm,code", [("", "LLM_NOT_CONFIGURED"), ("openai", "LLM_KEY_MISSING")])
def test_preflight_errors_carry_tsagent_code(tmp_path, monkeypatch, llm, code):
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)
    with pytest.raises(TddAgentError) as e:
        runner.preflight(cfg_for(tmp_path, llm=llm))
    assert e.value.code == code
    if code == "LLM_NOT_CONFIGURED":
        assert "[testspec] llm in .tdd-agent/config.toml" in str(e.value)
    assert runner.preflight(cfg_for(tmp_path, llm="none")) == {"llm": "none"}


def test_launch_detached_command(tmp_path, monkeypatch):
    seen = {}

    class P:
        pid = 99

    def popen(cmd, **kw):
        seen.update(cmd=cmd, **kw)
        return P()
    monkeypatch.setattr(gate.subprocess, "Popen", popen)
    cfg = cfg_for(tmp_path, llm="none")
    assert gate.launch_detached(cfg, "run-abc") == 99
    assert seen["cmd"][1:] == ["-m", "tdd_agent.cli", "-C", str(cfg.base_dir), "run", "advance", "run-abc"]
    assert seen["start_new_session"] is True
    assert Path(tmp_path / "w" / "logs" / "testspec-run-abc.log").exists()
