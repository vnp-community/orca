import json
import re
from pathlib import Path

from tdd_agent.core import o5_path
from tdd_agent.skills.base import Skill, SkillResult

REQUIRED = ("proposal.md", "design.md", "tasks.md")


def output_schema():
    """The o5 authoring schema with one change: the model names a line by its exact text, never by a number.

    Counting lines is logic; models miscount.  The skill turns ``line_text`` into the ``line:N`` anchors o5 verifies.
    """
    import copy
    from authoring import OUTPUT_SCHEMA  # o5 specgen schema (artifacts/assertions/proposed_decisions)
    schema = copy.deepcopy(OUTPUT_SCHEMA)
    item = schema["properties"]["assertions"]["items"]
    item["properties"] = {"artifact": {"type": "string"}, "line_text": {"type": "string", "minLength": 1}, "references": item["properties"]["references"]}
    item["required"] = ["artifact", "line_text", "references"]
    return schema


def anchors_from_text(artifacts: dict[str, str], assertions: list[dict]) -> tuple[list[dict], list[str]]:
    out, errors, seen = [], [], set()
    for a in assertions:
        lines = artifacts[a["artifact"]].splitlines()
        wanted = a["line_text"].strip()
        hits = [i for i, line in enumerate(lines, 1) if line.strip() == wanted]
        if not hits:
            errors.append(f"ASSERTION_TEXT_NOT_FOUND:{a['artifact']}")
            continue
        for i in hits:  # a repeated line is supported by the same references wherever it occurs
            key = (a["artifact"], i, tuple(a["references"]))
            if key not in seen:
                seen.add(key)
                out.append({"artifact": a["artifact"], "anchor": f"line:{i}", "references": list(a["references"])})
    out += _label_assertions(artifacts, out)
    return out, errors


LABEL_ONLY = re.compile(r"^\s*(?:[-*]\s+)?\*\*[^*]+\*\*:?\s*$|^\s*(?:[-*]\s+)?\*\*[^*]+:\*\*\s*$")


def _label_assertions(artifacts: dict[str, str], asserted: list[dict]) -> list[dict]:
    """A bare label such as ``**Goals:**`` states nothing; it inherits the references of the first asserted line below it (rule, not model)."""
    by_line = {(a["artifact"], int(a["anchor"].split(":")[1])): a["references"] for a in asserted}
    extra = []
    for path, text in artifacts.items():
        lines = text.splitlines()
        for i, line in enumerate(lines, 1):
            if (path, i) in by_line or not LABEL_ONLY.match(line):
                continue
            nxt = next((by_line[(path, j)] for j in range(i + 1, len(lines) + 1) if (path, j) in by_line), None)
            if nxt:
                extra.append({"artifact": path, "anchor": f"line:{i}", "references": list(nxt)})
    return extra


class Impl(Skill):
    def run(self, ctx, payload):
        o5_path.o5_root()
        schema = output_schema()
        skill_text = payload["instructions"].get("skill", "")
        prompt = ("Execute the openspec-propose authoring workflow using ONLY the pinned context below. Do not edit files, run commands, implement code or change the KG.\n"
                  "Follow the artifact instructions and templates exactly. Every non-empty substantive line of every artifact (all lines except headings starting with `#`, quotes starting with `>` and table separators) must have an assertion that maps the artifact and its `line:N` anchor to references. "
                  "A reference is exactly one of `kg:<element id from pack.graph.elements>`, `repo:<relative path present in pack.repo.contents>` or `decision:<id of one of your proposed_decisions>`; any other form (bare ids, `cs/...`, `SRS:...`) is rejected. "
                  "A purely editorial line beginning `Draft note:` may omit an assertion; if it states a normative rule, unknown/unresolved obligation, TODO/TBD or pending decision it must be asserted. "
                  "Anything the pack does not support must be written as an open question or a proposed decision, never as fact. Do not write numeric limits, status codes or paths unless they occur in the referenced pack facts.\n\n"
                  f"## Skill instructions\n{skill_text}\n\n## Artifact instructions\n{json.dumps({k: v for k, v in payload['instructions'].items() if k not in ('skill', 'skill_hash')}, ensure_ascii=False)}\n\n"
                  f"## Pinned pack\n{json.dumps(payload['pack'], ensure_ascii=False, sort_keys=True)}")
        resp = ctx.llm(prompt, schema, timeout_s=ctx.extras.get("authoring_timeout_s"))
        if not resp.ok:
            return SkillResult.blocked(*resp.blockers, resolved_by="llm")
        authored = resp.parsed
        from authoring import validate_relative_authoring_paths
        paths = {a["path"] for a in authored["artifacts"]}
        contents = {a["path"]: a["content"] for a in authored["artifacts"]}
        errors = [f"MISSING_ARTIFACT:{r}" for r in REQUIRED if r not in paths]
        if not any(p.startswith("specs/") and p.endswith("/spec.md") for p in paths):
            errors.append("MISSING_ARTIFACT:specs/*/spec.md")
        errors += [f"ASSERTION_ARTIFACT_MISSING:{a['artifact']}" for a in authored["assertions"] if a["artifact"] not in paths]
        known = [a for a in authored["assertions"] if a["artifact"] in paths]
        assertions, text_errors = anchors_from_text(contents, known)
        errors += text_errors
        errors += validate_relative_authoring_paths({**authored, "assertions": assertions})
        if errors:
            return SkillResult.blocked(*sorted(set(errors)), resolved_by="llm")
        return SkillResult.done({"artifacts": contents, "assertions": assertions,
                                 "proposed_decisions": authored["proposed_decisions"]}, resolved_by="llm")
