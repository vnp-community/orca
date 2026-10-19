import json,re,sys,collections,os
D=os.path.dirname(os.path.abspath(__file__))
gt=json.load(open(D+"/groundtruth.json"))
IDRE=re.compile(r"\b(?:FR|BRL|ENT|NFR|ADR|INT|BO|SVC|CT)-(?:[A-Z]{2,3}-)?\d+\b")
def expand(txt):
    out=set(IDRE.findall(txt))
    for pre,a,b in re.findall(r"((?:FR|BRL|ENT|NFR)-(?:[A-Z]{2,3}-)?)(\d+)\.\.(\d+)",txt):
        w=len(a)
        for i in range(int(a),int(b)+1): out.add(f"{pre}{i:0{w}d}")
    return out
print("| Request | đúng | Read: | recall(Read) | precision(Read) | recall(FR) | recall(get_element) | tool | lượt | $ | ký tự |")
print("|---|---|---|---|---|---|---|---|---|---|---|")
tot=collections.Counter(); R=[];P=[]
for rid,truth in gt.items():
    p=f"{D}/run-{rid}.jsonl"
    if not os.path.exists(p): continue
    tools=collections.Counter(); fetched=set(); res=None; cost=0; turns=0
    for l in open(p):
        try: d=json.loads(l)
        except: continue
        if d.get("type")=="assistant":
            for c in d["message"]["content"]:
                if c["type"]=="tool_use":
                    tools[c["name"].replace("mcp__tdd-agent__","")]+=1
                    if c["name"].endswith("kg_get_element"): 
                        i=c["input"].get("id") or c["input"].get("ids"); 
                        fetched|=set(i if isinstance(i,list) else [i])
        if d.get("type")=="result": res=d.get("result") or ""; cost=d.get("total_cost_usd",0); turns=d.get("num_turns",0)
    if res is None: print("|",rid,"(not finished)"); continue
    m=re.search(r"^Read:(.*)$",res,re.M); rd=expand(m.group(1)) if m else set()
    rng=len(re.findall(r"\d\.\.\d",m.group(1))) if m else 0
    T=set(truth); frT={x for x in T if x.startswith("FR-")}
    rec=len(rd&T)/len(T); prec=len(rd&T)/len(rd) if rd else 0
    recfr=len(rd&frT)/len(frT); recf=len(fetched&T)/len(T)
    R.append(rec);P.append(prec)
    mcp=sum(v for k,v in tools.items() if k.startswith("kg_") or k in("source_get",))
    print(f"| {rid} | {len(T)} | {len(rd)} | {rec:.0%} | {prec:.0%} | {recfr:.0%} | {recf:.0%} | {mcp} | {turns} | {cost:.2f} | {len(res)} |")
    tot["cost"]+=cost
if R: print(f"\nmean recall(Read)={sum(R)/len(R):.0%} min={min(R):.0%}  mean precision={sum(P)/len(P):.0%}  total cost=${tot['cost']:.2f}")
