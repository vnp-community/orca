#!/usr/bin/env python3
"""Reassess saved outputs without rerunning authoring or replacing original reports."""
import argparse,json,sys,shutil
from pathlib import Path
from concurrent.futures import ThreadPoolExecutor
ROOT=Path(__file__).resolve().parents[1]
for part in ('tools','specgen'):sys.path.insert(0,str(ROOT/part))
from authoring import CodexProposeRunner
from spec_change import validate_artifacts,validate_with_cli
from quality5 import fingerprint

def reconcile_corrected_review(latest, prior):
    """Keep a reviewed record when a later local rerun is unavailable.

    A blocked Codex app-server rerun is diagnostic evidence, not a new review
    verdict.  The historical reviewed output remains the acceptance record;
    the failed attempt is returned separately so it cannot silently erase it.
    """
    if prior and prior.get('status') == 'reviewed' and latest.get('status') != 'reviewed':
        return prior, {
            'status': latest.get('status'),
            'blockers': latest.get('blockers', []),
            'attempts': latest.get('attempts', []),
            'evidence': 'review-with-decisions/execution.json',
        }
    return latest, None

def main():
    parser=argparse.ArgumentParser();parser.add_argument('--run-dir',required=True);parser.add_argument('--review',action='store_true');args=parser.parse_args()
    root=Path(args.run_dir).resolve();pack=json.loads((root/'benchmark-context.json').read_text());name=pack['request']['change_name']
    previous = {}
    previous_path = root/'benchmark.reassessment.json'
    if previous_path.exists():
        try:
            previous = {str(x.get('id')): x for x in json.loads(previous_path.read_text()).get('runs', [])}
        except (OSError, ValueError, TypeError):
            previous = {}
    def assess(folder):
        authored=json.loads((folder/'authoring/output-1.json').read_text())
        artifacts={a['path']:a['content'] for a in authored['artifacts']}
        normalized={};paths={}
        for path,content in artifacts.items():
            prefix=f'openspec/changes/{name}/'
            rel=path[len(prefix):] if path.startswith(prefix) else path
            if Path(rel).is_absolute() or '..' in Path(rel).parts:raise ValueError('unsafe saved path')
            normalized[rel]=content;paths[path]=rel
        if len(normalized)!=len(artifacts):raise ValueError('normalization collision')
        assertions=[{**a,'artifact':paths.get(a['artifact'],a['artifact'])} for a in authored['assertions']]
        validation=validate_artifacts(normalized,pack,{'status':'draft'},assertions,authored['proposed_decisions'])
        staging=folder/'diagnostic-staging';shutil.copytree(root/'benchmark-repo/openspec',staging/'openspec',dirs_exist_ok=True)
        for path,content in normalized.items():
            target=staging/'openspec/changes'/name/path;target.parent.mkdir(parents=True,exist_ok=True);target.write_text(content)
        cli=validate_with_cli(staging,name)
        output={'id':folder.name,'raw_authoring_hash':fingerprint(authored),'normalization':'Diagnostic path-prefix normalization only; original outputs/reports preserved.','path_mapping':paths,'semantic_validation':validation,'strict_cli':cli}
        if args.review:
            original_input=json.loads((folder/'authoring/input.json').read_text())
            runner=CodexProposeRunner(timeout=180,max_attempts=1)
            latest=runner.review(original_input['pack'],artifacts,authored['assertions'],folder/'review-with-decisions',proposed_decisions=authored['proposed_decisions'])
            prior=previous.get(folder.name, {}).get('corrected_review')
            output['corrected_review'], latest_rerun = reconcile_corrected_review(latest, prior)
            if latest_rerun:
                output['latest_rerun'] = latest_rerun
        (folder/'reassessment.json').write_text(json.dumps(output,ensure_ascii=False,indent=2)+'\n')
        print(json.dumps({'run':folder.name,'cli':cli['ok'],'semantic_valid':validation['ok'],'corrected_review':output.get('corrected_review',{}).get('outcome')},ensure_ascii=False),flush=True)
        return output
    folders=sorted(p for p in (root/'benchmark').iterdir() if (p/'authoring/output-1.json').exists())
    with ThreadPoolExecutor(max_workers=3) as pool:results=list(pool.map(assess,folders))
    (root/'benchmark.reassessment.json').write_text(json.dumps({'original_reports_preserved':True,'human_review':'pending','runs':results, 'note':'Historical corrected-review outputs are preserved when a later local rerun is blocked.'},ensure_ascii=False,indent=2)+'\n')
if __name__=='__main__':main()
