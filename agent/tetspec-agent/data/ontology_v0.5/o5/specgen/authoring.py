"""Bounded agent authoring through an installed command runner.

The runner is exercised as the application under test, not used to delegate
repository implementation. It has a read-only sandbox and returns artifacts.
"""
from pathlib import Path
import json, shutil, subprocess, time, os, tomllib
from quality5 import fingerprint

OUTPUT_SCHEMA = {'type': 'object', 'properties': {
 'artifacts': {'type': 'array', 'items': {'type': 'object', 'properties': {'path': {'type': 'string'}, 'content': {'type': 'string'}}, 'required': ['path', 'content'], 'additionalProperties': False}},
 'assertions': {'type': 'array', 'items': {'type': 'object', 'properties': {'artifact': {'type': 'string'}, 'anchor': {'type': 'string'}, 'references': {'type': 'array', 'items': {'type': 'string'}}}, 'required': ['artifact', 'anchor', 'references'], 'additionalProperties': False}},
 'proposed_decisions': {'type': 'array', 'items': {'type': 'object', 'properties': {'id': {'type': 'string'}, 'rationale': {'type': 'string'}, 'status': {'enum': ['proposed']}}, 'required': ['id', 'rationale', 'status'], 'additionalProperties': False}}
 }, 'required': ['artifacts', 'assertions', 'proposed_decisions'], 'additionalProperties': False}


def validate_relative_authoring_paths(authored):
    """Reject traversal, absolute paths, duplicate artifacts and dangling assertions."""
    errors = []
    paths = [str(item.get('path', '')) for item in authored.get('artifacts', [])]
    if len(paths) != len(set(paths)):
        errors.append('DUPLICATE_ARTIFACT_PATH')
    for path in paths:
        candidate = Path(path)
        if not path or candidate.is_absolute() or '..' in candidate.parts or '\\' in path:
            errors.append('UNSAFE_ARTIFACT_PATH:' + path)
    known = set(paths)
    for assertion in authored.get('assertions', []):
        if assertion.get('artifact') not in known:
            errors.append('ASSERTION_ARTIFACT_MISSING:' + str(assertion.get('artifact')))
        anchor = str(assertion.get('anchor', ''))
        if not anchor.startswith('line:') or not anchor[5:].isdigit() or int(anchor[5:]) < 1:
            errors.append('INVALID_ASSERTION_ANCHOR:' + anchor)
    return sorted(set(errors))

class CodexProposeRunner:
    def __init__(self, command='codex', timeout=180, max_attempts=2, model=None):
        self.command, self.timeout, self.max_attempts, self.model = command, timeout, max_attempts, model
        config_path = Path(os.environ.get('CODEX_HOME', str(Path.home()/'.codex')))/'config.toml'
        try:
            config = tomllib.loads(config_path.read_text())
            self.configuration = {key:config.get(key) for key in ('model','model_provider','model_reasoning_effort','model_service_tier')}
        except (OSError, ValueError): self.configuration = {}
        self.configuration.update(command=command, timeout=timeout, max_attempts=max_attempts, model_override=model)
        self.config_hash = fingerprint(self.configuration)
        if max_attempts < 1 or max_attempts > 3: raise ValueError('max_attempts must be 1..3')

    def author(self, pack, output_dir, instructions):
        output = Path(output_dir); output.mkdir(parents=True, exist_ok=True)
        payload = {'pack': pack, 'instructions': instructions, 'output_schema': OUTPUT_SCHEMA}
        (output/'input.json').write_text(json.dumps(payload, indent=2, ensure_ascii=False))
        schema = output/'output.schema.json'; schema.write_text(json.dumps(OUTPUT_SCHEMA))
        record = {'workflow': 'openspec-propose', 'runner': self.command, 'model': self.model or 'inherited configuration', 'config_hash': self.config_hash, 'configuration': self.configuration, 'input_hash': fingerprint(payload), 'attempts': [], 'status': 'blocked'}
        if not shutil.which(self.command):
            record['blockers'] = ['AUTHORING_RUNNER_UNAVAILABLE']; self._save(output, record); return {}, [], record
        prompt = '''Execute the openspec-propose authoring workflow using ONLY the pinned context supplied below. Do not edit files, run shell commands, implement code or change KG. Read and follow the supplied skill and artifact instructions. Return the JSON artifacts, assertion trace and proposed decisions required by the output schema. Produce full proposal, design, tasks and per-capability spec deltas. Design must contain concrete contracts, decisions, alternatives, risks and applicable migration. Tasks require deliverable, dependency and acceptance. Preserve every still-effective scenario and GIVEN conditions for modified requirements. Do not guess missing behavior or silently invent values. Each nonempty substantive line must have assertion mapping artifact+line:N to references such as kg:ID, repo:relative/path or decision:ID. A purely editorial line beginning `Draft note:` may omit an assertion; if it states a normative rule, unknown/unresolved obligation, TODO/TBD or pending decision it MUST be asserted and grounded. Active changes are proposals, not proof of baseline behavior. All factual statements MUST be supported by those exact references; identify newly proposed design choices separately. Keep artifacts concise enough that every line is traceable. Output path is relative and must stay under change directory. Context:\n''' + json.dumps(payload, ensure_ascii=False)
        for attempt in range(1, self.max_attempts+1):
            result_path = output/f'output-{attempt}.json'
            command = [self.command, 'exec', '--sandbox', 'read-only', '--ephemeral', '--skip-git-repo-check', '--output-schema', str(schema.resolve()), '-o', str(result_path.resolve()), '-']
            selected_model = self.model or self.configuration.get('model')
            if selected_model: command[2:2] = ['--model', selected_model]
            started = time.monotonic()
            try:
                result = subprocess.run(command, input=prompt, cwd=output, capture_output=True, text=True, timeout=self.timeout)
                entry = {'attempt': attempt, 'returncode': result.returncode, 'elapsed_seconds': round(time.monotonic()-started, 2)}
                if result.returncode == 0 and result_path.exists():
                    authored = json.loads(result_path.read_text())
                    path_errors = validate_relative_authoring_paths(authored)
                    if path_errors: raise ValueError(';'.join(path_errors))
                    record['status'] = 'authored'; record['output_hash'] = fingerprint(authored); record['attempts'].append(entry)
                    record['proposed_decisions'] = authored['proposed_decisions']; self._save(output, record)
                    return {x['path']: x['content'] for x in authored['artifacts']}, authored['assertions'], record
                entry['error'] = 'RUNNER_NONZERO' if result.returncode else 'NO_STRUCTURED_OUTPUT'
                # Store diagnostics locally; never echo runner credential/config content.
                (output/f'diagnostics-{attempt}.log').write_text(result.stderr)
                record['attempts'].append(entry)
            except (subprocess.TimeoutExpired, ValueError, KeyError, OSError) as exc:
                record['attempts'].append({'attempt': attempt, 'error': type(exc).__name__, 'elapsed_seconds': round(time.monotonic()-started, 2)})
        record['blockers'] = ['AUTHORING_EXECUTION_FAILED']; self._save(output, record)
        return {}, [], record

    def _save(self, output, record):
        (output/'execution.json').write_text(json.dumps(record, indent=2))

    def review(self, pack, artifacts, assertions, output_dir, proposed_decisions=None):
        """Fresh critique invocation, independent from the authoring conversation."""
        output = Path(output_dir); output.mkdir(parents=True, exist_ok=True)
        payload = {'pack': pack, 'artifacts': artifacts, 'assertions': assertions,
                   'proposed_decisions': proposed_decisions or []}
        schema = {'type':'object','properties':{'verdict':{'enum':['pass','fail']},'findings':{'type':'array','items':{'type':'object','properties':{'artifact':{'type':'string'},'anchor':{'type':'string'},'reason':{'type':'string'}},'required':['artifact','anchor','reason'],'additionalProperties':False}}},'required':['verdict','findings'],'additionalProperties':False}
        (output/'input.json').write_text(json.dumps(payload,ensure_ascii=False,indent=2));(output/'schema.json').write_text(json.dumps(schema))
        record={'workflow':'semantic-review','config_hash':self.config_hash,'input_hash':fingerprint(payload),'status':'blocked','attempts':[]}
        if not shutil.which(self.command):record['blockers']=['REVIEW_RUNNER_UNAVAILABLE'];self._save(output,record);return record
        prompt='Independently review the proposed OpenSpec artifacts against ONLY the pinned evidence. Do not edit files or use tools. Reject unsupported statements/dependencies/status codes, lost baseline scenarios/GIVEN conditions, wrong capability deltas, generic tasks without deliverable/dependency/acceptance, missing mandatory error/boundary/security/migration behavior where applicable. Trace IDs must support the actual meaning at each anchor, not merely exist. Proposed choices must be identified as proposals. Return fail with exact artifact line:N findings if any substantive problem; do not silently repair. Context:\n'+json.dumps(payload,ensure_ascii=False)
        for attempt in range(1,self.max_attempts+1):
            path=output/f'output-{attempt}.json';command=[self.command,'exec','--sandbox','read-only','--ephemeral','--skip-git-repo-check','--output-schema',str((output/'schema.json').resolve()),'-o',str(path.resolve()),'-']
            selected_model=self.model or self.configuration.get('model')
            if selected_model:command[2:2]=['--model',selected_model]
            try:
                result=subprocess.run(command,input=prompt,cwd=output,capture_output=True,text=True,timeout=self.timeout)
                record['attempts'].append({'attempt':attempt,'returncode':result.returncode})
                if result.returncode==0 and path.exists():
                    review=json.loads(path.read_text());record.update(status='reviewed',outcome='fail' if review['findings'] else review['verdict'],findings=review['findings'],output_hash=fingerprint(review));self._save(output,record);return record
                (output/f'diagnostics-{attempt}.log').write_text(result.stderr)
            except (OSError,ValueError,KeyError,subprocess.TimeoutExpired) as exc:record['attempts'].append({'attempt':attempt,'error':type(exc).__name__})
        record['blockers']=['SEMANTIC_REVIEW_EXECUTION_FAILED'];self._save(output,record);return record
