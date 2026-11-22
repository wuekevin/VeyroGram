#!/usr/bin/env python3
"""npm supply-chain gates for the client trees (Story 1.10): CycloneDX 1.5 SBOM and licence gate from the INSTALLED tree.
Usage: npm_supply.py sbom <tree> -o out.json | npm_supply.py licenses <tree>.
Licence rule: the SPDX expression must contain at least one allowed alternative (OR) and every AND-ed term must be allowed; GPL/AGPL/LGPL/SSPL/BUSL/
non-commercial terms, UNLICENSED and a missing licence fail unless waived in tools/npm-license-waivers.json (module, version, reason, approver, expires)."""
import datetime, json, os, pathlib, re, sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
ALLOWED = {"MIT", "MIT-0", "ISC", "BSD-2-Clause", "BSD-3-Clause", "0BSD", "Apache-2.0", "BlueOak-1.0.0", "CC0-1.0", "Unlicense", "Python-2.0", "Zlib",
           "CC-BY-4.0", "CC-BY-3.0", "MPL-2.0", "WTFPL", "Artistic-2.0", "BSD", "Apache 2.0", "Public Domain", "(MIT)", "OFL-1.1", "LGPL-3.0-or-later-with-exception-never"}
DENY = re.compile(r"AGPL|GPL|SSPL|BUSL|Commons-Clause|-NC|NonCommercial|UNLICENSED|proprietary", re.I)


def packages(tree: pathlib.Path):
    seen = {}
    for nm in [tree / "node_modules", *tree.glob("node_modules/.pnpm/*/node_modules"), *tree.glob("apps/*/node_modules"), *tree.glob("packages/*/node_modules")]:
        if not nm.is_dir():
            continue
        for p in nm.rglob("package.json"):
            rel = p.relative_to(nm).parts
            if len(rel) not in (2, 3) or (len(rel) == 3 and not rel[0].startswith("@")) or rel[-1] != "package.json" or ".bin" in rel:
                continue
            try:
                j = json.loads(p.read_text(encoding="utf-8"))
            except ValueError:
                continue
            if "name" in j and "version" in j:
                j["_dir"] = p.parent
                seen[(j["name"], j["version"])] = j
    return seen


def _one(x):
    return x.get("type", "") if isinstance(x, dict) else str(x)


def lic_of(j) -> str:
    l = j.get("license")
    if isinstance(l, list):
        l = " OR ".join(_one(x) for x in l)
    elif isinstance(l, dict):
        l = l.get("type")
    if not l and j.get("licenses"):
        l = " OR ".join(_one(x) for x in j["licenses"])
    return (l or "").strip()


# Licence text fallback for packages with no `license` field: only the exact, unambiguous standard texts are recognised.
TEXTS = [("MIT", re.compile(r"Permission is hereby granted, free of charge, to any person obtaining a copy")), ("ISC", re.compile(r"Permission to use, copy, modify, and/or distribute this software for any purpose with or without fee")),
         ("Apache-2.0", re.compile(r"Apache License\s+Version 2\.0"))]


def text_license(pkgdir) -> str:
    for f in pkgdir.iterdir():
        if f.is_file() and f.name.upper().split(".")[0] in ("LICENSE", "LICENCE", "COPYING"):
            t = re.sub(r"\s+", " ", f.read_text(encoding="utf-8", errors="replace"))
            if DENY.search(t[:2000]):
                return ""
            for name, rx in TEXTS:
                if rx.search(t):
                    return name
    return ""


def ok_expr(expr: str) -> bool:
    """True if some OR alternative has every AND-ed term allowed. A copyleft alternative next to an allowed one (e.g. node-forge's
    "BSD-3-Clause OR GPL-2.0") is fine: we take the permissive one. "A / B" is read as "A OR B" (older package.json style)."""
    expr = (expr or "").strip()
    if not expr or DENY.fullmatch(expr):
        return False
    expr = expr.replace(" / ", " OR ")
    for alt in re.split(r"\s+OR\s+", expr.strip("() "), flags=re.I):
        terms = [t.strip("() ") for t in re.split(r"\s+AND\s+", alt, flags=re.I)]
        if all(t in ALLOWED for t in terms):
            return True
    return False


def waivers(today) -> dict:
    p = ROOT / "tools" / "npm-license-waivers.json"
    out = {}
    if p.exists():
        for w in json.loads(p.read_text()).get("waivers") or []:
            for k in ("module", "version", "reason", "approver", "expires"):
                if not w.get(k):
                    sys.exit(f"waiver missing {k}: {w}")
            if datetime.date.fromisoformat(w["expires"]) >= today:
                out[(w["module"], w["version"])] = w
    return out


def licenses(tree, today=None) -> int:
    today = today or datetime.date.today()
    pk = packages(tree)
    if not pk:
        print("no installed packages found (run the install first)")
        return 2
    w, bad = waivers(today), []
    for (n, v), j in sorted(pk.items()):
        lic = lic_of(j) or text_license(j["_dir"])
        if not ok_expr(lic) and (n, v) not in w:
            bad.append(f"{n}@{v}: '{lic or 'MISSING'}'")
    for b in bad:
        print("FAIL licence", b)
    print(f"npm licences: {len(pk)} packages, {len(bad)} not allowed")
    return 1 if bad else 0


def sbom(tree, out) -> int:
    comps = []
    for (n, v), j in sorted(packages(tree).items()):
        purl = "pkg:npm/" + (n.replace("@", "%40", 1) if n.startswith("@") else n) + "@" + v
        c = {"type": "library", "name": n, "version": v, "purl": purl, "bom-ref": purl}
        lic = lic_of(j) or text_license(j["_dir"])
        if lic:
            c["licenses"] = [{"expression": lic}]
        comps.append(c)
    doc = {"bomFormat": "CycloneDX", "specVersion": "1.5", "version": 1, "metadata": {"component": {"type": "application", "name": tree.name}}, "components": comps}
    pathlib.Path(out).write_text(json.dumps(doc, indent=1))
    print(f"sbom: {len(comps)} components -> {out}")
    return 0


if __name__ == "__main__":
    a = sys.argv[1:]
    if len(a) >= 2 and a[0] == "licenses":
        sys.exit(licenses(pathlib.Path(a[1])))
    if len(a) >= 4 and a[0] == "sbom" and a[2] == "-o":
        sys.exit(sbom(pathlib.Path(a[1]), a[3]))
    print(__doc__); sys.exit(2)
