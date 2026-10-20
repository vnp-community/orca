#!/usr/bin/env python3
"""PBOM regression and real propose authoring benchmark; scores require human review."""
import argparse, json, sys
from pathlib import Path
from copy import deepcopy
ROOT=Path(__file__).resolve().parents[1]
for part in ('tools','edit','harness','specgen'):sys.path.insert(0,str(ROOT/part))
from pbom_oracle import load_sources,oracle,curated_change,check_oracle,DOCS
from semantic_intake import build_graph,classify_cr
from quality5 import graph_checks,fingerprint
from quality_eval import counts,scoped_comparison,review_template,evaluate_reviews
from ontology5 import load as ontology_manifest
from render import render_tdd,round_trip,publish_tdd
from prompt_edit import EditRequest,plan_edit,plan_prompt,commit_edit
from change_request import plan_cr,commit_cr
from spec_change import SpecChangeRequest,repo_context,build_pack,propose_instructions,validate_artifacts,validate_with_cli
from authoring import CodexProposeRunner


def save(path, value):
    Path(path).parent.mkdir(parents=True,exist_ok=True)
    Path(path).write_text(json.dumps(value,indent=2,ensure_ascii=False)+'\n')


def main():
    parser=argparse.ArgumentParser();parser.add_argument('--run-dir',default=str(DOCS/'run-quality-full'));parser.add_argument('--benchmark',action='store_true');parser.add_argument('--timeout',type=int,default=180);parser.add_argument('--runner',default='codex');args=parser.parse_args()
    out=Path(args.run_dir).resolve();out.mkdir(parents=True,exist_ok=True)
    sources=load_sources();record=oracle(sources);save(out/'oracle.json',record)
    graph=build_graph(sources);save(out/'initial.kg.json',graph)
    graph['ontology_version'] = ontology_manifest()['version']; save(out/'initial.kg.json', graph)
    checks=graph_checks(graph,sources);blockers=[k for k,v in checks.items() if v['outcome']!='pass']
    ready={'status':'draft' if blockers else 'implementation_ready','blockers':blockers,'rule_coverage':checks}
    render=publish_tdd(graph,out/'TDD-PBOM.md',ready);save(out/'initial.readiness.json',ready)
    oracle_result=check_oracle(graph,record);save(out/'oracle.check.json',oracle_result)
    req=EditRequest('PROMPT-SBOM','Set Submit SBOM timeout to 25 seconds',['API-I-01'],graph['version'],{'scope':['API-I-01'],'operations':['modify']})
    # Timeout is a declared global contract constraint; provenance retained from SAD boundary policy.
    sad=next(s for s in sources if s['id']=='sad')
    from pbom_oracle import anchored
    operation=next(n for n in graph['elements'] if n['id']=='API-I-01')
    operation['props']['timeout_seconds']=30;operation['props']['upstream_value']=30
    operation['provenance'].append(anchored(sad,'| Timeout | 30 giây mỗi request'))
    plan=plan_prompt(req,graph)
    edit=commit_edit(req,graph,plan);save(out/'prompt.upstream-conflict.json',edit)
    # Explicit simulated review policy approves a TDD-local deviation; upstream
    # value/evidence remain intact. This is a test fixture, not production approval.
    local=deepcopy(graph)
    from transaction5 import snapshot_hash
    req.authorization['deviation_resolution']={'status':'approved','decision_id':'TEST-DEC-TIMEOUT','approved_by':'simulated-test-policy','prompt_hash':req.prompt_hash,'base_hash':snapshot_hash(local)}
    local_edit=commit_edit(req,local,plan_prompt(req,local));save(out/'prompt.local-draft.json',local_edit)
    if local_edit.get('graph'): (out/'TDD-PBOM.after-prompt.md').write_text('# DRAFT: simulated policy deviation; upstream unchanged\n'+render_tdd(local_edit['graph']))
    duplicate_intent='Require entitlement evidence'
    brl=next(n for n in graph['elements'] if n['id']=='BRL-47')
    matched=brl['props']['predicate']
    duplicate=classify_cr(duplicate_intent,graph,['BRL-47','FR-AST-12'],comparison={'outcome':'equivalent','matched_statement':matched,'review_basis':'Curated simulation checks BRL-47 and FR-AST-12 evidence','intent_hash':fingerprint(duplicate_intent),'base_hash':fingerprint(graph)});save(out/'duplicate-cr.json',duplicate)
    base,cr,fixture=curated_change(sources,record);plan=plan_cr(base,cr);changed=commit_cr(base,plan);save(out/'new-cr.json',changed)
    if changed['status']!='committed':raise RuntimeError('Curated CR rejected')
    changed_graph=changed['graph'];(out/'TDD-reminders.after-cr.md').write_text(render_tdd(changed_graph))
    old=json.loads((DOCS.parent/'ontology_v0.4/o4/kg4.json').read_text());save(out/'kg-comparison.json',scoped_comparison(old,graph,record))
    summary={'ontology_version': ontology_manifest()['version'], 'initial':counts(graph),'readiness':ready['status'],'oracle':oracle_result,'round_trip':render['round_trip'],'prompt_upstream':edit['status'],'prompt_local':local_edit['status'],'duplicate_cr':duplicate['status'],'new_cr':changed['status'],'new_cr_readiness':changed['readiness']['status'],'benchmark':'not_requested'}
    save(out/'summary.json',summary);print(json.dumps({k:v for k,v in summary.items() if k != 'initial'} | {'nodes':summary['initial']['nodes'],'edges':summary['initial']['edges']},ensure_ascii=False),flush=True)
    if not args.benchmark:return
    repo=out/'benchmark-repo';(repo/'openspec/specs/entitlement-reminders').mkdir(parents=True,exist_ok=True)
    (repo/'openspec/config.yaml').write_text('schema: spec-driven\ncontext: |\n  PBOM fixture. Preserve baseline semantics, source provenance and concrete scenarios.\n')
    before=base['elements'][0];spec='## Requirements\n\n### Requirement: Entitlement expiry reminders\n\n'+before['props']['statement']+'\n'
    for sc in base['elements'][1:]:
        sp=sc['props'];spec+=f"\n#### Scenario: {sc['name']}\n\n- **GIVEN** {sp['given']}\n- **WHEN** {sp['when']}\n- **THEN** {sp['then']}\n"
    (repo/'openspec/specs/entitlement-reminders/spec.md').write_text(spec)
    code_paths=[]
    for s in sources:
        excerpts=[]
        for row in record['expected_nodes']:
            if row['source']==s['id']:excerpts.append(row['evidence']['block']+': '+row['evidence']['quote'])
        if s['id']=='sad':excerpts+=[anchored(s,'| WRK-03 | Scheduler')['quote']]
        rel='sources/'+s['id']+'.md';(repo/rel).parent.mkdir(exist_ok=True);(repo/rel).write_text('\n'.join(excerpts));code_paths.append(rel)
    request=SpecChangeRequest('SPEC-T14','add-entitlement-t14',record['new_prompt'],['DELTA-T14'],changed_graph['version'],str(repo),code_paths=code_paths)
    pack=build_pack(request,changed_graph,repo_context(repo,code_paths));save(out/'benchmark-context.json',pack)
    runs=[];runner=CodexProposeRunner(args.runner,timeout=args.timeout,max_attempts=1)
    for mode in ('direct','kg'):
        for number in range(1,4):
            ident=f'{mode}-{number}';folder=out/'benchmark'/ident;folder.mkdir(parents=True,exist_ok=True)
            instructions=propose_instructions(repo,request.change_name,folder)
            input_pack=pack if mode=='kg' else {'request':pack['request'],'repo':pack['repo'],'raw_change_request':record['new_prompt'],'baseline':pack['baseline']}
            artifacts,assertions,execution=runner.author(input_pack,folder/'authoring',instructions)
            for rel,text in artifacts.items():
                path=Path(rel)
                if path.is_absolute() or '..' in path.parts:raise ValueError('unsafe output path')
                target=folder/'artifacts'/rel;target.parent.mkdir(parents=True,exist_ok=True);target.write_text(text)
            validation=validate_artifacts(artifacts,pack,{'status':'draft'},assertions,execution.get('proposed_decisions')) if artifacts else {'ok':False,'errors':['AUTHORING_FAILED']}
            semantic_review=runner.review(input_pack,artifacts,assertions,folder/'review',proposed_decisions=execution.get('proposed_decisions',[])) if artifacts else {'status':'blocked','outcome':'unknown'}
            item={'semantic_review':semantic_review,'id':ident,'mode':mode,'status':execution['status'],'output_hash':execution.get('output_hash'),'semantic_validation':validation,'execution':execution,'artifact_dir':str(folder/'artifacts')}
            save(folder/'report.json',item);runs.append(item);save(out/'benchmark.runs.json',runs)
            print(json.dumps({'run':ident,'status':execution['status'],'semantic_valid':validation['ok']},ensure_ascii=False),flush=True)
    review=review_template(runs);save(out/'benchmark.review.json',review)
    result=evaluate_reviews(runs,review);save(out/'benchmark.result.json',result)
    summary['benchmark']=result;save(out/'summary.json',summary);print(json.dumps(result),flush=True)

if __name__=='__main__':main()
