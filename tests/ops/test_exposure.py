#!/usr/bin/env python3
"""Story 1.7 tests: config check, exposure scanner, and (when VG_TEST_WUKONGIM_BIN is set) a real WuKongIM behind both.
Negative-by-design: each control (bad config, deliberately exposed listener) MUST be flagged, otherwise the gate proves nothing."""
import os, socket, subprocess, sys, tempfile, time, unittest, urllib.request
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..', '..', 'tools'))
import wukongim_config_check as cc, render_wukongim as rw, exposure_scan as es

ROOT = os.path.join(os.path.dirname(__file__), '..', '..')
TEMPLATE = open(os.path.join(ROOT, 'infrastructure/wukongim/wukongim.prod.toml.template')).read()
EXAMPLE = os.path.join(ROOT, 'upstream/WuKongIM/wukongim.toml.example')
BIN = os.environ.get('VG_TEST_WUKONGIM_BIN')

def free():
    s = socket.socket(); s.bind(('127.0.0.1', 0)); p = s.getsockname()[1]; s.close(); return p

def env(data, ports):
    return {'WK_DATA_DIR': data, 'WK_CLUSTER_ADDR': f'127.0.0.1:{ports["cluster"]}', 'WK_API_ADDR': f'127.0.0.1:{ports["api"]}',
            'WK_MANAGER_ADDR': f'127.0.0.1:{ports["manager"]}', 'WK_EXTERNAL_TCP': f'127.0.0.1:{ports["tcp"]}',
            'WK_EXTERNAL_WS': f'ws://127.0.0.1:{ports["ws"]}', 'WK_GATEWAY_TCP': f'127.0.0.1:{ports["tcp"]}',
            'WK_GATEWAY_WS': f'127.0.0.1:{ports["ws"]}', 'WK_JOIN_TOKEN': 'j' * 40, 'WK_MANAGER_JWT_SECRET': 'k' * 40,
            'WK_MANAGER_USER': 'vgops', 'WK_MANAGER_PASSWORD': 'p' * 24, 'WK_BENCH_TOKEN': 'b' * 40,
            'WK_BEFORE_SEND_URL': 'http://127.0.0.1:9102/hook'}

def lan_ip():
    s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    try:
        s.connect(('10.255.255.255', 1)); ip = s.getsockname()[0]
    except OSError:
        return None
    finally:
        s.close()
    return None if ip.startswith('127.') else ip

class ConfigCheck(unittest.TestCase):
    def good(self, **over):
        e = env('/var/lib/wukongim', {k: 1000 + i for i, k in enumerate(('cluster', 'api', 'manager', 'tcp', 'ws'))}); e.update(over)
        return rw.render(TEMPLATE, e)
    def test_template_passes(self): self.assertEqual(cc.check(self.good()), [])
    def test_upstream_defaults_fail_on_every_required_item(self):
        f = ' | '.join(cc.check(open(EXAMPLE).read()))
        for needle in ('cluster.join_token', 'manager.jwt_secret', 'bench.api_token', 'plugin.enable', 'plugin.hot_reload', 'mqtt.listen_addr', 'manager user'):
            self.assertIn(needle, f)
    def test_each_mutation_is_caught(self):
        base = self.good()
        muts = {
            'default join token': ('join_token = "' + 'j' * 40 + '"', 'join_token = "change-me"'),
            'default jwt': ('jwt_secret = "' + 'k' * 40 + '"', 'jwt_secret = "wukongim-dev-manager-secret"'),
            'empty bench token': ('api_token = "' + 'b' * 40 + '"', 'api_token = ""'),
            'plugin on': ('[plugin]\nenable = false', '[plugin]\nenable = true'),
            'hot reload': ('hot_reload = false', 'hot_reload = true'),
            'mqtt on': ('[mqtt]\nenable = false', '[mqtt]\nenable = true'),
            'api public': ('listen_addr = "127.0.0.1:1001"', 'listen_addr = "0.0.0.0:1001"'),
            'guest user': ('username = "vgops"', 'username = "guest"'),
            'manager auth off': ('auth_on = true', 'auth_on = false'),
            'debug api': ('debug_api_enable = false', 'debug_api_enable = true'),
            'before_send fail open': ('on_error = "deny"', 'on_error = "allow"'),
            'before_send public url': ('http://127.0.0.1:9102/hook', 'http://8.8.8.8/hook'),
            'before_send doc-range url': ('http://127.0.0.1:9102/hook', 'http://203.0.113.9/hook'),
            'api on public ip': ('listen_addr = "127.0.0.1:1001"', 'listen_addr = "8.8.8.8:1001"'),
        }
        for name, (a, b) in muts.items():
            with self.subTest(name):
                self.assertIn(a, base, 'mutation anchor missing: test is stale')
                self.assertTrue(cc.check(base.replace(a, b, 1)), f'{name} was NOT detected')
    def test_render_rejects_unset_and_escapes(self):
        e = env('/d', {k: 1 for k in ('cluster', 'api', 'manager', 'tcp', 'ws')}); del e['WK_JOIN_TOKEN']
        with self.assertRaises(SystemExit): rw.render(TEMPLATE, e)
        e['WK_JOIN_TOKEN'] = 'x"y\\z'
        self.assertIn(r'x\"y\\z', rw.render(TEMPLATE, e))

class Scanner(unittest.TestCase):
    def test_flags_exposed_listener_and_passes_closed_or_loopback_only(self):
        ip = lan_ip()
        if not ip: self.skipTest('no non-loopback interface')
        pub, loop = socket.socket(), socket.socket()
        pub.bind(('0.0.0.0', 0)); pub.listen(); loop.bind(('127.0.0.1', 0)); loop.listen()
        pp, lp = pub.getsockname()[1], loop.getsockname()[1]
        try:
            self.assertEqual(es.scan([ip], [pp]), [(ip, pp)], 'wildcard-bound listener must be flagged')
            self.assertEqual(es.scan([ip], [lp]), [], 'loopback-only listener must not be reachable via LAN address')
            self.assertEqual(es.scan([ip], [free()]), [], 'closed port is unreachable')
            self.assertEqual(es.main(['--target', ip, '--ports', str(pp)]), 1)
            self.assertEqual(es.main(['--target', ip, '--ports', f'{lp}']), 0)
        finally:
            pub.close(); loop.close()

@unittest.skipUnless(BIN and os.path.exists(BIN), 'VG_TEST_WUKONGIM_BIN unset: real WuKongIM exposure test SKIPPED')
class RealWuKongIM(unittest.TestCase):
    def run_im(self, **over):
        """Render the production template (with overrides), start the real binary, return (ports, cfg_text, stop)."""
        ports = {k: free() for k in ('cluster', 'api', 'manager', 'tcp', 'ws')}
        d = tempfile.mkdtemp(); cfg = os.path.join(d, 'wukongim.toml')
        e = {**os.environ, **env(os.path.join(d, 'data'), ports)}
        e.update({k: v.replace('{api}', str(ports['api'])) for k, v in over.items()})
        subprocess.run([sys.executable, '-I', os.path.join(ROOT, 'tools/render_wukongim.py'), os.path.join(ROOT, 'infrastructure/wukongim/wukongim.prod.toml.template'), cfg], check=True, env=e)
        self.assertEqual(oct(os.stat(cfg).st_mode & 0o777), '0o600')
        text = open(cfg).read()
        log = open(os.path.join(d, 'wk.log'), 'w')
        p = subprocess.Popen([BIN, '-config', cfg], stdout=log, stderr=subprocess.STDOUT, cwd=d)
        def stop():
            p.terminate(); p.wait(10); log.close()
        self.addCleanup(stop)
        deadline = time.time() + 40
        while time.time() < deadline:
            try:
                urllib.request.urlopen(f'http://127.0.0.1:{ports["api"]}/healthz', timeout=1); return ports, text
            except Exception:
                if p.poll() is not None: self.fail('WuKongIM exited: ' + open(log.name).read()[-1500:])
                time.sleep(0.5)
        self.fail('WuKongIM never became healthy: ' + open(log.name).read()[-1500:])

    def test_hardened_instance_is_not_reachable_off_loopback(self):
        ip = lan_ip()
        if not ip: self.skipTest('no non-loopback interface')
        ports, text = self.run_im()
        self.assertEqual(cc.check(text), [])
        internal = [ports['api'], ports['manager'], ports['cluster'], 1883, 9099, 5001, 5301, 7001, 6060]
        self.assertEqual(es.scan([ip], internal), [], 'internal ports reachable via non-loopback address')
        want = sorted([ports['api'], ports['manager'], ports['cluster']])
        self.assertEqual(sorted(p for _, p in es.scan(['127.0.0.1'], want)), want, 'control: scanner must see the instance on loopback')
        self.assertEqual(es.scan(['127.0.0.1'], [1883, 9099]), [], 'MQTT/Prometheus must be off')

    def test_misconfigured_api_bind_is_caught_by_both_gates(self):
        ip = lan_ip()
        if not ip: self.skipTest('no non-loopback interface')
        # Negative: API deliberately bound on all interfaces. Config check AND live scan must each fail, otherwise the gate is worthless.
        ports, text = self.run_im(WK_API_ADDR='0.0.0.0:{api}')
        self.assertTrue(any('api.listen_addr' in f for f in cc.check(text)), 'config check missed the public API bind')
        self.assertEqual(es.scan([ip], [ports['api']]), [(ip, ports['api'])], 'live scan missed the exposed API port')

if __name__ == '__main__':
    unittest.main(verbosity=2)
