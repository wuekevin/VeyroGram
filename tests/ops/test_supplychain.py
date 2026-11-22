# -*- coding: utf-8 -*-
"""Negative tests for the supply-chain gates (Story 1.24): they must really block. Test secrets are assembled at run time so this file is clean itself."""
import datetime, importlib.util, os, sys, tempfile

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))


def load(name):
    spec = importlib.util.spec_from_file_location(name, os.path.join(ROOT, "tools", name + ".py"))
    m = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(m)
    return m


sc, ss = load("supplychain"), load("secret_scan")
fails = []


def check(name, cond):
    print(("ok   " if cond else "FAIL ") + name)
    if not cond:
        fails.append(name)


MIT = "MIT License\n\nPermission is hereby granted, free of charge, to any person obtaining a copy of this software"
GPL = "GNU GENERAL PUBLIC LICENSE\nVersion 3, 29 June 2007\n Permission is hereby granted"  # a GPL text must lose even if it also contains MIT-like words
MPL = "Mozilla Public License Version 2.0\n1.12. \"Secondary License\" means either the GNU General Public License, Version 2.0, the GNU Lesser General Public License"
check("MIT is accepted", sc.classify(MIT)[0] == "MIT")
check("GPL is blocked", sc.classify(GPL)[0] == "")
check("MPL-2.0 naming the GPL as a secondary licence is accepted", sc.classify(MPL)[0] == "MPL-2.0")
check("no licence text (UNLICENSED) is blocked", sc.classify("")[0] == "")
check("'all rights reserved' without a grant is blocked", sc.classify("Copyright (c) Acme. All rights reserved. Proprietary.")[0] == "")
check("commercial restriction on top of MIT wording is blocked", sc.classify(MIT + "\nCommons Clause License Condition v1.0")[0] == "")
check("non-commercial terms are blocked", sc.classify(MIT + "\nFor non-commercial use only")[0] == "")

# Waivers: need reason, approver and expiry; an expired one stops applying.
d = tempfile.mkdtemp()
sc.WAIVERS = os.path.join(d, "w.yaml")
open(sc.WAIVERS, "w").write("waivers:\n  - {module: a/b, reason: r, approved_by: someone, expires: '2999-01-01'}\n  - {module: c/d, reason: r, approved_by: someone, expires: '2000-01-01'}\n")
w = sc.waivers(datetime.date(2026, 1, 1))
check("a current waiver applies", "a/b" in w)
check("an expired waiver does not", "c/d" not in w)
open(sc.WAIVERS, "w").write("waivers:\n  - {module: a/b, reason: r, expires: '2999-01-01'}\n")
try:
    sc.waivers(datetime.date(2026, 1, 1)); check("a waiver without an approver is refused", False)
except SystemExit:
    check("a waiver without an approver is refused", True)
check("third-party mirrors are recognised", bool(sc.MIRROR.search("registry=https://registry.npmmirror.com/")) and bool(sc.MIRROR.search("https://registry.npm.taobao.org")))
check("the official registry is not a mirror", not sc.MIRROR.search("https://registry.npmjs.org/") and not sc.MIRROR.search("https://proxy.golang.org"))

# Secret scan.
def hits(text):
    return [x["rule"] for x in ss.scan_text("f.txt", text)]

pem = "-----BEGIN " + "RSA PRIVATE KEY-----"
aws = "AKIA" + "ABCDEFGHIJKLMNOP"
gh = "ghp_" + "a1B2c3D4e5F6g7H8i9J0k1L2m3N4o5P6q7R8"
check("private key block", "private-key" in hits(pem))
check("AWS access key", "aws-access-key" in hits("id=" + aws))
check("GitHub token", "github-token" in hits("x " + gh))
check("high-entropy assigned secret", "assigned-secret" in hits('api_key = "' + "q9Zk3vL8xW2mT5pR7yB4nC6" + '"'))
check("password inside a DSN", "mysql-dsn-password" in hits("dsn := \"app:" + "Zq9mW3pL8vR2" + "@tcp(db:3306)/x\""))
check("placeholders are not findings", hits('password = "@@FILL:PASSWORD@@"') == [] and hits('api_key = "<your-api-key-here>"') == [])
check("low-entropy values are not findings", hits('secret = "aaaaaaaaaaaaaaaaaaaa"') == [])

# The allow-list excuses one value in one file and nothing else.
f = os.path.join(d, "x.txt")
open(f, "w").write('api_key = "' + "q9Zk3vL8xW2mT5pR7yB4nC6" + '"\n')
x = ss.scan_text("x.txt", open(f).read())[0]
ss.ALLOW = os.path.join(d, "allow.yaml")
open(ss.ALLOW, "w").write("allow:\n  - {path: x.txt, rule: %s, fp: %s, reason: test}\n" % (x["rule"], x["fp"]))
ss.ROOT = d
check("an allowed value passes (exact path)", ss.main(["--files", "x.txt"]) == 0)
open(f, "a").write('token = "' + "Zm8Qr2Lw5Xc7Vb3Nk9Pd4" + '"\n')
check("a second, different value in the same file is still caught", ss.main(["--files", "x.txt"]) == 1)
open(ss.ALLOW, "w").write("allow:\n  - {path: x.txt, rule: assigned-secret, fp: abc}\n")
try:
    ss.main(["--files", "x.txt"]); check("an allow entry without a reason is refused", False)
except SystemExit:
    check("an allow entry without a reason is refused", True)

sys.exit(1 if fails else 0)
