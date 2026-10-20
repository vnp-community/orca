"""Domain types of the source KG (ONTOLOGY-TDD): nodes, edges, the ontology sent along, the source package."""
import hashlib
import json
from collections import defaultdict
from dataclasses import dataclass, field

# Attributes shared by every node (ONTOLOGY-TDD section 1.1); everything else lives in props.
BASE_ATTRS = ("label", "version", "status", "sourceDoc", "sourceSection", "owner", "description", "note", "testScope", "primary")
# Not part of the content hash: the position in the source file changes when lines are merely inserted
VOLATILE = ("sourceLine", "sourceFile")


def stable_hash(obj):
    return hashlib.sha256(json.dumps(obj, sort_keys=True, ensure_ascii=False, default=str).encode("utf-8")).hexdigest()


@dataclass
class SrcNode:
    code: str
    cls: str
    base: dict = field(default_factory=dict)
    props: dict = field(default_factory=dict)
    origin: str = ""  # file and position of the declaration, for diagnostics

    def get(self, key, default=None):
        if key in self.base:
            return self.base[key]
        return self.props.get(key, default)

    @property
    def in_scope(self):
        return self.base.get("testScope") != "context"

    def content_hash(self):
        return stable_hash({"cls": self.cls, "base": {k: v for k, v in self.base.items() if k not in VOLATILE},
                            "props": {k: v for k, v in self.props.items() if k not in VOLATILE}})


@dataclass(frozen=True)
class SrcEdge:
    src: str
    rel: str
    dst: str
    props: tuple = ()  # sorted (k, v) pairs so the edge is hashable

    @classmethod
    def make(cls, src, rel, dst, props=None):
        return cls(src, rel, dst, tuple(sorted((props or {}).items(), key=lambda kv: kv[0])))

    @property
    def props_dict(self):
        return dict(self.props)


@dataclass
class OntologyDecl:
    """Classes and relations declared by the source package (ONTOLOGY-TDD section 8, field `ontology`)."""
    classes: dict = field(default_factory=dict)    # name -> {parent, required, attrs, doc}
    relations: dict = field(default_factory=dict)  # name -> {domain, range, doc}

    def ancestors(self, cls):
        out, cur = [], cls
        while cur and cur in self.classes and cur not in out:
            out.append(cur)
            cur = self.classes[cur].get("parent")
        if cur and cur not in out:
            out.append(cur)
        return out

    def is_a(self, cls, target):
        return target == "Entity" or target in self.ancestors(cls)

    def required(self, cls):
        req = []
        for c in self.ancestors(cls):
            for r in self.classes.get(c, {}).get("required", []):
                if r not in req:
                    req.append(r)
        return req

    def merge(self, other):
        for k, v in other.classes.items():
            self.classes.setdefault(k, v)
        for k, v in other.relations.items():
            if k in self.relations:
                cur = self.relations[k]
                cur["domain"] = sorted(set(cur.get("domain", [])) | set(v.get("domain", [])))
                cur["range"] = sorted(set(cur.get("range", [])) | set(v.get("range", [])))
            else:
                self.relations[k] = dict(v)

    def to_dict(self):
        return {"classes": [{"name": k, **v} for k, v in sorted(self.classes.items())],
                "relations": [{"name": k, **v} for k, v in sorted(self.relations.items())]}


@dataclass
class SourcePackage:
    """One source baseline of a feature: nodes, edges, ontology, supersede pairs on the TDD side."""
    feature: str
    nodes: dict = field(default_factory=dict)        # code -> SrcNode
    edges: set = field(default_factory=set)          # SrcEdge
    ontology: OntologyDecl = field(default_factory=OntologyDecl)
    supersede: list = field(default_factory=list)    # [{"old":, "new":}]
    meta: dict = field(default_factory=dict)         # format, ontologyVersion, package, docs, primaryDoc
    baseline_id: str = ""
    _out: dict = field(default=None, repr=False)
    _in: dict = field(default=None, repr=False)

    # ---------------------------------------------------------------- building
    def add_node(self, node):
        cur = self.nodes.get(node.code)
        if cur is None:
            self.nodes[node.code] = node
            self._invalidate()
            return node
        if cur.cls != node.cls:
            raise ValueError(f"code {node.code} is used by two classes: {cur.cls} ({cur.origin}) and {node.cls} ({node.origin})")
        for k, v in node.base.items():
            cur.base.setdefault(k, v)
        for k, v in node.props.items():
            cur.props.setdefault(k, v)
        return cur

    def add_edge(self, src, rel, dst, props=None):
        self.edges.add(SrcEdge.make(src, rel, dst, props))
        self._invalidate()

    def _invalidate(self):
        self._out = self._in = None

    def _index(self):
        if self._out is None:
            out, inn = defaultdict(lambda: defaultdict(list)), defaultdict(lambda: defaultdict(list))
            for e in sorted(self.edges, key=lambda e: (e.src, e.rel, e.dst)):
                out[e.src][e.rel].append(e.dst)
                inn[e.dst][e.rel].append(e.src)
            self._out, self._in = out, inn
        return self._out, self._in

    # ---------------------------------------------------------------- queries
    def get(self, code):
        return self.nodes.get(code)

    def of(self, *classes):
        for code in sorted(self.nodes):
            n = self.nodes[code]
            if n.cls in classes:
                yield n

    def targets(self, code, rel, cls=None):
        out, _ = self._index()
        res = [self.nodes[t] for t in out.get(code, {}).get(rel, []) if t in self.nodes]
        return [n for n in res if cls is None or n.cls == cls or (isinstance(cls, tuple) and n.cls in cls)]

    def sources(self, code, rel, cls=None):
        _, inn = self._index()
        res = [self.nodes[s] for s in inn.get(code, {}).get(rel, []) if s in self.nodes]
        return [n for n in res if cls is None or n.cls == cls or (isinstance(cls, tuple) and n.cls in cls)]

    def out_edges(self, code):
        out, _ = self._index()
        return {rel: list(ts) for rel, ts in out.get(code, {}).items()}

    def in_edges(self, code):
        _, inn = self._index()
        return {rel: list(ss) for rel, ss in inn.get(code, {}).items()}

    def participants(self, scenario):
        """Modules taking part in a Scenario: the ontology declares `participatesIn` Module -> Scenario; md v1 may declare it the other way."""
        mods = {n.code: n for n in self.sources(scenario, "participatesIn", "Module")}
        mods.update({n.code: n for n in self.targets(scenario, "participatesIn", "Module")})
        return [mods[c] for c in sorted(mods)]

    def content_hash(self):
        return stable_hash({"nodes": {c: n.content_hash() for c, n in sorted(self.nodes.items())},
                            "edges": sorted((e.src, e.rel, e.dst, e.props) for e in self.edges),
                            "supersede": sorted((s["old"], s["new"]) for s in self.supersede)})
