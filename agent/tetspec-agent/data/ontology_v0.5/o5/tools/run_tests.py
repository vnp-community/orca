#!/usr/bin/env python3
"""Run function tests and unittest cases without a pytest dependency."""
import importlib.util, inspect, tempfile, unittest, traceback
from pathlib import Path
import sys
ROOT = Path(__file__).resolve().parents[1]
for part in ('edit','specgen','harness','tools'): sys.path.insert(0,str(ROOT/part))
passed, failures = 0, []
for path in sorted((ROOT/'tests').glob('test_*.py')):
    spec = importlib.util.spec_from_file_location('suite_'+path.stem,path); module=importlib.util.module_from_spec(spec); spec.loader.exec_module(module)
    for name in sorted(n for n in dir(module) if n.startswith('test_')):
        fn = getattr(module,name)
        if not inspect.isfunction(fn): continue
        try:
            if inspect.signature(fn).parameters:
                with tempfile.TemporaryDirectory() as tmp: fn(Path(tmp))
            else: fn()
            passed+=1; print('PASS '+path.name+'::'+name)
        except Exception:
            failures.append(path.name+'::'+name); traceback.print_exc()
    suite=unittest.defaultTestLoader.loadTestsFromModule(module)
    if not suite.countTestCases(): continue
    result=unittest.TextTestRunner(verbosity=1).run(suite)
    passed += result.testsRun-len(result.failures)-len(result.errors)
    failures += [str(case) for case,_ in result.failures+result.errors]
print(f'{passed} passed, {len(failures)} failed')
if failures: print('\n'.join(failures)); sys.exit(1)
