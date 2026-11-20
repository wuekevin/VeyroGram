#!/usr/bin/env python3
"""VeyroGram mark v2.0 — "A + C" direction chosen by the product owner (2026-11-19): speech-bubble outline with tail (from concept A) holding large
italic VG letters filled with the brand gradient (from concept C); no underline/smile. Designed to match the UI language (rounded geometry, navy + gradient).
Letter outlines are derived from Poppins Black Italic (SIL Open Font License 1.1, (c) The Poppins Project Authors) — attribution recorded in docs/audit/license-and-ip.md.
Writes brand/svg/logo-mark.svg, -glow, -small, mono variants. Art centre ~ (512, 525); asset scripts depend on these constants."""
import os
SVG = os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', 'brand', 'svg')
BUBBLE = 'M460 785A300 300 0 1 0 300 702L250 860Z'       # circle r=300 centred (512,490) + tail at lower-left
SPARK = 'M790 190L803 222L835 235L803 248L790 280L777 248L745 235L777 222Z'
LETTERS = 'M374.3 366.9 393.1 520.2 465.8 366.9H540.1L425.9 589.7H335.6L300.0 366.9Z M649.9 364.7Q679.2 364.7 699.8 374.3Q720.4 383.9 730.6 401.5Q740.9 419.2 739.9 443.1H666.3Q663.5 436.5 656.5 432.8Q649.6 429.2 638.9 429.2Q618.8 429.2 605.4 442.4Q592.0 455.7 588.2 478.0Q586.7 487.4 586.7 493.4Q586.7 529.9 629.2 529.9Q653.4 529.9 671.6 510.4H615.9L624.4 461.0H738.0L725.8 530.9Q706.9 555.7 678.2 573.5Q649.6 591.3 610.6 591.3Q580.4 591.3 559.0 580.3Q537.6 569.3 526.4 549.3Q515.2 529.3 515.2 502.5Q515.2 490.0 517.4 478.0Q523.1 445.0 541.4 419.2Q559.6 393.4 587.6 379.0Q615.6 364.7 649.9 364.7Z'
DEFS = ('<defs>'
 '<linearGradient id="bg" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#0A2159"/><stop offset=".5" stop-color="#020B2A"/><stop offset="1" stop-color="#020B30"/></linearGradient>'
 '<linearGradient id="bub" gradientUnits="userSpaceOnUse" x1="240" y1="200" x2="800" y2="860"><stop offset="0" stop-color="#00EAFD"/><stop offset=".35" stop-color="#2AB0FD"/><stop offset=".7" stop-color="#6171FE"/><stop offset="1" stop-color="#8548FC"/></linearGradient>'
 '<linearGradient id="txt" gradientUnits="userSpaceOnUse" x1="300" y1="360" x2="740" y2="600"><stop offset="0" stop-color="#00EAFD"/><stop offset=".5" stop-color="#4C9BFF"/><stop offset="1" stop-color="#A25BFF"/></linearGradient>'
 '<filter id="glow" x="-20%" y="-20%" width="140%" height="140%"><feGaussianBlur stdDeviation="16"/></filter></defs>')
RING = f'<path d="{BUBBLE}" fill="none" stroke="url(#bub)" stroke-width="46" stroke-linejoin="round" stroke-linecap="round"/>'
LET = f'<path d="{LETTERS}" fill="url(#txt)"/>'
LET_UP = f'<g transform="translate(0 0)">{LET}</g>'   # letters stay centred in the bubble (matches the product owner's reference mock-up)
DOTS = ('<circle cx="440" cy="667" r="19" fill="#13CBFB"/><circle cx="510" cy="667" r="19" fill="#2AB0FD"/><circle cx="580" cy="667" r="19" fill="#6171FE"/>')   # original 'typing' dots under VG
SPK = f'<path d="{SPARK}" fill="#EAF1FF"/>'
BODY = RING + LET_UP + DOTS + SPK
BODY_SMALL = RING + LET + SPK        # <=32px: no dots (they would blur into noise)
def doc(inner, glow=False):
    g = f'<g filter="url(#glow)" opacity=".55">{RING}{LET_UP}</g>' if glow else ''
    return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024" role="img" aria-labelledby="t"><title id="t">VeyroGram</title>{DEFS}'
            f'<rect x="60" y="65" width="905" height="903" rx="235" fill="url(#bg)"/>\n  {g}{inner}</svg>\n')
open(os.path.join(SVG, 'logo-mark.svg'), 'w').write(doc(BODY))
open(os.path.join(SVG, 'logo-mark-glow.svg'), 'w').write(doc(BODY, glow=True))
open(os.path.join(SVG, 'logo-mark-small.svg'), 'w').write(doc(BODY_SMALL))
def mono(color):
    return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024"><path d="{BUBBLE}" fill="none" stroke="{color}" stroke-width="46" stroke-linejoin="round" stroke-linecap="round"/>'
            f'<g transform="translate(0 0)"><path d="{LETTERS}" fill="{color}"/></g><circle cx="440" cy="667" r="19" fill="{color}"/><circle cx="510" cy="667" r="19" fill="{color}"/><circle cx="580" cy="667" r="19" fill="{color}"/><path d="{SPARK}" fill="{color}"/></svg>\n')
open(os.path.join(SVG, 'logo-mono-white.svg'), 'w').write(mono('#FFFFFF'))
open(os.path.join(SVG, 'logo-mono-black.svg'), 'w').write(mono('#000000'))
print('logo v2.0 (A+C) written')
