# -*- coding: utf-8 -*-
"""Negative tests for tools/check_upstream_compat.py (Story 1.23): the guard must really fail when a legacy path or an undecided table appears."""
import os, shutil, subprocess, sys, tempfile

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))


def run(tmp):
    return subprocess.run([sys.executable, "-I", os.path.join(tmp, "tools/check_upstream_compat.py")], capture_output=True, text=True)


def fresh():
    tmp = tempfile.mkdtemp()
    for d in ("tools/check_upstream_compat.py", "docs/migration", "docs/audit/endpoints-inventory.yaml", "packages/contracts/openapi"):
        src, dst = os.path.join(ROOT, d), os.path.join(tmp, d)
        os.makedirs(os.path.dirname(dst), exist_ok=True)
        (shutil.copytree if os.path.isdir(src) else shutil.copy)(src, dst)
    return tmp


fails = []
def check(name, cond):
    print(("ok   " if cond else "FAIL ") + name)
    if not cond:
        fails.append(name)

t = fresh(); r = run(t); check("clean tree passes", r.returncode == 0)

t = fresh()  # a legacy upstream path (GET /v1/health is in the upstream inventory) sneaks into the contract
with open(os.path.join(t, "packages/contracts/openapi/auth.yaml"), encoding="utf-8") as f: y = f.read()
y = y.replace("paths:\n", "paths:\n  /v1/health:\n    get:\n      responses: {'200': {description: x}}\n", 1)
open(os.path.join(t, "packages/contracts/openapi/auth.yaml"), "w", encoding="utf-8").write(y)
r = run(t); check("legacy path in the contract is refused", r.returncode == 1 and "/v1/health" in r.stdout)

t = fresh()
p = os.path.join(t, "docs/migration/upstream-tables.yaml")
open(p, "a", encoding="utf-8").write("  some_table: UNDECIDED\n")
r = run(t); check("an undecided table is refused", r.returncode == 1 and "some_table" in r.stdout)

t = fresh()
p = os.path.join(t, "docs/migration/COMPAT-PATHS.md")
open(p, "w", encoding="utf-8").write("# listed\n/v1/user/login\n")
r = run(t); check("the empty-list declaration cannot be removed silently", r.returncode == 1)

sys.exit(1 if fails else 0)
