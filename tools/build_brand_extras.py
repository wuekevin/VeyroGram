#!/usr/bin/env python3
"""Extra brand assets: launch screens, Android status-bar icon, iOS dark/tinted app icons, horizontal/vertical lockups.
Run after tools/build_brand_assets.py. Needs Chrome + Pillow."""
import os, re, subprocess, tempfile, shutil
ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), '..'))
SVG, ICONS = os.path.join(ROOT, 'brand', 'svg'), os.path.join(ROOT, 'brand', 'icons')
CHROME = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'
src = open(os.path.join(SVG, 'logo-mark-glow.svg'), encoding='utf8').read()
flat = open(os.path.join(SVG, 'logo-mark.svg'), encoding='utf8').read()
defs = re.search(r'<defs>.*?</defs>', src, re.S).group(0)
body_glow = re.search(r'<rect x="60".*?/>\s*(.*)</svg>', src, re.S).group(1)
body_flat = re.search(r'<rect x="60".*?/>\s*(.*)</svg>', flat, re.S).group(1)
FONT = "font-family=\"Inter, 'Noto Sans SC', Helvetica, Arial, sans-serif\""
def art(body, scale, cx, cy): return f'<g transform="translate({cx} {cy}) scale({scale}) translate(-512 -525)">{body}</g>'
def svg(w, h, inner): return f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {w} {h}" width="{w}" height="{h}">{defs}{inner}</svg>'
BG = lambda w, h: f'<rect width="{w}" height="{h}" fill="url(#bg)"/>'
files = {}
# launch screens (portrait): mark with glow + wordmark; text is NOT outlined (font stack, see brand-guidelines)
for name, (w, h) in {'launch-ios-1290x2796': (1290, 2796), 'launch-android-1080x1920': (1080, 1920)}.items():
    files[name + '.svg'] = svg(w, h, BG(w, h) + art(body_glow, w / 1024 * 0.62, w / 2, h * 0.43) +
        f'<text x="{w/2}" y="{h*0.43 + w*0.36}" text-anchor="middle" {FONT} font-size="{w*0.07}" font-weight="700" fill="#EAF1FF">VeyroGram</text>')
# Android 12+ splash icon (centre 2/3 safe circle), transparent
files['android-splash-icon.svg'] = svg(1152, 1152, art(body_flat, 0.74, 576, 576))
# Android status-bar notification icon: alpha-only white outline (bubble + 3 dots), 24dp
tray = ('<g transform="translate(512 512) scale(1.0) translate(-512 -525)"><path d="M460 785A300 300 0 1 0 300 702L250 860Z" fill="none" stroke="#fff" stroke-width="96" stroke-linecap="round" stroke-linejoin="round"/>'
        '<g fill="#fff"><circle cx="418" cy="505" r="50"/><circle cx="512" cy="505" r="50"/><circle cx="606" cy="505" r="50"/></g></g>')
files['ic_stat_vg.svg'] = svg(1024, 1024, tray)
# iOS dark variant: darker base; tinted variant is generated from grayscale below
files['ios-dark-source.svg'] = svg(1024, 1024, '<rect width="1024" height="1024" fill="#01061A"/>' + art(body_flat, 0.98, 512, 512))
# lockups
wm = lambda x, y, size, fill: f'<text x="{x}" y="{y}" {FONT} font-size="{size}" font-weight="700" fill="{fill}" letter-spacing="-1">VeyroGram</text>'
files['lockup-horizontal-dark.svg'] = svg(1400, 420, BG(1400, 420).replace('<rect', '<rect rx="48"') + art(body_flat, 0.45, 215, 215) + wm(470, 250, 150, '#EAF1FF'))
files['lockup-horizontal-light.svg'] = svg(1400, 420, '<rect width="1400" height="420" rx="48" fill="#F4F7FF"/>' + '<g transform="translate(40 14) scale(.36)">' + re.sub(r'<defs>.*?</defs>', '', flat, flags=re.S).replace('<svg', '<svg', 1).split('>', 1)[1].rsplit('</svg>', 1)[0] + '</g>' + wm(470, 250, 150, '#0A1230'))
files['lockup-vertical-dark.svg'] = svg(900, 1000, BG(900, 1000).replace('<rect', '<rect rx="64"') + art(body_flat, 0.78, 450, 400) + wm(0, 0, 0, 'none') + f'<text x="450" y="880" text-anchor="middle" {FONT} font-size="130" font-weight="700" fill="#EAF1FF">VeyroGram</text>')
for n, c in files.items(): open(os.path.join(SVG, n), 'w', encoding='utf8').write(c)
tmp = tempfile.mkdtemp()
def render(name, out, w, h, transparent=False):
    hp = os.path.join(tmp, 'r.html'); open(hp, 'w').write(f'<!doctype html><body style="margin:0;background:transparent"><img src="file://{os.path.join(SVG, name)}" width="{w}" height="{h}" style="display:block"></body>')
    os.makedirs(os.path.dirname(out), exist_ok=True)
    a = [CHROME, '--headless=new', '--disable-gpu', '--hide-scrollbars', f'--window-size={w},{h}', '--force-device-scale-factor=1'] + (['--default-background-color=00000000'] if transparent else [])
    subprocess.run(a + [f'--screenshot={out}', f'file://{hp}'], check=True, capture_output=True)
render('launch-ios-1290x2796.svg', os.path.join(ICONS, 'ios', 'launch-1290x2796.png'), 1290, 2796)
render('launch-android-1080x1920.svg', os.path.join(ICONS, 'android', 'launch-1080x1920.png'), 1080, 1920)
render('android-splash-icon.svg', os.path.join(ICONS, 'android', 'splash-icon-1152.png'), 1152, 1152, True)
for dp, px in (('mdpi', 24), ('hdpi', 36), ('xhdpi', 48), ('xxhdpi', 72), ('xxxhdpi', 96)):
    render('ic_stat_vg.svg', os.path.join(ICONS, 'android', 'notification', f'drawable-{dp}', 'ic_stat_vg.png'), px, px, True)
render('ios-dark-source.svg', os.path.join(ICONS, 'ios', 'AppIcon-1024-dark.png'), 1024, 1024)
from PIL import Image, ImageOps
im = Image.open(os.path.join(ICONS, 'ios', 'AppIcon-1024-dark.png')).convert('L')
tinted = ImageOps.autocontrast(im, cutoff=1)   # grayscale mark on black; iOS applies the user's tint
tinted.convert('RGB').save(os.path.join(ICONS, 'ios', 'AppIcon-1024-tinted.png'))
for n, w, h in (('lockup-horizontal-dark', 1400, 420), ('lockup-horizontal-light', 1400, 420), ('lockup-vertical-dark', 900, 1000)):
    render(n + '.svg', os.path.join(ICONS, 'lockups', n + '.png'), w, h, True)
shutil.rmtree(tmp); print('extras ok')
