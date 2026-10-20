"""The tso.yaml ontology definition, its loader and the code generators (DDL, JSON Schema, shape rules)."""
import hashlib
import os
import re
from dataclasses import dataclass, field

import yaml

from ..errors import Diagnostic, InputError

YAML_PATH = os.path.join(os.path.dirname(os.path.abspath(__file__)), "tso.yaml")
TYPES = ("text", "int", "real", "bool", "json")
CARDS = ("!", "?", "+", "*")


def snake(name):
    return re.sub(r"(?<!^)(?=[A-Z])", "_", name).lower()


def table_of(cls):
    return "tso_" + snake(cls)


def join_table(cls, rel):
    return f"{table_of(cls)}__{snake(rel)}"


def fk_column(rel):
    return f"{snake(rel)}_node_id"


@dataclass
class FieldDef:
    name: str
    type: str
    card: str
    llm: bool = False
    enum: str = ""

    @property
    def column(self):
        return snake(self.name)

    @property
    def required(self):
        return self.card in ("!", "+")


@dataclass
class RelDef:
    name: str
    to: str
    card: str

    @property
    def many(self):
        return self.card in ("+", "*")

    @property
    def required(self):
        return self.card in ("!", "+")


@dataclass
class ClassDef:
    name: str
    layer: str
    doc: str
    fields: dict = field(default_factory=dict)
    relations: dict = field(default_factory=dict)

    @property
    def table(self):
        return table_of(self.name)

    def llm_fields(self):
        return [f for f in self.fields.values() if f.llm]


@dataclass
class Ontology:
    version: int
    enums: dict
    classes: dict
    source: dict
    ops: dict
    llm_text_max: int
    yaml_hash: str

    def cls(self, name):
        try:
            return self.classes[name]
        except KeyError:
            raise KeyError(f"class not in tso.yaml: {name}") from None

    def by_table(self, table):
        for c in self.classes.values():
            if c.table == table:
                return c
        raise KeyError(table)


def _mark(node, path):
    """Line (1-based) of the key at a path in the composed YAML tree; the nearest line when not found."""
    cur, line = node, 1
    for key in path:
        if not isinstance(cur, yaml.MappingNode):
            break
        nxt = None
        for k, v in cur.value:
            if k.value == key:
                line, nxt = k.start_mark.line + 1, v
                break
        if nxt is None:
            break
        cur = nxt
    return line


def load_ontology(path=YAML_PATH, text=None):
    text = text if text is not None else open(path, encoding="utf-8").read()
    fname = os.path.basename(path) if path else "tso.yaml"
    try:
        tree = yaml.compose(text)
        raw = yaml.safe_load(text)
    except yaml.YAMLError as e:
        raise InputError([Diagnostic(fname, str(getattr(getattr(e, "problem_mark", None), "line", 0) + 1), "YAML", str(e))]) from None
    diags = []

    def err(p, msg):
        diags.append(Diagnostic(fname, str(_mark(tree, p)), "ONT", msg))

    enums = raw.get("enums") or {}
    classes = {}
    for cname, cdef in (raw.get("classes") or {}).items():
        if cname in classes:
            err(["classes", cname], f"duplicate class name: {cname}")
        cdef = cdef or {}
        c = ClassDef(cname, cdef.get("layer", ""), cdef.get("doc", ""))
        if c.layer not in ("A", "B", "C"):
            err(["classes", cname, "layer"], f"{cname}: layer must be A, B or C")
        for fname_, fdef in (cdef.get("fields") or {}).items():
            fdef = fdef or {}
            t, card = str(fdef.get("type", "")), str(fdef.get("card", ""))
            enum = ""
            if t.startswith("enum:"):
                enum = t.split(":", 1)[1]
                if enum not in enums:
                    err(["classes", cname, "fields", fname_], f"{cname}.{fname_}: enum does not exist: {enum}")
            elif t not in TYPES:
                err(["classes", cname, "fields", fname_], f"{cname}.{fname_}: unsupported type: {t!r}")
            if card not in CARDS:
                err(["classes", cname, "fields", fname_], f"{cname}.{fname_}: card must be one of {CARDS}")
            if card in ("+", "*") and t != "json":
                err(["classes", cname, "fields", fname_], f"{cname}.{fname_}: array fields must have type json")
            c.fields[fname_] = FieldDef(fname_, t, card, bool(fdef.get("llm")), enum)
        for rname, rdef in (cdef.get("relations") or {}).items():
            rdef = rdef or {}
            card = str(rdef.get("card", ""))
            if card not in CARDS:
                err(["classes", cname, "relations", rname], f"{cname}.{rname}: card must be one of {CARDS}")
            if rname in c.fields:
                err(["classes", cname, "relations", rname], f"{cname}.{rname}: same name as a field")
            c.relations[rname] = RelDef(rname, str(rdef.get("to", "")), card)
        classes[cname] = c
    for c in classes.values():
        for r in c.relations.values():
            if r.to not in classes:
                err(["classes", c.name, "relations", r.name], f"{c.name}.{r.name}: relation targets a class that does not exist: {r.to!r}")
    if not classes:
        err(["classes"], "no classes")
    if diags:
        raise InputError(diags)
    return Ontology(int(raw.get("version", 0)), enums, classes, raw.get("source") or {}, raw.get("ops") or {},
                    int(raw.get("llmTextMaxLength", 600)), hashlib.sha256(text.encode("utf-8")).hexdigest())


_CACHE = {}


def ontology():
    """The default ontology (tso.yaml shipped with tsagent), loaded once."""
    if "o" not in _CACHE:
        _CACHE["o"] = load_ontology()
    return _CACHE["o"]
