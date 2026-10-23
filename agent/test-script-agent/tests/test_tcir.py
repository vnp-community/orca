import anyio
import pytest

from tsa import tcir
from tsa.errors import TcirError


def test_bundle_shapes(pay_json):
    b = tcir.Bundle.from_json(pay_json)
    assert b.feature == "PAY" and b.spec_version == 1 and len(b.cases) == 35
    one = tcir.Bundle.from_json({"testCase": pay_json["testCases"][0]})
    assert one.feature == "PAY" and [c["id"] for c in one.cases] == [pay_json["testCases"][0]["id"]]
    with pytest.raises(TcirError):
        tcir.Bundle.from_json({**pay_json, "schemaVersion": "tcir/v0"})
    with pytest.raises(TcirError):
        tcir.Bundle.from_json({"schemaVersion": "tcir/v1", "testCases": []})


def test_select_never_guesses(pay_bundle):
    cases = pay_bundle.cases
    broken = [{**cases[0], "expected": []}, {**cases[1], "automatable": False}, {**cases[2], "id": "OTHER"},
              {**cases[3], "expected": [{**cases[3]["expected"][0], "op": ""}]}] + cases[4:6] + [cases[4]]
    ok, skipped = tcir.select(tcir.Bundle("PAY", 1, None, broken))
    assert [c["id"] for c in ok] == [cases[4]["id"], cases[5]["id"]]
    reasons = {s.case: s.reason for s in skipped}
    assert "no expected" in reasons[cases[0]["id"]] and "automatable" in reasons[cases[1]["id"]]
    assert "<case>.E<n>" in reasons["OTHER"] and "operator" in reasons[cases[3]["id"]]
    assert sum(1 for s in skipped if s.reason == "duplicate test case id") == 1


def test_only_approved_and_only(pay_bundle):
    ok, skipped = tcir.select(pay_bundle, only_approved=True)
    assert ok == [] and len(skipped) == 35
    first = pay_bundle.cases[0]["id"]
    ok, skipped = tcir.select(pay_bundle, only={first})
    assert [c["id"] for c in ok] == [first] and skipped == []


def test_collect_pages_follows_next_and_reports_errors(pay_json):
    cases = pay_json["testCases"]
    pages = {0: {**pay_json, "testCases": cases[:10], "truncated": True, "next": 10},
             10: {"testCases": cases[10:30], "next": 30}, 30: {"testCases": cases[30:], "next": None}}
    seen = []

    async def call(tool, args):
        seen.append((tool, args))
        return pages[args.get("offset", 0)]

    out = anyio.run(tcir.collect_pages, call, "get_tcir", "PAY", "L3")
    assert len(out["testCases"]) == 35 and out["feature"] == "PAY" and "next" not in out and "truncated" not in out
    assert seen[0] == ("get_tcir", {"feature": "PAY", "level": "L3", "limit": 5}) and seen[2][1]["offset"] == 30

    async def bad(tool, args):
        return {"error": {"code": "TESTSPEC_NOT_FOUND", "message": "no specs"}}

    with pytest.raises(tcir.SourceError, match="TESTSPEC_NOT_FOUND"):
        anyio.run(tcir.collect_pages, bad, "testspec_tcir_get", "PAY")


def test_mcp_source_picks_the_tool_from_the_command():
    assert tcir.McpSource.from_command_line("tsagent serve --stdio").tool == "get_tcir"
    assert tcir.McpSource.from_command_line("tdd-agent -C /p mcp serve").tool == "testspec_tcir_get"
    with pytest.raises(tcir.SourceError):
        tcir.McpSource.from_command_line("x", tool="nope")
