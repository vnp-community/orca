"""tsagent error tree. Every business error derives from TsagentError so the CLI and the gateway return structured messages."""
from dataclasses import dataclass


@dataclass(frozen=True)
class Diagnostic:
    """One located diagnostic (NFR-012): file, location (line or JSON pointer), rule, message."""
    file: str
    loc: str
    rule: str
    message: str
    severity: str = "error"

    def __str__(self):
        return f"{self.file}:{self.loc} [{self.rule}] {self.message}"

    def to_dict(self):
        return {"file": self.file, "loc": self.loc, "rule": self.rule, "message": self.message, "severity": self.severity}


class TsagentError(Exception):
    """Root of every business error."""
    code = None


class ConfigError(TsagentError):
    code = "CONFIG_INVALID"


class LLMNotConfigured(ConfigError):
    code = "LLM_NOT_CONFIGURED"


class LLMKeyMissing(ConfigError):
    code = "LLM_KEY_MISSING"


class LLMModelMissing(ConfigError):
    code = "LLM_MODEL_MISSING"


class InvalidFeature(TsagentError):
    pass


class InputError(TsagentError):
    def __init__(self, diagnostics, message=None):
        self.diagnostics = list(diagnostics)
        errs = [d for d in self.diagnostics if d.severity == "error"]
        super().__init__(message or f"input has {len(errs)} error(s): " + "; ".join(str(d) for d in errs[:3]))


class ValidationFailed(TsagentError):
    def __init__(self, report, stage):
        self.report = report
        self.stage = stage
        rules = {r["rule"]: r for r in report.get("rules", [])}
        parts = [f"{rid} ({rules[rid]['violationCount']}): {rules[rid]['violations'][0]['code']} {rules[rid]['violations'][0]['message']}"
                 if rid in rules and rules[rid]["violations"] else rid for rid in report.get("blocking", [])]
        parts += report.get("errors", [])
        super().__init__(f"{stage}: gate {report.get('gate')} failed: " + "; ".join(parts[:4]))


class ImmutableError(TsagentError):
    pass


class NotFound(TsagentError):
    pass


class BusyError(TsagentError):
    pass


class HandEditedError(TsagentError):
    pass


class LLMError(TsagentError):
    """LLM call error following the ErrorMapping of INT-01. `fatal` means the whole run stops (bad key, not logged in)."""
    category = "permanent"
    retryable = False

    def __init__(self, message, fatal=False):
        super().__init__(message)
        self.fatal = fatal


class LLMTransient(LLMError):
    category = "transient"
    retryable = True


class LLMUnknownOutcome(LLMError):
    category = "unknownOutcome"
    retryable = True


class LLMPermanent(LLMError):
    category = "permanent"


class LLMSchemaError(LLMError):
    category = "schema"


class BudgetExceeded(LLMError):
    category = "budget"


class CircuitOpen(LLMError):
    category = "circuitOpen"


class LLMGroupsFailed(TsagentError):
    """More than half of the LLM groups of a step still failed after retries: the run fails (not an LLMError, so it is not swallowed as a group error)."""
    code = "LLM_GROUPS_FAILED"


class LLMDisabled(LLMError):
    category = "disabled"
