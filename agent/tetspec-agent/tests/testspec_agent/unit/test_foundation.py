"""T1.1: configuration, errors, logging."""
import io
import json

import pytest

from tsagent.config import AutoApprovePolicy, Config, check_feature
from tsagent.errors import ConfigError, Diagnostic, InputError, InvalidFeature, LLMTransient
from tsagent.log import get_logger, mask_secrets, setup

pytestmark = pytest.mark.unit


def test_defaults(monkeypatch):
    for k in ("TSAGENT_LLM", "TSAGENT_LLM_STEPS", "TSAGENT_DB", "TSAGENT_AUTO_APPROVE"):
        monkeypatch.delenv(k, raising=False)
    c = Config()
    assert c.llm_provider == "" and c.llm_steps == ("analysis", "design", "specification")  # no default any more
    assert c.db.endswith("tsagent.db") and not c.auto_approve.enabled and not c.mcp_review
    assert not c.llm_enabled_for("analysis")


@pytest.mark.parametrize("name,value,msg", [
    ("TSAGENT_LLM", "gpt", "not supported"),
    ("TSAGENT_LLM_STEPS", "analysis,author", "author"),
    ("TSAGENT_LLM_CAP_ANALYSIS", "abc", "integer"),
    ("TSAGENT_MCP_REVIEW", "maybe", "1/0"),
    ("TSAGENT_AUTO_APPROVE", "classes", "syntax"),
])
def test_bad_values_are_rejected_with_clear_message(monkeypatch, name, value, msg):
    monkeypatch.setenv(name, value)
    with pytest.raises(ConfigError, match=msg):
        Config()


def test_auto_approve_policy():
    p = AutoApprovePolicy.parse("classes=TestStep,Assertion;min_confidence=0.9")
    assert p.enabled and p.classes == ("TestStep", "Assertion") and p.min_confidence == 0.9
    assert not AutoApprovePolicy.parse("").enabled


@pytest.mark.parametrize("bad", ["", "../x", "a/b", "a..b", "x" * 65, "-x", None])
def test_feature_code_rejects_traversal_and_bad_chars(bad):
    with pytest.raises(InvalidFeature):
        check_feature(bad)


def test_feature_code_accepts_valid():
    assert check_feature("SCHEDULED-TRANSFER") == "SCHEDULED-TRANSFER"
    assert check_feature("PAY.v2_1") == "PAY.v2_1"


def test_input_error_lists_positions():
    e = InputError([Diagnostic("a.json", "/edges/3", "I1", "cạnh treo"), Diagnostic("b.md", "12", "MD-REF", "x", "warning")])
    assert "a.json:/edges/3 [I1]" in str(e) and "b.md" not in str(e)


def test_llm_error_categories():
    e = LLMTransient("429")
    assert e.category == "transient" and e.retryable and not e.fatal


def test_log_is_json_with_context_and_masks_secrets():
    buf = io.StringIO()
    setup(stream=buf)
    get_logger(runId="run-1", step="analysis").info("gọi với key sk-ant-ABCDEFGHIJKLMNOP và Bearer abcdefghijk")
    rec = json.loads(buf.getvalue().strip())
    assert rec["runId"] == "run-1" and rec["step"] == "analysis"
    assert "sk-ant-ABC" not in rec["msg"] and "abcdefghijk" not in rec["msg"]


def test_mask_secrets_assignment_forms():
    assert "hunter22" not in mask_secrets("password=hunter22xyz")
    assert mask_secrets("không có gì") == "không có gì"
