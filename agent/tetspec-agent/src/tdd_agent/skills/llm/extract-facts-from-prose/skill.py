from tdd_agent.skills.base import Skill, SkillResult
from tdd_agent.skills.lib import text as T

SCHEMA = {"type": "object", "required": ["facts"], "additionalProperties": False, "properties": {"facts": {"type": "array", "items": {
    "type": "object", "required": ["block", "class", "name", "props_items", "quote"], "additionalProperties": False,
    "properties": {"block": {"type": "string"}, "class": {"type": "string"}, "name": {"type": "string"}, "quote": {"type": "string"},
    "props_items": {"type": "array", "items": {"type": "object", "required": ["key", "value"], "additionalProperties": False, "properties": {"key": {"type": "string"}, "value": {"type": "string"}}}}}}}}}
BATCH = 25


def props_from_items(items: list[dict]) -> dict:
    """The model lists properties as key/value pairs whose value is JSON text (a schema with an open object is refused by strict providers); the object is rebuilt here."""
    import json
    props = {}
    for item in items:
        raw = item["value"]
        try:
            props[item["key"]] = json.loads(raw)
        except (json.JSONDecodeError, TypeError):
            props[item["key"]] = raw
    return props


class Impl(Skill):
    def run(self, ctx, payload):
        src, spans, allowed = payload["source"], payload["spans"], payload["allowed_classes"]
        by_block = {s["block"]: s for s in spans}
        lines = src["text"].splitlines()
        elements, gaps, rejected, seen = [], [], [], set()
        for i in range(0, len(spans), BATCH):
            chunk = spans[i:i + BATCH]
            listing = "\n".join(f"{s['block']} | ({' > '.join(s.get('heading_path', [])) or 'no heading'}{'; likely class: ' + s['hint'] if s.get('hint') else ''}) {s['text']}" for s in chunk)
            classes = "\n".join(f"- {c}: required properties {sorted(v.get('required', []))}" for c, v in sorted(allowed.items()))
            resp = ctx.llm(f"Source {src['id']} revision {src['revision']}. Candidate lines:\n{listing}\n\nAllowed classes:\n{classes}\n\n"
                           "Return one fact per design fact stated in these lines. Quote the exact words of the line that state it.", SCHEMA)
            if not resp.ok:
                return SkillResult.blocked(*resp.blockers, resolved_by="llm", diagnostics={"batch": i // BATCH})
            for f in resp.parsed["facts"]:
                f["props"] = props_from_items(f.pop("props_items", []))
                errors = []
                f["block"] = f["block"].strip().strip("[]()<> ").lower()  # anchor syntax only; the quote check below still decides
                span = by_block.get(f["block"])
                if span is None:
                    errors.append("BLOCK_NOT_IN_CANDIDATES")
                elif f["quote"].strip() not in lines[span["line"] - 1]:
                    errors.append("QUOTE_NOT_IN_LINE")
                spec = allowed.get(f["class"])
                if spec is None:
                    errors.append("CLASS_NOT_ALLOWED")
                else:
                    missing = [r for r in spec.get("required", []) if f["props"].get(r) in (None, "", [], {})]
                    if missing:
                        errors.append("MISSING_REQUIRED_PROPS:" + ",".join(missing))
                if span is not None:
                    unsupported = [n for n in T.numbers_in_props(f["props"]) if not T.number_supported(lines[span["line"] - 1], n)]
                    if unsupported:
                        errors.append("NUMBER_NOT_IN_QUOTE:" + ",".join(str(n) for n in unsupported))
                if errors:
                    rejected.append({"fact": f, "errors": errors})
                    gaps.append({"id": f"GAP-{f['block']}-{f['class']}", "target": f["block"], "need": "typed_fact", "status": "unknown",
                                 "reason": "model proposal failed verification: " + ", ".join(errors)})
                    continue
                # the id comes from what the document says (its class and the quoted words), never from the model's wording of the name or the line number
                key = T.sha(f"{src['id']}|{f['class']}|{T.normalize_name(f['quote'])}")[:10]
                ident = f"FACT-{key}"
                if ident in seen:
                    continue
                seen.add(ident)
                props = dict(f["props"]); props["evidence_span"] = f["quote"].strip(); props["sourced_props"] = sorted(k for k in f["props"])
                elements.append({"id": ident, "class": f["class"], "name": f["name"], "origin": "upstream", "maturity": "sketch", "owner_doc": src["id"],
                                 "version_introduced": str(src["revision"]), "review_status": "needs_review", "props": props,
                                 "provenance": [T.evidence(src["text"], src["id"], src["revision"], span["line"])]})
        return SkillResult.done({"elements": elements, "gaps": gaps, "rejected": rejected}, resolved_by="llm" if spans else "rule", gaps=gaps)
