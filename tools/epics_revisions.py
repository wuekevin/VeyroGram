# -*- coding: utf-8 -*-
"""Revision layer applied on top of tools/epics_data_{1,2,3}.py after the independent review (docs/product/review-epics-independent.md).
Every change is explicit here so it can be audited. apply() returns (stories, phase_of, row_fix, notes).
Covers: Critical C1-C5, High H1-H9, and the Medium/Low items listed at the bottom of this file."""
import re
REN = {'1.4': '1.7', '1.5': '1.8', '1.6': '1.9', '1.7': '1.10', '1.8': '1.11', '1.9': '1.12'}
# Second renumbering (Phase 2 restructure): first-pass id -> final id(s). Other epics' dependencies and row fixes are translated through it.
FINAL = {'1.1': ['1.1'], '1.2': ['1.4'], '1.3': ['1.6'], '1.4': ['1.5'], '1.5': ['1.9', '1.10'], '1.6': ['1.11'], '1.7': ['1.12'], '1.8': ['1.13'], '1.9': ['1.15'],
         '1.10': ['1.16'], '1.11': ['1.19'], '1.12': ['1.24'], '1.13': ['1.21'], '1.14': ['1.22'], '1.15': ['1.8'], '1.16': ['1.23']}
def final_ids(ids): 
    out = []
    for x in ids:
        for y in FINAL.get(x, [x]):
            if y not in out: out.append(y)
    return out
FIELDS = ['id', 'title', 'who', 'want', 'why', 'reqs', 'ads', 'deps', 'size', 'acs', 'tests', 'x', 'cfg', 'rb']

def apply(EPICS, STORIES):
    D = [dict(zip(FIELDS, s)) for s in STORIES]
    D = [dict(d, acs=list(d['acs']), tests=list(d['tests'])) for d in D]
    # ---------------------------------------------------------------- C1/C2/H4: renumber Epic 1 and insert foundation stories
    ren = REN
    def rd(deps): return ','.join(ren.get(x, x) for x in deps.split(',') if x)
    for d in D:
        d['id'] = ren.get(d['id'], d['id']); d['deps'] = rd(d['deps'])
    by = lambda sid: next(d for d in D if d['id'] == sid)
    def new(sid, title, who, want, why, reqs, ads, deps, size, acs, tests, x, cfg, rb):
        D.append(dict(zip(FIELDS, (sid, title, who, want, why, reqs, ads, deps, size, acs, tests, x, cfg, rb))))
    new('1.4', '可观测性基座（指标、日志、追踪与告警）', '平台所有者', '统一的指标、结构化日志、追踪与告警基座', '每个后续 Story 的监控要求（DoD #7）与 Phase 2 Exit“基本监控贯通”有可落地的载体',
        'NFR-7', 'AD-15', '1.1', 'M',
        ['Given 服务运行 / When 暴露指标 / Then Prometheus 兼容端点仅内网可达并包含投递延迟、连接数、ACK 率、DB 连接池、推送/文件/风控/开放 API 错误率的占位指标',
         'Given 日志 / When 输出 / Then 为结构化 JSON 带 trace_id，令牌、手机号、消息正文被脱敏过滤器移除',
         'Given 告警规则 / When 指标越界 / Then 触发告警并路由到运维通道；监控端点本身需认证且不对公网开放（T-17）'],
        ['安全负面：监控/调试端点公网不可达；日志脱敏扫描（注入令牌样例）', '集成：OpenTelemetry 追踪贯通 API→worker→imgw', '回归：关键指标在 /metrics 中存在'],
        'S:指标/日志/追踪库与中间件|P:/metrics（内网）|B:无|R:监控端点认证|E:监控冒烟与脱敏 E2E', 'Prometheus/Grafana 或兼容替代、告警通道（Phase 2 运维决策）', '指标仅追加；可关闭追踪采样')
    new('1.5', '客户端构建基线与现代化（四端）', '客户端开发者', '四端客户端在隔离环境可复现构建，依赖与包名问题清零', '后续各端界面与功能 Story 有可编译、可测试的起点（R-12/13/15/17/19）',
        'NFR-5,NFR-13,NFR-14', 'AD-17,AD-18', '1.1', 'L',
        ['Given Android / When 构建 / Then 使用 VeyroGram 包名而非 com.test.ts，应用 ID 与 R 导入重构完成，debug APK 可复现构建',
         'Given iOS / When 构建 / Then 过期 Pods（含 AFNetworking）已替换或升级，在 Xcode 26 工具链下无签名模拟器构建通过',
         'Given Web 与桌面 / When 构建 / Then 锁文件不含第三方镜像 URL，React/Electron 升级到受支持版本，高危漏洞清零，Manager 使用官方源冻结安装'],
        ['可复现构建：四端在干净环境各构建一次并保存日志', '安全负面：锁文件镜像 URL 扫描；Electron 安全配置检查', 'SCA：npm/Gradle/Pods 高危为零'],
        'A:包名与依赖重构|I:Pods 现代化|W:依赖升级与锁文件|D:Electron 升级|M:依赖与构建|P:无|E:四端构建流水线', '包名/应用 ID 以 PRD OQ-1 核验结果为准；Manager/Web 授权风险已由用户接受', '依赖升级以分支验证，失败可回退锁文件')
    new('1.6', '生成客户端与各端 IM SDK 接入', '客户端开发者', '由 OpenAPI 生成的各端客户端与固定版本的 IM SDK 接入层', '各端不各自实现协议与状态规则（AD-17）',
        'NFR-12', 'AD-8,AD-17', '1.2,1.3,1.5', 'M',
        ['Given OpenAPI 契约 / When 生成 / Then 产出 TS/Kotlin/Swift/Go 客户端并随版本发布',
         'Given IM SDK / When 接入 / Then 固定到与服务端 WuKongIM 兼容的版本，并有对该固定版本的契约测试',
         'Given 令牌 / When 保存 / Then 使用系统安全存储，校验服务器证书，敏感数据排除系统备份'],
        ['合约：生成客户端对契约样例通过', '安全负面：证书校验失败拒绝连接；备份排除验证', '兼容：IM SDK 与 WuKongIM 固定 SHA 互通'],
        'A:IM SDK 与安全存储|I:IM SDK 与钥匙串|W:IM SDK|D:IM SDK|P:/openapi 生成|E:SDK 联通 E2E', 'IM SDK 版本与服务端固定 SHA 的兼容矩阵', 'SDK 版本锁定，升级走兼容测试')
    new('1.13', '安装向导与运维套件', '平台所有者', '环境预检、数据库迁移、首次管理员初始化与配置管理一体的安装向导', '原生 systemd 部署可重复且不依赖 install.lock 作为鉴权',
        'NFR-10,NFR-3', 'AD-14,AD-20', '1.1,1.2,1.11', 'L',
        ['Given 全新主机 / When 运行预检 / Then 检查 MySQL 8.0/Redis 7/Nginx/端口/权限并给出阻断项',
         'Given 安装 / When 完成 / Then 创建独立 dev/test/staging/production 配置模板、日志轮转、滚动/灰度发布与回滚脚本',
         'Given 已安装系统 / When 删除 install.lock 后再次运行向导 / Then 拒绝重新初始化（见 1.11）'],
        ['集成：干净主机安装与回滚演练', '安全负面：重复初始化、弱配置被拒', '运维：日志轮转与健康探针'],
        'S:安装向导与脚本|P:无|B:迁移执行|E:安装与升级演练', '环境分层与主机划分（PRD OQ-7）', '每步可回滚；脚本幂等')
    new('1.14', '备份实现（MySQL PITR、IM 存储、对象存储）', '平台所有者', '加密的 MySQL 全量+binlog 备份、WuKongIM 数据目录快照、对象存储版本化', '试运营数据从第一天起就有备份，20.2 才有可演练的对象',
        'NFR-6', 'AD-22', '1.2,1.13', 'M',
        ['Given 生产数据库 / When 备份 / Then 全量+binlog 加密存放到主机外并限权',
         'Given WuKongIM 数据目录 / When 快照 / Then 有文档化的静默与一致性步骤并可恢复',
         'Given 对象存储 / When 配置 / Then 开启版本化与生命周期'],
        ['集成：在 staging 恢复一次并核对数据', '安全负面：备份权限与加密验证', '监控：备份失败告警'],
        'S:备份与恢复脚本|B:PITR 配置|E:恢复冒烟', 'RPO/RTO 目标（PRD OQ-7）；备份存储位置', '备份脚本只读源，不影响在线服务')
    new('1.15', 'WuKongIM fork、补丁与升级流程', '平台工程团队', '固定 SHA 的 WuKongIM fork、补丁（webhook HMAC）与升级验证流程', '内核升级可控，无签名 webhook 等已知缺口有闭环',
        'NFR-2,NFR-5', 'AD-2,AD-18', '1.3', 'M',
        ['Given fork / When 建立 / Then 保留 upstream remote、记录固定 SHA 与 go1.26.10 工具链，变更保留许可证与署名',
         'Given webhook 无签名 / When 补丁 / Then 增加 HMAC 签名与时间戳校验并在 imgw 校验',
         'Given 升级 / When 切换 SHA / Then 运行 imgw 契约测试与核心 E2E 才能放行'],
        ['契约：imgw 对新 SHA 的接口契约', '安全负面：伪造/重放 webhook 被拒', '回归：升级后核心 E2E'],
        'S:fork 与补丁|P:内部 webhook 契约|E:升级回归', 'WuKongIM 目前为 v3 beta，升级需谨慎（R-29）', '可切回上一固定 SHA')
    new('1.16', '上游兼容与数据迁移决策', '产品负责人', '决定是否需要兼容或迁移上游 TangSengDaoDao 的数据与旧客户端，并落实', '避免默默破坏既有部署或假设“绿地”',
        'NFR-12', 'AD-8', '1.3', 'M',
        ['Given 上游数据表与旧路径 / When 评估 / Then 输出书面决策：是否提供 compat 层与数据迁移、兼容窗口与下线日期',
         'Given 决定提供迁移 / When 执行 / Then 迁移脚本幂等、可回滚并有校验报告',
         'Given 决定不提供 / When 发布 / Then 在文档明确声明并保留“全新部署”路径'],
        ['迁移：样例库迁移演练', '回滚：迁移失败回退', '契约：compat 层路径清单'],
        'S:compat 模块或明确不做|B:迁移脚本|P:compat 路径|E:迁移 E2E', '依赖 Server modules/* 的逐表审计（现为 UNVERIFIED）', '迁移前全量备份')
    # ---------------------------------------------------------------- dependencies that needed the new foundation (C1/C2)
    by('1.7')['deps'] = '1.2,1.3,1.5,1.6'
    by('1.9')['deps'] = rd('1.5') if False else '1.8,1.6'
    by('1.4')['deps'] = '1.1'
    for sid in ('7.4', '17.5', '20.1'):
        d = by(sid); d['deps'] = ','.join([x for x in d['deps'].split(',') if x] + ['1.4'])
    # ---------------------------------------------------------------- C3: token revocation vs AD-9 (deny-list on ALL routes) + device_quit limit
    d = by('1.8')
    d['acs'].append('Given 访问令牌 / When 设备被撤销 / Then Redis 拒绝列表覆盖全部已认证路由（每请求一次查询），Redis 不可用时降级为令牌 TTL（≤15 分钟）并告警')
    d['acs'].append('Given 多节点 IM 集群 / When 调用 /user/device_quit / Then 仅保证 owner-local 会话关闭；v1 按单节点交付，集群上线前必须通过踢线传播测试')
    d['tests'].append('降级：Redis 故障时撤销生效时间退化为 TTL，告警触发')
    # ---------------------------------------------------------------- C4: send path story
    d = by('1.9')
    d['tests'][0] = '安全负面：IM 网关直发时，IM 原生 send_ban/黑名单与 before_send 拒绝（客户端经 SDK 直连 IM 是 v1 的正常发送路径；/v1/messages/* 只承载业务扩展消息）（T-23）'
    d['acs'][2] = 'Given 测试夹具在 IM 中写入禁言/黑名单规则 / When 该用户经 IM 网关发送 / Then 被 IM 原生 send_ban/黑名单拒绝；Given before_send 超时 / When 受限规则类 / Then fail-closed'
    d['tests'].append('故障注入：before_send 超时与业务服务不可达时受限规则 fail-closed')
    # ---------------------------------------------------------------- C5: 5.3 (device lock) contains the approval contract; QR login stays in 5.7
    d = by('5.3'); d['title'] = '设备管理、设备锁与新设备批准流程'
    d['acs'][1] = 'Given 开启设备锁 / When 新设备登录 / Then 需验证手机号验证码（对齐官方设备锁基线），或经已信任设备批准；批准请求经 IM 系统消息送达已信任设备（推送增强见 7.4），超时 4 分钟'
    d['acs'].append('Given 用户拒绝新设备 / When 选择“不是我” / Then 请求作废并提示修改密码，写 login_event')
    d['want'] = '查看登录设备并下线，开启设备锁，并通过手机号验证或已信任设备批准新设备'
    by('5.7')['acs'][1] = 'Given 扫码登录会话 / When 未在手机端确认 / Then 不能读取历史消息'
    # ---------------------------------------------------------------- H1: phases per story (PRD §6.1 / BMAD §11)
    phase = {}
    for sid in ('3.5', '4.3', '4.4', '4.5', '4.6', '4.7', '5.2', '5.3', '5.4', '5.5', '5.6', '5.7'): phase[sid] = 'Phase 4'
    phase['13.5'] = 'Phase 3'; phase['13.6'] = 'Phase 3'
    by('13.6')['deps'] = '2.4'
    by('13.6')['reqs'] += ',FR-34'
    by('13.5')['title'] = '会话内查找（日期/成员/类型）与定位跳转'
    by('13.5')['acs'].append('Given 查找结果 / When 点击 / Then 跳转到原消息位置并高亮提醒')
    by('5.2')['deps'] = '2.6,7.1'
    by('5.2')['acs'][0] = 'Given 关闭已读回执 / When 保存 / Then 偏好服务端强制并多设备同步，13.1 的已读详情与回执消费该偏好'
    # ---------------------------------------------------------------- H2: scan gate before download
    d = by('10.3'); d['deps'] = '2.2,8.3'; d['title'] = '文件上传、扫描门、预览、转发与签名下载'
    d['acs'].append('Given 文件上传 / When 状态机运行 / Then uploaded→scanning→clean|quarantined，仅 clean 可下载，扫描引擎不可用时保持 scanning；使用 8.3 提供的扫描适配器')
    d['acs'].append('Given 会话搜索与存储 / When 切换提供方 / Then 业务代码不变（对象存储端口），图片缩略图/变换由服务端生成；搜索结果包含文件（供 7.2 使用）')
    d = by('10.4'); d['title'] = '文件隔离复核与存储生命周期'
    d['acs'][0] = 'Given 命中隔离 / When 后台复核 / Then 仅见元数据，误隔离可申请复核并恢复'
    d['acs'][2] = 'Given 会话被删除或文件到期 / When 到期 / Then 关联对象在期限内被清理且旧 URL 在 TTL 内失效'
    d = by('8.3')
    d['acs'][1] = 'Given 扫描适配器 / When 提供接口 / Then 10.3 与 11.2 复用它；用测试夹具验证危险扩展名与魔数不符会被判隔离（上传入口在 10.3）'
    d['deps'] = '8.2'
    # ---------------------------------------------------------------- H3: "immediate URL invalidation" -> TTL semantics (AD-6)
    for sid in ('2.4', '7.3', '10.3', '12.4'):
        d = by(sid)
        d['acs'] = [a.replace('立即失效', '在签名 URL 的 TTL（≤5 分钟）内失效，敏感内容改走 API 代理下载则立即失效').replace('立即不可见', '在缓存与 URL 的 TTL 内不可见') for a in d['acs']]
        d['tests'] = [t.replace('后媒体 URL 失效', '后媒体 URL 在 TTL 内失效').replace('立即失效', 'TTL 内失效') for t in d['tests']]
    by('2.4')['acs'][1] = 'Given 消息已撤回 / When 通过历史同步、搜索、预览读取 / Then 服务端均不返回原内容；下载 URL 在 TTL（≤5 分钟）内失效，敏感内容走代理下载则立即失效'
    by('12.4')['acs'][1] = 'Given 下架 / When 完成 / Then 对所有人在列表与接口层立即不可见并写审计，媒体 URL 在 TTL（≤5 分钟）内失效，缓存与通知被清理'
    # ---------------------------------------------------------------- H5: admin MFA and install.lock
    d = by('1.11')
    d['acs'].append('Given 管理员账号 / When 首次登录 / Then 强制绑定 TOTP 二次验证（管理面独立于用户侧 MFA）；无 MFA 的管理员无法访问任何管理接口')
    d['tests'].append('安全负面：删除 install.lock 后重新初始化被拒；仅持有口令无 MFA 无法登录管理面')
    # ---------------------------------------------------------------- H6: missing security negative tests
    by('16.6')['acs'].append('Given Webhook 回调 URL / When 注册或投递 / Then 禁止内网、回环、链路本地地址，并防 DNS 重绑定（投递时重新解析并校验）')
    by('16.6')['tests'].append('安全负面：Webhook SSRF（内网、元数据地址、DNS 重绑定）')
    by('14.1')['tests'].append('安全负面：访客消息存储型 XSS（客服后台渲染必须转义/沙箱）、匿名访客滥用限频')
    by('14.2')['tests'].append('安全负面：访客输入在工作台/工单/质检页的 XSS 与 HTML 注入')
    by('18.2')['acs'].append('Given 订单金额与币种 / When 客户端或回调篡改 / Then 以服务端订单为准，金额/币种不一致拒绝，退款不得超过实付')
    by('18.2')['tests'].append('安全负面：金额/币种篡改、超额退款、回调乱序')
    d = by('3.3')
    d['acs'][0] = 'Given 用户授权通讯录 / When 匹配 / Then 使用带服务端密钥的 HMAC（非裸哈希）与限频接口，原始通讯录不落盘明文，防止对手机号空间的暴力还原'
    d['tests'].append('安全负面：手机号空间枚举/彩虹表还原尝试被限频与密钥化阻断')
    # ---------------------------------------------------------------- H7: mobile payment compliance
    d = by('18.2')
    d['acs'].append('Given 移动端数字内容订阅 / When 上架 / Then 通过商店内购（Apple IAP / Google Play Billing）并做服务端回执校验；或在 v1 明确不在移动端售卖并仅提供 Web 支付，由 OQ-3/OQ-5 决策')
    d['cfg'] += '；Phase 6 Exit 以 OQ-3/OQ-5 决策为条件'
    # ---------------------------------------------------------------- H8: 14.1 phase inversion
    d = by('14.1')
    d['title'] = '应用内客服入口、技能组与自动分配'
    d['acs'][0] = 'Given 通讯录内专属客服入口 / When 登录用户咨询 / Then 按技能组与负载自动分配，客服与用户身份有明确标记'
    d['acs'][2] = 'Given 登录用户会话 / When 查看 / Then 会话与技能组隔离，仅暴露最小资料（官网匿名访客入口见 19.3）'
    d['want'] = '从通讯录进入专属客服咨询并自动匹配合适客服'
    by('14.2')['acs'].append('Given 主管 / When 需要 / Then 可手动分配与改派会话并写审计')
    new('19.3', '官网访客客服入口与匿名访客认证', '访客', '在官网无需登录即可咨询客服，访客身份单独认证并受限', '未注册用户也能获得帮助而不扩大攻击面',
        'FR-38', 'AD-3,AD-10,AD-16', '14.2,19.1', 'M',
        ['Given 匿名访客 / When 发起咨询 / Then 获得短期、仅限客服会话范围的访客令牌，不能访问任何用户 API',
         'Given 访客滥用 / When 超限 / Then 按 IP/指纹限频并可封禁',
         'Given 访客资料 / When 客服查看 / Then 仅展示最小字段并受保留策略约束'],
        ['安全负面：访客令牌越权、会话枚举、XSS', 'E2E：官网咨询到分配', '合规：访客数据保留（OQ-4）'],
        'S:访客认证与会话|W:官网访客组件|M:访客管理|P:/v1/support/visitor|B:sp_session 访客字段|R:访客令牌范围|E:访客 E2E', '访客令牌 TTL 与限频', '访客入口可整体关闭')
    # ---------------------------------------------------------------- dependency fixes (review §2.1)
    d = by('2.7')
    d['acs'][0] = 'Given 用户清空会话 / When 确认 / Then 服务端记录该用户的 cleared_before_seq，多设备同步隐藏历史（保留上游“清空聊天记录”语义，仅影响本人可见范围），其他成员不受影响'
    d['acs'][1] = 'Given WuKongIM 无单条物理删除接口 / When 需要物理清理 / Then 能力边界写入文档：用墓碑/内容替换实现，物理清理走 9.1 的审批流程'
    d['cfg'] = '保留期（OQ-4 待法务）；决策：恢复用户侧“清空”语义，管理员物理清理另需审批（见 9.1）'
    by('9.1')['acs'].append('Given 管理员物理清理远程记录或删除消息 / When 发起 / Then 需要审批、理由并写审计')
    by('3.1')['acs'][2] = 'Given 重复或超限申请 / When 提交 / Then 返回统一错误（被拉黑的统一表现由 3.2 保证）'
    d = by('3.2')
    d['acs'][1] = 'Given 拉黑生效 / When 涉及群邀请或动态可见性 / Then 由 4.4 与 12.1 接入同一黑名单服务（本 Story 提供查询契约）'
    d = by('3.3')
    d['acs'][2] = 'Given 二维码 / When 过期或被撤销 / Then 失效；群二维码入群的邀请确认由 4.4 实现'
    d['acs'].append('Given 用户拒绝通讯录权限 / When 再次进入 / Then 功能降级并给出设置引导，不循环弹窗')
    d['acs'].append('Given 扫描个人名片二维码 / When 识别 / Then 需用户确认后才发起添加')
    d = by('4.1')
    d['acs'][1] = 'Given 邀请成员 / When 对方已被拉黑 / Then 被拒；群黑名单见 4.4'
    d['acs'].append('Given 创建群 / When 未设置群名与头像 / Then 默认群名由成员昵称拼接，群头像由成员头像拼图')
    d['acs'].append('Given 群主退出 / When 确认 / Then 自动指定第二位入群成员为群主')
    d = by('4.4')
    d['acs'].append('Given 群二维码 / When 扫码入群 / Then 遵守邀请确认设置与群黑名单')
    d['acs'].append('Given 邀请 / When 对方已被邀请人拉黑 / Then 被拒绝（沿用 3.2 黑名单服务）')
    by('4.3')['acs'].append('Given 群头像设置 / When 群主或管理员修改 / Then 校验权限并经内容审核（9.3）后生效')
    by('4.8')['acs'].append('Given 群备注与群内昵称 / When 设置 / Then 保存并可选择是否显示群昵称')
    by('4.5')['acs'].append('Given 点击成员头像 / When 管理员 / Then 可查看资料并单独设置禁言及时长')
    by('5.1')['acs'][2] = 'Given 头像文件 / When 上传 / Then 校验魔数、格式与大小；违规内容的审核由 9.3 处理'
    by('5.1')['acs'].append('Given 平台短号 / When 修改 / Then 只允许修改一次')
    by('9.3')['acs'].append('Given 用户头像与群头像 / When 命中规则或被举报 / Then 进入审核队列，违规被替换并通知')
    by('7.2')['deps'] = '2.4,3.4,4.1'
    by('7.2')['acs'][0] = 'Given 搜索联系人/群 / When 查询 / Then 结果受成员资格与可见性约束（文件搜索由 10.3 启用）'
    by('7.2')['acs'].append('Given 本地索引 / When 用户登出或注销 / Then 索引被擦除；索引在设备上加密存储且用户可手动清除')
    by('7.3')['acs'][0] = 'Given 会话 / When 打开媒体中心 / Then 先展示图片与链接，视频与文件由 11.2 与 10.3 交付后接入；内容均受成员资格与撤回约束'
    d = by('8.1'); d['want'] = '举报用户、群、消息，并对处置申诉（动态举报见 12.4）'
    d['acs'].append('Given 群设置与用户资料页 / When 点击投诉 / Then 可投诉本群或该用户并进入举报队列（9.3 通用化队列）')
    by('7.1')['acs'].append('Given 长按会话 / When 菜单 / Then 提供置顶、免打扰、删除聊天；置顶与免打扰跨设备同步')
    by('7.1')['acs'].append('Given 基础在线状态 / When 单聊与群聊 / Then 单聊显示在线或一小时内最近在线时长，群标题显示总人数与在线人数（遵守隐私默认；13.3 提供隐私控制与细化）')
    by('7.1')['acs'].append('Given 官方/系统账号 / When 出现在会话 / Then 显示官方标识（账号实体由 9.4 配置）')
    by('3.4')['acs'].append('Given 联系人列表 / When 浏览 / Then 显示在线时长；名片显示入群方式与时间；官方与机器人账号分类展示（机器人由 16.7 提供）')
    by('2.2')['acs'].append('Given 文本消息含系统 emoji 或自定义表情图片 / When 渲染 / Then 各端一致显示（表情商店由 11.3/11.5 提供数据）')
    by('2.6')['acs'].append('Given 向非好友发送消息 / When 发送 / Then 会话中提示需要先加好友或对方设置了限制（统一措辞，不泄露拉黑）')
    by('9.4')['acs'][1] = 'Given 版本配置 / When 保存 / Then 保存最低版本、强制更新开关与灰度比例字段（客户端行为见 19.2，发布演练见 20.6）'
    by('9.4')['acs'].append('Given 官方/系统账号 / When 配置 / Then 在后台创建系统账号与官方标识，客户端据此显示')
    by('9.4')['acs'].append('Given 客户端“关于/检查新版本” / When 点击 / Then 读取版本配置并提示（升级行为见 19.2）')
    by('5.8')['acs'].append('Given 聊天背景 / When 设置与清除 / Then 保存并多设备同步；清空缓存清除本地媒体缓存且不影响云端数据')
    by('1.8')['acs'].append('Given 用户退出登录 / When 确认 / Then 撤销该设备令牌与 IM 令牌并清除本地凭据')
    by('5.6')['acs'].append('Given 已注销用户 / When 他人尝试拉入群或搜索 / Then 不可被拉入群并提示“该用户已注销”，搜索不返回')
    by('13.6')['acs'].append('Given 敏感会话 / When 启用防截屏（拦截） / Then Android 使用 FLAG_SECURE、iOS 使用安全视图；平台不支持处明确声明不提供拦截')
    by('13.4')['acs'].append('Given 上传的图片 / When 服务端处理 / Then 服务端也剥离 EXIF 位置等元数据')
    by('13.3')['deps'] = '2.6,7.1'
    d = by('12.1'); d['deps'] = '10.3,3.5,10.2,11.2,7.4'
    d['acs'].append('Given 黑名单关系 / When 计算可见性 / Then 同步生效（沿用 3.2 黑名单服务）')
    by('12.3')['deps'] = '12.2,7.4'
    d = by('12.2'); d['deps'] = '12.1,10.1'
    d['acs'].append('Given 动态中的文本与图片 / When 长按 / Then 可复制、收藏（进入 10.1 的我的收藏）、转发或保存')
    d = by('10.1'); d['acs'].append('Given 收藏删除 / When 同步 / Then 多设备在 10 秒内一致 [目标值待测]')
    d['reqs'] += ',FR-20'
    d = by('11.2'); d['acs'].append('Given 会话媒体中心 / When 有视频 / Then 视频接入 7.3 的媒体中心')
    by('11.4')['deps'] = '11.3,9.3'
    by('11.5')['deps'] = '11.3,9.6'
    d = by('15.1'); d['deps'] = '15.0,1.9,7.4,2.1'
    d['acs'].append('Given 通话记录 / When 生成 / Then 使用已登记的通话记录消息类型（2.1），各端渲染与降级一致')
    d = by('15.2'); d['deps'] = '15.1,7.4'
    d['acs'].append('Given 后台或锁屏 / When 来电 / Then iOS 使用 VoIP 推送 + CallKit，Android 使用全屏意图 + ConnectionService（依赖 Apple 账号主体，OQ-5）')
    by('20.1')['deps'] = '1.9,13.1,1.4,4.1,7.4,15.3'
    by('20.2')['deps'] = '1.14'
    by('17.2')['acs'].append('Given 小程序运行 / When 网络与资源 / Then 网络与导航受域名白名单限制，应用 CSP 生效，内存与资源有上限，存储/Cookie 按应用分区')
    by('16.3')['reqs'] += ''
    by('16.3')['tests'].append('安全负面：Secret 不出现在日志、响应与错误信息中；轮换后旧密钥失效')
    by('19.2')['tests'].append('安全负面：降级攻击（回退到有漏洞的旧版本）被拒绝')
    by('10.2')['tests'].append('安全负面：位置权限最小化，禁止后台定位；伪造位置不影响他人权限')
    by('15.1')['tests'].append('安全负面：骚扰性频繁拨号限频；跨会话拨号越权')
    by('7.4')['tests'].append('安全负面：推送令牌劫持/重绑定、跨用户推送')
    by('2.2')['tests'].append('安全负面：多格式混合（polyglot）图片文件被拒绝')
    by('2.3')['tests'].append('安全负面：超长/畸形音频被拒绝')
    by('5.6')['tests'].append('安全负面：已注销用户的令牌、搜索与群关系全部失效')
    by('1.7')['tests'].append('安全负面：短信轰炸（SMS pumping）与账号枚举被限频、统一响应')
    by('1.2')['acs'].append('Given 审计日志 / When 每日 / Then 链头哈希锚定到主机外存储（对象存储版本化/WORM 或独立日志主机）')
    # ---------------------------------------------------------------- M-1 split: vendor push channels out of 7.4
    d = by('7.4'); d['title'] = '推送通道（APNs/FCM）与内容脱敏'
    d['acs'][1] = 'Given 通道 / When 设备注册 / Then 按设备类别选择 APNs 或 FCM，密钥不入源码；国内厂商通道见 7.6'
    new('7.6', '国内厂商推送通道与 Web Push', '终端用户', '在华为、小米、OPPO、VIVO 手机和浏览器上收到通知', '国内 Android 设备也能可靠收到消息',
        'FR-42', 'AD-7', '7.4', 'M',
        ['Given 厂商设备 / When 注册 / Then 使用对应厂商通道，密钥通过受保护配置注入，不入源码',
         'Given 通道不可用 / When 发送 / Then 回退到 FCM 或应用内同步并记录失败率',
         'Given 浏览器 / When 用户授权 / Then 支持 Web Push，内容可脱敏'],
        ['集成：各厂商沙箱推送', '安全负面：密钥扫描、令牌跨用户', '监控：分通道成功率'],
        'S:厂商通道适配器|A:华为/小米/OPPO/VIVO 集成|I:无|W:Web Push|D:系统通知|P:/v1/push|B:push_token|N:厂商通道|R:仅本人设备|E:厂商推送 E2E', '各厂商开发者账号与密钥（OQ-5）', '通道可逐一关闭')
    # ---------------------------------------------------------------- M-8: console split from SDK
    d = by('16.8'); d['title'] = 'SDK 生成与版本化'; d['want'] = '使用 TS/Go/Android/iOS SDK'
    d['reqs'] = 'OP-FR-10,FR-71'
    d['acs'][1] = 'Given SDK 版本 / When 发布 / Then 带兼容窗口与变更日志'
    d['acs'][2] = 'Given SDK 示例 / When CI / Then 在沙箱上端到端运行'
    new('16.9', '开发者控制台', '开发者', '在控制台管理应用、密钥、授权、版本与团队', '集中完成接入与运营',
        'OP-FR-23,FR-71', 'AD-8,AD-17', '16.8,6.1', 'L',
        ['Given 控制台每页 / When 渲染 / Then 具备加载/空/错误/无权限状态，密钥展示块一次性展示并可复制',
         'Given 控制台 / When 键盘与读屏 / Then 可完成全部操作',
         'Given 页面 / When 1024–1440 宽 / Then 无横向滚动'],
        ['视觉回归与 a11y 扫描', 'E2E：控制台核心流程', '安全负面：越权访问他人应用'],
        'S:控制台后端接口|W:控制台前端|P:/open/v1/*|R:团队角色|E:控制台 E2E', '控制台前端框架（不沿用上游外观）', '前端独立部署可回滚')
    # ---------------------------------------------------------------- M-6: FR consequence ACs
    by('7.1')['acs'].append('Given 置顶或免打扰变更 / When 保存 / Then 在其他设备 5 秒内一致 [目标值待测]')
    by('3.2')['acs'].append('Given 被拉黑方 / When 查看拉黑方资料 / Then 不可查看（统一失败，不泄露拉黑）')
    by('6.1')['acs'].append('Given 任何端的界面 / When 评审 / Then 不得保留上游页面样式或仅替换 Logo/主题色，需对照设计规范')
    by('6.3')['acs'].append('Given 矢量资产 / When 生成 / Then 由 SVG 源导出，禁止用位图放大冒充矢量')
    by('9.1')['acs'].append('Given 删除用户消息 / When 管理员发起 / Then 需审批并写审计（FR-46）')
    # ---------------------------------------------------------------- L-2: specific N/A explanations
    def na(sid, k, txt):
        d = by(sid); parts = [p for p in d['x'].split('|') if not p.startswith(k + ':')]; parts.append(f'{k}:无（{txt}）'); d['x'] = '|'.join(parts)
    na('1.8', 'D', '桌面端沿用 Web 壳的设备页，见 6.4')
    na('8.2', 'A', '风控为服务端能力，客户端不新增界面；被限速时显示统一提示')
    na('10.2', 'W', 'Web/桌面仅查看位置，不发送位置（无可靠定位入口），提供手动输入地址替代')
    na('14.2', 'A', '用户侧沿用 14.1 的会话界面，工作台仅限客服（Web/后台）')
    na('5.8', 'M', '后台管理员偏好沿用 9.6 的后台设置')
    na('2.4', 'N', '撤回不发推送，仅更新会话内提示')
    na('13.2', 'N', '编辑不发推送，仅更新会话内标记')
    # ---------------------------------------------------------------- Phase 2 restructure: every Phase 2 story <= M, server/client separated (review M-1)
    old = {d['id']: d for d in D if d['id'].startswith('1.')}
    def mk(nid, base, title, acs, deps, size='M', x=None, tests=None, want=None, why=None, reqs=None, ads=None, cfg=None, rb=None, who=None):
        b = old[base]
        return dict(b, id=nid, title=title, acs=acs, deps=deps, size=size, x=x if x is not None else b['x'], tests=tests if tests is not None else b['tests'],
                    want=want or b['want'], why=why or b['why'], reqs=reqs or b['reqs'], ads=ads or b['ads'], cfg=cfg or b['cfg'], rb=rb or b['rb'], who=who or b['who'])
    A = lambda base, *idx: [old[base]['acs'][i] for i in idx]
    P2 = []
    P2.append(mk('1.1', '1.1', '服务骨架、配置校验与健康探针', A('1.1', 0, 1) + ['Given 服务启动 / When 访问 /healthz 与 /readyz / Then 仅内网可达，依赖未就绪时 readyz 返回失败'], '',
                 x='S:服务骨架与配置|P:健康/就绪探针|B:无|E:启动与配置校验测试', tests=old['1.1']['tests'][:2]))
    P2.append(mk('1.2', '1.1', '原生部署骨架与 CI 基线', [old['1.1']['acs'][1], old['1.1']['acs'][2], 'Given 部署产物 / When 构建 / Then 二进制与 systemd unit 版本化、可回滚，Nginx 模板仅转发白名单路径'], '1.1',
                 x='S:systemd unit 与 Nginx 模板|P:无|B:无|E:安装与回滚演练', tests=[old['1.1']['tests'][2], 'CI：lint、go test -race、OpenAPI diff、依赖方向检查全部为必过门禁'],
                 want='原生 systemd 部署骨架与 CI 基线', why='后续每个 Story 都能被可重复地构建、部署与回滚', title_only=None) if False else
              mk('1.2', '1.1', '原生部署骨架与 CI 基线', [old['1.1']['acs'][1], old['1.1']['acs'][2], 'Given 部署产物 / When 构建 / Then 二进制与 systemd unit 版本化、可回滚，Nginx 模板仅转发白名单路径'], '1.1',
                 x='S:systemd unit 与 Nginx 模板|P:无|B:无|E:安装与回滚演练', tests=[old['1.1']['tests'][2], 'CI：lint、go test -race、OpenAPI diff、依赖方向检查全部为必过门禁'],
                 want='原生 systemd 部署骨架与 CI 基线', why='后续每个 Story 都能被可重复地构建、部署与回滚'))
    P2.append(mk('1.3', '1.2', '迁移基座与幂等表', A('1.2', 0) + ['Given 幂等键 / When 同一请求重复提交 / Then 按 Idempotency-Key 24 小时内去重'], '1.1',
                 x='S:迁移工具与幂等中间件|P:Idempotency-Key|B:schema_migration、idempotency_key|E:迁移与回滚演练', tests=[old['1.2']['tests'][0]], want='带版本的前向迁移与幂等键表', why='数据库变更可追踪、可回滚且重复请求不产生副作用'))
    P2.append(mk('1.4', '1.2', 'Outbox、worker 与审计日志', A('1.2', 1, 2, 3), '1.3',
                 x='S:outbox worker 与审计写入|B:outbox、dead_letter、audit_log|P:无|E:并发与审计校验', tests=old['1.2']['tests'][1:], want='事务 outbox、worker 与追加写审计日志', why='状态变更与事件原子提交，高危操作可追溯且不可被应用账号篡改'))
    P2.append(mk('1.5', '1.4', old['1.4']['title'], old['1.4']['acs'], '1.1'))
    P2.append(mk('1.6', '1.3', 'WuKongIM imgw 适配器与内部 webhook', A('1.3', 2) + ['Given imgw 适配器 / When 调用 IM API / Then 只经回环/私网，调用失败重试并上报指标', 'Given 固定的 WuKongIM SHA / When 运行契约测试 / Then 接口契约通过'], '1.1',
                 x='S:imgw 适配器、内部 webhook 监听|P:内部接口|B:无|E:适配器契约测试', tests=[old['1.3']['tests'][1], old['1.3']['tests'][2]], want='业务服务通过 imgw 适配器调用 WuKongIM 并接收内部 webhook', why='所有对 IM 内核的访问有唯一受控入口'))
    P2.append(mk('1.7', '1.3', '暴露面加固与负面测试', A('1.3', 0, 1), '1.6,1.2',
                 x='S:部署校验与加固脚本|P:无|B:无|E:暴露面负面测试', tests=[old['1.3']['tests'][0]], want='WuKongIM 与其他内部端口对公网与非白名单来源不可达，默认密钥全部覆盖', why='堵住无鉴权发信/改令牌的攻击面（R-21）'))
    P2.append(mk('1.8', '1.15', old['1.15']['title'], old['1.15']['acs'], '1.6'))
    P2.append(mk('1.9', '1.5', '客户端构建基线：Android 与 iOS', A('1.5', 0, 1), '1.2', x='A:包名与依赖重构|I:Pods 现代化|P:无|E:移动端构建流水线', tests=[old['1.5']['tests'][0], old['1.5']['tests'][2]]))
    P2.append(mk('1.10', '1.5', '客户端构建基线：Web、桌面与后台', A('1.5', 2), '1.2', x='W:依赖升级与锁文件|D:Electron 升级|M:依赖与构建|P:无|E:Web/桌面/后台构建流水线', tests=old['1.5']['tests'][1:]))
    P2.append(mk('1.11', '1.6', old['1.6']['title'], old['1.6']['acs'], '1.4,1.6,1.9,1.10'))
    P2.append(mk('1.12', '1.7', '手机号注册与登录（服务端）', old['1.7']['acs'], '1.3,1.6,1.11', x='S:认证服务与短信适配器|P:/v1/auth/*|B:user、device、login_event|R:匿名接口限流|E:登录接口 E2E'))
    P2.append(mk('1.13', '1.8', '设备、令牌与会话撤销（服务端）', old['1.8']['acs'], '1.12', x='S:设备与令牌服务|P:/v1/devices/*|B:device_session、refresh_token、im_token|N:新设备登录提醒|R:仅本人可撤销|E:撤销接口 E2E'))
    cl_login = ['Given 登录页 / When 输入手机号与验证码 / Then 登录成功并把令牌保存到系统安全存储，错误与限流有明确提示',
                'Given 设备列表 / When 打开 / Then 显示当前设备与其他设备并可下线（界面使用设计令牌与图标库，不沿用上游样式）',
                'Given 键盘与读屏 / When 操作登录与设备页 / Then 全部可达、焦点环可见、控件有名称']
    P2.append(mk('1.14', '1.7', '登录与设备页客户端（四端最小实现）', cl_login, '1.11,1.13', size='M', x='A:登录与设备页|I:登录与设备页|W:登录与设备页|D:登录与设备页|P:无|E:四端登录 E2E',
                 tests=['E2E：四端登录与下线', '无障碍：a11y_scan 与人工键盘检查', '安全负面：令牌不出现在日志与备份中'], want='在四端用手机号登录并管理设备', why='Phase 2 的登录纵切在四端可用', reqs='FR-1,FR-3', ads='AD-17,AD-9',
                 cfg='使用 packages/design-tokens 与 brand/icons；这是最小实现，Phase 3 在设计门禁签字后按完整规范重构', rb='客户端功能可随版本回滚'))
    P2.append(mk('1.15', '1.9', '单聊消息收发（服务端与 before_send 骨架）', old['1.9']['acs'], '1.13,1.11', x='S:message 模块与 before_send 骨架|P:/v1/messages/*|B:message_ref|R:会话成员校验|E:消息接口 E2E'))
    P2.append(mk('1.16', '1.10', '离线同步与多端一致（服务端）', old['1.10']['acs'], '1.15', x='S:同步服务与游标|P:/v1/sync/*|B:draft、conversation_setting|R:成员资格校验|E:同步接口 E2E'))
    cl_chat = ['Given 两台设备 / When 一端发送文本 / Then 另一端按序收到，重试不重复，失败显示重试入口',
               'Given 离线后重连 / When 同步 / Then 消息、未读与草稿补齐',
               'Given 聊天页 / When 键盘与读屏 / Then 消息列表为 log 区域、发送与重试可操作']
    P2.append(mk('1.17', '1.9', '聊天与同步客户端：Android 与 iOS', cl_chat, '1.14,1.15,1.16', x='A:聊天页与同步|I:聊天页与同步|P:无|E:移动端双端收发 E2E',
                 tests=['E2E：双端收发、断网重连、换机恢复', '无障碍：读屏朗读消息与状态', '性能：同步分页 P95 记录'], want='在 Android 与 iOS 上收发单聊消息并离线同步', why='移动端聊天纵切可用', reqs='FR-6,FR-7', ads='AD-4,AD-17'))
    P2.append(mk('1.18', '1.9', '聊天与同步客户端：Web 与桌面', cl_chat, '1.14,1.15,1.16', x='W:聊天页与同步|D:聊天页与同步（Electron 壳）|P:无|E:Web/桌面收发 E2E',
                 tests=['E2E：Web 与桌面收发、离线恢复', '无障碍：读屏与键盘', '安全负面：Electron IPC/外链检查'], want='在 Web 与桌面上收发单聊消息并离线同步', why='桌面与网页聊天纵切可用', reqs='FR-6,FR-7', ads='AD-4,AD-17'))
    P2.append(mk('1.19', '1.11', '管理员初始化与 RBAC 骨架（服务端）', old['1.11']['acs'], '1.4,1.12', x='S:admin 模块、策略引擎骨架|P:/v1/manager/*（独立监听）|B:admin_user、role、permission|R:路由必须声明策略否则启动失败|E:管理接口 E2E'))
    P2.append(mk('1.20', '1.11', '管理面最小界面（登录、首页、审计）', ['Given 管理员 / When 登录管理后台 / Then 通过 MFA 后进入首页，显示基础概览', 'Given 审计页 / When 查询 / Then 只读展示审计日志并支持筛选', 'Given 每个后台页面 / When 渲染 / Then 具备加载、空、错误、无权限状态，键盘可达、焦点环可见'], '1.19,1.10',
                 x='M:登录、首页、审计列表（新设计）|P:无|E:后台界面 E2E', tests=['E2E：登录-MFA-审计查看', '无障碍：a11y_scan', '视觉：后台基线'], want='在管理后台登录并查看审计日志', why='运营能看见并核对每一次管理动作', reqs='FR-45,FR-46', ads='AD-17,AD-24', cfg='不沿用上游 Element Plus 外观；使用设计令牌', rb='前端独立部署可回滚'))
    P2.append(mk('1.21', '1.13', old['1.13']['title'], old['1.13']['acs'], '1.2,1.3,1.19'))
    P2.append(mk('1.22', '1.14', old['1.14']['title'], old['1.14']['acs'], '1.3,1.21'))
    P2.append(mk('1.23', '1.16', old['1.16']['title'], old['1.16']['acs'], '1.6'))
    P2.append(mk('1.24', '1.12', old['1.12']['title'], old['1.12']['acs'], '1.2'))
    px = {d['id']: d for d in P2}
    px['1.3']['acs'] = ['Given 空的 MySQL 8.0 库 / When 运行迁移 / Then 创建 schema_migration 与 idempotency_key，所有表为 InnoDB + utf8mb4_0900_ai_ci，TEXT/BLOB/JSON 列无 DEFAULT（静态检查与 information_schema 双重验证）',
        'Given 请求带 Idempotency-Key / When 同一请求在 24 小时内重复提交 / Then 返回首次响应并标记 Idempotent-Replayed，不重复执行；同一键但请求内容不同返回 422，首次仍在处理中返回 409，5xx 不缓存',
        'Given 迁移执行中途失败 / When 再次运行或回滚 / Then 失败的版本不写入 schema_migration，状态被标记为 dirty 并拒绝继续（MySQL DDL 会隐式提交，无法依赖事务回滚）；对应 down 脚本可恢复到上一版本',
        'Given 并发运行迁移 / When 两个进程同时启动 / Then 由数据库命名锁保证只有一个执行；已应用迁移被改动时校验和不符并拒绝运行']
    px['1.3']['tests'] = ['集成：真实 MySQL 8.0 上跑迁移、回滚、dirty 恢复与并发运行', '安全负面：被篡改的已应用迁移（校验和）被拒；同键不同请求体不会重放他人响应；幂等键按租户与路径隔离', '静态：迁移文件必须成对（up/down）且无 TEXT/BLOB DEFAULT；information_schema 验证字符集/引擎']
    px['1.3']['cfg'] = 'VG_MYSQL_DSN；幂等键 TTL 默认 24h；迁移通过 `veyrogram-api migrate up|down|status` 显式执行，服务启动不自动迁移'
    px['1.3']['rb'] = '每个迁移带 down 脚本；失败后 dirty 标记阻止继续，按 down 脚本或从备份恢复；破坏性变更走 expand → migrate → contract'
    px['1.3']['acs'].append('Given 迁移执行中途失败 / When 回滚 / Then 事务回滚且 schema_migration 不记录该版本，可用回滚脚本恢复到上一版本')
    px['1.7']['acs'].append('Given 部署校验 / When 在 CI 与预发运行 / Then 暴露面扫描（外部+内网）为必过门禁，任何可达的内部端口即失败')
    px['1.9']['acs'].append('Given Android 与 iOS / When 在隔离环境运行 / Then 无签名 debug 构建与单元测试可由脚本一键复现，并保存日志')
    px['1.10']['acs'].append('Given Web 与桌面 / When 冻结安装并构建 / Then 构建通过且产物可复现，Electron 安全配置检查通过')
    px['1.10']['acs'].append('Given 后台 Manager / When 使用官方源冻结安装 / Then 构建通过，高危漏洞清零或有受审批的豁免记录')
    D = [d for d in D if not d['id'].startswith('1.')] + P2
    for d in D:
        if not d['id'].startswith('1.'): d['deps'] = ','.join(final_ids([x for x in d['deps'].split(',') if x]))
    # ---------------------------------------------------------------- final: phases for all other stories default to epic phase
    for d in D:
        e = int(d['id'].split('.')[0])
        phase.setdefault(d['id'], EPICS[e]['phase'])
    # ---------------------------------------------------------------- row-level traceability fixes (review §1.2). Keys are the 1-based row numbers in traceability.csv
    row_fix = {3: ['2.2', '11.3'], 33: ['4.2'], 34: ['1.9'], 35: ['3.1'], 36: ['11.3', '11.5'], 37: ['3.4'],
               38: ['4.8', '7.1'], 39: ['7.1'], 46: ['7.1'], 105: ['3.4', '7.1'],
               54: ['1.9', '7.1'], 55: ['13.1'], 56: ['2.6', '7.1'], 57: ['9.4', '7.1'], 106: ['16.7', '9.4', '3.4'],
               61: ['7.2'], 62: ['7.1'], 63: ['4.1', '3.1', '3.3'], 64: ['4.1'], 65: ['3.3', '4.4'],
               79: ['2.7'], 92: ['2.7'], 80: ['8.1', '4.8'], 89: ['8.1', '3.4'], 81: ['13.5'], 82: ['13.5'], 86: ['13.5'],
               83: ['4.1', '4.3'], 90: ['13.6'], 91: ['13.6'], 113: ['3.4'], 125: ['5.1'], 136: ['5.8'], 137: ['5.8'], 138: ['9.4'], 139: ['1.8'],
               143: ['13.6'], 184: ['4.3', '4.8'], 187: ['4.5', '4.8'], 20: ['2.6', '5.6'], 222: ['12.1', '10.2'], 227: ['12.1', '11.2'],
               240: ['12.2', '10.1'], 242: ['12.2', '10.1'], 73: ['4.8'], 74: ['4.8'], 75: ['4.8'], 24: ['2.6']}
    notes = {29: '基础版与付费收藏重复，按 10.1 交付（Phase 4）', 55: '基础版“已读”与旗舰已读未读重复，按 13.1 交付（Phase 4）'}
    row_fix = {k: sorted(set(final_ids(v)), key=lambda z: tuple(int(t) for t in z.split('.'))) for k, v in row_fix.items()}
    return [tuple(d[f] for f in FIELDS) for d in D], phase, row_fix, notes

# Medium/Low items intentionally NOT done in this pass (recorded in docs/product/review-epics-independent.md follow-up):
#  - M-1 split of 9.6, 6.4, 17.2, 19.1, 14.3 and the multi-client L stories into server/client halves
#  - M-4 stories: i18n pipeline, environment tiers, production TURN/SFU deployment, tenant guard, OSM-1 closure story, developer docs site
#  - L-3: R-30 extension-module scope decision, OQ-OP-6 package hosting/signing-key management
#  - L-4: split of 1.2 into migrations/outbox vs audit chain (an AC was added instead); 6.2 Emoji gate samples for Android menu XML / iOS TabBar
