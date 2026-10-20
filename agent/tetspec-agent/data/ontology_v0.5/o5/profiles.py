"""Deterministic specialist obligation profile composition.

Profiles are data, not a hard-coded list of ten use cases.  A package declares
the semantic checks it supports; composition preserves origin and reports
conflicts/unsupported checks instead of silently treating them as N/A.
"""
from __future__ import annotations

from dataclasses import dataclass, asdict
from typing import Any, Iterable
import json
from hashlib import sha256
from pathlib import Path


@dataclass(frozen=True)
class ProfilePackage:
    id: str
    version: str
    obligations: tuple[dict[str, Any], ...] = ()
    rules: tuple[str, ...] = ()
    supported_classes: tuple[str, ...] = ()
    origin: str = "local"

    @classmethod
    def from_dict(cls, value: dict[str, Any]) -> "ProfilePackage":
        missing = [x for x in ("id", "version") if not value.get(x)]
        if missing:
            raise ValueError("PROFILE_MISSING:" + ",".join(missing))
        return cls(str(value["id"]), str(value["version"]), tuple(value.get("obligations", ()) or ()), tuple(value.get("rules", ()) or ()), tuple(value.get("supported_classes", ()) or ()), str(value.get("origin", "local")))


def validate_profile_package(value: dict[str, Any]) -> list[str]:
    errors = []
    for field in ("id", "version"):
        if not value.get(field): errors.append("PROFILE_MISSING:" + field)
    if value.get("rules") is not None and not isinstance(value.get("rules"), (list, tuple)): errors.append("PROFILE_RULES_MUST_BE_LIST")
    if value.get("supported_classes") is not None and not isinstance(value.get("supported_classes"), (list, tuple)): errors.append("PROFILE_CLASSES_MUST_BE_LIST")
    for obligation in value.get("obligations", []) or []:
        if not obligation.get("id") and not obligation.get("logical_id"): errors.append("PROFILE_OBLIGATION_ID_MISSING:" + str(value.get("id")))
        if not obligation.get("question"): errors.append("PROFILE_OBLIGATION_QUESTION_MISSING:" + str(value.get("id")))
        if obligation.get("validator") and obligation["validator"] not in set(value.get("rules", []) or []): errors.append("PROFILE_VALIDATOR_UNREGISTERED:" + str(obligation["validator"]))
    return sorted(set(errors))


def compose_profiles(packages: Iterable[ProfilePackage | dict[str, Any]]) -> dict[str, Any]:
    raw = [asdict(x) if isinstance(x, ProfilePackage) else dict(x) for x in packages]
    package_errors = [error for value in raw for error in validate_profile_package(value)]
    normalized = [x if isinstance(x, ProfilePackage) else ProfilePackage.from_dict(x) for x in raw]
    obligations: dict[tuple[str, str], dict[str, Any]] = {}
    conflicts: list[dict[str, Any]] = []
    for package in normalized:
        for obligation in package.obligations:
            key = (str(obligation.get("logical_id", obligation.get("id", ""))), json.dumps(obligation.get("constraints", {}), sort_keys=True, ensure_ascii=False))
            prior = obligations.get(key)
            if prior and prior.get("question") != obligation.get("question"):
                conflicts.append({"type": "obligation_definition", "key": key[0], "origins": [prior.get("origin"), package.id]})
                continue
            item = dict(obligation); item.setdefault("origin", package.id); item.setdefault("profile_version", package.version)
            obligations[key] = item
    rules = sorted({rule for package in normalized for rule in package.rules})
    supported = sorted({kind for package in normalized for kind in package.supported_classes})
    unsupported = sorted({str(x.get("kind", x.get("class", ""))) for x in obligations.values() if x.get("kind") and x.get("kind") not in supported})
    payload = {"packages": [asdict(x) for x in normalized], "obligations": list(obligations.values()), "rules": rules, "supported_classes": supported, "conflicts": conflicts, "unsupported": unsupported, "package_errors": sorted(package_errors)}
    payload["hash"] = sha256(json.dumps(payload, sort_keys=True, ensure_ascii=False).encode()).hexdigest()
    payload["status"] = "blocked" if conflicts or unsupported or package_errors else "composed"
    return payload


def load_specialist_profiles(path: str | Path | None = None) -> dict[str, Any]:
    path = Path(path or Path(__file__).with_name("specialist_profiles.v0.5.json"))
    payload = json.loads(path.read_text(encoding="utf-8"))
    return compose_profiles(payload.get("packages", []))


def validate_profile_pin(profile: dict[str, Any], expected_hash: str | None = None) -> list[str]:
    """Validate a composed profile before a workflow can consume it."""
    errors = []
    if profile.get("status") != "composed":
        errors.append("SPECIALIST_PROFILE_COMPOSITION_BLOCKED")
    if not profile.get("hash"):
        errors.append("SPECIALIST_PROFILE_HASH_MISSING")
    if expected_hash and profile.get("hash") != expected_hash:
        errors.append("SPECIALIST_PROFILE_HASH_STALE")
    if not profile.get("rules") and not profile.get("obligations"):
        errors.append("SPECIALIST_PROFILE_EMPTY")
    return sorted(set(errors))


def compose_workflow_profile(profile_id: str, packages: Iterable[ProfilePackage | dict[str, Any]], *, workflow_kind: str, context: dict[str, Any] | None = None) -> dict[str, Any]:
    """Bind specialist obligations to one of the three workflow profiles.

    Specialist packages can add checks and N/A evidence, but the fixed
    document, approval and realization gates are always retained.
    """
    composed = compose_profiles(packages)
    composed.update({"profile_id": profile_id, "workflow_kind": workflow_kind, "mandatory_gates": ["source_roles", "srs_delta", "sad_readonly", "impact_approval", "tdd_realization"] if workflow_kind != "initial" else ["source_roles", "srs", "sad"], "unknown_case_obligations": [x for x in composed.get("obligations", []) if not x.get("validator")], "context_hash": sha256(json.dumps(context or {}, sort_keys=True, ensure_ascii=False).encode()).hexdigest()})
    if workflow_kind not in {"initial", "change-request", "prompt-edit", "spec-change"}:
        composed["status"] = "blocked"; composed.setdefault("package_errors", []).append("WORKFLOW_KIND_UNSUPPORTED")
    return composed


def default_workflow_profile(workflow_kind: str, *, context: dict[str, Any] | None = None) -> dict[str, Any]:
    specialist = load_specialist_profiles()
    return compose_workflow_profile("tdd-" + workflow_kind, specialist.get("packages", []), workflow_kind=workflow_kind, context=context)
