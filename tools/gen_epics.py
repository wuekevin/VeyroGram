# -*- coding: utf-8 -*-
"""Generate docs/product/EPICS.md, docs/stories/*.md and docs/product/traceability.csv from tools/epics_data_{1,2,3}.py and validate them.
Validation (exit 1 on failure): every PRD FR (1..72) and OP-FR (1..26) is covered by >=1 story; every dependency exists and is scheduled EARLIER (execution order = phase, then id) so the graph is acyclic;
every feature-matrix row (260) maps to >=1 story; every story has >=3 ACs in Given/When/Then form. Run: python3 -I tools/gen_epics.py"""
import csv, os, re, sys
HERE = os.path.dirname(os.path.abspath(__file__)); sys.path.insert(0, HERE)
ROOT = os.path.abspath(os.path.join(HERE, '..'))
import epics_data_1 as d1
import epics_data_2, epics_data_3          # noqa: F401  (register more epics/stories on import)
import epics_revisions as rev
EPICS = d1.EPICS
STORIES, PH, ROW_FIX, ROW_NOTES = rev.apply(EPICS, d1.STORIES)
def key(sid): a, b = sid.split('.'); return (int(a), int(b))
STORIES = sorted(STORIES, key=lambda s: key(s[0]))
PHORD = {'Phase 2': 2, 'Phase 3': 3, 'Phase 4': 4, 'Phase 5': 5, 'Phase 6': 6, 'Phase 7': 7, '未排期': 9}
read = lambda p: open(os.path.join(ROOT, p), encoding='utf8').read()
PRD, OPP, ARCH = read('docs/product/PRD.md'), read('docs/product/OPEN-PLATFORM-PRD.md'), read('docs/architecture/ARCHITECTURE.md')

st = {s[0]: s for s in STORIES}
SEQ = {sid: i for i, sid in enumerate(sorted(st, key=lambda x: (PHORD[PH[x]], key(x))))}   # execution order: phase first, then id
errors = []
# ---- story normalisation
KEYS = {'S': 'Server', 'A': 'Android', 'I': 'iOS', 'W': 'Web', 'D': 'Desktop', 'M': 'Manager', 'P': 'API', 'B': 'DB', 'N': '通知', 'R': '权限', 'E': 'E2E'}
NA = {'S': '不涉及服务端变更', 'A': '不涉及 Android 界面', 'I': '不涉及 iOS 界面', 'W': '不涉及 Web 界面', 'D': '不涉及桌面端界面', 'M': '不涉及管理后台', 'P': '不新增/变更接口',
      'B': '不涉及数据库变更', 'N': '不触发通知', 'R': '无新增权限规则（沿用既有策略）', 'E': '无独立 E2E（由所属 Epic 的 E2E 覆盖）'}
def parse_x(x):
    out = {}
    for part in x.split('|'):
        k, _, v = part.partition(':')
        k = k.strip()
        if k in KEYS and v.strip() and not v.strip().startswith('无'): out.setdefault(k, []).append(v.strip())
        elif k in KEYS and v.strip().startswith('无'): out.setdefault(k, [])
    return out
for s in STORIES:
    sid, title, who, want, why, reqs, ads, deps, size, acs, tests, x, cfg, rb = s
    if len(acs) < 3: errors.append(f'{sid}: <3 ACs')
    for a in acs:
        if not (a.startswith('Given') and ' / When ' in a and ' / Then ' in a): errors.append(f'{sid}: AC not Given/When/Then: {a[:40]}')
    for dpd in [d for d in deps.split(',') if d]:
        if dpd not in st: errors.append(f'{sid}: unknown dep {dpd}')
        elif dpd == sid: errors.append(f'{sid}: self dep')
        elif SEQ[dpd] >= SEQ[sid]: errors.append(f'{sid}: depends on {dpd} which is scheduled later or same slot (execution order = phase, then id)')
    if sid.split('.')[0] not in map(str, EPICS): errors.append(f'{sid}: unknown epic')

def req_ids(prefix_re):
    got = set()
    for s in STORIES:
        for r in s[5].split(','):
            m = re.fullmatch(prefix_re, r.strip())
            if m: got.add(int(m.group(1)))
    return got
fr_cov, op_cov, nfr_cov = req_ids(r'FR-(\d+)'), req_ids(r'OP-FR-(\d+)'), req_ids(r'NFR-(\d+)')
for n in range(1, 73):
    if n not in fr_cov: errors.append(f'FR-{n} not covered')
for n in range(1, 27):
    if n not in op_cov: errors.append(f'OP-FR-{n} not covered')
unc_nfr = [n for n in range(1, 19) if n not in nfr_cov]

# ---- traceability: feature-matrix row -> stories
rows = list(csv.DictReader(open(os.path.join(ROOT, 'docs/audit/feature-matrix.csv'), encoding='utf8')))
def has(f, *ks): return any(k in f for k in ks)
def map_row(r):
    m, a, b, f = r['module'], r['level1'], r['level2'], r['feature']; S = []
    if m == '基础版':
        base = {('会话', '文本消息'): ['1.6', '2.2'], ('会话', '图片消息'): ['2.2'], ('会话', '语音消息'): ['2.3'], ('会话', '名片消息'): ['2.3', '3.4'], ('会话', '正在输入消息'): ['2.6'],
                ('会话', '群系统消息'): ['4.1', '2.6'], ('会话', '单聊系统消息'): ['3.1', '2.6'], ('会话', '消息撤回'): ['2.4', '4.7'], ('会话', '消息转发'): ['2.5'], ('会话', '消息复制'): ['2.5'],
                ('会话', '消息收藏'): ['10.1'], ('会话', '查看历史消息'): ['2.6', '13.5'], ('会话', '常用事件'): ['4.2', '2.6'], ('会话', '在线状态'): ['13.3'],
                ('会话列表', '基础信息'): ['7.1'], ('会话列表', '在线状态'): ['13.3', '7.1'], ('会话列表', '消息红点'): ['7.1'], ('会话列表', '会话设置'): ['7.1'], ('会话列表', '消息强提醒'): ['4.2', '7.1'],
                ('会话列表', '消息状态'): ['1.6', '7.1'], ('会话列表', '用户标识'): ['7.1'], ('会话列表', '其他设备连接状态'): ['5.7', '7.1'], ('会话列表', '网络状态'): ['7.1'],
                ('会话列表', '常用事件'): ['7.2', '7.1'], ('会话列表', '添加群/联系人'): ['3.3', '4.1'], ('通讯录', '新朋友'): ['3.1'], ('通讯录', '保存的群聊'): ['4.8'], ('通讯录', '联系人'): ['3.4']}
        if (a, b) in base: return base[(a, b)]
        if (a, b) == ('会话设置', '群聊设置'):
            S = ['4.8']
            if has(f, '公告'): S.append('4.2')
            if has(f, '管理员'): S.append('4.3')
            if has(f, '黑名单'): S.append('4.4')
            if has(f, '禁言'): S.append('4.5')
            if has(f, '清空'): S.append('2.7')
            if has(f, '退出', '解散', '邀请'): S.append('4.1')
            if has(f, '免打扰', '置顶'): S.append('7.1')
            return S
        if (a, b) == ('会话设置', '单聊设置'):
            S = ['3.4']
            if has(f, '清空'): S.append('2.7')
            if has(f, '黑名单'): S.append('3.2')
            if has(f, '免打扰', '置顶'): S.append('7.1')
            if has(f, '创建群'): S.append('4.1')
            return S
        if (a, b) == ('通讯录', '个人名片'):
            S = ['3.4']
            if has(f, '黑名单'): S.append('3.2')
            if has(f, '标签'): S.append('3.5')
            if has(f, '二维码'): S.append('3.3')
            if has(f, '好友', '添加'): S.append('3.1')
            return S
        if (a, b) == ('设置', '登录用户信息'):
            S = ['5.1']
            if has(f, '二维码'): S.append('3.3')
            if has(f, '设备'): S.append('5.3')
            if has(f, '退出'): S.append('1.5')
            return S
        if (a, b) == ('设置', '新消息通知'): return ['7.5']
        if (a, b) == ('设置', '通用'):
            S = ['5.8']
            if has(f, '设备'): S.append('5.3')
            if has(f, '密码'): S.append('5.4')
            return S
        return S
    if m == '旗舰模块':
        return {'消息已读未读': ['13.1'], '截屏消息': ['13.6'], '消息编辑': ['13.2'], '消息点赞': ['13.3'], '在线状态': ['13.3'], '图片编辑': ['13.4'], '查看历史消息': ['13.5']}.get(b, ['13.6'] if a == '会话设置' else [])
    if m == '音视频模块':
        return ['15.3'] if has(f, '会议') else (['15.1'] if has(f, '入口', '消息UI') else ['15.2'])
    if m == '表情商店': return {'消息面板': ['11.3'], '制作表情': ['11.4'], '平台表情': ['11.5']}.get(b, [])
    if m == '群管理模块':
        if has(f, '黑名单', '邀请确认'): return ['4.4']
        if has(f, '管理员'): return ['4.3']
        if has(f, '头像'): return ['4.8']
        if has(f, '禁言'): return ['4.5']
        if has(f, '互加', '历史消息'): return ['4.6']
        if has(f, '撤回'): return ['4.7']
        return []
    if m == '安全与隐私模块':
        if has(f, '搜索'): return ['5.2', '3.3']
        if has(f, '密码'): return ['5.4']
        if has(f, '设备锁'): return ['5.3']
        if has(f, '注销'): return ['5.6']
        if has(f, '黑名单'): return ['3.2']
        return []
    if m == '小视频模块': return ['11.1'] if has(f, '录制', '相册') else ['11.2']
    if m == '位置模块': return ['10.2']
    if m == '文件模块': return ['10.3']
    if m == '标签模块': return ['3.5']
    if m == '收藏模块': return ['10.1']
    if m == '客服模块': return ['14.2'] if has(f, '欢迎词', '访客资料') else ['14.1']
    if m == '朋友圈模块':
        if has(f, '删除'): return ['12.4', '12.2']
        if has(f, '评论', '点赞', '提醒用户', '新消息', '更多弹出'): return ['12.3']
        if has(f, '封面', '用户名称', '备注', '发布者', '发布时间', '地址', '查看该用户', '头像', '大图', '视频播放', '点击视频'): return ['12.2']
        if has(f, '复制', '收藏', '转发'): return ['12.2', '10.1']
        return ['12.1']
    return S
trace = []
for i, r in enumerate(rows, 1):
    S = rev.final_ids([rev.REN.get(x, x) for x in map_row(r)])
    if i in ROW_FIX: S = ROW_FIX[i]
    S = sorted(set(S), key=key)
    for sid in S:
        if sid not in st: errors.append(f'trace row {i}: unknown story {sid}')
    if not S: errors.append(f'trace row {i}: UNMAPPED {r["module"]}/{r["level1"]}/{r["level2"]}/{r["feature"][:30]}')
    trace.append((i, r['module'], r['level1'], r['level2'], r['feature'], ';'.join(S), ROW_NOTES.get(i, ''), r['md_source']))

# ---- requirement inventory text
fr_title = {int(n): t.strip() for n, t in re.findall(r'#### FR-(\d+): (.+)', PRD)}
op_title = {int(n): t.strip() for n, t in re.findall(r'#### OP-FR-(\d+): (.+)', OPP)}
nfr_text = {int(n): t.strip() for n, t in re.findall(r'- \*\*NFR-(\d+) ([^*]+)\*\*', PRD)}
ad_title = {int(n): t.strip() for n, t in re.findall(r'### AD-(\d+) — (.+)', ARCH)}
UXDR = [('UX-DR1', '设计令牌单一来源并校验对比度', ['6.1']), ('UX-DR2', '原创图标库且导航/菜单禁用 Emoji', ['6.2']), ('UX-DR3', '多平台品牌资产（启动页、应用/通知图标、PWA/桌面图标）', ['6.3']),
        ('UX-DR4', '桌面端三栏、快捷键、托盘、拖拽与加固壳', ['6.4']), ('UX-DR5', '每个界面具备加载/空/错误/离线/无权限/降级状态', ['7.1', '9.6', '16.8']),
        ('UX-DR6', 'WCAG 2.1 AA 验收（自动化 + 人工 + 第三方审计）', ['6.5', '20.4']), ('UX-DR7', '200% 字号与 320px 回流无内容丢失', ['5.8', '6.5']),
        ('UX-DR8', '授权同意页与小程序容器顶栏由宿主绘制，不可伪造', ['16.4', '17.2']), ('UX-DR9', '拖拽操作提供非拖拽替代控件', ['11.4', '12.1']), ('UX-DR10', '简体中文与英文 i18n，主题明/暗', ['5.8'])]
for _, _, ss in UXDR:
    for sid in ss:
        if sid not in st: errors.append(f'UX-DR unknown story {sid}')

# ---- write outputs
def table_x(xs):
    lines = ['| 列 | 任务 |', '|---|---|']
    for k, name in KEYS.items():
        lines.append(f'| {name} | ' + ('；'.join(xs[k]) if xs.get(k) else '不适用：' + NA[k]) + ' |')
    return '\n'.join(lines)
out = os.path.join(ROOT, 'docs', 'stories'); os.makedirs(out, exist_ok=True)
KEEP = {}   # sid -> (status, trailing '## Dev Agent Record' section): implementation notes must survive regeneration
for f in os.listdir(out):
    if f.endswith('.md') and f != 'README.md':
        t = open(os.path.join(out, f), encoding='utf8').read()
        m = re.search(r'^id: "([^"]+)"', t, re.M)
        if m:
            stt = re.search(r'^status: (\S+)', t, re.M)
            i = t.find('\n## Dev Agent Record')
            KEEP[m.group(1)] = (stt.group(1) if stt else 'backlog', t[i:] if i >= 0 else '')
for f in os.listdir(out):
    if f.endswith('.md'): os.remove(os.path.join(out, f))
DOD = '''## 通用完成定义（DoD，适用于每个 Story）
1. 代码经 PR 合并，评审通过；改动遵守 `docs/architecture/ARCHITECTURE.md` 的 AD 规则（尤其依赖方向 AD-1 与对象级授权 AD-3）。
2. 测试通过：单元 + 集成（真实 MySQL 8.0 / Redis 7）+ `go test -race`；本 Story 列出的**安全负面测试**已实现并通过；契约测试（OpenAPI / 消息类型 schema）通过。
3. 静态/供应链门禁通过：SAST、SCA、secret 扫描、许可证扫描、SBOM；高危/严重漏洞为零（AD-23）。
4. 涉及接口：OpenAPI 3.1 已更新且 diff 门禁通过；涉及数据库：迁移与回滚说明已评审并在 staging 演练。
5. 涉及界面：对照 `docs/design/` 的令牌与组件规格；明/暗、中/英、200% 字号截图基线已更新；`tools/a11y_scan.py` 通过；WCAG 清单对应条目已填证据；导航无 Emoji（lint）。
6. 跨端任务（Server/Android/iOS/Web/Desktop/Manager/API/DB/通知/权限/E2E）逐列完成或有正式“不适用”说明。
7. 配置项与回滚方案已写入运维文档；功能开关默认值经确认；监控指标与告警已接入（NFR-7）。
8. **追溯闭环：** 将 `docs/audit/feature-matrix.csv` 中本 Story 对应的行（见 `docs/product/traceability.csv`）的状态更新为 `TESTED` 并附证据路径（测试报告/截图），否则 Epic 不得关闭（SM-4）。
9. 文档：用户/管理员/开发者手册（如适用）与变更说明已更新；风险登记与威胁模型相关条目已更新。
'''
open(os.path.join(out, 'README.md'), 'w', encoding='utf8').write('# Stories\n\n按依赖排序的 Story 文件。命名：`<epic>.<story>-<slug>.md`。状态默认 `backlog`（尚未开始；设计门禁未签字前 Phase 3 不启动）。\n\n' + DOD +
    '\n## 状态约定\n`backlog` → `ready` → `in-progress` → `review` → `done`。Story 只能依赖执行顺序（先按阶段，再按编号）更靠前的 Story，依赖图无环（生成器已校验）。\n')
for s in STORIES:
    sid, title, who, want, why, reqs, ads, deps, size, acs, tests, x, cfg, rb = s
    e = int(sid.split('.')[0]); xs = parse_x(x)
    slug = re.sub(r'[^0-9A-Za-z一-鿿]+', '-', title).strip('-')[:40]
    body = f'''---
id: "{sid}"
epic: {e}
phase: "{PH[sid]}"
title: "{title}"
status: backlog
size: {size}
depends_on: [{", ".join(f'"{d}"' for d in deps.split(",") if d)}]
requirements: [{", ".join(f'"{r.strip()}"' for r in reqs.split(","))}]
architecture: [{", ".join(f'"{a.strip()}"' for a in ads.split(",") if a.strip())}]
---

# Story {sid}: {title}

**Epic {e}：{EPICS[e]['title']}（本 Story：{PH[sid]}）** · 规模 {size}（S=半天–1天，M=1–3天，L=3–7天；按单个开发 Agent 的上下文可完成估算）

As a {who},
I want {want},
So that {why}.

## 验收标准（Acceptance Criteria）
''' + '\n'.join('\n'.join(f'**{k}**' + f' {v}' for k, v in zip(('Given', 'When', 'Then'), re.split(r' / (?:When|Then) ', a)[0:3])).replace('**Given** Given ', '**Given** ') + '\n' for a in acs) + f'''
## 测试（含安全负面测试）
''' + '\n'.join(f'- {t}' for t in tests) + f'''

## 跨端任务
{table_x(xs)}

## 配置与回滚
- **配置/决策点：** {cfg}
- **回滚方案：** {rb}

## 依赖
{("依赖 Story：" + "、".join(deps.split(",")) ) if deps else "无前置 Story（本 Epic 起点或由基座提供）"}

## 完成定义
见 [README.md](README.md) 的通用 DoD；另须满足以上验收标准与测试。
'''
    if sid in KEEP:
        stt, rec = KEEP[sid]
        body = body.replace('status: backlog', f'status: {stt}', 1) + rec
    open(os.path.join(out, f'{sid}-{slug}.md'), 'w', encoding='utf8').write(body)

L = ['---', 'stepsCompleted: [1, 2, 3, 4]', 'inputDocuments: [docs/product/PRD.md, docs/product/OPEN-PLATFORM-PRD.md, docs/product/addendum.md, docs/architecture/ARCHITECTURE.md, docs/design/DESIGN-SYSTEM.md, docs/audit/feature-matrix.csv, docs/audit/risk-register.md]', 'status: draft', '---', '',
     '# VeyroGram - Epic Breakdown', '', '## Overview', '',
     '本文把 PRD（FR-1…72、NFR-1…18）、开放平台 PRD（OP-FR-1…26）、架构（AD-1…24）与设计系统拆解为可实现的 Epic 与 Story。按 BMAD-MASTER §11 的 Phase 2–7 依赖排序；Story 只依赖更早的 Story。**阶段按 Story 标注**（Epic 可跨阶段）：标签、群高级管理、安全与隐私深化（设备锁/改密/注销/MFA/隐私）属 Phase 4，与 PRD §6.1 和 BMAD-MASTER §11 一致；基础在线状态、会话内查找、截屏/撤回通知属 Phase 3。执行顺序为“先阶段、再编号”，Story 只依赖顺序更靠前的 Story。每个 Story 的完整细节（测试、11 列跨端任务、配置与回滚）见 `docs/stories/`；逐行追溯见 `docs/product/traceability.csv`（功能矩阵 260 行 → Story）。', '',
     '**约束提醒：** MySQL 8.0（已过 EOL，风险已接受）、Redis 7.x、Go 模块化单体、原生 systemd（禁 Docker）、WuKongIM 仅经 imgw 可达、客户端关键词搜索在本地（ADR-0010）。设计门禁（`docs/design/APPROVALS.md`）未全部签字前，Phase 3 批量编码不启动。', '', '## Requirements Inventory', '', '### Functional Requirements', '']
L += [f'- FR-{n}: {fr_title.get(n, "")}' for n in range(1, 73)] + [''] + [f'- OP-FR-{n}: {op_title.get(n, "")}' for n in range(1, 27)] + ['', '### NonFunctional Requirements', '']
L += [f'- NFR-{n}: {nfr_text.get(n, "")}' for n in range(1, 19)] + ['', '### Additional Requirements（架构决策）', ''] + [f'- AD-{n}: {ad_title.get(n, "")}' for n in sorted(ad_title)] + ['', '### UX Design Requirements', '']
L += [f'- {k}: {t}（Story {", ".join(ss)}）' for k, t, ss in UXDR] + ['', '### FR Coverage Map', '']
cov = {}
for s in STORIES:
    for r in s[5].split(','):
        r = r.strip()
        if r.startswith(('FR-', 'OP-FR-')): cov.setdefault(r, []).append(s[0])
L += [f'{r}: Story {", ".join(v)}' for r, v in sorted(cov.items(), key=lambda kv: (kv[0].startswith('OP'), int(re.findall(r"\d+", kv[0])[0])))] + ['', '### 功能矩阵行追溯', '', f'`docs/product/traceability.csv` 将 `docs/audit/feature-matrix.csv` 的 {len(rows)} 行逐行映射到 Story（基础版 139 行 + 付费模块 121 行）。', '', '## Epic List', '']
for n, e in EPICS.items():
    ss = [s for s in STORIES if s[0].startswith(f'{n}.')]
    pset = sorted({PH[s[0]] for s in ss}, key=lambda p: PHORD[p]); ptxt = pset[0] if len(pset) == 1 else pset[0] + '–' + pset[-1].replace('Phase ', '')
    frs = sorted({r.strip() for s in ss for r in s[5].split(',') if r.strip().startswith(('FR-', 'OP-FR-'))})
    L += [f'### Epic {n}: {e["title"]}（{ptxt}）', e['goal'], f'**FRs covered:** {", ".join(frs) if frs else "—"}  ·  **Stories:** {len(ss)}', '']
for n, e in EPICS.items():
    L += [f'## Epic {n}: {e["title"]}', '', e['goal'], '']
    for s in [s for s in STORIES if s[0].startswith(f'{n}.')]:
        sid, title, who, want, why, reqs, ads, deps, size, acs, tests, x, cfg, rb = s
        L += [f'### Story {sid}: {title}', '', f'As a {who},', f'I want {want},', f'So that {why}.', '', f'*阶段：{PH[sid]}｜需求：{reqs}｜架构：{ads}｜依赖：{deps or "无"}｜规模：{size}*', '', '**Acceptance Criteria:**', '']
        for a in acs:
            g, w, t = (re.split(r' / (?:When|Then) ', a) + ['', ''])[:3]
            L += [f'**Given** {g[6:].strip()}', f'**When** {w.strip()}', f'**Then** {t.strip()}', '']
        L += [f'详见 `docs/stories/{sid}-*.md`（测试、跨端任务、配置与回滚）。', '']
open(os.path.join(ROOT, 'docs/product/EPICS.md'), 'w', encoding='utf8').write('\n'.join(L) + '\n')
with open(os.path.join(ROOT, 'docs/product/traceability.csv'), 'w', newline='', encoding='utf8') as f:
    w = csv.writer(f); w.writerow(['row', 'module', 'level1', 'level2', 'feature', 'stories', 'note', 'md_source']); w.writerows(trace)
with open(os.path.join(ROOT, 'docs/product/traceability-fr.csv'), 'w', newline='', encoding='utf8') as f:
    w = csv.writer(f); w.writerow(['requirement', 'stories']); [w.writerow([r, ';'.join(v)]) for r, v in sorted(cov.items())]
nfr_cov_map = {}
for s in STORIES:
    if s[0].startswith('21.'): continue
    for r_ in s[5].split(','):
        r_ = r_.strip()
        if r_.startswith('NFR-'): nfr_cov_map.setdefault(r_, []).append(s[0])
with open(os.path.join(ROOT, 'docs/product/traceability-nfr.csv'), 'w', newline='', encoding='utf8') as f:
    w = csv.writer(f); w.writerow(['nfr', 'stories', 'note']); [w.writerow([f'NFR-{n}', ';'.join(nfr_cov_map.get(f'NFR-{n}', [])), '' if nfr_cov_map.get(f'NFR-{n}') else 'UNCOVERED']) for n in range(1, 19)]
    w.writerow(['FR-58;FR-59;FR-60', '21.1;21.2;21.3', 'deferred（未排期，不计入覆盖）'])
# ---- report
ph = {}
for s in STORIES: ph.setdefault(PH[s[0]], []).append(s[0])
print(f'{len(EPICS)} epics, {len(STORIES)} stories; matrix rows mapped {len(trace)}/{len(rows)}; FR covered {len(fr_cov)}/72; OP-FR {len(op_cov)}/26')
print('stories per phase:', {k: len(v) for k, v in ph.items()})
print('NFRs without a dedicated story (cross-cutting, covered by DoD/gates):', unc_nfr)
if errors:
    print(f'{len(errors)} VALIDATION ERROR(S):'); [print(' -', e) for e in errors[:40]]; sys.exit(1)
print('validation OK')
