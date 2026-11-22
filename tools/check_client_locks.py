#!/usr/bin/env python3
"""Lockfile policy gate for the Web/Desktop and Manager clients (Story 1.10, AD-18).

Every resolved tarball must come from the official npm registry (registry.npmjs.org / registry.yarnpkg.com) with an integrity hash, and no
registry/mirror configuration may point anywhere else. Usage: check_client_locks.py <tree> [<tree> ...]  (exit 1 on any violation)."""
import pathlib, re, sys

OFFICIAL = ("https://registry.npmjs.org/", "https://registry.yarnpkg.com/")
URL = re.compile(r"https?://[^\s\"'}),]+")
CONFIG_FILES = (".yarnrc", ".npmrc", ".yarnrc.yml", "pnpm-workspace.yaml", "Dockerfile", "Dockerfile.ghcr", "Makefile", "electron-builder.js")
MIRROR_WORDS = re.compile(r"npmmirror|taobao|cnpm|mirrors/electron|ELECTRON_MIRROR|electron_mirror", re.I)


def check_tree(tree: pathlib.Path) -> list[str]:
    errs = []
    locks = [p for p in tree.rglob("*") if p.name in ("yarn.lock", "pnpm-lock.yaml", "package-lock.json") and "node_modules" not in p.parts]
    if not locks:
        errs.append(f"{tree}: no lockfile found")
    for lock in locks:
        text = lock.read_text(encoding="utf-8", errors="replace")
        rel = lock.relative_to(tree)
        resolved = 0
        for i, line in enumerate(text.splitlines(), 1):
            if "resolved " in line or "tarball:" in line or "\"resolved\"" in line:
                resolved += 1
                for u in URL.findall(line):
                    if not u.startswith(OFFICIAL):
                        errs.append(f"{rel}:{i}: non-official tarball URL {u[:90]}")
            elif MIRROR_WORDS.search(line):
                errs.append(f"{rel}:{i}: mirror reference")
        if lock.name == "yarn.lock":
            blocks = re.split(r"\n\n", text)
            for b in blocks:
                if b.startswith("#") or not b.strip():
                    continue
                if "  resolved " in b and "  integrity " not in b:
                    errs.append(f"{rel}: entry without integrity: {b.splitlines()[0][:70]}")
    for p in tree.rglob("*"):
        if p.name in CONFIG_FILES and p.is_file() and "node_modules" not in p.parts:
            for i, line in enumerate(p.read_text(encoding="utf-8", errors="replace").splitlines(), 1):
                if MIRROR_WORDS.search(line):
                    errs.append(f"{p.relative_to(tree)}:{i}: mirror/registry override: {line.strip()[:90]}")
    return errs


def main(argv: list[str]) -> int:
    if not argv:
        print(__doc__)
        return 2
    errs = []
    for a in argv:
        errs += check_tree(pathlib.Path(a))
    for e in errs:
        print("FAIL", e)
    if not errs:
        print("lockfile policy ok: official registries only, integrity present")
    return 1 if errs else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
