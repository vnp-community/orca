"""Versioned quality schemas generated from this single contract source."""
import json
from pathlib import Path
VERSION = '1.0.0'
STRING = {'type': 'string', 'minLength': 1}
ARRAY = {'type': 'array'}
SCHEMAS = {
 'KnowledgeNeedResult': {'need_id': STRING, 'scope': ARRAY, 'applicability': {'enum': ['applicable', 'not_applicable']}, 'reason': STRING, 'searched_sources': ARRAY, 'matched_evidence': ARRAY, 'status': {'enum': ['found', 'missing', 'conflict', 'not_applicable']}, 'conflicts': ARRAY},
 'FactLineage': {'kind': {'enum': ['extracted', 'derived', 'proposed', 'prompt']}, 'source_id': STRING, 'source_version': STRING, 'source_hash': STRING, 'block': STRING},
 'ValidatorExecution': {'rule_id': STRING, 'version': STRING, 'input_hash': STRING, 'outcome': {'enum': ['pass', 'fail', 'error', 'unknown', 'not_applicable']}, 'evidence': {'type': 'array', 'minItems': 1}},
 'NeedValidatorExecution': {'need_id': STRING, 'rule_id': STRING, 'version': STRING, 'input_hash': STRING, 'outcome': {'enum': ['pass', 'fail', 'error', 'unknown', 'not_applicable']}, 'evidence': {'type': 'array', 'minItems': 1}, 'applicability': {'enum': ['applicable', 'not_applicable']}, 'matched_evidence': {'type': 'array'}},
 'ArtifactAnchorTrace': {'artifact': STRING, 'anchor': STRING, 'text_hash': STRING, 'references': {'type': 'array', 'minItems': 1}}
}

def generate(root=None):
    root = Path(root or Path(__file__).resolve().parents[1] / 'schemas')
    paths = []
    for name, properties in SCHEMAS.items():
        schema = {'$schema': 'https://json-schema.org/draft/2020-12/schema', '$id': f'urn:tdd:quality:{name}:{VERSION}', 'type': 'object', 'properties': properties, 'required': list(properties), 'additionalProperties': True}
        if name == 'FactLineage':
            schema['allOf'] = [{'if': {'properties': {'kind': {'const': 'proposed'}}}, 'then': {'required': ['decision_id']}}, {'if': {'properties': {'kind': {'const': 'derived'}}}, 'then': {'required': ['inputs', 'derivation']}}]
        path = root / (name + '.quality.schema.json'); path.write_text(json.dumps(schema, indent=2)+'\n'); paths.append(str(path))
    return paths

def validate(name, record):
    errors = []
    for key, spec in SCHEMAS[name].items():
        value = record.get(key)
        if value is None: errors.append('missing:' + key); continue
        if spec.get('type') == 'string' and (not isinstance(value, str) or not value): errors.append('invalid:' + key)
        if spec.get('type') == 'array' and (not isinstance(value, list) or len(value) < spec.get('minItems', 0)): errors.append('invalid:' + key)
        if spec.get('enum') and value not in spec['enum']: errors.append('invalid:' + key)
    if name == 'FactLineage':
        from quality5 import evidence_errors
        errors += evidence_errors(record)
        if record.get('kind') == 'derived' and (not record.get('inputs') or not record.get('derivation')): errors.append('missing:derivation')
    return errors

if __name__ == '__main__': print(json.dumps(generate()))
