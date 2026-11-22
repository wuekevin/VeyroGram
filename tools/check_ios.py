#!/usr/bin/env python3
"""iOS source policy gate (Story 1.9). Usage: check_ios.py <patched-ios-tree>
Fails on: the upstream vendor team id / bundle ids / API host / URL scheme / associated domain, and on AFNetworking's private-header import
that breaks Xcode 26 builds. Pods are vendored in the upstream tree, so the checks read them in place."""
import pathlib, sys

BAD = {"H8PU463W68": "vendor development team id", "com.xinbida": "vendor bundle id", "api.botgate.cn": "vendor API host",
       "applinks:githubim.com": "vendor associated domain", "<string>botgate</string>": "vendor URL scheme"}
SCAN = {".pbxproj", ".plist", ".entitlements", ".m", ".h", ".swift"}


def main() -> int:
    tree = pathlib.Path(sys.argv[1])
    bad = []
    for p in tree.rglob("*"):
        rel = p.relative_to(tree).as_posix()
        if not p.is_file() or rel.startswith(("Pods/", ".git/")) and "Pods/AFNetworking" not in rel:
            continue
        if p.suffix in SCAN and not rel.startswith("Pods/"):
            t = p.read_text(errors="replace")
            for k, why in BAD.items():
                if k in t and not (k == "com.xinbida" and "Copyright" in t.split(k)[0][-40:]):
                    bad.append(f"{rel}: {why} ({k})")
        if rel.startswith("Pods/AFNetworking/") and p.suffix == ".m" and "netinet6/in6.h" in p.read_text(errors="replace"):
            bad.append(f"{rel}: private header netinet6/in6.h (fails under Xcode 26)")
    for b in sorted(set(bad)):
        print("FAIL", b)
    print(f"ios policy: {len(set(bad))} problem(s)")
    return 1 if bad else 0


if __name__ == "__main__":
    sys.exit(main())
