# Calc sandbox

Tiny throwaway repo used by an automated test. Work only inside this directory: do not read or write anything outside it,
do not install packages, do not use the network, and do not push or change git remotes.

- Code is in `src/calc.py`, tests in `tests/test_calc.py`. Run the tests with `python3 -m unittest` (Python 3 only; there is no Node).
- Keep changes minimal; the harness reads the working tree.
