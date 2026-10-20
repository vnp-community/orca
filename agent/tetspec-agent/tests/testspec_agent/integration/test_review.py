"""T7.2: review by class and derivation; auto-approval policy; baseline blocked by unreviewed llm nodes or open gaps (TST-14)."""
import pytest

from testspec_agent.helpers.fake_answers import answer
from tsagent.config import AutoApprovePolicy
from tsagent.errors import ValidationFailed
from tsagent.orchestrator import Orchestrator
from tsagent.orchestrator.review import baseline_spec, pending_review, pending_summary, review

pytestmark = pytest.mark.integration


def with_proposal(system, user):
    out = answer(system, user)
    if out.get("subject") == "INT-01":
        out["proposedConditions"] = [{"conditionKind": "concurrency", "statement": "Hai yêu cầu đồng thời", "expectedEffect": "một hiệu ứng",
                                      "oracle": {"oracleKind": "invariant", "expected": "một hiệu ứng", "sourceCode": "INT-01"},
                                      "basedOn": ["INT-01"], "evidence": [{"sourceCode": "INT-01", "quote": "x"}], "confidence": 0.4}]
    return out


@pytest.fixture
def draft(cfg, tmp_db, fake_llm, pay_docs):
    fake_llm.script = with_proposal
    st = Orchestrator(tmp_db, cfg).start("PAY", pay_docs)
    assert st["state"] == "done", st
    return tmp_db


def test_pending_ordering_and_filters(draft):
    items = pending_review(draft, "PAY")
    assert items[0]["origin"] == "llmProposal"
    first_rule = next(i for i, x in enumerate(items) if x["derivation"] == "rule")
    assert all(x["derivation"] == "llm" for x in items[:first_rule])
    only = pending_review(draft, "PAY", classes=["TestCase"], derivation="llm")
    assert only and all(x["class"] == "TestCase" and x["derivation"] == "llm" for x in only)
    s = pending_summary(draft, "PAY")
    assert s["total"] == len(items) and s["inferred"] and s["byClass"]["TestCase"] > 0


def test_baseline_blocked_until_llm_nodes_reviewed(cfg, draft):
    review(draft, "PAY", "approved", "qa", derivation="rule")
    review(draft, "PAY", "approved", "qa", classes=["TestGap"])
    with pytest.raises(ValidationFailed, match="TSO-020"):
        baseline_spec(draft, cfg, "PAY", "qa")
    review(draft, "PAY", "approved", "qa", derivation="llm")
    assert baseline_spec(draft, cfg, "PAY", "qa")["version"] == 1


def test_open_gap_blocks_baseline(cfg, draft):
    review(draft, "PAY", "approved", "qa")
    from tsagent.store import tso as store_tso
    g = store_tso.load_spec(draft, "PAY")
    gap = next(x for x in g.of("TestGap") if x.rel("blocks"))
    review(draft, "PAY", "needsInfo", "qa", codes=[gap.code])
    with pytest.raises(ValidationFailed, match="TSO-007"):
        baseline_spec(draft, cfg, "PAY", "qa")


def test_auto_approve_policy(cfg, draft):
    review(draft, "PAY", "approved", "qa", derivation="rule")
    review(draft, "PAY", "approved", "qa", classes=["TestGap"])
    llm_classes = sorted({x["class"] for x in pending_review(draft, "PAY", derivation="llm")})
    cfg.auto_approve = AutoApprovePolicy(tuple(llm_classes), 0.5)  # a proposal with confidence 0.4 is below the threshold
    with pytest.raises(ValidationFailed, match="TSO-020"):
        baseline_spec(draft, cfg, "PAY", "qa")
    left = pending_review(draft, "PAY", derivation="llm")
    assert left and all(x["confidence"] < 0.5 for x in left)
    row = draft.one("SELECT reviewer FROM tso_lifecycle WHERE reviewer LIKE 'auto:%' LIMIT 1")
    assert row["reviewer"].startswith("auto:classes=")
    review(draft, "PAY", "rejected", "qa", codes=[x["code"] for x in left])
    cfg.auto_approve = AutoApprovePolicy(tuple(llm_classes), 0.5)
    with pytest.raises(ValidationFailed):
        baseline_spec(draft, cfg, "PAY", "qa")  # rejected nodes are still in the Draft: TSO-020 blocks


def test_needs_info_and_reject_by_code(draft):
    code = pending_review(draft, "PAY", classes=["TestCase"])[0]["code"]
    assert review(draft, "PAY", "needsInfo", "qa", codes=[code], note="thiếu dữ liệu")["updated"] == 1
    assert next(x for x in pending_review(draft, "PAY") if x["code"] == code)["reviewStatus"] == "needsInfo"
    assert review(draft, "PAY", "rejected", "qa", codes=[code])["updated"] == 1
    assert code not in {x["code"] for x in pending_review(draft, "PAY")}
