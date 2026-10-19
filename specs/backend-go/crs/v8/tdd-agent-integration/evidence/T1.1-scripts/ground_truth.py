import re,json,sys
SRS="/Users/datbeohbbh/Desktop/work/vnpay/agentic-app/vnp-qa-platform/docs/SRS.md"
FR={ # request -> FR ids judged relevant (written before any agent run)
 "BL-H01":["FR-SYS-07"],
 "BL-H02":["FR-SYS-07","FR-CAS-02","FR-CAS-07"],
 "BL-B06":["FR-BUG-02","FR-BUG-03","FR-BUG-06","FR-BUG-07","FR-PRJ-05","FR-JIR-03"],
 "BL-C05":["FR-API-01","FR-API-02","FR-API-03","FR-API-12"],
 "BL-D03":["FR-DSH-05","FR-DSH-02","FR-NTF-04"],
 "BL-B08":["FR-PLN-05","FR-API-09","FR-SYS-06"],
 "BL-B10":["FR-ORG-06","FR-RBA-02","FR-RBA-04","FR-RBA-05"],
 "BL-C04":["FR-API-08"],
 "BL-B02":["FR-CAS-05","FR-CAS-06"],
 "BL-B07":["FR-PLN-03","FR-PLN-04","FR-PLN-06","FR-PLN-07","FR-PLN-08","FR-UI-04"],
}
def expand(cell):
    out=[]
    for tok in re.findall(r"(?:BRL|ENT)-\d+(?:\.\.\d+)?",cell):
        m=re.match(r"(BRL|ENT)-(\d+)\.\.(\d+)",tok)
        if m: out+= [f"{m.group(1)}-{i:02d}" for i in range(int(m.group(2)),int(m.group(3))+1)]
        else: out.append(tok)
    return out
rows={}
for l in open(SRS):
    if re.match(r"\| FR-",l):
        c=[x.strip() for x in l.split("|")]
        rows[c[1]]=expand(c[5])+expand(c[6])
gt={}
for k,v in FR.items():
    ids=list(v)
    for f in v: ids+=rows[f]
    gt[k]=sorted(set(ids))
json.dump(gt,open(sys.argv[1],"w"),indent=1)
for k,v in gt.items(): print(k,len(v),v)
