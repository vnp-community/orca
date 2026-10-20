"""Step orchestrator (T7.1, FR-012, MECH-01, MECH-02, SCN-01, SCN-02, SCN-04).

Steps: ingest -> analysis -> design -> specification -> validate -> commit -> render. The LLM runs outside transactions; each step writes
the run's stage in one short transaction (`run_step` records the status). A run with the same `runKey` resumes after the last finished step;
`from_step` reruns from a step; `dry_run` runs everything except commit and render. CR flow: the LLM is called only for subjects in the
affected set, the previous version's LLM content is reused for the rest; regression picks baselined TCs of the affected part that did not change.
"""
import json
import uuid

from .. import analyze, design, specify
from ..config import check_feature
from ..delta import impact, prepare_change
from ..errors import NotFound, TsagentError, ValidationFailed
from ..ingest import ingest
from ..llm import make_client
from ..log import get_logger
from ..model import stable_hash
from ..ontology import ontology
from ..render import render
from ..store import source
from ..store import tso as store_tso
from ..store.db import now
from ..validate.tso_rules import Ctx, validate

log = get_logger("tsagent.orchestrator")
STEPS = ("ingest", "analysis", "design", "specification", "validate", "commit", "render")
STATE = {"ingest": "Ingested", "analysis": "Analyzed", "design": "Designed", "specification": "Specified",
         "validate": "Validated", "commit": "Committed", "render": "Rendered"}


def run_key(feature, pkg_hash, cfg, model):
    return stable_hash({"feature": feature, "pkg": pkg_hash, "tso": ontology().yaml_hash, "llm": cfg.llm_provider,
                        "steps": list(cfg.llm_steps), "model": model, "caps": cfg.llm_caps})[:24]


class Orchestrator:
    def __init__(self, db, cfg, client=None):
        self.db, self.cfg = db, cfg
        self.client = client if client is not None else make_client(cfg)
        self.model = getattr(getattr(self.client, "provider", None), "tag", "") or "none"

    # ------------------------------------------------------------------ run table
    def _run(self, run_id):
        r = self.db.one("SELECT * FROM run WHERE run_id = ?", (run_id,))
        if r is None:
            raise NotFound(f"no run {run_id}")
        d = dict(r)
        for k in ("params", "notes", "result"):
            d[k] = json.loads(d[k])
        return d

    def _update(self, run_id, **kw):
        sets, vals = [], []
        for k, v in kw.items():
            sets.append(f"{k} = ?")
            vals.append(json.dumps(v, ensure_ascii=False, sort_keys=True) if k in ("params", "notes", "result") else v)
        with self.db.tx():
            self.db.x(f"UPDATE run SET {', '.join(sets)}, updated_at = ? WHERE run_id = ?", (*vals, now(), run_id))

    def done_steps(self, run_id):
        return {r["step"] for r in self.db.q("SELECT step FROM run_step WHERE run_id = ? AND status = 'done'", (run_id,))}

    def status(self, run_id):
        r = self._run(run_id)
        steps = sorted((dict(x) for x in self.db.q("SELECT step, status, llm_calls, cache_hits, error FROM run_step WHERE run_id = ?",
                                                   (run_id,))), key=lambda s: STEPS.index(s["step"]) if s["step"] in STEPS else 99)
        state = "failed" if r["status"] == "Failed" else "done" if r["result"].get("finished") else "running"
        err = next((s["error"] for s in steps if s["status"] == "failed"), None)
        if err is None and r["status"] == "Failed" and r["notes"]:
            err = r["notes"][-1]  # gate error before the step opened its transaction: only in notes
        return {"runId": run_id, "feature": r["feature"], "state": state, "status": r["status"], "steps": steps,
                "error": err, "result": r["result"], "notes": r["notes"], "runKey": r["run_key"]}

    # ------------------------------------------------------------------ running
    def prepare(self, feature, documents, dry_run=False, force=False, from_step=None, steps=None, origin=None):
        """Load and check the input (no writes), find or create the run. Returns (run_id, pkg, deduplicated).
        `origin`: provenance recorded by the caller (e.g. the tdd-agent design-KG snapshot); stored in run.params, not part of runKey."""
        check_feature(feature)
        pkg, diags = ingest(documents, feature, self.cfg)
        key = run_key(feature, pkg.content_hash(), self.cfg, self.model) + ("-dry" if dry_run else "")
        last = self.db.one("SELECT run_id, status, result FROM run WHERE run_key = ? ORDER BY created_at DESC, rowid DESC LIMIT 1", (key,))
        if last is not None and not force:
            res = json.loads(last["result"])
            if res.get("finished") and not from_step:
                return last["run_id"], pkg, True
            return last["run_id"], pkg, False       # resume an unfinished run, or rerun from `from_step`
        run_id = "run-" + uuid.uuid4().hex[:12]
        params = {"dryRun": dry_run, "documents": [d if isinstance(d, str) else {"name": d.get("name")} for d in documents or []],
                  "steps": list(steps or STEPS), "warnings": [str(x) for x in diags if x.severity != "error"][:50]}
        if origin:
            params["origin"] = origin
        with self.db.tx():
            self.db.x("INSERT INTO run(run_id, feature, run_key, mode, model, status, params, created_at, updated_at) "
                      "VALUES (?, ?, ?, ?, ?, 'Created', ?, ?, ?)",
                      (run_id, feature, key, self.cfg.llm_provider, self.model, json.dumps(params, ensure_ascii=False), now(), now()))
        return run_id, pkg, False

    def start(self, feature, documents, dry_run=False, force=False, from_step=None, steps=None, origin=None):
        run_id, pkg, dedup = self.prepare(feature, documents, dry_run, force, from_step, steps, origin)
        if dedup:
            out = self.status(run_id)
            out["deduplicated"] = True
            return out
        return self.execute(run_id, pkg, from_step=from_step)

    def execute(self, run_id, pkg=None, from_step=None):
        r = self._run(run_id)
        feature, params = r["feature"], r["params"]
        wanted = [s for s in STEPS if s in params.get("steps", STEPS)]
        if params.get("dryRun"):
            wanted = [s for s in wanted if s not in ("commit", "render")]
        done = self.done_steps(run_id)
        if from_step:
            if from_step not in STEPS:
                raise TsagentError(f"invalid step: {from_step}")
            redo = STEPS[STEPS.index(from_step):]
            with self.db.tx():
                self.db.x(f"DELETE FROM run_step WHERE run_id = ? AND step IN ({','.join('?' * len(redo))})", (run_id, *redo))
                self.db.x(f"DELETE FROM stage_item WHERE run_id = ? AND step IN ({','.join('?' * len(redo))})", (run_id, *redo))
            done -= set(redo)
            self._update(run_id, result={})
            self._restage(run_id, feature, [x for x in ("analysis", "design", "specification") if x not in redo])
        ctx = {"pkg": pkg, **params.get("ctx", {})}
        try:
            for step in wanted:
                if step in done:
                    continue
                getattr(self, "step_" + step)(run_id, feature, ctx)
                self._update(run_id, status=STATE[step])
            res = self._run(run_id)["result"]
            res["finished"] = True
            self._update(run_id, result=res)
        except Exception as e:
            log.warning("run %s failed: %s", run_id, e)
            r2 = self._run(run_id)
            res = r2["result"]
            if isinstance(e, ValidationFailed):
                res["report"] = e.report
            self._update(run_id, status="Failed", notes=r2["notes"] + [f"{type(e).__name__}: {e}"[:500]], result=res)
            if not isinstance(e, TsagentError):
                raise  # programming error: never swallowed
        return self.status(run_id)

    def _restage(self, run_id, feature, steps):
        """Steps before `from_step` were committed (stage deleted): reload the stage from the latest version, by layer."""
        layers = {"analysis": "A", "design": "B", "specification": "C"}
        missing = [s for s in steps if not self.db.one("SELECT 1 FROM stage_item WHERE run_id = ? AND step = ?", (run_id, s))]
        if not missing or not store_tso.versions(self.db, feature):
            return
        g = store_tso.load_spec(self.db, feature)
        o = ontology()
        with self.db.tx():
            for s in missing:
                store_tso.stage_put(self.db, run_id, s, [e for e in g.entities.values() if o.cls(e.cls).layer == layers[s]])

    def _ctx_save(self, run_id, **kw):
        r = self._run(run_id)
        params = r["params"]
        params.setdefault("ctx", {}).update(kw)
        self._update(run_id, params=params)

    def _pkg(self, ctx, feature):
        if ctx.get("pkg") is None or ctx["pkg"].baseline_id != ctx.get("baseline"):
            ctx["pkg"] = source.load_source(self.db, feature, ctx["baseline"])
        return ctx["pkg"]

    def _prev_graph(self, feature):
        vs = store_tso.versions(self.db, feature)
        return store_tso.load_spec(self.db, feature) if vs else None

    # ------------------------------------------------------------------ steps
    def step_ingest(self, run_id, feature, ctx):
        if ctx.get("pkg") is None:
            raise TsagentError("the ingest step needs the source documents (rerun with the same documents)")
        with self.db.step_tx(run_id, "ingest"):
            ch = prepare_change(self.db, ctx["pkg"])
        prev = ch["previous"]
        affected = sorted(impact(ch["source"], prev, ch["delta"].changed)) if prev is not None else None
        ctx.update(baseline=ch["baseline"], crs=ch["crs"], affected=affected, prevBaseline=ch["prev"])
        ctx["pkg"] = ch["source"]
        self._ctx_save(run_id, baseline=ch["baseline"], crs=ch["crs"], affected=affected, prevBaseline=ch["prev"],
                       delta=ch["delta"].to_dict())
        with self.db.tx():
            self.db.x("UPDATE run SET base_baseline_id = ? WHERE run_id = ?", (ch["baseline"], run_id))

    def step_analysis(self, run_id, feature, ctx):
        pkg = self._pkg(ctx, feature)
        prev = self._prev_graph(feature)
        only = set(ctx["affected"]) if prev is not None and ctx.get("affected") is not None else None
        res = analyze.run_analysis(self.db, run_id, pkg, ctx["baseline"], self.client, self.cfg, only=only, prev=prev)
        self._note(run_id, res["fill"], "analysis")

    def step_design(self, run_id, feature, ctx):
        pkg = self._pkg(ctx, feature)
        g = store_tso.stage_load(self.db, run_id, feature, ["analysis"])
        res = design.run_design(self.db, run_id, g, pkg, ctx["baseline"], self.client, self.cfg)
        self._note(run_id, res["fill"], "design")

    def step_specification(self, run_id, feature, ctx):
        pkg = self._pkg(ctx, feature)
        g = store_tso.stage_load(self.db, run_id, feature, ["analysis", "design"])
        res = specify.run_specify(self.db, run_id, g, pkg, ctx["baseline"], self.client, self.cfg)
        self._note(run_id, res["fill"], "specification")

    def step_validate(self, run_id, feature, ctx):
        pkg = self._pkg(ctx, feature)
        g = store_tso.stage_load(self.db, run_id, feature, ["analysis", "design", "specification"])
        rep = validate(Ctx(g, pkg, ctx["baseline"]), "GB", specify.PRE_REVIEW + ["TSO-007"])
        with self.db.step_tx(run_id, "validate"):
            res = self._run(run_id)["result"]
            res["report"] = rep.to_dict()
            res["counts"] = rep.counts
            self.db.x("UPDATE run SET result = ? WHERE run_id = ?", (json.dumps(res, ensure_ascii=False, sort_keys=True), run_id))
        if not rep.passed:
            raise ValidationFailed(rep.to_dict(), "validate")

    def regression(self, feature, g, affected):
        """Baselined TCs in the affected part (by the source subject of their conditions) whose content did not change."""
        rows = [r for r in store_tso.versions(self.db, feature) if r["status"] == "Baselined"]
        if not rows or not affected:
            return []
        old = store_tso.load_spec(self.db, feature, rows[-1]["version"])
        aff = set(affected)
        out = []
        for tc in g.of("TestCase"):
            o = old.get(tc.code)
            if o is None or o.content_hash() != tc.content_hash():
                continue
            cov = g.get(tc.rel("covers")[0]) if tc.rel("covers") else None
            cond = g.get(cov.rel("realizes")[0]) if cov is not None else None
            if cond is not None and set(cond.prov.based_on) & aff:
                out.append(tc.code)
        return sorted(out)

    def step_commit(self, run_id, feature, ctx):
        g = store_tso.stage_load(self.db, run_id, feature)
        reg = self.regression(feature, g, ctx.get("affected"))
        with self.db.step_tx(run_id, "commit"):
            stats = store_tso.commit(self.db, run_id, g, ctx["baseline"], changed_by=ctx.get("crs") or (), regression=reg)
            res = self._run(run_id)["result"]
            res.update(version=stats["version"], new=len(stats["new"]), modified=len(stats["modified"]),
                       unchanged=stats["unchanged"], retiring=stats["retiring"], regression=reg)
            self.db.x("UPDATE run SET result = ? WHERE run_id = ?", (json.dumps(res, ensure_ascii=False, sort_keys=True), run_id))

    def step_render(self, run_id, feature, ctx):
        with self.db.step_tx(run_id, "render"):
            out = render(self.db, self.cfg, feature, force=bool(ctx.get("forceRender")))
            res = self._run(run_id)["result"]
            res["files"] = out["files"]
            self.db.x("UPDATE run SET result = ? WHERE run_id = ?", (json.dumps(res, ensure_ascii=False, sort_keys=True), run_id))

    def _note(self, run_id, fill, step):
        notes = self._run(run_id)["notes"]
        if fill.get("calls"):
            notes.append(f"{step}: {fill['calls']} LLM calls, {len(fill.get('filled', []))} filled")
        for x in fill.get("notes", []):
            notes.append(f"{step}: {x}")
        if fill.get("skipped"):
            notes.append(f"{step}: {len(fill['skipped'])} groups kept rule template content (LLM error/cap reached)")
        if fill.get("rejected"):
            notes.append(f"{step}: {len(fill['rejected'])} groups had LLM items dropped")
        if fill.get("retried"):
            notes.append(f"{step}: {len(fill['retried'])} groups retried ({fill.get('retry_calls', 0)} retries)")
        failed = fill.get("failed_groups") or []
        if failed:
            res = self._run(run_id)["result"]
            res.setdefault("warnings", []).extend(
                {"step": step, "group": x.get("subject") or x.get("group"), "reason": x.get("reason")} for x in failed)
            notes.append(f"{step}: {len(failed)} groups still failed after retries, rule template content kept")
            self._update(run_id, notes=notes, result=res)
            return
        self._update(run_id, notes=notes)


def run_many(db, cfg, items, client=None):
    """Several features in sequence; one feature's error does not block the others. items: [(feature, documents)]."""
    orch = Orchestrator(db, cfg, client)
    out = {}
    for feature, docs in items:
        try:
            out[feature] = orch.start(feature, docs)
        except Exception as e:  # isolate errors per feature
            out[feature] = {"state": "failed", "error": f"{type(e).__name__}: {e}"}
    return out
