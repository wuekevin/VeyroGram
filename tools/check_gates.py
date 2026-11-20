#!/usr/bin/env python3
"""Exit 1 unless every required gate in docs/design/APPROVALS.md is APPROVED or WAIVED (with a note). Run before Phase 3 coding."""
import os, re, sys
p = os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', 'docs', 'design', 'APPROVALS.md')
rows = [[c.strip() for c in l.strip().strip('|').split('|')] for l in open(p, encoding='utf8') if re.match(r'\|\s*G\d+', l)]
bad = 0
for r in rows:
    gate, what, _, required, decision, who, date, note = (r + [''] * 8)[:8]
    ok = decision in ('APPROVED', 'WAIVED') and who and date and (decision != 'WAIVED' or note)
    if required == '是' and not ok:
        bad += 1; print(f'{gate} {what[:30]}: {decision or "EMPTY"} (signer={who or "-"}, date={date or "-"}) -> NOT PASSED')
print(f'{len(rows)} gates, {bad} not passed'); sys.exit(1 if bad else 0)
