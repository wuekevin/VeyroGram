# -*- coding: utf-8 -*-
"""Negative tests for the client build gates (Story 1.10): tools/check_client_locks.py and tools/check_electron_security.py must really fail
on mirror URLs, missing integrity, unsafe Electron settings, raw IPC and allowlist drift."""
import os, subprocess, sys, tempfile, textwrap

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
fails = []


def check(name, cond):
    print(("ok   " if cond else "FAIL ") + name)
    if not cond:
        fails.append(name)


def run(tool, *args):
    return subprocess.run([sys.executable, "-I", os.path.join(ROOT, "tools", tool), *args], capture_output=True, text=True)


def write(base, rel, text):
    p = os.path.join(base, rel)
    os.makedirs(os.path.dirname(p), exist_ok=True)
    open(p, "w", encoding="utf-8").write(textwrap.dedent(text))


# ---- lockfile policy -------------------------------------------------------------------------------------------------
GOOD_YARN = '''\
# yarn lockfile v1

"a@^1.0.0":
  version "1.0.0"
  resolved "https://registry.npmjs.org/a/-/a-1.0.0.tgz"
  integrity sha512-AAAA
'''
t = tempfile.mkdtemp(); write(t, "yarn.lock", GOOD_YARN)
check("lock: official registry passes", run("check_client_locks.py", t).returncode == 0)

t = tempfile.mkdtemp(); write(t, "yarn.lock", GOOD_YARN.replace("registry.npmjs.org", "registry.npmmirror.com"))
r = run("check_client_locks.py", t); check("lock: mirror URL is refused", r.returncode == 1 and "non-official" in r.stdout)

t = tempfile.mkdtemp(); write(t, "yarn.lock", GOOD_YARN.replace("registry.npmjs.org", "evil.example"))
check("lock: arbitrary host is refused", run("check_client_locks.py", t).returncode == 1)

t = tempfile.mkdtemp(); write(t, "yarn.lock", GOOD_YARN.replace("  integrity sha512-AAAA\n", ""))
r = run("check_client_locks.py", t); check("lock: entry without integrity is refused", r.returncode == 1 and "integrity" in r.stdout)

t = tempfile.mkdtemp(); write(t, "yarn.lock", GOOD_YARN); write(t, ".yarnrc", 'registry "https://registry.npmmirror.com/"\n')
check("lock: mirror in .yarnrc is refused", run("check_client_locks.py", t).returncode == 1)

t = tempfile.mkdtemp(); write(t, "pnpm-lock.yaml", "packages:\n  x@1:\n    resolution: {integrity: sha512-A, tarball: https://registry.npmmirror.com/x/-/x-1.tgz}\n")
check("lock: pnpm tarball mirror is refused", run("check_client_locks.py", t).returncode == 1)

t = tempfile.mkdtemp()
check("lock: tree without a lockfile is refused", run("check_client_locks.py", t).returncode == 1)

# ---- electron static config ---------------------------------------------------------------------------------------------
SECURITY = 'export function secureWebPreferences(p: string) { return { preload: p, contextIsolation: true, nodeIntegration: false, sandbox: true }; }\n'
PRELOAD = '''\
export const SEND_CHANNELS = ["check-update"] as const;
export const INVOKE_CHANNELS = ["show-native-notification"] as const;
export const RECEIVE_CHANNELS = ["update-available"] as const;
const has = (l: readonly string[], c: string) => l.includes(c);
ipcRenderer.send(channel); if (!has(SEND_CHANNELS, channel)) throw 1;
'''
MAIN = '''\
import { onTrusted, handleTrusted, secureWebPreferences } from "./security";
new BrowserWindow({ webPreferences: {
  // preload
  ...secureWebPreferences("x") } });
onTrusted("check-update", () => {});
handleTrusted("show-native-notification", () => {});
win.webContents.send("update-available", 1);
'''


def tree(main=MAIN, preload=PRELOAD, electron="45.4.1", builder="module.exports = { asar: true };\n"):
    d = tempfile.mkdtemp()
    write(d, "package.json", '{"devDependencies":{"electron":"%s"}}' % electron)
    write(d, "src-election/main/security.ts", SECURITY)
    write(d, "src-election/main/index.ts", main)
    write(d, "src-election/preload/index.ts", preload)
    write(d, "electron-builder.js", builder)
    return d


def elec(d):
    return run("check_electron_security.py", d)


check("electron: hardened tree passes", elec(tree()).returncode == 0)
r = elec(tree(main=MAIN.replace('...secureWebPreferences("x")', "nodeIntegration: true"))); check("electron: nodeIntegration true is refused", r.returncode == 1 and "nodeIntegration" in r.stdout)
check("electron: contextIsolation false is refused", elec(tree(main=MAIN + "const x = { contextIsolation: false };\n")).returncode == 1)
check("electron: sandbox false is refused", elec(tree(main=MAIN + "const x = { sandbox: false };\n")).returncode == 1)
check("electron: webSecurity false is refused", elec(tree(main=MAIN + "const x = { webSecurity: false };\n")).returncode == 1)
check("electron: literal webPreferences is refused", elec(tree(main=MAIN + "new BrowserWindow({ webPreferences: { preload: 'x' } });\n")).returncode == 1)
check("electron: raw ipcMain handler is refused", elec(tree(main=MAIN + 'ipcMain.on("evil", () => {});\n')).returncode == 1)
r = elec(tree(main=MAIN + 'onTrusted("new-channel", () => {});\n')); check("electron: handler missing from the preload allowlist is refused", r.returncode == 1 and "new-channel" in r.stdout)
r = elec(tree(preload=PRELOAD.replace('["check-update"]', '["check-update", "orphan"]'))); check("electron: allowlisted channel without a handler is refused", r.returncode == 1 and "orphan" in r.stdout)
r = elec(tree(main=MAIN + 'win.webContents.send("leak", 1);\n')); check("electron: sending a non-allowlisted event is refused", r.returncode == 1 and "leak" in r.stdout)
r = elec(tree(electron="26.0.0")); check("electron: outdated Electron is refused", r.returncode == 1 and "older" in r.stdout)
r = elec(tree(electron="^45.4.1")); check("electron: a version range is refused", r.returncode == 1 and "exact" in r.stdout)
check("electron: download mirror is refused", elec(tree(builder='module.exports = { electronDownload: { mirror: "https://x/" } };\n')).returncode == 1)
check("electron: asar disabled is refused", elec(tree(builder="module.exports = { asar: false };\n")).returncode == 1)
check("electron: unchecked preload passthrough is refused", elec(tree(preload=PRELOAD.replace("if (!has(SEND_CHANNELS, channel)) throw 1;", ""))).returncode == 1)

# ---- npm licence gate -------------------------------------------------------------------------------------------------
def pkgtree(**pk):
    d = tempfile.mkdtemp()
    for name, meta in pk.items():
        files = meta.pop("_files", {})
        write(d, f"node_modules/{name}/package.json", __import__("json").dumps({"name": name, "version": "1.0.0", **meta}))
        for fn, txt in files.items():
            write(d, f"node_modules/{name}/{fn}", txt)
    return d


def lic(d):
    return run("npm_supply.py", "licenses", d)


check("npm licences: permissive packages pass", lic(pkgtree(a={"license": "MIT"}, b={"license": "(MIT OR Apache-2.0)"})).returncode == 0)
check("npm licences: GPL is refused", lic(pkgtree(a={"license": "GPL-3.0"})).returncode == 1)
check("npm licences: AGPL is refused", lic(pkgtree(a={"license": "AGPL-3.0-only"})).returncode == 1)
check("npm licences: dual licence with a permissive alternative passes", lic(pkgtree(a={"license": "(BSD-3-Clause OR GPL-2.0)"})).returncode == 0)
check("npm licences: AND with a copyleft term is refused", lic(pkgtree(a={"license": "MIT AND GPL-3.0"})).returncode == 1)
check("npm licences: missing licence is refused", lic(pkgtree(a={})).returncode == 1)
check("npm licences: a commercial 'SEE LICENSE IN' is refused", lic(pkgtree(a={"license": "SEE LICENSE IN LICENSE.md"})).returncode == 1)
check("npm licences: UNLICENSED is refused", lic(pkgtree(a={"license": "UNLICENSED"})).returncode == 1)
check("npm licences: no license field but an MIT text passes", lic(pkgtree(a={"_files": {"LICENSE": "Permission is hereby granted, free of charge, to any person obtaining a copy"}})).returncode == 0)
check("npm licences: an empty tree is refused", lic(tempfile.mkdtemp()).returncode == 2)
t = pkgtree(a={"license": "MIT"}); o = os.path.join(t, "sbom.json")
r = run("npm_supply.py", "sbom", t, "-o", o); doc = __import__("json").load(open(o))
check("npm sbom: CycloneDX 1.5 with purl and licence", r.returncode == 0 and doc["specVersion"] == "1.5" and doc["components"][0]["purl"] == "pkg:npm/a@1.0.0" and doc["components"][0]["licenses"][0]["expression"] == "MIT")

sys.exit(1 if fails else 0)
