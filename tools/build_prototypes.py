#!/usr/bin/env python3
"""Build high-fidelity HTML prototypes from design tokens + the original icon set, then screenshot baselines (light/dark).
Usage: python3 tools/build_prototypes.py [--no-shots]   (screenshots need Google Chrome)
Everything here is a DESIGN PROTOTYPE for review, not production code. Static data only; no Emoji used as chrome icons."""
import os, re, subprocess, sys, json
ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), '..'))
OUT = os.path.join(ROOT, 'docs', 'design', 'prototypes')
BASE = os.path.join(ROOT, 'docs', 'design', 'baselines')
CHROME = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'
os.makedirs(OUT, exist_ok=True); os.makedirs(BASE, exist_ok=True)
TOK = open(os.path.join(ROOT, 'packages', 'design-tokens', 'dist', 'tokens.css'), encoding='utf8').read()
SPRITE = open(os.path.join(ROOT, 'brand', 'icons', 'ui', 'sprite.svg'), encoding='utf8').read()
MARK = '../../../brand/svg/logo-mark.svg'

def ic(name, size=24, cls=''):
    return f'<svg class="ic {cls}" width="{size}" height="{size}" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><use href="#vg-{name}"/></svg>'

CSS = TOK + """
*{box-sizing:border-box}html,body{margin:0}
body{background:var(--vg-bg-canvas);color:var(--vg-text-primary);font-family:Inter,'Noto Sans SC','PingFang SC',system-ui,sans-serif;font-size:16px;line-height:24px;-webkit-font-smoothing:antialiased}
button{font:inherit;color:inherit;border:0;background:transparent;padding:0;cursor:pointer;text-align:inherit}
a{color:inherit;text-decoration:none}
:focus-visible{outline:2px solid var(--vg-focus-ring);outline-offset:2px}
.inp{flex:1 1 0;width:0;min-width:0;border:0;background:transparent;font:inherit;color:inherit;outline:none;padding:0}.inp::placeholder{color:var(--vg-text-placeholder);opacity:1}
h1,h2{margin:0}
@media (prefers-reduced-motion:reduce){*{animation:none!important;transition:none!important}}
.ic{flex:none;vertical-align:middle}.sel .ic{fill:currentColor;fill-opacity:.16;stroke-width:2.1}
.muted{color:var(--vg-text-secondary)}.sm{font-size:14px;line-height:20px}.cap{font-size:12px;line-height:16px;font-weight:500}
.row{display:flex;align-items:center}.gap2{gap:8px}.gap3{gap:12px}.gap4{gap:16px}.grow{flex:1;min-width:0}.col{display:flex;flex-direction:column}
.ell{white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.phone{width:100%;height:100vh;overflow-x:hidden;overflow-y:auto;display:flex;flex-direction:column;background:var(--vg-bg-canvas)}
.phone>*{flex-shrink:0}
.status{min-height:44px;display:flex;justify-content:space-between;align-items:center;padding:0 24px;font-weight:600;font-size:14px}
.topbar{min-height:52px;display:flex;align-items:center;gap:12px;padding:0 16px;background:var(--vg-surface);border-bottom:1px solid var(--vg-border)}
.topbar h1{font-size:17px;line-height:24px;margin:0;font-weight:600}
.iconbtn{width:44px;height:44px;border-radius:12px;display:grid;place-items:center;color:var(--vg-text-primary)}
.tabbar{position:sticky;bottom:0;margin-top:auto;display:flex;min-height:64px;background:var(--vg-surface);border-top:1px solid var(--vg-border)}
.tab{flex:1;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:2px;font-size:11px;font-weight:500;color:var(--vg-text-secondary);position:relative}
.tab.sel{color:var(--vg-primary);font-weight:700}
.dot{position:absolute;top:8px;right:calc(50% - 18px);min-width:18px;height:18px;border-radius:9px;background:var(--vg-danger);color:#fff;font-size:11px;font-weight:700;display:grid;place-items:center;padding:0 5px;border:2px solid var(--vg-surface)}
.avatar{width:48px;height:48px;border-radius:50%;display:grid;place-items:center;color:#fff;font-weight:700;font-size:18px;flex:none}
.avatar.sm{width:36px;height:36px;font-size:14px}.avatar.lg{width:72px;height:72px;font-size:28px}
.li{display:flex;align-items:center;gap:12px;padding:12px 16px;min-height:72px;background:var(--vg-surface);border-bottom:1px solid var(--vg-border)}
.li .t{font-weight:600}.badge{min-width:20px;height:20px;border-radius:10px;background:var(--vg-primary);color:var(--vg-on-primary);font-size:12px;font-weight:700;display:grid;place-items:center;padding:0 6px}
.badge.mute{background:var(--vg-text-secondary);color:var(--vg-surface)}
.sr{position:absolute;width:1px;height:1px;overflow:hidden;clip:rect(0 0 0 0);white-space:nowrap}
.search{margin:12px 16px;min-height:44px;border-radius:12px;background:var(--vg-surface-sunken);display:flex;align-items:center;gap:8px;padding:0 12px;color:var(--vg-text-placeholder)}
.btn{min-height:44px;border-radius:12px;border:0;font:600 16px/24px inherit;padding:0 20px;display:inline-flex;align-items:center;justify-content:center;gap:8px;background:var(--vg-primary);color:var(--vg-on-primary);cursor:pointer;font-family:inherit}
.btn.sec{background:var(--vg-primary-subtle);color:var(--vg-link)}.btn.ghost{background:transparent;color:var(--vg-link)}.btn.danger{background:transparent;color:var(--vg-danger);border:1px solid var(--vg-danger)}
.btn.grad{background:var(--vg-gradient-action);color:#020B2A}.btn.block{width:100%}.btn.sm{min-height:36px;font-size:14px;padding:0 14px}
.field{flex-wrap:wrap;min-height:48px;row-gap:4px;border-radius:12px;background:var(--vg-surface);border:1px solid var(--vg-border-strong);display:flex;align-items:center;padding:0 14px;gap:10px}
.field:focus-within,.search:focus-within{outline:2px solid var(--vg-focus-ring);outline-offset:2px}.tlink{display:inline-flex;align-items:center;justify-content:center;min-height:44px;padding:0 8px}
.field>button{margin-left:auto}.field .inp{min-width:4em}.inputbar .field{flex-wrap:nowrap;min-width:0}.wraprow{flex-wrap:wrap;gap:8px 16px}
.field .ph{color:var(--vg-text-placeholder)}.field.focus{border:2px solid var(--vg-focus-ring);padding:0 13px}
.chip{display:inline-flex;align-items:center;gap:4px;min-height:24px;padding:0 10px;border-radius:12px;font-size:12px;font-weight:600;background:var(--vg-primary-subtle);color:var(--vg-link)}
.chip.ok{background:transparent;border:1px solid var(--vg-success);color:var(--vg-success)}.chip.warn{background:transparent;border:1px solid var(--vg-warning);color:var(--vg-warning)}.chip.bad{background:transparent;border:1px solid var(--vg-danger);color:var(--vg-danger)}
.card{background:var(--vg-surface);border:1px solid var(--vg-border);border-radius:20px;padding:16px}
.toggle{width:48px;height:28px;border-radius:14px;background:var(--vg-border-strong);position:relative;flex:none}.toggle::after{content:"";position:absolute;top:3px;left:3px;width:22px;height:22px;border-radius:50%;background:var(--vg-surface)}
.toggle.on{background:var(--vg-primary)}.toggle.on::after{left:23px;background:var(--vg-on-primary)}
.phone>.msgs{flex-shrink:1}
.msgs{padding:12px 12px 8px;display:flex;flex-direction:column;gap:8px;flex:1;min-height:0;overflow:auto}
.compact .li{min-height:56px;padding:8px 16px}.compact .sec-title{padding:10px 16px 4px}
.b{max-width:78%;padding:8px 12px;border-radius:18px;font-size:16px;line-height:22px;position:relative}
.b.in{align-self:flex-start;background:var(--vg-bubble-in);color:var(--vg-on-bubble-in);border:1px solid var(--vg-border);border-bottom-left-radius:4px}
.b.out{align-self:flex-end;background:var(--vg-bubble-out);color:var(--vg-on-bubble-out);border-bottom-right-radius:4px}
.b .meta{font-size:11px;line-height:14px;margin-top:2px;display:flex;gap:4px;justify-content:flex-end;align-items:center}
.b.in .meta{color:var(--vg-on-bubble-in-secondary)}.b.out .meta{color:var(--vg-on-bubble-out-secondary)}
.quote{border-left:3px solid currentColor;padding:2px 8px;margin-bottom:6px;font-size:13px;line-height:18px;opacity:.9}
.sys{align-self:center;font-size:12px;color:var(--vg-text-secondary);background:var(--vg-surface-sunken);padding:4px 12px;border-radius:12px}
.inputbar{display:flex;align-items:center;gap:6px;padding:8px 8px 12px;background:var(--vg-surface);border-top:1px solid var(--vg-border)}
.inputbar .field{flex:1;min-height:44px;border-radius:22px}
.skel{background:linear-gradient(90deg,var(--vg-surface-sunken),var(--vg-border),var(--vg-surface-sunken));border-radius:8px;height:14px}
.sec-title{padding:16px 16px 8px;font-size:13px;line-height:18px;font-weight:600;color:var(--vg-text-secondary)}
.ph-img{background:linear-gradient(135deg,var(--vg-brand-cyan-400),var(--vg-brand-violet-500));opacity:.85;border-radius:12px}
/* desktop / manager */
.desk{display:flex;min-height:100vh}
.rail{width:72px;background:var(--vg-surface-sunken);border-right:1px solid var(--vg-border);display:flex;flex-direction:column;align-items:center;padding:12px 0;gap:6px}
.rail .iconbtn.sel{background:var(--vg-primary-subtle);color:var(--vg-primary)}
.side{width:248px;background:var(--vg-surface);border-right:1px solid var(--vg-border);padding:16px 12px;display:flex;flex-direction:column;gap:2px}
.nav{min-height:40px;border-radius:10px;display:flex;align-items:center;gap:12px;padding:0 12px;color:var(--vg-text-secondary);font-weight:500;font-size:14px}
.nav.sel{background:var(--vg-primary-subtle);color:var(--vg-link);font-weight:700}
.main{flex:1;min-width:0;display:flex;flex-direction:column}
.page{padding:24px;display:flex;flex-direction:column;gap:16px}
table{width:100%;border-collapse:collapse;background:var(--vg-surface);border:1px solid var(--vg-border);border-radius:16px;overflow:hidden;font-size:14px}
th{text-align:left;padding:12px 16px;font-size:12px;color:var(--vg-text-secondary);background:var(--vg-surface-sunken);font-weight:600}
td{padding:12px 16px;border-top:1px solid var(--vg-border)}
.drawer{width:380px;background:var(--vg-surface);border-left:1px solid var(--vg-border);padding:20px;display:flex;flex-direction:column;gap:16px;box-shadow:var(--vg-elev-e3)}
.kv{display:flex;justify-content:space-between;font-size:14px;padding:8px 0;border-top:1px solid var(--vg-border)}
.mono{font-family:'JetBrains Mono',ui-monospace,monospace;font-size:13px}
.banner{display:flex;align-items:center;gap:8px;padding:8px 16px;font-size:13px;font-weight:600}
.banner.warn{background:var(--vg-primary-subtle);color:var(--vg-text-primary)}
.hero{background:radial-gradient(1200px 500px at 70% -10%,var(--vg-brand-ink-900),var(--vg-brand-ink-950));color:#EAF1FF;padding:72px 80px 96px}
.grad-text{background:var(--vg-gradient-brand);-webkit-background-clip:text;background-clip:text;color:transparent}
"""

AV = ['#1F6FEB', '#7A3FE0', '#0E7A8A', '#B3447A', '#3B7D2A', '#A15C07']
def av(i, ch, cls=''): return f'<div class="avatar {cls}" style="background:{AV[i % 6]}">{ch}</div>'

ICONS = {i['name']: i for i in json.load(open(os.path.join(ROOT, 'brand', 'icons', 'ui', 'icons.json'), encoding='utf8'))['icons']}
def semantic(body, lang):
    lab = lambda n: ICONS.get(n, {}).get('label_en' if lang.startswith('en') else 'label_zh', n)
    # icon-only controls -> real buttons with accessible names
    body = re.sub(r'<div class="iconbtn([^"]*)"([^>]*)>(<svg[^>]*><use href="#vg-([\w-]+)"/></svg>)</div>',
                  lambda m: f'<button type="button" class="iconbtn{m.group(1)}"{m.group(2)} aria-label="{lab(m.group(4))}">{m.group(3)}</button>', body)
    # placeholder-only fields -> real inputs
    body = re.sub(r'<div class="field([^"]*)"([^>]*)><span class="ph">([^<]*)</span>', lambda m: f'<div class="field{m.group(1)}"{m.group(2)}><input class="inp" placeholder="{m.group(3)}" aria-label="{m.group(3)}">', body)
    body = re.sub(r'<div class="search"([^>]*)>(<svg[^>]*>.*?</svg>)<span>([^<]*)</span></div>', lambda m: f'<div class="search" role="search"{m.group(1)}>{m.group(2)}<input class="inp" placeholder="{m.group(3)}" aria-label="{m.group(3)}"></div>', body)
    body = re.sub(r'<div class="sec-title">(.*?)</div>', r'<h2 class="sec-title">\1</h2>', body)
    body = body.replace('<div class="msgs"', '<div class="msgs" tabindex="0" role="log" aria-label="消息列表"').replace('<div class="msgs" tabindex="0" role="log" aria-label="消息列表" style', '<div class="msgs" tabindex="0" role="log" aria-label="消息列表" style')
    body = body.replace('<div class="banner', '<div role="status" class="banner')
    body = body.replace('<th>', '<th scope="col">').replace('<div class="avatar', '<div aria-hidden="true" class="avatar')
    return body
def page(title, body, extra_css='', lang='zh-CN'):
    css = re.sub(r'(font-size|line-height):\s*(\d+(?:\.\d+)?)px', r'\1:calc(\2px*var(--ts,1))', CSS + extra_css)
    body = re.sub(r'(font-size|line-height):\s*(\d+(?:\.\d+)?)px', r'\1:calc(\2px*var(--ts,1))', semantic(body, lang))
    return (f'<!doctype html><html lang="{lang}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">'
            f'<title>{title}</title><style>{css}</style></head><body>{SPRITE}<main>{body}</main>'
            '<script>const q=new URLSearchParams(location.search);document.documentElement.dataset.theme=q.get("theme")||"light";document.documentElement.style.setProperty("--ts",q.get("ts")||1)</script></body></html>')

def tabbar(sel):
    items = [('chat', '会话', 3), ('contacts', '联系人', 0), ('moments', '动态', 0), ('discover', '发现', 0), ('me', '我', 0)]
    return '<nav class="tabbar" aria-label="主导航">' + ''.join(
        f'<a href="#" class="tab {"sel" if n == sel else ""}"{" aria-current=page" if n == sel else ""}>{ic(n)}<span>{l}</span>{f"<span class=dot aria-label=未读{b}条>{b}</span>" if b and n != sel else ""}</a>' for n, l, b in items) + '</nav>'
status = '<div class="status"><span>9:41</span><span class="cap">100%</span></div>'

pages = {}
# 1 login ---------------------------------------------------------------------------------------------------------
pages['login'] = (390, 844, f'''<div class="phone">{status}<div style="padding:24px;display:flex;flex-direction:column;gap:20px;flex:1">
<img src="{MARK}" width="72" height="72" alt="VeyroGram"><div><h1 style="font-size:28px;line-height:36px;margin:0">登录 VeyroGram</h1><p class="muted" style="margin:6px 0 0">使用手机号验证码登录，新设备需要额外确认。</p></div>
<div class="col gap3"><div class="field"><span>+86</span>{ic('chevron-down',16)}<span style="width:1px;height:20px;background:var(--vg-border-strong)"></span><input class="inp" value="138 0013 8000" aria-label="手机号" inputmode="tel" autocomplete="tel"></div>
<div class="field focus"><input class="inp" placeholder="6 位验证码" aria-label="6 位验证码" inputmode="numeric" autocomplete="one-time-code"><button type="button" style="color:var(--vg-link);font-weight:600;white-space:nowrap">重新获取 (52s)</button></div>
<div class="row gap2 sm" style="color:var(--vg-info)">{ic('info',16)}<span>验证码已发送到你的手机</span></div></div>
<button type="button" class="btn grad block">登录</button>
<div class="row wraprow" style="justify-content:space-between"><a href="#" style="color:var(--vg-link);font-weight:600">使用密码登录</a><a href="#" style="color:var(--vg-link);font-weight:600">使用 Passkey</a></div>
<div class="grow"></div><div class="row gap2 sm muted">{ic('check',16)}<span>登录即表示你同意《服务条款》与《隐私政策》</span></div></div></div>''')
# 2 chat list ----------------------------------------------------------------------------------------------------
convs = [('产品设计组', '林夏: 新版图标已经上传', '09:40', 3, 0, 1, 0), ('周聿', '[语音] 12"', '09:12', 1, 1, 0, 0), ('客服 · 官方', '您好，已为您转接专属客服', '昨天', 0, 2, 0, 0), ('家人群', '妈妈: 周末回家吃饭', '昨天', 12, 3, 0, 1), ('陈岚', '会议改到下午三点', '周二', 0, 4, 0, 0), ('开发者通知', '你的小程序审核已通过', '周一', 1, 5, 0, 0), ('吴迪', '收到，谢谢', '11/12', 0, 0, 0, 0)]
rows = ''
for i, (n, m, t, u, c, pin, mute) in enumerate(convs):
    rows += f'<div class="li">{av(c, n[0])}<div class="grow"><div class="row"><span class="t ell grow">{n}</span><span class="cap muted">{t}</span></div><div class="row gap2"><span class="sm muted ell grow">{m}</span>{ic("pin",16,"muted") if pin else ""}{ic("bell-off",16,"muted") if mute else ""}{f"<span class=badge{chr(32)+chr(109)+chr(117)+chr(116)+chr(101) if mute else chr(34)}>{u}</span>" if u else ""}</div></div></div>'
rows = rows.replace('class=badge mute>', 'class="badge mute">').replace('class=badge">', 'class="badge">')
pages['chat-list'] = (390, 844, f'''<div class="phone">{status}<div class="topbar" style="background:var(--vg-bg-canvas);border:0"><h1 style="font-size:28px;line-height:36px;flex:1">会话</h1><div class="iconbtn">{ic('plus')}</div></div>
<div class="search">{ic('search',20)}<span>搜索联系人、群组、文件</span></div><div>{rows}</div>{tabbar('chat')}</div>''')
# 3 chat ---------------------------------------------------------------------------------------------------------
pages['chat'] = (390, 844, f'''<div class="phone">{status}<div class="topbar"><div class="iconbtn" style="margin-left:-12px">{ic('back')}</div>{av(1,'周','sm')}<div class="grow"><h1>周聿</h1><div class="cap muted">在线</div></div><div class="iconbtn">{ic('call')}</div><div class="iconbtn">{ic('more')}</div></div>
<div class="msgs"><div class="sys">今天 09:05</div>
<div class="b in">下午的评审你能参加吗？<div class="meta">09:05</div></div>
<div class="b out">可以，我把图标稿先发给你<div class="meta">09:06 {ic('check-double',14)}</div></div>
<div class="b out" style="padding:6px"><div class="ph-img" style="width:180px;height:100px"></div><div class="meta" style="padding:0 6px">09:06 {ic('check-double',14)}</div></div>
<div class="b in"><div class="quote">可以，我把图标稿先发给你</div>收到，看起来不错<div class="meta">09:08</div></div>
<div class="b in row gap3" style="padding:10px 14px">{ic('mic',20)}<div style="width:110px;height:4px;border-radius:2px;background:var(--vg-text-secondary);opacity:.6"></div><span class="sm">12"</span></div>
<div class="b out" style="width:250px"><div class="row gap3" style="padding:4px 0">{ic('file',32)}<div><div style="font-weight:600">图标规范-v2.pdf</div><div class="cap" style="opacity:.85">2.4 MB · 已扫描</div></div></div><div class="meta">09:10 {ic('check-double',14)}</div></div>
<div class="b in" style="width:250px;padding:6px"><div class="ph-img" style="height:90px;background:linear-gradient(135deg,var(--vg-surface-sunken),var(--vg-border));opacity:1;display:grid;place-items:center;color:var(--vg-primary)">{ic('location',36)}</div><div style="padding:6px 6px 2px;font-weight:600">上海 · 徐汇滨江</div><div class="meta">09:12</div></div>
<div class="sys">对方撤回了一条消息</div>
<div class="b in sm" style="background:var(--vg-surface-sunken);color:var(--vg-text-secondary);border-style:dashed">{ic('alert',16)} 当前版本不支持该消息，请升级应用<div class="meta">09:13</div></div></div>
<div style="margin-top:auto" class="inputbar"><div class="iconbtn">{ic('attach')}</div><div class="field"><span class="ph">输入消息</span><span class="grow"></span>{ic('sticker',22)}</div><div class="iconbtn" style="background:var(--vg-primary);color:var(--vg-on-primary)">{ic('send')}</div></div></div>''')
# 4 group settings -----------------------------------------------------------------------------------------------
def tog(label, on): return f'<button type="button" role="switch" aria-checked="{"true" if on else "false"}" aria-label="{label}" class="toggle {"on" if on else ""}"></button>'
def trow(icn, t, sub, on): return f'<div class="li" style="min-height:64px">{ic(icn,22,"muted")}<div class="grow"><div>{t}</div><div class="cap muted">{sub}</div></div>{tog(t, on)}</div>'
pages['group-settings'] = (390, 844, f'''<div class="phone compact">{status}<div class="topbar"><div class="iconbtn" style="margin-left:-12px">{ic('back')}</div><h1>群管理</h1></div>
<div class="card" style="margin:16px;display:flex;gap:16px;align-items:center">{av(0,'设','lg')}<div class="grow"><div style="font-weight:700;font-size:18px">产品设计组</div><div class="sm muted">128 位成员 · 你是群主</div></div>{ic('chevron-right',20,'muted')}</div>
<div class="sec-title">成员权限</div>{trow('lock','全员禁言','仅群主和管理员可发言',0)}{trow('shield','入群邀请需确认','成员邀请需管理员审核',1)}{trow('users-admin','禁止群内互加好友','隐藏来自本群的好友申请入口',1)}{trow('clock','新成员可查看历史消息','关闭后仅可查看入群后的消息',0)}
<div class="sec-title">管理</div><div class="li">{ic('users-admin',22,'muted')}<span class="grow">群管理员</span><span class="muted">3 人</span>{ic('chevron-right',18,'muted')}</div><div class="li">{ic('flag',22,'muted')}<span class="grow">群黑名单</span><span class="muted">2 人</span>{ic('chevron-right',18,'muted')}</div><div class="li">{ic('audit',22,'muted')}<span class="grow">管理操作记录</span>{ic('chevron-right',18,'muted')}</div>
<div style="padding:24px 16px;margin-top:auto"><button class="btn danger block">退出群聊</button></div></div>''')
# 5 moments publish ----------------------------------------------------------------------------------------------
cells = ''.join(f'<div style="position:relative;aspect-ratio:1" class="ph-img"><span class="cap" style="position:absolute;top:6px;left:8px;background:#0008;color:#fff;border-radius:8px;padding:0 6px">{i}</span></div>' for i in range(1, 8))
pages['moments-publish'] = (390, 844, f'''<div class="phone compact">{status}<div class="topbar"><div class="iconbtn" style="margin-left:-12px">{ic('close')}</div><h1 class="grow">发布动态</h1><button class="btn sm">发布</button></div>
<div style="padding:16px"><div class="sm muted" style="min-height:36px">这一刻的想法…</div>
<div style="display:grid;grid-template-columns:repeat(3,1fr);gap:8px;margin-top:8px">{cells}<div style="aspect-ratio:1;border:1.5px dashed var(--vg-border-strong);border-radius:12px;display:grid;place-items:center;color:var(--vg-text-secondary)">{ic('plus',28)}</div></div><div class="cap muted" style="margin-top:8px">已选 7/9 张，可长按拖动，或选中图片后用按钮调整顺序</div><div class="row gap2" style="margin-top:8px"><button type="button" class="btn sec sm" aria-label="将第 3 张前移">{ic('chevron-up',16)}前移</button><button type="button" class="btn sec sm" aria-label="将第 3 张后移">{ic('chevron-down',16)}后移</button></div></div>
<div class="li">{ic('location',22,'muted')}<span class="grow">所在位置</span><span class="muted">不显示</span>{ic('chevron-right',18,'muted')}</div>
<div class="li">{ic('at' if False else 'contacts',22,'muted')}<span class="grow">提醒谁看</span><span class="muted">未选择</span>{ic('chevron-right',18,'muted')}</div>
<div class="li">{ic('eye',22,'muted')}<div class="grow"><div>谁可以看</div><div class="cap muted">部分可见 · 标签：同事（18 人）</div></div>{ic('chevron-right',18,'muted')}</div>
<div class="banner warn" style="margin:12px 16px;border-radius:12px">{ic('info',18)}<span>被排除的朋友看不到这条动态，也看不到它的点赞和评论。</span></div></div>''')
# 6 call ---------------------------------------------------------------------------------------------------------
tiles = ''.join(f'<div style="border-radius:20px;background:linear-gradient(160deg,#10225A,#050F33);position:relative;display:grid;place-items:center">{av(i,n,"lg")}<span class="cap" style="position:absolute;left:12px;bottom:10px;background:#0008;padding:2px 8px;border-radius:10px;color:#fff">{nm}</span></div>' for i, (n, nm) in enumerate([('陈', '陈岚'), ('周', '周聿'), ('吴', '吴迪'), ('我', '我')]))
pages['call'] = (390, 844, f'''<div class="phone" style="background:#020B2A;color:#EAF1FF;--vg-text-secondary:#A5B2DA"><h1 class="sr">语音通话：产品周会</h1>{status}<div class="banner" style="background:#0F2A66;margin:0 16px;border-radius:12px">{ic('offline',18)}<span>网络较弱，已自动降低视频画质</span></div>
<div style="padding:16px;display:grid;grid-template-columns:1fr 1fr;gap:12px;flex:1">{tiles}</div><div class="cap" style="text-align:center;color:#A5B2DA">产品周会 · 04:12 · 4 人</div>
<div class="row" style="justify-content:center;gap:20px;padding:20px 0 36px"><div class="iconbtn" style="background:#10225A;color:#EAF1FF;width:56px;height:56px;border-radius:28px">{ic('mic-off',26)}</div><div class="iconbtn" style="background:#10225A;color:#EAF1FF;width:56px;height:56px;border-radius:28px">{ic('video',26)}</div><div class="iconbtn" style="background:#10225A;color:#EAF1FF;width:56px;height:56px;border-radius:28px">{ic('screen-share',26)}</div><div class="iconbtn" style="background:#FF7A80;color:#020B2A;width:56px;height:56px;border-radius:28px">{ic('call-end',26)}</div></div></div>''')
# 7 desktop 3-pane -----------------------------------------------------------------------------------------------
rail = ''.join(f'<div class="iconbtn {"sel" if n == "chat" else ""}" title="{l}">{ic(n)}</div>' for n, l in [('chat', '会话'), ('contacts', '联系人'), ('moments', '动态'), ('discover', '发现')])
pages['desktop'] = (1440, 900, f'''<div class="desk"><div class="rail"><div style="margin-bottom:12px"><img src="{MARK}" width="40" height="40" alt="VeyroGram"></div>{rail}<div class="grow"></div><div class="iconbtn">{ic('settings')}</div></div>
<div style="width:340px;border-right:1px solid var(--vg-border);background:var(--vg-surface)"><div class="search" style="margin:16px">{ic('search',20)}<span>搜索（Ctrl K）</span></div>{rows}</div>
<div class="main" style="background:var(--vg-bg-canvas)"><div class="topbar" style="height:64px">{av(1,'周','sm')}<div class="grow"><h1>周聿</h1><div class="cap muted">在线</div></div><div class="iconbtn">{ic('call')}</div><div class="iconbtn">{ic('video')}</div><div class="iconbtn">{ic('search')}</div><div class="iconbtn">{ic('more')}</div></div>
<div class="msgs" style="padding:24px 48px;flex:1"><div class="sys">今天 09:05</div><div class="b in">下午的评审你能参加吗？<div class="meta">09:05</div></div><div class="b out">可以，我把图标稿先发给你<div class="meta">09:06 {ic('check-double',14)}</div></div><div class="b out" style="width:300px"><div class="row gap3" style="padding:4px 0">{ic('file',32)}<div><div style="font-weight:600">图标规范-v2.pdf</div><div class="cap" style="opacity:.85">2.4 MB · 已扫描</div></div></div><div class="meta">09:10 {ic('check-double',14)}</div></div></div>
<div class="inputbar" style="padding:12px 24px"><div class="iconbtn">{ic('attach')}</div><div class="iconbtn">{ic('sticker')}</div><div class="field"><span class="ph">输入消息，Enter 发送，Shift+Enter 换行</span></div><div class="iconbtn" style="background:var(--vg-primary);color:var(--vg-on-primary)">{ic('send')}</div></div></div>
<div style="width:320px;border-left:1px solid var(--vg-border);background:var(--vg-surface);padding:24px;display:flex;flex-direction:column;align-items:center;gap:12px">{av(1,'周','lg')}<div style="font-weight:700;font-size:18px">周聿</div><div class="sm muted">备注：产品设计</div><div class="row gap2"><span class="chip">同事</span><span class="chip">设计</span></div>
<div style="width:100%"><div class="kv"><span class="muted">共同群组</span><span>3 个</span></div><div class="kv"><span class="muted">消息免打扰</span>{tog("消息免打扰", 0)}</div><div class="kv"><span class="muted">已读回执</span>{tog("已读回执", 1)}</div></div></div></div>''')
# 8 manager ------------------------------------------------------------------------------------------------------
nav = ''.join(f'<div class="nav {"sel" if n == "users-admin" else ""}">{ic(n,20)}<span>{l}</span></div>' for n, l in [('dashboard', '概览'), ('users-admin', '用户与设备'), ('group', '群与消息'), ('flag', '举报与审核'), ('support', '客服与工单'), ('image', '媒体'), ('megaphone', '运营'), ('wallet', '订阅'), ('miniapp', '开发者与小程序'), ('chart', '报表'), ('audit', '审计日志'), ('key', '权限'), ('settings', '系统配置')])
urows = ''.join(f'<tr><td><div class="row gap3">{av(i,n[0],"sm")}<div><div style="font-weight:600">{n}</div><div class="cap muted">{p}</div></div></div></td><td>{d}</td><td><span class="chip {c}">{s}</span></td><td class="muted">{t}</td><td style="text-align:right">{ic("more",20,"muted")}</td></tr>' for i, (n, p, d, s, c, t) in enumerate([('林夏', '138****8000', '3 台 · iOS / Mac', '正常', 'ok', '5 分钟前'), ('周聿', '139****2231', '2 台 · Android', '正常', 'ok', '1 小时前'), ('吴迪', '137****5509', '1 台 · Web', '待复核', 'warn', '昨天'), ('陈岚', '136****7712', '4 台 · Win / iOS', '已封禁', 'bad', '11/10'), ('孙敏', '135****0098', '2 台 · Android', '正常', 'ok', '11/17')]))
pages['manager'] = (1440, 900, f'''<div class="desk"><div class="side"><div class="row gap3" style="padding:4px 8px 16px"><img src="{MARK}" width="36" height="36" alt="VeyroGram"><div style="font-weight:700">VeyroGram 管理后台</div></div>{nav}</div>
<div class="main"><div class="topbar" style="height:64px"><h1 class="grow">用户与设备</h1><div class="field" style="height:40px;width:280px">{ic('search',18)}<span class="ph">搜索手机号 / 用户 ID</span></div><span class="chip">运营 · 张静</span></div>
<div class="row" style="flex:1;align-items:stretch"><div class="page grow"><div class="row gap3"><span class="chip">全部 12,408</span><span class="chip ok">正常</span><span class="chip warn">待复核 23</span><span class="chip bad">已封禁 41</span><span class="grow"></span><button class="btn sec sm">{ic('download',16)}导出（需审批）</button></div>
<table><tr><th>用户</th><th>设备</th><th>状态</th><th>最近活跃</th><th><span class="sr">操作</span></th></tr>{urows}</table><div class="cap muted">手机号默认脱敏；查看明文或聊天记录需填写理由，并写入审计日志。</div></div>
<div class="drawer"><div class="row gap3">{av(2,'吴','lg')}<div class="grow"><div style="font-weight:700;font-size:18px">吴迪</div><span class="chip warn">待复核</span></div></div>
<div><div class="kv"><span class="muted">用户 ID</span><span class="mono">u_8f3a…c21</span></div><div class="kv"><span class="muted">注册时间</span><span>2026-09-03</span></div><div class="kv"><span class="muted">风险事件</span><span>2 条（异地登录、批量加友）</span></div></div>
<div style="font-weight:600">设备</div><div class="li" style="border:1px solid var(--vg-border);border-radius:12px;min-height:56px">{ic('device',22,'muted')}<div class="grow"><div class="sm">Chrome · Web</div><div class="cap muted">杭州 · 昨天 21:08</div></div><button class="btn danger sm">下线</button></div>
<div class="banner warn" style="border-radius:12px">{ic('lock',18)}<span>敏感操作将记录操作人、对象与理由</span></div><div class="row gap2"><button class="btn danger grow">封禁（需审批）</button><button class="btn sec grow">查看聊天记录</button></div></div></div></div></div>''')
# 9 developer console --------------------------------------------------------------------------------------------
pages['developer'] = (1440, 900, f'''<div class="desk"><div class="side"><div class="row gap3" style="padding:4px 8px 16px"><img src="{MARK}" width="36" height="36" alt="VeyroGram"><div style="font-weight:700">开发者平台</div></div>
{''.join(f'<div class="nav {"sel" if n == "miniapp" else ""}">{ic(n,20)}<span>{l}</span></div>' for n, l in [('miniapp', '应用清单'), ('settings', '应用配置'), ('key', '密钥'), ('shield', '授权设置'), ('clock', '版本与审核'), ('chart', '统计'), ('group', '团队')])}</div>
<div class="main"><div class="topbar" style="height:64px"><h1 class="grow">轻松记账 · 小程序</h1><span class="chip ok">已发布 v1.2.0</span></div>
<div class="page"><div class="row gap4" style="align-items:stretch"><div class="card grow"><div style="font-weight:700;margin-bottom:12px">凭据</div><div class="kv"><span class="muted">AppID</span><span class="mono">vg_app_7c2e91a4</span></div>
<div class="kv" style="align-items:center"><span class="muted">App Secret</span><span class="mono">sk_live_••••••••••••••••3f9a</span><button class="btn sec sm">{ic('refresh',16)}轮换</button></div><div class="banner warn" style="border-radius:12px;margin-top:8px">{ic('info',18)}<span>Secret 只在创建或轮换时显示一次，请立即保存；平台只保存其哈希。</span></div></div>
<div class="card grow"><div style="font-weight:700;margin-bottom:12px">已申请权限</div>{''.join(f'<div class="kv" style="align-items:center"><div><div>{a}</div><div class="cap muted">{b}</div></div><span class="chip {c}">{d}</span></div>' for a, b, c, d in [('openid', '识别用户', 'ok', '低'), ('profile.basic', '显示昵称与头像', 'ok', '低'), ('location.once', '为你推荐附近商户（单次）', 'bad', '高')])}</div></div>
<div class="card"><div class="row" style="margin-bottom:12px"><div style="font-weight:700" class="grow">版本</div><button class="btn danger sm">回滚到 v1.1.4</button></div>
{''.join(f'<div class="li" style="border-radius:12px;border:1px solid var(--vg-border);margin-bottom:8px;min-height:60px"><span class="mono">{v}</span><div class="grow"><div class="sm">{n}</div><div class="cap muted">{d}</div></div><span class="chip {c}">{s}</span></div>' for v, n, d, s, c in [('v1.2.0', '新增账单导出', '风险扫描通过 · 人工审核通过', '灰度 10%', 'warn'), ('v1.1.4', '修复统计偏差', '2026-11-02 发布', '全量', 'ok'), ('v1.1.3', '界面调整', '已被 v1.1.4 取代', '已下线', '')])}</div></div></div></div>''')
# 10 website -----------------------------------------------------------------------------------------------------
pages['website'] = (1440, 900, f'''<div class="hero"><div class="row" style="justify-content:space-between"><div class="row gap3"><img src="{MARK}" width="44" height="44" alt="VeyroGram"><b style="font-size:20px">VeyroGram</b></div><div class="row gap4 sm" style="color:#A5B2DA"><span>功能</span><span>安全与隐私</span><span>下载</span><span>开发者</span><span>帮助</span><span class="row gap2">{ic('globe',18)}简体中文</span></div></div>
<div style="max-width:820px;margin-top:96px"><div class="chip" style="background:#0F2A66;color:#6FD0FF">v1.0 公测筹备中</div><h1 style="font-size:64px;line-height:72px;margin:20px 0 16px">为每一块屏幕<br>而生的<span class="grad-text">即时通讯</span></h1><p style="font-size:20px;line-height:32px;color:#A5B2DA;max-width:640px">手机、网页与桌面共用同一套会话；文件、位置、动态、通话与开放平台在同一个产品里协同。</p>
<div class="row gap3" style="margin-top:32px"><button class="btn grad" style="height:52px;padding:0 28px">{ic('download',20)}下载应用</button><button class="btn" style="height:52px;padding:0 28px;background:#10225A;color:#EAF1FF">开发者文档</button></div></div></div>
<div style="padding:64px 80px;display:grid;grid-template-columns:repeat(3,1fr);gap:20px">{''.join(f'<div class="card"><div class="iconbtn" style="background:var(--vg-primary-subtle);color:var(--vg-primary);margin-bottom:12px">{ic(i,26)}</div><div style="font-weight:700;font-size:18px">{t}</div><p class="muted sm" style="margin:6px 0 0">{d}</p></div>' for i, t, d in [('chat', '跨端会话', '多设备同步，离线消息自动补齐，草稿跟着你走。'), ('group', '群组管理', '角色、禁言、邀请审核与审计记录，权限由服务端执行。'), ('miniapp', '开放平台', '开发者可提交、审核并灰度发布小程序，随时可回滚。')])}</div>''')
# 11 states ------------------------------------------------------------------------------------------------------
pages['states'] = (390, 1400, f'''<div class="phone compact" style="height:auto;overflow:visible">{status}<div class="topbar"><h1>状态集</h1></div>
<div class="sec-title">加载（骨架屏）</div>{''.join(f'<div class="li"><div class="skel" style="width:48px;height:48px;border-radius:50%"></div><div class="grow col gap2"><div class="skel" style="width:{60+i*8}%"></div><div class="skel" style="width:40%"></div></div></div>' for i in range(3))}
<div class="sec-title">空状态</div><div class="card" style="margin:0 16px;text-align:center;padding:32px 16px"><div style="color:var(--vg-primary)">{ic('moments',48)}</div><div style="font-weight:700;margin:8px 0 4px">还没有动态</div><div class="sm muted">好友发布的动态会出现在这里</div><button class="btn sec" style="margin-top:16px">发布第一条</button></div>
<div class="sec-title">错误</div><div class="card" style="margin:0 16px;text-align:center;padding:32px 16px"><div style="color:var(--vg-danger)">{ic('alert',48)}</div><div style="font-weight:700;margin:8px 0 4px">加载失败</div><div class="sm muted">请检查网络后重试（错误码 NET-1102）</div><button class="btn" style="margin-top:16px">{ic('refresh',18)}重试</button></div>
<div class="sec-title">离线</div><div class="banner warn">{ic('offline',18)}<span>当前离线，消息会在恢复网络后自动发送</span></div>
<div class="sec-title">无权限</div><div class="card" style="margin:0 16px;text-align:center;padding:32px 16px"><div class="muted">{ic('lock',48)}</div><div style="font-weight:700;margin:8px 0 4px">仅群主和管理员可以查看</div><div class="sm muted">如需访问，请联系群管理员</div></div>
<div class="sec-title">降级</div><div class="b in sm" style="margin:0 16px;background:var(--vg-surface-sunken);color:var(--vg-text-secondary);border-style:dashed;display:block;max-width:none">{ic('alert',16)} 当前版本不支持该消息，请升级应用</div></div>''')

# ===================== additional flows (device approval, QR login, mini-app consent/market, support workbench, read detail, edit, settings, tablet) =====================
def kv(a, b): return f'<div class="kv"><span class="muted">{a}</span><span>{b}</span></div>'
def li(icn, t, right='', sub=''): return f'<div class="li" style="min-height:56px">{ic(icn,22,"muted")}<div class="grow"><div>{t}</div>{f"<div class=cap>{sub}</div>" if sub else ""}</div><span class="muted">{right}</span>{ic("chevron-right",18,"muted")}</div>'
def bar(title, right=''): return f'<div class="topbar"><button type="button" class="iconbtn" style="margin-left:-12px" aria-label="返回">{ic("back")}</button><h1 class="grow">{title}</h1>{right}</div>'
pages['device-approve'] = (390, 844, f'''<div class="phone">{status}{bar("新设备登录请求")}
<div class="card" style="margin:16px"><div class="row gap3" style="margin-bottom:8px"><div class="iconbtn" style="background:var(--vg-primary-subtle);color:var(--vg-primary)">{ic('device',26)}</div><div><div style="font-weight:700">Chrome · Web</div><div class="cap muted">正在请求登录你的账号</div></div></div>
{kv('大致位置','杭州（根据 IP 估算）')}{kv('请求时间','今天 21:08')}{kv('网络地址','223.104.*.*')}{kv('将获得的权限','同步会话与消息')}</div>
<div class="banner warn" style="margin:0 16px;border-radius:12px">{ic('alert',18)}<span>如果这不是你本人的操作，请拒绝。拒绝后建议立即修改密码。</span></div>
<div style="padding:24px 16px;display:flex;flex-direction:column;gap:12px;margin-top:auto"><button type="button" class="btn block">批准登录</button><button type="button" class="btn danger block">不是我，拒绝并保护账号</button><div class="cap muted" style="text-align:center">请求将在 4 分钟后失效</div></div></div>''')
pages['devices'] = (390, 844, f'''<div class="phone compact">{status}{bar("登录设备")}
<div class="sec-title">此设备</div><div class="li">{ic('device',24,'muted')}<div class="grow"><div>iPhone 15</div><div class="cap muted">上海 · 刚刚活跃</div></div><span class="chip ok">此设备</span></div>
<div class="sec-title">其他设备</div><div class="li">{ic('device',24,'muted')}<div class="grow"><div>Mac mini · 桌面端</div><div class="cap muted">上海 · 2 小时前</div></div><button type="button" class="btn danger sm">下线</button></div>
<div class="li">{ic('globe',24,'muted')}<div class="grow"><div>Chrome · Web</div><div class="cap muted">杭州 · 昨天 21:08</div></div><button type="button" class="btn danger sm">下线</button></div>
<div class="sec-title">设备锁</div><div class="li" style="min-height:64px">{ic('lock',22,'muted')}<div class="grow"><div>开启设备锁</div><div class="cap muted">在新设备登录需经已信任设备批准</div></div>{tog("开启设备锁",1)}</div>
<div class="banner warn" style="margin:16px;border-radius:12px">{ic('info',18)}<span>下线设备后，该设备的登录凭据会立即失效。</span></div></div>''')
import random as _r
_r.seed(7); qr = ''.join(f'<rect x="{x*10}" y="{y*10}" width="10" height="10"/>' for x in range(25) for y in range(25) if _r.random() > .52 and not ((x < 8 and y < 8) or (x > 16 and y < 8) or (x < 8 and y > 16)))
def finder(x, y): return f'<rect x="{x}" y="{y}" width="70" height="70" fill="none" stroke="currentColor" stroke-width="10"/><rect x="{x+20}" y="{y+20}" width="30" height="30"/>'
pages['qr-login-desktop'] = (1100, 700, f'''<div style="display:flex;min-height:100vh"><div class="hero" style="width:48%;padding:64px 56px"><div class="row gap3"><img src="{MARK}" width="44" height="44" alt="VeyroGram"><b style="font-size:20px">VeyroGram</b></div><h1 style="font-size:40px;line-height:48px;margin:64px 0 12px">在电脑上<br>使用 VeyroGram</h1><p style="color:#A5B2DA;margin:0">会话、文件与草稿与手机保持同步。</p></div>
<div style="flex:1;display:grid;place-items:center;background:var(--vg-bg-canvas)"><div style="width:360px;display:flex;flex-direction:column;align-items:center;gap:16px"><h2 style="font-size:24px;line-height:32px">扫码登录</h2>
<div class="card" style="padding:20px"><svg viewBox="0 0 250 250" width="220" height="220" role="img" aria-label="登录二维码（示意）" style="color:var(--vg-text-primary);fill:var(--vg-text-primary)">{qr}{finder(0,0)}{finder(180,0)}{finder(0,180)}</svg></div>
<ol class="sm muted" style="margin:0;padding-left:20px;align-self:stretch"><li>在手机上打开 VeyroGram</li><li>进入「发现」→「扫一扫」</li><li>在手机上确认登录</li></ol><div class="chip warn">二维码将在 48 秒后过期</div><button type="button" class="btn sec block">{ic('refresh',18)}刷新二维码</button><a href="#" style="color:var(--vg-link);font-weight:600">使用手机号验证码登录</a></div></div></div>''')
pages['qr-confirm'] = (390, 844, f'''<div class="phone">{status}{bar("登录确认")}<div style="padding:40px 24px;display:flex;flex-direction:column;align-items:center;gap:16px;text-align:center"><div class="iconbtn" style="background:var(--vg-primary-subtle);color:var(--vg-primary);width:80px;height:80px;border-radius:40px">{ic('globe',40)}</div><h2 style="font-size:24px;line-height:32px">登录 VeyroGram Web？</h2><p class="muted" style="margin:0">Chrome · macOS · 上海<br>请求时间 今天 21:12</p></div>
<div class="banner warn" style="margin:0 16px;border-radius:12px">{ic('alert',18)}<span>仅在是你本人在电脑上扫码时确认。</span></div><div style="padding:24px 16px;margin-top:auto;display:flex;flex-direction:column;gap:12px"><button type="button" class="btn block">确认登录</button><button type="button" class="btn ghost block">取消</button></div></div>''')
pages['miniapp-consent'] = (390, 844, f'''<div class="phone" style="position:relative"><div inert aria-hidden="true" style="opacity:.35;pointer-events:none">{status}<div class="topbar"><button type="button" class="iconbtn" aria-label="关闭">{ic('close')}</button><h1 class="grow">轻松记账</h1><button type="button" class="iconbtn" aria-label="更多">{ic('more')}</button></div><div class="card" style="margin:16px"><div class="skel" style="width:50%"></div><div class="skel" style="width:80%;margin-top:10px"></div></div></div>
<div style="position:absolute;left:0;right:0;bottom:0;background:var(--vg-surface);border-radius:28px 28px 0 0;padding:20px 20px 32px;box-shadow:var(--vg-elev-e3);display:flex;flex-direction:column;gap:14px" role="dialog" aria-modal="true" aria-labelledby="ct"><div class="row gap3"><div class="avatar" aria-hidden="true" style="background:#0E7A8A;border-radius:14px">记</div><div><h2 id="ct" style="font-size:18px;line-height:26px">轻松记账 申请使用你的信息</h2><div class="cap muted">开发者：杭州某某科技有限公司</div></div></div>
<div class="kv"><div><div>昵称与头像</div><div class="cap muted">用于显示你的账户</div></div><span class="chip ok">低风险</span></div><div class="kv"><div><div>单次位置</div><div class="cap muted">仅在你点击“附近商户”时获取一次</div></div><span class="chip bad">高风险</span></div>
<div class="cap muted">你可以随时在「我 → 已授权应用」撤销。此授权窗口由 VeyroGram 绘制，小程序无法修改。</div><div class="row gap3"><button type="button" class="btn sec grow">拒绝</button><button type="button" class="btn grow">允许</button></div><a href="#" class="cap tlink" style="color:var(--vg-link);font-weight:600">查看隐私政策</a></div></div>''')
pages['miniapp-market'] = (390, 844, f'''<div class="phone">{status}<div class="topbar" style="background:var(--vg-bg-canvas);border:0"><h1 style="font-size:28px;line-height:36px" class="grow">小程序</h1></div><div class="search" role="search">{ic('search',20)}<span>搜索小程序</span></div>
<div class="row gap2" style="padding:0 16px 8px;overflow:hidden"><span class="chip" style="background:var(--vg-primary);color:var(--vg-on-primary)">推荐</span><span class="chip">效率</span><span class="chip">生活</span><span class="chip">购物</span><span class="chip">学习</span></div>
{''.join(f'<div class="li">{av(i,n[0])}<div class="grow"><div class="t">{n}</div><div class="sm muted ell">{d}</div><div class="cap muted row gap2" style="margin-top:2px">{ic("star",14)}{r} · {u}</div></div><button type="button" class="btn sec sm">打开</button></div>' for i, (n, d, r, u) in enumerate([("轻松记账", "三十秒记一笔，月底自动汇总", "4.8", "12 万人使用"), ("团队排班", "班次、请假与提醒一站管理", "4.6", "3.4 万人使用"), ("读书笔记", "划线、摘录与回顾", "4.7", "8 万人使用"), ("附近咖啡", "找到步行 5 分钟内的好店", "4.5", "5.1 万人使用")]))}{tabbar('discover')}</div>''')
pages['support-workbench'] = (1440, 900, f'''<div class="desk"><div class="side" style="width:320px;padding:0"><div class="topbar" style="min-height:64px"><h1 class="grow">客服工作台</h1><span class="chip ok">在线</span></div><div class="row gap2" style="padding:12px 16px"><span class="chip" style="background:var(--vg-primary);color:var(--vg-on-primary)">我的会话 5</span><span class="chip">排队中 3</span><span class="chip">工单</span></div>
{''.join(f'<div class="li" style="min-height:76px">{av(i,n[0])}<div class="grow"><div class="row"><span class="t grow">{n}</span><span class="chip {c}">{s}</span></div><div class="sm muted ell">{m}</div></div></div>' for i, (n, m, s, c) in enumerate([("访客 #2814", "订阅为什么没有生效？", "SLA 02:10", "warn"), ("林夏", "文件一直显示待扫描", "SLA 08:40", "ok"), ("访客 #2809", "如何申请开发者认证？", "SLA 00:45", "bad")]))}</div>
<div class="main" style="background:var(--vg-bg-canvas)"><div class="topbar" style="min-height:64px">{av(0,'访','sm')}<div class="grow"><h1>访客 #2814</h1><div class="cap muted">来自官网 · 技能组：订阅与账单</div></div><button type="button" class="btn sec sm">转接</button><button type="button" class="btn sec sm">创建工单</button><button type="button" class="btn danger sm">结束会话</button></div>
<div class="msgs" style="padding:24px 48px"><div class="sys">客服欢迎词已发送</div><div class="b in">订阅为什么没有生效？我已经付款了。<div class="meta">21:02</div></div><div class="b out">您好，我先帮您核对订单状态，请稍等。<div class="meta">21:03 {ic('check-double',14)}</div></div></div>
<div class="row gap2" style="padding:8px 24px"><span class="chip">快捷回复：核对订单</span><span class="chip">快捷回复：稍后回复</span><span class="chip">知识库：订阅生效时间</span></div><div class="inputbar" style="padding:12px 24px"><div class="field"><input class="inp" placeholder="输入回复，/ 调出快捷回复" aria-label="输入回复"></div><button type="button" class="btn">发送</button></div></div>
<div class="drawer" style="width:340px"><div class="row gap3">{av(0,'访','lg')}<div><div style="font-weight:700">访客 #2814</div><div class="cap muted">未登录访客 · 首次咨询</div></div></div><div><div style="font-weight:600;margin-bottom:6px">用户标签</div><div class="row gap2"><span class="chip">订阅</span><span class="chip warn">待复核</span></div></div><div><div style="font-weight:600">历史工单</div><div class="kv"><span>#1203 订阅异常</span><span class="chip ok">已解决</span></div></div><div><div style="font-weight:600">知识库建议</div><div class="sm muted" style="margin-top:6px">《订阅生效时间与对账说明》</div></div><div class="banner warn" style="border-radius:12px">{ic('lock',18)}<span>会话内容仅限当前技能组查看，质检与导出会留痕。</span></div></div></div>''')
pages['message-read-detail'] = (390, 844, f'''<div class="phone" style="position:relative"><div inert aria-hidden="true" style="opacity:.35">{status}<div class="topbar"><h1 class="grow">产品设计组</h1></div><div class="msgs"><div class="b out">新版图标已经上传<div class="meta">09:40 {ic('check-double',14)}</div></div></div></div>
<div style="position:absolute;left:0;right:0;bottom:0;top:180px;background:var(--vg-surface);border-radius:28px 28px 0 0;box-shadow:var(--vg-elev-e3);display:flex;flex-direction:column" role="dialog" aria-modal="true" aria-labelledby="rt"><div style="padding:20px 20px 8px"><h2 id="rt" style="font-size:18px;line-height:26px">消息详情</h2><div class="cap muted">新版图标已经上传</div></div>
<div class="row gap2" style="padding:8px 20px"><span class="chip" style="background:var(--vg-primary);color:var(--vg-on-primary)">已读 5</span><span class="chip">未读 3</span><span class="chip">点赞 2</span></div>{''.join(f'<div class="li" style="min-height:60px">{av(i,n[0],"sm")}<div class="grow">{n}</div><span class="cap muted">{t}</span></div>' for i, (n, t) in enumerate([("林夏", "09:41"), ("周聿", "09:42"), ("陈岚", "09:50"), ("吴迪", "10:02"), ("孙敏", "10:15")]))}
<div class="banner warn" style="margin:12px 16px;border-radius:12px">{ic('info',18)}<span>仅显示已开启已读回执的成员。</span></div></div></div>''')
pages['message-edit'] = (390, 844, f'''<div class="phone">{status}<div class="topbar"><button type="button" class="iconbtn" style="margin-left:-12px" aria-label="返回">{ic('back')}</button>{av(1,'周','sm')}<div class="grow"><h1>周聿</h1></div></div>
<div class="msgs"><div class="b in">下午的评审你能参加吗？<div class="meta">09:05</div></div><div class="b out" style="outline:2px solid var(--vg-focus-ring);outline-offset:2px">可以，我把图标稿先发给你（已补充 v2）<div class="meta">已编辑 · 09:06 {ic('check-double',14)}</div></div></div>
<div class="banner warn">{ic('edit',18)}<span class="grow">正在编辑消息</span><button type="button" class="btn ghost sm">取消</button></div><div class="inputbar"><div class="field" style="border:2px solid var(--vg-focus-ring)"><input class="inp" value="可以，我把图标稿先发给你（已补充 v2）" aria-label="编辑消息"></div><button type="button" class="iconbtn" style="background:var(--vg-primary);color:var(--vg-on-primary)" aria-label="确认">{ic('check')}</button></div></div>''')
pages['settings'] = (390, 844, f'''<div class="phone compact">{status}{bar("设置")}
<h2 class="sec-title">隐私</h2>{trow('search','通过手机号被搜索','关闭后他人无法用手机号找到你',1)}{trow('search','通过短号被搜索','平台短号可被搜索',0)}{trow('eye','展示在线状态','仅联系人可见',1)}{trow('check-double','已读回执','关闭后你也看不到他人的已读',1)}{trow('camera','截屏通知','仅在系统支持的平台生效',0)}
<h2 class="sec-title">安全</h2>{trow('lock','设备锁','新设备登录需批准',1)}{li('device','登录设备','3 台')}{li('key','修改密码')}
<h2 class="sec-title">通用</h2>{li('globe','语言','简体中文')}{li('moon','深色模式','跟随系统')}{li('settings','字体大小','标准')}
<div style="padding:12px 16px;margin-top:auto"><button type="button" class="btn danger block">注销账号</button></div></div>''')
pages['tablet-chat'] = (834, 1112, f'''<div class="desk"><div style="width:300px;background:var(--vg-surface);border-right:1px solid var(--vg-border);display:flex;flex-direction:column"><div class="topbar" style="background:var(--vg-surface)"><h1 class="grow" style="font-size:24px;line-height:32px">会话</h1><button type="button" class="iconbtn" aria-label="新增">{ic('plus')}</button></div><div class="search" role="search">{ic('search',20)}<span>搜索</span></div>{rows}</div>
<div class="main" style="background:var(--vg-bg-canvas)"><div class="topbar">{av(1,'周','sm')}<div class="grow"><h1>周聿</h1><div class="cap muted">在线</div></div><button type="button" class="iconbtn" aria-label="通话">{ic('call')}</button><button type="button" class="iconbtn" aria-label="更多">{ic('more')}</button></div>
<div class="msgs" style="padding:20px 28px"><div class="sys">今天 09:05</div><div class="b in">下午的评审你能参加吗？<div class="meta">09:05</div></div><div class="b out">可以，我把图标稿先发给你<div class="meta">09:06 {ic('check-double',14)}</div></div></div><div class="inputbar"><button type="button" class="iconbtn" aria-label="附件">{ic('attach')}</button><div class="field"><input class="inp" placeholder="输入消息" aria-label="输入消息"></div><button type="button" class="iconbtn" style="background:var(--vg-primary);color:var(--vg-on-primary)" aria-label="发送">{ic('send')}</button></div></div></div>''')

# ----------------------------------------------------------------------------------------------------------------
EN = {
 '登录 VeyroGram': 'Sign in to VeyroGram', '使用手机号验证码登录，新设备需要额外确认。': 'Sign in with a phone code. New devices need extra confirmation.', '6 位验证码': '6-digit code', '重新获取 (52s)': 'Resend (52s)',
 '验证码已发送到你的手机': 'Code sent to your phone', '使用密码登录': 'Use password', '使用 Passkey': 'Use a passkey', '登录即表示你同意《服务条款》与《隐私政策》': 'By signing in you agree to the Terms and Privacy Policy',
 '>登录<': '>Sign in<', '"手机号"': '"Phone number"', '搜索联系人、群组、文件': 'Search contacts, groups, files', '>会话<': '>Chats<', '产品设计组': 'Product Design', '林夏: 新版图标已经上传': 'Lin Xia: New icons uploaded',
 '周聿': 'Zhou Yu', '[语音] 12&quot;': '[Voice] 12&quot;', '[语音] 12"': '[Voice] 12"', '客服 · 官方': 'Support · Official', '您好，已为您转接专属客服': 'Hi, you are connected to a dedicated agent', '家人群': 'Family', '妈妈: 周末回家吃饭': 'Mom: Dinner at home this weekend',
 '陈岚': 'Chen Lan', '会议改到下午三点': 'Meeting moved to 3 pm', '开发者通知': 'Developer notices', '你的小程序审核已通过': 'Your mini app passed review', '吴迪': 'Wu Di', '收到，谢谢': 'Got it, thanks', '昨天': 'Yesterday', '周二': 'Tue', '周一': 'Mon',
 '>联系人<': '>Contacts<', '>动态<': '>Moments<', '>发现<': '>Discover<', '>我<': '>Me<', '主导航': 'Main navigation', 'aria-label=未读3条': 'aria-label="3 unread"',
 '>在线<': '>Online<', '今天 09:05': 'Today 09:05', '下午的评审你能参加吗？': 'Can you join the review this afternoon?', '可以，我把图标稿先发给你': "Sure, I'll send you the icon draft first", '收到，看起来不错': 'Got it, looks good',
 '已扫描': 'Scanned', '上海 · 徐汇滨江': 'Shanghai · Xuhui Riverside', '对方撤回了一条消息': 'The other person recalled a message', '当前版本不支持该消息，请升级应用': "This message isn't supported in this version. Please update.", '输入消息': 'Type a message',
 '>设置<': '>Settings<', '>隐私<': '>Privacy<', '通过手机号被搜索': 'Searchable by phone number', '关闭后他人无法用手机号找到你': "Others can't find you by phone number when off", '通过短号被搜索': 'Searchable by short ID', '平台短号可被搜索': 'Your platform short ID can be searched',
 '展示在线状态': 'Show online status', '仅联系人可见': 'Visible to contacts only', '已读回执': 'Read receipts', '关闭后你也看不到他人的已读': "When off, you also can't see others' read status", '截屏通知': 'Screenshot notice', '仅在系统支持的平台生效': 'Works only on supported platforms',
 '>安全<': '>Security<', '设备锁': 'Device lock', '新设备登录需批准': 'New-device sign-in needs approval', '登录设备': 'Signed-in devices', '3 台': '3 devices', '修改密码': 'Change password', '>通用<': '>General<', '>语言<': '>Language<',
 '深色模式': 'Dark mode', '跟随系统': 'Follow system', '字体大小': 'Text size', '>标准<': '>Standard<', '注销账号': 'Delete account'}
def en(html):
    for k in sorted(EN, key=len, reverse=True): html = html.replace(k, EN[k])
    return html
EN_PAGES = ['login', 'chat-list', 'chat', 'settings']
TS_PAGES = ['login', 'chat-list', 'chat', 'settings', 'group-settings', 'device-approve', 'message-read-detail']
manifest = []
for name, (w, h, body) in pages.items():
    open(os.path.join(OUT, f'{name}.html'), 'w', encoding='utf8').write(page(name, body))
    manifest.append({'page': name, 'width': w, 'height': h, 'file': f'{name}.html', 'tag': ''})
    if name in EN_PAGES:
        open(os.path.join(OUT, f'{name}.en.html'), 'w', encoding='utf8').write(page(name, en(body), lang='en'))
        manifest.append({'page': name, 'width': w, 'height': h, 'file': f'{name}.en.html', 'tag': '-en'})
    if name in TS_PAGES:
        manifest.append({'page': name, 'width': w, 'height': h, 'file': f'{name}.html', 'tag': '-ts200', 'query': '&ts=2'})
json.dump(manifest, open(os.path.join(OUT, 'manifest.json'), 'w'), indent=1)
print(len(pages), 'prototype pages +', sum(1 for m in manifest if m['tag'] == '-en'), 'English variants written')
if '--no-shots' in sys.argv: sys.exit(0)
import tempfile
from PIL import Image
n = 0
tmp = tempfile.mkdtemp()
for f in os.listdir(BASE):
    if f.endswith('.png'): os.remove(os.path.join(BASE, f))
for m in manifest:
    for theme in ('light', 'dark'):
        sizes = [(m['width'], m['height'], m['tag'])]
        if m['width'] == 390 and m['page'] != 'states' and not m['tag']: sizes.append((360, 780, '-360'))
        for (w, h, tag) in sizes:
            out = os.path.join(BASE, f"{m['page']}{tag}-{w}x{h}-{theme}.png")
            wrap = os.path.join(tmp, 'w.html')   # headless Chrome enforces a ~500px minimum window width: render inside an iframe of the exact viewport, then crop
            open(wrap, 'w').write(f'<!doctype html><body style="margin:0;background:#888"><iframe src="file://{os.path.join(OUT, m["file"])}?theme={theme}{m.get("query", "")}" width="{w}" height="{h}" style="border:0;display:block"></iframe></body>')
            raw = os.path.join(tmp, 'raw.png')
            subprocess.run([CHROME, '--headless=new', '--disable-gpu', '--hide-scrollbars', f'--window-size={max(w, 600)},{h + 120}', '--force-device-scale-factor=1',
                            f'--screenshot={raw}', f'file://{wrap}'], check=True, capture_output=True)
            Image.open(raw).crop((0, 0, w, h)).save(out); n += 1
print(n, 'baseline screenshots written to', BASE)
