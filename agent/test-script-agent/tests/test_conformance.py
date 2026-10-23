from tsa import conformance

CASES = [{"id": "TC-1", "expected": [{"id": "TC-1.E1", "op": "==", "value": "FB-01", "kind": "errorCode"},
                                     {"id": "TC-1.E2", "op": "<=", "value": "2 giây", "kind": "latency"}]},
         {"id": "TC-2", "expected": [{"id": "TC-2.E1", "op": "==", "value": "NEW → PAID; ghi sổ", "kind": "state"}]}]

GOOD = '''import pytest


@pytest.mark.tc("TC-1", spec_version=1)
def test_one(api):
    r = api.pay()
    assert r["errorCode"] == "FB-01"  # @exp TC-1.E1
    assert r["latency"] <= 2  # @exp TC-1.E2


@pytest.mark.tc("TC-2", spec_version=1)
def test_two(api):
    assert api.state() == "PAID"  # @exp TC-2.E1
'''


def run(src, cases=CASES):
    scans = [conformance.scan(src, "t.py")]
    findings, manifest = conformance.check(cases, scans)
    return {f.rule for f in findings if f.severity == conformance.ERROR}, findings, manifest, scans


def test_good_file_passes_and_prose_values_are_reported_as_unchecked():
    errors, findings, manifest, _ = run(GOOD)
    assert errors == set()
    assert [r["status"] for r in manifest] == ["ok", "ok", "ok"]
    assert {f.rule for f in findings} == {"value-unchecked"} and all(f.exp == "TC-2.E1" for f in findings)
    assert manifest[0]["line"] == 7 and manifest[0]["function"] == "test_one"


def test_missing_extra_and_unknown():
    src = GOOD.replace('  # @exp TC-1.E2', '').replace('# @exp TC-2.E1', '# @exp TC-2.E9')
    errors, *_ = run(src)
    assert {"missing", "extra", "unknown-exp"} <= errors


def test_value_and_operator_must_match_the_spec():
    errors, findings, manifest, _ = run(GOOD.replace('== "FB-01"', '== "FB-02"').replace("<= 2", "< 2"))
    assert {"value-mismatch", "op-mismatch"} <= errors
    assert {r["exp"]: r["status"] for r in manifest}["TC-1.E1"] == "mismatch"


def test_value_must_be_a_literal_when_the_spec_value_is_a_token():
    errors, *_ = run(GOOD.replace('== "FB-01"', '== expected_code'))
    assert "value-not-literal" in errors


def test_threshold_with_unit_is_a_warning_not_an_error():
    errors, findings, *_ = run(GOOD.replace("<= 2", "<= 2000"))
    assert not errors and any(f.rule == "value-unit" for f in findings)


def test_vacuous_assertions_are_blocked():
    src = GOOD.replace('    assert r["errorCode"] == "FB-01"  # @exp TC-1.E1',
                       '    try:\n        assert r["errorCode"] == "FB-01"  # @exp TC-1.E1\n    except AssertionError:\n        pass')
    src = src.replace('@pytest.mark.tc("TC-2", spec_version=1)', '@pytest.mark.tc("TC-2", spec_version=1)\n@pytest.mark.skip')
    src += '\n\n@pytest.mark.tc("TC-1")\ndef test_three():\n    assert True  # @exp TC-1.E1\n'
    errors, *_ = run(src)
    assert {"in-try", "skip-marker", "vacuous", "duplicate"} <= errors


def test_assertion_must_live_in_the_test_of_its_case_and_every_case_needs_a_test():
    src = GOOD.replace('@pytest.mark.tc("TC-2", spec_version=1)', '@pytest.mark.tc("TC-1", spec_version=1)')
    errors, *_ = run(src)
    assert {"wrong-test", "missing-test"} <= errors


def test_syntax_error_is_reported():
    errors, *_ = run("def test_x(:\n")
    assert "syntax" in errors


def test_diff_guard_blocks_changes_to_verified_assertions():
    _, findings, manifest, _ = run(GOOD)
    base = conformance.verified(findings, manifest)
    assert set(base) == {"TC-1.E1", "TC-1.E2", "TC-2.E1"}
    _, _, _, scans = run(GOOD.replace('== "PAID"', '== "NEW"'))
    broken = conformance.guard(base, scans)
    assert [f.exp for f in broken] == ["TC-2.E1"]
    assert conformance.guard(base, run(GOOD)[3]) == []


NOTES = [{
    "id": "TC-B0", "objective": "Ghi chú rỗng (0 ký tự) bị từ chối",
    "steps": [{"id": "TC-B0.S1", "kind": "when", "action": "Gửi yêu cầu tạo ghi chú với nội dung rỗng", "input": {"value": 0}}],
    "expected": [{"id": "TC-B0.E1", "op": "==", "kind": "errorCode",
                  "value": "Yêu cầu bị từ chối; không có ghi chú nào được lưu. Mã lỗi cụ thể cần bổ sung từ nguồn."}],
    "gaps": [{"code": "TGAP-1", "link": "subject", "question": "Mã lỗi khi nội dung rỗng là gì?"}],
    "contract": {"endpoints": [{"method": "POST", "path": "/notes"}, {"method": "GET", "path": "/notes"}],
                 "entities": [{"code": "ENT-01", "fields": [{"name": "id"}, {"name": "text"}, {"name": "created_at"}]}],
                 "statusCodes": [{"status": 422, "where": "A note of 281 characters is refused with 422.", "related": False}]},
}, {
    "id": "TC-H1", "objective": "GET /healthz trả 200",
    "steps": [{"id": "TC-H1.S1", "kind": "when", "action": "GET /healthz"}],
    "expected": [{"id": "TC-H1.E1", "op": "==", "kind": "status", "value": "200"}],
    "contract": {"endpoints": [{"method": "GET", "path": "/healthz"}]},
}]

NOTES_SRC = """import pytest


@pytest.mark.tc("TC-B0")
def test_b0(api_client):
    before = len(api_client.get("/notes").json() or [])
    r = api_client.post("/notes", json={"text": ""})
    assert len(api_client.get("/notes").json() or []) == before  # @exp TC-B0.E1


@pytest.mark.tc("TC-H1")
def test_h1(api_client):
    assert api_client.get("/healthz").status_code == 200  # @exp TC-H1.E1
"""


def test_assertions_built_only_from_the_spec_pass():
    errors, findings, manifest, _ = run(NOTES_SRC, NOTES)
    assert errors == set(), findings


def test_an_invented_status_code_is_blocked():
    errors, findings, *_ = run(NOTES_SRC.replace("assert len(api_client.get(\"/notes\").json() or []) == before",
                                                 "assert r.status_code == 400"), NOTES)
    assert "value-not-in-spec" in errors
    assert any("400" in f.message for f in findings if f.rule == "value-not-in-spec")


def test_a_status_of_an_unrelated_requirement_is_not_the_spec_of_this_case():
    errors, *_ = run(NOTES_SRC.replace("assert len(api_client.get(\"/notes\").json() or []) == before",
                                       "assert r.status_code == 422"), NOTES)
    assert "value-not-in-spec" in errors


def test_a_field_name_the_contract_does_not_know_is_blocked():
    src = NOTES_SRC.replace("assert len(api_client.get(\"/notes\").json() or []) == before",
                            "assert api_client.get(\"/notes\").json()[0][\"content\"] == \"\"")
    errors, findings, *_ = run(src, NOTES)
    assert "value-not-in-spec" in errors and any("'content'" in f.message for f in findings)
    ok = src.replace('["content"]', '["text"]')
    assert "value-not-in-spec" not in run(ok, NOTES)[0]


def test_gap_marker_only_where_the_spec_leaves_the_value_open():
    src = NOTES_SRC.replace("    assert len(api_client.get(\"/notes\").json() or []) == before  # @exp TC-B0.E1",
                            "    # @gap TC-B0.E1 mã lỗi khi rỗng chưa có trong spec (TGAP-1)")
    errors, findings, manifest, _ = run(src, NOTES)
    assert errors == set()
    row = next(r for r in manifest if r["exp"] == "TC-B0.E1")
    assert row["status"] == "gap" and "TGAP-1" in row["reason"]
    bad = NOTES_SRC.replace("    assert api_client.get(\"/healthz\").status_code == 200  # @exp TC-H1.E1",
                            "    api_client.get(\"/healthz\")\n    # @gap TC-H1.E1 not sure")
    assert "gap-not-allowed" in run(bad, NOTES)[0]
    both = NOTES_SRC.replace("== before  # @exp TC-B0.E1", "== before  # @exp TC-B0.E1\n    # @gap TC-B0.E1 x")
    assert "gap-and-assert" in run(both, NOTES)[0]
