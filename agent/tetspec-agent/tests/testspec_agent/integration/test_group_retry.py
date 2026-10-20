"""Integration T1.2-T1.4: cache bypass on retry, group retry, 50 % failed-group threshold (TC-25..29, 31, 32)."""
import pytest

from testspec_agent.integration.test_analyze import good, pack_of
from tsagent.analyze import analyze
from tsagent.analyze.fill import run_groups
from tsagent.errors import LLMGroupsFailed, LLMPermanent, LLMTransient
from tsagent.ingest import ingest
from tsagent.llm import make_client

pytestmark = pytest.mark.integration
SCHEMA = {"type": "object", "required": ["items"], "properties": {"items": {"type": "array"}}}


@pytest.fixture
def pay(cfg, pay_docs):
    pkg, _ = ingest(pay_docs, "PAY", cfg)
    return pkg


def test_fresh_skips_cache_read_but_writes(cfg, fake_llm):
    fake_llm.script, fake_llm.sequence = [{"items": [1]}, {"items": [2]}, {"items": [3]}], True
    c = make_client(cfg)
    assert c.complete_json("analysis", "S", "U", SCHEMA) == {"items": [1]}
    assert c.complete_json("analysis", "S", "U", SCHEMA) == {"items": [1]}          # cache
    assert c.complete_json("analysis", "S", "U", SCHEMA, fresh=True) == {"items": [2]}
    assert c.complete_json("analysis", "S", "U", SCHEMA) == {"items": [2]}          # the new answer was written to the cache
    assert len(fake_llm.calls) == 2


def _flaky(fail_times, subjects=None):
    """Fake LLM: subjects (in `subjects`, default all) fail transiently for the first `fail_times` calls, then answer correctly."""
    seen = {}

    def script(system, user):
        subj = pack_of(user)["subject"]["code"]
        if subjects is None or subj in subjects:
            seen[subj] = seen.get(subj, 0) + 1
            if seen[subj] <= fail_times:
                return LLMTransient(f"quá tải {subj}")
        return good(system, user)
    return script, seen


@pytest.mark.parametrize("k", [1, 2])
def test_group_recovers_within_retries(cfg, fake_llm, pay, k):
    script, seen = _flaky(k)
    fake_llm.script = script
    g, fr, rep, _ = analyze(pay, "b", make_client(cfg), cfg)
    assert rep.passed and not fr.failed_groups and fr.filled
    assert set(fr.retried) == set(seen) and fr.retry_calls == k * len(seen)
    assert g.get("TCND-PAY-INT-01-idempotency-replay").prov.derivation == "llm"


def test_group_exhausting_retries_keeps_seed_with_gap(cfg, fake_llm, pay):
    script, _ = _flaky(3, subjects={"INT-01"})          # k = 3 > N = 2, a single subject
    fake_llm.script = script
    g, fr, rep, _ = analyze(pay, "b", make_client(cfg), cfg)
    assert rep.passed
    assert [x["subject"] for x in fr.failed_groups] == ["INT-01"]
    assert g.get("TCND-PAY-INT-01-idempotency-replay").prov.derivation != "llm"
    gaps = [g.get(x) for x in fr.gaps if g.get(x) is not None]
    assert any("sau retry" in (gp.attrs.get("missing") or "") for gp in gaps)


def test_more_than_half_failed_fails_step(cfg, fake_llm, pay):
    fake_llm.script = lambda s, u: LLMTransient("hỏng")
    with pytest.raises(LLMGroupsFailed, match=r"analysis: (\d+)/\1 groups failed after 2 retries") as e:
        analyze(pay, "b", make_client(cfg), cfg)
    assert e.value.code == "LLM_GROUPS_FAILED"


def test_half_or_less_failed_is_warning(cfg, fake_llm, pay):
    subjects = sorted({pack_of(c)["subject"]["code"] for c in _users(cfg, fake_llm, pay)})
    bad = set(subjects[: len(subjects) // 2])                    # exactly half (rounded down)
    script, _ = _flaky(99, subjects=bad)
    fake_llm.script = script
    _, fr, rep, _ = analyze(pay, "b", make_client(cfg), cfg)
    assert rep.passed and {x["subject"] for x in fr.failed_groups} == bad


def _users(cfg, fake_llm, pay):
    fake_llm.script = good
    analyze(pay, "b", make_client(cfg), cfg)
    users = [c["user"] for c in fake_llm.calls]
    fake_llm.calls.clear()
    cfg.cache_dir = cfg.cache_dir + "-2"                         # the real run does not hit the probe run's cache
    return users


@pytest.mark.parametrize("n", [0, 3])
def test_retry_count_read_per_run(cfg, fake_llm, pay, n):
    cfg.llm_group_retries = n
    fake_llm.script = lambda s, u: LLMTransient("hỏng")
    with pytest.raises(LLMGroupsFailed):
        analyze(pay, "b", make_client(cfg), cfg)
    subjects = {pack_of(c["user"])["subject"]["code"] for c in fake_llm.calls}
    assert len(fake_llm.calls) == len(subjects) * (n + 1)


def test_fatal_error_is_not_retried(cfg, fake_llm, pay):
    fake_llm.script = LLMPermanent("401", fatal=True)
    with pytest.raises(LLMPermanent):
        analyze(pay, "b", make_client(cfg), cfg)
    assert len(fake_llm.calls) == 1


def test_parallel_with_retry_matches_sequential(cfg, fake_llm, pay):
    def run(parallel, cache):
        script, _ = _flaky(1, subjects={"INT-01", "FR-001"})
        fake_llm.script = script
        cfg.llm_parallel, cfg.cache_dir = parallel, cache
        g, _, rep, _ = analyze(pay, "b", make_client(cfg), cfg)
        assert rep.passed
        return {c: e.content_hash() for c, e in g.entities.items()}
    base = cfg.cache_dir
    assert run(1, base + "-seq") == run(4, base + "-par")


def test_run_groups_keeps_best_attempt():
    tries = {}

    def call(job, fresh=False):
        tries[job] = tries.get(job, 0) + 1
        return ({"items": list(range(tries[job] if job == "a" else 1))}, "", 1)

    def grade(job, res):
        n = len(res[0]["items"])
        return ("ok" if job == "b" else "partial"), n

    class C:
        llm_group_retries, llm_parallel = 2, 1
    final = run_groups(call, ["a", "b"], C(), grade)
    assert [f["grade"] for f in final] == ["partial", "ok"]
    assert final[0]["items"] == 3 and final[0]["tries"] == 3 and final[1]["tries"] == 1
