"""Declared tier of every pipeline step (design D12).

Each step of the four agents is listed here with its tier.  Non-logic steps carry
the reason they cannot be rules and a demotion note.  Agents run logic steps
through :func:`run_logic_step`, which forbids LLM calls for its duration, so a
step declared ``logic`` cannot silently start using a model.
"""
from __future__ import annotations

from contextlib import contextmanager
from dataclasses import dataclass
from typing import Callable, Iterator, TypeVar

from ..errors import TddAgentError
from ..obs import context
from ..obs.observer import Observer
from .base import SkillRegistry

T = TypeVar("T")


@dataclass(frozen=True)
class Step:
    id: str
    agent: str
    tier: str
    skill: str | None = None
    reason: str = ""
    demotion: str = ""


def _s(id, agent, tier, skill=None, reason="", demotion=""):
    return Step(id, agent, tier, skill, reason, demotion)


STEPS: dict[str, Step] = {s.id: s for s in (
    # generation
    _s("gen.pin-sources", "generation", "logic"), _s("gen.resolve-needs", "generation", "logic"),
    _s("gen.segment", "generation", "logic", "segment-and-anchor"), _s("gen.extract-structured", "generation", "logic", "extract-structured-facts"),
    _s("gen.select-prose", "generation", "logic", "select-prose-candidates"),
    _s("gen.extract-prose", "generation", "llm", "extract-facts-from-prose", "free-form sentences have no fixed structure", "promote repeated sentence patterns to extract-structured-facts"),
    _s("gen.map-class", "generation", "hybrid", "map-candidate-to-class", "headings are not a closed vocabulary", "heading/keyword rule table grows from logged LLM choices"),
    _s("gen.match-entities", "generation", "hybrid", "resolve-entity-match", "synonyms cannot be fully normalised", "alias table"),
    _s("gen.verify-evidence", "generation", "logic", "verify-evidence"), _s("gen.build-changeset", "generation", "logic"),
    _s("gen.validate-readiness", "generation", "logic"), _s("gen.render", "generation", "logic"), _s("gen.gap-questions", "generation", "logic"), _s("gen.defect-record", "generation", "logic"),
    # impact
    _s("imp.cr-to-srs-edits", "impact", "llm", "propose-srs-edits-from-cr", "which requirements a free-text change request touches and how they are reworded",
       "accept the CR as structured rows for the common cases; rank candidate requirements by id mention and word overlap before asking"),
    _s("imp.derive-delta", "impact", "logic", "derive-srs-delta"), _s("imp.candidates-q1", "impact", "logic"),
    _s("imp.scan-inventory", "impact", "logic", "scan-inventory-overlap"),
    _s("imp.judge-coverage", "impact", "hybrid", "judge-design-coverage", "semantic coverage of reworded requirements", "require trace edges so fewer TDDs are residue"),
    _s("imp.sad-conflict", "impact", "hybrid", "detect-sad-conflict", "prose architecture constraints", "type SAD constraints in the KG"),
    _s("imp.assemble-report", "impact", "logic"), _s("imp.verify-evidence", "impact", "logic", "verify-evidence"),
    # change spec
    _s("cs.build-table", "changespec", "logic", "build-cs-table"), _s("cs.verification", "changespec", "logic", "derive-verification"),
    _s("cs.migration-ownership", "changespec", "logic"),
    _s("cs.code-target", "changespec", "hybrid", "select-code-target", "several symbols can plausibly implement a requirement", "ids in code comments / mapping file"),
    _s("cs.propose-acceptance", "changespec", "llm", "propose-acceptance-from-cr", "turning a prose request into testable criteria when nobody wrote them", "the CR template names the acceptance criteria"),
    _s("cs.propose-satisfier", "changespec", "llm", "propose-satisfying-element", "matching a requirement with the component responsible for it", "a row of the SRS naming the component; change update --satisfied-by"),
    _s("cs.project-openspec", "changespec", "logic"),
    _s("cs.author-prose", "changespec", "llm", "author-spec-prose", "prose generation", "move recurring prose into the projection template"),
    _s("cs.critique", "changespec", "llm", "critique-spec", "judging faithfulness of prose", "add each recurring finding as a deterministic check"),
    _s("cs.validate-publish", "changespec", "logic"),
    # prompt edit
    _s("pe.write-section", "promptedit", "llm", "write-section", "turning a free request into well-formed section text", "a structured form (section, text) for plain insertions"),
    _s("pe.parse-intent", "promptedit", "llm", "parse-edit-intent", "understanding natural language", "structured edit form for frequent operations"),
    _s("pe.classify", "promptedit", "logic", "classify-edit-from-intent"),
    _s("pe.resolve-target", "promptedit", "hybrid", "disambiguate-target", "informal names match several elements", "alias table"),
    _s("pe.authorize-conflict-version", "promptedit", "logic"), _s("pe.plan-commit-undo", "promptedit", "logic"), _s("pe.clarification", "promptedit", "logic"),
    # orchestration (always rules)
    _s("orc.gates-approvals-ledger", "orchestrator", "logic"), _s("orc.retrieval-closure", "orchestrator", "logic"),
)}

# Work that must never be given to a model.
ALWAYS_LOGIC = ("gen.pin-sources", "gen.resolve-needs", "gen.segment", "gen.extract-structured", "gen.verify-evidence", "gen.build-changeset", "gen.validate-readiness",
                "gen.render", "gen.defect-record", "imp.candidates-q1", "imp.scan-inventory", "imp.assemble-report", "cs.build-table", "cs.verification", "cs.project-openspec", "cs.validate-publish",
                "pe.classify", "pe.authorize-conflict-version", "pe.plan-commit-undo", "orc.gates-approvals-ledger", "orc.retrieval-closure")


def validate_steps(registry: SkillRegistry) -> list[str]:
    errors: list[str] = []
    for s in STEPS.values():
        if s.tier not in ("logic", "hybrid", "llm"):
            errors.append(f"STEP_TIER_INVALID:{s.id}")
        if s.tier != "logic" and (not s.reason.strip() or not s.demotion.strip()):
            errors.append(f"STEP_REASON_OR_DEMOTION_MISSING:{s.id}")
        if s.skill:
            try:
                skill = registry.get(s.skill)
            except TddAgentError:
                errors.append(f"STEP_SKILL_UNKNOWN:{s.id}:{s.skill}")
                continue
            if skill.meta.tier != s.tier:
                errors.append(f"STEP_TIER_MISMATCH:{s.id}:{s.tier}!={skill.meta.tier}")
    errors += [f"ALWAYS_LOGIC_NOT_LOGIC:{i}" for i in ALWAYS_LOGIC if STEPS.get(i) is None or STEPS[i].tier != "logic"]
    return errors


@contextmanager
def logic_step(observer: Observer, step_id: str) -> Iterator[None]:
    """Run a rule-only step: declared, traced, and any LLM call inside it is an error."""
    step = STEPS.get(step_id)
    if step is None:
        raise TddAgentError(f"step {step_id!r} is not declared in STEPS", code="STEP_UNDECLARED")
    if step.tier != "logic":
        raise TddAgentError(f"step {step_id!r} is {step.tier}, not logic", code="STEP_NOT_LOGIC")
    with observer.bind(step=step_id, llm_forbidden=True):
        with observer.span(step_id, "step", tier="logic"):
            yield


def run_logic_step(observer: Observer, step_id: str, fn: Callable[[], T]) -> T:
    with logic_step(observer, step_id):
        return fn()
