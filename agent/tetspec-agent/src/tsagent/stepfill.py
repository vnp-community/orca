"""Common frame of the LLM fill steps of layers B and C: packs per group, `complete_json`, grounding check, one repair turn, per-item filtering.

Failed groups (LLM error, breaker open, ungrounded output after the repair turn) are retried in rounds, at most `llm_group_retries` times.
When retries or the cap run out: keep the rule template content and record `skipped`/`failed_groups` (MECH-11). A `fatal` error stops the run.
More than half of the groups of the step still failing after retries: the run fails (`LLMGroupsFailed`).
"""
import json
import os
from dataclasses import dataclass, field

from .analyze.fill import check_failed_ratio, n_items, prompt_hash, record_retry, run_groups
from .errors import BudgetExceeded, LLMDisabled
from .llm import wrap_data
from .log import get_logger
from .model import Evidence

log = get_logger("tsagent.stepfill")


@dataclass
class StepResult:
    calls: int = 0
    filled: list = field(default_factory=list)
    rejected: list = field(default_factory=list)
    skipped: list = field(default_factory=list)
    notes: list = field(default_factory=list)
    failed_groups: list = field(default_factory=list)
    retried: list = field(default_factory=list)
    retry_calls: int = 0

    def to_dict(self):
        return dict(self.__dict__)


class StepFiller:
    step = ""
    schema = ""
    prompt_dir = ""

    def __init__(self, client, g, pkg, cfg, run_id=""):
        self.client, self.g, self.pkg, self.cfg, self.run_id = client, g, pkg, cfg, run_id
        self.res = StepResult()
        with open(os.path.join(self.prompt_dir, f"{self.step}.md"), encoding="utf-8") as fh:
            self.system = fh.read()
        self.model = getattr(getattr(client, "provider", None), "tag", "") or ""

    # implemented by subclasses
    def groups(self):  # pragma: no cover
        """Yield (group key, context pack dict with `allowedCodes`, set of valid seedCodes)."""
        raise NotImplementedError

    def problems(self, out, pack, seeds):  # pragma: no cover
        """Return (filtered output, problems)."""
        raise NotImplementedError

    def apply(self, key, out, ph):  # pragma: no cover
        raise NotImplementedError

    def mark(self, e, ph, confidence=0.7, evidence=()):
        e.prov.derivation = "llm"
        e.prov.model = self.model
        e.prov.prompt_hash = ph
        e.prov.run_id = self.run_id
        e.prov.confidence = confidence
        ev = [Evidence(c, q) for c, q in evidence if c in self.pkg.nodes]
        if not ev:
            ev = [Evidence(b, "") for b in e.prov.based_on[:1]]
        e.prov.evidence = ev

    def call(self, job, fresh=False):
        """LLM call only (thread-safe); one repair turn if the output is ungrounded. `fresh`: a retry, bypass the cache."""
        key, pack, seeds, user = job
        out = self.client.complete_json(self.step, self.system, user, self.schema, fresh=fresh)
        calls = 1
        _, probs = self.problems(out, pack, seeds)
        if probs:
            user = user + "\n\n## Grounding errors of the previous answer\n- " + "\n- ".join(probs[:30]) + \
                "\nReturn the whole corrected JSON object.\n" + wrap_data("previous", json.dumps(out, ensure_ascii=False)[:20000])
            out = self.client.complete_json(self.step, self.system, user, self.schema, fresh=fresh)
            calls += 1
        return out, user, calls

    def grade(self, job, res):
        _, pack, seeds, _ = job
        keep, probs = self.problems(res[0], pack, seeds)
        n = n_items(keep)
        if not probs:
            return "ok", n
        return ("partial" if n else "failed"), n

    def run(self):
        if not self.client.enabled(self.step):
            self.res.notes.append(f"LLM is off for step {self.step}: using rule templates")
            return self.res
        # each group's pack reads only the group's own nodes, so building all packs first and calling in parallel equals a sequential run
        jobs = [(key, pack, seeds, f"Group: {key}\n\n" + wrap_data("context", pack) + "\n\nFill following the rules. Use only codes in allowedCodes.")
                for key, pack, seeds in self.groups()]
        final = run_groups(self.call, jobs, self.cfg, self.grade, self.client)
        for (key, pack, seeds, _), f in zip(jobs, final, strict=True):
            record_retry(self.res, key, f)
            err = f["err"]
            if f["res"] is None:
                if isinstance(err, LLMDisabled):
                    self.res.skipped.append({"group": key, "reason": "LLM off"})
                elif isinstance(err, BudgetExceeded):
                    reason = f"cap reached: {err}"
                    self.res.skipped.append({"group": key, "reason": reason})
                    if reason not in self.res.notes:
                        self.res.notes.append(reason)
                else:
                    log.warning("LLM error in group %s: %s", key, err)
                    reason = f"{getattr(err, 'category', 'error')}: {err}"
                    self.res.skipped.append({"group": key, "reason": reason})
                    self.res.failed_groups.append({"group": key, "reason": reason})
                continue
            out, user, calls = f["res"]
            self.res.calls += f["calls"]
            keep, dropped = self.problems(out, pack, seeds)
            if dropped:
                self.res.rejected.append({"group": key, "problems": dropped})
            if f["grade"] == "failed":
                self.res.failed_groups.append({"group": key, "reason": "every item returned by the LLM was ungrounded after retries"})
            self.apply(key, keep, prompt_hash(self.system, user))
        check_failed_ratio(self.step, final, self.res, self.cfg)
        return self.res
