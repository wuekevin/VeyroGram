# 需要人工完成的事项清单（Mac 上无法完成的部分）

> 本清单只列“我在这台 Mac 上做不了、或必须由你本人决定/签字/付费/拥有权限”的事。每条都写明：为什么我做不了、你怎么做、做完怎么验证、验证失败怎么办。
> 已在 Mac 上完成并有测试证据的内容不在此列（见各 Story 的 Dev Agent Record）。
> 约定：`<…>` 表示你需要替换的值；所有命令都假设在仓库根目录 `VeyroGram/` 执行，除非另有说明。

---

## A. 需要你本人决定或签字（我不能代替）

### A1. 设计门禁 G1–G6 签字（阻塞 Phase 3 的批量客户端编码）
- **为什么我不能做：** 这是产品负责人的验收，AI 自己批准自己的设计没有意义（`docs/design/APPROVALS.md`）。
- **怎么做（二选一）：**
  1. 直接编辑 `docs/design/APPROVALS.md`，把 G1–G6 每行的“决定”改成 `APPROVED`（或 `REJECTED` / `WAIVED`），填“签字人 / 日期 / 备注”，保存并提交；
  2. 在对话里写“批准 G1、G2、G4；驳回 G3，原因……”，我会原样记录并注明“据用户指示记录”。
- **评审顺序与材料：** G1 `brand/svg/logo-mark.svg` 与 `brand/icons/` → G3 用浏览器打开 `docs/design/prototypes/*.html`（加 `?theme=dark` 看暗色）→ G2 `docs/design/COMPONENT-SPECS.md` → G4 `docs/design/baselines/` → G5 `docs/design/WCAG-2.1-AA-CHECKLIST.md`，并按 `docs/design/ACCESSIBILITY-REVIEW-GUIDE.md` 用 VoiceOver（Mac：`Cmd+F5`）、键盘 Tab 和 200% 缩放亲自试 2–3 个页面 → G6 `docs/design/DESIGN.md` 的 Typography 一节。
- **验证：** `python3 tools/check_gates.py`，退出码 0 即通过。

### A2. 业务/法务决策（PRD 的开放问题）
| 编号 | 问题 | 不决定的影响 | 我暂时的做法 |
|---|---|---|---|
| OQ-1 | 品牌/包名/域名/商标是否冲突 | 应用 ID、URL Scheme、签名、上架 | 开发期用临时包名，发布前必须替换 |
| OQ-7 | 首发 DAU/连接数；RPO/RTO 目标 | 压测目标、备份与灾备预算 | 先用保守默认值 |
| OQ-8 | 短信服务商与覆盖地区 | 注册登录上线 | 只做 `SmsProvider` 端口 + 沙箱适配器，预发前必须选定 |
| OQ-4 | 运营地区/法规辖区 | 实名、数据驻留、未成年人、隐私政策 | Epic 2 的保留期等先参数化 |
| OQ-3/5/6/9/12 | 支付、Apple/Google 账号主体、音视频、对象存储、开发者结算 | 后续 Epic | 到对应阶段前必须回复 |
- **怎么做：** 在对话里直接回答（每个一句话即可，例如“OQ-8：先用阿里云短信，仅中国大陆”）。我会写入 `docs/product/` 并据此调整 Story。

### A3. 法务确认：再分发打补丁的 WuKongIM
- 上游许可证见 `upstream/WuKongIM/LICENSE`。我们发布的是“固定 SHA + 补丁”的构建产物，需要你（或法务）确认许可证署名与再分发义务。我没有做法律判断。

---

## B. GitHub 侧设置（需要你的仓库管理权限）

仓库：`https://github.com/wuekevin/VeyroGram`（私有）。

### B1. main 分支保护（让 CI 变成合并门禁）
- **注意：** 私有仓库的分支保护/Rulesets 需要 GitHub Pro/Team 套餐；免费版会返回 403 `Upgrade to GitHub Pro`。
- **网页：** Settings → Branches → Add branch ruleset：目标 `main`；勾选 Require a pull request before merging、Require status checks to pass（添加 `go`、`infra`、`alerts`、`design`，以及 design-checks 的任务名）、Block force pushes、Restrict deletions。
- **命令行（有权限时）：**
  ```
  gh api -X PUT repos/wuekevin/VeyroGram/branches/main/protection --input - <<'JSON'
  {"required_status_checks":{"strict":true,"contexts":["go","infra","alerts","design"]},
   "enforce_admins":true,"required_pull_request_reviews":{"required_approving_review_count":1},
   "restrictions":null,"allow_force_pushes":false,"allow_deletions":false}
  JSON
  ```
- **验证：** 直接 `git push origin main` 应被拒绝；开 PR 后 CI 未绿时 Merge 按钮不可用。
- **失败处理：** 若只有你一个人，把 `required_approving_review_count` 设为 0，保留状态检查即可。

### B2. 打开安全功能
Settings → Code security：启用 Dependabot alerts / security updates（已在工作）、Secret scanning、Push protection。
- **验证：** `gh api repos/wuekevin/VeyroGram/dependabot/alerts --jq '.[]|select(.state=="open")|.number'` 应为空（上一条 edwards25519 提示已升级修复，等 GitHub 重新扫描后自动关闭）。

### B3. （可选）建立 WuKongIM 托管 fork
- 当前不需要：构建靠 `patches/wukongim/*.patch`。如果你想要一个可对照上游的 fork：
  ```
  gh repo fork WuKongIM/WuKongIM --clone=false --fork-name WuKongIM
  git -C upstream/WuKongIM remote add vgfork https://github.com/wuekevin/WuKongIM.git
  git -C upstream/WuKongIM checkout -b veyrogram/patches 7b810e08 && git -C upstream/WuKongIM am ../../patches/wukongim/*.patch
  git -C upstream/WuKongIM push vgfork veyrogram/patches
  git -C upstream/WuKongIM checkout 7b810e08     # 之后务必回到固定 SHA，构建脚本会检查
  ```
- 注意：fork 默认公开；若要私有需在 fork 的设置里改（公开仓库的 fork 无法直接改私有，需要“复制”而不是 fork）。

---

## C. 必须在真实 Linux 主机上做的演练（Mac 没有 systemd/防火墙/真实网络）

目标主机建议：Ubuntu 22.04/24.04 或 Debian 12，已装宝塔（若你用）。下面的命令以 root 执行。

### C1. 基础软件
```
apt-get update && apt-get install -y nginx python3 curl ca-certificates
# MySQL 8.0（不是 8.4）：用官方 APT 源固定到 8.0；Redis 7.x 同理
mysql --version      # 必须显示 8.0.x
redis-server --version   # 必须显示 7.x
```
（用宝塔装也行，但安装后必须核对上面两条版本输出。）

### C2. 账号、目录、环境文件
```
useradd --system --home-dir /nonexistent --shell /usr/sbin/nologin veyrogram
useradd --system --home-dir /nonexistent --shell /usr/sbin/nologin veyrogram-im
install -d -o root -g veyrogram -m 0750 /etc/veyrogram
install -d -o veyrogram -g veyrogram -m 0750 /opt/veyrogram/api
install -d -o veyrogram-im -g veyrogram-im -m 0750 /opt/veyrogram/wukongim /var/lib/veyrogram-im
install -m 0640 -o root -g veyrogram /dev/null /etc/veyrogram/api.env
```
`/etc/veyrogram/api.env` 的变量清单见 `services/api/README.md`；密钥用 `openssl rand -hex 32` 生成，**不要**写进仓库或 shell 历史（用编辑器写入）。生产必须 `VG_ENV=production`，程序会拒绝默认/弱密钥启动。

### C3. 数据库账号（三账号模型）
在本机（Mac）或服务器上构建二进制后：
```
make -C services/api linux            # 产物 services/api/bin/veyrogram-api-linux-amd64
export VG_MYSQL_ADMIN_DSN='root:<管理员口令>@tcp(127.0.0.1:3306)/'
export VG_PROVISION_APP_PASSWORD="$(openssl rand -hex 24)"   VG_PROVISION_MIGRATOR_PASSWORD="$(openssl rand -hex 24)"
./veyrogram-api db provision --database veyrogram --app-user vg_app --migrator-user vg_migrator --host 127.0.0.1
VG_ENV=production VG_MYSQL_DSN="vg_migrator:${VG_PROVISION_MIGRATOR_PASSWORD}@tcp(127.0.0.1:3306)/veyrogram" ./veyrogram-api migrate up
./veyrogram-api db grant-app --database veyrogram --user vg_app --host 127.0.0.1
```
把 `VG_PROVISION_APP_PASSWORD` 写进 `/etc/veyrogram/api.env` 的 `VG_MYSQL_DSN`（账号 `vg_app`），然后清掉这两个环境变量。
- **验证：** 用 `vg_app` 登录后执行 `DELETE FROM audit_log;` 必须报权限错误（1142）；`CREATE TABLE t(id int);` 也必须报错。
- **注意：** 每次新增迁移后都要重跑 `grant-app`，否则 app 账号访问不到新表（这是故意的安全默认）。

### C4. 发布、回滚演练、systemd 加固评分
```
sha256sum services/api/bin/veyrogram-api-linux-amd64 > services/api/bin/veyrogram-api-linux-amd64.sha256
infrastructure/scripts/deploy.sh services/api/bin/veyrogram-api-linux-amd64 v0.0.1     # 校验和→预检→切换→启动→等就绪→失败自动回滚
cp infrastructure/systemd/veyrogram-*.service infrastructure/systemd/veyrogram-*.timer /etc/systemd/system/ && systemctl daemon-reload
systemctl enable --now veyrogram-api veyrogram-worker
systemd-analyze security veyrogram-api.service veyrogram-worker.service veyrogram-audit-anchor.service
# 每日清理“已清空草稿”的墓碑（Story 1.16，保留 30 天；离线超过 30 天的设备下次全量同步以服务器为准，不会复活）
cp infrastructure/systemd/veyrogram-sync-purge.service infrastructure/systemd/veyrogram-sync-purge.timer /etc/systemd/system/ && systemctl daemon-reload && systemctl enable --now veyrogram-sync-purge.timer
```
- **验证：** `systemd-analyze security` 的总分建议 ≤ 3.0（越低越好）；`curl -s http://127.0.0.1:9101/readyz` 返回 200。
- **回滚演练：** 先部署 v0.0.2 的一个故意失败版本（例如把 `VG_MYSQL_DSN` 改错），确认脚本自动回到 v0.0.1；再执行 `infrastructure/scripts/rollback.sh` 手动回滚一次。
- **失败处理：** 把 `journalctl -u veyrogram-api -n 100` 的输出发给我。

### C5. WuKongIM 上线与硬化
```
bash tools/build_wukongim.sh /tmp/wukongim                       # 在有 Go 1.26.10 的机器上（会应用补丁）
install -d /opt/veyrogram/wukongim/releases/v1 /opt/veyrogram/wukongim/releases/v1/tools
install -m 0755 /tmp/wukongim /opt/veyrogram/wukongim/releases/v1/wukongim
install -m 0644 tools/wukongim_config_check.py /opt/veyrogram/wukongim/releases/v1/tools/
ln -sfn /opt/veyrogram/wukongim/releases/v1 /opt/veyrogram/wukongim/current
# 渲染配置：变量清单与说明见 infrastructure/wukongim/README.md；5 个密钥用 openssl rand -hex 32，口令至少 16 位
python3 -I tools/render_wukongim.py infrastructure/wukongim/wukongim.prod.toml.template /etc/veyrogram/wukongim.toml
chown root:veyrogram-im /etc/veyrogram/wukongim.toml && chmod 0640 /etc/veyrogram/wukongim.toml
python3 -I tools/wukongim_config_check.py /etc/veyrogram/wukongim.toml      # 必须输出 ok
systemctl enable --now veyrogram-wukongim
```
`WK_WEBHOOK_SIGNING_SECRET` 必须与 `/etc/veyrogram/api.env` 里的 `VG_IM_WEBHOOK_SIGNING_SECRET` 完全相同。

**发送前校验（Story 1.15）上线核对**：渲染配置时 `WK_BEFORE_SEND_URL` 必须是 `http://127.0.0.1:9102/im/webhook/<path-secret>`（`<path-secret>` 与 api.env 的 `VG_IM_WEBHOOK_SECRET` 相同，至少 32 位；IM 会自己追加 `?event=msg.before_send`），模板里 `on_timeout`/`on_error` 保持 `deny`，IM 超时 500 ms 大于网关预算 400 ms。上线后用两个测试账号各发一条消息（应成功），再对不存在的 uid 发一条（应被拒绝，原因码 130），并在监控里确认 `vg_message_gate_total` 在增长；`VeyroGramSendGateFailClosed` 告警出现说明网关在拒绝发送，先查 MySQL。生产压测下 before_send 的 P95（目标 ≤ 50 ms，仍是假设）需要你在真实机器上用压测确认。

### C6. 防火墙（默认拒绝入站）与暴露面扫描（Story 1.7 的预发门禁）
```
ufw default deny incoming && ufw default allow outgoing
ufw allow 22/tcp            # 先放行你的 SSH，再启用，否则会把自己锁在外面
ufw allow 443/tcp           # Nginx
ufw allow 5100/tcp          # IM 网关 TCP（如要对外；WS 走 Nginx 则不需要 5200）
ufw enable && ufw status verbose
```
宝塔/云厂商安全组里也要做同样的入站规则（只开 22/443/5100）；MySQL 3306、Redis 6379、5001、5301、7001、9099、9101、9102、1883 一律不开。
- **验证（两个视角都必须跑，缺一不可）：**
  1. 从你家里/另一台公网机器：`python3 -I tools/exposure_scan.py --target <服务器公网IP>`
  2. 从同一内网网段的另一台非白名单机器：`python3 -I tools/exposure_scan.py --target <服务器内网IP>`
  两次都必须输出 `all unreachable` 且退出码 0。任何 `EXPOSED:` 行都是事故级问题，先修再继续。
- **失败处理：** 看 `ss -ltnp` 确认该端口监听在哪个地址；API/Manager/集群/MQTT 必须是 `127.0.0.1`。

### C7. Nginx 站点
```
SERVER_NAME=<域名> TLS_CERT=<证书路径> TLS_KEY=<私钥路径> UPSTREAM=127.0.0.1:8080 infrastructure/scripts/render-nginx.sh infrastructure/nginx/veyrogram-api.conf.template /www/server/panel/vhost/nginx/<域名>.conf
nginx -t && systemctl reload nginx
```
- **验证：** `curl -s -o /dev/null -w '%{http_code}' https://<域名>/healthz` 必须是 404；`/metrics` 也是 404；`/v1/` 才转发。

---

## D. 审计锚点的异地存储（防“控制了数据库的人改历史”）

为什么必须你来：它必须部署在**主机和数据库管理员都无法改动**的另一台机器/另一个云账号上，这是安全属性本身，我不能在同一台机器上“模拟”。
实现已完成：`anchor-store`（只写一次的 HTTP 存储，改写返回 409，无删除接口）、`audit anchor`（每日写入）、`audit check-anchors`（逐日比对）、systemd timer。

在**另一台**小机器（1 核 512MB 足够）上：
```
make -C services/api anchor-store-linux         # 产物 services/api/bin/anchor-store-linux-amd64
useradd --system --home-dir /nonexistent --shell /usr/sbin/nologin veyrogram-anchor
install -d -o veyrogram-anchor -g veyrogram-anchor -m 0750 /opt/veyrogram/anchor-store/releases/v1
install -m 0755 services/api/bin/anchor-store-linux-amd64 /opt/veyrogram/anchor-store/releases/v1/anchor-store
ln -sfn /opt/veyrogram/anchor-store/releases/v1 /opt/veyrogram/anchor-store/current
install -m 0640 -o root -g veyrogram-anchor /dev/null /etc/veyrogram/anchor-store.env
# 写入（两个令牌各 openssl rand -hex 32，且必须不同）:
#   ANCHOR_STORE_DIR=/var/lib/veyrogram-anchor
#   ANCHOR_STORE_ADDR=127.0.0.1:9200
#   ANCHOR_STORE_WRITE_TOKEN=<写令牌>
#   ANCHOR_STORE_READ_TOKEN=<读令牌>
cp infrastructure/systemd/veyrogram-anchor-store.service /etc/systemd/system/ && systemctl daemon-reload && systemctl enable --now veyrogram-anchor-store
# 前面放 Nginx + HTTPS（域名如 anchors.<你的域名>），只转发 /audit-anchor/ 前缀到 127.0.0.1:9200
chattr +a /var/lib/veyrogram-anchor 2>/dev/null || true    # 目录只追加（可选的第二层）
```
在**主服务器**的 `/etc/veyrogram/api.env` 增加：
```
VG_AUDIT_ANCHOR_URL=https://anchors.<域名>
VG_AUDIT_ANCHOR_TOKEN=<写令牌>
```
读令牌**不要**放在主服务器上（否则控制主机的人能读不能写，但仍应分离）；比对任务如果也在主服务器跑，需要读令牌——更安全的做法是在你自己的电脑或第三台机器上定期运行 `veyrogram-api audit check-anchors`（用只读数据库账号 DSN + 读令牌）。我这版 timer 默认把两个任务都放主机上，请按你的安全要求取舍。
- **启用定时任务：** `systemctl enable --now veyrogram-audit-anchor.timer veyrogram-audit-check-anchors.timer`
- **验证：** `systemctl start veyrogram-audit-anchor.service` 后，在锚点机上 `ls -l /var/lib/veyrogram-anchor/` 应出现当天 `YYYY-MM-DD.json`（权限 0400/0444）；再手动运行 `veyrogram-api audit check-anchors -days 7`，应显示 `anchors ok checked>=1`。
- **告警：** 给 `veyrogram-audit-*.service` 的 failed 状态配置通知（宝塔监控或 `OnFailure=` 钩子）；任何一次失败都要当作潜在篡改调查。
- **已知限制：** 只有“每天 03:17 之后”的变化被覆盖；锚点之间的窗口内，攻击者仍可在 24 小时内改写且无法被发现——要缩短就把 timer 改成每小时（同一天内容不同会被存储拒绝，需要把键名改成带小时；这是后续改进）。

---

## E. 监控与可观测性落地（需要你的监控环境）

### E1. 加载告警规则并校准阈值
- 规则文件：`infrastructure/prometheus/veyrogram-alerts.yml`（已通过 `promtool check/test`）。
- 在你的 Prometheus 配置里增加：
  ```
  rule_files: ["/etc/prometheus/rules/veyrogram-alerts.yml"]
  scrape_configs:
    - job_name: veyrogram-api
      authorization: { credentials_file: /etc/prometheus/veyrogram-metrics.token }
      static_configs: [{ targets: ["127.0.0.1:9101"] }]
    - job_name: veyrogram-worker
      authorization: { credentials_file: /etc/prometheus/veyrogram-metrics.token }
      static_configs: [{ targets: ["127.0.0.1:9103"] }]
  ```
  `veyrogram-metrics.token` 内容 = `VG_METRICS_TOKEN`。重载：`promtool check config /etc/prometheus/prometheus.yml && systemctl reload prometheus`。
- **验证：** Prometheus 页面 Status → Targets 全部 UP；Alerts 页能看到 11 条规则。
- **阈值是未校准的：** 上线后用真实流量跑一周，再按 P95/错误率实际值调整 `veyrogram-alerts.yml`，并补相应单测。Alertmanager 的接收渠道（邮件/企业微信/钉钉）需你配置。

### E2. 链路追踪后端（可选）
当前只做了 `traceparent` 传播，没有 span 导出。若要看调用链，需要选择后端（Jaeger/Tempo）并在后续 Story 接入 OTel SDK；这是产品选择，请告诉我是否需要。

---

## F. 需要真实环境/账号才能验证的功能性事项

| 事项 | 需要什么 | 你来做的动作 |
|---|---|---|
| Redis 7 | 已在本机用源码编译 7.4.2 并通过 1.13 的集成测试，CI 也用 redis:7.4 | 生产 Linux 机上请按 C1 装 7.x 并核对版本，设置密码、只监听回环/内网，`VG_REDIS_ADDR`/`VG_REDIS_PASSWORD` 写入 api.env；建议开启 AOF 或接受“重启后拒绝列表清空，吊销退化为令牌 TTL”这一取舍 |
| 多节点 IM 集群、踢线传播 | 3 台机器 | v1 目标单节点；做集群前需要单独立项 |
| 真实短信：短信宝（适配器已完成，缺账号与报备） | 短信宝账号、ApiKey、已报备的签名/模板 | ① 注册短信宝并实名认证（国内短信必须），在后台“签名/模板报备”里报备签名 `【VeyroGram】`（需与我们短信开头的签名一致，否则返回 50）；国际短信要求目标国家开通。② 后台取 ApiKey（或登录密码的 MD5），在“IP 限制”里加上 API 服务器的出口 IP（否则返回 43）。③ 在 `/etc/veyrogram/api.env`（0640）设置：`VG_AUTH_SMS_LOGIN=true`、`VG_SMS_PROVIDER=smsbao`、`VG_SMSBAO_USER=<账号>`、`VG_SMSBAO_KEY=<ApiKey>`、`VG_TOKEN_KEYS=k1:<密钥>`、`VG_PHONE_HASH_KEY=<密钥>`、`VG_IM_TOKEN_KEY=<另一个密钥，不能与手机号哈希密钥相同>`、`VG_REDIS_ADDR=127.0.0.1:6379`（Redis 7.x，设密码并写入 `VG_REDIS_PASSWORD`；生产必填，用于设备撤销的访问令牌拒绝列表）、`VG_TRUSTED_PROXIES=<Nginx 地址>`。令牌密钥：`openssl rand 32 \| base64 \| tr '+/' '-_' \| tr -d =`；手机号哈希密钥：`openssl rand -hex 32`。④ **`VG_PHONE_HASH_KEY` 上线后不能改**（改了所有账号都找不到），必须备份。⑤ 用一个真手机号走一遍：收码→登录→刷新；再用一个 +852/+60 之类的国际号码验证国际通道；再故意输错 5 次，确认被锁定。⑥ 我只依据短信宝官方文档（国内 `GET /sms`、国际 `GET /wsms`，返回 `0` 为成功）并用本地模拟服务器测过，**没有用真实账号验证**，第一次真发请先小量测试；账号余额不足（41）、IP 未放行（43）会表现为“发送验证码”返回 503。 |
| 推送（APNs/FCM） | Apple/Google 开发者账号（OQ-5） | 创建推送证书/密钥，放服务器密文目录 |
| 对象存储 | S3 兼容服务（OQ-9） | 建桶、开版本控制，给我访问域名与权限策略（密钥自己放服务器） |
| iOS/Android 真机与商店构建 | Xcode 签名证书、Android SDK/签名 keystore | Story 1.9 开始时，我会列出具体的安装/签名步骤 |

---

## G. 我之后仍会在 Mac 上继续做的（不需要你操作）
Story 1.12（手机号注册登录服务端）、1.13（设备/令牌/撤销）、1.15（单聊与 before_send 骨架）、1.16（离线同步）、1.19（管理员与 RBAC）、1.21–1.24（安装向导、备份实现、迁移决策、测试与安全门禁）。客户端界面类 Story（1.9–1.11、1.14、1.17、1.18、1.20）受 A1 设计门禁与本机 Android SDK 影响，会先做能在本机验证的部分。
