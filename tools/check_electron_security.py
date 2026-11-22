#!/usr/bin/env python3
"""Static Electron security configuration gate (Story 1.10, AD-17). Usage: check_electron_security.py <apps/web dir>.

Fails on: unsafe webPreferences, a BrowserWindow not built from secureWebPreferences(), raw ipcMain.on/handle (must use onTrusted/handleTrusted),
IPC channel lists in the preload that do not match the main-process handlers/senders, Electron older than the supported window, a
download mirror or non-https update URL in electron-builder config. The runtime counterpart is src-election/test/smoke.js."""
import json, pathlib, re, sys

MIN_ELECTRON_MAJOR = 43  # supported window on 2026-11-22 is 43, 44, 45 (latest three stable majors); raise this as Electron ships.
UNSAFE = [
    (r"nodeIntegration\s*:\s*true", "nodeIntegration enabled"),
    (r"nodeIntegrationInWorker\s*:\s*true", "nodeIntegrationInWorker enabled"),
    (r"nodeIntegrationInSubFrames\s*:\s*true", "nodeIntegrationInSubFrames enabled"),
    (r"contextIsolation\s*:\s*false", "contextIsolation disabled"),
    (r"webSecurity\s*:\s*false", "webSecurity disabled"),
    (r"sandbox\s*:\s*false", "sandbox disabled"),
    (r"allowRunningInsecureContent\s*:\s*true", "insecure content allowed"),
    (r"webviewTag\s*:\s*true", "webview tag enabled"),
    (r"enableRemoteModule", "remote module"),
    (r"@electron/remote|require\(['\"]electron['\"]\)\.remote", "remote module"),
]


def list_in(text: str, name: str) -> set[str]:
    m = re.search(rf"export const {name} = \[(.*?)\] as const", text, re.S)
    return set(re.findall(r"\"([^\"]+)\"", m.group(1))) if m else set()


def check(web: pathlib.Path) -> list[str]:
    errs = []
    src = web / "src-election"
    files = [p for p in src.rglob("*.ts") if "test" not in p.parts and p.suffix == ".ts" and not p.name.endswith(".d.ts")]
    if not files:
        return [f"{src}: no Electron sources found"]
    sends, invokes, outgoing = set(), set(), set()
    for p in files:
        t = p.read_text(encoding="utf-8")
        rel = p.relative_to(web)
        for rx, what in UNSAFE:
            for m in re.finditer(rx, t):
                errs.append(f"{rel}: {what}")
        if p.name != "security.ts":
            if re.search(r"\bwebPreferences\s*:(?!\s*\{(?:\s*//[^\n]*\n)*\s*\.\.\.secureWebPreferences\()", t):
                errs.append(f"{rel}: literal webPreferences (use secureWebPreferences)")
            if re.search(r"ipcMain\s*\.\s*(on|handle|once)\s*\(", t):
                errs.append(f"{rel}: raw ipcMain handler (use onTrusted/handleTrusted)")
            if "new BrowserWindow(" in t and "secureWebPreferences(" not in t:
                errs.append(f"{rel}: BrowserWindow without secureWebPreferences")
        for m in re.finditer(r"onTrusted\(\s*[\"']([^\"']+)[\"']", t):
            sends.add(m.group(1))
        for m in re.finditer(r"handleTrusted\(\s*[\"']([^\"']+)[\"']", t):
            invokes.add(m.group(1))
        for m in re.finditer(r"webContents\.send\(\s*[\"']([^\"']+)[\"']", t):
            outgoing.add(m.group(1))
        for m in re.finditer(r"webContents\.send\(\s*opt\.cmd", t):
            outgoing.add("<dynamic>")
    pre = (src / "preload" / "index.ts").read_text(encoding="utf-8")
    allow_send, allow_invoke, allow_recv = list_in(pre, "SEND_CHANNELS"), list_in(pre, "INVOKE_CHANNELS"), list_in(pre, "RECEIVE_CHANNELS")
    if not (allow_send and allow_invoke and allow_recv):
        errs.append("preload: channel allowlists missing")
    for c in sorted(sends - allow_send):
        errs.append(f"main handles '{c}' but the preload does not allow it")
    for c in sorted(allow_send - sends):
        errs.append(f"preload allows send '{c}' but no main handler exists")
    for c in sorted(invokes - allow_invoke):
        errs.append(f"main handles invoke '{c}' but the preload does not allow it")
    for c in sorted(allow_invoke - invokes):
        errs.append(f"preload allows invoke '{c}' but no main handler exists")
    for c in sorted(outgoing - allow_recv - {"<dynamic>"}):
        errs.append(f"main sends '{c}' but the preload does not allow receiving it")
    if re.search(r"ipcRenderer\.(send|invoke)\(\s*channel", pre) and not re.search(r"has\(\s*(SEND|INVOKE)_CHANNELS", pre):
        errs.append("preload: unchecked channel passthrough")
    pj = json.loads((web / "package.json").read_text())
    spec = (pj.get("devDependencies", {}).get("electron") or pj.get("dependencies", {}).get("electron") or "")
    m = re.search(r"(\d+)\.", spec)
    if not m:
        errs.append("package.json: electron version not found")
    elif not re.fullmatch(r"\d+\.\d+\.\d+", spec):
        errs.append(f"electron {spec} must be an exact version (electron-builder cannot resolve a range)")
    elif int(m.group(1)) < MIN_ELECTRON_MAJOR:
        errs.append(f"electron {spec} is older than supported major {MIN_ELECTRON_MAJOR}")
    for cfg in web.glob("electron-builder*.js"):
        t = cfg.read_text(encoding="utf-8")
        if re.search(r"\bmirror\s*:", t):
            errs.append(f"{cfg.name}: Electron download mirror")
        if re.search(r"asar\s*:\s*false", t):
            errs.append(f"{cfg.name}: asar disabled")
        for u in re.findall(r"url\s*:\s*[\"'](http://[^\"']+)", t):
            errs.append(f"{cfg.name}: non-https update URL {u}")
    return errs


def main() -> int:
    if len(sys.argv) != 2:
        print(__doc__)
        return 2
    errs = check(pathlib.Path(sys.argv[1]))
    for e in errs:
        print("FAIL", e)
    if not errs:
        print("electron security config ok")
    return 1 if errs else 0


if __name__ == "__main__":
    sys.exit(main())
