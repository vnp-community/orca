"""TS-01..TS-10 implement the named tech-spec contract, not generic graph checks."""
from quality5 import execute, known


def evaluate(pack, quality, artifacts=None, validation=None, cli=None, review=None):
    graph=pack.get('zone_graph') or pack['graph'];nodes={n['id']:n for n in graph['elements']};edges=graph['edges'];deltas=pack['deltas'];results={}
    def run(ident, check): results[ident]=execute(ident,{'pack':pack,'artifacts':artifacts,'validation':validation,'cli':cli,'review':review},lambda _:check())
    def capabilities():
        caps=[n for n in nodes.values() if n['class']=='Capability']
        return (['CAPABILITY_MISSING'] if not caps else [])+[n['id']+':'+field for n in caps for field in ('purpose','scope','owner') if not known(n.get('props',{}).get(field))]
    run('TS-01',capabilities)
    def delta_versions():
        errors=[]
        for delta in deltas:
            p=delta['props'];kind=p.get('kind');before=nodes.get(p.get('before_requirement_id'));after=nodes.get(p.get('after_requirement_id'))
            if kind not in ('added','modified','removed','renamed'):errors.append(delta['id']+':invalid kind')
            if kind in ('modified','removed','renamed') and not before:errors.append(delta['id']+':before missing')
            if kind in ('added','modified','renamed') and not after:errors.append(delta['id']+':after missing')
            if kind=='added' and before:errors.append(delta['id']+':added has before')
            if before and after and (before.get('props',{}).get('logical_requirement_id')!=after.get('props',{}).get('logical_requirement_id') or before['id']==after['id']):errors.append(delta['id']+':unstable identity/version')
            if before and not before.get('props',{}).get('baseline_id'):errors.append(delta['id']+':baseline missing')
            if kind=='renamed' and not all(p.get(k) for k in ('old_name','new_name')):errors.append(delta['id']+':rename names missing')
            if before:
                baseline=pack['repo'].get('contents',{}).get('openspec/specs/'+str(p.get('capability_id'))+'/spec.md','')
                statement=before.get('props',{}).get('statement')
                if statement and statement not in baseline:errors.append(delta['id']+':KG before statement conflicts with repo baseline')
        return errors
    run('TS-02',delta_versions)
    def requirements():
        errors=[]
        for delta in deltas:
            p=delta['props'];kind=p.get('kind')
            if kind=='removed':
                if not all(p.get(k) for k in ('rationale','migration')):errors.append(delta['id']+':removal rationale/migration missing')
                continue
            node=nodes.get(p.get('after_requirement_id'),{});props=node.get('props',{})
            for field in ('statement','modality','acceptance'):
                if not known(props.get(field)):errors.append(delta['id']+':'+field+' missing')
            scenarios=[n for n in nodes.values() if n['class']=='Scenario' and n.get('props',{}).get('requirement_id')==node.get('id')]
            if not scenarios:errors.append(delta['id']+':scenarios missing')
            for scenario in scenarios:
                for field in ('given','when','then'):
                    if not known(scenario['props'].get(field)):errors.append(scenario['id']+':'+field+' missing')
        return errors
    run('TS-03',requirements)
    run('TS-04',lambda:[key+':'+str(record['evidence']) for key,record in quality.items() if record['outcome']!='pass'])
    def designs():
        errors=[];design_ids={n['id'] for n in pack['design']}
        for delta in deltas:
            p=delta['props'];subject=p.get('after_requirement_id') or p.get('before_requirement_id')
            if not any((e['src'] in (subject,delta['id']) and e['dst'] in design_ids) or (e['dst']==subject and e['rel']=='satisfies' and e['src'] in design_ids) for e in edges):errors.append(delta['id']+':design trace missing')
        for n in pack['design']:
            if n['class']=='Decision' and not known(n.get('props',{}).get('rationale')):errors.append(n['id']+':rationale missing')
        return errors
    run('TS-05',designs)
    def work():
        errors=[];work_ids={n['id'] for n in pack['work']};adj={i:[] for i in work_ids}
        for n in pack['work']:
            for dependency in n.get('props',{}).get('dependencies',[]):
                if dependency not in work_ids:errors.append(n['id']+':dangling dependency')
                else:adj[n['id']].append(dependency)
            if not all(known(n.get('props',{}).get(k)) for k in ('deliverable','definition_of_done')):errors.append(n['id']+':deliverable/DoD missing')
        for e in edges:
            if e['rel']=='depends_on_work':
                if e['src'] not in work_ids or e['dst'] not in work_ids:errors.append('dangling work edge')
                else:adj[e['src']].append(e['dst'])
        visiting=set();done=set()
        def visit(i):
            if i in visiting:return True
            if i in done:return False
            visiting.add(i)
            if any(visit(j) for j in adj.get(i,[])):return True
            visiting.remove(i);done.add(i);return False
        if any(visit(i) for i in work_ids):errors.append('work cycle')
        for delta in deltas:
            p=delta['props'];subject=p.get('after_requirement_id') or p.get('before_requirement_id')
            for relation,cls in (('implemented_by','WorkItem'),('verified_by','TestCase')):
                if not any(e['src'] in ({subject}|{link['src'] for link in edges if link['rel']=='satisfies' and link['dst']==subject}) and e['rel']==relation and nodes.get(e['dst'],{}).get('class')==cls for e in edges):errors.append(delta['id']+':'+relation+' missing')
        return errors
    run('TS-06',work)
    run('TS-07',lambda:(['CONTEXT_TRUNCATED'] if pack['impact']['truncated'] else [])+[e['src']+'→'+e['dst'] for e in edges if e['src'] not in nodes or e['dst'] not in nodes])
    run('TS-08',lambda:[str(x) for x in graph.get('conflicts',[]) if x.get('status')!='resolved']+[str(x) for x in graph.get('gaps',[]) if x.get('status') not in ('resolved','not_applicable')]+[n['id'] for n in nodes.values() if n['class'] in ('Assumption','OpenIssue') and n.get('props',{}).get('status') not in ('approved','resolved')])
    run('TS-09',lambda:(validation or {}).get('errors',['ARTIFACT_SEMANTIC_VALIDATION_NOT_EXECUTED'])+([] if review and review.get('status')=='reviewed' and review.get('outcome')=='pass' else ['SEMANTIC_REVIEW_NOT_PASSED']))
    def target():
        errors=[]
        if not cli or not cli.get('ok'):errors.append('CLI_VALIDATION_NOT_PASSED')
        for delta in deltas:
            p=delta['props'];cap=p.get('capability_id');path=f'specs/{cap}/spec.md'
            if not artifacts or path not in artifacts:errors.append(path+':artifact missing')
            if p.get('kind') in ('modified','removed','renamed'):
                baseline=pack['repo'].get('contents',{}).get('openspec/specs/'+str(cap)+'/spec.md','')
                name=delta.get('name')
                if not baseline or ('### Requirement: '+str(name)) not in baseline:errors.append(str(cap)+':target requirement missing from repository baseline')
        return errors
    run('TS-10',target)
    return results
