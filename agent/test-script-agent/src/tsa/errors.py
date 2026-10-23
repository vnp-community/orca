"""Errors with a stable code; the CLI prints the code and exits non-zero."""


class TsaError(Exception):
    code = "TSA_ERROR"

    def __init__(self, message: str, code: str | None = None):
        super().__init__(message)
        if code:
            self.code = code


class TcirError(TsaError):
    code = "TCIR_INVALID"


class SourceError(TsaError):
    code = "TCIR_SOURCE"


class WorktreeError(TsaError):
    code = "WORKTREE"


class UnsupportedFramework(TsaError):
    code = "FRAMEWORK_UNSUPPORTED"


class GeneratorError(TsaError):
    code = "GENERATOR"
