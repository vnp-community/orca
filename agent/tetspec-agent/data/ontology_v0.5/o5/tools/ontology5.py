#!/usr/bin/env python3
"""Generate the v0.5 contract artefacts from ontology.json.

The JSON manifest is intentionally the only normative source. Generated files
are disposable and can be reproduced with: python tools/ontology5.py generate.
"""
from __future__ import annotations
import json
from pathlib import Path
import sys
import re

ROOT = Path(__file__).resolve().parents[1]
MANIFEST = ROOT / "ontology.json"

def load():
    return json.loads(MANIFEST.read_text(encoding="utf-8"))

def canonical_property(name: str) -> str:
    """Resolve a legacy property spelling without losing the source name."""
    return load().get("property_aliases", {}).get(name, name)

def canonical_properties(props: dict) -> tuple[dict, dict]:
    """Return canonical properties and an alias audit map.

    Conflicting values are retained in the audit map and are never silently
    selected by the validator.
    """
    result, conflicts = dict(props or {}), {}
    for old, new in load().get("property_aliases", {}).items():
        if old not in result: continue
        if new in result and result[new] != result[old]: conflicts[old] = {"canonical": result[new], "alias": result[old]}
        else: result.setdefault(new, result[old])
    return result, conflicts

def inventory() -> dict:
    """Return the complete canonical inventory and alias audit surface."""
    manifest = load()
    classes = sorted(manifest.get("classes", {}))
    relations = sorted(manifest.get("relations", {}))
    legacy_classes = sorted(legacy_class_names())
    legacy_relations = sorted(legacy_relation_names())
    legacy_properties = legacy_property_inventory()
    canonical_names = {
        prop
        for spec in manifest.get("classes", {}).values()
        for prop in spec.get("required", [])
    }
    legacy_property_names = sorted({
        prop
        for spec in legacy_properties.values()
        for prop in spec.get("effective", [])
    })
    aliases = dict(sorted(manifest.get("property_aliases", {}).items()))
    return {
        "version": manifest["version"],
        "canonical_release": manifest.get("canonical_release", {}),
        "classes": classes,
        "relations": relations,
        "legacy_classes": legacy_classes,
        "legacy_relations": legacy_relations,
        "legacy_properties": legacy_properties,
        "legacy_property_names": legacy_property_names,
        "canonical_property_names": sorted(canonical_names),
        "property_aliases": aliases,
        "canonical_alias_map": {
            old: {"canonical": new, "status": "accepted"}
            for old, new in aliases.items()
        },
        "typed_contracts": manifest.get("typed_contracts", {}),
        "legacy_required_properties": manifest.get("legacy_required_properties", {}),
        "profiles": sorted(manifest.get("profiles", {})),
        "required_properties": {name: list(spec.get("required", [])) for name, spec in sorted(manifest.get("classes", {}).items())},
        "maturity_policy": manifest.get("maturity_policy", {}),
        "conditional_requirements": manifest.get("conditional_requirements", []),
        "parity": {
            "legacy_class_count": len(legacy_classes),
            "canonical_class_count": len(classes),
            "legacy_relation_count": len(legacy_relations),
            "canonical_relation_count": len(relations),
            "legacy_property_count": len(legacy_property_names),
            "canonical_property_count": len(canonical_names),
            "unmapped_legacy_classes": sorted(set(legacy_classes) - set(classes)),
            "passthrough_legacy_relations": sorted(set(legacy_relations) - set(relations)),
            "unmapped_legacy_properties": sorted(set(legacy_property_names) - canonical_names - set(aliases)),
        },
    }


def resolved_required(class_name: str) -> list[str]:
    """Resolve manifest inheritance without losing the concrete class."""
    manifest = load(); seen = set(); result = []
    current = class_name
    while current and current not in seen:
        seen.add(current)
        spec = manifest.get("classes", {}).get(current, {})
        for prop in spec.get("required", []):
            if prop not in result: result.append(prop)
        current = spec.get("extends")
    for prop in load().get("legacy_required_properties", {}).get(class_name, []):
        if prop not in result:
            result.append(prop)
    return result


def conditional_required(element: dict) -> list[str]:
    """Return fields required by predicates that match this element."""
    manifest = load(); props = element.get("props", {}) or {}; result = []
    for rule in manifest.get("conditional_requirements", []):
        if rule.get("class") != element.get("class"): continue
        condition = rule.get("when", {}); value = props.get(condition.get("property"))
        matched = True
        if "equals" in condition: matched = value == condition["equals"]
        if "in" in condition: matched = value in condition["in"]
        if matched:
            result.extend(rule.get("required", []))
    return sorted(set(result))


def manifest_errors() -> list[str]:
    """Validate the canonical registry itself before graph validation."""
    manifest = load(); errors = []
    aliases = manifest.get("property_aliases", {})
    for old, new in aliases.items():
        if old == new: errors.append("ALIAS_SELF_REFERENCE:" + old)
        if new not in {p for spec in manifest.get("classes", {}).values() for p in spec.get("required", [])} and new not in {"path", "auth", "concurrency", "measurement"}:
            errors.append("ALIAS_TARGET_UNDECLARED:" + old + ":" + new)
    for name, spec in manifest.get("classes", {}).items():
        if not spec.get("required") and name not in {"Requirement", "Scenario", "WorkItem", "TestCase"}:
            errors.append("CLASS_WITHOUT_REQUIRED_PROPERTIES:" + name)
        if spec.get("extends") and spec["extends"] not in set(manifest.get("classes", {})) | legacy_class_names():
            errors.append("UNKNOWN_BASE_CLASS:" + name + ":" + str(spec["extends"]))
    for name, required in manifest.get("legacy_required_properties", {}).items():
        if name not in legacy_class_names():
            errors.append("UNKNOWN_LEGACY_REQUIRED_CLASS:" + name)
        if not isinstance(required, list) or not all(isinstance(prop, str) and prop for prop in required):
            errors.append("INVALID_LEGACY_REQUIRED_PROPERTIES:" + name)
    maturity = manifest.get("maturity_policy", {})
    values = set(maturity.get("values", [])); requirements = maturity.get("requirements", {})
    if not values or set(requirements) != values:
        errors.append("MATURITY_POLICY_INCOMPLETE")
    for rule in manifest.get("conditional_requirements", []):
        if rule.get("class") not in set(manifest.get("classes", {})) | legacy_class_names(): errors.append("UNKNOWN_CONDITIONAL_CLASS:" + str(rule.get("id")))
        if not rule.get("property") and not rule.get("when", {}).get("property"): errors.append("CONDITIONAL_PROPERTY_MISSING:" + str(rule.get("id")))
    for name, spec in manifest.get("relations", {}).items():
        if spec.get("inverse") and spec["inverse"] not in manifest.get("relations", {}) and spec["inverse"] not in legacy_relation_names() and not manifest.get("base", {}).get("legacy_relations_passthrough"):
            errors.append("UNKNOWN_INVERSE:" + name + ":" + str(spec["inverse"]))
    return sorted(set(errors))

def legacy_relation_names():
    """Read relation names from the frozen v0.4 YAML without requiring PyYAML."""
    source = (ROOT / load()["base"]["relations_source"]).resolve()
    text = source.read_text(encoding="utf-8")
    section = text.split("\nrelations:", 1)[-1]
    return {m.group(1) for m in re.finditer(r"^  ([A-Za-z][A-Za-z0-9_]*)", section, re.MULTILINE)}

def legacy_relation_specs():
    text=(ROOT/load()["base"]["relations_source"]).resolve().read_text().split("\nrelations:",1)[-1]
    result={};current=None;field=None
    for line in text.splitlines():
        match=re.match(r"^  ([A-Za-z][A-Za-z0-9_]*):",line)
        if match:current=match[1];result[current]={};field=None;continue
        match=re.match(r"^    (domain|range):\s*(.*)$",line)
        if match and current:
            field=match[1];result[current][field]=match[2] or [];continue
        match=re.match(r"^    - ([A-Za-z][A-Za-z0-9_]*)$",line)
        if match and current and field and isinstance(result[current][field],list):result[current][field].append(match[1]);continue
        if re.match(r"^    [A-Za-z]",line):field=None
    return result


def legacy_class_names():
    source = (ROOT / load()["base"]["classes_source"]).resolve()
    text = source.read_text(encoding="utf-8")
    section = text.split("\nclasses:", 1)[-1]
    return {m.group(1) for m in re.finditer(r"^  ([A-Za-z][A-Za-z0-9_]*)", section, re.MULTILINE)}


def legacy_property_inventory() -> dict:
    """Parse the frozen v0.4 class/property contract without mutating it.

    The v0.4 source is YAML, but the runtime deliberately has no YAML
    dependency.  This parser reads the stable class/own_props subset and
    resolves inherited properties for inventory and migration audits.
    """
    source = (ROOT / load()["base"]["classes_source"]).resolve()
    section = source.read_text(encoding="utf-8").split("\nclasses:", 1)[-1]
    lines = section.splitlines()
    parsed = {}
    current = None
    in_props = False
    prop = None
    for line in lines:
        class_match = re.match(r"^  ([A-Za-z][A-Za-z0-9_]*):\s*$", line)
        if class_match:
            current = class_match.group(1)
            parsed[current] = {"parent": None, "own": []}
            in_props = False
            prop = None
            continue
        if current is None:
            continue
        parent_match = re.match(r"^    parent:\s*([A-Za-z][A-Za-z0-9_]*)", line)
        if parent_match:
            parsed[current]["parent"] = parent_match.group(1)
            continue
        if re.match(r"^    own_props:\s*\[\]", line):
            in_props = False
            prop = None
            continue
        if re.match(r"^    own_props:\s*$", line):
            in_props = True
            prop = None
            continue
        if in_props:
            name_match = re.match(r"^    - name:\s*([^\s#]+)", line)
            if name_match:
                prop = {"name": name_match.group(1)}
                parsed[current]["own"].append(prop)
                continue
            if prop is not None:
                type_match = re.match(r"^      type:\s*(.*?)\s*$", line)
                level_match = re.match(r"^      level:\s*([A-Z])\s*$", line)
                if type_match:
                    prop["type"] = type_match.group(1).strip("'\"")
                elif level_match:
                    prop["level"] = level_match.group(1)
    def effective(name, seen=None):
        seen = set() if seen is None else seen
        if name in seen or name not in parsed:
            return []
        seen.add(name)
        parent = parsed[name].get("parent")
        result = effective(parent, seen) if parent else []
        result.extend(p["name"] for p in parsed[name]["own"] if p["name"] not in result)
        return result
    return {
        name: {
            "parent": spec["parent"],
            "own": spec["own"],
            "effective": effective(name),
        }
        for name, spec in sorted(parsed.items())
    }

def schema_for(name, required, version=None):
    version = version or load()["version"]
    props = {"id": {"type": "string"}, "class": {"const": name}, "name": {"type": "string"}}
    for p in required:
        props.setdefault(p, {"type": ["string", "number", "boolean", "object", "array", "null"]})
    return {"$schema": "https://json-schema.org/draft/2020-12/schema", "$id": f"https://example.org/tdd/schema/{name}-v{version.rsplit('.', 1)[0]}", "title": name, "type": "object", "additionalProperties": True, "properties": props, "required": sorted(set(["id", "class", "name", *required]))}

def generate():
    m = load()
    schemas = ROOT / "schemas"
    schemas.mkdir(exist_ok=True)
    release = m["version"]
    compatibility = list(m.get("base", {}).get("compatibility_versions", []))
    for name, spec in m["classes"].items():
        (schemas / f"{name}.schema.json").write_text(json.dumps(schema_for(name, spec["required"], release), indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    for old_version in compatibility:
        compat_dir = schemas / ("compat-v" + old_version.rsplit('.', 1)[0])
        compat_dir.mkdir(exist_ok=True)
        for name, spec in m["classes"].items():
            (compat_dir / f"{name}.schema.json").write_text(json.dumps(schema_for(name, spec["required"], old_version), indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    (ROOT / f"classes.v{release.rsplit('.', 1)[0]}.json").write_text(json.dumps({"version": release, "classes": m["classes"]}, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    (ROOT / f"relations.v{release.rsplit('.', 1)[0]}.json").write_text(json.dumps({"version": release, "relations": m["relations"]}, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    (ROOT / f"profiles.v{release.rsplit('.', 1)[0]}.json").write_text(json.dumps({"version": release, "profiles": m["profiles"]}, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    current_inventory = inventory()
    (ROOT / f"inventory.v{release.rsplit('.', 1)[0]}.json").write_text(json.dumps(current_inventory, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    for old_version in compatibility:
        old_slug = old_version.rsplit('.', 1)[0]
        compat_payload = {"version": old_version, "canonical_version": release, "classes": m["classes"]}
        (ROOT / f"classes.v{old_slug}.json").write_text(json.dumps(compat_payload, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
        (ROOT / f"relations.v{old_slug}.json").write_text(json.dumps({"version": old_version, "canonical_version": release, "relations": m["relations"]}, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
        (ROOT / f"profiles.v{old_slug}.json").write_text(json.dumps({"version": old_version, "canonical_version": release, "profiles": m["profiles"]}, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
        compat_inventory = dict(current_inventory); compat_inventory["version"] = old_version; compat_inventory["canonical_version"] = release
        (ROOT / f"inventory.v{old_slug}.json").write_text(json.dumps(compat_inventory, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    owl = ["@prefix tdd: <https://example.org/tdd#> .", "@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .", f"# Generated from ontology.json {release}"]
    for c,spec in m["classes"].items():
        owl.append(f"tdd:{c} a rdfs:Class .")
        if spec.get("extends"): owl.append(f"tdd:{c} rdfs:subClassOf tdd:{spec['extends']} .")
    for r, spec in m["relations"].items():
        owl.extend([f"tdd:{r} a rdfs:Property .", f"tdd:{r} tdd:cardinality \"{spec['cardinality']}\" ."])
    (ROOT / f"tdd-ontology-v{release.rsplit('.', 1)[0]}.ttl").write_text("\n".join(owl) + "\n", encoding="utf-8")
    for old_version in compatibility:
        (ROOT / f"tdd-ontology-v{old_version.rsplit('.', 1)[0]}.ttl").write_text("\n".join(owl + [f"# Compatibility alias for {old_version}"]) + "\n", encoding="utf-8")
    shapes = ["@prefix sh: <http://www.w3.org/ns/shacl#> .", "@prefix tdd: <https://example.org/tdd#> .", f"# Generated from ontology.json {release}"]
    for c, spec in m["classes"].items():
        shapes.append(f"tdd:{c}Shape a sh:NodeShape ; sh:targetClass tdd:{c} .")
        for p in spec["required"]: shapes.append(f"tdd:{c}Shape sh:property [ sh:path tdd:{p} ; sh:minCount 1 ] .")
    (ROOT / f"tdd-shapes-v{release.rsplit('.', 1)[0]}.ttl").write_text("\n".join(shapes) + "\n", encoding="utf-8")
    for old_version in compatibility:
        (ROOT / f"tdd-shapes-v{old_version.rsplit('.', 1)[0]}.ttl").write_text("\n".join(shapes + [f"# Compatibility alias for {old_version}"]) + "\n", encoding="utf-8")
    from quality_contract import generate as generate_quality
    generate_quality(schemas)
    return sorted(str(p.relative_to(ROOT)) for p in schemas.glob("*.schema.json"))

if __name__ == "__main__":
    if len(sys.argv) != 2 or sys.argv[1] != "generate":
        raise SystemExit("usage: ontology5.py generate")
    print(json.dumps({"version": load()["version"], "generated_schemas": generate()}, ensure_ascii=False))
