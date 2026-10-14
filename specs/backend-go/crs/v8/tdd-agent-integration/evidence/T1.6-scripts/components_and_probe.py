import sys,copy,collections,json
from pathlib import Path
from tdd_agent.store.sqlite_knowledge import SqliteKnowledgeStore
from tdd_agent.core import o5_path
s=SqliteKnowledgeStore(Path(sys.argv[1])); g=s.materialize(s.head().id)
o5=o5_path.load()
# components
adj=collections.defaultdict(set)
for e in g["edges"]: adj[e["src"]].add(e["dst"]); adj[e["dst"]].add(e["src"])
seen=set(); comps=[]
for x in g["elements"]:
    i=x["id"]
    if i in seen: continue
    st=[i]; c=set()
    while st:
        n=st.pop()
        if n in c: continue
        c.add(n); st.extend(adj[n]-c)
    seen|=c; comps.append(c)
comps.sort(key=len,reverse=True)
print("components:",len(comps),"largest:",[len(c) for c in comps[:5]],"isolated:",sum(1 for c in comps if len(c)==1))
# scenario: whole graph approved (upper bound for debt); realistic: approve a zone
def run(name, approved_ids, cs):
    base=copy.deepcopy(g)
    for x in base["elements"]:
        x["review_status"]="approved" if x["id"] in approved_ids else "needs_review"
    before=sum(1 for x in base["elements"] if x["review_status"]=="approved")
    cand,rep=o5.apply_changeset(copy.deepcopy(base),cs)
    if not rep.get("ok"):
        print(name,"FAIL",json.dumps(rep)[:300]); return
    after=sum(1 for x in cand["elements"] if x["review_status"]=="approved")
    print(f"{name}: approved_before={before} approved_after={after} reset={before-after} touched={len(rep['touched'])}")
ids={x["id"] for x in g["elements"]}
cls={x["id"]:x["class"] for x in g["elements"]}
tmpl=next(x for x in g["elements"] if x["id"]=="FR-SYS-07")
print("FR-SYS-07 degree",len(adj["FR-SYS-07"]), "component size", len(next(c for c in comps if "FR-SYS-07" in c)))
def mk(n, tgt_per, targets):
    ops=[]
    for i in range(n):
        eid=f"TR-SPIKE-{n}-{i}"
        ops.append({"seq":len(ops)+1,"op":"add","after":{"class":"TestReceipt","id":eid,"name":f"spike receipt {i}","origin":"own","owner_doc":"","props":{"cs_id":"CS-SPIKE","criterion_refs":["AC-AI-01"],"result":"passed","executed_at":"2026-10-14T00:00:00Z"},"provenance":copy.deepcopy(tmpl["provenance"][:1]) or [{"source":"spike","kind":"test"}],"review_status":"needs_review","tdds":[],"variants":[],"maturity":"draft","version_introduced":"0"}})
        for t in targets[:tgt_per]:
            ops.append({"seq":len(ops)+1,"op":"link","after":{"src":eid,"rel":"verifies","dst":t}})
    return {"id":f"cs-spike-{n}","base_version":g.get("version"),"ops":ops}
which=sys.argv[2] if len(sys.argv)>2 else "all"
for label,approved in (("ALL approved",ids),):
    for n,tp,targets in ((1,1,["FR-SYS-07"]),(5,1,["FR-SYS-07"]),(20,1,["FR-SYS-07"]),(1,1,["BRL-64"]),(1,1,["ENT-47"])):
        run(f"{label} n={n} tgt={targets[0]}",approved,mk(n,tp,targets))
iso=[c for c in comps if len(c)==1]
# control: add with no link
cs={"id":"cs-ctl","base_version":g.get("version"),"ops":[mk(1,0,[])["ops"][0]]}
run("ALL approved control add-only no link",ids,cs)
# approved zone: only requirement FR-* approved
run("FR-only approved, n=1 link FR-SYS-07",{i for i in ids if i.startswith("FR-")},mk(1,1,["FR-SYS-07"]))
