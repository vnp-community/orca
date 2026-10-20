"""Domain types of the test KG (ONTOLOGY-TEST-TSO): entities by class, relations by code, provenance."""
from dataclasses import dataclass, field

from .source import stable_hash

DERIVATIONS = ("stated", "rule", "llm", "human")


@dataclass
class Evidence:
    source_code: str
    quote: str = ""


@dataclass
class Provenance:
    derivation: str = "rule"
    based_on: list = field(default_factory=list)   # source node codes
    evidence: list = field(default_factory=list)   # Evidence
    model: str = ""
    prompt_hash: str = ""
    confidence: float = 0.7
    run_id: str = ""

    def __post_init__(self):
        if self.derivation not in DERIVATIONS:
            raise ValueError(f"invalid derivation: {self.derivation}")


@dataclass
class TsoEntity:
    """One test KG node. `rels`: relation name -> list of target codes (sorted, unique)."""
    cls: str
    code: str
    attrs: dict = field(default_factory=dict)
    rels: dict = field(default_factory=dict)
    prov: Provenance = field(default_factory=Provenance)

    def get(self, key, default=None):
        return self.attrs.get(key, default)

    def rel(self, name):
        return self.rels.get(name, [])

    def link(self, name, *targets):
        targets = [t for t in targets if t]
        if not targets:
            return self
        cur = self.rels.setdefault(name, [])
        for t in targets:
            if t not in cur:
                cur.append(t)
        cur.sort()
        return self

    def content_hash(self):
        """Content hash: class, attributes, relations (by code), basis. Excludes volatile provenance (model, run, confidence)."""
        return stable_hash({"cls": self.cls, "attrs": self.attrs,
                            "rels": {k: sorted(v) for k, v in sorted(self.rels.items()) if v},
                            "basedOn": sorted(self.prov.based_on), "derivation": self.prov.derivation})


@dataclass
class TsoGraph:
    """The test KG entities of one feature (one run or one SpecVersion)."""
    feature: str
    entities: dict = field(default_factory=dict)  # code -> TsoEntity
    regression: list = field(default_factory=list)
    meta: dict = field(default_factory=dict)

    def add(self, e):
        cur = self.entities.get(e.code)
        if cur is not None and cur.cls != e.cls:
            raise ValueError(f"code {e.code} is used by two classes: {cur.cls} and {e.cls}")
        self.entities[e.code] = e
        return e

    def get(self, code):
        return self.entities.get(code)

    def of(self, *classes):
        for code in sorted(self.entities):
            e = self.entities[code]
            if e.cls in classes:
                yield e

    def referencing(self, code, rel=None, cls=None):
        out = []
        for c in sorted(self.entities):
            e = self.entities[c]
            if cls and e.cls != cls:
                continue
            for name, ts in e.rels.items():
                if (rel is None or name == rel) and code in ts:
                    out.append(e)
                    break
        return out
