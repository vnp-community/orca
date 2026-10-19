import sys,copy,collections,json
from pathlib import Path
from tdd_agent.store.sqlite_knowledge import SqliteKnowledgeStore
from tdd_agent.core import o5_path
s=SqliteKnowledgeStore(Path(sys.argv[1])); g=s.materialize(s.head().id); o5=o5_path.load()
adj=collections.defaultdict(set)
for e in g["edges"]: adj[e["src"]].add(e["dst"]); adj[e["dst"]].add(e["src"])
def comp(i):
    c=set(); st=[i]
    while st:
        n=st.pop()
        if n in c: continue
        c.add(n); st.extend(adj[n]-c)
    return c
ids={x["id"] for x in g["elements"]}; by={x["id"]:x for x in g["elements"]}
def run(name, approved_ids, cs):
    base=copy.deepcopy(g)
    for x in base["elements"]: x["review_status"]="approved" if x["id"] in approved_ids else "needs_review"
    before=sum(1 for x in base["elements"] if x["review_status"]=="approved")
    cand,rep=o5.apply_changeset(copy.deepcopy(base),cs)
    if not rep.get("ok"): print(name,"FAIL",json.dumps(rep)[:250]); return
    after=sum(1 for x in cand["elements"] if x["review_status"]=="approved")
    print(f"| {name} | {before} | {len(rep['touched'])} | {before-after} | {after} |")
tmpl=by["FR-SYS-07"]
def csi(i):
    return {"class":"ChangeSpecItem","id":f"CSI-SPIKE-{i}","name":f"spike item {i}","origin":"own","owner_doc":"","props":{"action":"modify","code_target":"backend/shared/pkg/aiclient/client.go::Client","srs_refs":["FR-SYS-07"],"tdd_refs":["ENT-17"],"verification":"go test","cr_acceptance_refs":["AC-AI-01"]},"provenance":copy.deepcopy(tmpl["provenance"][:1]),"review_status":"needs_review","tdds":[],"variants":[],"maturity":"draft","version_introduced":"0"}
def adds(n): return {"id":f"cs-add-{n}","base_version":g.get("version"),"ops":[{"seq":i+1,"op":"add","after":csi(i)} for i in range(n)]}
def mod(target,k=1):
    ops=[]
    for i in range(k):
        t=target[i]; ops.append({"seq":i+1,"op":"modify","target":t,"path":"name","before":by[t]["name"],"after":by[t]["name"]+" (rev)"})
    return {"id":"cs-mod","base_version":g.get("version"),"ops":ops}

for rel in ("traces_to","implements","verifies","refines"):
    cs=adds(1); cs["ops"].append({"seq":9,"op":"link","after":{"src":"CSI-SPIKE-0","rel":rel,"dst":"FR-SYS-07"}})
    run(f"link CSI -{rel}-> FR-SYS-07",ids,cs)
