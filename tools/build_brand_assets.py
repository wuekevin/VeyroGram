#!/usr/bin/env python3
"""Export VeyroGram platform brand assets from brand/svg/logo-mark.svg (vector). Requires Google Chrome (headless) and Pillow for ICO.
Usage: tools/venv-python tools/build_brand_assets.py   (PNG rasterization is always from SVG, never from logo.png)"""
import os, re, subprocess, sys, tempfile, shutil
ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), '..'))
SVG = os.path.join(ROOT, 'brand', 'svg')
OUT = os.path.join(ROOT, 'brand', 'icons')
CHROME = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'
src = open(os.path.join(SVG, 'logo-mark.svg'), encoding='utf8').read()
defs = re.search(r'<defs>.*?</defs>', src, re.S).group(0)
small_src = open(os.path.join(SVG, 'logo-mark-small.svg'), encoding='utf8').read()
body_small = re.search(r'<rect x="60".*?/>\s*(.*)</svg>', small_src, re.S).group(1)
body = re.search(r'<rect x="60".*?/>\s*(.*)</svg>', src, re.S).group(1)       # everything after the rounded-square background
def wrap(inner, w=1024, h=1024, extra=''):
    return f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {w} {h}" width="{w}" height="{h}">{defs}{extra}{inner}</svg>'
def art_small(scale, cx=512, cy=512):
    return f'<g transform="translate({cx} {cy}) scale({scale}) translate(-512 -525)">{body_small}</g>'
def art(scale, cx=512, cy=512):   # art bbox centre ~ (538, 528)
    return f'<g transform="translate({cx} {cy}) scale({scale}) translate(-512 -525)">{body}</g>'
BG = '<rect width="1024" height="1024" fill="url(#bg)"/>'
def _mono_inner(color):
    t = open(os.path.join(SVG, 'logo-mono-white.svg'), encoding='utf8').read()
    t = t.split('>', 1)[1].rsplit('</svg>', 1)[0]
    return t.replace('#FFFFFF', color)
def mono(scale, cx=512, cy=512, color='#000'):
    return f'<g transform="translate({cx} {cy}) scale({scale}) translate(-512 -525)">{_mono_inner(color)}</g>'
def tray(color):
    return (f'<path d="M460 785A300 300 0 1 0 300 702L250 860Z" fill="none" stroke="{color}" stroke-width="78" stroke-linecap="round" stroke-linejoin="round" transform="translate(512 512) scale(0.98) translate(-512 -525)"/>'
            f'<g fill="{color}" transform="translate(512 512) scale(0.98) translate(-512 -525)"><circle cx="418" cy="505" r="38"/><circle cx="512" cy="505" r="38"/><circle cx="606" cy="505" r="38"/></g>')
def glyph(color_stroke, color_dots, dots=True):
    d = ('<g transform="translate(512 512) scale(0.98) translate(-512 -525)">'
         f'<path d="M460 785A300 300 0 1 0 300 702L250 860Z" fill="none" stroke="{color_stroke}" stroke-width="86" stroke-linecap="round" stroke-linejoin="round"/>')
    if dots: d += f'<g fill="{color_dots}"><circle cx="418" cy="505" r="44"/><circle cx="512" cy="505" r="44"/><circle cx="606" cy="505" r="44"/></g>'
    return d + '</g>'
svgs = {
  'favicon-simple.svg': wrap(BG + art_small(1.0)),                              # new mark stays legible down to 16px (VG strokes are bold)
  'tray-template-16.svg': wrap(glyph('#000', '#000', dots=False)),             # 16px: bubble only (3 dots cannot be resolved at 16px)
  'tray-white-16.svg': wrap(glyph('#FFFFFF', '#FFFFFF', dots=False)),
  'logo-icon-fullbleed.svg': wrap(BG + art(0.98)),                     # iOS / store / desktop base (opaque square)
  'android-adaptive-background.svg': wrap(BG),
  'android-adaptive-foreground.svg': wrap(art(0.80)),                  # art ≈59% of canvas: inside the 66/108 (61%) safe zone
  'android-adaptive-monochrome.svg': wrap(mono(0.80, color='#FFFFFF')),
  'web-maskable.svg': wrap(BG + art(0.74)),                            # content inside 80% safe zone
  'tray-template.svg': wrap(tray('#000')),                             # macOS template / light tray: simplified glyph (bubble + dots) for 16-22px
  'tray-white.svg': wrap(tray('#FFFFFF')),
}
for n, s in svgs.items():
    open(os.path.join(SVG, n), 'w', encoding='utf8').write(s)
tmp = tempfile.mkdtemp()
def render(svg_name, out_png, size, transparent=False, w=None, h=None):
    w = w or size; h = h or size
    html = f'<!doctype html><body style="margin:0;background:transparent"><img src="file://{os.path.join(SVG, svg_name)}" width="{w}" height="{h}" style="display:block"></body>'
    hp = os.path.join(tmp, 'r.html'); open(hp, 'w').write(html)
    os.makedirs(os.path.dirname(out_png), exist_ok=True)
    args = [CHROME, '--headless=new', '--disable-gpu', '--hide-scrollbars', f'--window-size={w},{h}', '--force-device-scale-factor=1']
    if transparent: args.append('--default-background-color=00000000')
    subprocess.run(args + [f'--screenshot={out_png}', f'file://{hp}'], check=True, capture_output=True)
I = lambda *p: os.path.join(OUT, *p)
# iOS
render('logo-icon-fullbleed.svg', I('ios', 'AppIcon-1024.png'), 1024)
# Android
for n, f in (('foreground', 'android-adaptive-foreground.svg'), ('monochrome', 'android-adaptive-monochrome.svg')):
    render(f, I('android', f'ic_launcher_{n}-432.png'), 432, transparent=True)
render('android-adaptive-background.svg', I('android', 'ic_launcher_background-432.png'), 432)
render('logo-icon-fullbleed.svg', I('android', 'play-store-512.png'), 512)
# Web / PWA
for s in (16, 32, 48): render('favicon-simple.svg', I('web', f'favicon-{s}.png'), s)
for s in (180,): render('logo-icon-fullbleed.svg', I('web', 'apple-touch-icon-180.png'), s)
for s in (192, 512): render('logo-icon-fullbleed.svg', I('web', f'icon-{s}.png'), s)
render('web-maskable.svg', I('web', 'maskable-512.png'), 512)
shutil.copy(os.path.join(SVG, 'favicon-simple.svg'), I('web', 'favicon.svg'))
try:
    from PIL import Image
    Image.open(I('web', 'favicon-48.png')).convert('RGBA').save(I('web', 'favicon.ico'), sizes=[(16,16),(32,32),(48,48)])
except Exception as e:
    print('favicon.ico skipped:', e)
open(I('web', 'site.webmanifest'), 'w').write('{"name":"VeyroGram","short_name":"VeyroGram","icons":[{"src":"icon-192.png","sizes":"192x192","type":"image/png"},{"src":"icon-512.png","sizes":"512x512","type":"image/png"},{"src":"maskable-512.png","sizes":"512x512","type":"image/png","purpose":"maskable"}],"theme_color":"#020B2A","background_color":"#020B2A","display":"standalone"}\n')
# Desktop
for s in (16, 24, 32, 48, 64, 128, 256, 512, 1024): render('favicon-simple.svg' if s <= 32 else 'logo-mark.svg', I('desktop', f'icon-{s}.png'), s, transparent=True)
for s in (16, 22, 32):
    render('tray-template-16.svg' if s <= 22 else 'tray-template.svg', I('desktop', f'tray-template-{s}.png'), s, transparent=True)
    render('tray-white-16.svg' if s <= 22 else 'tray-white.svg', I('desktop', f'tray-white-{s}.png'), s, transparent=True)
try:
    from PIL import Image
    im = Image.open(I('desktop', 'icon-256.png')).convert('RGBA')
    im.save(I('desktop', 'icon.ico'), sizes=[(16,16),(24,24),(32,32),(48,48),(64,64),(128,128),(256,256)])
except Exception as e:
    print('ICO skipped:', e)
iconset = os.path.join(tmp, 'VeyroGram.iconset'); os.makedirs(iconset)
for s, n in ((16,'16x16'),(32,'16x16@2x'),(32,'32x32'),(64,'32x32@2x'),(128,'128x128'),(256,'128x128@2x'),(256,'256x256'),(512,'256x256@2x'),(512,'512x512'),(1024,'512x512@2x')):
    shutil.copy(I('desktop', f'icon-{s}.png'), os.path.join(iconset, f'icon_{n}.png'))
subprocess.run(['iconutil', '-c', 'icns', iconset, '-o', I('desktop', 'icon.icns')], check=True)
# OG / Twitter card 1200x630
og = wrap(f'<rect width="1200" height="630" fill="url(#bg)"/>{art(0.62, 300, 315)}'
          '<text x="560" y="300" font-family="Inter, Helvetica, Arial, sans-serif" font-size="88" font-weight="700" fill="#EAF1FF">VeyroGram</text>'
          '<text x="562" y="372" font-family="Inter, Helvetica, Arial, sans-serif" font-size="34" fill="#A5B2DA">Messaging for every screen</text>', 1200, 630)
open(os.path.join(SVG, 'og-card.svg'), 'w', encoding='utf8').write(og)
render('og-card.svg', I('web', 'og-1200x630.png'), 0, w=1200, h=630)
shutil.rmtree(tmp)
print('ok')
