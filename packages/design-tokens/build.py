#!/usr/bin/env python3
"""Generate platform token files from tokens.json (single source, ARCHITECTURE AD-17). Output: dist/{tokens.css,android/colors.xml,ios/VGTokens.swift}"""
import json, os
import subprocess, sys
H = os.path.dirname(os.path.abspath(__file__))
if subprocess.call([sys.executable, '-I', os.path.join(H, 'check_contrast.py')]): sys.exit('contrast check failed - tokens not built')
T = json.load(open(os.path.join(H, 'tokens.json'), encoding='utf8'))
out = os.path.join(H, 'dist'); os.makedirs(os.path.join(out, 'android'), exist_ok=True); os.makedirs(os.path.join(out, 'ios'), exist_ok=True)
def css_vars(d, prefix=''): return ''.join(f'  --vg-{prefix}{k}: {v};\n' for k, v in d.items())
css = ':root{\n' + css_vars(T['brand'], 'brand-') + css_vars(T['theme']['light']) + f"  --vg-gradient-brand: {T['gradient']['brand']};\n  --vg-gradient-action: {T['gradient']['action']};\n"
css += ''.join(f'  --vg-space-{k}: {v}px;\n' for k, v in T['spacing'].items()) + ''.join(f'  --vg-radius-{k}: {v}px;\n' for k, v in T['radius'].items())
css += ''.join(f'  --vg-elev-{k}: {v};\n' for k, v in T['elevation'].items()) + '}\n'
css += '@media (prefers-color-scheme: dark){:root:not([data-theme="light"]){\n' + css_vars(T['theme']['dark']) + '}}\n:root[data-theme="dark"]{\n' + css_vars(T['theme']['dark']) + '}\n'
open(os.path.join(out, 'tokens.css'), 'w').write(css)
def xml(d, suffix=''): return ''.join(f'    <color name="vg_{k.replace("-", "_")}{suffix}">{v}</color>\n' for k, v in d.items())
open(os.path.join(out, 'android', 'colors.xml'), 'w').write('<?xml version="1.0" encoding="utf-8"?>\n<resources>\n' + xml(T['brand']) + xml(T['theme']['light']) + '</resources>\n')
os.makedirs(os.path.join(out, 'android-night'), exist_ok=True)
open(os.path.join(out, 'android-night', 'colors.xml'), 'w').write('<?xml version="1.0" encoding="utf-8"?>\n<resources>\n' + xml(T['theme']['dark']) + '</resources>\n')
def sw(d, pre): return ''.join(f'    static let {pre}{"".join(w.capitalize() if i else w for i, w in enumerate(k.split("-")))} = UIColor(hex: 0x{v.lstrip("#")})\n' for k, v in d.items())
open(os.path.join(out, 'ios', 'VGTokens.swift'), 'w').write('import UIKit\n\nenum VGLight {\n' + sw(T['theme']['light'], '') + '}\nenum VGDark {\n' + sw(T['theme']['dark'], '') + '}\nenum VGBrand {\n' + sw(T['brand'], '') + '}\n')
print('tokens built ->', out)
