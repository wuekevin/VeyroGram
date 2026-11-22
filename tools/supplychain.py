# -*- coding: utf-8 -*-
"""Supply-chain gates (Story 1.24; AD-18, NFR-5, NFR-17).

  supplychain.py sbom     [-o FILE]   CycloneDX 1.5 JSON for what the Go build actually links (test dependencies included), with go.sum hashes
  supplychain.py licenses            classify each dependency's licence text; fail on anything not in the allow-list (UNLICENSED, unknown,
                                     custom/commercial, copyleft). An entry in tools/supplychain-waivers.yaml (reason, approver, expiry) is the only way past.
  supplychain.py mirrors             fail if any manifest or lockfile points at a third-party mirror (npmmirror, taobao, ...)

Everything is read from the module cache and the repository; nothing is sent anywhere.
"""
import datetime, glob, hashlib, json, os, re, subprocess, sys
import yaml

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
API = os.path.join(ROOT, "services/api")
WAIVERS = os.path.join(ROOT, "tools/supplychain-waivers.yaml")

# Checked first: anything matching is blocked, whatever else the file also says.
DENY = [
    ("AGPL", r"GNU AFFERO GENERAL PUBLIC LICENSE"), ("GPL", r"GNU GENERAL PUBLIC LICENSE"), ("LGPL", r"GNU LESSER GENERAL PUBLIC LICENSE|GNU LIBRARY GENERAL PUBLIC"),
    ("SSPL", r"Server Side Public License"), ("BUSL", r"Business Source License"), ("Commons-Clause", r"Commons Clause"),
    ("Elastic", r"Elastic License"), ("non-commercial", r"non-?commercial|not for commercial|no commercial use"),
]
ALLOW = [
    ("Apache-2.0", r"Apache License[\s\S]{0,80}Version 2\.0"), ("MPL-2.0", r"Mozilla Public License[\s\S]{0,40}(Version )?2\.0"),
    ("MIT", r"Permission is hereby granted, free of charge"), ("ISC", r"Permission to use, copy, modify, and/or distribute this software"),
    ("BSD-3-Clause", r"Redistribution and use in source and binary forms[\s\S]*Neither the name"), ("BSD-2-Clause", r"Redistribution and use in source and binary forms"),
    ("Unlicense", r"This is free and unencumbered software released into the public domain"), ("0BSD", r"Permission to use, copy, modify, and/or distribute this software for any purpose with or without fee is hereby granted"),
]
LICENSE_FILES = ("LICENSE", "LICENCE", "COPYING", "COPYRIGHT", "NOTICE", "UNLICENSE")


MPL = re.compile(r"Mozilla Public License[\s\S]{0,40}(Version )?2\.0")
# The MPL-2.0 text itself names the GPL family as "Secondary Licenses"; that is not a GPL grant, so a mixed-case mention is ignored for an MPL file.
SOFT_DENY = ("GPL", "AGPL", "LGPL")


def classify(text):
    """-> (spdx-ish id or '', reason). '' means not acceptable."""
    mpl = bool(MPL.search(text))
    for name, rx in DENY:
        # Real GPL-family licences carry their title in capitals; a lower-case mention elsewhere only counts when the file is not an MPL one.
        flags = 0 if (mpl and name in SOFT_DENY) else re.I
        if re.search(rx, text, flags):
            return "", "matches a denied licence family: " + name
    ids = [name for name, rx in ALLOW if re.search(rx, text, re.I)]
    if not ids:
        return "", "no recognised licence text (UNLICENSED, unknown or custom terms)"
    return ids[0], ""


def modules():
    """Modules whose packages the build (tests included) links, from the Go tool itself."""
    env = dict(os.environ, GOFLAGS="-mod=mod")
    out = subprocess.run(["go", "list", "-deps", "-test", "-f", "{{with .Module}}{{if not .Main}}{{.Path}}\t{{.Version}}\t{{.Dir}}{{end}}{{end}}", "./..."],
                         cwd=API, capture_output=True, text=True, env=env)
    if out.returncode != 0:
        sys.exit("go list failed: " + out.stderr.strip())
    seen = {}
    for line in out.stdout.splitlines():
        p, v, d = (line.split("\t") + ["", ""])[:3]
        if p:
            seen[(p, v)] = d
    return sorted((p, v, d) for (p, v), d in seen.items())


def gosum():
    h = {}
    for line in open(os.path.join(API, "go.sum"), encoding="utf-8"):
        f = line.split()
        if len(f) == 3 and not f[1].endswith("/go.mod"):
            h[(f[0], f[1])] = f[2]
    return h


def license_text(d):
    if not d or not os.path.isdir(d):
        return ""
    parts = []
    for f in sorted(os.listdir(d)):
        if f.upper().split(".")[0] in LICENSE_FILES and os.path.isfile(os.path.join(d, f)):
            parts.append(open(os.path.join(d, f), encoding="utf-8", errors="replace").read())
    return "\n".join(parts)


def sbom(out):
    sums = gosum()
    comps = []
    for p, v, d in modules():
        c = {"type": "library", "bom-ref": "pkg:golang/%s@%s" % (p, v), "name": p, "version": v, "purl": "pkg:golang/%s@%s" % (p, v)}
        if (p, v) in sums:
            c["properties"] = [{"name": "go.sum:h1", "value": sums[(p, v)]}]
        lic, _ = classify(license_text(d))
        if lic:
            c["licenses"] = [{"license": {"id": lic}}]
        comps.append(c)
    doc = {"bomFormat": "CycloneDX", "specVersion": "1.5", "version": 1,
           "metadata": {"component": {"type": "application", "name": "veyrogram-api", "bom-ref": "veyrogram-api"}, "tools": [{"name": "tools/supplychain.py"}]},
           "components": comps}
    s = json.dumps(doc, indent=2, sort_keys=True) + "\n"
    if out:
        open(out, "w", encoding="utf-8").write(s)
        print("sbom: %d components -> %s (sha256 %s)" % (len(comps), out, hashlib.sha256(s.encode()).hexdigest()[:16]))
    else:
        sys.stdout.write(s)
    return 0


def waivers(today):
    if not os.path.exists(WAIVERS):
        return {}
    out = {}
    for w in (yaml.safe_load(open(WAIVERS, encoding="utf-8")) or {}).get("waivers") or []:
        miss = [k for k in ("module", "reason", "approved_by", "expires") if not w.get(k)]
        if miss:
            sys.exit("waiver for %s is missing %s: an exemption needs a reason, an approver and an expiry" % (w.get("module", "?"), miss))
        if datetime.date.fromisoformat(str(w["expires"])) < today:
            print("note: waiver for %s expired on %s and no longer applies" % (w["module"], w["expires"]))
            continue
        out[w["module"]] = w
    return out


def licenses(today=None):
    today = today or datetime.date.today()
    w = waivers(today)
    bad, rows = [], []
    for p, v, d in modules():
        lic, why = classify(license_text(d))
        if lic:
            rows.append((p, v, lic))
        elif p in w:
            rows.append((p, v, "WAIVED(%s, until %s)" % (w[p]["approved_by"], w[p]["expires"])))
        else:
            bad.append("%s %s: %s" % (p, v, why))
    for r in rows:
        print("  %-48s %-28s %s" % r)
    for b in bad:
        print("FAIL:", b)
    print("licenses: %d dependencies, %d blocked" % (len(rows) + len(bad), len(bad)))
    return 1 if bad else 0


MIRROR = re.compile(r"npmmirror|taobao|cnpmjs|registry\.npm\.taobao|mirrors\.(aliyun|tuna|ustc)|goproxy\.(cn|io)|gocenter", re.I)


def mirrors():
    bad = []
    pats = ["package.json", "package-lock.json", "pnpm-lock.yaml", "yarn.lock", ".npmrc", ".yarnrc*", "go.mod", "go.work", "Dockerfile*", "build.gradle*", "settings.gradle*", "Podfile*", "*.toml"]
    files = subprocess.run(["git", "ls-files"], cwd=ROOT, capture_output=True, text=True).stdout.split()
    for f in files:
        base = os.path.basename(f)
        if f.startswith("upstream/") or not any(glob.fnmatch.fnmatch(base, p) for p in pats):
            continue
        for i, line in enumerate(open(os.path.join(ROOT, f), encoding="utf-8", errors="replace"), 1):
            if MIRROR.search(line):
                bad.append("%s:%d: third-party mirror: %s" % (f, i, line.strip()[:100]))
    ci = open(os.path.join(ROOT, ".github/workflows/ci.yml"), encoding="utf-8").read()
    for i, line in enumerate(ci.splitlines(), 1):
        if MIRROR.search(line) and "supplychain" not in line:
            bad.append(".github/workflows/ci.yml:%d: third-party mirror" % i)
    for b in bad:
        print("FAIL:", b)
    print("mirrors: %d offending lines" % len(bad))
    return 1 if bad else 0


if __name__ == "__main__":
    cmd = sys.argv[1] if len(sys.argv) > 1 else ""
    if cmd == "sbom":
        sys.exit(sbom(sys.argv[sys.argv.index("-o") + 1] if "-o" in sys.argv else None))
    if cmd == "licenses":
        sys.exit(licenses())
    if cmd == "mirrors":
        sys.exit(mirrors())
    sys.exit(__doc__)
