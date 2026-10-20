"""MOD-Analyze: layer A (seed rules, context pack, LLM fill, gate GA)."""
from ..errors import ValidationFailed
from ..store import tso as store_tso
from ..validate.tso_rules import Ctx, validate
from .fill import fill
from .seed_core import seed


def analyze(pkg, baseline_id, client, cfg, run_id="", only=None, prev=None):
    """Seed -> LLM fill -> GA check. Returns (TsoGraph, FillResult, Report, Seeder). Writes nothing to the DB."""
    s = seed(pkg, baseline_id, run_id)
    expected = {e.code for e in s.g.of("TestCondition", "TestObligation")}
    fr = fill(client, s, cfg, run_id, only, prev)
    rep = validate(Ctx(s.g, pkg, baseline_id, expected_seed=expected), "GA")
    return s.g, fr, rep, s


def run_analysis(db, run_id, pkg, baseline_id, client, cfg, only=None, prev=None):
    """The `analysis` step of a run: the LLM runs outside the transaction; stage + call counts are written in one transaction."""
    g, fr, rep, _ = analyze(pkg, baseline_id, client, cfg, run_id, only, prev)
    if not rep.passed:
        raise ValidationFailed(rep.to_dict(), "analysis")
    use = client.usage().get("analysis", {})
    with db.step_tx(run_id, "analysis"):
        store_tso.stage_put(db, run_id, "analysis", g.entities.values())
        db.x("UPDATE run_step SET llm_calls = ?, cache_hits = ? WHERE run_id = ? AND step = 'analysis'",
             (use.get("calls", 0), use.get("cacheHits", 0), run_id))
    return {"graph": g, "fill": fr.to_dict(), "report": rep.to_dict()}


__all__ = ["analyze", "run_analysis", "seed", "fill"]
