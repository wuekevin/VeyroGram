#!/usr/bin/env python3
"""Compute WCAG 2.1 contrast for every rule in tokens.json for both themes. Exit 1 on any failure; writes dist/contrast-report.json."""
import json, os, sys
H = os.path.dirname(os.path.abspath(__file__))
T = json.load(open(os.path.join(H, 'tokens.json'), encoding='utf8'))
def lum(h):
    h = h.lstrip('#'); c = [int(h[i:i + 2], 16) / 255 for i in (0, 2, 4)]
    c = [x / 12.92 if x <= 0.03928 else ((x + 0.055) / 1.055) ** 2.4 for x in c]
    return 0.2126 * c[0] + 0.7152 * c[1] + 0.0722 * c[2]
def ratio(a, b):
    hi, lo = sorted((lum(a), lum(b)), reverse=True); return (hi + 0.05) / (lo + 0.05)
rows, fails = [], 0
for kind, need in (('text', 4.5), ('ui', 3.0)):
    for theme, t in T['theme'].items():
        for fg, bg in T['contrast_rules'][kind]:
            r = round(ratio(t[fg], t[bg]), 2); ok = r >= need; fails += not ok
            rows.append({'theme': theme, 'kind': kind, 'fg': fg, 'bg': bg, 'ratio': r, 'min': need, 'pass': ok})
for g in T['contrast_rules'].get('gradient_text', []):
    stops = T['gradient'][g['stops']] if isinstance(g['stops'], str) else g['stops']
    for st in stops:
        a, b = (g['fg'], st) if 'fg' in g else (st, g['fg_on'])
        r = round(ratio(a, b), 2); ok = r >= g['min']; fails += not ok
        rows.append({'theme': 'gradient', 'kind': 'gradient:' + g['name'], 'fg': a, 'bg': b, 'ratio': r, 'min': g['min'], 'pass': ok})
os.makedirs(os.path.join(H, 'dist'), exist_ok=True)
json.dump({'checks': len(rows), 'failures': fails, 'results': rows}, open(os.path.join(H, 'dist', 'contrast-report.json'), 'w'), indent=1)
for r in rows:
    if not r['pass']: print('FAIL', r)
print(f"{len(rows)} contrast checks, {fails} failure(s); min text ratio {min(r['ratio'] for r in rows if r['kind']=='text')}; min gradient-text ratio {min(r['ratio'] for r in rows if r['kind'].startswith('gradient'))}")
sys.exit(1 if fails else 0)
