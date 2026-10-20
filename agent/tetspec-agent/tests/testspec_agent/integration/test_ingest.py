"""T3.2, T3.3: new md parser, source merging, path limits, inbox, baseline creation."""
import os

import pytest

from tsagent.errors import InputError
from tsagent.ingest import ingest
from tsagent.ingest.md import parse_documents
from tsagent.store import source

pytestmark = pytest.mark.integration
F = "SCHEDULED-TRANSFER"


def test_minipay_kg_solution_supplement_become_one_package(cfg, minipay_docs):
    pkg, diags = ingest([minipay_docs["slice"], minipay_docs["solution"], minipay_docs["supplement"]], F, cfg)
    assert not [d for d in diags if d.severity == "error"]
    # solution endpoints link to KG FRs; supplement ACs link to KG FRs
    assert {n.code for n in pkg.targets("EP-CreateSchedule", "implements")} == {"FR-040", "FR-041", "FR-048"}
    assert "AC-REV-001" in {n.code for n in pkg.targets("FR-041", "hasAcceptance")}
    assert pkg.get("AC-REV-001").props["derivation"] == "supplement" and pkg.get("AC-REV-001").props["basis"]
    assert {n.code for n in pkg.targets("SM-Schedule", "governs")} == {"DE-ScheduledTransfer"}
    assert "TRN-Schedule-1" in {n.code for n in pkg.targets("SM-Schedule", "hasTransition")}
    assert pkg.get("FLD-Create-amount").props["max"] == 50000000 and pkg.get("FLD-Create-recurrence").props["enum"][0] == "ONCE"
    assert "Field" in pkg.ontology.classes and pkg.meta["format"] == "tdd-kg/v1+md"
    assert pkg.meta["primaryDoc"]


def test_md_only_pay(cfg, pay_docs):
    pkg, _ = ingest(pay_docs, "PAY", cfg)
    assert pkg.get("FR-001").props["text"].startswith("Hệ thống cho phép thanh toán")
    assert [n.code for n in pkg.targets("FR-001", "hasAcceptance")] == ["AC-FR-001-1"]
    assert pkg.get("TO-01").props == {"connectMs": 2000, "readMs": 10000, "totalMs": 25000, "poolAcquireMs": 3000,
                                      "sourceFile": "tdd.md", "sourceLine": 36}
    assert pkg.get("SCN-01").props["steps"][0] == "Khách gửi yêu cầu thanh toán"
    assert {n.code for n in pkg.targets("INT-01", "hasTimeoutPolicy")} == {"TO-01"}
    assert pkg.meta["primaryDoc"] == "TDD-PAY" and "Integration" in pkg.ontology.classes


def test_baseline_from_ingest_roundtrips(cfg, tmp_db, minipay_docs):
    pkg, _ = ingest([minipay_docs["slice"], minipay_docs["solution"]], F, cfg)
    bid = source.save_baseline(tmp_db, pkg)["baseline"]
    back = source.load_source(tmp_db, F, bid)
    assert back.content_hash() == pkg.content_hash() and bid.startswith("b-")


def test_path_outside_source_roots_is_rejected(cfg, tmp_path):
    outside = "/etc/hostname"
    with pytest.raises(InputError) as e:
        ingest([outside], F, cfg)
    assert e.value.diagnostics[0].rule == "INGEST-ROOT"


def test_documents_content_goes_to_inbox(cfg, minipay_docs):
    content = open(minipay_docs["solution"], encoding="utf-8").read()
    kg = open(minipay_docs["slice"], encoding="utf-8").read()
    pkg, _ = ingest([{"name": "kg.json", "content": kg}, {"name": "../../evil/solution.md", "content": content}], F, cfg)
    assert os.path.isfile(os.path.join(cfg.inbox(), "solution.md"))  # the name is sanitised and cannot escape the inbox
    assert "EP-CreateSchedule" in pkg.nodes


def test_dangling_reference_reports_file_and_line(cfg, tmp_path):
    p = tmp_path / "srs.md"
    p.write_text("---\ndoc_id: SRS-X\ndoc_type: srs\nstatus: active\n---\n\n| Mã | Nội dung | allocatedTo |\n| --- | --- | --- |\n"
                 "| FR-001 | Làm gì đó | MOD-NOPE |\n", encoding="utf-8")
    with pytest.raises(InputError) as e:
        ingest([str(p)], "X", cfg)
    d = [x for x in e.value.diagnostics if x.rule == "MD-DANGLING"][0]
    assert d.file == "srs.md" and d.loc == "9" and "MOD-NOPE" in d.message


def test_same_code_two_classes_is_error(cfg, tmp_path, minipay_pkg):
    import copy
    import json
    kg = copy.deepcopy(minipay_pkg)
    kg["nodes"].append({"code": "FR-777", "class": "Module", "props": {"responsibility": "x"}})
    md = "| Mã | Nội dung |\n| --- | --- |\n| FR-777 | yêu cầu |\n"
    with pytest.raises(InputError) as e:
        ingest([{"name": "kg.json", "content": json.dumps(kg)}, {"name": "srs.md", "content": md}], F, cfg)
    assert "INGEST-DUP-CLASS" in {d.rule for d in e.value.diagnostics}


def test_supplement_rules(cfg, tmp_path):
    p = tmp_path / "supplement.md"
    p.write_text("| Mã | Yêu cầu | Tiêu chí | Căn cứ |\n| --- | --- | --- | --- |\n| AC-REV-1 | FR-001 | x | |\n"
                 "| MOD-X | | | SRS |\n", encoding="utf-8")
    pkg, diags = parse_documents([("supplement.md", str(p))], "X", known={"FR-001"})
    rules = sorted(d.rule for d in diags if d.severity == "error")
    assert rules == ["MD-EMPTY", "MD-SUPPLEMENT-BASIS", "MD-SUPPLEMENT-PREFIX"]


def test_solution_free_text_tables_are_ignored(cfg, minipay_docs, minipay_pkg):
    pkg, _ = parse_documents([("solution.md", minipay_docs["solution"])], F, known={n["code"] for n in minipay_pkg["nodes"]})
    # the "Mã | Yêu cầu (rút gọn)" table of section 1 is outside the appendix: no FR node from the solution
    assert not list(pkg.of("FunctionalRequirement"))


def test_turtle_is_rejected_with_hint(cfg, tmp_path):
    p = tmp_path / "kg-slice.ttl"
    p.write_text("@prefix tdd: <https://example.org/tdd#> .\n", encoding="utf-8")
    with pytest.raises(InputError) as e:
        ingest([str(p)], "X", cfg)
    d = e.value.diagnostics[0]
    assert d.rule == "INGEST-TYPE" and "tdd-kg/v1" in d.message and "Turtle" in d.message
