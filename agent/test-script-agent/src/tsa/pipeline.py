"""The 9 steps of option C: read TCIR, check it, create the worktree, plan, generate, static check, spec check, fix (<= N rounds, diff
guard), commit. TSA never runs the tests and never opens a PR: those are later steps."""
from __future__ import annotations

import json
import time
from dataclasses import dataclass, field
from pathlib import Path

from . import conformance, conventions, planner, report, static_check, tcir, worktree
from .conformance import ERROR, Finding
from .generators import Request
from .log import log


@dataclass
class Options:
    repo: Path
    base: str
    feature: str
    level: str | None = None
    branch: str | None = None
    worktree_path: Path | None = None
    reuse: bool = False
    max_fix: int = 3
    only_approved: bool = False
    only: set[str] | None = None
    commit: bool = True
    # Folders the worktree checks out when the test repository is also the application repository (sparse checkout).
    sparse_paths: list[str] | None = None
    # The spec version the caller expects (Orca sends the one its test-spec lane produced); another version is an error.
    spec_version: int | None = None


@dataclass
class FileOutcome:
    path: str
    group: str
    cases: list[str]
    status: str                    # generated | needs-review | failed
    attempts: int
    findings: list[Finding] = field(default_factory=list)
    manifest: list[dict] = field(default_factory=list)
    cost_usd: float = 0.0
    generator_detail: str = ""


@dataclass
class Outcome:
    feature: str
    spec_version: int
    branch: str
    worktree: str
    generator: str
    files: list[FileOutcome]
    skipped: list[tcir.Skipped]
    commit: str | None
    manifest_path: str
    report_path: str
    seconds: float

    @property
    def counts(self) -> dict:
        out = {"files": len(self.files), "skipped": len(self.skipped)}
        for f in self.files:
            out[f.status] = out.get(f.status, 0) + len(f.cases)
        return out

    def as_dict(self) -> dict:
        return {"feature": self.feature, "specVersion": self.spec_version, "branch": self.branch, "worktree": self.worktree,
                "generator": self.generator, "commit": self.commit, "counts": self.counts, "manifest": self.manifest_path,
                "report": self.report_path, "files": [{"path": f.path, "status": f.status, "attempts": f.attempts, "cases": f.cases,
                                                        "errors": sum(1 for x in f.findings if x.severity == ERROR)} for f in self.files],
                "skipped": [{"case": s.case, "reason": s.reason} for s in self.skipped], "seconds": round(self.seconds, 1),
                "costUsd": round(sum(f.cost_usd for f in self.files), 4)}


def _checks(root: Path, plan: planner.FilePlan, fixtures: set[str]) -> tuple[list[Finding], list[dict], list[conformance.Scan]]:
    static = static_check.check_file(root, plan.path, fixtures)
    path = root / plan.path
    scans = [conformance.scan(path.read_text(encoding="utf-8"), plan.path)] if path.is_file() else []
    spec, manifest = conformance.check(plan.cases, scans)
    seen = {(f.rule, f.line) for f in static}
    return static + [f for f in spec if (f.rule, f.line) not in seen], manifest, scans


def _feedback(findings: list[Finding]) -> list[str]:
    return [f"{f.rule}: {f.message}" + (f" (line {f.line})" if f.line else "") + (f" [@exp {f.exp}]" if f.exp else "")
            for f in findings if f.severity == ERROR]


def generate_file(root: Path, plan: planner.FilePlan, gen, req_base: dict, fixtures: set[str], max_fix: int) -> FileOutcome:
    out = FileOutcome(plan.path, plan.group, [c["id"] for c in plan.cases], "failed", 0)
    path = root / plan.path
    baseline: dict[str, str] | None = None
    previous: str | None = None
    feedback: list[str] = []
    for attempt in range(1, max_fix + 2):
        out.attempts = attempt
        res = gen.generate(Request(root, plan, feedback=feedback, **req_base))
        out.cost_usd += res.cost_usd
        out.generator_detail = res.detail
        if not res.ok:
            out.findings = [Finding("generator", ERROR, res.detail, plan.path)]
            log("generator failed", file=plan.path, attempt=attempt, detail=res.detail[:200])
            break
        findings, manifest, scans = _checks(root, plan, fixtures)
        if baseline:
            broken = conformance.guard(baseline, scans)
            if broken:
                # Keep the version whose assertions matched the spec; the fix round is thrown away.
                if previous is not None:
                    path.write_text(previous, encoding="utf-8")
                findings, manifest, _ = _checks(root, plan, fixtures)
                out.findings, out.manifest, out.status = broken + findings, manifest, "needs-review"
                log("diff guard: fix round rewrote a verified assertion, reverted", file=plan.path, attempt=attempt)
                return out
        out.findings, out.manifest = findings, manifest
        errors = [f for f in findings if f.severity == ERROR]
        log("checked", file=plan.path, attempt=attempt, errors=len(errors))
        if not errors:
            out.status = "generated"
            return out
        verified = conformance.verified(findings, manifest)
        baseline = {**verified, **(baseline or {})}
        previous = path.read_text(encoding="utf-8") if path.is_file() else None
        feedback = _feedback(findings)
    else:
        out.status = "needs-review"
        return out
    return out


def run(opts: Options, source, gen) -> Outcome:
    t0 = time.monotonic()
    bundle = source.load(opts.feature, opts.level) if not isinstance(source, tcir.Bundle) else source
    if bundle.feature != opts.feature:
        raise tcir.TcirError(f"TCIR is for feature {bundle.feature!r}, not {opts.feature!r}")
    if opts.spec_version is not None and bundle.spec_version != opts.spec_version:
        raise tcir.TcirError(f"TCIR of {bundle.feature} is version {bundle.spec_version}, expected {opts.spec_version}",
                             code="SPEC_VERSION_CHANGED")
    cases, skipped = tcir.select(bundle, opts.only_approved, opts.only)
    log("tcir", feature=bundle.feature, version=bundle.spec_version, cases=len(cases), skipped=len(skipped))
    branch = opts.branch or worktree.branch_name(bundle.feature, bundle.spec_version)
    root = worktree.create(opts.repo, opts.base, branch, opts.worktree_path, opts.reuse, opts.sparse_paths)
    log("worktree", path=str(root), branch=branch)
    conv = conventions.detect(root)
    fixtures = set(conv.fixtures)
    plans = planner.plan(cases, conv, bundle.feature)
    req_base = {"feature": bundle.feature, "spec_version": bundle.spec_version, "conventions": conv}
    files = [generate_file(root, p, gen, req_base, fixtures, opts.max_fix) for p in plans]
    out_dir = root / planner.output_dir(conv, bundle.feature)
    out_dir.mkdir(parents=True, exist_ok=True)
    manifest_path = out_dir / "assertion-manifest.json"
    report_path = out_dir / "tsa-report.md"
    outcome = Outcome(bundle.feature, bundle.spec_version, branch, str(root), gen.name, files, skipped, None,
                      str(manifest_path.relative_to(root)), str(report_path.relative_to(root)), time.monotonic() - t0)
    manifest_path.write_text(json.dumps(report.manifest(outcome), ensure_ascii=False, indent=1) + "\n", encoding="utf-8")
    report_path.write_text(report.markdown(outcome, bundle), encoding="utf-8")
    if opts.commit:
        written = [f.path for f in files if (root / f.path).is_file()] + [outcome.manifest_path, outcome.report_path]
        c = outcome.counts
        msg = (f"test(tsa): {bundle.feature} v{bundle.spec_version} test scripts\n\n"
               f"generated {c.get('generated', 0)}, needs-review {c.get('needs-review', 0)}, failed {c.get('failed', 0)}, "
               f"skipped {c['skipped']} (generator: {gen.name})")
        outcome.commit = worktree.commit(root, msg, written)
    outcome.seconds = time.monotonic() - t0
    return outcome
