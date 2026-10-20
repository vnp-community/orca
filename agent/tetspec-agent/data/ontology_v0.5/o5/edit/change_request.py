"""CR planning consumes a reviewed semantic comparison, never ID counts."""
from copy import deepcopy
from semantic_intake import classify_cr
from transaction5 import apply_changeset, snapshot_hash
from quality5 import graph_checks


def plan_cr(graph, request):
    assessment = classify_cr(request['intent'], graph, request.get('baseline_ids'), request.get('statement'), request.get('comparison'))
    if assessment['status'] == 'no_op': return {'status': 'no_op', 'assessment': assessment, 'snapshot': snapshot_hash(graph)}
    if assessment['status'] != 'proposed': return {'status': 'blocked', 'assessment': assessment}
    if assessment['kind'] == 'defect' and (not request.get('reproduction') or not request.get('expected_behavior')):
        return {'status':'blocked','reason':'Defect needs reproduction and expected behavior evidence'}
    if not request.get('elements') or not request.get('edges'):
        return {'status': 'blocked', 'reason': 'Reviewed requirements/design/scenarios/work/tests are required', 'assessment': assessment}
    required = {'RequirementVersion', 'RequirementDelta', 'Scenario', 'WorkItem', 'TestCase'}
    classes = {n['class'] for n in request['elements']}
    if not required <= classes: return {'status': 'blocked', 'reason': 'missing change implementation spine', 'missing': sorted(required-classes)}
    if not any(n['class'] not in required for n in request['elements']): return {'status': 'blocked', 'reason': 'affected design missing'}
    ops = [{'seq': i, 'op': 'add', 'after': deepcopy(n)} for i, n in enumerate(request['elements'], 1)]
    ops += [{'seq': len(ops)+i, 'op': 'link', 'after': deepcopy(e)} for i,e in enumerate(request['edges'], 1)]
    return {'status': 'planned', 'assessment': assessment, 'change_set': {'id': 'CS-'+request['id'], 'base_version': graph['version'], 'result_version': graph['version']+'+'+request['id'], 'ops': ops}}


def commit_cr(graph, plan):
    if plan['status'] != 'planned': return {**plan, 'graph': graph}
    result, validation = apply_changeset(graph, plan['change_set'])
    if not validation['ok']: return {'status': 'rejected', 'graph': graph, 'validation': validation}
    checks = graph_checks(result)
    blockers = [key for key, record in checks.items() if record['outcome'] != 'pass']
    return {'status': 'committed', 'graph': result, 'validation': validation,
            'readiness': {'status': 'draft' if blockers else 'implementation_ready', 'blockers': blockers, 'rule_coverage': checks}}
