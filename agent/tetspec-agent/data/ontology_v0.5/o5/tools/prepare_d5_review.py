#!/usr/bin/env python3
"""Prepare and validate the tiered D5 review bundle.

This tool never invents scores.  It creates a hash-bound review form for every
direct and KG authoring run and evaluates it only after a reviewer supplies
identity, independence, critical-error decisions and seven 0-4 scores. L1 is
diagnostic; only L2 independent human review is acceptance evidence.
"""
from __future__ import annotations

import argparse
import copy
import json
from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "tools"))
from quality_eval import DIMENSIONS, evaluate_reviews, review_template  # noqa: E402


def write_json(path: Path, value: dict) -> None:
    path.write_text(json.dumps(value, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")


def markdown_bundle(runs: list[dict], review_path: str) -> str:
    lines = [
        "# D5 independent human review — v0.6 benchmark",
        "",
        "Review the generated OpenSpec artifacts against the pinned CR/SRS/SAD/TDD context.",
        "This form is L1 diagnostic evidence by default; it can identify findings but cannot close acceptance.",
        "For acceptance, run `--promote-l2 <L1-file> --output <L2-file>`, then use an independent human reviewer on the new file.",
        "Do not use the authoring model's critique as the human score. Record findings in " + review_path + ".",
        "",
        "## Protocol",
        "",
        "1. For L1, identify the reviewer and record provisional findings; L1 is diagnostic only.",
        "2. For L2, confirm the reviewer is independent of generation and fill `reviewer.identity`.",
        "3. For every run, set `critical_errors` and `missing_mandatory_behavior` to true/false.",
        "4. Score each dimension from 0 to 4. Keep the output hash unchanged.",
        "5. Cite concrete artifact paths/sections in `findings`; do not score formatting alone.",
        "6. Run `python3 data/ontology_v0.5/o5/tools/prepare_d5_review.py --run-dir <dir> --review <file> --evaluate <out>`.",
        "",
        "## Dimensions",
        "",
    ]
    lines.extend(f"- `{dimension}`: 0 absent/wrong; 1 major gaps; 2 partial; 3 implementable with minor gaps; 4 complete and grounded." for dimension in DIMENSIONS)
    lines += ["", "## Runs", ""]
    for run in runs:
        ident = run.get("id")
        lines += [
            f"### {ident} ({run.get('mode')})",
            "",
            f"- output hash: `{run.get('output_hash')}`",
            f"- artifacts: `{run.get('artifact_dir')}`",
            f"- semantic report: `benchmark/{ident}/report.json`",
            "- Review the full proposal, design, tasks and spec deltas in the artifact directory.",
            "",
        ]
    return "\n".join(lines) + "\n"


def prepare(run_dir: Path) -> tuple[dict, str]:
    runs = json.loads((run_dir / "benchmark.runs.json").read_text(encoding="utf-8"))
    if len(runs) != 6 or {r.get("mode") for r in runs} != {"direct", "kg"}:
        raise ValueError("D5_REQUIRES_THREE_DIRECT_AND_THREE_KG_RUNS")
    review = review_template(runs, review_level='L1', review_id='D5-L1-v6')
    review.update({
        "benchmark_version": "0.6.0",
        "context": "benchmark-context.json",
        "protocol": "D5-tiered-review-v1",
        "reviewer": {"kind": "human", "identity": None, "independent_of_generation": None},
    })
    return review, markdown_bundle(runs, "benchmark.review.v6.json")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--run-dir", required=True)
    parser.add_argument("--review", help="review JSON to validate/evaluate")
    parser.add_argument("--evaluate", help="write evaluation JSON")
    parser.add_argument("--prepare", action="store_true", help="write the v0.6 review template")
    parser.add_argument("--promote-l2", help="create an L2 review file from an L1 file without overwriting L1")
    parser.add_argument("--output", help="output path for --promote-l2")
    parser.add_argument("--review-id", default="D5-L2-pending", help="new ID for --promote-l2")
    args = parser.parse_args()
    run_dir = Path(args.run_dir).resolve()
    if args.promote_l2:
        source = Path(args.promote_l2).resolve()
        review = json.loads(source.read_text(encoding="utf-8"))
        if review.get("review_level") != "L1":
            raise ValueError("L2_PROMOTION_REQUIRES_L1_SOURCE")
        promoted = copy.deepcopy(review)
        promoted["review_level"] = "L2"
        promoted["review_id"] = args.review_id
        promoted["supersedes_review_id"] = review.get("review_id")
        promoted["reviewer"] = {"kind": "human", "identity": None, "independent_of_generation": None}
        output = Path(args.output).resolve() if args.output else source.with_name(source.stem + ".L2.json")
        write_json(output, promoted)
        print(json.dumps({"status": "promoted_l2", "source": str(source), "output": str(output), "supersedes_review_id": promoted["supersedes_review_id"]}, ensure_ascii=False))
        return 0
    if args.prepare or not args.review:
        review, markdown = prepare(run_dir)
        write_json(run_dir / "benchmark.review.v6.json", review)
        (run_dir / "benchmark-review.v6.md").write_text(markdown, encoding="utf-8")
        print(json.dumps({"status": "prepared", "review": str(run_dir / "benchmark.review.v6.json"), "guide": str(run_dir / "benchmark-review.v6.md")}, ensure_ascii=False))
    if args.review:
        runs = json.loads((run_dir / "benchmark.runs.json").read_text(encoding="utf-8"))
        review = json.loads(Path(args.review).read_text(encoding="utf-8"))
        result = evaluate_reviews(runs, review)
        if args.evaluate:
            write_json(Path(args.evaluate), result)
        print(json.dumps(result, ensure_ascii=False))
        return 0 if result.get("status") in {"pass", "provisional_l1"} else 2
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
