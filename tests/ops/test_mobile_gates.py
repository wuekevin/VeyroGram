# -*- coding: utf-8 -*-
"""Negative tests for the mobile build gates (Story 1.9): tools/check_android.py and tools/audit_gradle.py parsing must fail on vendor
credentials, the demo package, non-allowlisted repositories, an unpinned Gradle wrapper and shipped push configs/keystores."""
import os, subprocess, sys, tempfile, textwrap

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
sys.path.insert(0, os.path.join(ROOT, "tools"))
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


def good_tree():
    t = tempfile.mkdtemp()
    write(t, "app/build.gradle", "android {\n    namespace 'com.veyrogram.app'\n}\n")
    write(t, "settings.gradle", "repositories {\n  google()\n  mavenCentral()\n  maven { url \"https://jitpack.io\" }\n}\n")
    write(t, "gradle/wrapper/gradle-wrapper.properties",
          "distributionUrl=https\\://services.gradle.org/distributions/gradle-8.13-bin.zip\ndistributionSha256Sum=" + "a" * 64 + "\n")
    return t


t = good_tree(); r = run("check_android.py", t)
check("android: a clean tree passes", r.returncode == 0)

t = good_tree(); write(t, "wkpush/src/main/java/K.java", 'class K { static final String A = "108677047"; }\n')
r = run("check_android.py", t); check("android: a vendor push credential is refused", r.returncode == 1 and "vendor credential" in r.stdout)

t = good_tree(); write(t, "app/src/main/java/M.kt", "import com.test.ts.R\n")
r = run("check_android.py", t); check("android: the upstream demo package is refused", r.returncode == 1 and "com.test.ts" in r.stdout)

t = good_tree(); write(t, "app/src/test/java/T.kt", "// asserts com.test.ts is gone\n")
check("android: tests may name forbidden strings", run("check_android.py", t).returncode == 0)

t = good_tree(); write(t, "build.gradle", "repositories {\n  maven { url \"https://mirror.example.cn/repository/public\" }\n}\n")
r = run("check_android.py", t); check("android: a non-allowlisted repository is refused", r.returncode == 1 and "allowlist" in r.stdout)

t = good_tree(); write(t, "build.gradle", "repositories {\n  jcenter()\n}\n")
r = run("check_android.py", t); check("android: jcenter is refused", r.returncode == 1)

t = good_tree(); write(t, "build.gradle", "repositories {\n  maven { url \"http://jitpack.io\" }\n}\n")
r = run("check_android.py", t); check("android: a non-https repository is refused", r.returncode == 1 and "non-https" in r.stdout)

t = good_tree(); write(t, "gradle/wrapper/gradle-wrapper.properties", "distributionUrl=https\\://services.gradle.org/x.zip\n")
r = run("check_android.py", t); check("android: an unpinned Gradle wrapper is refused", r.returncode == 1 and "distributionSha256Sum" in r.stdout)

t = good_tree(); write(t, "app/google-services.json", "{}")
r = run("check_android.py", t); check("android: a committed google-services.json is refused", r.returncode == 1)

t = good_tree(); write(t, "release.jks", "x")
r = run("check_android.py", t); check("android: a committed keystore is refused", r.returncode == 1)

t = good_tree(); write(t, "app/build.gradle", "android {\n    namespace appId\n}\n")
r = run("check_android.py", t); check("android: a non-fixed namespace is refused", r.returncode == 1)

t = good_tree(); write(t, "app/build/generated/K.java", 'class K { static final String A = "108677047"; }\n')
check("android: build outputs are not scanned", run("check_android.py", t).returncode == 0)

GOOD_MANIFEST = '<manifest><application android:allowBackup="false" android:usesCleartextTraffic="false" android:dataExtractionRules="@xml/d" android:fullBackupContent="@xml/b"/></manifest>'
t = good_tree(); write(t, "app/src/main/AndroidManifest.xml", GOOD_MANIFEST)
check("android: a hardened manifest passes", run("check_android.py", t).returncode == 0)
t = good_tree(); write(t, "app/src/main/AndroidManifest.xml", GOOD_MANIFEST.replace('allowBackup="false"', 'allowBackup="true"'))
r = run("check_android.py", t); check("android: allowBackup=true is refused", r.returncode == 1 and "allowBackup" in r.stdout)
t = good_tree(); write(t, "app/src/main/AndroidManifest.xml", GOOD_MANIFEST.replace('usesCleartextTraffic="false"', 'usesCleartextTraffic="true"'))
r = run("check_android.py", t); check("android: cleartext in the main manifest is refused", r.returncode == 1 and "cleartext" in r.stdout)
t = good_tree(); write(t, "app/src/main/AndroidManifest.xml", GOOD_MANIFEST); write(t, "app/src/debug/AndroidManifest.xml", GOOD_MANIFEST.replace('usesCleartextTraffic="false"', 'usesCleartextTraffic="true"'))
check("android: cleartext in the debug manifest is allowed", run("check_android.py", t).returncode == 0)
t = good_tree(); write(t, "app/src/main/AndroidManifest.xml", GOOD_MANIFEST.replace(' android:dataExtractionRules="@xml/d"', ''))
r = run("check_android.py", t); check("android: missing data-extraction rules are refused", r.returncode == 1)
t = good_tree(); write(t, "w/T.java", "class T { void checkServerTrusted(X509Certificate[] c, String a) {\n }\n }\n")
r = run("check_android.py", t); check("android: a trust-all TrustManager is refused", r.returncode == 1 and "every server certificate" in r.stdout)
t = good_tree(); write(t, "w/H.java", "class H { Object v = b.hostnameVerifier((s, ss) -> true); }\n")
r = run("check_android.py", t); check("android: a trust-all HostnameVerifier is refused", r.returncode == 1 and "every host" in r.stdout)
t = good_tree(); write(t, "w/V.java", "class V { void onReceivedSslError(WebView w, SslErrorHandler h, SslError e) { h.proceed(); } }\n")
r = run("check_android.py", t); check("android: a WebView that proceeds past TLS errors is refused", r.returncode == 1 and "TLS" in r.stdout)
t = good_tree(); write(t, "w/V.java", "class V { void onReceivedSslError(WebView w, SslErrorHandler h, SslError e) { h.cancel(); } }\n")
check("android: a WebView that cancels on TLS errors passes", run("check_android.py", t).returncode == 0)

# audit_gradle coordinate parsing (no network)
import audit_gradle
tree = """\
+--- project :wkbase
|    +--- com.alibaba:fastjson:1.2.83
|    +--- com.google.guava:guava:33.3.1-android -> 33.8.0-android
|    \\--- androidx.core:core:{strictly 1.13.0} -> 1.13.0 (c)
"""
c = audit_gradle.coords(tree)
check("gradle audit: parses plain and upgraded coordinates", ("com.alibaba", "fastjson", "1.2.83") in c and ("com.google.guava", "guava", "33.8.0-android") in c)
empty = tempfile.mkdtemp(); write(empty, "deps.txt", "nothing here\n")
r = subprocess.run([sys.executable, "-I", os.path.join(ROOT, "tools", "audit_gradle.py"), os.path.join(empty, "deps.txt")], capture_output=True, text=True)
check("gradle audit: an empty/wrong dependency listing is refused", r.returncode == 2)

# ---- iOS policy ---------------------------------------------------------------------------------------------------------
def ios_tree():
    t = tempfile.mkdtemp()
    write(t, "TangSengDaoDao/AppDelegate.m", "#define VG_SERVER_IP @\"127.0.0.1:8080\"\n")
    write(t, "Pods/AFNetworking/AFNetworking/A.m", "#import <netinet/in.h>\n")
    return t


t = ios_tree(); check("ios: a clean tree passes", run("check_ios.py", t).returncode == 0)
t = ios_tree(); write(t, "TangSengDaoDao/AppDelegate.m", '#define SERVER_IP @"api.botgate.cn"\n')
r = run("check_ios.py", t); check("ios: the vendor API host is refused", r.returncode == 1 and "vendor API host" in r.stdout)
t = ios_tree(); write(t, "X.xcodeproj/project.pbxproj", "DEVELOPMENT_TEAM = H8PU463W68;\n")
r = run("check_ios.py", t); check("ios: the vendor team id is refused", r.returncode == 1)
t = ios_tree(); write(t, "X.xcodeproj/project.pbxproj", "PRODUCT_BUNDLE_IDENTIFIER = com.xinbida.tangsengdaodao;\n")
r = run("check_ios.py", t); check("ios: the vendor bundle id is refused", r.returncode == 1)
t = ios_tree(); write(t, "Pods/AFNetworking/AFNetworking/A.m", "#import <netinet6/in6.h>\n")
r = run("check_ios.py", t); check("ios: AFNetworking private header import is refused", r.returncode == 1 and "private header" in r.stdout)
t = ios_tree(); write(t, "Modules/WuKongBase/WuKongBase/Classes/WKApp.m", '[Bugly startWithAppId:@"82f8dd98ff"];\n')
r = run("check_ios.py", t); check("ios: the vendor Bugly app id is refused", r.returncode == 1)
t = ios_tree(); write(t, "Pods/YYImage/Vendor/WebP.framework/WebP", "x")
r = run("check_ios.py", t); check("ios: the old prebuilt WebP.framework is refused", r.returncode == 1 and "prebuilt" in r.stdout)
t = ios_tree(); write(t, "Modules/WuKongBase/WuKongBase/Classes/WKApp.m", "#import <Bugly/Bugly.h>\n")
check("ios: Bugly itself is allowed (only the vendor app id is refused)", run("check_ios.py", t).returncode == 0)


print(f"\n{len(fails)} FAILED" if fails else "\nall mobile gate tests passed")
sys.exit(1 if fails else 0)
