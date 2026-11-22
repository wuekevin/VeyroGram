# -*- coding: utf-8 -*-
"""Tests for tools/gen_clients.py (Story 1.11): it must refuse what it cannot generate faithfully, and detect clients that are out of date."""
import importlib.util, os, re, sys

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
spec = importlib.util.spec_from_file_location("gen_clients", os.path.join(ROOT, "tools/gen_clients.py"))
g = importlib.util.module_from_spec(spec)
spec.loader.exec_module(g)
fails = []


def check(name, cond):
    print(("ok   " if cond else "FAIL ") + name)
    if not cond:
        fails.append(name)


ir = g.IR()
for bad in ({"oneOf": [{"type": "string"}, {"type": "integer"}]}, {"allOf": [{"type": "object"}]}, {"type": ["string", "integer"]}, {"type": "object", "properties": {"a": {"type": "string"}}}):
    try:
        ir.conv(bad)
        check("unsupported construct is refused: %s" % list(bad)[0], False)
    except g.Unsupported:
        check("unsupported construct is refused: %s" % list(bad)[0], True)

ir, files = g.generate()
check("every operation in the contract is generated", len(ir.ops) == 28)
check("the open payload envelope keeps extra fields in Go", "Extra map[string]any" in files["go/client.go"] and "[extra: string]: unknown;" in files["typescript/src/index.ts"])
check("the error schema does not shadow the language's Error type", "interface Error " not in files["typescript/src/index.ts"] and "type Error struct" in files["go/client.go"] and "type ErrorBody struct" in files["go/client.go"])
def auth_flag(op):
    body = re.search(r"func \(c \*Client\) %s\(.*?\n}\n" % g.camel(op["id"]), files["go/client.go"], re.S).group(0)
    m = re.search(r"c\.call\([^\n]*, (true|false), (\d+), (?:&out|nil)\)", body)
    return m.group(1) == "true", int(m.group(2))


check("every operation's auth requirement and success status in the Go client match the contract", all(auth_flag(o) == (o["auth"], o["status"]) for o in ir.ops))
check("the checked-in clients are current", all(os.path.exists(os.path.join(g.OUT, p)) and open(os.path.join(g.OUT, p), encoding="utf-8").read() == c for p, c in files.items()))
sys.exit(1 if fails else 0)
