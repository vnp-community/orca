"""Typed errors shared by all runtime components.

Every error carries a stable ``code`` so callers (orchestrator, MCP, CLI) can
turn it into a machine-readable blocker instead of parsing messages.
"""
from __future__ import annotations


class TddAgentError(Exception):
    code = "TDD_AGENT_ERROR"

    def __init__(self, message: str = "", *, code: str | None = None, details: dict | None = None):
        super().__init__(message or (code or self.code))
        if code:
            self.code = code
        self.details = details or {}

    def as_dict(self) -> dict:
        return {"code": self.code, "message": str(self), "details": self.details}


class ConfigError(TddAgentError):
    code = "CONFIG_ERROR"


class VersionConflict(TddAgentError):
    code = "VERSION_CONFLICT"


class ChangeSetRejected(TddAgentError):
    code = "CHANGESET_REJECTED"


class SourceRoleError(TddAgentError):
    code = "SOURCE_ROLE_ERROR"


class SourceConflict(TddAgentError):
    code = "SOURCE_CONFLICT"


class NotFound(TddAgentError):
    code = "NOT_FOUND"


class SearchNotSupported(TddAgentError):
    code = "SEARCH_NOT_SUPPORTED"


class ProviderError(TddAgentError):
    code = "PROVIDER_ERROR"


class RelationNotRegistered(TddAgentError):
    code = "RELATION_NOT_REGISTERED"


class ApprovalRejected(TddAgentError):
    code = "APPROVAL_REJECTED"
