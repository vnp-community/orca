from __future__ import annotations
from dataclasses import dataclass, asdict
from hashlib import sha256
from copy import deepcopy
import json, os, re, shutil, tempfile, subprocess, sys
from pathlib import Path
TOOLS = Path(__file__).resolve().parents[1] / 'tools'
HARNESS = Path(__file__).resolve().parents[1] / 'harness'
O5_ROOT = Path(__file__).resolve().parents[1]
for p in (TOOLS, HARNESS, O5_ROOT):
    if str(p) not in sys.path: sys.path.insert(0, str(p))
from quality5 import fingerprint, typed_closure, graph_checks, execute, source_snapshot_errors
from semantic_resolver import resolve_scope
from readiness5 import assess
from workflow import validate_workflow_context, build_change_spec, WorkflowError, ImpactReport, ImpactEntry


_WORK_STOPWORDS = {
    'a', 'an', 'and', 'are', 'as', 'at', 'be', 'by', 'for', 'from', 'in',
    'into', 'is', 'it', 'of', 'on', 'or', 'that', 'the', 'this', 'to', 'under',
    'with', 'without', 'all', 'each', 'every', 'must', 'shall', 'system',
}
_WORK_SYNONYMS = {
    'deduplication': 'duplicate', 'duplicates': 'duplicate', 'duplicate': 'duplicate',
    'reprocessing': 'repeat', 'repeated': 'repeat', 'retry': 'repeat', 'retries': 'repeat',
    'reminders': 'reminder', 'milestones': 'milestone', 'policies': 'policy',
    'tests': 'test', 'testing': 'test', 'records': 'record', 'markers': 'marker',
}


def _work_acceptance_tokens(value: str) -> set[str]:
    tokens = set(re.findall(r"[0-9]+|[^\W_]+", str(value).lower(), re.UNICODE))
    return {_WORK_SYNONYMS.get(token.rstrip('s'), _WORK_SYNONYMS.get(token, token)) for token in tokens if token not in _WORK_STOPWORDS}


def _work_acceptance_covered(value: str, tasks: str) -> bool:
    """Accept faithful paraphrase while retaining a meaningful coverage gate."""
    if not value or value in tasks:
        return True
    expected = _work_acceptance_tokens(value)
    actual = _work_acceptance_tokens(tasks)
    if len(expected) < 3:
        return False
    return len(expected & actual) / len(expected) >= 0.5


def digest(path):
    path = Path(path); h = sha256()
    if path.is_file(): h.update(path.read_bytes())
    elif path.is_dir():
        for child in sorted(x for x in path.rglob('*') if x.is_file()):
            h.update(str(child.relative_to(path)).encode()); h.update(child.read_bytes())
    return h.hexdigest()


def repo_context(root, code_paths=None):
    root = Path(root).resolve(); spec = root/'openspec'; contents = {}
    for folder in ('specs', 'changes'):
        for path in sorted((spec/folder).rglob('*')):
            if path.is_file() and path.suffix in ('.md', '.yaml'): contents[str(path.relative_to(root))] = path.read_text()
    for rel in ['openspec/config.yaml', 'AGENTS.md', 'README.md', *(code_paths or [])]:
        path = (root/rel).resolve()
        if not path.is_relative_to(root): raise ValueError('context path escapes repository')
        if path.is_file(): contents[rel] = path.read_text()
    existing = sorted(p.name for p in (spec/'changes').iterdir() if p.is_dir() and not p.name.startswith('.')) if (spec/'changes').exists() else []
    return {'repo': str(root), 'config_hash': digest(spec/'config.yaml') if (spec/'config.yaml').exists() else None,
            'specs_hash': digest(spec/'specs'), 'changes_hash': digest(spec/'changes'), 'context_hash': fingerprint(contents),
            'code_paths': code_paths or [], 'context_roles':{path:('baseline_spec' if path.startswith('openspec/specs/') else 'active_proposal' if path.startswith('openspec/changes/') else 'configuration' if path.endswith('config.yaml') else 'source_or_code_evidence') for path in contents}, 'contents': contents, 'existing_changes': existing,
            'baseline': sorted(str(p.relative_to(root)) for p in (spec/'specs').rglob('*.md'))}


@dataclass
class SpecChangeRequest:
    id: str
    change_name: str
    intent: str
    scope: list[str]
    kg_snapshot: str
    repo_root: str
    profile_id: str = 'spec-change-generation'
    allow_draft: bool = True
    code_paths: list[str] | None = None
    context_budget: int | None = None
    baseline_kind: str = 'existing'
    # The settled workflow is opt-in for backward-compatible draft fixtures.
    # Production callers must provide this context; KG-only input remains a
    # legacy diagnostic path and cannot become implementation_ready.
    workflow_context: dict | None = None
    run_root: str | None = None  # where the working copies of one publication go (default: <repo_root>/.tdd-runs); a caller that owns a work folder keeps them out of the repository

    def __post_init__(self):
        if not re.fullmatch(r'[a-z0-9]+(?:-[a-z0-9]+)*', self.change_name): raise ValueError('invalid change name')
        if self.baseline_kind not in ('existing', 'greenfield'): raise ValueError('invalid baseline kind')


def split_conflicts(conflicts, ids):
    """Conflicts the change must settle (about an element in its closure, or about nothing in particular) and the ones elsewhere in the graph.

    A conflict somewhere else in the project is reported but does not stop this change; a resolved one blocks nothing."""
    inside, outside = [], []
    for c in conflicts:
        if c.get('status') == 'resolved': continue
        named = set(c.get('ids') or []) | ({c['id']} if c.get('id') else set())
        (inside if not named or named & ids else outside).append(deepcopy(c))
    return inside, outside


def change_zone(by_id, edges, seeds):
    """The elements this change answers for: what it names, their direct neighbours, and the scenarios, work, tests and records that hang on them.

    The closure given to the author can be most of a dense graph; a completeness rule (work and test for a requirement, a schema for an entity, an
    open gap) is about the change's own elements, so it is applied to the zone and not to every element the closure happens to reach."""
    zone = {i for i in seeds if i in by_id}
    for e in edges:
        if e['src'] in zone or e['dst'] in zone:
            zone.update(i for i in (e['src'], e['dst']) if i in by_id)
    for node in by_id.values():
        p = node.get('props', {})
        if p.get('requirement_id') in zone or p.get('logical_requirement_id') in zone or p.get('capability_id') in zone:
            zone.add(node['id'])
    return zone


def build_pack(request, graph, repo):
    by_id = {x['id']: x for x in graph.get('elements', [])}
    deltas = [x for x in by_id.values() if x.get('class') == 'RequirementDelta' and (not request.scope or x['id'] in request.scope or x.get('props', {}).get('capability_id') in request.scope)]
    seeds = set(request.scope)
    for delta in deltas:
        for field in ('before_requirement_id', 'after_requirement_id', 'requirement_id'):
            if delta.get('props', {}).get(field): seeds.add(delta['props'][field])
    closure = resolve_scope(graph, sorted(seeds), budget=request.context_budget)
    ids = set(closure['ids'])
    ids.update(d.get('props', {}).get('capability_id') for d in deltas if d.get('props', {}).get('capability_id') in by_id)
    # Scenario parent references are supported even when graph export omitted inverse edge.
    for node in by_id.values():
        if node.get('class') == 'Scenario' and node.get('props', {}).get('requirement_id') in ids: ids.add(node['id'])
    selected = [deepcopy(by_id[i]) for i in sorted(ids) if i in by_id]
    zone = change_zone(by_id, [e for e in graph.get('edges', []) if e['src'] in ids and e['dst'] in ids], set(request.scope) | seeds | {d['id'] for d in deltas}) if deltas else ids
    zone_edges = [deepcopy(e) for e in graph.get('edges', []) if e['src'] in zone and e['dst'] in zone]
    scoped_conflicts, outside_conflicts = split_conflicts(graph.get('conflicts', []), zone)
    semantic = {'source_snapshots':deepcopy(graph.get('source_snapshots', {})), 'version': graph.get('version'), 'elements': selected, 'edges': [deepcopy(e) for e in graph.get('edges', []) if e['src'] in ids and e['dst'] in ids], 'gaps': [g for g in graph.get('gaps', []) if g.get('target') in ids], 'conflicts': scoped_conflicts}
    semantic_zone = {**semantic, 'elements': [x for x in selected if x['id'] in zone], 'edges': zone_edges, 'gaps': [g for g in graph.get('gaps', []) if g.get('target') in zone]}
    excluded = {'Requirement', 'RequirementVersion', 'RequirementDelta', 'Capability', 'Scenario', 'ChangeRequest', 'WorkItem', 'TestCase'}
    from profiles import default_workflow_profile
    return {'request': asdict(request), 'repo': repo, 'specialist_profile': default_workflow_profile('spec-change', context={'scope': request.scope}), 'baseline': {'kind': request.baseline_kind, 'snapshot': request.kg_snapshot},
            'kg_hash': fingerprint(graph), 'deltas': deepcopy(deltas), 'versions': [x for x in selected if x.get('class') in ('RequirementVersion', 'Requirement')],
            'scenarios': [x for x in selected if x.get('class') == 'Scenario'], 'design': [x for x in selected if x.get('class') not in excluded],
            'work': [x for x in selected if x.get('class') == 'WorkItem'], 'tests': [x for x in selected if x.get('class') == 'TestCase'],
            'graph': semantic, 'zone_graph': semantic_zone, 'impact': {'changed': sorted(request.scope), 'affected': sorted(ids - set(request.scope)), 'truncated': closure['truncated'], 'missing_ids': closure['missing_ids'], 'conflicts_outside_scope': outside_conflicts, 'resolution_hash': closure.get('resolution_hash'), 'unresolved': closure.get('unresolved', []), 'boundary': closure.get('boundary', [])},
            'provenance': {x['id']: x.get('provenance') or x.get('evidence') or [] for x in selected},
            'workflow_context': request.workflow_context}


def canonical_stage5_spec(workflow_context):
    """Return the canonical CS table for an approved workflow context.

    The public generator accepts JSON dictionaries as well as the typed
    ``ImpactReport`` object, making this boundary usable by a CLI or a KG
    adapter without allowing the KG to silently become the source of intent.
    """
    errors = validate_workflow_context(workflow_context or {})
    if errors:
        return {'status': 'blocked', 'errors': errors, 'items': [], 'pending': []}
    impact = workflow_context['impact']
    if isinstance(impact, dict):
        entries = [ImpactEntry(**entry) for entry in impact.get('entries', [])]
        impact = ImpactReport(id=impact['id'], cr_ref=impact['cr_ref'], srs_ref=impact['srs_ref'], sad_ref=impact['sad_ref'], baseline_tdd_snapshot=impact['baseline_tdd_snapshot'], entries=entries, findings=impact.get('findings', []), status=impact.get('status', 'pending_approval'), approval=impact.get('approval'))
    try:
        return build_change_spec(workflow_context['srs_delta'], workflow_context['tdd_set'], workflow_context['sad'], impact, workflow_context.get('cr'))
    except WorkflowError as exc:
        return {'status': 'blocked', 'errors': [str(exc)], 'items': [], 'pending': []}


def _ref_text(ref):
    if isinstance(ref, str):
        return ref
    if not isinstance(ref, dict):
        return str(ref)
    return ref.get('section_id') or ref.get('section') or ref.get('document_id') or ref.get('id') or json.dumps(ref, ensure_ascii=False, sort_keys=True)


def _refs_text(refs):
    return ', '.join(_ref_text(ref) for ref in refs) or 'unresolved'


def _md_cell(value):
    return str(value).replace('|', '\\|').replace('\n', ' ').strip()


def _display_scalar(value, limit=280):
    text = str(value).replace('\n', ' ').strip()
    return text if len(text) <= limit else text[:limit - 1].rstrip() + '…'


def _kg_facts_for_item(item, kg_context):
    """Select design facts from the semantic KG without inventing new facts."""
    if not kg_context:
        return []
    graph = kg_context.get('graph') if isinstance(kg_context, dict) and isinstance(kg_context.get('graph'), dict) else kg_context
    if not isinstance(graph, dict):
        return []
    nodes = graph.get('elements', [])
    refs = set()
    for ref in (item.get('srs_refs', []) + item.get('tdd_refs', [])):
        if isinstance(ref, dict):
            refs.update(str(ref.get(k)) for k in ('id', 'node_id', 'element_id', 'requirement_id') if ref.get(k))
        elif ref:
            refs.add(str(ref))
    target = str(item.get('code_target', '')).lower()
    tokens = {t for t in re.findall(r'[a-z0-9]+', target) if len(t) > 2}
    selected = []
    for node in nodes:
        if not isinstance(node, dict):
            continue
        blob = json.dumps(node, ensure_ascii=False, sort_keys=True).lower()
        if node.get('id') in refs or (tokens and any(t in blob for t in tokens)):
            selected.append(node)
    return sorted(selected, key=lambda node: str(node.get('id', '')))[:8]


def _kg_fact_line(node):
    props = node.get('props') if isinstance(node.get('props'), dict) else {}
    useful = []
    for key in ('statement', 'definition', 'contract', 'when', 'then', 'given', 'technology', 'timeout', 'retry', 'owner_doc'):
        if node.get(key) not in (None, '', [], {}): useful.append(f"{key}={node[key]}")
        elif props.get(key) not in (None, '', [], {}): useful.append(f"{key}={props[key]}")
    detail = '; '.join(_display_scalar(value) for value in useful)
    return f"`{node.get('id', 'unknown')}` [{node.get('class', 'Unknown')}] {node.get('name', '')}" + (f" — {detail}" if detail else '')


def project_change_spec_to_openspec(canonical: dict, change_name: str, intent: str, kg_context: dict | None = None) -> dict:
    """Project the approved SRS/TDD/SAD table into a detailed OpenSpec draft.

    Every implementation statement is anchored to a canonical CS row. When
    ``kg_context`` is supplied, the design section also quotes matching KG
    facts; it never treats an unreferenced KG node as permission to invent work.
    """
    if canonical.get('status') != 'eligible':
        raise WorkflowError('CHANGE_SPEC_NOT_ELIGIBLE')
    items = canonical.get('items', [])
    grouped = {"add": [], "modify": [], "remove": []}
    for item in items:
        grouped.setdefault(item.get('action', 'modify'), []).append(item)

    proposal = ['## Why', '', intent, '',
                '## What Changes', '',
                'This change implements the approved SRS delta using the TDD sections selected by the impact assessment. Each row below is a required implementation boundary.', '']
    for item in items:
        srs = _refs_text(item.get('srs_refs', [])); tdd = _refs_text(item.get('tdd_refs', [])); cr = ', '.join(item.get('cr_acceptance_refs', [])) or 'unresolved'
        proposal.append(f"- **{item['id']}**: `{item['action']}` `{item['code_target']}`; SRS `{srs}`; TDD `{tdd}`; CR acceptance `{cr}`; verification: {item['verification']}")
    proposal += ['', '## Capabilities', '', f'- **Modified capability:** `{change_name}`', '- The capability must preserve the referenced TDD constraints and expose the verification evidence listed in the canonical table.', '', '## Impact', '', '- **Code:** the targets listed in the canonical rows.', '- **Contracts and tests:** the SRS references, TDD references, and CR acceptance criteria listed for each row.', '- **Data or operations:** only where the corresponding canonical row names that target.', '', '## Source trace', '', 'The canonical CS table is the source of truth for this projection.']
    for item in items:
        proposal.append(f"- `{item['id']}` ← SRS `{_refs_text(item.get('srs_refs', []))}` ← TDD `{_refs_text(item.get('tdd_refs', []))}` ← CR `{', '.join(item.get('cr_acceptance_refs', [])) or 'unresolved'}`")

    design = ['## Context', '', intent, '',
              '## Goals', '', '- Implement every eligible CS row with its SRS and TDD trace intact.', '- Preserve the SAD constraints while applying the approved impact result.', '',
              '## Non-goals', '', '- Do not change architecture or SAD constraints outside the canonical rows.', '- Do not infer requirements from KG nodes that are not referenced by a CS row.', '',
              '## Decisions', '']
    for item in items:
        action = item.get('action', 'modify')
        design += [f"### {item['id']} — `{item['code_target']}`", '', f"- **Decision:** {action} the target exactly as required by SRS `{_refs_text(item.get('srs_refs', []))}`.", f"- **TDD constraint:** `{_refs_text(item.get('tdd_refs', []))}`.", f"- **Acceptance evidence:** {item.get('verification', '')} (CR `{', '.join(item.get('cr_acceptance_refs', [])) or 'unresolved'}`)."]
        facts = _kg_facts_for_item(item, kg_context)
        if facts:
            design += ['- **KG facts used:**'] + [f'  - {_kg_fact_line(node)}' for node in facts]
        design.append('')
    design += ['## Canonical implementation rows', '', '| CS | Action | Code target | SRS | TDD sections | CR criteria | Verification |', '|---|---|---|---|---|---|---|']
    for item in items:
        design.append('| ' + ' | '.join(_md_cell(x) for x in (item['id'], item['action'], f"`{item['code_target']}`", _refs_text(item.get('srs_refs', [])), _refs_text(item.get('tdd_refs', [])), ', '.join(item.get('cr_acceptance_refs', [])) or 'unresolved', item['verification'])) + ' |')
    design += ['', '## Risks and mitigations', '', '- A missing or conflicting SRS/TDD reference blocks the row and must return to impact assessment.', '- Verification evidence is incomplete until the CR acceptance test is attached to the implementation.', '', '## Migration and rollout', '', '- Use the migration, compatibility, timeout, retry, and rollback constraints in the cited TDD sections.', '- Record test evidence against the same CS identifiers before implementation is accepted.']

    tasks = ['## 1. Preparation', '', '- [ ] Confirm the SRS delta, SAD constraint, and approved impact report are the snapshots named in `trace-manifest.json`.', '- [ ] Confirm every CS row has a resolvable TDD reference before editing code.', '', '## 2. Implementation', '']
    for index, item in enumerate(items, 1):
        srs = _refs_text(item.get('srs_refs', [])); tdd = _refs_text(item.get('tdd_refs', [])); cr = ', '.join(item.get('cr_acceptance_refs', [])) or 'unresolved'
        verification = str(item.get('verification', '')).rstrip('.')
        tasks += [f"### 2.{index} `{item['id']}` — {item['action']} `{item['code_target']}`", '', f"- [ ] Implement the target according to SRS `{srs}` and TDD `{tdd}`.", f"- [ ] Add or update tests proving: {verification}.", f"- [ ] Attach CR acceptance evidence `{cr}` to the CS row.", '- [ ] Record the resulting code paths, schema/policy changes, and rollback evidence.', '']
    tasks += ['## 3. Verification', '', '- [ ] Run the tests named by each CS row and check the expected behavior against the cited TDD scenario.', '- [ ] Re-run trace validation; no task is complete if its SRS, TDD, or CR reference is missing.']

    spec = []
    for action, title in (("add", "ADDED"), ("modify", "MODIFIED"), ("remove", "REMOVED")):
        if grouped.get(action): spec.extend([f"## {title} Requirements", ""])
        for item in grouped.get(action, []):
            srs = _refs_text(item.get('srs_refs', [])); tdd = _refs_text(item.get('tdd_refs', [])); cr = ', '.join(item.get('cr_acceptance_refs', [])) or 'unresolved'
            verb = {"add": "add", "modify": "change", "remove": "remove"}.get(action, action)
            spec.extend([f"### Requirement: {item['id']} — {item['code_target']}", '', f"The implementation SHALL {verb} `{item['code_target']}` according to SRS section(s) `{srs}` and TDD section(s) `{tdd}`. The implementation SHALL provide evidence for CR acceptance criterion(s) `{cr}`.", '', f"#### Scenario: Verify {item['id']}", '', f"- **WHEN** the implementation is exercised using the cited TDD scenario for CR `{cr}`", f"- **THEN** {item['verification']}", '', f"#### Scenario: Preserve traceability for {item['id']}", '', f"- **WHEN** the change is reviewed", f"- **THEN** the implementation, test, SRS `{srs}`, TDD `{tdd}`, and CR `{cr}` are linked by the CS identifier `{item['id']}`.", ''])

    trace = {"change_name": change_name, "intent": intent, "canonical_status": canonical.get('status'), "items": []}
    for index, item in enumerate(items, 1):
        trace['items'].append({"cs_id": item["id"], "action": item.get('action'), "code_target": item.get('code_target'), "task": f"tasks.md:2.{index}", "spec": f"specs/{change_name}/spec.md:Requirement: {item['id']}", "srs_refs": item.get("srs_refs", []), "tdd_refs": item.get("tdd_refs", []), "cr_acceptance_refs": item.get("cr_acceptance_refs", []), "verification": item.get('verification'), "kg_fact_ids": [n.get('id') for n in _kg_facts_for_item(item, kg_context)]})
    artifacts = {'proposal.md': '\n'.join(proposal) + '\n', 'design.md': '\n'.join(design) + '\n', 'tasks.md': '\n'.join(tasks) + '\n', f'specs/{change_name}/spec.md': '\n'.join(spec) + '\n'}
    artifacts['trace-manifest.json'] = json.dumps(trace, indent=2, ensure_ascii=False) + '\n'
    return artifacts

def readiness(pack, graph, execution=None, artifacts=None, validation=None, cli=None, review=None):
    required = {'intent': pack['request']['intent'], 'baseline': pack['baseline'], 'delta': pack['deltas'], 'scenarios': pack['scenarios'], 'design': pack['design'], 'impact': pack['impact'], 'work': pack['work'], 'validation': pack['tests'], 'repo_context': pack['repo'], 'provenance': pack['provenance'], 'rule_coverage': graph_checks(pack.get('zone_graph') or pack['graph'])}
    from techspec_rules import evaluate
    required['rule_coverage'].update(evaluate(pack, required['rule_coverage'], artifacts, validation, cli, review))
    if pack.get('workflow_context'):
        from workflow_quality import evaluate_workflow
        required['workflow_evaluation'] = evaluate_workflow({'context': pack['workflow_context'], 'change_spec': pack.get('canonical_change_spec', {})}, 'spec')
    result = assess({'profile_id': 'spec-change-generation'}, required)
    if pack['impact']['truncated']: result['blockers'].append('MANDATORY_CONTEXT_TRUNCATED')
    blocking_unresolved = [x for x in pack['impact'].get('unresolved', []) if x.get('type') not in {'unsupported_relation'}]
    if blocking_unresolved:
        result['blockers'].append('SEMANTIC_RESOLUTION_HAS_UNRESOLVED_BOUNDARIES')
    if pack.get('workflow_context'):
        workflow_errors = validate_workflow_context(pack['workflow_context'])
        if workflow_errors:
            result['blockers'].extend(workflow_errors)
        elif not pack.get('canonical_change_spec', {}).get('status') == 'eligible':
            result['blockers'].append('CANONICAL_CHANGE_SPEC_REQUIRED')
    if not execution or execution.get('status') != 'authored' or not execution.get('input_hash') or not execution.get('output_hash'):
        result['blockers'].append('AUTHORING_EXECUTION_REQUIRED')
    result['blockers'] = sorted(set(result['blockers'])); result['status'] = 'draft' if result['blockers'] else 'implementation_ready'
    return result


def _scenarios_for(version, scenarios):
    p = version.get('props', {}); ids = set(p.get('scenario_ids', []))
    return [x for x in scenarios if x['id'] in ids or x.get('props', {}).get('requirement_id') in (version['id'], p.get('logical_requirement_id'))]


def render_artifacts(pack, readiness_report):
    """Diagnostic fallback only. Authoring ready output requires an agent runner."""
    artifacts = {'proposal.md': '## Why\n\n' + pack['request']['intent'] + '\n\n> DRAFT: authoring workflow not executed.\n', 'design.md': '## Context\n\n> DRAFT: design authoring required.\n', 'tasks.md': '## 1. Resolve blockers\n\n- [ ] 1.1 Resolve scoped design/evidence gaps before authoring.\n'}
    versions = {x['id']: x for x in pack['versions']}
    for delta in pack['deltas']:
        p = delta.get('props', {}); cap = p.get('capability_id', 'change'); kind = p.get('kind', 'added'); name = p.get('name') or delta.get('name', delta['id'])
        path = f'specs/{cap}/spec.md'
        if kind == 'removed': text = f"## REMOVED Requirements\n\n### Requirement: {name}\n\n**Reason**: {p.get('rationale', '')}\n**Migration**: {p.get('migration', '')}\n"
        elif kind == 'renamed': text = f"## RENAMED Requirements\n\nFROM: {p.get('old_name', '')}\nTO: {p.get('new_name', '')}\n"
        else:
            node = versions.get(p.get('after_requirement_id'), delta); vp = node.get('props', {})
            text = f"## {kind.upper()} Requirements\n\n### Requirement: {name}\n\n{vp.get('statement', node.get('name', ''))}\n"
            for sc in _scenarios_for(node, pack['scenarios']):
                sp = sc.get('props', {}); text += f"\n#### Scenario: {sc.get('name', sc['id'])}\n\n- **GIVEN** {sp.get('given', '')}\n- **WHEN** {sp.get('when', '')}\n- **THEN** {sp.get('then', '')}\n"
        artifacts[path] = artifacts.get(path, '') + text
    return artifacts


def validate_artifacts(artifacts, pack, report, assertions=None, decisions=None):
    errors = []
    for path in artifacts:
        if Path(path).is_absolute() or '..' in Path(path).parts or '\\' in path: errors.append('UNSAFE_PATH:' + path)
    for name in ('proposal.md', 'design.md', 'tasks.md'):
        if not artifacts.get(name, '').strip(): errors.append('MISSING:' + name)
    if assertions:
        errors.extend(validate_structured_assertions(assertions, artifacts))
    specs = [p for p in artifacts if re.fullmatch(r'specs/[^/]+/spec.md', p)]
    if not specs: errors.append('MISSING:spec')
    for path in specs:
        blocks = re.split(r'(?m)^### Requirement: ', artifacts[path])[1:]
        for block in blocks:
            if '## REMOVED Requirements' in artifacts[path] and '**Reason**' in block: continue
            if '#### Scenario:' not in block: errors.append('REQUIREMENT_WITHOUT_SCENARIO:' + path + ':' + block.splitlines()[0])
            for scenario in re.split(r'(?m)^#### Scenario: ', block)[1:]:
                if not re.search(r'\*\*WHEN\*\*\s+\S', scenario) or not re.search(r'\*\*THEN\*\*\s+\S', scenario): errors.append('EMPTY_SCENARIO:' + path)
    if assertions is not None:
        by_id = {x['id']: x for x in pack['graph']['elements']}; proposed = {x['id']: x for x in decisions or []}; anchors = set()
        for assertion in assertions:
            artifact = assertion.get('artifact'); anchor = assertion.get('anchor', ''); references = assertion.get('references', [])
            try:
                line = int(anchor.removeprefix('line:')); content = artifacts[artifact].splitlines()[line-1]
                if line < 1 or not content.strip(): raise ValueError('empty anchor')
            except (KeyError, ValueError, IndexError): errors.append('INVALID_ANCHOR:' + str(artifact) + ':' + anchor); continue
            anchors.add((artifact, line))
            if not references: errors.append('EMPTY_TRACE:' + artifact + ':' + anchor)
            for ref in references:
                prefix, _, ident = ref.partition(':')
                if prefix == 'kg':
                    if ident not in by_id or not pack['provenance'].get(ident): errors.append('INVALID_KG_TRACE:' + ref)
                elif prefix == 'repo':
                    if ident not in pack['repo'].get('contents', {}): errors.append('INVALID_REPO_TRACE:' + ref)
                elif prefix == 'decision':
                    if ident not in proposed or not proposed[ident].get('rationale'): errors.append('INVALID_DECISION_TRACE:' + ref)
                else: errors.append('UNKNOWN_TRACE:' + ref)
            # Sensitive literal facts must occur in cited evidence or explicit proposed decisions.
            grounded = '\n'.join(json.dumps(by_id.get(r[3:], {}), ensure_ascii=False) if r.startswith('kg:') else pack['repo'].get('contents', {}).get(r[5:], '') if r.startswith('repo:') else json.dumps(proposed.get(r[9:], {}), ensure_ascii=False) for r in references)
            for token in re.findall(r'\b(?:[45]\d\d|\d+\s*(?:seconds|giây|MB|days))\b|/(?:api/)?v\d/[^\s`),]+', content):
                if token not in grounded: errors.append('UNSUPPORTED_LITERAL:' + artifact + ':' + anchor + ':' + token)
            assertion['text_hash'] = fingerprint(content)
            assertion['versions'] = {r[3:]: by_id[r[3:]].get('version_introduced', pack['graph']['version']) for r in references if r.startswith('kg:') and r[3:] in by_id}
        for path, content in artifacts.items():
            for i, line in enumerate(content.splitlines(), 1):
                stripped = line.strip()
                if stripped.lower().startswith('draft note:'):
                    # A purely editorial note is diagnostic. A note that makes
                    # a normative claim or leaves an unresolved obligation is
                    # part of the evidence surface and must be anchored.
                    normative = re.search(r'\b(?:shall|must|required|unknown|unresolved|tbd|todo|pending|cannot|should)\b', stripped, re.I)
                    if not normative:
                        continue
                if stripped and not line.startswith(('#', '>')) and not re.fullmatch(r'[| :\-]+', line):
                    if (path, i) not in anchors: errors.append('UNTRACED_ASSERTION:' + path + f':line:{i}')
        # Entire baseline scenario blocks of modified requirements are preserved by semantic parent mapping.
        for delta in pack['deltas']:
            props = delta.get('props', {})
            if props.get('kind') != 'modified': continue
            before = next((n for n in pack['versions'] if n['id'] == props.get('before_requirement_id')), None)
            if not before: errors.append('MISSING_BEFORE_REQUIREMENT:' + delta['id']); continue
            cap = props.get('capability_id'); target = artifacts.get(f'specs/{cap}/spec.md', '')
            scenario_changes = {c.get('before_id'):c for c in props.get('scenario_changes', [])}
            for sc in _scenarios_for(before, pack['scenarios']):
                changed = scenario_changes.get(sc['id'])
                if changed:
                    replacement = next((n for n in pack['scenarios'] if n['id'] == changed.get('after_id')), None)
                    if not changed.get('rationale') or not replacement:
                        errors.append('INVALID_SCENARIO_CHANGE:' + sc['id']); continue
                    for key in ('given','when','then'):
                        if not replacement.get('props',{}).get(key) or replacement['props'][key] not in target: errors.append('UPDATED_SCENARIO_LOST:' + replacement['id'] + ':' + key)
                    continue
                for key in ('given', 'when', 'then'):
                    if sc.get('props', {}).get(key) and sc['props'][key] not in target: errors.append('BASELINE_SCENARIO_LOST:' + sc['id'] + ':' + key)
        for work in pack['work']:
            for key in ('deliverable', 'definition_of_done'):
                value = work.get('props', {}).get(key)
                if value and not _work_acceptance_covered(value, artifacts.get('tasks.md', '')): errors.append('WORK_ACCEPTANCE_LOST:' + work['id'] + ':' + key)
    return {'ok': not errors, 'errors': sorted(set(errors)), 'artifact_paths': sorted(artifacts)}


def validate_structured_assertions(assertions, artifacts):
    """Validate semantic assertion records without exact prose matching."""
    errors = []
    for assertion in assertions or []:
        semantic = assertion.get('semantic') or {}
        if not semantic:
            continue
        artifact = assertion.get('artifact')
        content = artifacts.get(artifact, '')
        if artifact not in artifacts:
            errors.append('SEMANTIC_ARTIFACT_MISSING:' + str(artifact)); continue
        for term in semantic.get('required_terms', []):
            if str(term).lower() not in content.lower():
                errors.append(f"SEMANTIC_TERM_MISSING:{artifact}:{term}")
        for term in semantic.get('forbidden_terms', []):
            if str(term).lower() in content.lower():
                errors.append(f"SEMANTIC_FORBIDDEN_TERM:{artifact}:{term}")
        modality = semantic.get('modality')
        if modality and not re.search(rf'\b{re.escape(str(modality))}\b', content, re.I):
            errors.append(f"SEMANTIC_MODALITY_MISSING:{artifact}:{modality}")
    return errors


def validate_with_cli(repo_root, change_name):
    try: result = subprocess.run(['openspec', 'validate', change_name, '--type', 'change', '--strict', '--no-interactive'], cwd=repo_root, capture_output=True, text=True, timeout=30)
    except (FileNotFoundError, subprocess.TimeoutExpired) as exc: return {'available': False, 'ok': False, 'error': type(exc).__name__}
    return {'available': True, 'ok': result.returncode == 0, 'returncode': result.returncode, 'stdout': result.stdout[-4000:], 'stderr': result.stderr[-4000:]}


def resolve_propose_skill():
    """Locate the openspec-propose skill without a machine-specific path.

    Order: explicit override, the copy vendored with this runtime (pinned by
    hash in the authoring record), then conventional per-user skill folders.
    """
    candidates = []
    override = os.environ.get('OPENSPEC_PROPOSE_SKILL')
    if override: candidates.append(Path(override))
    candidates.append(Path(__file__).resolve().parent/'skills'/'openspec-propose'/'SKILL.md')
    home = Path.home()
    for base in ('.claude', '.agents', '.codex'):
        candidates.append(home/base/'skills'/'openspec-propose'/'SKILL.md')
    return next((c for c in candidates if c.is_file()), None)


def propose_instructions(repo_root, name, workdir):
    """Resolve schema instructions inside an isolated copy, never modify target repo."""
    folder = Path(workdir)/'instruction-repo'; folder.mkdir(parents=True, exist_ok=True)
    if (Path(repo_root)/'openspec').exists(): shutil.copytree(Path(repo_root)/'openspec', folder/'openspec', dirs_exist_ok=True)
    result = subprocess.run(['openspec', 'new', 'change', name], cwd=folder, text=True, capture_output=True, timeout=30)
    if result.returncode: return {'error': 'INSTRUCTION_CHANGE_FAILED'}
    instructions = {}
    for artifact in ('proposal', 'design', 'specs', 'tasks'):
        result = subprocess.run(['openspec', 'instructions', artifact, '--change', name, '--json'], cwd=folder, text=True, capture_output=True, timeout=30)
        if result.returncode: return {'error': 'INSTRUCTION_FAILED:' + artifact}
        instructions[artifact] = json.loads(result.stdout)
    skill = resolve_propose_skill()
    if skill is None: return {'error': 'PROPOSE_SKILL_UNAVAILABLE'}
    instructions['skill'] = skill.read_text(); instructions['skill_hash'] = digest(skill)
    return instructions


def trace_manifest(pack, report, artifacts, assertions=None, execution=None):
    return {'request_id': pack['request']['id'], 'status': report['status'], 'blockers': report['blockers'], 'kg_snapshot': pack['request']['kg_snapshot'], 'kg_hash': pack['kg_hash'], 'repo_hash': pack['repo'].get('context_hash'), 'elements': sorted(pack['provenance']), 'artifacts': {p: fingerprint(c) for p,c in artifacts.items()}, 'assertions': assertions or [], 'authoring': execution}


def publish(request, graph, output_root, before_publish=None, runner=None, validator=validate_with_cli):
    if request.workflow_context is not None:
        workflow_errors = validate_workflow_context(request.workflow_context)
        if workflow_errors:
            return {'status': 'blocked', 'blockers': workflow_errors, 'change_name': request.change_name}
        canonical = canonical_stage5_spec(request.workflow_context)
        if canonical.get('status') != 'eligible':
            return {'status': 'blocked', 'blockers': canonical.get('errors', ['CHANGE_SPEC_NOT_ELIGIBLE']), 'change_name': request.change_name, 'change_spec': canonical}
    repo = repo_context(request.repo_root, request.code_paths); pack = build_pack(request, graph, repo)
    if request.workflow_context is not None:
        # The authoring runner receives the canonical table as source context;
        # it must project these rows instead of inventing design work from KG.
        pack['canonical_change_spec'] = canonical
    target_root = Path(output_root); target = target_root/request.change_name
    if target.exists() or request.change_name in repo['existing_changes']: return {'status': 'name_collision', 'change_name': request.change_name}
    if request.kg_snapshot not in (graph.get('version'), fingerprint(graph)): return {'status': 'stale', 'reason': 'KG_SNAPSHOT_MISMATCH'}
    target_root.parent.mkdir(parents=True, exist_ok=True)
    run_root = Path(request.run_root) if request.run_root else Path(request.repo_root)/'.tdd-runs'; run_root.mkdir(parents=True, exist_ok=True)
    run = Path(tempfile.mkdtemp(prefix=request.change_name+'-', dir=run_root))
    artifacts, assertions, execution = {}, [], None
    if runner:
        try:
            instructions = propose_instructions(request.repo_root, request.change_name, run)
            if 'error' in instructions: execution = {'status': 'blocked', 'blockers': [instructions['error']]}
            else: artifacts, assertions, execution = runner.author(pack, run/'authoring', instructions)
        except (OSError, subprocess.TimeoutExpired) as exc: execution = {'status': 'blocked', 'blockers': [type(exc).__name__]}
    report = readiness(pack, graph, execution)
    if not artifacts:
        if request.workflow_context is not None and canonical.get('status') == 'eligible':
            # A canonical Stage-5 table is a safer draft fallback than
            # inventing prose from an unscoped KG closure.  It remains draft
            # until the real openspec-propose author/review runner executes.
            artifacts = project_change_spec_to_openspec(canonical, request.change_name, request.intent, kg_context=pack.get('graph'))
        else:
            artifacts = render_artifacts(pack, report)
    validation = validate_artifacts(artifacts, pack, report, assertions if execution and execution.get('status') == 'authored' else None, (execution or {}).get('proposed_decisions'))
    if not validation['ok']:
        report['blockers'] += validation['errors']; report['status'] = 'draft'
    review = None
    if execution and execution.get('status') == 'authored':
        if runner and hasattr(runner, 'review'):
            review = runner.review(pack, artifacts, assertions, run/'review', proposed_decisions=execution.get('proposed_decisions', []))
        if not review or review.get('status') != 'reviewed' or review.get('outcome') != 'pass' or not review.get('input_hash') or not review.get('output_hash'):
            report['blockers'].append('SEMANTIC_REVIEW_REQUIRED_OR_FAILED'); report['status'] = 'draft'
    manifest = trace_manifest(pack, report, artifacts, assertions, execution)
    if request.workflow_context is not None:
        manifest['canonical_change_spec'] = canonical
    manifest['semantic_review'] = review
    staging_repo = run/'validation-repo'
    if (Path(request.repo_root)/'openspec').exists(): shutil.copytree(Path(request.repo_root)/'openspec', staging_repo/'openspec', dirs_exist_ok=True)
    staged = staging_repo/'openspec'/'changes'/request.change_name; staged.mkdir(parents=True, exist_ok=True)
    if any(e.startswith('UNSAFE_PATH:') for e in validation['errors']): return {'status': 'blocked', 'validation': validation, 'run_dir': str(run)}
    for rel, content in artifacts.items():
        path = staged/rel; path.parent.mkdir(parents=True, exist_ok=True); path.write_text(content)
    cli = validator(staging_repo, request.change_name)
    if not cli.get('ok'):
        report['blockers'].append('CLI_VALIDATION_FAILED'); report['status'] = 'draft'
    report = readiness(pack, graph, execution, artifacts, validation, cli, review)
    manifest['status'] = report['status']; manifest['blockers'] = sorted(set(report['blockers'])); manifest['cli_validation'] = cli
    (staged/'trace-manifest.json').write_text(json.dumps(manifest, indent=2, ensure_ascii=False))
    if before_publish: before_publish()
    if source_snapshot_errors(graph) or fingerprint(graph) != pack['kg_hash'] or repo_context(request.repo_root, request.code_paths) != repo:
        return {'status': 'stale', 'reason': 'SNAPSHOT_CHANGED', 'run_dir': str(run), 'manifest': manifest}
    if target.exists(): return {'status': 'name_collision', 'run_dir': str(run)}
    if report['status'] != 'implementation_ready':
        if not request.allow_draft: return {'status': 'blocked', 'readiness': report, 'validation': validation, 'cli_validation': cli, 'manifest': manifest, 'run_dir': str(run)}
        target_root = target_root.parent/'drafts'; target = target_root/request.change_name
        for name in ('proposal.md','design.md','tasks.md'):
            path = staged/name
            if path.exists(): path.write_text('> DRAFT: see trace-manifest.json for blockers.\n\n' + path.read_text())
    target_root.mkdir(parents=True, exist_ok=True)
    # mkdir is the atomic no-clobber reservation; target only becomes visible on
    # ready publication after staged validation. Hidden lock prevents competing runs.
    lock = target_root/('.'+request.change_name+'.publish-lock')
    try: lock.mkdir()
    except FileExistsError: return {'status': 'name_collision', 'run_dir': str(run)}
    try:
        if source_snapshot_errors(graph) or fingerprint(graph) != pack['kg_hash'] or repo_context(request.repo_root, request.code_paths) != repo:
            return {'status': 'stale', 'reason': 'SNAPSHOT_CHANGED', 'run_dir': str(run)}
        if target.exists(): return {'status': 'name_collision', 'run_dir': str(run)}
        os.rename(staged, target)
    finally: lock.rmdir()
    return {'status': 'published' if report['status'] == 'implementation_ready' else 'draft', 'readiness': report, 'validation': validation, 'cli_validation': cli, 'manifest': manifest, 'path': str(target), 'run_dir': str(run)}
