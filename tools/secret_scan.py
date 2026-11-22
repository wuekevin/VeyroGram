# -*- coding: utf-8 -*-
"""Secret scanner for tracked files (Story 1.24; NFR-5). Blocks on any finding that is not in tools/secret-scan-allow.yaml.

  secret_scan.py [--files F ...]     default: every file git tracks (plus staged/untracked-but-not-ignored ones)

Rules: private-key blocks, cloud/VCS/chat tokens by their well-known shapes, JWTs, passwords inside DSNs, and quoted high-entropy values assigned to
names that say secret/password/token/key. Placeholders (`@@FILL...@@`, `@@GEN...@@`, `<...>`, `changeme`, `example`) are not findings.
An allow entry needs a path glob, the rule id, a reason and a sha256 prefix of the matched text, so it excuses exactly one known value and nothing
that is added to the same file later.
"""
import fnmatch, hashlib, math, os, re, subprocess, sys
import yaml

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
ALLOW = os.path.join(ROOT, "tools/secret-scan-allow.yaml")

RULES = [
    ("private-key", re.compile(r"-----BEGIN (?:RSA |EC |DSA |OPENSSH |PGP |ENCRYPTED )?PRIVATE KEY(?: BLOCK)?-----")),
    ("aws-access-key", re.compile(r"\b(?:AKIA|ASIA)[0-9A-Z]{16}\b")),
    ("github-token", re.compile(r"\b(?:gh[pousr]_[A-Za-z0-9]{36,}|github_pat_[A-Za-z0-9_]{50,})\b")),
    ("slack-token", re.compile(r"\bxox[baprs]-[A-Za-z0-9-]{10,}")),
    ("stripe-key", re.compile(r"\b[sr]k_live_[A-Za-z0-9]{16,}")),
    ("google-api-key", re.compile(r"\bAIza[0-9A-Za-z_-]{35}\b")),
    ("jwt", re.compile(r"\beyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}")),
    ("dsn-password", re.compile(r"\b[a-z][a-z0-9+.-]*://[^\s:/@'\"]+:([^\s@'\"/]{6,})@")),
    ("mysql-dsn-password", re.compile(r"\b[A-Za-z0-9_.-]+:([^\s@'\"/:]{6,})@(?:tcp|unix)\(")),
    ("assigned-secret", re.compile(r"(?i)(?:password|passwd|secret|token|api[_-]?key|private[_-]?key|signing[_-]?key)[A-Za-z0-9_]*\"?\s*[:=]\s*[\"']([^\"'\s]{16,})[\"']")),
]
PLACEHOLDER = re.compile(r"@@(?:FILL|GEN)|<[^>]+>|\$\{|\$\(|\{\{|changeme|change-me|example|placeholder|your[_-]|xxxx|\*\*\*|redacted|dummy", re.I)
SKIP_EXT = {".png", ".jpg", ".jpeg", ".gif", ".ico", ".woff", ".woff2", ".pdf", ".zip", ".gz", ".svg", ".lock", ".sum", ".bin"}


def entropy(s):
    return -sum(c / len(s) * math.log2(c / len(s)) for c in (s.count(x) for x in set(s))) if s else 0.0


def fingerprint(text):
    return hashlib.sha256(text.encode()).hexdigest()[:12]


def scan_text(path, text):
    out = []
    for n, line in enumerate(text.splitlines(), 1):
        if len(line) > 2000:
            continue
        for rid, rx in RULES:
            for m in rx.finditer(line):
                val = m.group(1) if m.groups() else m.group(0)
                if rid in ("dsn-password", "mysql-dsn-password", "assigned-secret"):
                    if PLACEHOLDER.search(line) or PLACEHOLDER.search(val):
                        continue
                    if rid == "assigned-secret" and entropy(val) < 3.0:
                        continue
                out.append({"path": path, "line": n, "rule": rid, "fp": fingerprint(m.group(0)), "preview": m.group(0)[:6] + "..."})
    return out


def allowed(allow):
    keyed = set()
    for a in allow:
        miss = [k for k in ("path", "rule", "fp", "reason") if not a.get(k)]
        if miss:
            sys.exit("allow entry %r is missing %s: an exception needs a path, a rule, the value's fingerprint and a reason" % (a, miss))
        keyed.add((a["path"], a["rule"], a["fp"]))
    return keyed


def tracked():
    r = subprocess.run(["git", "ls-files", "-co", "--exclude-standard"], cwd=ROOT, capture_output=True, text=True)
    return [f for f in r.stdout.split("\n") if f and not f.startswith("upstream/")]


def main(argv):
    files = argv[argv.index("--files") + 1:] if "--files" in argv else tracked()
    allow = (yaml.safe_load(open(ALLOW, encoding="utf-8")) or {}).get("allow") or [] if os.path.exists(ALLOW) else []
    ok = allowed(allow)
    findings, used = [], set()
    for f in files:
        if os.path.splitext(f)[1].lower() in SKIP_EXT:
            continue
        p = f if os.path.isabs(f) else os.path.join(ROOT, f)
        try:
            txt = open(p, encoding="utf-8").read()
        except (UnicodeDecodeError, FileNotFoundError, IsADirectoryError):
            continue
        for x in scan_text(f, txt):
            key = next(((a, x["rule"], x["fp"]) for a in {k[0] for k in ok} if fnmatch.fnmatch(f, a) and (a, x["rule"], x["fp"]) in ok), None)
            if key:
                used.add(key)
            else:
                findings.append(x)
    for x in findings:
        print("FAIL: %s:%d [%s] %s fp=%s" % (x["path"], x["line"], x["rule"], x["preview"], x["fp"]))
    for k in sorted(ok - used):
        print("note: allow entry no longer matches anything (remove it): %s %s %s" % k)
    print("secret-scan: %d files, %d findings, %d allowed" % (len(files), len(findings), len(used)))
    return 1 if findings else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
