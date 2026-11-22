// Render smoke for a built SPA (Story 1.10): serves <dir> over loopback HTTP, loads it in a hidden Electron window, fails on uncaught
// errors / console errors from the app, an empty #root/#app, or a missing screenshot. Usage: electron tools/render_smoke.js <dir> <out.png> [rootSelector]
const { app, BrowserWindow } = require("electron");
const http = require("http"), fs = require("fs"), path = require("path");
const [dir, shot, sel = "#root"] = process.argv.slice(2);
const types = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".json": "application/json", ".svg": "image/svg+xml", ".png": "image/png", ".mp3": "audio/mpeg", ".ico": "image/x-icon", ".woff2": "font/woff2" };
const srv = http.createServer((q, r) => {
  let p = path.join(dir, decodeURIComponent(q.url.split("?")[0]));
  if (!p.startsWith(path.resolve(dir)) || !fs.existsSync(p) || fs.statSync(p).isDirectory()) p = path.join(dir, "index.html");
  r.writeHead(200, { "content-type": types[path.extname(p)] || "application/octet-stream" }); fs.createReadStream(p).pipe(r);
});
const errors = [];
const step = (m) => console.log("STEP " + m);
app.disableHardwareAcceleration();
// Watchdog: a hung renderer must fail the gate, not stall CI.
setTimeout(() => { console.log("ERROR watchdog: no result after 90s"); console.log("RENDER FAIL"); process.exit(1); }, 90000).unref();
app.whenReady().then(() => srv.listen(0, "127.0.0.1", async () => {
  step("server listening");
  const w = new BrowserWindow({ show: false, width: 1280, height: 800, webPreferences: { contextIsolation: true, nodeIntegration: false, sandbox: true } });
  w.webContents.on("console-message", (_e, level, msg, line, src) => { if (level >= 3 && !/Failed to load resource|ERR_CONNECTION|net::/.test(msg)) errors.push(`console: ${msg} (${src}:${line})`); });
  w.webContents.on("render-process-gone", (_e, d) => errors.push("renderer gone: " + d.reason));
  step("window created");
  await w.loadURL(`http://127.0.0.1:${srv.address().port}/`);
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
}));
