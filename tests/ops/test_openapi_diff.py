import copy, json, os, subprocess, sys, tempfile, unittest
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..', '..', 'tools'))
import openapi_diff as D

BASE = {'openapi': '3.1.0', 'info': {'title': 't', 'version': '1.2.0'}, 'paths': {
    '/v1/users': {'get': {'parameters': [{'name': 'cursor', 'in': 'query', 'required': False}],
                          'responses': {'200': {'content': {'application/json': {'schema': {'properties': {'items': {}, 'next_cursor': {}}}}}}}},
                  'post': {'requestBody': {'required': True, 'content': {'application/json': {'schema': {'required': ['phone'], 'properties': {'phone': {}, 'nick': {}}}}}},
                           'responses': {'201': {}}}},
    '/v1/devices': {'get': {'responses': {'200': {}}}}}}

def mut(f):
    d = copy.deepcopy(BASE); f(d); return d

class Breaking(unittest.TestCase):
    def test_identical_is_clean(self): self.assertEqual(D.breaking(BASE, copy.deepcopy(BASE)), [])
    def test_added_path_and_optional_param_are_fine(self):
        def f(d):
            d['paths']['/v1/new'] = {'get': {'responses': {'200': {}}}}
            d['paths']['/v1/users']['get']['parameters'].append({'name': 'q', 'in': 'query', 'required': False})
        self.assertEqual(D.breaking(BASE, mut(f)), [])
    def test_removed_path(self): self.assertTrue(any('removed path /v1/devices' in x for x in D.breaking(BASE, mut(lambda d: d['paths'].pop('/v1/devices')))))
    def test_removed_method(self): self.assertTrue(any('removed POST' in x for x in D.breaking(BASE, mut(lambda d: d['paths']['/v1/users'].pop('post')))))
    def test_removed_2xx(self): self.assertTrue(any('removed 201' in x for x in D.breaking(BASE, mut(lambda d: d['paths']['/v1/users']['post'].update(responses={'202': {}})))))
    def test_new_required_param(self): self.assertTrue(any('newly required' in x for x in D.breaking(BASE, mut(lambda d: d['paths']['/v1/users']['get']['parameters'].append({'name': 'tenant', 'in': 'header', 'required': True})))))
    def test_param_made_required(self): self.assertTrue(any('cursor' in x for x in D.breaking(BASE, mut(lambda d: d['paths']['/v1/users']['get']['parameters'][0].update(required=True)))))
    def test_new_required_body_property(self):
        def f(d): d['paths']['/v1/users']['post']['requestBody']['content']['application/json']['schema']['required'].append('nick')
        self.assertTrue(any('request property nick' in x for x in D.breaking(BASE, mut(f))))
    def test_response_property_removed(self):
        def f(d): d['paths']['/v1/users']['get']['responses']['200']['content']['application/json']['schema']['properties'].pop('next_cursor')
        self.assertTrue(any('lost property next_cursor' in x for x in D.breaking(BASE, mut(f))))
    def test_ref_resolution(self):
        base = {'paths': {'/a': {'get': {'responses': {'200': {'$ref': '#/components/responses/R'}}}}}, 'components': {'responses': {'R': {'content': {'application/json': {'schema': {'properties': {'x': {}}}}}}}}}
        head = copy.deepcopy(base); head['components']['responses']['R']['content']['application/json']['schema']['properties'] = {}
        self.assertTrue(any('lost property x' in x for x in D.breaking(base, head)))

class Gate(unittest.TestCase):
    def run_files(self, head, *extra):
        with tempfile.TemporaryDirectory() as t:
            b, h = os.path.join(t, 'b.json'), os.path.join(t, 'h.json')
            json.dump(BASE, open(b, 'w')); json.dump(head, open(h, 'w'))
            return subprocess.run([sys.executable, '-I', D.__file__, '--base-file', b, '--head-file', h, *extra], capture_output=True, text=True)
    def test_breaking_without_major_or_adr_fails(self): self.assertEqual(self.run_files(mut(lambda d: d['paths'].pop('/v1/devices'))).returncode, 1)
    def test_major_bump_allows(self):
        def f(d): d['paths'].pop('/v1/devices'); d['info']['version'] = '2.0.0'
        self.assertEqual(self.run_files(mut(f)).returncode, 0)
    def test_minor_bump_does_not_allow(self):
        def f(d): d['paths'].pop('/v1/devices'); d['info']['version'] = '1.3.0'
        self.assertEqual(self.run_files(mut(f)).returncode, 1)
    def test_adr_allows_only_if_file_exists(self):
        head = mut(lambda d: d['paths'].pop('/v1/devices'))
        self.assertEqual(self.run_files(head, '--adr', '/nonexistent/adr.md').returncode, 1)
        with tempfile.NamedTemporaryFile(suffix='.md') as adr: self.assertEqual(self.run_files(head, '--adr', adr.name).returncode, 0)

class GitMode(unittest.TestCase):
    def test_no_specs_and_unknown_ref_pass(self):
        with tempfile.TemporaryDirectory() as t:
            subprocess.run(['git', 'init', '-q', t]); subprocess.run(['git', '-C', t, 'commit', '-q', '--allow-empty', '-m', 'i', '-c', 'user.name=t', '-c', 'user.email=t@t'], capture_output=True)
            r = subprocess.run([sys.executable, '-I', D.__file__, '--dir', 'specs', '--base-ref', 'origin/main'], cwd=t, capture_output=True, text=True)
            self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
    def test_removed_spec_file_is_breaking(self):
        with tempfile.TemporaryDirectory() as t:
            env = dict(os.environ, GIT_AUTHOR_NAME='t', GIT_AUTHOR_EMAIL='t@t', GIT_COMMITTER_NAME='t', GIT_COMMITTER_EMAIL='t@t')
            g = lambda *a: subprocess.run(['git', '-C', t, *a], capture_output=True, text=True, env=env)
            g('init', '-q'); os.makedirs(os.path.join(t, 'specs')); json.dump(BASE, open(os.path.join(t, 'specs', 'api.json'), 'w'))
            g('add', '.'); g('commit', '-q', '-m', 'base'); g('branch', 'base')
            os.remove(os.path.join(t, 'specs', 'api.json'))
            r = subprocess.run([sys.executable, '-I', D.__file__, '--dir', 'specs', '--base-ref', 'base'], cwd=t, capture_output=True, text=True)
            self.assertEqual(r.returncode, 1, r.stdout + r.stderr); self.assertIn('spec file removed', r.stdout)

if __name__ == '__main__': unittest.main()
