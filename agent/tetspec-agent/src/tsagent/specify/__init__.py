"""MOD-Specify: layer C (specification rules, LLM fill, GS check without review)."""
from ..errors import ValidationFailed
from ..store import tso as store_tso
from ..validate.tso_rules import GATES, Ctx, validate
from .fill import SpecFiller
from .rules import C_CLASSES, specify

PRE_REVIEW = [r for r in GATES["GS"] if r not in ("TSO-007", "TSO-020", "TSO-021")]


def run_specify_step(g, pkg, baseline_id, client, cfg, run_id="", expected_seed=None):
    sp = specify(g, pkg, run_id)
    fr = SpecFiller(client, g, pkg, cfg, run_id).run()
    rep = validate(Ctx(g, pkg, baseline_id, expected_seed=expected_seed), "GB", PRE_REVIEW)
    return sp, fr, rep


def run_specify(db, run_id, g, pkg, baseline_id, client, cfg, expected_seed=None):
    sp, fr, rep = run_specify_step(g, pkg, baseline_id, client, cfg, run_id, expected_seed)
    if not rep.passed:
        raise ValidationFailed(rep.to_dict(), "specification")
    use = client.usage().get("specification", {})
    with db.step_tx(run_id, "specification"):
        store_tso.stage_put(db, run_id, "specification", [e for e in g.entities.values() if e.cls in C_CLASSES])
        db.x("UPDATE run_step SET llm_calls = ?, cache_hits = ? WHERE run_id = ? AND step = 'specification'",
             (use.get("calls", 0), use.get("cacheHits", 0), run_id))
    return {"fill": fr.to_dict(), "report": rep.to_dict()}


__all__ = ["specify", "run_specify", "run_specify_step", "C_CLASSES", "PRE_REVIEW"]
