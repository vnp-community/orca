"""T8.1: profile detection from ontologyVersion; Extended relation name mapping, override through TSAGENT_PROFILES."""
import pytest

from tsagent.analyze import profiles
from tsagent.model import SourcePackage

pytestmark = pytest.mark.unit


def pkg(ov):
    p = SourcePackage("F")
    p.meta["ontologyVersion"] = ov
    return p


@pytest.mark.parametrize("ov,prof", [("tdd-core-1", "core"), ("tdd-core-1+ext-0", "core"), ("tdd-core-1+ext-1", "extended"),
                                     ("tdd-core-1+ext-3", "extended")])
def test_profile_detection(ov, prof):
    assert profiles.profile_of(pkg(ov)) == prof


def test_relation_mapping_and_override(tmp_path, monkeypatch):
    assert profiles.rel("idempotencyOf") == "idempotencyOf" and profiles.rel("decidesOperation") == "decides"
    f = tmp_path / "p.yaml"
    f.write_text("extended:\n  relations:\n    idempotencyOf: hasIdempotencyPolicy\n", encoding="utf-8")
    monkeypatch.setenv("TSAGENT_PROFILES", str(f))
    assert profiles.rel("idempotencyOf") == "hasIdempotencyPolicy" and profiles.rel("protects") == "protects"
