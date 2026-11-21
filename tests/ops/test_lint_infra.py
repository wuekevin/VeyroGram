import os, sys, unittest
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..', '..', 'tools'))
import lint_infra as L
ROOT = os.path.join(os.path.dirname(__file__), '..', '..')
UNIT = open(os.path.join(ROOT, 'infrastructure/systemd/veyrogram-api.service'), encoding='utf8').read()
NGX = open(os.path.join(ROOT, 'infrastructure/nginx/veyrogram-api.conf.template'), encoding='utf8').read()

class Systemd(unittest.TestCase):
    def test_repo_unit_is_clean(self): self.assertEqual(L.lint_systemd(UNIT), [])
    def test_root_user_rejected(self): self.assertTrue(L.lint_systemd(UNIT.replace('User=veyrogram', 'User=root')))
    def test_missing_user_rejected(self): self.assertTrue(L.lint_systemd(UNIT.replace('User=veyrogram\n', '')))
    def test_no_new_privs_required(self): self.assertTrue(L.lint_systemd(UNIT.replace('NoNewPrivileges=yes', 'NoNewPrivileges=no')))
    def test_protect_system_required(self): self.assertTrue(L.lint_systemd(UNIT.replace('ProtectSystem=strict', 'ProtectSystem=no')))
    def test_inline_secret_rejected(self): self.assertTrue(L.lint_systemd(UNIT + '\nEnvironment=VG_TOKEN_SIGNING_KEY=abcdef\n'))
    def test_capabilities_must_be_dropped(self): self.assertTrue(L.lint_systemd(UNIT.replace('CapabilityBoundingSet=', 'CapabilityBoundingSet=CAP_NET_BIND_SERVICE')))
    def test_canary_unit_may_use_the_canary_symlink_but_nothing_else(self):
        canary = UNIT.replace('/opt/veyrogram/api/current/veyrogram-api', '/opt/veyrogram/api/canary/veyrogram-api')
        self.assertEqual([], L.lint_systemd(canary, 'veyrogram-api-canary.service'))
        self.assertTrue(L.lint_systemd(UNIT.replace('/opt/veyrogram/api/current/veyrogram-api', '/usr/local/bin/api'), 'veyrogram-api-canary.service'))
        self.assertTrue(L.lint_systemd(canary, 'veyrogram-api.service'))  # the live unit must not run the canary link

    def test_execstart_must_use_current_symlink(self): self.assertTrue(L.lint_systemd(UNIT.replace('/opt/veyrogram/api/current/veyrogram-api', '/usr/local/bin/api')))

class Nginx(unittest.TestCase):
    def test_repo_template_is_clean(self): self.assertEqual(L.lint_nginx(NGX), [])
    def test_wukongim_api_proxy_rejected(self):
        bad = NGX.replace('location / {', 'location ^~ /v1/im/ { proxy_pass http://127.0.0.1:5001; }\n    location / {')
        self.assertTrue(any('5001' in f for f in L.lint_nginx(bad)))
    def test_health_listener_proxy_rejected(self):
        bad = NGX.replace('location / {', 'location = /v1/healthz { proxy_pass http://127.0.0.1:9101; }\n    location / {')
        self.assertTrue(any('9101' in f for f in L.lint_nginx(bad)))
    def test_non_whitelisted_prefix_rejected(self):
        bad = NGX.replace('location / {', 'location ^~ /admin/ { proxy_pass http://veyrogram_api; }\n    location / {')
        self.assertTrue(any('not a whitelisted' in f for f in L.lint_nginx(bad)))
    def test_regex_location_proxy_rejected(self):
        bad = NGX.replace('location / {', 'location ~ ^/v1/.* { proxy_pass http://veyrogram_api; }\n    location / {')
        self.assertTrue(any('regex' in f for f in L.lint_nginx(bad)))
    def test_catch_all_404_required(self): self.assertTrue(any('catch-all' in f for f in L.lint_nginx(NGX.replace('return 404;', 'return 200;'))))
    def test_old_tls_rejected(self): self.assertTrue(any('ssl_protocols' in f for f in L.lint_nginx(NGX.replace('TLSv1.2 TLSv1.3', 'TLSv1 TLSv1.1 TLSv1.2'))))
    def test_xff_append_rejected(self): self.assertTrue(any('X-Forwarded-For' in f for f in L.lint_nginx(NGX.replace('$remote_addr;\n        proxy_set_header X-Forwarded-Proto', '$proxy_add_x_forwarded_for;\n        proxy_set_header X-Forwarded-Proto'))))
    def test_server_tokens_required(self): self.assertTrue(any('server_tokens' in f for f in L.lint_nginx(NGX.replace('server_tokens off;', ''))))

class Repo(unittest.TestCase):
    def test_repo_passes_cli(self): self.assertEqual(L.main(os.path.abspath(ROOT)), 0)

if __name__ == '__main__': unittest.main()
