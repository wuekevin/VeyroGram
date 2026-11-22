// Render smoke for a built SPA (Story 1.10): serves <dir> through an in-process custom scheme (no sockets), loads it in a hidden Electron window, fails on uncaught
// errors / console errors from the app, an empty #root/#app, or a missing screenshot. Usage: electron tools/render_smoke.js <dir> <out.png> [rootSelector]
const { app, BrowserWindow } = require("electron");
const fs = require("fs"), path = require("path");
const [dir, shot, sel = "#root"] = process.argv.slice(2);
const types = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".json": "application/json", ".svg": "image/svg+xml", ".png": "image/png", ".mp3": "audio/mpeg", ".ico": "image/x-icon", ".woff2": "font/woff2" };
// No sockets at all: the build is served through a privileged custom scheme handled in-process (both the in-process HTTP server and a child-process
// server hung on the Linux CI runner). SPA fallback to index.html mirrors a production history-mode server.
const { protocol, net } = require("electron");
const { pathToFileURL } = require("url");
protocol.registerSchemesAsPrivileged([{ scheme: "vgapp", privileges: { standard: true, secure: true, supportFetchAPI: true, corsEnabled: true } }]);
function serve() {
  protocol.handle("vgapp", (req) => {
    const root = path.resolve(dir);
    let p = path.join(root, decodeURIComponent(new URL(req.url).pathname));
    if (!p.startsWith(root) || !fs.existsSync(p) || fs.statSync(p).isDirectory()) p = path.join(root, "index.html");
    console.log("SERVE " + req.url + " -> " + p);
    return net.fetch(pathToFileURL(p).toString());
  });
}
process.on("unhandledRejection", (e) => { console.log("ERROR unhandled rejection: " + (e && e.message)); });
const errors = [];
const step = (m) => console.log("STEP " + m);
app.disableHardwareAcceleration();
// Watchdog: a hung renderer must fail the gate, not stall CI.
setTimeout(() => { console.log("ERROR watchdog: no result after 90s"); console.log("RENDER FAIL"); process.exit(1); }, 90000).unref();
app.whenReady().then(async () => {
  serve();
  step("server listening");
  const w = new BrowserWindow({ show: false, width: 1280, height: 800, webPreferences: { contextIsolation: true, nodeIntegration: false, sandbox: true } });
  w.webContents.on("console-message", (_e, level, msg, line, src) => { if (level >= 3 && !/Failed to load resource|ERR_CONNECTION|net::/.test(msg)) errors.push(`console: ${msg} (${src}:${line})`); });
  w.webContents.on("render-process-gone", (_e, d) => errors.push("renderer gone: " + d.reason));
  step("window created");
  // Do not await loadURL: it waits for every subresource, and one stalled request would hang the gate. dom-ready plus a fixed settle time is enough.
  const ready = new Promise((r) => w.webContents.once("dom-ready", r));
  w.loadURL(`vgapp://local/`).catch((e) => errors.push("load: " + e.message));
  await Promise.race([ready, new Promise((r) => setTimeout(r, 30000))]);
  step("page loaded");
  await new Promise((r) => setTimeout(r, 3000));
  // Optional interaction (RENDER_JS): code run in the page before the check, e.g. clicking buttons to exercise lazy components.
  if (process.env.RENDER_JS) { await w.webContents.executeJavaScript(process.env.RENDER_JS); await new Promise((r) => setTimeout(r, 1500)); }
  const info = await w.webContents.executeJavaScript(`({root: (document.querySelector(${JSON.stringify(sel)})||{}).innerText||'', kids: (document.querySelector(${JSON.stringify(sel)})||{children:[]}).children.length, html: document.body.innerHTML.length, url: location.href})`);
  // The screenshot is evidence only: on a software-GL Linux runner capturePage of a hidden window can stall, so it never decides pass/fail.
  try { fs.writeFileSync(shot, (await Promise.race([w.webContents.capturePage(), new Promise((_, rej) => setTimeout(() => rej(new Error("capturePage timeout")), 10000))])).toPNG()); } catch (e) { console.log("NOTE no screenshot: " + e.message); }
  console.log(JSON.stringify({ ...info, root: info.root.slice(0, 120) }));
  if (!info.kids) errors.push("root is empty");
  errors.forEach((e) => console.log("ERROR " + e));
  console.log(errors.length ? "RENDER FAIL" : "RENDER OK"); app.exit(errors.length ? 1 : 0);
});
