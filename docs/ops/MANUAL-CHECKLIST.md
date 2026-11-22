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
- **网页：** Settings → Branches → Add branch ruleset：目标 `main`；勾选 Require a pull request before merging、Require status checks to pass（添加 `go`、`infra`、`alerts`、`supply-chain`、`design`（Story 1.24 新增 `supply-chain`），以及 design-checks 的任务名）、Block force pushes、Restrict deletions。
- **命令行（有权限时）：**
  ```
  gh api -X PUT repos/wuekevin/VeyroGram/branches/main/protection --input - <<'JSON'
  {"required_status_checks":{"strict":true,"contexts":["go","infra","alerts","supply-chain","design"]},
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
bash tools/build_wukongim.sh /tmp/wukongim                       # 在有 Go 1.26.11 的机器上（会应用补丁）
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

**发送前校验（Story 1.15）上线核对**：渲染配置时 `WK_BEFORE_SEND_URL` 与 `WK_WEBHOOK_URL`（异步 `msg.notify`，用于记录消息引用）都必须是 `http://127.0.0.1:9102/im/webhook/<path-secret>`（`<path-secret>` 与 api.env 的 `VG_IM_WEBHOOK_SECRET` 相同，至少 32 位；IM 会自己追加 `?event=msg.before_send`），模板里 `on_timeout`/`on_error` 保持 `deny`，IM 超时 500 ms 大于网关预算 400 ms。上线后用两个测试账号各发一条消息（应成功），再对不存在的 uid 发一条（应被拒绝，原因码 130），并在监控里确认 `vg_message_gate_total` 在增长；`VeyroGramSendGateFailClosed` 告警出现说明网关在拒绝发送，先查 MySQL。生产压测下 before_send 的 P95（目标 ≤ 50 ms，仍是假设）需要你在真实机器上用压测确认。

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

### C8. 管理面与首个管理员（Story 1.19）
管理面是**独立监听、独立主机名**，默认关闭。上线步骤：
```
# 1) /etc/veyrogram/api.env（0600，root:veyrogram）增加；密钥自己生成，不要发给我：
VG_MANAGER_ENABLED=true
VG_MANAGER_ADDR=127.0.0.1:9103
VG_ADMIN_TOTP_KEY=$(openssl rand -hex 32)        # 管理员 TOTP 密钥的加密密钥；丢失 = 所有管理员需重新绑定，备份到密码管理器
# VG_MANAGER_ALLOW=203.0.113.0/24                  # 强烈建议：只允许办公网/VPN 访问管理面
# 2) 创建首个管理员（只能做一次；口令不经命令行参数，走环境变量或标准输入；必须强口令）：
sudo systemd-run --wait --pipe -p EnvironmentFile=/etc/veyrogram/api.env -p User=veyrogram /opt/veyrogram/api/current/veyrogram-api admin bootstrap -username <你的管理员名> -password-stdin   # 输入口令后回车；Mac 或已走 C9 向导时这一步已经做过
#    命令会打印 TOTP 密钥和 otpauth URI——只显示一次，立刻用手机验证器（Google Authenticator/1Password 等）添加。
# 3) 为管理面单独申请证书和域名（例如 manager.<域名>），用独立站点模板：
SERVER_NAME=manager.<域名> TLS_CERT=... TLS_KEY=... UPSTREAM=127.0.0.1:9103 infrastructure/scripts/render-nginx.sh infrastructure/nginx/veyrogram-manager.conf.template /www/server/panel/vhost/nginx/veyrogram-manager.conf
#    模板里有可选的 allow/deny 行（办公网/VPN），按需取消注释。不要把管理面域名和用户 API 域名放在同一个站点里。
systemctl restart veyrogram-api
```
- **首次登录**：`POST /v1/manager/login` 用口令换一个 5 分钟的一次性令牌，再 `POST /v1/manager/mfa/enroll`，带上验证器当前的 6 位码（证明验证器里确实有第 2 步的密钥）；之后每次登录走 `mfa/verify`。口令单独永远换不到会话。
- **验证**：用错误口令、空口令、`admin`/`Password123!` 这类弱口令运行 bootstrap，应被拒绝；再运行一次 bootstrap（即使删除了任何 `install.lock` 文件）应提示 already initialised；`GET /v1/manager/me` 不带令牌应为 401；审计：`GET /v1/manager/audit/verify` 应返回 `ok: true`。
- **告警**：`VeyroGramAdminProbing`（管理面被拒绝请求激增）；管理员被锁定会写入审计 `admin.locked`。
- **还没有的（诚实说明）**：高危动作目前只需要 owner + 理由并写审计，AD-3 提到的“高危动作需第二人审批”没有做；管理员忘记口令/丢失验证器的恢复流程没有做（目前只能由另一位 owner 禁用后重建，或直接在数据库里处理，所以至少保留两个 owner）；管理面 Web 界面属于 Manager 前端 Story。

### C9. 安装向导、灰度发布与健康探针（Story 1.21）
Mac 上已经用真实二进制、真实 MySQL/Redis、真实 Nginx 演练过全部脚本和命令；下面这些**只能在干净的 Linux 主机上验证**（账号、systemd、journald、logrotate、真实网络）。
```
# 1) 在主机上 checkout 仓库后，以 root 运行安装脚本（四选一：development / test / staging / production；每个环境单独一台或单独一套端口、库、Redis）：
sudo infrastructure/scripts/install.sh production
#    它会：创建 veyrogram 服务账号；建 /etc/veyrogram(0750)、/opt/veyrogram/api、/var/lib/veyrogram、/var/log/veyrogram；
#    在**本机**生成所有随机密钥写入 /etc/veyrogram/api.env（0640 root:veyrogram，不打印、不外传）；安装 systemd 单元、journald 限额、logrotate；
#    已存在的 api.env 永远不会被覆盖。运行结束会列出你必须手填的项（@@FILL:...@@）：数据库连接串、短信账号与密钥。
# 2) 手填那几项（用编辑器，不要写进 shell 历史），再做预检——它会逐项列出阻断项和修复办法：
sudo systemd-run --wait --pipe -p EnvironmentFile=/etc/veyrogram/api.env /opt/veyrogram/api/current/veyrogram-api install preflight -install -fresh -ports 127.0.0.1:5001,127.0.0.1:5100,127.0.0.1:5200
#    （用 systemd-run 读环境文件：连接串里有 & 号，直接 source 会被 shell 当成后台符号。第一次还没有发布版本时，用构建出的二进制路径代替 current/。需 root：预检会核对账号、目录、systemd。）
# 3) 用迁移账号（不是应用账号，见 C3）跑向导：配置检查 → 口令策略 → 预检 → 是否已安装 → 迁移 → 首个管理员：
sudo systemd-run --wait --pipe -p EnvironmentFile=/etc/veyrogram/api.env -p User=veyrogram env 'VG_MYSQL_DSN=vg_migrator:<迁移账号口令>@tcp(127.0.0.1:3306)/veyrogram?parseTime=true' /opt/veyrogram/api/current/veyrogram-api install run -username <管理员名> -password-stdin
#    （env 放在命令前面，覆盖 api.env 里的应用账号连接串；口令从标准输入读，输入后回车。）
#    再跑一次、或删掉任何 install.lock 后再跑，都必须被拒绝（退出码 4）。
# 4) 日志与探针：
sudo systemctl enable --now veyrogram-probe.timer          # 每分钟一次；输出到 node_exporter 的 textfile 目录
journalctl --disk-usage ; ls /etc/logrotate.d/veyrogram ; sudo logrotate -d /etc/logrotate.d/veyrogram
sudo -u veyrogram /opt/veyrogram/api/current/veyrogram-api probe      # 全部 ok 时退出码 0；停掉 worker 后应显示 DOWN worker 且退出码 1
```
- **必须核对的主机设置**（预检会报，这里是原因）：① Redis `maxmemory-policy noeviction`（令牌黑名单在 Redis 里，被淘汰就等于撤销失效）；② Redis `requirepass`；③ MySQL `sql_mode` 含 `STRICT_TRANS_TABLES`、`log_bin=ON` 且 `binlog_format=ROW`（1.22 的按时间点恢复依赖它）；④ 时钟 NTP 同步（管理员验证码依赖准确时间）；⑤ `/var/lib/node_exporter/textfile_collector` 目录存在且 node_exporter 启用了 textfile collector，否则探针告警 `VeyroGramProbeStale` 会一直响。
- **灰度发布演练**（需要第二个实例的端口：`/etc/veyrogram/canary.env` 已由 install.sh 生成，默认 8081/9111/9112）：
  ```
  # Nginx 站点用 render-nginx.sh 渲染（模板已带灰度成员的 include，没有灰度时不起作用）；宝塔的重载命令不是 nginx -s reload 时：
  export VG_NGINX_RELOAD_CMD='/etc/init.d/nginx reload'
  infrastructure/scripts/canary.sh start <binary> <version> 10     # 启动灰度实例并把约 10% 的 /v1 请求导给它
  infrastructure/scripts/canary.sh status ; VG_METRICS_TOKEN=... infrastructure/scripts/canary.sh check
  infrastructure/scripts/canary.sh promote                          # 或 abort；promote 失败会自动回滚
  ```
  灰度期间新旧两个版本**同时**连同一个 MySQL：迁移必须向前兼容（先扩展、下个版本再收缩），否则不要用灰度。IM 长连接不走灰度，只有 `/v1` HTTP 请求分流。
- **待你决定（PRD OQ-7）**：环境如何划分主机——同机多套（模板里 staging 已把端口整体偏移 10000、用独立库和独立 Redis 实例）还是 staging 单独一台。我没有替你定，两种都能用现有模板。
- **没有覆盖的（诚实说明）**：install.sh 在 Mac 上只能用演练模式（不创建账号、不改属主、不动 systemd/journald）；真实 `useradd`、`systemctl daemon-reload`、journald 重启、logrotate 实际轮转以及 Nginx 经 宝塔 的重载只能在这台主机上验证；灰度的流量比例在 Mac 上是用真实 Nginx 加桩进程测的（20% 的配置实测 400 个请求恰好 80 个落到灰度），不是真实流量。

---

### C10. 备份与恢复演练（Story 1.22）
Mac 上已用真实 MySQL 8.0（含 binlog）、第二个真实 mysqld、真实 WuKongIM 进程演练过完整流程（按时间点恢复到事故前一秒、篡改/截断/缺块/错钥一律在写入目标之前被拒、IM 数据目录快照恢复后消息与序号一致）。**下面这些只有你能做**：
1. **在你自己的电脑上生成密钥对**（不要在服务器上做）：`veyrogram-api backup keygen -out-dir ~/vg-backup-keys`。把 `private.pem` 存到**至少两个**安全的地方（密码管理器 + 离线介质）：**丢了它，所有备份都无法读取，没有任何补救办法**。只把 `public.pem` 拷到服务器 `/etc/veyrogram/backup-public.pem`。不要把私钥发给我或放进仓库。
2. **选异地存储并准备“只能追加”的上传凭证**：另一台机器（scp/rsync）或对象存储（rclone）。凭证只能新增对象，不能删除/覆盖（这样主机被攻破也抹不掉历史）。把上传命令写进 `/etc/veyrogram/backup.env` 和 `backup-im.env` 的 `VG_BACKUP_UPLOAD_CMD`（**单引号包住整条命令**，例如 `VG_BACKUP_UPLOAD_CMD='rclone copyto "$1" remote:veyrogram-backups/$(basename "$1")'`）。
3. **创建备份账号并写 my.cnf**：`VG_MYSQL_ADMIN_DSN=... VG_PROVISION_BACKUP_PASSWORD=$(openssl rand -hex 24) veyrogram-api db provision-backup`，把账号口令写进 `/etc/veyrogram/backup-mysql.cnf`（`[client]` 段 user/password/host/port；`chmod 0600`，属主 `veyrogram-backup`）。服务器 MySQL 必须开 binlog 且 `binlog_format=ROW`（预检会检查）。
4. **手动跑一遍再启用定时器**：`sudo -u veyrogram-backup /opt/veyrogram/backup/current/backup.sh mysql-full`，确认异地出现 `mysql-full-*.vgbk`；再跑 `mysql-binlog`；IM 用 `sudo -u veyrogram-im ... backup.sh im`（**会让 IM 停几秒**；先确认 polkit 规则生效：该账号能执行 `systemctl stop/start veyrogram-wukongim`，但不能操作别的单元）。然后 `systemctl enable --now veyrogram-backup-mysql-full.timer veyrogram-backup-mysql-binlog.timer veyrogram-backup-prune.timer veyrogram-backup-im.timer`，并确认 node_exporter 的 textfile 目录里出现 `veyrogram_backup*.prom`，告警 `VeyroGramBackupMetricsMissing` 随之消失。
5. **在 staging（另一台主机或另一个 MySQL 实例）做一次真实恢复演练**，并把耗时记下来（这就是 RTO 的依据）：
   ```
   restore.sh verify  -d <从异地拉回的目录> -k private.pem                       # 先认证所有文件
   restore.sh mysql   -d <目录> -k private.pem -t staging.cnf -u '2026-11-21 12:00:00'   # 时间为 UTC
   restore.sh im      -f <im-*.vgbk> -k private.pem -o /var/lib/veyrogram-im-restore
   ```
   核对：关键表行数、最新消息、`audit_log` 链用 `veyrogram-api audit verify` 通过。恢复到“事故前一刻”时，确认事故本身没有被重放。
6. **IM 的数据丢失窗口 = 快照间隔（默认每天一次）**。如果这个窗口不可接受，决定：缩短间隔（每次会停 IM 几秒），或用 LVM/ZFS/btrfs 卷快照（`VG_IM_QUIESCE=none`，这条路径我没有演练过，上线前必须自己演练），或等多节点复制（v1 单节点不支持）。**这是产品决定，不是我能替你选的。**
7. **数据库与 IM 恢复到不同时刻之后的对账脚本还没有写**（以 IM 序号为准重建 `message_ref`）。跨存储恢复暂时要人工核对：`message_ref` 里指向的消息在 IM 里是否存在。我会在后续 Story 里补这个脚本；在此之前，演练时请把“两边恢复点不一致”这一步写进演练记录。
8. **对象存储**（选定厂商后）：`VG_S3_ENDPOINT=... infrastructure/backup/object-storage-setup.sh backup <桶名>`，以及 `media <桶名>`；它会读回并校验版本化与生命周期是否真的生效。保留天数（备份 35 天、旧版本 7 天、媒体旧版本 30 天）只是默认值，先改 `object-storage/*.json`（PRD OQ-7）。仅用假 provider 测过，真实 provider 的行为必须在这里验证。
9. **季度演练**：把第 5 步排进日历；演练失败或缺失会阻断发布就绪（AD-22）。

---

### C11. 确认“没有需要迁移的上游部署”（Story 1.23，需要你确认一个事实）
我按 ADR-0011 做了决定：**不提供 compat 层，不提供 TangSengDaoDao 数据迁移，只支持全新部署**。依据是 VeyroGram 尚未发布、没有线上部署要保护，以及上游口令是无盐 `MD5(MD5())`、手机号明文、消息历史在 WuKongIM 里，导入成本和风险都很高。
- **你要做的：** 确认下面三条都**不成立**；任何一条成立，请告诉我，我会重新打开这个决策（ADR 里写了最小可行方案）：
  1. 你有一套正在运行的 TangSengDaoDao 现网，要把用户搬到 VeyroGram；
  2. 有合作方必须继续用上游旧客户端；
  3. 合规要求保留旧系统的历史消息。
- **不需要你做的：** 没有迁移脚本，也就没有“迁移回滚”可演练；全新部署走安装向导（1.21）。


### C12. 客户端包的发布（Story 1.11，需要你的注册表账号）
生成的客户端在 `packages/clients/`，随契约版本放在仓库里。要让各端工程按版本依赖它们，需要发布到包仓库；我没有账号，也不会替你注册：
- **TypeScript**：npm（建议私有 scope，如 `@veyrogram/client`）。需要 npm 账号和 2FA；发布命令 `npm publish --access restricted`。
- **Kotlin**：Maven（私有仓库或 GitHub Packages）；Kotlin 客户端到 Story 1.9 的 Android 工程里才第一次被编译，发布前先过那一步。
- **Swift**：SwiftPM 直接引用 Git 标签即可，不需要中央注册表。
- 在你决定之前，各端工程可以直接用相对路径引用仓库里的 `packages/clients/*`。
### C13. 桌面端打包、签名与更新服务器（Story 1.10，需要你的证书与服务器）
`tools/build_clients.sh web` 只产出 Web 构建和 Electron 的编译产物，不打安装包。要发布桌面端，需要：
- **应用 ID/名称**：`app.veyrogram.desktop` 是临时值，待 OQ-1（品牌核验）定稿后改 `apps/web/patches/0002` 里的 `electron-builder*.js`。
- **代码签名**：macOS 需要 Apple Developer ID 证书 + 公证（`notarytool`）；Windows 需要代码签名证书（OV/EV）。证书和密码放在你的构建机上，不要发给我也不要提交仓库。
- **更新服务器**：自动更新现在是**关闭**的（没有厂商默认值）。要开启：准备一个 HTTPS 静态站点放 `electron-builder` 产出的 `latest*.yml` 与安装包，构建时设置环境变量 `VEYROGRAM_UPDATE_URL=https://你的更新域名/`；签名更新要求 Windows 证书发布者名与 `publisherName` 一致。
- 做完后告诉我更新域名，我再补更新通道的端到端测试。
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
| 多节点 IM 集群、踢线传播 | 3 台机器（仅当你决定做集群） | 已在本机用真实三节点 WuKongIM 集群验证：令牌是全集群有效的，但 `device_quit` 只踢发到的那个节点上的连接；因此网关支持把踢线发给每个节点（`VG_IM_API_PEER_URLS`，逗号分隔其他节点的 API 地址），任一节点不可达则失败并由 worker 重试。v1 仍以单节点为目标；做集群前需要单独立项并在真实多机网络上重做这一项验证（本机回环不代表跨机延迟与分区） |
| 真实短信：短信宝（适配器已完成，缺账号与报备） | 短信宝账号、ApiKey、已报备的签名/模板 | ① 注册短信宝并实名认证（国内短信必须），在后台“签名/模板报备”里报备签名 `【VeyroGram】`（需与我们短信开头的签名一致，否则返回 50）；国际短信要求目标国家开通。② 后台取 ApiKey（或登录密码的 MD5），在“IP 限制”里加上 API 服务器的出口 IP（否则返回 43）。③ 在 `/etc/veyrogram/api.env`（0640）设置：`VG_AUTH_SMS_LOGIN=true`、`VG_SMS_PROVIDER=smsbao`、`VG_SMSBAO_USER=<账号>`、`VG_SMSBAO_KEY=<ApiKey>`、`VG_TOKEN_KEYS=k1:<密钥>`、`VG_PHONE_HASH_KEY=<密钥>`、`VG_IM_TOKEN_KEY=<另一个密钥，不能与手机号哈希密钥相同>`、`VG_REDIS_ADDR=127.0.0.1:6379`（Redis 7.x，设密码并写入 `VG_REDIS_PASSWORD`；生产必填，用于设备撤销的访问令牌拒绝列表）、`VG_TRUSTED_PROXIES=<Nginx 地址>`。令牌密钥：`openssl rand 32 \| base64 \| tr '+/' '-_' \| tr -d =`；手机号哈希密钥：`openssl rand -hex 32`。④ **`VG_PHONE_HASH_KEY` 上线后不能改**（改了所有账号都找不到），必须备份。⑤ 用一个真手机号走一遍：收码→登录→刷新；再用一个 +852/+60 之类的国际号码验证国际通道；再故意输错 5 次，确认被锁定。⑥ 我只依据短信宝官方文档（国内 `GET /sms`、国际 `GET /wsms`，返回 `0` 为成功）并用本地模拟服务器测过，**没有用真实账号验证**，第一次真发请先小量测试；账号余额不足（41）、IP 未放行（43）会表现为“发送验证码”返回 503。 |
| 推送（APNs/FCM） | Apple/Google 开发者账号（OQ-5） | 创建推送证书/密钥，放服务器密文目录 |
| 对象存储 | S3 兼容服务（OQ-9） | 建桶、开版本控制，给我访问域名与权限策略（密钥自己放服务器） |
| iOS/Android 真机与商店构建 | Xcode 签名证书、Android SDK/签名 keystore | Story 1.9 开始时，我会列出具体的安装/签名步骤 |

---

## G. 我之后仍会在 Mac 上继续做的（不需要你操作）
Story 1.12（手机号注册登录服务端）、1.13（设备/令牌/撤销）、1.15（单聊与 before_send 骨架）、1.16（离线同步）、1.19（管理员与 RBAC）、1.24（测试与安全门禁）。客户端界面类 Story（1.9–1.11、1.14、1.17、1.18、1.20）受 A1 设计门禁与本机 Android SDK 影响，会先做能在本机验证的部分。
