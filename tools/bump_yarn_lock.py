#!/usr/bin/env python3
"""Maintenance helper (Story 1.10): bump vulnerable yarn.lock entries to the newest same-major release that every requesting range still accepts.
Usage: bump_yarn_lock.py <tree> [min-severity: moderate|high]. Edits <tree>/yarn.lock in place (versions/tarballs/integrity come from the official
registry via `npm view`); run `yarn install --ignore-scripts` afterwards. Entries that need a range or major change are printed as STUCK."""
import json, pathlib, re, subprocess, sys

tree = pathlib.Path(sys.argv[1]); floor = sys.argv[2] if len(sys.argv) > 2 else "high"
order = ["info", "low", "moderate", "high", "critical"]
audit = subprocess.run(["npx", "--yes", "yarn@1.22.22", "audit", "--json"], cwd=tree, capture_output=True, text=True).stdout
bad = set()
for line in audit.splitlines():
    try:
        j = json.loads(line)
    except ValueError:
        continue
    if j.get("type") == "auditAdvisory" and order.index(j["data"]["advisory"]["severity"]) >= order.index(floor):
        for f in j["data"]["advisory"]["findings"]:
            bad.add((j["data"]["advisory"]["module_name"], f["version"]))
NODE = r'''
const semver=require(process.argv[1]);const [,sp,name,pats,cur]=process.argv;const {execSync}=require('child_process');
const all=JSON.parse(execSync(`npm view ${name} versions --json`,{encoding:'utf8'})).filter(v=>/^\d+\.\d+\.\d+$/.test(v));
const ps=JSON.parse(pats).map(p=>p.slice(name.length+1));
const ok=all.filter(v=>ps.every(r=>semver.satisfies(v,r))&&semver.gt(v,cur)).sort(semver.compare);
if(!ok.length){console.log('');process.exit(0)}
const v=ok[ok.length-1];const d=JSON.parse(execSync(`npm view ${name}@${v} dist --json`,{encoding:'utf8'}));
console.log(JSON.stringify({v,tarball:d.tarball,integrity:d.integrity}));'''
sem = subprocess.run(["npm", "root", "-g"], capture_output=True, text=True).stdout.strip() + "/npm/node_modules/semver"
lock = (tree / "yarn.lock").read_text()
head, *blocks = re.split(r"\n\n", lock)
out, changed = [head], 0
for b in blocks:
    m = re.match(r'^(.+?):\n  version "([^"]+)"\n  resolved "[^"]+"\n  integrity (\S+)', b)
    if m:
        pats = [p.strip().strip('"') for p in m.group(1).split(",")]
        name = re.match(r"^(@?[^@]+)@", pats[0]).group(1)
        if (name, m.group(2)) in bad:
            r = subprocess.run(["node", "-e", NODE, sem, name, json.dumps(pats), m.group(2)], capture_output=True, text=True).stdout.strip()
            if r:
                d = json.loads(r)
                b = re.sub(r'version "[^"]+"\n  resolved "[^"]+"\n  integrity \S+', f'version "{d["v"]}"\n  resolved "{d["tarball"]}"\n  integrity {d["integrity"]}', b, count=1)
                changed += 1
            else:
                print("STUCK", name, m.group(2), pats)
    out.append(b)
(tree / "yarn.lock").write_text("\n\n".join(out))
print("bumped", changed)
