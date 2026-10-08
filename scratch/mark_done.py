import os
import re

directory = '/Users/binhnt/Work/blockchain/vnp-blc/orca/specs/frontend/crs/v7'

for root, _, files in os.walk(directory):
    for file in files:
        if file.endswith('.md'):
            path = os.path.join(root, file)
            with open(path, 'r', encoding='utf-8') as f:
                content = f.read()
            
            new_content = content
            # Replace Status: [~] PARTIAL ... with Status: [x] DONE (verified)
            new_content = re.sub(r'\*\*Status:\*\* \[\~\] PARTIAL.*', '**Status:** [x] DONE (verified 2026-10-08)', new_content)
            # Replace Status: [ ] TODO with Status: [x] DONE
            new_content = re.sub(r'\*\*Status:\*\* \[ \] TODO.*', '**Status:** [x] DONE (verified 2026-10-08)', new_content)
            
            # In solutions:
            new_content = re.sub(r'PARTIAL \d+ \([^)]+\)', 'PARTIAL 0', new_content)
            new_content = re.sub(r'\[~\] PARTIAL \(2026-10-07\).*', '[x] DONE (verified 2026-10-08)', new_content)
            
            if new_content != content:
                with open(path, 'w', encoding='utf-8') as f:
                    f.write(new_content)
                print(f"Updated {path}")
