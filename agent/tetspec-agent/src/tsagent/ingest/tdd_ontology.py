"""TDD ontology, Core profile + tsagent extensions (ONTOLOGY-TDD sections 2-4), used when the input is md only, and to add
the extension classes to the ontology sent along with a tdd-kg/v1 package."""
from ..model import OntologyDecl


def _c(parent="Entity", required=(), attrs=(), doc=""):
    return {"parent": parent, "required": ["code", *required], "attrs": list(attrs), "doc": doc}


CORE_CLASSES = {
    "Entity": {"parent": None, "required": ["code"], "attrs": [], "doc": "Root class"},
    "Document": _c(required=["docType"], attrs=["docType", "baselineDate", "title", "path"]),
    "Requirement": _c(required=["text"], attrs=["text", "priority", "targetValue", "targetUnit"]),
    "FunctionalRequirement": _c("Requirement", ["text"]),
    "NonFunctionalRequirement": _c("Requirement", ["text"], ["category", "condition"]),
    "Constraint": _c(required=["constraintType"]),
    "Assumption": _c(required=["statement"], attrs=["statement", "dueDate", "confirmStatus"]),
    "OpenQuestion": _c(required=["statement"], attrs=["statement", "dueDate", "confirmStatus"]),
    "Module": _c(required=["responsibility"], attrs=["responsibility", "layer"]),
    "Technology": _c(required=["category"], attrs=["category", "techVersion"]),
    "Decision": _c(required=["statement"], attrs=["statement", "rationale", "noAlternativeNote"]),
    "Alternative": _c(required=["reasonRejected"]),
    "DataEntity": _c(attrs=["retentionRule", "volumeEstimate"]),
    "Mechanism": _c(required=["mechanismKind"]),
    "Scenario": _c(required=["responseBudgetMs"], attrs=["steps", "responseBudgetMs"]),
    "Environment": _c(attrs=["hostingType"]),
    "Risk": _c(required=["likelihood", "impact"], attrs=["mitigation"]),
    "Evidence": _c(required=["summary"], attrs=["measuredAt", "location", "metrics"]),
    "TestCase": _c(required=["level"], attrs=["testStatus", "location"]),
    "DesignItem": _c(required=["kind", "location"]),
    "Integration": _c(required=["integrationType", "direction", "interactionStyle", "idempotent"], attrs=["protocol", "expectedRps"]),
    "ExternalParty": _c(required=["partyType"]),
    "Endpoint": _c(required=["method", "path"], attrs=["apiVersion", "mediaType", "auth", "idempotent"]),
    "Channel": _c(required=["partitionKey", "deliverySemantics"], attrs=["dlqName", "consumerIdempotent"]),
    "Message": _c(required=["messageType"]),
    "Schema": _c(required=["format", "location"]),
    "Header": _c(required=["name", "required"]),
    "ErrorMapping": _c(required=["trigger", "errorCategory", "retryable", "action"]),
    "TimeoutPolicy": _c(required=["connectMs", "readMs", "totalMs"], attrs=["poolAcquireMs", "idleMs"]),
    "RetryPolicy": _c(required=["maxAttempts", "idempotentOnly"], attrs=["backoffType", "jitter"]),
    "CircuitBreaker": _c(required=["failureRatePercent", "minimumCalls", "openDurationSec"]),
    "ConnectionPool": _c(required=["maxTotal"]),
    "SecurityScheme": _c(required=["schemeType"], attrs=["secretStore", "rotationDays", "signatureCheck"]),
    "PartnerLimit": _c(required=["maxConcurrentConnections"], attrs=["requestsPerSecond", "maxPayloadKB"]),
    "FallbackStrategy": _c(required=["mode"]),
    "ReconciliationJob": _c(attrs=["schedule", "scope"]),
    "MonitoringSpec": _c(attrs=["metrics", "alertThreshold"]),
}
# extension classes tsagent accepts through md, the solution appendix and the supplement (ONTOLOGY-TDD section 4)
EXT_CLASSES = {
    "UserRequirement": _c(required=["text"], attrs=["priority"]),
    "AcceptanceCriterion": _c(required=["statement"], attrs=["derivation", "basis"]),
    "StateMachine": _c(attrs=["name", "initial"]),
    "State": _c(required=["name"], attrs=["terminal"]),
    "Transition": _c(required=["from", "to", "trigger"], attrs=["guard", "effect"]),
    "Field": _c(required=["name"], attrs=["type", "required", "min", "max", "minLength", "maxLength", "enum"]),
    "Response": _c(required=["statusCode"], attrs=["errorCode", "condition"]),
    "ChangeRequest": _c(attrs=["description", "changeKind"]),
    "Actor": _c(required=["name"]),
}
REQ = ["Requirement"]
CORE_RELATIONS = {
    "satisfiedBy": (REQ, ["Module", "Mechanism", "Decision"]), "allocatedTo": (["FunctionalRequirement"], ["Module"]),
    "realizedBy": (REQ, ["Scenario"]), "verifiedBy": (REQ, ["TestCase"]), "refinedBy": (["Module", "Integration"], ["DesignItem"]),
    "dependsOn": (["Module"], ["Module"]), "uses": (["Module"], ["Technology"]), "ownsData": (["Module"], ["DataEntity"]),
    "relatesTo": (["DataEntity"], ["DataEntity"]), "participatesIn": (["Module", "Scenario"], ["Scenario", "Module"]),
    "invokes": (["Scenario"], ["Integration"]), "integratesVia": (["Module"], ["Integration"]),
    "deployedIn": (["Module"], ["Environment"]), "selects": (["Decision"], ["Technology"]), "rejects": (["Decision"], ["Alternative"]),
    "supportedBy": (["Decision"], ["Evidence"]), "constrainedBy": (["Decision", "Module", "Integration"], ["Constraint"]),
    "assumes": (["Entity"], ["Assumption"]), "blockedBy": (["Entity"], ["OpenQuestion"]),
    "hasRisk": (["Entity"], ["Risk"]), "mitigatedBy": (["Risk"], ["Mechanism", "Decision"]),
    "supersedes": (["Entity"], ["Entity"]), "describedIn": (["Entity"], ["Document"]),
    "withParty": (["Integration"], ["ExternalParty"]), "hasEndpoint": (["Integration"], ["Endpoint"]),
    "usesChannel": (["Integration"], ["Channel"]), "hasRequest": (["Endpoint"], ["Message"]),
    "hasResponse": (["Endpoint"], ["Message", "Response"]), "carries": (["Channel"], ["Message"]),
    "hasSchema": (["Message"], ["Schema"]), "requiresHeader": (["Endpoint", "Message"], ["Header"]),
    "hasErrorMapping": (["Integration"], ["ErrorMapping"]), "hasTimeoutPolicy": (["Integration"], ["TimeoutPolicy"]),
    "hasRetryPolicy": (["Integration"], ["RetryPolicy"]), "protectedBy": (["Integration"], ["CircuitBreaker"]),
    "usesPool": (["Integration"], ["ConnectionPool"]), "securedBy": (["Integration"], ["SecurityScheme"]),
    "boundedBy": (["Integration"], ["PartnerLimit"]), "hasFallback": (["Integration"], ["FallbackStrategy"]),
    "reconciledBy": (["Integration"], ["ReconciliationJob"]), "monitoredBy": (["Integration"], ["MonitoringSpec"]),
}
EXT_RELATIONS = {
    "tracesTo": (["UserRequirement"], REQ), "hasAcceptance": (REQ, ["AcceptanceCriterion"]),
    "governs": (["StateMachine"], ["DataEntity", "Module"]), "hasTransition": (["StateMachine"], ["Transition"]),
    "inMachine": (["Transition", "State"], ["StateMachine"]), "hasState": (["StateMachine"], ["State"]),
    "hasField": (["Endpoint"], ["Field"]), "implements": (["Endpoint"], REQ),
    "affects": (["ChangeRequest"], ["Entity"]), "removes": (["ChangeRequest"], ["Entity"]),
}


def _rel(pairs):
    return {k: {"domain": d, "range": r} for k, (d, r) in pairs.items()}


def core_ontology():
    return OntologyDecl({**CORE_CLASSES, **EXT_CLASSES}, {**_rel(CORE_RELATIONS), **_rel(EXT_RELATIONS)})


def extension_ontology():
    return OntologyDecl(dict(EXT_CLASSES), {**_rel(EXT_RELATIONS), "hasResponse": {"domain": ["Endpoint"], "range": ["Response"]}})


ALL_RELATIONS = sorted({*CORE_RELATIONS, *EXT_RELATIONS})
