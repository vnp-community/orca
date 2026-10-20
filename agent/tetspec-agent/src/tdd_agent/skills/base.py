"""Skill framework (design D12).

A skill is one step of an agent, declared as ``logic`` (code/rules only),
``hybrid`` (rules first; an LLM only for the residue, on a narrow typed input) or
``llm`` (open-ended language).  Every skill lives in a package directory::

    skills/<tier>/<skill_id>/skill.toml        id, version, agent, tier, reason, demotion, verifiers, tools
                              schema.json        {"input": {...}, "output": {...}} JSON Schemas
                              instructions.md    (hybrid/llm) provider-independent instructions
                              examples.jsonl     regression examples (run against a real provider for hybrid/llm)
                              skill.py           ``class Impl(Skill)``

The package hash (all files) is part of every LLM audit record and of the input
hash of the step that uses the skill, so changing instructions makes the step stale.
Registration refuses ``hybrid``/``llm`` skills without a reason and a demotion note.
"""
from __future__ import annotations

import importlib.util
import json
import tomllib
from dataclasses import dataclass, field
from hashlib import sha256
from pathlib import Path
from typing import Any

import jsonschema

from ..errors import ConfigError, TddAgentError
from ..llm.base import LlmRequest, LlmResponse
from ..obs.observer import Observer
from ..store.base import digest

TIERS = ("logic", "hybrid", "llm")
RESOLVED_BY = ("rule", "llm", "human")
PACKAGE_ROOT = Path(__file__).resolve().parent


class SkillError(TddAgentError):
    code = "SKILL_ERROR"


@dataclass(frozen=True)
class SkillMeta:
    id: str
    version: str
    agent: str
    tier: str
    description: str = ""
    reason: str = ""
    demotion: str = ""
    verifiers: tuple[str, ...] = ()
    tools: tuple[str, ...] = ()
    role: str = "extractor"

    def validate(self) -> list[str]:
        errors = []
        if self.tier not in TIERS:
            errors.append(f"SKILL_TIER_INVALID:{self.id}")
        if self.tier in ("hybrid", "llm") and not self.reason.strip():
            errors.append(f"SKILL_REASON_MISSING:{self.id}")
        if self.tier in ("hybrid", "llm") and not self.demotion.strip():
            errors.append(f"SKILL_DEMOTION_NOTE_MISSING:{self.id}")
        if self.tier in ("hybrid", "llm") and not self.verifiers:
            errors.append(f"SKILL_VERIFIERS_MISSING:{self.id}")
        return errors


@dataclass
class SkillResult:
    ok: bool
    output: dict = field(default_factory=dict)
    resolved_by: str = "rule"
    blockers: list[str] = field(default_factory=list)
    gaps: list[dict] = field(default_factory=list)
    diagnostics: dict = field(default_factory=dict)

    @staticmethod
    def done(output: dict, *, resolved_by: str = "rule", gaps: list[dict] | None = None, diagnostics: dict | None = None) -> "SkillResult":
        return SkillResult(True, output, resolved_by, [], gaps or [], diagnostics or {})

    @staticmethod
    def blocked(*blockers: str, resolved_by: str = "rule", diagnostics: dict | None = None, gaps: list[dict] | None = None) -> "SkillResult":
        return SkillResult(False, {}, resolved_by, list(dict.fromkeys(blockers)), gaps or [], diagnostics or {})


class SkillContext:
    """What a skill may use: the LLM gateway (through audit), other skills and the observer."""

    def __init__(self, runner: "SkillRunner", skill: "Skill", extras: dict | None = None):
        self.runner, self.skill = runner, skill
        self.observer: Observer = runner.observer
        self.extras = dict(extras or {})
        self.llm_calls = 0
        self.pattern: str | None = None

    @property
    def meta(self) -> SkillMeta:
        return self.skill.meta

    def skill_ref(self) -> dict:
        ref = {"id": self.meta.id, "version": self.meta.version, "tier": self.meta.tier, "package_hash": self.skill.package_hash}
        if self.pattern:
            ref["pattern"] = self.pattern
        return ref

    def llm(self, prompt: str, schema: dict, *, system: str | None = None, role: str | None = None, timeout_s: int | None = None, pattern: str | None = None) -> LlmResponse:
        """One audited LLM call with the skill's own instructions as the system prompt.

        ``pattern`` is a short, payload-free description of the *kind* of input (for example a normalised heading);
        the demotion report groups identical patterns that always get the same answer to propose new rules.
        """
        self.pattern = pattern
        if self.meta.tier == "logic":
            raise SkillError(f"logic skill {self.meta.id} must not call an LLM", code="LOGIC_SKILL_LLM_CALL")
        gateway = self.runner.gateway
        if gateway is None:
            raise SkillError("no LLM gateway configured for this runner", code="NO_GATEWAY")
        self.llm_calls += 1
        request = LlmRequest(role=role or self.meta.role, prompt=prompt, system=system or self.skill.instructions or None,
                             output_schema=schema, timeout_s=timeout_s, metadata={"skill": self.meta.id})
        return gateway.call(request, skill=self.skill_ref())

    def call(self, skill_id: str, payload: dict) -> SkillResult:
        return self.runner.run(skill_id, payload, extras=self.extras)


class Skill:
    meta: SkillMeta
    schema: dict
    instructions: str
    package_hash: str

    def run(self, ctx: SkillContext, payload: dict) -> SkillResult:  # pragma: no cover - abstract
        raise NotImplementedError


def _load_package(directory: Path) -> Skill:
    manifest = tomllib.loads((directory / "skill.toml").read_text(encoding="utf-8"))
    meta = SkillMeta(id=manifest["id"], version=str(manifest["version"]), agent=manifest["agent"], tier=manifest["tier"],
                     description=manifest.get("description", ""), reason=manifest.get("reason", ""), demotion=manifest.get("demotion", ""),
                     verifiers=tuple(manifest.get("verifiers", ())), tools=tuple(manifest.get("tools", ())), role=manifest.get("role", "extractor"))
    problems = meta.validate()
    if problems:
        raise ConfigError(f"invalid skill package {directory.name}: {problems}", code="SKILL_PACKAGE_INVALID")
    if directory.name != meta.id or directory.parent.name != meta.tier:
        raise ConfigError(f"skill {meta.id} must live in skills/{meta.tier}/{meta.id}", code="SKILL_PACKAGE_INVALID")
    spec = importlib.util.spec_from_file_location(f"tdd_agent_skill_{meta.id.replace('-', '_')}", directory / "skill.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)  # type: ignore[union-attr]
    skill: Skill = module.Impl()
    skill.meta = meta
    skill.schema = json.loads((directory / "schema.json").read_text(encoding="utf-8"))
    instr = directory / "instructions.md"
    skill.instructions = instr.read_text(encoding="utf-8") if instr.is_file() else ""
    if meta.tier in ("hybrid", "llm") and not skill.instructions.strip():
        raise ConfigError(f"skill {meta.id} ({meta.tier}) needs instructions.md", code="SKILL_PACKAGE_INVALID")
    h = sha256()
    for name in sorted(p.name for p in directory.iterdir() if p.is_file() and p.suffix in (".toml", ".json", ".md", ".jsonl", ".py")):
        h.update(name.encode()); h.update((directory / name).read_bytes())
    skill.package_hash = h.hexdigest()
    return skill


class SkillRegistry:
    def __init__(self, skills: dict[str, Skill]):
        self._skills = skills

    @classmethod
    def load(cls, root: Path | None = None) -> "SkillRegistry":
        root = root or PACKAGE_ROOT
        found: dict[str, Skill] = {}
        for tier in TIERS:
            base = root / tier
            if not base.is_dir():
                continue
            for directory in sorted(p for p in base.iterdir() if (p / "skill.toml").is_file()):
                skill = _load_package(directory)
                if skill.meta.id in found:
                    raise ConfigError(f"duplicate skill id {skill.meta.id}", code="SKILL_PACKAGE_INVALID")
                found[skill.meta.id] = skill
        return cls(found)

    def get(self, skill_id: str) -> Skill:
        if skill_id not in self._skills:
            raise ConfigError(f"skill {skill_id!r} is not registered (have {sorted(self._skills)})", code="SKILL_UNKNOWN")
        return self._skills[skill_id]

    def all(self) -> list[Skill]:
        return [self._skills[k] for k in sorted(self._skills)]

    def for_agent(self, agent: str) -> list[Skill]:
        return [s for s in self.all() if s.meta.agent == agent]

    def fingerprint(self, ids: list[str] | None = None) -> str:
        return digest({s.meta.id: s.package_hash for s in self.all() if ids is None or s.meta.id in ids})


class SkillRunner:
    def __init__(self, registry: SkillRegistry, observer: Observer, gateway=None):
        self.registry, self.observer, self.gateway = registry, observer, gateway

    def run(self, skill_id: str, payload: dict, *, extras: dict | None = None) -> SkillResult:
        skill = self.registry.get(skill_id)
        ctx = SkillContext(self, skill, extras)
        with self.observer.span(f"skill:{skill_id}", "step", skill=skill_id, tier=skill.meta.tier, version=skill.meta.version) as span:
            errors = self._schema_errors(skill, "input", payload)
            if errors:
                span.block("SKILL_INPUT_INVALID")
                return SkillResult.blocked("SKILL_INPUT_INVALID", diagnostics={"errors": errors})
            result = skill.run(ctx, payload)
            if result.ok:
                errors = self._schema_errors(skill, "output", result.output)
                if errors:
                    span.block("SKILL_OUTPUT_INVALID")
                    return SkillResult.blocked("SKILL_OUTPUT_INVALID", diagnostics={"errors": errors, "skill": skill_id})
            if skill.meta.tier == "logic" and (ctx.llm_calls or result.resolved_by != "rule"):
                raise SkillError(f"logic skill {skill_id} reported a non-rule resolution", code="LOGIC_SKILL_NOT_RULE")
            if result.resolved_by == "llm" and ctx.llm_calls == 0:
                raise SkillError(f"skill {skill_id} claims an LLM resolution without an LLM call", code="RESOLUTION_MISMATCH")
            self.observer.metrics.skill_resolution(skill.meta.agent, skill_id, result.resolved_by)
            span.set(resolved_by=result.resolved_by, llm_calls=ctx.llm_calls, gaps=len(result.gaps))
            if not result.ok:
                span.block(*result.blockers)
            return result

    @staticmethod
    def _schema_errors(skill: Skill, part: str, value: Any) -> list[str]:
        schema = skill.schema.get(part)
        if not schema:
            return []
        validator = jsonschema.Draft202012Validator(schema)
        return [f"{'/'.join(str(p) for p in e.absolute_path) or '$'}: {e.message[:160]}" for e in list(validator.iter_errors(value))[:6]]
