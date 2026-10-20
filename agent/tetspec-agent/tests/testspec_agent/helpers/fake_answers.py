"""Valid answers for the three LLM steps (analysis, design, specification) built from the context pack in the prompt."""
import json
import re


def pack_of(user):
    return json.loads(re.search(r'<data name="context">\n(.*?)\n</data>', user, re.S).group(1))


def step_of(system):
    m = re.search(r'"title": "(analysis|design|specification)"', system)
    return m.group(1) if m else None


def analysis(p):
    subj = p["subject"]["code"]
    return {"schemaVersion": 2, "subject": subj,
            "obligations": [{"seedCode": o["seedCode"], "question": "LLM hỏi: " + o["question"][:100]} for o in p["obligations"]],
            "conditions": [{"seedCode": c["seedCode"], "statement": "LLM: " + c["statement"][:200],
                            "expectedEffect": c["expectedEffect"][:300], "basedOn": [subj],
                            "evidence": [{"sourceCode": subj, "quote": "trích"}], "confidence": 0.9} for c in p["conditions"]],
            "proposedConditions": [], "gaps": []}


def design(p):
    return {"schemaVersion": 2, "requirements": [
        {"seedCode": t["seedCode"], "statement": "LLM: " + t["statement"][:300], "scopeNote": "LLM: " + t["scopeNote"][:300],
         "passCriteria": [{"conditionCode": c["code"], "text": "Đạt khi " + c["oracle"]["expected"][:200]} for c in t["conditions"]]}
        for t in p["requirements"]]}


def specification(p):
    out = []
    for t in p["testcases"]:
        steps = [{"stepKind": s["stepKind"], "action": "LLM: " + s["action"][:300],
                  **({"targetCode": s["targetCode"]} if s.get("targetCode") else {}), **({"input": s["input"]} if "input" in s else {})}
                 for s in t["steps"]]
        out.append({"seedCode": t["seedCode"], "objective": "LLM: " + t["objective"][:300], "steps": steps})
    return {"schemaVersion": 2, "testcases": out}


def answer(system, user):
    return {"analysis": analysis, "design": design, "specification": specification}[step_of(system)](pack_of(user))
