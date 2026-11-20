#!/usr/bin/env python3
"""Fail if an Emoji character appears in navigation/menu/tab/toolbar/sidebar UI source (BMAD-MASTER 0.7, PRD FR-68).
Chat content and sticker data are out of scope. Usage: lint_no_emoji_nav.py <dir> [<dir> ...]
Heuristic: emoji found in files whose path looks like navigation/router/layout, or on any line that defines icon/menu/tab/nav/title/label; exit 1 on findings. Not a proof of absence: review manually and extend rules per client."""
import os, re, sys
# Emoji ranges only (no arrows/keyboard symbols such as U+23CE) to avoid false positives.
EMOJI = re.compile('[\U0001F300-\U0001FAFF\U0001F000-\U0001F2FF\u2600-\u27BF\u2B50\u2B55\u2B06\u2B07\u231A\u231B\u23E9-\u23F3\u23F8-\u23FA\u2705\u274C]')
PATH = re.compile(r'(nav|menu|tab|sidebar|toolbar|bottombar|drawer|router|route|layout)', re.I)
CTX = re.compile(r'(icon|menu|tab|nav|sidebar|toolbar|title|label)', re.I)   # any file: flag emoji on lines that look like UI-chrome definitions
EXT = {'.vue', '.ts', '.tsx', '.js', '.jsx', '.json', '.xml', '.kt', '.java', '.swift', '.m', '.h', '.storyboard', '.xib', '.strings', '.html', '.dart'}
SKIP = {'node_modules', '.git', 'build', 'dist', 'Pods', '.gradle'}
bad = 0
for root_dir in sys.argv[1:] or ['.']:
    for r, ds, fs in os.walk(root_dir):
        ds[:] = [d for d in ds if d not in SKIP]
        for f in fs:
            p = os.path.join(r, f)
            if os.path.splitext(f)[1].lower() not in EXT: continue
            path_hit = bool(PATH.search(p))
            try: txt = open(p, encoding='utf8', errors='ignore').read()
            except OSError: continue
            for i, line in enumerate(txt.splitlines(), 1):
                m = EMOJI.search(line)
                if m and (path_hit or CTX.search(line)):
                    bad += 1; print(f'{p}:{i}: emoji U+{ord(m.group()[0]):04X} in navigation UI source')
print(f'{bad} finding(s)')
sys.exit(1 if bad else 0)
