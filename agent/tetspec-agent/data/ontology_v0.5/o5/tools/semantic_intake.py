"""Source anchored Markdown intake. Missing contracts stay gaps, never guessed."""
from copy import deepcopy
from hashlib import sha256
import re
from quality5 import fingerprint

ID_PATTERN = r'(?:API-[A-Z]+|NFR-[A-Z]+|FR-[A-Z]+|BRL|ENT|ADR|SVC|WRK|M|CT|INT)-\d+'

def source_digest(text):
    return sha256(text.encode()).hexdigest()

def evidence(text, source_id, version, line, quote):
    return {'kind': 'extracted', 'source_id': source_id, 'source_version': version,
            'source_hash': source_digest(text), 'block': f'line:{line}', 'quote': quote,
            'quote_hash': source_digest(quote)}

def explicit_references(text):
    result=set(re.findall(ID_PATTERN,text))
    for prefix,first,last in re.findall(r'((?:API-[A-Z]+|NFR-[A-Z]+|FR-[A-Z]+|BRL|ENT|M)-)(\d+)\.\.(\d+)',text):
        start,end=int(first),int(last)
        if end-start>10000:raise ValueError('Reference range requires explicit bounded intake')
        result.update(prefix+str(n).zfill(len(first)) for n in range(start,end+1))
    return result


def cells(line):
    # Pipe characters escaped in Markdown are content, not column separators.
    return [v.strip() for v in re.split(r'(?<!\\)\|', line.strip())[1:-1]]

def definitions(text, source_id, version):
    result, heading, header = [], '', []
    for number, line in enumerate(text.splitlines(), 1):
        if line.startswith('#'):
            heading = line.lstrip('#').strip(); header = []
        if not line.startswith('|'): continue
        values = cells(line)
        if not values or all(re.fullmatch(r'[-: ]+', v) for v in values): continue
        if not re.search(ID_PATTERN, line): header = values; continue
        # Trace tables provide links, not normative definitions.
        if re.search(r'^(?:10\.[1-4]|1\.5)', heading): continue
        found = re.match(r'^(' + ID_PATTERN + r')\b', values[0])
        if not found and len(values) > 1 and heading.startswith('7.2'):
            found = re.match(r'^(ENT-\d+)\b', values[1])
        if not found: continue
        ident = found[1]
        cls = ('BusinessRule' if ident.startswith('BRL-') else 'QualityTarget' if ident.startswith('NFR-') else
               'Operation' if ident.startswith('API-') else 'Entity' if ident.startswith('ENT-') else
               'Decision' if ident.startswith('ADR-') else 'Component' if ident.startswith(('SVC-','WRK-')) else
               'Module' if ident.startswith('M-') else 'Constraint' if ident.startswith(('CT-', 'INT-')) else 'Requirement')
        content = dict(zip(header, values)) if len(header) == len(values) else {'columns': values}
        props = {'definition': values[1:], 'source_section': heading, 'logical_id': ident,
                 'revision': version, 'source_columns': content}
        if cls == 'BusinessRule': props['predicate'] = values[1]
        if cls == 'QualityTarget':
            props['target'] = content.get('Chỉ tiêu đo được', content.get('Chỉ tiêu đo', values[1:]))
            if content.get('Phương pháp') or content.get('KC'): props['measurement'] = {'method':content.get('Phương pháp', content.get('KC')), 'target':props['target']}
        if cls == 'Requirement':
            props['statement'] = values[1]
            props['acceptance'] = values[-1] if source_id.lower() == 'srs' else None
        if cls == 'Decision' and len(values) >= 5:
            props.update(chosen=values[1], rationale=values[2], alternatives=values[3], tradeoffs=values[4])
        if cls == 'Entity' and heading.startswith('7.2'):
            props.update(table=values[2], purpose=values[3], business_key=values[4])
        if cls == 'Operation':
            match = re.search(r'\b(GET|POST|PUT|PATCH|DELETE)\s+(`?)(/[^\s`|]+)', line)
            if match: props.update(method=match[1], path=match[3])
        result.append({'id': ident, 'class': cls, 'name': values[1] if cls != 'Entity' else values[1],
                       'origin': 'upstream', 'maturity': 'sketch', 'owner_doc': source_id,
                       'version_introduced': version, 'review_status': 'needs_review', 'props': props,
                       'provenance': [evidence(text, source_id, version, number, line)]})
    return result

def build_graph(sources):
    nodes, edges, conflicts, gaps = {}, [], [], []
    by_source={s['id'].lower():s for s in sources}
    if 'srs' in by_source and 'urd' in by_source:
        expected=re.search(r'REF-A[^\n]*URD v(\d+(?:\.\d+)+)',by_source['srs']['content'])
        if expected and expected[1]!=by_source['urd']['version']:
            conflicts.append({'type':'source_version','source_id':'urd','actual':by_source['urd']['version'],'expected_by_srs':expected[1],'status':'unresolved','reason':'Supplied URD metadata does not match the SRS referenced version'})
    for source in sources:
        text = source['content']; sid = source['id']; version = source['version']
        for node in definitions(text, sid, version):
            previous = nodes.get(node['id'])
            if previous:
                # SRS refines URD; preserve both exact source statements, never silently drop evidence.
                if sid.lower() == 'srs' and previous.get('owner_doc', '').lower() == 'urd':
                    node['provenance'] += previous['provenance']; node['props']['upstream_definition'] = previous['props']
                    nodes[node['id']] = node
                elif previous['props'] != node['props']:
                    previous.setdefault('variants', []).append(node)
            else: nodes[node['id']] = node
        # Endpoint contracts omitted from input appendices must be represented as unresolved refs.
        if sid.lower() == 'srs':
            declared_catalogue = {}
            for line_number, catalogue_line in enumerate(text.splitlines(), 1):
                if not catalogue_line.startswith('Mã endpoint liên tục'): continue
                for group, first, last in re.findall(r'API-([A-Z]+)-(\d+) đến API-\1-(\d+)', catalogue_line):
                    for number in range(int(first), int(last)+1): declared_catalogue[f'API-{group}-{number:02d}'] = (line_number,catalogue_line)
            for ident in sorted(set(re.findall(r'\bAPI-[A-Z]+-\d+\b', text)) | set(declared_catalogue)):
                if ident in nodes: continue
                matching = [(i, l) for i, l in enumerate(text.splitlines(), 1) if ident in l and (l.startswith('| FR-') or l.startswith('| `pbom'))]
                if not matching and ident in declared_catalogue: matching = [declared_catalogue[ident]]
                if not matching: continue
                n, row = matching[0]
                links = [r['id'] for r in nodes.values() if r['class'] == 'Requirement' and r['id'] in row]
                label = 'SSO login' if ident == 'API-P-01' else 'SBOM submission' if ident == 'API-I-01' else ident
                nodes[ident] = {'id': ident, 'class': 'Operation', 'name': label,
                    'origin': 'upstream', 'maturity': 'sketch', 'owner_doc': sid, 'version_introduced': version,
                    'review_status': 'needs_review', 'props': {'referenced_by': links, 'contract_status': 'missing', 'gaps': ['endpoint definition from Appendix B']},
                    'provenance': [evidence(text, sid, version, n, row)]}
                gaps.append({'id': 'GAP-' + ident, 'target': ident, 'need': 'endpoint_contract', 'status': 'missing', 'searched_sources': [sid], 'reason': 'Reference found; endpoint definition absent from supplied source'})
                for rid in links: edges.append({'src': ident, 'rel': 'satisfies', 'dst': rid, 'provenance': deepcopy(nodes[ident]['provenance'])})
        # Global errors: declared catalogue only; do not attach arbitrary errors to every API.
        section, machine, state_header = '', None, False
        for number, line in enumerate(text.splitlines(), 1):
            if line.startswith('#'): section = line.lstrip('#').strip(); machine = None
            ev = [evidence(text, sid, version, number, line)]
            ent = re.match(r'\*\*(ENT-\d+) (.+?)\.\*\* Trạng thái: (.+?)\.', line)
            if ent:
                machine = 'SM-' + ent[1]; states = [v.strip() for v in ent[3].split(',')]
                nodes[machine] = {'id': machine, 'class': 'StateMachine', 'name': ent[2], 'props': {'states': states}, 'origin': 'upstream', 'maturity': 'sketch', 'review_status': 'needs_review', 'provenance': ev}
                if ent[1] in nodes: edges.append({'src': machine, 'rel': 'models_state_of', 'dst': ent[1], 'provenance': ev})
                for state in states:
                    ident = machine + ':' + state
                    nodes[ident] = {'id': ident, 'class': 'State', 'name': state, 'props': {'machine': machine}, 'origin': 'upstream', 'maturity': 'sketch', 'review_status': 'needs_review', 'provenance': ev}
                    edges.append({'src': machine, 'rel': 'has_state', 'dst': ident, 'provenance': ev})
            if not line.startswith('|'): continue
            cols = cells(line)
            if section.startswith('9.6') and len(cols) == 3 and re.fullmatch(r'[45]\d\d', cols[0]):
                code = cols[1].strip('`'); ident = 'ERR-' + code
                nodes[ident] = {'id': ident, 'class': 'ErrorCode', 'name': code, 'props': {'http_status': int(cols[0]), 'meaning': cols[2]}, 'origin': 'upstream', 'maturity': 'sketch', 'review_status': 'needs_review', 'provenance': ev}
            if machine and len(cols) >= 5 and cols[0] not in ('Từ', '---') and not re.fullmatch(r'[-: ]+', cols[0]):
                ident = f'TR-{machine}-{number}'
                nodes[ident] = {'id': ident, 'class': 'Transition', 'name': f'{cols[0]} → {cols[1]}', 'props': {'from': cols[0], 'to': cols[1], 'event': cols[2], 'guard': cols[3], 'side_effects': cols[4]}, 'origin': 'upstream', 'maturity': 'sketch', 'review_status': 'needs_review', 'provenance': ev}
                edges.append({'src':ident,'rel':'in_machine','dst':machine,'provenance':ev})
                for rel, state in [('transition_from', cols[0]), ('transition_to', cols[1])]:
                    if machine + ':' + state in nodes: edges.append({'src': ident, 'rel': rel, 'dst': machine + ':' + state, 'provenance': ev})
    # Logical attribute names in URD are available even when physical types in
    # Appendix A are missing. Preserve this distinction instead of inventing SQL.
    for entity in list(nodes.values()):
        if entity['class'] != 'Entity': continue
        definition = entity.get('props', {}).get('upstream_definition', entity.get('props', {}))
        columns = definition.get('source_columns', {})
        fields = columns.get('Thuộc tính chính')
        if not fields: continue
        attributes = []
        for field in re.findall(r'(?:^|, )([a-z][a-z0-9_\\]*(?:\\\[\\\])?)', fields):
            name = field.replace('\\', ''); ident = entity['id'] + ':attribute:' + name
            attributes.append(ident)
            nodes[ident] = {'id': ident, 'class': 'Attribute', 'name': name, 'origin': 'upstream', 'maturity': 'sketch', 'review_status': 'needs_review', 'props': {'level': 'logical', 'physical_type': None}, 'provenance': deepcopy(entity['provenance'])}
            edges.append({'src': entity['id'], 'rel': 'has_attribute', 'dst': ident, 'provenance': deepcopy(entity['provenance'])})
        entity['props']['logical_attribute_ids'] = attributes
        entity['props']['schema_status'] = 'physical schema missing from supplied Appendix A'
        gaps.append({'id':'GAP-'+entity['id']+'-SCHEMA','target':entity['id'],'need':'physical_schema','status':'missing','reason':'Only logical fields supplied; detailed Appendix A is absent'})
    # Only explicit definition-row references: relation direction and rationale remain visible.
    for node in list(nodes.values()):
        cls = node['class']
        records = node.get('provenance', []) + [ev for variant in node.get('variants', []) for ev in variant.get('provenance', [])]
        seen_refs = set()
        for record in records:
            refs = explicit_references(record.get('quote', ''))
            parts = cells(record.get('quote', ''))
            if cls == 'Module' and len(parts) > 1 and 'mọi module' in parts[1].lower():
                gap_id = 'GAP-' + node['id'] + '-SYMBOLIC-DEPENDENCY'
                if not any(gap['id'] == gap_id for gap in gaps):
                    gaps.append({'id': gap_id, 'target': node['id'], 'need': 'dependency_binding', 'status': 'missing', 'reason': parts[1], 'provenance': [deepcopy(record)]})
            for ident in sorted(refs - {node['id']}):
                if ident not in nodes or ident in seen_refs: continue
                target = nodes[ident]['class']
                parts=cells(record.get('quote',''))
                called=explicit_references(parts[1]) if len(parts)>1 else set()
                # Dependencies come from the called-module column, never a
                # co-listed subject or a negated 'does not call' statement.
                positive_call=cls=='Module' and target=='Module' and ident in called and not any(term in parts[1].lower() for term in ('không gọi','không phụ thuộc'))
                rel = 'constrained_by' if target == 'Constraint' else 'references_entity' if target == 'Entity' and cls in ('Entity','Attribute') else 'depends_on' if positive_call else 'traces_to' if target in ('BusinessRule','QualityTarget','Entity') else None
                if rel:
                    edges.append({'src': node['id'], 'rel': rel, 'dst': ident, 'provenance': [deepcopy(record)]}); seen_refs.add(ident)
    return {'source_snapshots': {s['id']:{'version':s['version'],'source_hash':source_digest(s['content']),'file_path':s.get('file_path')} for s in sources}, 'version': 'PBOM-quality-1', 'elements': sorted(nodes.values(), key=lambda x: x['id']), 'edges': edges, 'gaps': gaps, 'conflicts': conflicts}

def classify_cr(intent, graph, baseline_ids=None, explicit_new_behavior=None, comparison=None):
    by_id = {x['id']: x for x in graph.get('elements', [])}
    baseline = [by_id[i] for i in baseline_ids or [] if i in by_id]
    if comparison and (comparison.get('intent_hash') != fingerprint(intent) or comparison.get('base_hash') != fingerprint(graph)):
        return {'kind':'unknown','status':'blocked','reason':'Semantic comparison is not bound to current intent and baseline'}
    if explicit_new_behavior:
        if not comparison or comparison.get('outcome') != 'not_present' or not comparison.get('searched_ids'):
            return {'kind': 'unknown', 'status': 'blocked', 'reason': 'New behavior requires baseline search evidence', 'intent': intent}
        kind = comparison.get('kind', 'added')
        if kind not in ('added', 'modified', 'clarification', 'defect'): raise ValueError('invalid change kind')
        return {'kind': kind, 'status': 'proposed', 'statement': explicit_new_behavior, 'comparison': comparison, 'intent': intent}
    if comparison and comparison.get('outcome') == 'equivalent' and baseline and comparison.get('matched_statement') and any(comparison['matched_statement'] in ev.get('quote','') for n in baseline for ev in n.get('provenance',[])):
        return {'kind': 'duplicate', 'status': 'no_op', 'evidence': baseline, 'comparison': comparison, 'intent': intent}
    if baseline: return {'kind': 'duplicate_candidate', 'status': 'needs_review', 'evidence': baseline, 'intent': intent}
    return {'kind': 'unknown', 'status': 'blocked', 'intent': intent, 'reason': 'Baseline comparison or explicit new behavior required'}
