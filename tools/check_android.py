#!/usr/bin/env python3
"""Android source policy gate (Story 1.9). Usage: check_android.py <patched-android-tree>
Fails on: repositories outside the allowlist, vendor Firebase/push credentials, the upstream demo
package name, a missing/unpinned Gradle wrapper, secret-looking files, build outputs checked in."""
import pathlib, re, sys

ALLOWED_REPOS = {"google()", "mavenCentral()", "gradlePluginPortal()", "https://jitpack.io", "https://developer.huawei.com/repo/", "https://maven.google.com"}
VENDOR_STRINGS = ["108677047", "d14dac0803bc8470d9ace9e29657a160", "105660344", "2882303761520252187", "5302025297187",
                  "df1fd1874702472d973cbe3ffea25c66", "6739dd8f1150469680fc1a2ba1697a12", "tsdddemo", "tangsengdaodaoandroid"]
EXCLUDE = ("/build/", "/.gradle/", "/.git/")


def files(tree: pathlib.Path, suffixes):
    for p in tree.rglob("*"):
        s = "/" + p.relative_to(tree).as_posix()
        if p.is_file() and p.suffix in suffixes and not any(e in s for e in EXCLUDE):
            yield p


def main() -> int:
    tree = pathlib.Path(sys.argv[1])
    bad = []
    for p in files(tree, {".gradle"}):
        t = p.read_text(errors="replace")
        for m in re.finditer(r"\b(jcenter\(\)|mavenLocal\(\)|google\(\)|mavenCentral\(\)|gradlePluginPortal\(\))|url\s*[\"']([^\"']+)[\"']", t):
            tok = m.group(1) or m.group(2)
            if tok.startswith("http") and not tok.startswith("https://"):
                bad.append(f"{p.relative_to(tree)}: non-https repository {tok}")
            elif tok in ("jcenter()", "mavenLocal()") or (tok not in ALLOWED_REPOS and ("maven" in m.group(0) or m.group(2)) and tok.startswith("http")):
                if tok not in ALLOWED_REPOS and not tok.startswith("https://services.gradle.org"):
                    bad.append(f"{p.relative_to(tree)}: repository not on the allowlist: {tok}")
    for p in files(tree, {".java", ".kt", ".xml", ".gradle", ".json", ".properties"}):
        if "/src/test/" in "/" + p.relative_to(tree).as_posix():
            continue  # tests may name the forbidden strings to assert their absence
        t = p.read_text(errors="replace")
        for v in VENDOR_STRINGS:
            if v in t:
                bad.append(f"{p.relative_to(tree)}: vendor credential/identity string {v}")
        if re.search(r"\bcom\.test\.ts\b", t):
            bad.append(f"{p.relative_to(tree)}: upstream demo package com.test.ts")
    for name in ("google-services.json", "agconnect-services.json"):
        for p in tree.rglob(name):
            if not any(e in "/" + p.relative_to(tree).as_posix() for e in EXCLUDE):
                bad.append(f"{p.relative_to(tree)}: vendor push config must not be in the tree")
    for p in list(tree.rglob("*.jks")) + list(tree.rglob("*.keystore")):
        if not any(e in "/" + p.relative_to(tree).as_posix() for e in EXCLUDE):
            bad.append(f"{p.relative_to(tree)}: keystore must not be in the tree")
    wrapper = tree / "gradle/wrapper/gradle-wrapper.properties"
    if not wrapper.exists():
        bad.append("gradle wrapper missing")
    else:
        w = wrapper.read_text()
        if not re.search(r"^distributionSha256Sum=[0-9a-f]{64}$", w, re.M):
            bad.append("gradle wrapper distribution is not pinned by distributionSha256Sum")
        if "distributionUrl=https" not in w:
            bad.append("gradle wrapper distributionUrl is not https")
    app = (tree / "app/build.gradle").read_text() if (tree / "app/build.gradle").exists() else ""
    if "namespace 'com.veyrogram.app'" not in app:
        bad.append("app/build.gradle: namespace is not the fixed VeyroGram namespace")
    for b in bad:
        print("FAIL", b)
    print(f"android policy: {len(bad)} problem(s)")
    return 1 if bad else 0


if __name__ == "__main__":
    sys.exit(main())
