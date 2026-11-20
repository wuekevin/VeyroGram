#!/usr/bin/env python3
"""Static policy checks for infrastructure/ (Story 1.2, AD-2/AD-14/NFR-16).
 - systemd units: dedicated non-root account, sandboxing, no secrets inline.
 - nginx templates: only whitelisted prefixes are proxied, never WuKongIM/health/DB ports, catch-all is 404.
Usage: python3 -I tools/lint_infra.py [repo-root]   exit 1 on any finding."""
import os, re, sys

INTERNAL_PORTS = {'5001', '5100', '5200', '5300', '5301', '7001', '9099', '9101', '6379', '3306', '1884'}
PROXY_WHITELIST = ('/v1/', '/open/v1/')

def lint_systemd(text, name='unit'):
    out = []
    kv = {}
    for line in text.splitlines():
        line = line.strip()
        if not line or line.startswith(('#', ';', '[')) or '=' not in line: continue
        k, v = line.split('=', 1); kv.setdefault(k.strip(), []).append(v.strip())
    g = lambda k: kv.get(k, [''])[-1]
    if not g('User') or g('User') in ('root', '0'): out.append(f'{name}: User must be a dedicated non-root account')
    if not g('Group') or g('Group') in ('root', '0'): out.append(f'{name}: Group must be a dedicated non-root group')
    for k, ok in (('NoNewPrivileges', {'yes', 'true'}), ('PrivateTmp', {'yes', 'true'}), ('ProtectHome', {'yes', 'true', 'read-only'}),
                  ('ProtectSystem', {'strict', 'full'}), ('LockPersonality', {'yes', 'true'}), ('RestrictSUIDSGID', {'yes', 'true'})):
        if g(k).lower() not in ok: out.append(f'{name}: {k} must be one of {sorted(ok)}')
    if not g('EnvironmentFile'): out.append(f'{name}: secrets must come from EnvironmentFile')
    if re.search(r'/opt/veyrogram/[^ ]+/current/', g('ExecStart')) is None: out.append(f'{name}: ExecStart must run from the versioned .../current/ symlink')
    if g('CapabilityBoundingSet') != '': out.append(f'{name}: CapabilityBoundingSet must be empty (drop all capabilities)')
    for e in kv.get('Environment', []):
        if re.search(r'(PASSWORD|SECRET|TOKEN|KEY)\w*=\S+', e, re.I): out.append(f'{name}: secret-looking value in Environment= (use EnvironmentFile)')
    return out

def lint_nginx(text, name='nginx'):
    out = []
    body = re.sub(r'#.*', '', text)
    for m in re.finditer(r'proxy_pass\s+([^;]+);', body):
        target = m.group(1)
        for p in re.findall(r':(\d{2,5})\b', target):
            if p in INTERNAL_PORTS: out.append(f'{name}: proxy_pass to internal port {p} is forbidden')
    for loc in re.finditer(r'location\s+(?:(=|\^~|~\*?)\s+)?(\S+)\s*\{([^{}]*)\}', body):
        mod, path, blk = loc.groups()
        if 'proxy_pass' in blk:
            if not any(path == w or path.startswith(w) for w in PROXY_WHITELIST): out.append(f'{name}: location {path} proxies but is not a whitelisted prefix {PROXY_WHITELIST}')
            if mod == '~' or mod == '~*': out.append(f'{name}: regex location {path} must not proxy (use exact/prefix whitelist)')
    if not re.search(r'location\s+/\s*\{\s*return\s+404;\s*\}', body): out.append(f'{name}: catch-all `location / {{ return 404; }}` is required')
    if re.search(r'server_tokens\s+off;', body) is None: out.append(f'{name}: server_tokens off is required')
    m = re.search(r'ssl_protocols\s+([^;]+);', body)
    if not m or re.search(r'TLSv1(\.1)?\b(?!\.)|SSLv', m.group(1)) and not re.fullmatch(r'(\s*TLSv1\.[23])+\s*', m.group(1)): out.append(f'{name}: ssl_protocols must be TLSv1.2/1.3 only')
    if re.search(r'autoindex\s+on', body): out.append(f'{name}: autoindex on is forbidden')
    if re.search(r'proxy_set_header\s+X-Forwarded-For\s+\$proxy_add_x_forwarded_for', body): out.append(f'{name}: at the edge X-Forwarded-For must be overwritten with $remote_addr, not appended')
    return out

def main(root):
    findings = []
    for dirpath, _, files in os.walk(os.path.join(root, 'infrastructure')):
        for f in files:
            p = os.path.join(dirpath, f); rel = os.path.relpath(p, root)
            if f.endswith('.service'): findings += lint_systemd(open(p, encoding='utf8').read(), rel)
            elif f.endswith('.template') and 'nginx' in dirpath: findings += lint_nginx(open(p, encoding='utf8').read(), rel)
    for f in findings: print('FINDING:', f)
    print(f'{len(findings)} finding(s)'); return 1 if findings else 0

if __name__ == '__main__': sys.exit(main(sys.argv[1] if len(sys.argv) > 1 else os.path.join(os.path.dirname(os.path.abspath(__file__)), '..')))
