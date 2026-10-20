"""Build the Extended profile fixture (T8.2): the CR-001 slice + module 04 entities (ONTOLOGY-TDD section 5), ontologyVersion +ext-1.

python scripts/tsagent/make_ext_fixture.py  ->  tests/testspec_agent/fixtures/minipay/cr001-ext.tdd-kg.json
The entities are modelled from solution.md and the SRS of CR-001 (Vietnamese content); someone owning the TDD ontology must review them before they count as reference.
"""
import copy
import json
import pathlib

FIX = pathlib.Path(__file__).resolve().parents[2] / "tests" / "testspec_agent" / "fixtures" / "minipay"
EPS = ["EP-CreateSchedule", "EP-GetSchedule", "EP-ListSchedules", "EP-ListRuns", "EP-CancelSchedule", "EP-UpdateScheduleStatus"]
SRC = {"sourceDoc": "SOL-CR-001", "sourceSection": "Thiết kế chi tiết (module 04)", "status": "Baselined", "version": "1"}

CLASSES = {
    "DesignObligation": ["question", "subjectScope", "applicability", "requiredChecks"],
    "ObligationResolution": ["status", "explanation"],
    "DesignGap": ["missingKnowledge", "owner", "blockingGates"],
    "Operation": ["inputs", "outputs", "preconditions", "postconditions", "errorOutcomes", "sideEffects", "authContext"],
    "Invariant": ["predicate", "scope", "enforcementPoints", "failureBehavior"],
    "ConcurrencyPolicy": ["sharedResources", "isolation", "conflictHandling", "progressGuarantee"],
    "ErrorContract": ["condition", "classification", "retryable", "sideEffectState"],
    "TimeoutBudget": ["scope", "endToEndDeadline", "callBudgets", "cancellation"],
    "IdempotencyPolicy": ["keyDerivation", "keyScope", "payloadFingerprint", "retention", "atomicStore", "concurrentDuplicateBehavior"],
    "Transaction": ["boundary", "atomicity", "isolation", "commitPoint", "rollbackBehavior"],
    "PermissionDesign": ["subject", "resource", "action", "decisionPredicate", "enforcementPoints", "denyBehavior"],
}
REQ = ["FunctionalRequirement", "NonFunctionalRequirement"]
RELATIONS = {
    "obligationFor": (["DesignObligation"], REQ + ["Module", "Endpoint"]), "resolves": (["ObligationResolution"], ["DesignObligation"]),
    "designRef": (["ObligationResolution"], list(CLASSES) + ["Module"]), "gapOf": (["DesignGap"], ["DesignObligation"]),
    "ownedBy": (["Operation"], ["Module"]), "operationFor": (["Operation"], REQ),
    "invariantOf": (["Invariant"], ["StateMachine", "DataEntity", "Transaction"]), "governs": (["ConcurrencyPolicy", "StateMachine"], ["DataEntity", "Module"]),
    "errorOf": (["ErrorContract"], ["Operation", "Integration"]), "budgetFor": (["TimeoutBudget"], ["Scenario", "Operation"]),
    "idempotencyOf": (["IdempotencyPolicy"], ["Operation"]), "includesOp": (["Transaction"], ["Operation"]),
    "protects": (["PermissionDesign"], ["Endpoint", "Operation"]),
}


def node(code, cls, label, **props):
    return {"code": code, "class": cls, "label": label, **SRC, "props": props}


def main():
    pkg = json.loads((FIX / "cr001-slice.tdd-kg.json").read_text(encoding="utf-8"))
    pkg = copy.deepcopy(pkg)
    pkg["ontologyVersion"] = "tdd-core-1+ext-1"
    names = {c["name"] for c in pkg["ontology"]["classes"]}
    for cls, attrs in CLASSES.items():
        if cls not in names:
            pkg["ontology"]["classes"].append({"name": cls, "parent": "Entity", "required": ["code"], "attrs": attrs,
                                               "doc": f"module 04: {cls}"})
    rels = {r["name"]: r for r in pkg["ontology"]["relations"]}
    for r, (dom, rng) in RELATIONS.items():
        if r in rels:
            rels[r]["domain"] = sorted(set(rels[r]["domain"]) | set(dom))
            rels[r]["range"] = sorted(set(rels[r]["range"]) | set(rng))
        else:
            pkg["ontology"]["relations"].append({"name": r, "domain": dom, "range": rng, "doc": "module 04 (tên đề xuất, OQ-O1)"})
    N = [
        node("OBL-1", "DesignObligation", "Một giao dịch cho mỗi lần chạy", question="Lần chạy đến hạn có luôn tạo tối đa một giao dịch, kể cả khi hai scheduler cùng quét hoặc tiến trình khởi động lại?",
             subjectScope="scheduler", applicability="applicable", requiredChecks=["concurrency", "idempotency", "resilience"]),
        node("OBL-2", "DesignObligation", "Ngân sách thời gian lần chạy", question="Lần chạy có hoàn tất trong ngân sách 60 giây ở trường hợp xấu nhất?",
             subjectScope="scheduler", applicability="applicable", requiredChecks=["timing"]),
        node("OBL-3", "DesignObligation", "Phân quyền mọi endpoint lịch", question="Mọi endpoint lịch có kiểm chủ sở hữu và từ chối người khác?",
             subjectScope="api", applicability="applicable", requiredChecks=["access"]),
        node("OBL-4", "DesignObligation", "Bù trừ khi lỗi sau commit", question="Có cần saga bù trừ khi chuyển tiền lỗi sau commit?",
             subjectScope="scheduler", applicability="notApplicable", requiredChecks=[]),
        node("GAP-1", "DesignGap", "Thời hạn giữ khóa idempotency", missingKnowledge="Thời hạn giữ khóa idempotency của lần chạy (retention) chưa chốt",
             owner="Tech Lead", blockingGates=["baseline"]),
        node("OP-ExecuteScheduledRun", "Operation", "Thực hiện lần chạy đến hạn", inputs=["runId"], outputs=["transferId", "status"],
             preconditions=["Lần chạy ở trạng thái PENDING và đã đến hạn", "Lịch ở trạng thái ACTIVE"],
             postconditions=["Lần chạy SUCCEEDED có transferId", "Đúng một giao dịch được ghi sổ"],
             errorOutcomes=["FRAUD_CHECK_UNAVAILABLE: lần chạy chờ thử lại", "INSUFFICIENT_FUNDS: lần chạy FAILED, không ghi sổ"],
             sideEffects=["giao dịch", "outbox ScheduledRunCompleted", "audit"], authContext="SCHEDULER"),
        node("INV-OneTransferPerRun", "Invariant", "Một giao dịch cho mỗi lần chạy", predicate="Mỗi lần chạy có tối đa một giao dịch",
             scope="scheduled_runs", enforcementPoints=["UPDATE có điều kiện khi chiếm lần chạy", "Idempotency-Key cố định của lần chạy"],
             failureBehavior="Lần chiếm thứ hai không thành công; gọi lại cùng khóa trả giao dịch cũ"),
        node("IDP-ScheduledRun", "IdempotencyPolicy", "Khóa idempotency của lần chạy", keyDerivation="scheduleId:scheduledAt#attempt",
             keyScope="fromAccount", payloadFingerprint="sha256 thân yêu cầu", retention="24h", atomicStore="UNIQUE(from_account, idempotency_key)",
             concurrentDuplicateBehavior="Yêu cầu sau nhận kết quả của yêu cầu thắng"),
        node("CP-RunClaim", "ConcurrencyPolicy", "Chiếm lần chạy", sharedResources=["scheduled_runs"], isolation="UPDATE có điều kiện trên status",
             conflictHandling="Bên thua bỏ qua lần chạy", progressGuarantee="Lần chạy bị chiếm quá 120 giây được chiếm lại"),
        node("TB-ScheduledRun", "TimeoutBudget", "Ngân sách lần chạy", scope="SCN-ScheduledRun", endToEndDeadline=60000,
             callBudgets={"INT-01": 1000}, cancellation="Hết ngân sách thì lần chạy FAILED có reasonCode TIMEOUT"),
        node("TX-ExecuteTransfer", "Transaction", "Giao dịch chuyển tiền của lần chạy", boundary="execute_transfer", atomicity="all-or-nothing",
             isolation="serializable", commitPoint="Ghi sổ + outbox", rollbackBehavior="Lỗi trước commit: không ghi gì"),
        node("EC-FraudUnavailable", "ErrorContract", "Fraud Check không khả dụng", condition="Fraud Check không khả dụng", classification="transient",
             retryable=True, sideEffectState="noEffect"),
        node("EC-LedgerUnknown", "ErrorContract", "Kết quả ghi sổ không rõ", condition="Mất kết nối sau khi gửi lệnh ghi sổ", classification="unknownOutcome",
             retryable=True, sideEffectState="unknown"),
    ]
    for ep in EPS:
        N.append(node(f"PD-{ep[3:]}", "PermissionDesign", f"Quyền {ep}", subject="chủ tài khoản nguồn", resource="lịch chuyển tiền",
                      action="gọi " + ep, decisionPredicate="caller.accountId == schedule.fromAccountId", enforcementPoints=[ep],
                      denyBehavior="403 FORBIDDEN"))
    E = [["OBL-1", "obligationFor", "FR-045"], ["OBL-1", "obligationFor", "NFR-011"], ["OBL-2", "obligationFor", "NFR-010"],
         ["OBL-3", "obligationFor", "FR-042"], ["OBL-4", "obligationFor", "FR-045"], ["GAP-1", "gapOf", "OBL-1"],
         ["OP-ExecuteScheduledRun", "ownedBy", "MOD-Scheduler"], ["OP-ExecuteScheduledRun", "operationFor", "FR-045"],
         ["INV-OneTransferPerRun", "invariantOf", "DE-ScheduledRun"], ["IDP-ScheduledRun", "idempotencyOf", "OP-ExecuteScheduledRun"],
         ["CP-RunClaim", "governs", "DE-ScheduledRun"], ["TB-ScheduledRun", "budgetFor", "SCN-ScheduledRun"],
         ["TX-ExecuteTransfer", "includesOp", "OP-ExecuteScheduledRun"], ["EC-FraudUnavailable", "errorOf", "OP-ExecuteScheduledRun"],
         ["EC-LedgerUnknown", "errorOf", "OP-ExecuteScheduledRun"]]
    # Endpoints live in solution.md (loaded with the package), not in the KG: PermissionDesign points to them through `enforcementPoints`;
    # the seed rules resolve the codes after merging the md (a `protects` edge in the JSON would dangle, violating I1)
    pkg["nodes"] += N
    pkg["edges"] += [e + [{}] for e in E]
    pkg["package"]["kind"] = "slice"
    out = FIX / "cr001-ext.tdd-kg.json"
    out.write_text(json.dumps(pkg, ensure_ascii=False, indent=1) + "\n", encoding="utf-8")
    print(out, len(pkg["nodes"]), len(pkg["edges"]))


if __name__ == "__main__":
    main()
