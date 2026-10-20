"""Executed quality checks; no presence-only readiness certification."""
from hashlib import sha256
import json

def fingerprint(value):
    return sha256(json.dumps(value, sort_keys=True, ensure_ascii=False).encode()).hexdigest()

def evidence_errors(record):
    errors = []
    if record.get('kind') == 'proposed':
        if not record.get('decision_id'): errors.append('MISSING_DECISION')
    else:
        for field in ('source_id', 'source_version', 'source_hash', 'block'):
            if not record.get(field): errors.append('MISSING_' + field.upper())
    return errors

def execute(rule_id, inputs, validator):
    try:
        errors = validator(inputs)
        return {'rule_id': rule_id, 'version': '1', 'input_hash': fingerprint(inputs),
                'outcome': 'fail' if errors else 'pass', 'evidence': errors or ['validator completed']}
    except Exception as exc:
        return {'rule_id': rule_id, 'version': '1', 'input_hash': fingerprint(inputs),
                'outcome': 'error', 'evidence': [str(exc)]}

def assess_legacy(report):
    return {'status': 'unverified', 'original_hash': fingerprint(report),
            'reason': 'Legacy readiness requires executed semantic revalidation', 'original_status': report.get('status')}

QUALITY_VERSION = '1.0.0'

def verify_evidence(record, sources=None):
    errors = evidence_errors(record)
    if record.get('kind') == 'derived':
        if not record.get('inputs') or not record.get('derivation'): errors.append('MISSING_DERIVATION_LINEAGE')
    if record.get('kind') == 'prompt':
        if not record.get('prompt_hash'): errors.append('MISSING_PROMPT_HASH')
        return errors
    if record.get('kind') == 'proposed': return errors
    if sources is not None:
        src = next((s for s in sources if s['id'] == record.get('source_id')), None)
        if not src: return errors + ['SOURCE_NOT_FOUND']
        from semantic_intake import source_digest
        if str(src.get('version')) != str(record.get('source_version')): errors.append('SOURCE_VERSION_MISMATCH')
        if source_digest(src['content']) != record.get('source_hash'): errors.append('SOURCE_HASH_MISMATCH')
        try:
            line = int(record['block'].split(':')[1]); quote = src['content'].splitlines()[line-1]
            if record.get('quote') != quote or source_digest(quote) != record.get('quote_hash'): errors.append('QUOTE_MISMATCH')
        except (KeyError, ValueError, IndexError): errors.append('ANCHOR_INVALID')
    return errors

def typed_closure(graph, seeds, budget=None):
    membership = {'has_requirement', 'requirement_of'}
    links = {'depends_on', 'depends_on_work', 'satisfies', 'implemented_by', 'verified_by', 'has_scenario',
             'constrained_by', 'references_entity', 'reads', 'writes', 'may_fail_with', 'has_attribute',
             'models_state_of', 'has_state', 'traces_to', 'in_machine', 'transition_from', 'transition_to', 'before_requirement', 'after_requirement', 'decided_by'}
    ids = set(seeds); by_id = {n['id']: n for n in graph.get('elements', [])}
    for seed in seeds:
        if by_id.get(seed, {}).get('class') == 'Capability':
            ids.update(e['dst'] for e in graph.get('edges', []) if e['src'] == seed and e['rel'] in membership)
    reasons = {}
    while True:
        previous = set(ids)
        for e in graph.get('edges', []):
            if e['rel'] in links and (e['src'] in ids or e['dst'] in ids):
                # A shared read-only dependency must not pull in every unrelated consumer.
                if e['src'] in ids:
                    ids.add(e['dst']); reasons.setdefault(e['dst'], []).append(e['rel'])
                if e['dst'] in ids and (e['rel'] in ('satisfies','depends_on','depends_on_work','models_state_of','has_state','in_machine','transition_from','transition_to') or by_id.get(e['src'], {}).get('class') in ('Scenario', 'WorkItem', 'TestCase', 'RequirementDelta')):
                    ids.add(e['src']); reasons.setdefault(e['src'], []).append(e['rel'])
        if previous == ids: break
    ordered = sorted(ids)
    omitted = ordered[budget:] if budget is not None else []
    return {'ids': ordered[:budget] if budget is not None else ordered, 'missing_ids': omitted,
            'truncated': bool(omitted), 'reasons': {k: sorted(set(v)) for k, v in reasons.items()}}

def known(value):
    if value is None or value == "" or value == [] or value == {}: return False
    if isinstance(value, str) and value.lower() in ("unknown", "from-contract", "tbd", "pending"): return False
    if isinstance(value, dict) and value.get("status", value.get("slot_status")) in ("unknown", "missing", "proposed", "conflict", "stale"): return False
    return True


def source_snapshot_errors(graph):
    from pathlib import Path
    from semantic_intake import source_digest
    errors=[]
    for ident, snapshot in graph.get('source_snapshots', {}).items():
        path=snapshot.get('file_path')
        if not path: continue
        try:
            if source_digest(Path(path).read_text()) != snapshot['source_hash']: errors.append(ident+':stale source snapshot')
        except (OSError, KeyError): errors.append(ident+':source snapshot unavailable')
    return errors


def in_change(g, node):
    """In a change pack the acceptance a requirement needs is the one its changed version carries (from the CR): the logical requirement the SRS row defines, and the
    neighbours the closure reached, are not asked for it.  Without deltas (a generation) every requirement is asked."""
    deltas = [x for x in g.get('elements', []) if x.get('class') == 'RequirementDelta']
    if not deltas:
        return True
    own = {i for x in deltas for i in (x.get('props', {}).get('before_requirement_id'), x.get('props', {}).get('after_requirement_id')) if i}
    return node.get('class') == 'RequirementVersion' and node['id'] in own


def graph_checks(graph, sources=None):
    from envelope5 import validate_graph
    by_id = {x['id']: x for x in graph.get('elements', [])}
    design_sources = {}
    linked_classes = {}
    for edge in graph.get('edges', []):
        if edge['rel'] == 'satisfies':
            design_sources.setdefault(edge['dst'], set()).add(edge['src'])
        linked_classes.setdefault((edge['src'], edge['rel']), set()).add(by_id.get(edge['dst'], {}).get('class'))
    def missing_work_test(g):
        errors = []
        # a version a RequirementDelta replaces is history: the work and the test belong to the version that replaces it
        deltas = [x for x in g.get('elements', []) if x.get('class') == 'RequirementDelta']
        superseded = {x.get('props', {}).get('before_requirement_id') for x in deltas} - {None}
        # in a change pack only what the change itself modifies needs work and a test; the neighbours the closure happens to reach are not its business
        own = {i for x in deltas for i in (x.get('props', {}).get('before_requirement_id'), x.get('props', {}).get('after_requirement_id')) if i}
        own |= {x.get('props', {}).get('logical_requirement_id') for x in g.get('elements', []) if x['id'] in own} - {None}
        for node in g.get('elements', []):
            if node.get('class') not in ('Requirement', 'RequirementVersion', 'Operation') or node['id'] in superseded or (deltas and node['id'] not in own):
                continue
            sources_for_node = {node['id']} | design_sources.get(node['id'], set())
            for relation, target in (('implemented_by', 'WorkItem'), ('verified_by', 'TestCase')):
                if not any(target in linked_classes.get((src, relation), set()) for src in sources_for_node):
                    errors.append(f"{node['id']}:work or test link missing")
        return errors
    checks = {}
    checks['TS-01'] = execute('TS-01', graph, lambda g: (['EMPTY_SEMANTIC_GRAPH'] if not g.get('elements') else []) + [e['message'] for e in validate_graph(g)['errors']])
    checks['TS-02'] = execute('TS-02', graph, lambda g: source_snapshot_errors(g) + [f"{x['id']}:missing evidence" for x in g.get('elements', []) if not x.get('provenance')] +
        [f"{x['id']}:{err}" for x in g.get('elements', []) for ev in x.get('provenance', []) for err in verify_evidence(ev, sources)])
    checks['TS-03'] = execute('TS-03', graph, lambda g: [f"{x['id']}:review required" for x in g.get('elements', []) if x.get('review_status') != 'approved'])
    checks['TS-04'] = execute('TS-04', graph, lambda g: [str(x) for x in g.get('conflicts', []) if x.get('status') != 'resolved'] + [f"{x['id']}:pending assumption" for x in g.get('elements', []) if x.get('class') in ('Assumption', 'OpenIssue') and x.get('props', {}).get('status') not in ('resolved', 'approved')])
    checks['TS-05'] = execute('TS-05', graph, lambda g: [f"{x['id']}:contract {field} missing" for x in g.get('elements', []) if x.get('class') == 'Operation' for field in ('method', 'path', 'auth', 'errors', 'request', 'response') if not known(x.get('props', {}).get(field))] + [f"{x['id']}:write behavior {field} missing" for x in g.get('elements', []) if x.get('class') == 'Operation' and x.get('props', {}).get('method') in ('POST', 'PUT', 'PATCH', 'DELETE') for field in ('idempotency', 'concurrency') if not known(x.get('props', {}).get(field))] + [f"{x['id']}:transaction behavior {field} missing" for x in g.get('elements', []) if x.get('class') == 'Operation' and x.get('props', {}).get('transactional') for field in ('timeout', 'retry', 'failure_policy') if not known(x.get('props', {}).get(field))])
    checks['TS-06'] = execute('TS-06', graph, lambda g: [f"{x['id']}:acceptance missing" for x in g.get('elements', []) if x.get('class') in ('Requirement', 'RequirementVersion') and not known(x.get('props', {}).get('acceptance')) and in_change(g, x)] + [f"{x['id']}:scenario {field} missing" for x in g.get('elements', []) if x.get('class') == 'Scenario' for field in ('given', 'when', 'then') if not known(x.get('props', {}).get(field))])
    checks['TS-07'] = execute('TS-07', graph, missing_work_test)
    checks['TS-08'] = execute('TS-08', graph, lambda g: [str(x) for x in g.get('gaps', []) if x.get('status') not in ('resolved', 'not_applicable')])
    checks['TS-09'] = execute('TS-09', graph, lambda g: [f"{x['id']}:work acceptance missing" for x in g.get('elements', []) if x.get('class') == 'WorkItem' and (not known(x.get('props', {}).get('deliverable')) or not known(x.get('props', {}).get('definition_of_done')) or not isinstance(x.get('props', {}).get('dependencies'), list))])
    checks['TS-10'] = execute('TS-10', graph, lambda g: [f"{x['id']}:data schema missing" for x in g.get('elements', []) if x.get('class') == 'Entity' and not known(x.get('props', {}).get('schema'))] + [f"{x['id']}:quality measurement missing" for x in g.get('elements', []) if x.get('class') == 'QualityTarget' and not known(x.get('props', {}).get('measurement'))] + [f"{x['id']}:specialized contract {field} missing" for x in g.get('elements', []) if x.get('class') in ('Transaction', 'Saga', 'RetryPolicy', 'TimeoutBudget', 'LoadProfile', 'HighAvailabilityTopology', 'RecoveryObjective', 'LedgerInvariant', 'ReconciliationPolicy', 'AuditTrail') for field in {'Transaction': ('boundary','isolation','consistency'), 'Saga': ('steps','compensation'), 'RetryPolicy': ('max_attempts','backoff','retryable_errors'), 'TimeoutBudget': ('total','allocations'), 'LoadProfile': ('average','peak','payload','growth'), 'HighAvailabilityTopology': ('replicas','failure_domains','failover'), 'RecoveryObjective': ('rto','rpo','recovery_procedure'), 'LedgerInvariant': ('currency','scale','invariants'), 'ReconciliationPolicy': ('source','schedule','mismatch_handling'), 'AuditTrail': ('events','retention','integrity')}.get(x.get('class'), ()) if not known(x.get('props', {}).get(field))])
    from specialized_checks import evaluate_specialists
    checks['SPECIALIST'] = execute('SPECIALIST', graph, lambda g: [f"{key}:{value}" for key, record in evaluate_specialists(g).items() if record.get('outcome') != 'pass' for value in record.get('evidence', [])])
    return {key.replace('TS-', 'QG-'): {**record, 'rule_id': key.replace('TS-', 'QG-')} for key, record in checks.items()}
