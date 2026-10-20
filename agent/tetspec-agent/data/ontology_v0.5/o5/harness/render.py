from __future__ import annotations
import base64, binascii, json, re, zlib
from collections import Counter
from pathlib import Path

def projection(graph, tdd_id=None):
    nodes = [x for x in graph.get('elements', []) if not tdd_id or (x.get('owner_doc') or x.get('props', {}).get('owner_doc')) == tdd_id]
    ids = {x['id'] for x in nodes}
    # A gap about something that is not an element anywhere in the KG (undefined reference, unmapped id, unread line) cannot be
    # attributed to a member, so it stays with every document rendered from this KG; ``known_ids`` is the id set of the whole KG.
    known = set(graph.get('known_ids') or ids)
    return {'gaps': sorted([g for g in graph.get('gaps',[]) if not g.get('target') or g['target'] in ids or g.get('referenced_by') in ids or g['target'] not in known],key=lambda g:json.dumps(g,sort_keys=True)), 'conflicts': graph.get('conflicts', []), 'elements': sorted(nodes, key=lambda x: x['id']), 'edges': sorted([e for e in graph.get('edges', []) if e.get('src') in ids and e.get('dst') in ids], key=lambda e: json.dumps(e, sort_keys=True))}

def render_tdd(graph, tdd_id=None):
    p = projection(graph, tdd_id)
    groups = {'Requirement': 'Requirements and acceptance', 'RequirementVersion': 'Requirements and acceptance', 'BusinessRule': 'Business rules', 'QualityTarget': 'Quality targets', 'Operation': 'Interface contracts', 'Interface': 'Interface contracts', 'Event': 'Interface contracts', 'Entity': 'Data design', 'Attribute': 'Data design', 'Migration': 'Data evolution', 'MigrationStage': 'Data evolution', 'Quantity': 'Quantities and measurements', 'Measurement': 'Quantities and measurements', 'StateMachine': 'Behavior and state', 'State': 'Behavior and state', 'Transition': 'Behavior and state', 'Algorithm': 'Behavior and state', 'Flow': 'Behavior and state', 'Condition': 'Behavior and state', 'DecisionTable': 'Behavior and state', 'Decision': 'Design decisions', 'WorkItem': 'Implementation work', 'TestCase': 'Validation', 'PermissionDecision': 'Security and authorization', 'Threat': 'Security and authorization', 'Control': 'Security and authorization', 'Credential': 'Security and authorization', 'Role': 'Security and authorization'}
    p['elements'].sort(key=lambda x: (groups.get(x.get('class'), 'Architecture and constraints'), x['id']))
    counts = {}
    for node in p['elements']:
        counts[groups.get(node.get('class'), 'Architecture and constraints')] = counts.get(groups.get(node.get('class'), 'Architecture and constraints'), 0) + 1
    lines = ['# Technical Design Document', '', f"Graph version: {graph.get('version', 'unknown')}", '',
             '## Document guide', '',
             '- **Purpose:** readable TDD projection of the canonical ontology KG.',
             '- **Traceability:** each design fact shows its source evidence; expand the canonical payload only when machine-level audit or round-trip reconstruction is needed.',
             '- **Integrity:** the renderer preserves every element, property, relationship, gap, and conflict from the selected KG projection.', '',
             '## Contents', '']
    lines.extend(f"- **{title}:** {counts[title]} element(s)" for title in sorted(counts))
    lines += [f"- **Relationships:** {len(p['edges'])}", f"- **Gaps:** {len(p['gaps'])}; **Conflicts:** {len(p['conflicts'])}", '']
    section = None
    for x in p['elements']:
        title = groups.get(x.get('class'), 'Architecture and constraints')
        if title != section:
            lines += [f'# {title}', '']; section = title
        name = x.get('name', '')
        lines += [f"## {x['id']} — {name}" if name else f"## {x['id']}"]
        lines.append(f"- **Class:** {x.get('class', 'Unknown')}")
        props = x.get('props') if isinstance(x.get('props'), dict) else {}
        # Put the fields people need to read first.  Large provenance payloads
        # stay in the collapsed canonical block below so the document remains
        # useful as a design document instead of looking like a JSON dump.
        for key in ('summary', 'purpose', 'description', 'statement', 'contract', 'owner_doc', 'status', 'maturity'):
            value = x.get(key, props.get(key))
            if value not in (None, '', [], {}):
                lines.append(f"- **{key}:** {_display_value(value)}")
        for key, value in sorted(props.items()):
            if key in {'summary', 'purpose', 'description', 'statement', 'contract', 'owner_doc', 'status', 'maturity', 'provenance', 'evidence'}:
                continue
            lines.append(f"- **{key}:** {_display_value(value)}")
        evidence = x.get('provenance') or x.get('evidence') or []
        if evidence:
            lines.append('- **Evidence:** ' + _display_evidence(evidence))
        if not props and not any(x.get(key) for key in ('summary', 'purpose', 'description', 'statement', 'contract', 'owner_doc', 'status', 'maturity')):
            lines.append('- **Design facts:** See the canonical KG payload.')
        lines += ['<details>', '<summary>Canonical KG payload (round-trip source)</summary>', '',
                  '- Canonical node (zlib): ' + _canonical_payload(x),
                  ]
        # Keep the legacy scalar anchors inside the collapsed block.  They
        # make edits to a rendered document detectable by older tooling while
        # the canonical node remains the lossless source of truth.
        def values(value, path):
            if isinstance(value, dict) and value:
                for key, item in sorted(value.items()):
                    yield from values(item, path + [key])
            else:
                yield path, value
        # Only props retain the legacy field anchors.  Identity, provenance,
        # and lifecycle metadata are already losslessly carried by the
        # compressed canonical payload and do not need another full dump.
        if x.get('props'):
            for field, item in values(x.get('props', {}), ['props']):
                lines.append(f"- Value {json.dumps(field, ensure_ascii=False)} = {json.dumps(item, ensure_ascii=False, sort_keys=True)}")
        lines += ['', '</details>', '']
    lines += ['', '## Relationships']
    by_rel = {}
    for edge in p['edges']:
        by_rel.setdefault(edge.get('rel', 'related_to'), []).append(edge)
    for rel, edges in sorted(by_rel.items()):
        lines += [f'### {rel}', '']
        for edge in edges:
            lines.append(f"- `{edge.get('src')}` → `{edge.get('dst')}`")
        lines += ['<details>', '<summary>Canonical relationship payload</summary>', '']
        for edge in edges:
            lines.append('- Relation: ' + json.dumps(edge, ensure_ascii=False, sort_keys=True))
        lines += ['', '</details>', '']
    lines += ['', '## Gaps and conflicts']
    if not p['gaps'] and not p['conflicts']:
        lines.append('- No recorded gaps or conflicts.')
    else:
        for gap in p['gaps']:
            lines.append(f"- **Gap:** {_display_value(gap)}")
        for conflict in p['conflicts']:
            lines.append(f"- **Conflict:** {_display_value(conflict)}")
        lines += ['<details>', '<summary>Canonical gap and conflict payload</summary>', '']
        for gap in p['gaps']: lines.append('- Gap: '+json.dumps(gap,ensure_ascii=False,sort_keys=True))
        for conflict in p['conflicts']: lines.append('- Conflict: '+json.dumps(conflict,ensure_ascii=False,sort_keys=True))
        lines += ['', '</details>']
    return '\n'.join(lines) + '\n'


def _display_value(value, limit=600):
    """Return a compact, reader-facing representation of a KG value."""
    if isinstance(value, str):
        text = ' '.join(value.split())
    else:
        text = json.dumps(value, ensure_ascii=False, sort_keys=True)
    return text if len(text) <= limit else text[:limit - 1].rstrip() + '…'


def _canonical_payload(value):
    raw = json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(',', ':')).encode('utf-8')
    return base64.b64encode(zlib.compress(raw, 9)).decode('ascii')


def _decode_canonical_payload(value):
    return json.loads(zlib.decompress(base64.b64decode(value)).decode('utf-8'))


def _display_evidence(evidence):
    snippets = []
    for item in evidence[:3] if isinstance(evidence, list) else []:
        if not isinstance(item, dict):
            snippets.append(_display_value(item, 180)); continue
        source = item.get('source_id') or item.get('document_id') or item.get('kind') or 'source'
        version = item.get('source_version') or item.get('version')
        quote = item.get('quote') or item.get('text') or item.get('anchor')
        label = f"{source}@{version}" if version else str(source)
        snippets.append(f"`{label}`" + (f" — {_display_value(quote, 220)}" if quote else ''))
    return '; '.join(snippets) or 'recorded in canonical payload'

def semantic_ids(markdown):
    ids = {m.group(2) for m in re.finditer(r'^- \[([^\]]+)\] (.+?):', markdown, re.M)}
    ids.update(json.loads(m.group(1))['id'] for m in re.finditer(r'^- Canonical node: (\{.*\})$', markdown, re.M))
    ids.update(_decode_canonical_payload(m.group(1))['id'] for m in re.finditer(r'^- Canonical node \(zlib\): (.+)$', markdown, re.M))
    return ids

def parse_projection(markdown):
    nodes, edges, current = [], [], None
    gaps, conflicts = [], []
    ids, paths = set(), set()
    for line in markdown.splitlines():
        canonical_node = re.match(r'^- Canonical node \(zlib\): (.+)$', line)
        if canonical_node:
            node = _decode_canonical_payload(canonical_node.group(1))
        else:
            legacy_node = re.match(r'^- Canonical node: (\{.*\})$', line)
            node = json.loads(legacy_node.group(1)) if legacy_node else None
        if node is not None:
            if not isinstance(node, dict) or not node.get('id') or not node.get('class'):
                raise ValueError('invalid canonical element')
            if node['id'] in ids: raise ValueError('duplicate element anchor: '+node['id'])
            ids.add(node['id']); nodes.append(node); current = node; paths = set()
            continue
        if line.startswith('- Canonical relationship: '):
            edges.append(json.loads(line[len('- Canonical relationship: '):]))
            continue
        if line.startswith('- Canonical gap: '):
            gaps.append(json.loads(line[len('- Canonical gap: '):]))
            continue
        if line.startswith('- Canonical conflict: '):
            conflicts.append(json.loads(line[len('- Canonical conflict: '):]))
            continue
        match = re.match(r'^- \[([^\]]+)\] (.+?): (.*)$', line)
        if match:
            if match[2] in ids: raise ValueError('duplicate element anchor: '+match[2])
            ids.add(match[2]); paths=set()
            current = {'class': match[1], 'id': match[2], 'name': match[3]}; nodes.append(current)
            continue
        elif line.startswith('- Value '):
            if not current: raise ValueError('value without element')
            fields, offset = json.JSONDecoder().raw_decode(line[len('- Value '):])
            tail = line[len('- Value ')+offset:]
            if not fields or not tail.startswith(' = '): raise ValueError('invalid property anchor')
            key=tuple(fields)
            if key in paths: raise ValueError('duplicate property anchor')
            paths.add(key)
            container = current
            for field in fields[:-1]: container = container.setdefault(field, {})
            container[fields[-1]] = json.loads(tail[3:])
        elif line.startswith('- Fact `'):
            match = re.match(r'^- Fact `([^`]+)`: (.*)$', line)
            if not current or not match: raise ValueError('invalid fact anchor')
            current[match[1]] = json.loads(match[2])
        elif line.startswith('- Relation: '): edges.append(json.loads(line[len('- Relation: '):]))
        elif line.startswith('- Gap: '): gaps.append(json.loads(line[len('- Gap: '):]))
        elif line.startswith('- Conflict: '): conflicts.append(json.loads(line[len('- Conflict: '):]))
        elif line.startswith('> Readiness blockers: '): pass
        elif line.startswith(('- **', '- `', '- No recorded gaps or conflicts.')) or line.startswith(('<details', '</details>', '<summary>', '</summary>')) or not line.strip(): pass
        elif line and not line.startswith(('#', 'Graph version:')):
            raise ValueError('unmapped prose: ' + line)
    return projection({'elements': nodes, 'edges': edges, 'gaps': gaps, 'conflicts': conflicts})

def round_trip(graph, markdown, tdd_id=None):
    expected = projection(graph, tdd_id)
    try:
        actual = parse_projection(markdown); errors = []
        expected_headers=Counter(l for l in render_tdd(graph,tdd_id).splitlines() if l.startswith('#'))
        actual_headers=Counter(l for l in markdown.splitlines() if l.startswith('#') and l!='# DRAFT')
        if actual_headers!=expected_headers:errors.append('HEADINGS_CHANGED_OR_UNMAPPED_ASSERTION')
        versions=[l for l in markdown.splitlines() if l.startswith('Graph version:')]
        if versions!=[f"Graph version: {graph.get('version','unknown')}"]:errors.append('RENDER_SNAPSHOT_MISMATCH')
        expected_nodes = {x['id']: x for x in expected['elements']}; actual_nodes = {x['id']: x for x in actual['elements']}
        for ident in sorted(set(expected_nodes) | set(actual_nodes)):
            left, right = expected_nodes.get(ident), actual_nodes.get(ident)
            if left is None or right is None: errors.append(f'{ident}:element missing/extra'); continue
            for key in sorted(set(left) | set(right)):
                if left.get(key) != right.get(key): errors.append(f'{ident}:{key}:changed/missing/invented')
        if expected['edges'] != actual['edges']: errors.append('relationships:changed/missing/invented')
        for key in ('gaps','conflicts'):
            if expected[key]!=actual[key]:errors.append(key+':changed/missing/invented')
    except (ValueError, TypeError, zlib.error, binascii.Error) as exc: actual = {'elements': []}; errors = [str(exc)]
    expected_ids = {x['id'] for x in expected['elements']}; actual_ids = {x['id'] for x in actual['elements']}
    return {'ok': not errors, 'errors': errors, 'missing': sorted(expected_ids-actual_ids), 'extra': sorted(actual_ids-expected_ids), 'expected': len(expected_ids), 'actual': len(actual_ids)}

def publish_tdd(graph, output, readiness, tdd_id=None, allow_draft=True):
    from quality5 import fingerprint
    from ontology5 import load
    coverage = readiness.get('rule_coverage', {})
    ready = readiness.get('status') == 'implementation_ready' and not readiness.get('blockers') and set(load()['profiles']['tdd-generation']['rules']) <= set(coverage) and all(isinstance(v, dict) and v.get('outcome') == 'pass' and v.get('input_hash') == fingerprint(graph) for v in coverage.values())
    if not ready and not allow_draft: return {'status': 'blocked', 'blockers': readiness.get('blockers', [])}
    content = render_tdd(graph, tdd_id)
    if not ready: content = '# DRAFT\n> Readiness blockers: '+json.dumps(readiness.get('blockers', []))+'\n'+ content
    check = round_trip(graph, content, tdd_id)
    if not check['ok']: return {'status': 'blocked', 'round_trip': check}
    Path(output).write_text(content, encoding='utf-8')
    return {'status': 'published' if ready else 'draft', 'round_trip': check, 'blockers': readiness.get('blockers', [])}
