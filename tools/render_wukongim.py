#!/usr/bin/env python3
"""Render infrastructure/wukongim/wukongim.prod.toml.template from WK_* environment variables.
Fails if any placeholder is unset/empty or any ${...} remains. Output is written 0600 and then checked with wukongim_config_check.
Usage: WK_...=... python3 -I tools/render_wukongim.py <template> <out.toml>"""
import os, re, sys

def render(template, env):
    missing = sorted({m for m in re.findall(r'\$\{(WK_[A-Z_]+)\}', template) if not env.get(m)})
    if missing:
        raise SystemExit('render: unset/empty variables: ' + ', '.join(missing))
    out = re.sub(r'\$\{(WK_[A-Z_]+)\}', lambda m: env[m.group(1)].replace('\\', '\\\\').replace('"', '\\"'), template)
    if '${' in out: raise SystemExit('render: unresolved placeholder remains')
    return out

if __name__ == '__main__':
    if len(sys.argv) != 3: raise SystemExit(__doc__)
    text = render(open(sys.argv[1]).read(), os.environ)
    fd = os.open(sys.argv[2], os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    with os.fdopen(fd, 'w') as f: f.write(text)
