#!/usr/bin/env python3
"""Automated accessibility + layout scan of docs/design/prototypes (every manifest entry x light/dark), in an iframe of the EXACT viewport.
Checks: axe-core (WCAG 2.0/2.1 A+AA + best-practice), keyboard focus indicator on every focusable, focus-target size (>=24px, WCAG 2.2 2.5.8 as a floor),
clipped text at the tested text scale, horizontal page scroll (reflow) — plus an extra 320px-wide reflow run for 390px pages.
Usage: python3 -I tools/a11y_scan.py <path-to-axe.min.js>   Writes docs/design/a11y-axe-report.json; exit 1 on any finding.
IMPORTANT: automated checks cover only part of WCAG; they do NOT replace screen-reader / keyboard-flow / zoom testing and do not prove conformance."""
import html, json, os, re, subprocess, sys, tempfile
ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), '..'))
P = os.path.join(ROOT, 'docs', 'design', 'prototypes')
CHROME = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'
axe = os.path.abspath(sys.argv[1])
manifest = json.load(open(os.path.join(P, 'manifest.json')))
JS = r'''
const FRAME = document.getElementById('f'); const out = document.getElementById('out');
FRAME.addEventListener('load', () => {
  const d = FRAME.contentDocument, w = FRAME.contentWindow, s = d.createElement('script'); s.src = 'file://AXE';
  s.onload = async () => {
    const r = await w.axe.run(d, {runOnly: ['wcag2a','wcag2aa','wcag21a','wcag21aa','best-practice']});
    const custom = [];
    const foc = [...d.querySelectorAll('a[href],button,input,select,textarea,[tabindex]:not([tabindex="-1"])')].filter(e => e.offsetParent !== null && !e.closest('[inert]'));
    for (const e of foc) {
      const label = (e.getAttribute('aria-label') || e.innerText || e.getAttribute('placeholder') || e.tagName).trim().slice(0, 24);
      e.focus(); const has = el => { const c = w.getComputedStyle(el); return (c.outlineStyle !== 'none' && parseFloat(c.outlineWidth) > 0) || c.boxShadow !== 'none'; }; const host = e.closest('.field,.search'); const ring = has(e) || (host && has(host));
      if (!ring) custom.push({id: 'focus-indicator-missing', el: label});
      const b = e.getBoundingClientRect(); if (Math.min(b.width, b.height) < 24 && e.tagName !== 'INPUT') custom.push({id: 'target-below-24px', el: label, size: Math.round(b.width) + 'x' + Math.round(b.height)});
    }
    for (const e of d.querySelectorAll('body *')) {
      const cs = w.getComputedStyle(e);
      if (['hidden','clip'].includes(cs.overflowX) && e.scrollWidth > e.clientWidth + 1 && e.tagName !== 'INPUT' && !e.closest('.sr') && cs.textOverflow !== 'ellipsis' && e.clientWidth > 0 && e.children.length < 6 && (e.innerText || '').trim()) custom.push({id: 'text-clipped-horizontally', el: (e.innerText || '').trim().slice(0, 24)});
    }
    const phone = d.querySelector('.phone'); const sw = Math.max(d.documentElement.scrollWidth, phone ? phone.scrollWidth : 0);
    if (sw > w.innerWidth + 1) custom.push({id: 'horizontal-scroll', el: 'page', size: sw + ' > ' + w.innerWidth});
    out.textContent = JSON.stringify({v: r.violations.map(v => ({id: v.id, impact: v.impact, n: v.nodes.length, help: v.help, targets: v.nodes.slice(0, 4).map(n => n.target.join(' '))})), inc: r.incomplete.map(i => i.id), pass: r.passes.length, focusables: foc.length, custom});
  };
  d.head.appendChild(s);
});
'''.replace('AXE', axe)
res, bad = {}, 0
tmp = tempfile.mkdtemp()
runs = []
for m in manifest:
    for theme in ('light', 'dark'):
        runs.append((m, theme, m['width'], m['height'], m['tag']))
        if m['width'] == 390 and not m['tag']: runs.append((m, theme, 320, 700, '-320reflow'))
for m, theme, w, h, tag in runs:
    wrap = os.path.join(tmp, 't.html')
    open(wrap, 'w', encoding='utf8').write(f'<!doctype html><body style="margin:0"><iframe id="f" src="file://{os.path.join(P, m["file"])}?theme={theme}{m.get("query","")}" width="{w}" height="{h}" style="border:0"></iframe><pre id="out"></pre><script>{JS}</script></body>')
    o = subprocess.run([CHROME, '--headless=new', '--disable-gpu', '--allow-file-access-from-files', '--virtual-time-budget=12000', f'--window-size={max(w, 600)},{h + 100}', '--dump-dom', f'file://{wrap}'], capture_output=True, text=True, timeout=180).stdout
    mm = re.search(r'<pre id="out">(.*?)</pre>', o, re.S)
    r = json.loads(html.unescape(mm.group(1))) if mm and mm.group(1).strip() else {'error': 'no output'}
    key = f"{m['page']}{m['tag']}{'' if tag == m['tag'] else tag}/{theme}"
    res[key] = r; bad += len(r.get('v', [])) + len(r.get('custom', [])) + ('error' in r)
json.dump(res, open(os.path.join(ROOT, 'docs', 'design', 'a11y-axe-report.json'), 'w', encoding='utf8'), ensure_ascii=False, indent=1)
for k, v in res.items():
    if 'error' in v: print(f'{k}: ERROR {v["error"]}')
    for x in v.get('v', []): print(f"{k}: axe {x['impact']} {x['id']} ({x['n']}) {x['targets'][:2]}")
    for x in v.get('custom', []): print(f"{k}: {x['id']} {x.get('el','')} {x.get('size','')}")
print(f"{len(res)} runs, {bad} finding(s); axe 'incomplete' (manual review needed): {sum(len(v.get('inc', [])) for v in res.values())}; focusables checked: {sum(v.get('focusables', 0) for v in res.values())}")
sys.exit(1 if bad else 0)
