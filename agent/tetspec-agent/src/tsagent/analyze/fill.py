"""LLM fill of layer A (T5.4, FR-005, MECH-11, MECH-12).

One call per subject (a TDD node with obligations; a large subject is split into parts). The output is schema-checked (client),
then grounding-checked: `seedCode` must belong to the pack, codes in `basedOn`/`evidence` must be on the allow list, each condition needs evidence.
On problems one repair turn is made; if still wrong the group is retried (group retry, at most `llm_group_retries` times, bypassing the cache).
When retries run out: the invalid items are dropped (the seed content is kept) and the subject gets a TestGap with the reason.
LLM error, breaker open: group retry as above; cap reached: keep the seed, add a note; a `fatal` error (bad key, not logged in) stops the run.
More than half of the groups of the step still failing after retries: the run fails (`LLMGroupsFailed`).
"""
import hashlib
import json
import os
import threading
import time
from dataclasses import dataclass, field

from ..errors import BudgetExceeded, LLMDisabled, LLMError, LLMGroupsFailed
from ..llm import wrap_data
from ..log import get_logger
from ..model import Evidence
from . import codes
from .context import context_pack

log = get_logger("tsagent.analyze")
PROMPT_DIR = os.path.join(os.path.dirname(__file__), "prompts")
CHUNK = 20
FILL_TARGETS = (("oracle", ("expected", "tolerance", "measurement")), ("faultModel", ("expectedHandling",)),
                ("partition", ("values",)), ("path", ("steps",)))


def load_prompt(name):
    with open(os.path.join(PROMPT_DIR, f"{name}.md"), encoding="utf-8") as fh:
        return fh.read()


def prompt_hash(system, user):
    return hashlib.sha256((system + "\x00" + user).encode("utf-8")).hexdigest()[:16]


@dataclass
class FillResult:
    calls: int = 0
    filled: list = field(default_factory=list)      # condition codes filled by the LLM
    proposed: list = field(default_factory=list)
    gaps: list = field(default_factory=list)
    rejected: list = field(default_factory=list)    # {"subject", "problems"}
    skipped: list = field(default_factory=list)     # {"subject", "reason"}
    reused: list = field(default_factory=list)
    notes: list = field(default_factory=list)
    failed_groups: list = field(default_factory=list)  # {"subject", "reason"}: groups still failing after retries
    retried: list = field(default_factory=list)        # groups retried at least once
    retry_calls: int = 0                               # number of retries (per group)

    def to_dict(self):
        return {k: v for k, v in self.__dict__.items()}


def check_output(out, pack, seed_codes):
    """Grounding problems of one LLM output (already schema-valid)."""
    allowed = set(pack["allowedCodes"])
    probs = []
    if out.get("subject") != pack["subject"]["code"]:
        probs.append(f"subject must be {pack['subject']['code']}, got {out.get('subject')}")
    for i, c in enumerate(out.get("conditions", [])):
        probs += item_problems(f"conditions[{i}]", c, allowed)
        if c.get("seedCode") not in seed_codes:
            probs.append(f"conditions[{i}].seedCode {c.get('seedCode')} is not in the pack")
    for i, c in enumerate(out.get("proposedConditions", [])):
        probs += item_problems(f"proposedConditions[{i}]", c, allowed)
        src = (c.get("oracle") or {}).get("sourceCode")
        if src and src not in allowed:
            probs.append(f"proposedConditions[{i}].oracle.sourceCode {src} is not on the allow list")
    obs = {o["seedCode"] for o in pack["obligations"]}
    for i, o in enumerate(out.get("obligations", [])):
        if o.get("seedCode") not in obs:
            probs.append(f"obligations[{i}].seedCode {o.get('seedCode')} is not in the pack")
    return probs


def item_problems(path, c, allowed):
    probs = []
    bad = [b for b in c.get("basedOn", []) if b not in allowed]
    bad += [e["sourceCode"] for e in c.get("evidence", []) if e["sourceCode"] not in allowed]
    if bad:
        probs.append(f"{path}: codes not on the allow list {sorted(set(bad))}")
    if not c.get("evidence"):
        probs.append(f"{path}: missing evidence")
    return probs


def split_valid(out, pack, seed_codes):
    """Keep the valid items; return (filtered output, problems of the dropped items)."""
    allowed = set(pack["allowedCodes"])
    keep, dropped = dict(out), []
    keep["conditions"] = []
    for i, c in enumerate(out.get("conditions", [])):
        p = item_problems(f"conditions[{i}]", c, allowed) + \
            ([f"conditions[{i}].seedCode is not in the pack"] if c.get("seedCode") not in seed_codes else [])
        (dropped.extend(p) if p else keep["conditions"].append(c))
    keep["proposedConditions"] = []
    for i, c in enumerate(out.get("proposedConditions", [])):
        p = item_problems(f"proposedConditions[{i}]", c, allowed)
        src = (c.get("oracle") or {}).get("sourceCode")
        if src and src not in allowed:
            p.append("oracle.sourceCode is not on the allow list")
        (dropped.extend(p) if p else keep["proposedConditions"].append(c))
    obs = {o["seedCode"] for o in pack["obligations"]}
    keep["obligations"] = [o for o in out.get("obligations", []) if o.get("seedCode") in obs]
    return keep, dropped


class Filler:
    def __init__(self, client, seeder, cfg, run_id="", only=None, prev=None):
        self.client, self.s, self.cfg, self.run_id = client, seeder, cfg, run_id
        self.g, self.pkg, self.f = seeder.g, seeder.pkg, seeder.pkg.feature
        self.only = set(only) if only is not None else None  # affected subjects (update driven by a CR)
        self.prev = prev                                     # TsoGraph of the previous version, to reuse LLM content
        self.res = FillResult()
        self.system = load_prompt("analysis")
        self.model = getattr(getattr(client, "provider", None), "tag", "") or ""
        self.stop = None

    def groups(self):
        for subject in sorted(self.s.obligations):
            obs = self.s.obligations[subject]
            conds = [c.code for c in self.g.of("TestCondition") if c.rel("obligation") and c.rel("obligation")[0] in obs]
            for i in range(0, max(1, len(conds)), CHUNK):
                yield subject, obs, conds[i:i + CHUNK]

    def run(self):
        if not self.client.enabled("analysis"):
            self.res.notes.append("LLM is off for step analysis: using rule templates")
            return self.res
        jobs = []
        for subject, obs, conds in self.groups():
            if self.only is not None and subject not in self.only:
                self.reuse(obs, conds)
                continue
            jobs.append(self.prepare(subject, obs, conds))
        # call the LLM in parallel (TSAGENT_LLM_PARALLEL) with round-based group retries; apply results in group order so the result is deterministic
        final = run_groups(self.call, jobs, self.cfg, self.grade, self.client)
        for job, f in zip(jobs, final, strict=True):
            record_retry(self.res, job["subject"], f)
            self.finish(job, f["res"], f["err"], f["grade"], f["calls"])
        check_failed_ratio("analysis", final, self.res, self.cfg)
        return self.res

    def reuse(self, obs, conds):
        if self.prev is None:
            return
        for code in list(obs) + list(conds):
            old = self.prev.get(code)
            cur = self.g.get(code)
            if old is None or cur is None or old.prov.derivation != "llm":
                continue
            cur.attrs.update({k: v for k, v in old.attrs.items() if k in cur.attrs or v is not None})
            cur.prov = old.prov
            for rel, _ in FILL_TARGETS:
                for t in cur.rel(rel):
                    o = self.prev.get(t)
                    if o is not None and o.prov.derivation == "llm" and self.g.get(t) is not None:
                        self.g.get(t).attrs.update(o.attrs)
                        self.g.get(t).prov = o.prov
            self.drop_content_gap(code)
            self.res.reused.append(code)

    def prepare(self, subject, obs, conds):
        """The context pack is built from the seed state (before any result is applied), so it is identical to a sequential run: the cache still hits."""
        gaps = [self.s.content_gaps[c] for c in conds if c in self.s.content_gaps and self.g.get(self.s.content_gaps[c])]
        pack = context_pack(self.pkg, self.g, subject, obs, conds, self.cfg.context_depth, self.cfg.context_max_chars, gaps)
        user = ("Subject: " + subject + "\n\n" + wrap_data("context", pack) +
                "\n\nFill the seed nodes following the rules. Use only codes in allowedCodes.")
        return {"subject": subject, "obs": obs, "conds": conds, "pack": pack, "user": user, "seeds": set(conds)}

    def call(self, job, fresh=False):
        """LLM call only (thread-safe): returns (output, last user prompt, number of calls). `fresh`: a retry, bypass the cache."""
        out = self.client.complete_json("analysis", self.system, job["user"], "analysis", fresh=fresh)
        calls, user = 1, job["user"]
        probs = check_output(out, job["pack"], job["seeds"])
        if probs:
            user = user + "\n\n## Grounding errors of the previous answer\n- " + "\n- ".join(probs[:30]) + \
                "\nReturn the whole corrected JSON object.\n" + wrap_data("previous", json.dumps(out, ensure_ascii=False)[:20000])
            out = self.client.complete_json("analysis", self.system, user, "analysis", fresh=fresh)
            calls += 1
        return out, user, calls

    def grade(self, job, res):
        """ok: no grounding problem; partial: problems but some valid items; failed: no valid item. Returns (grade, number of valid items)."""
        out = res[0]
        if not check_output(out, job["pack"], job["seeds"]):
            return "ok", n_items(out)
        keep, _ = split_valid(out, job["pack"], job["seeds"])
        n = n_items(keep)
        return ("partial" if n else "failed"), n

    def finish(self, job, res, err, grade=None, total_calls=None):
        subject, obs = job["subject"], job["obs"]
        if err is not None:
            if isinstance(err, LLMDisabled):
                self.res.skipped.append({"subject": subject, "reason": "LLM off"})
                return
            if isinstance(err, BudgetExceeded):
                reason = f"cap reached: {err}"
                self.res.skipped.append({"subject": subject, "reason": reason})
                if reason not in self.res.notes:
                    self.res.notes.append(reason)
                return
            if isinstance(err, LLMError):
                if err.fatal:
                    raise err
                log.warning("LLM error for subject %s: %s", subject, err)
                reason = f"{err.category}: {err}"
                self.res.skipped.append({"subject": subject, "reason": reason})
                self.res.failed_groups.append({"subject": subject, "reason": reason})
                subj = self.pkg.get(subject)
                if subj is not None:  # TestGap text is document content (Vietnamese)
                    self.res.gaps.append(self.s.gap(subj, "llm-failed", f"LLM lỗi ở {subject} sau retry: {reason}"[:300],
                                                    f"Rà soát {subject}: các điều kiện giữ nội dung mẫu luật, chạy lại khi LLM ổn định?",
                                                    subj.base.get("owner") or "QA Lead", blocks_ob=obs[0]))
                return
            raise err
        if grade == "failed":
            self.res.failed_groups.append({"subject": subject, "reason": "every item returned by the LLM was ungrounded after retries"})
        out, user, calls = res
        self.res.calls += calls if total_calls is None else total_calls
        out, dropped = split_valid(out, job["pack"], job["seeds"])
        if dropped:
            self.res.rejected.append({"subject": subject, "problems": dropped})
            subj = self.pkg.get(subject)  # TestGap text is document content (Vietnamese)
            g = self.s.gap(subj, "llm-invalid", f"LLM trả nội dung không có căn cứ cho {subject} ({len(dropped)} mục bị loại)",
                           f"Rà soát {subject}: các điều kiện giữ nội dung mẫu luật, cần bổ sung thủ công?",
                           subj.base.get("owner") or "QA Lead", blocks_ob=obs[0])
            self.res.gaps.append(g)
        self.apply(subject, obs, out, prompt_hash(self.system, user))

    def _prov(self, e, item, ph):
        e.prov.derivation = "llm"
        e.prov.model = self.model
        e.prov.prompt_hash = ph
        e.prov.run_id = self.run_id
        e.prov.confidence = float(item.get("confidence", 0.7))
        ev = [Evidence(x["sourceCode"], (x.get("quote") or "")[:600]) for x in item.get("evidence", [])]
        if ev:
            e.prov.evidence = ev
        e.prov.based_on = sorted(set(e.prov.based_on) | set(item.get("basedOn", [])))

    def apply(self, subject, obs, out, ph):
        for o in out.get("obligations", []):
            e = self.g.get(o["seedCode"])
            e.attrs["question"] = o["question"]
            self._prov(e, {"confidence": 0.8, "evidence": [{"sourceCode": subject, "quote": ""}]}, ph)
        for c in out.get("conditions", []):
            e = self.g.get(c["seedCode"])
            for k in ("statement", "expectedEffect", "preState", "trigger"):
                if c.get(k):
                    e.attrs[k] = c[k]
            self._prov(e, c, ph)
            for rel, keys in FILL_TARGETS:
                if c.get(rel) and e.rel(rel):
                    t = self.g.get(e.rel(rel)[0])
                    vals = {k: c[rel][k] for k in keys if c[rel].get(k) not in (None, "", [])}
                    if vals:
                        t.attrs.update(vals)
                        self._prov(t, c, ph)
            self.drop_content_gap(e.code)
            self.res.filled.append(e.code)
        subj = self.pkg.get(subject)
        for c in out.get("proposedConditions", []):
            self.propose(subj, obs, c, ph)
        for gp in out.get("gaps", []):
            blocks = [b for b in gp.get("blocks", []) if self.g.get(b) is not None and self.g.get(b).cls == "TestCondition"]
            code = self.s.gap(subj, "llm-" + codes.short_hash(gp["missing"]), gp["missing"], gp["askedQuestion"],
                              subj.base.get("owner") or "QA Lead", blocks=blocks, blocks_ob=obs[0])
            self._prov(self.g.get(code), {"confidence": 0.6, "evidence": [{"sourceCode": subject, "quote": gp["missing"]}]}, ph)
            self.res.gaps.append(code)

    def propose(self, subj, obs, c, ph):
        ob_code = codes.obligation(self.f, subj.code, "proposal")
        if self.g.get(ob_code) is None:
            base_ob = self.g.get(obs[0])
            self.s.obligation(subj, "proposal", f"Điều kiện bổ sung do LLM đề xuất cho {subj.code}",
                              base_ob.attrs["priority"], origin="llmProposal")
        key = "p" + codes.short_hash(c["conditionKind"], c["statement"])
        orc = c["oracle"]
        src = self.pkg.get(orc.get("sourceCode") or subj.code)
        oc = self.s.oracle(src, orc["oracleKind"], orc["expected"], key=key, measurement=orc.get("measurement"),
                           tolerance=orc.get("tolerance"))
        code = self.s.condition(ob_code, subj, c["conditionKind"], key, c["statement"], c["expectedEffect"], oc,
                                based=c.get("basedOn", []), pre=c.get("preState"), trigger=c.get("trigger"))
        for x in (self.g.get(code), self.g.get(oc), self.g.get(ob_code)):
            self._prov(x, c, ph)
        self.res.proposed.append(code)

    def drop_content_gap(self, cond_code):
        gcode = self.s.content_gaps.pop(cond_code, None)
        if gcode and self.g.get(gcode) is not None:
            gp = self.g.get(gcode)
            gp.rels["blocks"] = [b for b in gp.rel("blocks") if b != cond_code]
            if not gp.rel("blocks"):
                del self.g.entities[gcode]


def run_parallel(fn, jobs, workers):
    """Run fn(job) for every job on at most `workers` threads; return [(result, error)] in the order of jobs.
    A `fatal` error (bad key, not logged in) cancels the jobs not started yet."""
    workers = max(1, int(workers or 1))
    out = [None] * len(jobs)
    if workers == 1 or len(jobs) <= 1:
        stop = False
        for i, j in enumerate(jobs):
            if stop:
                out[i] = (None, LLMError("run stopped by an earlier error", fatal=False))
                continue
            try:
                out[i] = (fn(j), None)
            except Exception as e:  # noqa: BLE001 - handed back to the caller, which handles each error kind
                out[i] = (None, e)
                stop = getattr(e, "fatal", False)
        return out
    from concurrent.futures import ThreadPoolExecutor
    fatal = threading.Event()

    def guarded(i):
        if fatal.is_set():
            return i, (None, LLMError("run stopped by an earlier error", fatal=False))
        try:
            return i, (fn(jobs[i]), None)
        except Exception as e:  # noqa: BLE001
            if getattr(e, "fatal", False):
                fatal.set()
            return i, (None, e)
    with ThreadPoolExecutor(max_workers=workers, thread_name_prefix="llm") as ex:
        for i, r in ex.map(guarded, range(len(jobs))):
            out[i] = r
    return out


def n_items(out):
    """Number of items in the top-level lists of an LLM output."""
    return sum(len(v) for v in (out or {}).values() if isinstance(v, list))


def wait_breaker(client, cap_s=60.0, sleep=time.sleep):
    """Before a retry round: if the provider's breaker is open, wait until it half-opens (at most `cap_s`). Returns the seconds waited."""
    br = getattr(getattr(client, "provider", None), "breaker", None)
    if br is None or getattr(br, "state", "closed") != "open":
        return 0.0
    left = max(0.0, br.open_s - (br.clock() - br._opened_at))
    d = min(cap_s, left)
    sleep(d)
    return d


_RANK = {"failed": 0, "partial": 1, "ok": 2}


def run_groups(call, jobs, cfg, grade, client=None):
    """Call `call(job, fresh=...)` for every group in parallel; groups not `ok` are retried in rounds, at most
    `cfg.llm_group_retries` rounds (retry rounds bypass the cache). Each group keeps its best result.

    Returns a list in the order of `jobs`: {"res", "err", "grade" (ok|partial|failed|skip), "tries", "items", "calls"}.
    A `fatal` error is raised at once; `LLMDisabled`/`BudgetExceeded` are not retried (grade `skip`); a non-LLMError is a programming error: raised.
    """
    final = [{"res": None, "err": None, "grade": None, "tries": 0, "items": -1, "calls": 0} for _ in jobs]
    todo = list(range(len(jobs)))
    retries = max(0, int(getattr(cfg, "llm_group_retries", 0) or 0))
    workers = getattr(cfg, "llm_parallel", 1)
    for rnd in range(retries + 1):
        if not todo:
            break
        if rnd:
            wait_breaker(client)
        fresh = rnd > 0
        results = run_parallel(lambda i, fresh=fresh: call(jobs[i], fresh=fresh), todo, workers)
        nxt = []
        for i, (res, err) in zip(todo, results, strict=True):
            f = final[i]
            f["tries"] += 1
            if err is not None:
                if getattr(err, "fatal", False) or not isinstance(err, LLMError):
                    raise err
                if isinstance(err, (LLMDisabled, BudgetExceeded)):
                    if f["res"] is None:
                        f.update(err=err, grade="skip")
                    continue
                if f["res"] is None:
                    f.update(err=err, grade="failed")
                nxt.append(i)
                continue
            f["calls"] += res[2] if isinstance(res, tuple) and len(res) > 2 else 1
            g, n = grade(jobs[i], res)
            cur = _RANK.get(f["grade"], -1) if f["res"] is not None else -1
            if f["res"] is None or (_RANK[g], n) > (cur, f["items"]):
                f.update(res=res, err=None, grade=g, items=n)
            if g != "ok":
                nxt.append(i)
        todo = nxt
    return final


def record_retry(res, key, f):
    if f["tries"] > 1:
        res.retried.append(key)
        res.retry_calls += f["tries"] - 1


def check_failed_ratio(step, final, res, cfg):
    """After retries: if more than 50 % of the groups that called the LLM still failed, raise `LLMGroupsFailed` (run fails) with `result` = the step result."""
    failed_groups = res.failed_groups
    attempted = sum(1 for f in final if f["grade"] in _RANK)
    failed = sum(1 for f in final if f["grade"] == "failed")
    if attempted and failed * 2 > attempted:
        sample = failed_groups[0]["reason"] if failed_groups else ""
        err = LLMGroupsFailed(f"LLM_GROUPS_FAILED: {step}: {failed}/{attempted} groups failed after "
                              f"{getattr(cfg, 'llm_group_retries', 0)} retries: {sample}"[:500])
        err.result, err.failed, err.attempted = res, failed, attempted
        raise err


def fill(client, seeder, cfg, run_id="", only=None, prev=None):
    return Filler(client, seeder, cfg, run_id, only, prev).run()
