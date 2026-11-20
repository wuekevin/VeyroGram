#!/usr/bin/env python3
"""OpenAPI breaking-change gate (ARCHITECTURE AD-8, Story 1.2).
Compares each spec under --dir between a base git ref and the working tree. Breaking = removed path/method, removed 2xx response, newly required
parameter or request body, newly required request-body property, removed top-level response property, removed spec file.
A breaking change passes only if info.version's MAJOR increased or --adr points to an existing ADR file.
Usage: openapi_diff.py --dir packages/contracts/openapi [--base-ref origin/main] [--adr docs/architecture/adr/00NN-....md]
       openapi_diff.py --base-file a.json --head-file b.json     (direct comparison, used by tests)
Needs PyYAML for .yaml/.yml specs (pip install pyyaml); JSON works without it."""
import argparse, json, os, subprocess, sys

METHODS = ('get', 'put', 'post', 'delete', 'patch', 'head', 'options', 'trace')

def load(text, name):
    if name.endswith('.json'): return json.loads(text)
    try: import yaml
    except ImportError: sys.exit(f'PyYAML is required to read {name} (pip install pyyaml)')
    return yaml.safe_load(text)

def resolve(doc, node):
    seen = 0
    while isinstance(node, dict) and '$ref' in node and seen < 10:
        ref = node['$ref']
        if not ref.startswith('#/'): return node
        cur = doc
        for part in ref[2:].split('/'): cur = cur.get(part.replace('~1', '/').replace('~0', '~'), {}) if isinstance(cur, dict) else {}
        node = cur; seen += 1
    return node

def schema_props(doc, schema):
    s = resolve(doc, schema or {})
    return set((s.get('properties') or {}).keys()), set(s.get('required') or [])

def body_schema(doc, op):
    rb = resolve(doc, op.get('requestBody') or {})
    for ct, v in (rb.get('content') or {}).items():
        if 'json' in ct: return (v or {}).get('schema')
    return None

def resp_schema(doc, resp):
    r = resolve(doc, resp or {})
    for ct, v in (r.get('content') or {}).items():
        if 'json' in ct: return (v or {}).get('schema')
    return None

def params(doc, path_item, op):
    out = {}
    for p in list(path_item.get('parameters') or []) + list(op.get('parameters') or []):
        p = resolve(doc, p); out[(p.get('in'), p.get('name'))] = bool(p.get('required'))
    return out

def breaking(base, head):
    out = []
    bp, hp = base.get('paths') or {}, head.get('paths') or {}
    for path, bitem in bp.items():
        if path not in hp: out.append(f'removed path {path}'); continue
        hitem = hp[path]
        for m in METHODS:
            if m not in bitem: continue
            if m not in hitem: out.append(f'removed {m.upper()} {path}'); continue
            bo, ho = bitem[m], hitem[m]; tag = f'{m.upper()} {path}'
            for code in (bo.get('responses') or {}):
                if str(code).startswith('2') and code not in (ho.get('responses') or {}): out.append(f'{tag}: removed {code} response')
                else:
                    bs = schema_props(base, resp_schema(base, (bo['responses'] or {}).get(code)))[0]
                    hs = schema_props(head, resp_schema(head, (ho.get('responses') or {}).get(code)))[0]
                    for p in sorted(bs - hs): out.append(f'{tag}: response {code} lost property {p}')
            bparams, hparams = params(base, bitem, bo), params(head, hitem, ho)
            for k, req in hparams.items():
                if req and not bparams.get(k, False): out.append(f'{tag}: parameter {k[1]} ({k[0]}) is newly required')
            brb, hrb = resolve(base, bo.get('requestBody') or {}), resolve(head, ho.get('requestBody') or {})
            if hrb.get('required') and not brb.get('required'): out.append(f'{tag}: request body is newly required')
            _, breq = schema_props(base, body_schema(base, bo)); _, hreq = schema_props(head, body_schema(head, ho))
            for p in sorted(hreq - breq): out.append(f'{tag}: request property {p} is newly required')
    return out

def major(doc):
    try: return int(str((doc.get('info') or {}).get('version', '0')).split('.')[0])
    except ValueError: return 0

def git_show(ref, path):
    r = subprocess.run(['git', 'show', f'{ref}:{path}'], capture_output=True, text=True)
    return r.stdout if r.returncode == 0 else None

def compare_docs(base, head, name, adr):
    b = breaking(base, head)
    if not b: print(f'ok: {name}: no breaking changes'); return 0
    for x in b: print(f'BREAKING: {name}: {x}')
    if major(head) > major(base): print(f'note: {name}: breaking changes allowed, major version bumped {major(base)} -> {major(head)}'); return 0
    if adr and os.path.isfile(adr): print(f'note: {name}: breaking changes allowed by ADR {adr}'); return 0
    print(f'FAIL: {name}: bump info.version MAJOR or reference an existing ADR with --adr'); return 1

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--dir'); ap.add_argument('--base-ref', default='origin/main'); ap.add_argument('--adr')
    ap.add_argument('--base-file'); ap.add_argument('--head-file')
    a = ap.parse_args()
    if a.base_file and a.head_file:
        return compare_docs(load(open(a.base_file).read(), a.base_file), load(open(a.head_file).read(), a.head_file), a.head_file, a.adr)
    if not a.dir: ap.error('--dir or --base-file/--head-file required')
    if subprocess.run(['git', 'rev-parse', '--verify', '-q', a.base_ref], capture_output=True).returncode != 0:
        print(f'note: base ref {a.base_ref} not found (first run or shallow clone) — nothing to compare'); return 0
    names = {f for f in os.listdir(a.dir)} if os.path.isdir(a.dir) else set()
    tracked = subprocess.run(['git', 'ls-tree', '-r', '--name-only', a.base_ref, a.dir], capture_output=True, text=True).stdout.split()
    names |= {os.path.basename(t) for t in tracked}
    specs = sorted(n for n in names if n.endswith(('.json', '.yaml', '.yml')))
    if not specs: print('ok: no OpenAPI specs yet'); return 0
    rc = 0
    for n in specs:
        path = os.path.join(a.dir, n); base_txt = git_show(a.base_ref, path)
        if not os.path.isfile(path):
            print(f'BREAKING: {n}: spec file removed'); rc = 1; continue
        if base_txt is None: print(f'ok: {n}: new spec'); continue
        rc |= compare_docs(load(base_txt, n), load(open(path).read(), n), n, a.adr)
    return rc

if __name__ == '__main__': sys.exit(main())
