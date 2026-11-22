#!/usr/bin/env python3
"""Dependency audit gate (Story 1.10): high/critical advisories must be zero. Usage: audit_clients.py <yarn|pnpm> <tree>.
Uses the package manager's own audit against the official registry. A waiver (tools/client-audit-waivers.json: advisory id, reason, approver,
expiry) is the only way to pass with findings; an empty/absent file means none. Needs network."""
import datetime, json, pathlib, subprocess, sys

ROOT = pathlib.Path(__file__).resolve().parent.parent


def waivers() -> dict:
    p = ROOT / "tools" / "client-audit-waivers.json"
    out = {}
    if not p.exists():
        return out
    for w in json.loads(p.read_text()).get("waivers") or []:
        for k in ("id", "reason", "approver", "expires"):
            if not w.get(k):
                sys.exit(f"waiver missing {k}: {w}")
        if datetime.date.fromisoformat(str(w["expires"])) < datetime.date.today():
            sys.exit(f"waiver {w['id']} expired {w['expires']}")
        out[str(w["id"])] = w
    return out


def main() -> int:
    kind, tree = sys.argv[1], sys.argv[2]
    cmd = ["npx", "--yes", "yarn@1.22.22", "audit", "--json"] if kind == "yarn" else ["pnpm", "audit", "--json"]
    r = subprocess.run(cmd, cwd=tree, capture_output=True, text=True)
    found = {}
    if kind == "yarn":
        for line in r.stdout.splitlines():
            try:
                j = json.loads(line)
            except ValueError:
                continue
            if j.get("type") == "auditAdvisory":
                a = j["data"]["advisory"]
                if a["severity"] in ("high", "critical"):
                    found[str(a["id"])] = f'{a["module_name"]} {a["severity"]} {a["title"][:60]}'
    else:
        try:
            d = json.loads(r.stdout)
        except ValueError:
            print("audit produced no JSON (network/registry problem):", r.stderr[-300:])
            return 2
        for k, a in d.get("advisories", {}).items():
            if a["severity"] in ("high", "critical"):
                found[str(a["id"])] = f'{a["module_name"]} {a["severity"]} {a["title"][:60]}'
    if not r.stdout.strip():
        print("audit produced no output (network/registry problem):", r.stderr[-300:])
        return 2
    w = waivers()
    open_ = {k: v for k, v in found.items() if k not in w}
    for k, v in open_.items():
        print("FAIL high/critical advisory", k, v)
    print(f"audit {kind}: {len(found)} high/critical, {len(found) - len(open_)} waived, {len(open_)} open")
    return 1 if open_ else 0


if __name__ == "__main__":
    sys.exit(main())
