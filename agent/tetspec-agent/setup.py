"""Packaging hook: ship the ``o5`` reference harness inside the package so ``tdd-agent`` works outside a repository checkout.

The harness lives in ``data/ontology_v0.5/o5`` (not renamed or moved: historical reports quote its hashes).  The wheel gets a
copy under ``tdd_agent/_data`` without its test fixtures, tests and caches (about 2 MB instead of 49 MB), together with the two
ontology v0.4 files its manifest points to.
"""
from __future__ import annotations

import shutil
from pathlib import Path

from setuptools import setup
from setuptools.command.build_py import build_py

DATA = Path(__file__).resolve().parent / "data"
SKIP = shutil.ignore_patterns("fixtures", "tests", "__pycache__", "*.pyc", ".DS_Store")
# (source under data/, what to copy).  o5's manifest names ../../ontology_v0.4/o4/{classes,relations}.yaml, so the layout is kept under tdd_agent/_data.
PARTS = (("ontology_v0.5/o5", None), ("ontology_v0.4/o4", ("classes.yaml", "relations.yaml")))


class BuildPy(build_py):
    def run(self):
        super().run()
        for rel, only in PARTS:
            source = DATA / rel
            if not source.is_dir():
                continue
            target = Path(self.build_lib) / "tdd_agent" / "_data" / rel
            if target.exists():
                shutil.rmtree(target)
            if only is None:
                shutil.copytree(source, target, ignore=SKIP)
            else:
                target.mkdir(parents=True)
                for name in only:
                    shutil.copy2(source / name, target / name)


setup(cmdclass={"build_py": BuildPy})
