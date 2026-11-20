#!/usr/bin/env python3
"""Deployment validation for a RENDERED WuKongIM config (Story 1.7, AD-2, R-21). Exit 1 on any finding.
Checks: default/weak secrets (cluster.join_token, manager.jwt_secret, manager users, bench.api_token), plugin + hot reload off,
MQTT off, debug/bench/diagnostics surfaces off, auth on, and API/Manager/cluster/Prometheus/MQTT addresses not publicly bindable.
Parser handles the single-line `key = value` TOML subset used by wukongim.toml.example (not full TOML).
Usage: python3 -I tools/wukongim_config_check.py <wukongim.toml>"""
import ipaddress, re, sys

KNOWN_BAD = {'change-me', 'changeme', 'wukongim-dev-manager-secret', 'a1234567', 'admin', 'guest', 'password', ''}
MIN_SECRET = 32

def parse(text):
    sec, d = '', {}
    for raw in text.splitlines():
        line = raw.strip()
        if not line or line.startswith('#'): continue
        m = re.match(r'^\[([^\]]+)\]$', line)
        if m: sec = m.group(1); continue
        m = re.match(r'^([A-Za-z0-9_.]+)\s*=\s*(.*)$', line)
        if m: d[(sec, m.group(1))] = m.group(2).strip()
    return d

def s(v):  # string scalar
    m = re.match(r'^"((?:[^"\\]|\\.)*)"', v or '')
    return m.group(1) if m else None

def b(v): return {'true': True, 'false': False}.get((v or '').lower())

def private_host(addr):
    host = addr.rsplit(':', 1)[0].strip('[]')
    if host in ('localhost',): return True
    try:
        ip = ipaddress.ip_address(host)
    except ValueError:
        return False
    # Explicit ranges: ipaddress.is_private also covers documentation/reserved blocks (e.g. 203.0.113.0/24), which are not internal.
    return ip.is_loopback or any(ip in ipaddress.ip_network(n) for n in ('10.0.0.0/8', '172.16.0.0/12', '192.168.0.0/16', 'fc00::/7'))

def check(text):
    d, out = parse(text), []
    def get(sec, key): return d.get((sec, key))
    def weak(label, v, need_len=True):
        if v is None or v.lower() in KNOWN_BAD or (need_len and len(v) < MIN_SECRET):
            out.append(f'{label} is missing, a known default, or shorter than {MIN_SECRET} chars')
    weak('cluster.join_token', s(get('cluster', 'join_token')))
    weak('manager.jwt_secret', s(get('manager', 'jwt_secret')))
    weak('bench.api_token', s(get('bench', 'api_token')))
    if b(get('manager', 'auth_on')) is not True: out.append('manager.auth_on must be true')
    users = get('manager', 'users') or ''
    names = re.findall(r'username\s*=\s*"([^"]*)"', users); pws = re.findall(r'password\s*=\s*"([^"]*)"', users)
    if not names: out.append('manager.users must define at least one non-default user')
    for n in names:
        if n.lower() in ('guest', 'admin'): out.append(f'manager user "{n}" is a default/publicly-displayed name')
    for p in pws:
        if p.lower() in KNOWN_BAD or len(p) < 16: out.append('manager user password is a known default or shorter than 16 chars')
    if b(get('plugin', 'enable')) is not False: out.append('plugin.enable must be false')
    if b(get('plugin', 'hot_reload')) is True: out.append('plugin.hot_reload must not be true')
    if b(get('mqtt', 'enable')) is not False: out.append('mqtt.enable must be false')
    if b(get('observability', 'debug_api_enable')) is True: out.append('observability.debug_api_enable must be false')
    if b(get('bench', 'api_enable')) is True: out.append('bench.api_enable must be false')
    if b(get('diagnostics', 'enable')) is True: out.append('diagnostics.enable must be false in production')
    if b(get('prometheus', 'enable')) is True and not private_host(s(get('prometheus', 'listen_addr')) or ''): out.append('prometheus.listen_addr must be private')
    for sec, key in (('api', 'listen_addr'), ('manager', 'listen_addr'), ('cluster', 'listen_addr'), ('mqtt', 'listen_addr')):
        a = s(get(sec, key))
        if a is None: out.append(f'{sec}.{key} must be set explicitly')
        elif not private_host(a): out.append(f'{sec}.{key}={a} must bind loopback/private only')
    for n in re.findall(r'addr\s*=\s*"([^"]+)"', get('cluster', 'nodes') or ''):
        if not private_host(n): out.append(f'cluster.nodes address {n} must be private')
    bs = d.get(('webhook.before_send', 'enabled'))
    if b(bs) is True:
        if s(d.get(('webhook.before_send', 'on_timeout'))) != 'deny' or s(d.get(('webhook.before_send', 'on_error'))) != 'deny':
            out.append('webhook.before_send must fail closed (on_timeout/on_error = "deny")')
        u = s(d.get(('webhook.before_send', 'http_addr'))) or ''
        m = re.match(r'^https?://([^/:]+)', u)
        if not m or not private_host(m.group(1) + ':0'): out.append('webhook.before_send.http_addr must point at a loopback/private host')
    return out

if __name__ == '__main__':
    if len(sys.argv) != 2: raise SystemExit(__doc__)
    errs = check(open(sys.argv[1]).read())
    for e in errs: print('FAIL:', e)
    print('wukongim config check:', 'FAILED' if errs else 'ok'); sys.exit(1 if errs else 0)
