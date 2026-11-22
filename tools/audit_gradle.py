#!/usr/bin/env python3
"""Gradle dependency audit gate (Story 1.9): high/critical advisories must be zero.
Usage: audit_gradle.py <gradle-dependencies-output.txt> [--offline-fixture <osv.json>]
Reads the resolved coordinates from `gradlew :app:dependencies --configuration debugRuntimeClasspath`,
asks OSV (api.osv.dev; only public Maven coordinates are sent) and reads each advisory's severity.
Unknown severity counts as high (fail safe). Waivers: tools/client-audit-waivers.json (same file as the npm gate)."""
import json, pathlib, re, sys, urllib.request

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))
from audit_clients import waivers  # noqa: E402

OSV = "https://api.osv.dev/v1"
COORD = re.compile(r"([A-Za-z0-9_.\-]+):([A-Za-z0-9_.\-]+):([^\s]+)(?: -> ([^\s(]+))?")


def coords(text: str) -> list:
    out = set()
    for line in text.splitlines():
        if "+---" not in line and "\\---" not in line:
            continue
        m = COORD.search(line)
        if not m:
            continue
        v = m.group(4) or m.group(3)
        out.add((m.group(1), m.group(2), v.split(":")[-1]))
    return sorted(out)


def post(path: str, body: dict) -> dict:
    req = urllib.request.Request(OSV + path, json.dumps(body).encode(), {"Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=60) as r:
        return json.load(r)


def severity(vuln: dict) -> str:
    s = (vuln.get("database_specific") or {}).get("severity")
    if s:
        return s.lower()
    return "unknown"


def main() -> int:
    deps = coords(pathlib.Path(sys.argv[1]).read_text())
    if len(deps) < 20:
        print(f"only {len(deps)} coordinates parsed - wrong or empty dependency output")
        return 2
    ids = {}
    for i in range(0, len(deps), 100):
        chunk = deps[i:i + 100]
        res = post("/querybatch", {"queries": [{"package": {"ecosystem": "Maven", "name": f"{g}:{a}"}, "version": v} for g, a, v in chunk]})
        for (g, a, v), r in zip(chunk, res["results"]):
            for vu in r.get("vulns", []):
                ids.setdefault(vu["id"], f"{g}:{a}:{v}")
    w = waivers()
    bad = {}
    for vid, dep in ids.items():
        with urllib.request.urlopen(f"{OSV}/vulns/{vid}", timeout=60) as r:
            sev = severity(json.load(r))
        if sev in ("high", "critical", "unknown"):
            bad[vid] = (dep, sev)
    open_ = {k: v for k, v in bad.items() if k not in w}
    for k, (dep, sev) in sorted(open_.items()):
        print("FAIL", sev, k, dep)
    print(f"audit gradle: {len(deps)} artifacts, {len(ids)} advisories, {len(bad)} high/critical/unknown, {len(bad) - len(open_)} waived, {len(open_)} open")
    return 1 if open_ else 0


if __name__ == "__main__":
    sys.exit(main())
