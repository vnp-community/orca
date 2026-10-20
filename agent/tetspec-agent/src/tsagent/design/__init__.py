"""MOD-Design: layer B (design rules, LLM fill, gate GB)."""
from ..errors import ValidationFailed
from ..store import tso as store_tso
from ..validate.tso_rules import Ctx, validate
from .fill import DesignFiller
from .rules import B_CLASSES, design


def run_design_step(g, pkg, baseline_id, client, cfg, run_id="", expected_seed=None):
    """Add layer B to `g` (which already has layer A), LLM fill, GB check. Returns (Designer, StepResult, Report). Writes nothing to the DB."""
    d = design(g, pkg, run_id)
    fr = DesignFiller(client, g, pkg, cfg, run_id).run()
    rep = validate(Ctx(g, pkg, baseline_id, expected_seed=expected_seed), "GB")
    return d, fr, rep


def run_design(db, run_id, g, pkg, baseline_id, client, cfg, expected_seed=None):
    d, fr, rep = run_design_step(g, pkg, baseline_id, client, cfg, run_id, expected_seed)
    if not rep.passed:
        raise ValidationFailed(rep.to_dict(), "design")
    use = client.usage().get("design", {})
    with db.step_tx(run_id, "design"):
        store_tso.stage_put(db, run_id, "design", [e for e in g.entities.values() if e.cls in B_CLASSES])
        db.x("UPDATE run_step SET llm_calls = ?, cache_hits = ? WHERE run_id = ? AND step = 'design'",
             (use.get("calls", 0), use.get("cacheHits", 0), run_id))
    return {"fill": fr.to_dict(), "report": rep.to_dict()}


__all__ = ["design", "run_design", "run_design_step", "B_CLASSES"]
