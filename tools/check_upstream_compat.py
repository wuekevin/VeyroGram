# -*- coding: utf-8 -*-
"""Story 1.23 contract check: VeyroGram must not carry upstream (TangSengDaoDao) paths, and every upstream table has a written disposition.

  python3 tools/check_upstream_compat.py [--regen]

Checks (exit 1 on any failure):
  1. Every (method, path) declared in packages/contracts/openapi/*.yaml is absent from the upstream endpoint inventory. The compat path list is
     EMPTY by decision (ADR-0011); a collision means a legacy path leaked into the new contract.
  2. docs/migration/upstream-tables.yaml lists every table the pinned upstream creates (checked against upstream/ when it is present) and gives
     each one a disposition from a closed set.
  3. docs/migration/COMPAT-PATHS.md still declares the list empty.
--regen rewrites the table list from upstream/ keeping existing dispositions (new tables get "UNDECIDED", which fails check 2).
"""
import glob, os, re, sys
import yaml

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
DISPOSITIONS = {"replaced", "im-owned", "import-candidate", "out-of-scope"}
HEADER = "".join(l for l in open(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "docs/migration/upstream-tables.yaml"), encoding="utf-8") if l.startswith("#"))
TABLES = os.path.join(ROOT, "docs/migration/upstream-tables.yaml")


def norm(p):
    p = re.sub(r":([A-Za-z_]+)", "{x}", p)
    p = re.sub(r"\{[^}/]+\}", "{x}", p)
    return p.rstrip("/") or "/"


def upstream_tables():
    base = os.path.join(ROOT, "upstream/TangSengDaoDaoServer")
    if not os.path.isdir(base):
        return None
    out = set()
    for f in glob.glob(base + "/**/*.sql", recursive=True):
        txt = open(f, encoding="utf-8", errors="replace").read()
        for m in re.finditer(r"create\s+table(?:\s+if\s+not\s+exists)?\s+`?([A-Za-z_0-9]+)`?", txt, re.I):
            out.add(m.group(1).lower())
    return out


def main():
    bad = []
    inv = yaml.safe_load(open(os.path.join(ROOT, "docs/audit/endpoints-inventory.yaml"), encoding="utf-8"))["endpoints"]
    legacy = {(e["method"].upper(), norm(e["path"])) for e in inv}
    ours = set()
    for f in glob.glob(os.path.join(ROOT, "packages/contracts/openapi/*.yaml")):
        for path, ops in (yaml.safe_load(open(f, encoding="utf-8")).get("paths") or {}).items():
            for m in ops:
                if m.lower() in ("get", "post", "put", "patch", "delete"):
                    ours.add((m.upper(), norm(path)))
    for c in sorted(ours & legacy):
        bad.append("contract path collides with an upstream legacy path: %s %s" % c)

    doc = yaml.safe_load(open(TABLES, encoding="utf-8")) if os.path.exists(TABLES) else {"tables": {}}
    tabs = doc.get("tables") or {}
    up = upstream_tables()
    if "--regen" in sys.argv and up is not None:
        for t in sorted(up):
            tabs.setdefault(t, "UNDECIDED")
        doc["tables"] = dict(sorted(tabs.items()))
        with open(TABLES, "w", encoding="utf-8") as fh:
            fh.write(HEADER)
            yaml.safe_dump({"tables": doc["tables"]}, fh, allow_unicode=True, sort_keys=False)
    if up is not None:
        for t in sorted(up - set(tabs)):
            bad.append("upstream table without a disposition: " + t)
    for t, d in tabs.items():
        if d not in DISPOSITIONS:
            bad.append("table %s has invalid disposition %r" % (t, d))
    cp = os.path.join(ROOT, "docs/migration/COMPAT-PATHS.md")
    if not os.path.exists(cp) or "COMPAT_PATHS: none" not in open(cp, encoding="utf-8").read():
        bad.append("docs/migration/COMPAT-PATHS.md must declare 'COMPAT_PATHS: none'")
    for b in bad:
        print("FAIL:", b)
    print("upstream-compat: %d contract operations, %d upstream endpoints, %d tables dispositioned%s" % (
        len(ours), len(legacy), len(tabs), "" if up is not None else " (upstream/ absent: table list not re-verified)"))
    return 1 if bad else 0


if __name__ == "__main__":
    sys.exit(main())
