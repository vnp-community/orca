"""Profile of a TDD package (T8.1): Core always runs; Extended is on when `ontologyVersion` declares `+ext-N` (N >= minExt)."""
import os
import re
from functools import cache

import yaml

FILE = os.path.join(os.path.dirname(os.path.dirname(__file__)), "ontology", "profiles.yaml")


@cache
def _load(path):
    with open(path, encoding="utf-8") as fh:
        return yaml.safe_load(fh)


def config():
    base = _load(FILE)
    extra = os.environ.get("TSAGENT_PROFILES")
    if extra:
        over = _load(extra)
        base = {**base, "extended": {**base["extended"], **over.get("extended", {}),
                                     "relations": {**base["extended"]["relations"], **over.get("extended", {}).get("relations", {})}}}
    return base


def ext_level(pkg):
    m = re.search(r"\+ext-(\d+)", str(pkg.meta.get("ontologyVersion", "")))
    return int(m.group(1)) if m else 0


def profile_of(pkg):
    return "extended" if ext_level(pkg) >= config()["extended"]["minExt"] else "core"


def rel(name):
    """Relation name in the KG for the logical name `name`."""
    return config()["extended"]["relations"].get(name, name)
