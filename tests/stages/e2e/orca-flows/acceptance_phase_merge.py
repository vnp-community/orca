"""Combines the outcomes of the two `--ai-stub` phases (stub logged in, stub logged out) into one result per test id.

The phases exist because two groups of tests need opposite environments: AIS-02/04 need an AI, AIS-01/03 need none.
A test that was BLOCKED in one phase and ran in the other takes the result of the phase that ran it. A FAIL in either
phase always wins, so combining can hide a missing environment but never a product failure.
"""
from __future__ import annotations

RANK = {"FAIL": 3, "PASS": 2, "BLOCKED": 1, "SKIP": 0}


def merge_outcomes(first: list[dict], second: list[dict]) -> list[dict]:
    merged: dict[str, dict] = {}
    order: list[str] = []
    for source, tag in ((first, ""), (second, " [logged-out phase]")):
        for o in source:
            key = o["test_id"]
            if key not in merged:
                merged[key] = dict(o)
                order.append(key)
            elif RANK[o["status"]] > RANK[merged[key]["status"]]:
                kept = dict(o)
                kept["detail"] = (o["detail"] + tag).strip()
                if tag and not o["detail"]:
                    kept["detail"] = tag.strip()
                merged[key] = kept
    return [merged[k] for k in order]


def count(outcomes: list[dict]) -> dict[str, int]:
    c = {"PASS": 0, "FAIL": 0, "BLOCKED": 0, "SKIP": 0}
    for o in outcomes:
        c[o["status"]] += 1
    return c
