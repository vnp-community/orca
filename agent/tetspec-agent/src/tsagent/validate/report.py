"""Report frame of the TSO-* rules: rule levels, violations by node code, gates GA/GB/GS."""
from dataclasses import dataclass, field

BLOCK, BLOCK_BASELINE, WARN = "block", "blockBaseline", "warn"


@dataclass
class RuleResult:
    rule: str
    level: str
    title: str
    checked: int = 0
    violations: list = field(default_factory=list)  # {"code", "message"}

    @property
    def ok(self):
        return not self.violations

    def add(self, code, message):
        self.violations.append({"code": code, "message": message})

    def to_dict(self, limit=200):
        return {"rule": self.rule, "level": self.level, "title": self.title, "checked": self.checked, "ok": self.ok,
                "violationCount": len(self.violations), "violations": self.violations[:limit]}


@dataclass
class Report:
    gate: str
    results: list = field(default_factory=list)
    counts: dict = field(default_factory=dict)

    def result(self, rule):
        return next((r for r in self.results if r.rule == rule), None)

    @property
    def blocking(self):
        """Violations that fail the gate: level `block`; for GS also `blockBaseline`."""
        levels = (BLOCK, BLOCK_BASELINE) if self.gate == "GS" else (BLOCK,)
        return [r for r in self.results if r.level in levels and not r.ok]

    @property
    def passed(self):
        return not self.blocking

    def to_dict(self):
        return {"gate": self.gate, "passed": self.passed, "blocking": [r.rule for r in self.blocking],
                "counts": dict(sorted(self.counts.items())), "rules": [r.to_dict() for r in self.results]}

    def summary(self):
        bad = ", ".join(f"{r.rule}({len(r.violations)})" for r in self.results if not r.ok) or "no violations"
        return f"gate {self.gate}: {'passed' if self.passed else 'failed'}; {bad}"
