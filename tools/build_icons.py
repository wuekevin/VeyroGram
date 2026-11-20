#!/usr/bin/env python3
"""Build the VeyroGram UI icon set (original, hand-authored paths; 24px grid, 1.75 stroke).
Usage: python3 -I tools/build_icons.py  -> writes brand/icons/ui/*.svg, sprite.svg, icons.json, contact-sheet.html"""
import json, os, html
OUT = os.path.join(os.path.dirname(__file__), '..', 'brand', 'icons', 'ui')
os.makedirs(OUT, exist_ok=True)
D = lambda d: f'<path d="{d}"/>'
C = lambda cx, cy, r: f'<circle cx="{cx}" cy="{cy}" r="{r}"/>'
R = lambda x, y, w, h, rx=0: f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="{rx}"/>'
DOT = lambda cx, cy, r=1.4: f'<circle cx="{cx}" cy="{cy}" r="{r}" fill="currentColor" stroke="none"/>'
ICONS = {
 # name: (zh label, en label, inner svg)
 'chat': ('会话', 'Chats', D('M5 5h14a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2h-7l-4 3v-3H5a2 2 0 0 1-2-2V7a2 2 0 0 1 2-2z')),
 'contacts': ('联系人', 'Contacts', C(9,8,3)+D('M3.5 19a5.5 5.5 0 0 1 11 0')+C(17,9,2.5)+D('M16 14.2a4.5 4.5 0 0 1 4.5 4.3')),
 'moments': ('动态', 'Moments', C(12,12,4.5)+'<ellipse cx="12" cy="12" rx="9.6" ry="3.6" transform="rotate(-28 12 12)"/>'),
 'camera': ('相机', 'Camera', D('M4 8h3l2-3h6l2 3h3a1 1 0 0 1 1 1v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V9a1 1 0 0 1 1-1z')+C(12,13,3.5)),
 'discover': ('发现', 'Discover', C(12,12,9)+D('M15.5 8.5l-2 5-5 2 2-5z')),
 'me': ('我', 'Me', C(12,8,4)+D('M4.5 20a7.5 7.5 0 0 1 15 0')),
 'search': ('搜索', 'Search', C(11,11,6)+D('M15.5 15.5L20 20')),
 'plus': ('新增', 'Add', D('M12 5v14M5 12h14')),
 'settings': ('设置', 'Settings', D('M4 7h10M18 7h2M4 17h2M10 17h10')+C(16,7,2)+C(8,17,2)),
 'bell': ('通知', 'Notifications', D('M6 17v-6a6 6 0 0 1 12 0v6l1.5 2h-15z')+D('M10 21a2 2 0 0 0 4 0')),
 'bell-off': ('免打扰', 'Muted', D('M6 17v-6a6 6 0 0 1 9-5.2M18 11v6l1.5 2H8')+D('M10 21a2 2 0 0 0 4 0')+D('M4 4l16 16')),
 'lock': ('锁定', 'Lock', R(5,11,14,9,2)+D('M8 11V8a4 4 0 0 1 8 0v3')),
 'shield': ('安全', 'Security', D('M12 3l7 3v5c0 5-3 8-7 10-4-2-7-5-7-10V6z')+D('M9 12l2 2 4-4')),
 'device': ('设备', 'Devices', R(7,3,10,18,2.5)+D('M11 18h2')),
 'file': ('文件', 'File', D('M7 3h7l5 5v11a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2z')+D('M14 3v5h5')),
 'image': ('图片', 'Image', R(3,5,18,14,3)+C(9,10,1.5)+D('M4 17l5-4 4 3 3-2 4 3')),
 'video': ('视频', 'Video', R(3,6,13,12,2.5)+D('M16 10.5l5-3v9l-5-3z')),
 'mic': ('语音', 'Voice', R(9,3,6,11,3)+D('M5.5 11a6.5 6.5 0 0 0 13 0M12 17.5V21')),
 'location': ('位置', 'Location', D('M12 21s6.5-5.7 6.5-11A6.5 6.5 0 0 0 5.5 10C5.5 15.3 12 21 12 21z')+C(12,10,2.3)),
 'star': ('收藏', 'Favorites', D('M12 3.5l2.6 5.4 5.9.8-4.3 4.1 1 5.9L12 17l-5.2 2.7 1-5.9L3.5 9.7l5.9-.8z')),
 'sticker': ('表情', 'Stickers', D('M5 4h14a1 1 0 0 1 1 1v9l-6 6H5a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1z')+D('M14 20v-5a1 1 0 0 1 1-1h5')+DOT(9,9)+DOT(13,9)+D('M8.5 13a4 4 0 0 0 3 1.5')),
 'tag': ('标签', 'Tags', D('M3 5v7l9 9 9-9-9-9H5a2 2 0 0 0-2 2z')+C(8,8,1.3)),
 'group': ('群组', 'Groups', C(12,8,3)+D('M6.5 19a5.5 5.5 0 0 1 11 0')+C(5.5,10,2)+C(18.5,10,2)+D('M2 17a3.5 3.5 0 0 1 3.5-3M22 17a3.5 3.5 0 0 0-3.5-3')),
 'qr': ('二维码', 'QR code', R(3,3,7,7,1.5)+R(14,3,7,7,1.5)+R(3,14,7,7,1.5)+D('M14 14h3v3M21 14v.01M14 21h3M21 17v4h-3')),
 'call': ('通话', 'Call', D('M5 4h4l2 5-2.5 1.5a11 11 0 0 0 5 5L15 13l5 2v4a2 2 0 0 1-2 2A16 16 0 0 1 3 6a2 2 0 0 1 2-2z')),
 'send': ('发送', 'Send', D('M21 3L3 10.5l7 3 3 7zM10 13.5L21 3')),
 'more': ('更多', 'More', DOT(5,12,1.6)+DOT(12,12,1.6)+DOT(19,12,1.6)),
 'back': ('返回', 'Back', D('M15 5l-7 7 7 7')),
 'close': ('关闭', 'Close', D('M6 6l12 12M18 6L6 18')),
 'check': ('确认', 'Confirm', D('M5 12.5l4.5 4.5L19 7.5')),
 'trash': ('删除', 'Delete', D('M4 7h16M9 7V4h6v3M6 7l1 13h10l1-13M10 11v5M14 11v5')),
 'edit': ('编辑', 'Edit', D('M4 20h4L19 9a2.1 2.1 0 0 0-3-3L5 17zM14 7l3 3')),
 'share': ('分享', 'Share', C(6,12,2.5)+C(18,6,2.5)+C(18,18,2.5)+D('M8.2 10.8l7.6-3.6M8.2 13.2l7.6 3.6')),
 'download': ('下载', 'Download', D('M12 4v11M7.5 11l4.5 4.5 4.5-4.5M5 20h14')),
 'link': ('链接', 'Link', D('M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1')),
 'clock': ('时间', 'Time', C(12,12,9)+D('M12 7v5l3.5 2')),
 'flag': ('举报', 'Report', D('M5 21V4M5 4h11l-2 4 2 4H5')),
 'heart': ('点赞', 'Like', D('M12 20s-7.5-4.6-7.5-10A4.3 4.3 0 0 1 12 7.2 4.3 4.3 0 0 1 19.5 10c0 5.4-7.5 10-7.5 10z')),
 'eye': ('可见', 'Visible', D('M2.5 12S6 5.5 12 5.5 21.5 12 21.5 12 18 18.5 12 18.5 2.5 12 2.5 12z')+C(12,12,3)),
 'globe': ('语言', 'Language', C(12,12,9)+D('M3 12h18M12 3c3 3 3 15 0 18M12 3c-3 3-3 15 0 18')),
 'moon': ('深色', 'Dark theme', D('M20 14.5A8.5 8.5 0 0 1 9.5 4a8.5 8.5 0 1 0 10.5 10.5z')),
 'sun': ('浅色', 'Light theme', C(12,12,4)+D('M12 3v2M12 19v2M3 12h2M19 12h2M5.6 5.6l1.4 1.4M17 17l1.4 1.4M18.4 5.6L17 7M7 17l-1.4 1.4')),
 'reply': ('回复', 'Reply', D('M9 7L4 12l5 5M4 12h9a7 7 0 0 1 7 7')),
 'forward': ('转发', 'Forward', D('M15 7l5 5-5 5M20 12h-9a7 7 0 0 0-7 7')),
 'pin': ('置顶', 'Pin', D('M9 4h6l-1 6 3 3H7l3-3zM12 13v8')),
 'miniapp': ('小程序', 'Mini apps', R(4,4,7,7,2)+R(13,4,7,7,2)+R(4,13,7,7,2)+R(13,13,7,7,2)),
 'dashboard': ('概览', 'Overview', D('M4 13h4v7H4zM10 4h4v16h-4zM16 9h4v11h-4z')),
 'chart': ('报表', 'Reports', D('M4 19V5M4 19h16M7 15l4-4 3 3 5-6')),
 'audit': ('审计', 'Audit log', R(5,3,14,18,2)+D('M9 8h6M9 12h6M9 16h3')),
 'support': ('客服', 'Support', D('M4 14v-2a8 8 0 0 1 16 0v2M4 14h3v5H5a1 1 0 0 1-1-1zM20 14h-3v5h2a1 1 0 0 0 1-1zM17 19a3 3 0 0 1-3 2h-1')),
 'ticket': ('工单', 'Tickets', D('M4 8a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2v2a2 2 0 0 0 0 4v2a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2v-2a2 2 0 0 0 0-4zM14 6v12')),
 'key': ('密钥', 'Keys', C(8,15,3.5)+D('M10.5 12.5L20 3M16 7l3 3M14 9l2 2')),
 'wallet': ('订阅', 'Billing', D('M4 7a2 2 0 0 1 2-2h12v4M4 7v11a2 2 0 0 0 2 2h14V9H6a2 2 0 0 1-2-2z')+DOT(16,14.5)),
 'bot': ('机器人', 'Bots', R(5,8,14,11,3)+D('M12 8V5M9 13v.01M15 13v.01M9 17h6')),
 'megaphone': ('运营', 'Campaigns', D('M4 10v4h3l7 4V6l-7 4zM17 9a4 4 0 0 1 0 6')),
 'attach': ('附件', 'Attach', D('M20 11.5l-8 8a5 5 0 0 1-7-7l8.5-8.5a3.3 3.3 0 0 1 4.7 4.7L9.7 17.2a1.7 1.7 0 0 1-2.4-2.4L15 7')),
 'alert': ('警告', 'Warning', D('M12 3.5l9.5 16.5h-19z')+D('M12 10v4.5')+DOT(12,17.2,1.1)),
 'info': ('信息', 'Information', C(12,12,9)+D('M12 11v5.5')+DOT(12,7.8,1.1)),
 'check-double': ('已读', 'Read', D('M2.5 12.5L7 17l8-9.5M10 15.5l1.5 1.5L21 7')),
 'offline': ('离线', 'Offline', D('M2.5 9a15 15 0 0 1 5-3M21.5 9a15 15 0 0 0-9.5-4.5M5.5 12.5a10 10 0 0 1 3.2-2M18.5 12.5a10 10 0 0 0-4-2.4M9 16a5 5 0 0 1 6 0')+DOT(12,19.2,1.2)+D('M3 3l18 18')),
 'chevron-right': ('向右', 'Next', D('M9 5l7 7-7 7')),
 'chevron-down': ('向下', 'Expand', D('M5 9l7 7 7-7')),
 'chevron-up': ('向上', 'Collapse', D('M5 15l7-7 7 7')),
 'refresh': ('刷新', 'Refresh', D('M20 12a8 8 0 1 1-2.6-5.9M20 4v5h-5')),
 'screen-share': ('共享屏幕', 'Share screen', R(3,4,18,12,2)+D('M8 20h8M12 16v4M9 11l3-3 3 3M12 8v6')),
 'call-end': ('挂断', 'End call', D('M3 14.5c5-4.5 13-4.5 18 0l-1.6 3-3.4-1.2v-2.6c-2-.8-4-.8-6 0v2.6L4.6 17.5z')),
 'video-off': ('关闭摄像头', 'Camera off', R(3,6,13,12,2.5)+D('M16 10.5l5-3v9l-5-3zM3 3l18 18')),
 'mic-off': ('静音', 'Mute', R(9,3,6,11,3)+D('M5.5 11a6.5 6.5 0 0 0 11 4.6M18.5 11M12 17.5V21M3 3l18 18')),
 'users-admin': ('成员管理', 'Member admin', C(9,8,3)+D('M3.5 19a5.5 5.5 0 0 1 11 0')+D('M15.5 9l2.2 2.2L21.5 7.4')),
}
STYLE = 'fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round"'
def svg(name, inner, zh, en):
    return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24" {STYLE} role="img" '
            f'aria-labelledby="t"><title id="t">{html.escape(en)}</title>{inner}</svg>\n')
meta = []
for n, (zh, en, inner) in ICONS.items():
    open(os.path.join(OUT, f'{n}.svg'), 'w', encoding='utf8').write(svg(n, inner, zh, en))
    meta.append({'name': n, 'label_zh': zh, 'label_en': en})
sprite = '<svg xmlns="http://www.w3.org/2000/svg" style="display:none">' + ''.join(
    f'<symbol id="vg-{n}" viewBox="0 0 24 24">{i}</symbol>' for n, (_, _, i) in ICONS.items()) + '</svg>\n'
open(os.path.join(OUT, 'sprite.svg'), 'w', encoding='utf8').write(sprite)
json.dump({'grid': 24, 'stroke': 1.75, 'count': len(meta), 'icons': meta, 'status': 'v0.1 hand-authored draft; designer review pending'},
          open(os.path.join(OUT, 'icons.json'), 'w', encoding='utf8'), ensure_ascii=False, indent=1)
cells = ''.join(f'<figure><svg viewBox="0 0 24 24" width="32" height="32" {STYLE}>{i}</svg><figcaption>{n}<br>{zh}</figcaption></figure>' for n, (zh, _, i) in ICONS.items())
page = f'''<!doctype html><meta charset="utf-8"><title>VeyroGram icons v0.1</title><style>
body{{margin:0;font:12px system-ui;display:grid;grid-template-columns:1fr 1fr}}
.t{{padding:16px;display:grid;grid-template-columns:repeat(auto-fill,88px);gap:12px;align-content:start}}
.l{{background:#F4F7FF;color:#0A1230}}.d{{background:#020B2A;color:#EAF1FF}}
figure{{margin:0;text-align:center}}figcaption{{opacity:.75;margin-top:4px}}</style>
<div class="t l">{cells}</div><div class="t d">{cells}</div>'''
open(os.path.join(OUT, 'contact-sheet.html'), 'w', encoding='utf8').write(page)
# --- state set: selected (heavier stroke + 16% tint fill) and badge-anchor guide; disabled/pressed are token-driven (opacity .38 / .72)
ST = os.path.join(OUT, 'states'); os.makedirs(ST, exist_ok=True)
STYLE_SEL = 'fill="currentColor" fill-opacity=".16" stroke="currentColor" stroke-width="2.1" stroke-linecap="round" stroke-linejoin="round"'
for n, (zh, en, inner) in ICONS.items():
    open(os.path.join(ST, f'{n}-selected.svg'), 'w', encoding='utf8').write(
        f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24" {STYLE_SEL} role="img" aria-labelledby="t"><title id="t">{html.escape(en)} (selected)</title>{inner}</svg>\n')
json.dump({'states': {'default': 'icons/<name>.svg', 'selected': 'states/<name>-selected.svg', 'disabled': 'default at opacity .38 (token: text-disabled)', 'pressed': 'default at opacity .72', 'badge': 'dot r=4 at (19,5) in token danger/primary, 1.5px canvas-colored ring'}},
          open(os.path.join(ST, 'states.json'), 'w'), indent=1)
print(len(meta), 'icons written to', os.path.normpath(OUT), '+', len(meta), 'selected-state variants')
