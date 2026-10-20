"""Curated oracle; expectations fixed before pipeline and agent authoring."""
from pathlib import Path
from copy import deepcopy
import json, re
from semantic_intake import evidence, build_graph, source_digest
from quality5 import verify_evidence, fingerprint

DOCS = Path(__file__).resolve().parents[3]/'test-pbom'

def load_sources():
    result=[]
    for name in ('URD','SRS','SAD'):
        path=DOCS/(name+'.md');text=path.read_text()
        match=re.search(r'^\|\s*Phiên bản\s*\|\s*v(\d+(?:\.\d+)+)',text,re.M) or re.search(r'^#+ .+?\bv(\d+(?:\.\d+)+)',text,re.M)
        if not match:raise ValueError('Document version missing: '+name)
        result.append({'id':name.lower(),'version':match[1],'kind':name,'file_path':str(path.resolve()),'content':text})
    return result


def anchored(source, phrase):
    lines = [(i,line) for i,line in enumerate(source['content'].splitlines(),1) if phrase in line]
    if not lines: raise ValueError('Oracle phrase not found: '+phrase)
    i,line = lines[0]
    return evidence(source['content'], source['id'], source['version'], i,line)

def oracle(sources):
    by = {s['id']:s for s in sources}
    expected = [
      {'id': 'FR-ADM-01', 'class': 'Requirement', 'source': 'srs', 'phrase': '| FR-ADM-01 | Hệ thống phải xác thực'},
      {'id': 'FR-INV-02', 'class': 'Requirement', 'source': 'srs', 'phrase': '| FR-INV-02 | Hệ thống phải có'},
      {'id': 'BRL-47', 'class': 'BusinessRule', 'source': 'urd', 'phrase': '| BRL-47 | Entitlement bắt buộc'},
      {'id': 'BRL-48', 'class': 'BusinessRule', 'source': 'urd', 'phrase': '| BRL-48 | Cảnh báo'},
      {'id': 'FR-AST-13', 'class': 'Requirement', 'source': 'srs', 'phrase': '| FR-AST-13 | Hệ thống phải cảnh báo'},
      {'id': 'M-06', 'class': 'Module', 'source': 'sad', 'phrase': '| M-06 | Asset Compliance'},
      {'id': 'ENT-35', 'class': 'Entity', 'source': 'srs', 'phrase': '| F | ENT-35'},
      {'id': 'ERR-invalid_request', 'class': 'ErrorCode', 'source': 'srs', 'phrase': '| 400 | `invalid_request`'},
      {'id': 'ENT-35:attribute:licensee', 'class': 'Attribute', 'source': 'urd', 'phrase': '| ENT-35 | Entitlement |'},
      {'id': 'SM-ENT-35', 'class': 'StateMachine', 'source': 'srs', 'phrase': '**ENT-35 Entitlement.** Trạng thái:'},
      {'id': 'WRK-03', 'class': 'Component', 'source': 'sad', 'phrase': '| WRK-03 | Scheduler'},
      {'id': 'ADR-01', 'class': 'Decision', 'source': 'sad', 'phrase': '| ADR-01 | Golang'}]
    for row in expected:
        row['evidence'] = anchored(by[row['source']], row['phrase'])
        if verify_evidence(row['evidence'], sources): raise ValueError('Invalid oracle anchor')
    return {'version': '1.0.0', 'source_hashes': {s['id']:source_digest(s['content']) for s in sources}, 'expected_nodes': expected,
            'expected_relations': [{'src':'FR-AST-13','rel':'traces_to','dst':'BRL-48'}],
            'expected_missing': ['Appendix A detailed schema', 'Appendix B endpoint contracts', 'Appendix C algorithms'],
            'new_prompt': 'Add an entitlement expiry reminder at T-14; retain T-60/T-30/T-7/T-0, exactly-once milestones and reset after renewal.',
            'new_behavior_search': {'outcome': 'not_present', 'searched_ids': ['BRL-48', 'FR-AST-13'], 'kind': 'modified', 'reason': 'Baseline enumerates 60/30/7/0, no 14-day milestone'}}

def curated_change(sources, oracle_record):
    """Design proposals are explicit; they do not masquerade as extracted facts."""
    srs = next(s for s in sources if s['id']=='srs'); sad = next(s for s in sources if s['id']=='sad')
    ev = anchored(srs, '| FR-AST-13 | Hệ thống phải cảnh báo')
    sad_ev = anchored(sad, '| WRK-03 | Scheduler')
    prompt = oracle_record['new_prompt']; fixture = {'id':'test-cr-t14', 'version':'1', 'content':prompt}
    prompt_ev = evidence(prompt, fixture['id'], fixture['version'],1,prompt)
    cap = 'entitlement-reminders'
    def node(ident, cls, name, props, evidence_records=None):
        return {'id':ident, 'class':cls, 'name':name, 'props':props, 'origin':'own', 'maturity':'specified', 'version_introduced':'1', 'review_status':'needs_review', 'provenance':evidence_records or [ev]}
    baseline = node('REQ-REMINDER-v1','RequirementVersion','Entitlement expiry reminders', {'logical_requirement_id':'FR-AST-13','revision':'1','statement':'The system SHALL remind subscription entitlement expiry at T-60, T-30, T-7 and T-0, once per milestone; renewal beyond 60 days resets reminded milestones and returns Active.','acceptance':'Once per milestone, renewal resets reminders.','modality':'must','baseline_id':'pbom-baseline','capability_id':cap})
    baseline_scenarios = []
    for n in (60,30,7,0):
        baseline_scenarios.append(node(f'SC-REMINDER-{n}','Scenario',f'Expiry milestone T-{n}',{'requirement_id':baseline['id'], 'given':f'an active subscription entitlement reaches T-{n} with this milestone not reminded', 'when':'the daily entitlement job runs', 'then':f'emit exactly one reminder for T-{n} and persist the milestone'}))
    baseline_scenarios += [node('SC-REMINDER-RENEW','Scenario','Renewal resets reminders',{'requirement_id':baseline['id'], 'given':'a subscription entitlement is renewed beyond 60 days', 'when':'renewal is committed', 'then':'return entitlement to Active and clear reminded milestones'})]
    base_graph = {'source_snapshots':{s['id']:{'version':s['version'],'source_hash':source_digest(s['content']),'file_path':s.get('file_path')} for s in sources}, 'version':'pbom-reminder-baseline', 'elements':[baseline,*baseline_scenarios], 'edges':[{'src':baseline['id'],'rel':'has_scenario','dst':n['id'],'provenance':[ev]} for n in baseline_scenarios]}
    version = deepcopy(baseline); version['id']='REQ-REMINDER-v2'; version['props']['revision']='2'; version['props']['statement']=baseline['props']['statement'].replace('T-60, T-30, T-7 and T-0','T-60, T-30, T-14, T-7 and T-0'); version['provenance'] += [prompt_ev]
    sc = node('SC-REMINDER-14','Scenario','Expiry milestone T-14',{'requirement_id':version['id'],'given':'an active subscription entitlement reaches T-14 with this milestone not reminded','when':'the daily entitlement job runs','then':'emit exactly one reminder for T-14 and persist the milestone'},[prompt_ev])
    sc_retry=node('SC-REMINDER-RETRY','Scenario','Repeated scheduler run', {'requirement_id':version['id'],'given':'a T-14 reminder was persisted','when':'the daily job is retried for the same entitlement and milestone','then':'emit no second reminder'},[ev,prompt_ev])
    decision=node('DEC-REMINDER-T14','Decision','Extend reminder milestone set', {'chosen':'Add 14 to the existing milestone policy; preserve renewal reset and exactly-once storage.', 'alternatives':'Separate T-14 job rejected because it duplicates retry coordination.', 'rationale':'Reuse the existing daily scheduler and reminded_milestones semantics.', 'migration':'No endpoint or entity schema change; update job milestone configuration and regression coverage.', 'status':'proposed'},[ev,sad_ev,prompt_ev])
    decision['provenance'].append({'kind':'proposed','decision_id':decision['id'],'source_id':fixture['id'],'source_version':'1','source_hash':source_digest(prompt),'block':'line:1'})
    job=node('JOB-ENTITLEMENT','Job','Daily entitlement expiry job', {'owner':'WRK-03', 'deadline':'before 06:00 Vietnam time', 'milestones':[60,30,14,7,0],'deduplication':'reminded_milestones','renewal_reset':'beyond 60 days'},[sad_ev,ev,prompt_ev])
    work=node('WI-REMINDER-T14','WorkItem','Extend daily reminder policy', {'deliverable':'Extend reminder milestones with T-14 while retaining T-60/T-30/T-7/T-0.', 'definition_of_done':'Tests cover all five milestones, retry deduplication and renewal reset.', 'dependencies':[], 'design_id':decision['id']},[ev,prompt_ev])
    test=node('TC-REMINDER-T14','TestCase','Milestone regression matrix', {'cases':['T-60','T-30','T-14','T-7','T-0','retry deduplication','renewal reset'], 'expected':'exactly one reminder per milestone, none on duplicate retry, reset after renewal'},[ev,prompt_ev])
    delta=node('DELTA-T14','RequirementDelta','Entitlement expiry reminders', {'kind':'modified','rationale':prompt,'change_request_id':'CR-T14','capability_id':cap,'before_requirement_id':baseline['id'],'after_requirement_id':version['id']},[prompt_ev])
    cr=node('CR-T14','ChangeRequest','Test fixture: add T-14 milestone', {'status':'proposed','requester':'test fixture','rationale':prompt},[prompt_ev])
    capability=node(cap,'Capability','Entitlement reminders', {'stable_key':cap,'purpose':'subscription expiry reminders','scope':'daily reminder policy','owner':'Asset Compliance','lifecycle':'active'},[ev])
    for generated in (job,work,test):
        generated['provenance'] = [{**generated['provenance'][0], 'kind':'derived', 'inputs':deepcopy(generated['provenance']), 'derivation':'Curated test design from source behavior and explicit T-14 fixture; requires review'}]
    scheduler=node('WRK-03','Component','Scheduler',{'responsibility':'Daily entitlement jobs before 06:00 Vietnam time'},[sad_ev]);scheduler['origin']='upstream'
    elements=[capability,cr,version,sc,sc_retry,decision,job,scheduler,work,test,delta]
    edges=[{'src':delta['id'],'rel':'before_requirement','dst':baseline['id']},{'src':delta['id'],'rel':'after_requirement','dst':version['id']},{'src':delta['id'],'rel':'for_change','dst':cr['id']},{'src':cap,'rel':'has_requirement','dst':version['id']},{'src':version['id'],'rel':'has_scenario','dst':sc['id']},{'src':version['id'],'rel':'has_scenario','dst':sc_retry['id']},{'src':version['id'],'rel':'decided_by','dst':decision['id']},{'src':job['id'],'rel':'satisfies','dst':version['id']}]
    for req in [baseline,version]:
        edges += [{'src':job['id'],'rel':'satisfies','dst':req['id']},{'src':req['id'],'rel':'verified_by','dst':test['id']}]
    edges.append({'src':job['id'],'rel':'implemented_by','dst':work['id']})
    edges.append({'src':job['id'],'rel':'depends_on','dst':scheduler['id']})
    for edge in edges: edge['provenance']=[ev,prompt_ev]
    request={'id':'CR-T14','intent':prompt,'statement':version['props']['statement'],'baseline_ids':[baseline['id']], 'comparison':{**oracle_record['new_behavior_search'],'intent_hash':fingerprint(prompt),'base_hash':fingerprint(base_graph)}, 'elements':elements,'edges':edges}
    return base_graph, request, fixture


def check_oracle(graph, record):
    by = {n['id']:n for n in graph['elements']}; misses=[]
    for n in record['expected_nodes']:
        if n['id'] not in by or by[n['id']]['class']!=n['class']: misses.append(n['id'])
    for edge in record['expected_relations']:
        if not any(all(e.get(k)==v for k,v in edge.items()) for e in graph['edges']): misses.append(str(edge))
    return {'ok':not misses,'missing':misses,'expected_node_count':len(record['expected_nodes']), 'actual_matched':len(record['expected_nodes'])-sum(n['id'] in misses for n in record['expected_nodes'])}
