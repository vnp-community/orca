"""Scoped coverage and human benchmark rubric. No synthetic reviewer scores."""
from collections import Counter
from statistics import median
from quality5 import fingerprint

DIMENSIONS = ['correctness','scenario_coverage','implementability','trace_fidelity','scope_control','design_decisions','task_completeness']
REVIEW_LEVELS = ('L1', 'L2')

def counts(graph):
    return {'nodes':len(graph.get('elements', [])), 'node_types':dict(sorted(Counter(n.get('class',n.get('type')) for n in graph.get('elements', [])).items())), 'edges':len(graph.get('edges', [])), 'edge_types':dict(sorted(Counter(e['rel'] for e in graph.get('edges', [])).items()))}

def scoped_comparison(original, generated, oracle):
    scope={n['id'] for n in oracle['expected_nodes']}
    def project(graph):
        return {'elements':[n for n in graph['elements'] if n['id'] in scope], 'edges':[e for e in graph['edges'] if e['src'] in scope and e['dst'] in scope]}
    left, right = project(original), project(generated)
    old={n['id']:n for n in left['elements']}; new={n['id']:n for n in right['elements']}
    mapped=set(old)&set(new)
    return {'scope': sorted(scope), 'mapping':sorted(mapped), 'coverage':'unknown' if mapped!=scope else 'mapped IDs only; compare evidence/property differences below', 'original_total':counts(original), 'generated_total':counts(generated), 'original_scoped':counts(left),'generated_scoped':counts(right), 'presentation_nodes':sum(n.get('class') in ('Block','Section','Document') for n in original['elements']), 'out_of_scope_original':len(original['elements'])-len(left['elements']), 'differences':{i:{'class_before':old[i].get('class'), 'class_after':new[i].get('class'), 'properties_equal':old[i].get('props')==new[i].get('props'), 'evidence_equal':old[i].get('provenance',old[i].get('evidence'))==new[i].get('provenance')} for i in sorted(mapped)}}

def review_template(runs, review_level='L2', review_id=None, supersedes_review_id=None):
    """Create a review form without supplying reviewer judgments.

    L1 is diagnostic evidence. Only L2, completed by an independent human,
    can satisfy benchmark acceptance or make a change archive-eligible. The
    generic helper defaults to L2 for compatibility; D5 preparation explicitly
    creates an L1 form.
    """
    if review_level not in REVIEW_LEVELS:
        raise ValueError('INVALID_REVIEW_LEVEL:' + str(review_level))
    return {
        'version':'1.1.0',
        'review_id': review_id or ('D5-' + review_level + '-pending'),
        'review_level': review_level,
        'acceptance_level': 'L2',
        'supersedes_review_id': supersedes_review_id,
        'reviewer':{'kind':'human','identity':None, 'independent_of_generation':None},
        'rubric':{d:'0 absent/wrong, 1 major gaps, 2 partial, 3 implementable with minor gaps, 4 complete and grounded' for d in DIMENSIONS},
        'reviews':[{'run_id':r['id'],'output_hash':r.get('output_hash'),'critical_errors':None,'missing_mandatory_behavior':None,'scores':{d:None for d in DIMENSIONS}, 'findings':[]} for r in runs]
    }

def evaluate_reviews(runs, review):
    failed=[r['id'] for r in runs if r.get('status')!='authored' or not r.get('output_hash')]
    if failed: return {'status':'blocked_authoring','blockers':['AUTHORING_FAILED:'+i for i in failed]}
    human=review.get('reviewer', {})
    # Pre-tier files with a verified independent human are treated as L2 for
    # backward compatibility. New forms carry the level explicitly.
    level=review.get('review_level')
    if level is None:
        level='L2' if human.get('kind') == 'human' and human.get('independent_of_generation') is True else 'L1'
    if level not in REVIEW_LEVELS:
        return {'status':'blocked_review','review_level':level,'blockers':['INVALID_REVIEW_LEVEL:'+str(level)]}
    if level == 'L2' and (human.get('kind')!='human' or not human.get('identity') or human.get('independent_of_generation') is not True):
        return {'status':'pending_human_review','review_level':'L2','blockers':['INDEPENDENT_HUMAN_REVIEW_REQUIRED']}
    if level == 'L1' and not human.get('identity'):
        return {'status':'pending_l1_review','review_level':'L1','blockers':['L1_REVIEW_REQUIRED']}
    groups={mode:[r for r in runs if r['mode']==mode] for mode in ('direct','kg')}
    errors=[]; scores={mode:{d:[] for d in DIMENSIONS} for mode in groups}
    reviews={r['run_id']:r for r in review.get('reviews', [])}
    for mode,items in groups.items():
        if len(items)<3: errors.append('INSUFFICIENT_RUNS:'+mode)
        for run in items:
            rating=reviews.get(run['id'], {})
            if run.get('status')!='authored': errors.append('AUTHORING_FAILED:'+run['id'])
            if not run.get('output_hash') or rating.get('output_hash')!=run['output_hash']: errors.append('REVIEW_HASH_MISMATCH:'+run['id'])
            if rating.get('critical_errors') not in (True, False) or rating.get('missing_mandatory_behavior') not in (True, False): errors.append('CRITICAL_OR_UNREVIEWED:'+run['id'])
            for d in DIMENSIONS:
                v=rating.get('scores',{}).get(d)
                if isinstance(v,bool) or not isinstance(v,(int,float)) or not 0<=v<=4: errors.append('INVALID_SCORE:'+run['id']+':'+d)
                else:scores[mode][d].append(v)
    medians={mode:{d:median(v) if v else None for d,v in dims.items()} for mode,dims in scores.items()}
    for d in DIMENSIONS:
        direct,kg=medians['direct'][d],medians['kg'][d]
        if direct is None or kg is None or kg<3 or direct<3 or kg<direct-0.5: errors.append('QUALITY_THRESHOLD:'+d)
    result={'status':'pass' if not errors else 'fail','review_level':level,'blockers':sorted(set(errors)), 'medians':medians, 'review_hash':fingerprint(review), 'review_id':review.get('review_id'), 'supersedes_review_id':review.get('supersedes_review_id')}
    if level == 'L1':
        result['status']='provisional_l1' if not errors else 'provisional_l1_fail'
        result['blockers']=sorted(set(result['blockers'] + ['L2_INDEPENDENT_HUMAN_REVIEW_REQUIRED']))
    return result
