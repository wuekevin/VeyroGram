# 上游文档审计：dev/backend 与 api（TangSengDaoDaoDocs @ 41e11cb）

- 范围：`upstream/TangSengDaoDaoDocs/src/dev/backend`（21 个 .md）与 `src/api`（7 个 .md），共 28 页，均已逐页完整读取（只读，未修改 upstream/）。
- 约定：下文"页面所述"= 页面原文；"推断"= 审计者推断，非页面明说。所有页面状态均为 **READ_LOCAL**。
- 路径均相对于 `upstream/TangSengDaoDaoDocs/src/`。

---

# 一、dev/backend（21 页）

## 1. dev/backend/architecture.md
- 状态：READ_LOCAL
- 标题：项目架构
- 摘要：系统分两层：通讯层 WuKongIM（长连接、收发消息、存储）与业务层 TangSengDaoDao（好友、群组、朋友圈等）。WuKongIM 通过 Webhook（页面写 GRPC）把聊天数据推给业务层；业务层发系统消息时调用 WuKongIM 发消息 API。客户端用 SDK 连 WuKongIM 长连接收发消息，业务逻辑调业务层 API。
- 具体事实：技术栈 golang、mysql、redis；设计原则 简洁化/可扩展性/高可用；WuKongIM 也可作"消息中台"（图片 architecture2.png，本地未解析图片内容）。
- WuKongIM 集成点：Webhook(GRPC) 方向 WuKongIM→业务层；HTTP 发消息 API 方向 业务层→WuKongIM；客户端 SDK 直连 WuKongIM 长连接。
- 安全观察：无。
- 矛盾/缺口：写"Webhook 机制（GRPC）"，但 api/webhook.md 描述的是 HTTP POST + `?event=` 查询参数（见该页）；未提及 MinIO（文件服务）作为组件；未给端口。

## 2. dev/backend/structure.md
- 状态：READ_LOCAL
- 标题：项目结构
- 摘要：给出 TangSengDaoDaoServer 仓库目录树。
- 具体事实：`configs/tsdd.yaml` 为配置文件；`configs/push` 放苹果推送证书；`assets/sql` 为所有 SQL 脚本；`internal/api/` 下模块：base、channel、common、file、group、message、qrcode、report、robot、source、statistics、user、webhook；`internal/config`、`internal/server/server.go`、`internal/testutil`、`pkg`、`testenv/docker-compose.yaml`、`main.go`；`docker/` 脚本目录。
- 安全观察：苹果推送证书与密码按目录/配置放置（`configs/push/push.p12`，见 fullconfig），属需保护文件（推断）。
- 矛盾/缺口：目录树未列出 `modules/` 目录与 `internal/modules.go`，而 moduleimport.md 依赖它们；`internal/api` 未列出 moments（朋友圈）模块，与 architecture.md 提到朋友圈、moduleimport.md 的 `modules/moments` 的关系未解释（推断：moments 为可选独立模块）。

## 3. dev/backend/api.md
- 状态：READ_LOCAL
- 标题：（无标题，仅"待补充"）
- 摘要：占位页，全文仅"待补充"。
- 具体事实：无。
- 安全观察：无。
- 缺口：业务层（8090 `/v1`）的 API 文档缺失；api/ 目录文档只覆盖 WuKongIM 服务端 API（5001），未覆盖 TangSengDaoDao 自身 API。

## 4. dev/backend/fullconfig.md
- 状态：READ_LOCAL
- 标题：完整配置
- 摘要：列出 `tsdd.yaml` 全部配置项（yaml），说明默认可全注释使用系统默认值；并给出"配置转环境变量"规则（`TS_` 前缀 + 层级用 `_` + 全大写）。
- 具体事实（配置键与默认/示例值）：
  - 基础：`mode: "debug"`（默认示例为 debug）、`addr: ":8090"`、`grpcAddr: "0.0.0.0:6979"`（给悟空 IM 的 webhook gRPC 监听）、`appName`、`rootDir`、`messageSaveAcrossDevice: true`、`welcomeMessage`、`phoneSearchOff: true`、`onlineStatusOn: true`、`groupUpgradeWhenMemberCount: 1000`、`eventPoolSize: 100`。
  - `wukongIM.apiURL`（格式 `http://xx.xx.xx.xx:5001`）、`wukongIM.managerToken`（"悟空IM配置了就需要填写，没配置就不需要"）。
  - `db.mysqlAddr` 示例 `root:demo@tcp(127.0.0.1:3306)/test?charset=utf8mb4&parseTime=true`；`db.redisAddr`；`db.asynctaskRedisAddr`（默认同 redisAddr）。
  - `external.ip`、`external.baseURL`（例 `http://10.2.3.1:8090`）、`external.webLoginURL`。
  - 日志：`logger.level`(0 未配置/1 debug/2 info/3 warn/4 error)、`logger.dir: ./logs`、`logger.lineNum`。
  - 短信：`smsCode`（非空则验证码固定为该值）、`smsProvider: aliyun`（可选 unisms）；`aliyunSMS`/`aliyunInternationalSMS`/`uniSMS`/`smsbao` 的 accessKey/secret 类字段。
  - 文件：`fileService: minio`（可选 aliyunOSS/seaweedFS/qiniu）；`minio.url/accessKeyID/secretAccessKey/uploadURL/downloadURL`；`oss.*`；`seaweed.url`；`qiniu.*`。
  - 推送：`push.contentDetailOn: true`、`pushPoolSize: 100`；apns(`dev/topic/password/cert`)、hms、mi、vivo、oppo、firebase(`jsonPath/projectId/packageName`)。
  - `register.off/onlyChina/stickerAddOff`；`account.systemUID: u_10000`、`fileHelperUID: fileHelper`、`systemGroupID: g_10000`、`systemGroupName: 意见反馈群`、`adminUID: admin`。
  - `avatar.defaultBaseURL/defaultCount:900/partition:100`；`shortNo.numOn/numLen:7/editOff`；`robot.messageExpire: 7d`、`inlineQueryTimeout: 10s`；第三方登录 `gitee`、`github`（`oauthURL/clientID/clientSecret`）。
  - 缓存：`tokenCachePrefix: "token:"`、`tokenExpire: 30d`、`loginDeviceCacheExpire: 5m`、`friendApplyExpire: 15d`、`nameCacheExpire: 7d` 等。
  - 环境变量示例：`db.mysqlAddr` → `TS_DB_MYSQLADDR`。
- WuKongIM 集成点：`wukongIM.apiURL`、`wukongIM.managerToken`、`grpcAddr`（6979）。
- 安全观察：
  - 示例 `mode: "debug"`，示例 DSN 使用 `root` 账号 + 弱密码 `demo`（仅示例）。
  - `grpcAddr` 绑定 `0.0.0.0:6979`（页面所述）；页面未说明该端口的访问控制（见 port.md 未列 6979）。
  - 大量密钥以明文写入 yaml（短信、OSS、推送、OAuth `clientSecret`、MinIO secret）。
  - `managerToken` 被描述为"可选"，不配则 WuKongIM 管理 API 无令牌（推断）。
  - `smsCode` 非空时固定验证码，生产使用即绕过短信验证（推断，页面仅称"测试短信验证码"）。
- 矛盾/缺口：compose/宝塔页中的 `TS_ADMINPWD`（superAdmin 密码）、`TS_MINIO_UPLOADURL` 以外的管理员配置项在本页 yaml 中**没有对应键**（`account.adminUID: admin` 与 superAdmin 用户名关系不明）；`TS_AVATAR_DEFAULTBASEURL` 对应 `avatar.defaultBaseURL` 为空默认，但 compose 给出了 dicebear URL；WuKongIM 侧的 `WK_*` 环境变量没有任何配置页说明（WuKongIM 完整配置不在此范围，见 api/conversation.md 引用 `/guide/fullconfig`，为跨站链接）；`webhook.msgNotifyEventPushInterval` 属 WuKongIM 配置，本页无。

## 5. dev/backend/deploy-mysql.md
- 状态：READ_LOCAL
- 标题：连接MYSQL
- 摘要：说明通过 adminer 网页访问 MySQL。
- 具体事实：地址 `http://xx.xx.xx.xx:8306`；服务器 `mysql`、用户 `root`、密码为 `MYSQL_ROOT_PASSWORD`、数据库 `im`。
- 安全观察：8306 adminer 对公网 IP 暴露 root 登录入口；compose 注释"调试用，为了安全生产不要打开"，但本页无此警告。
- 缺口：未说明如何关闭 adminer。

## 6. dev/backend/docker-install.md
- 状态：READ_LOCAL
- 标题：Docker 安装
- 摘要：Ubuntu 上按官方方式安装 Docker（卸载旧包 → 加 GPG key 与 apt 源 → 安装 docker-ce 等），其他系统指向官方文档。
- 具体事实：命令含 `apt-get remove` docker.io/docker-doc/docker-compose/podman-docker/containerd/runc；`curl -fsSL https://download.docker.com/linux/ubuntu/gpg`；安装 `docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin`。
- 安全观察：使用官方 GPG 签名源，较规范；无。
- 缺口：仅 Ubuntu。

## 7. dev/backend/xp-panel.md
- 状态：READ_LOCAL
- 标题：小皮面板部署
- 摘要：通过小皮面板应用商店一键安装唐僧叨叨，需自行在面板防火墙放行端口。
- 具体事实：安装脚本 `curl -O https://dl.xp.cn/dl/xp/install.sh && bash install.sh`；管理后台账号 `superAdmin`，密码在安装输出信息中；放行端口示例 82、83；监控 `http://IP:5300/web`；管理后台默认端口 83；Web 端"默认端口 83"。
- 安全观察：`curl | bash` 安装第三方面板；页面称 mysql、redis 在 Docker 内网不可外部访问，只暴露 wukongim、web、manager、**adminer** 端口（页面所述——adminer 被暴露）。
- 矛盾/缺口：Web 端口写 83，与管理后台端口 83 冲突，而 deploy-compose.yaml 中 Web=82、Manager=83；"通用脚本/其他"标签下写"第一步：安装 docker"但命令并未安装 docker；未说明 5001、9000 暴露情况。

## 8. dev/backend/verify.md
- 状态：READ_LOCAL
- 标题：验证部署
- 摘要：访问 `/v1/health` 验证。
- 具体事实：`GET http://xx.xx.xx.xx:8090/v1/health` 返回 `{"status":"up"}`，其他状态为启动失败。
- 安全观察：health 端点从公网可访问且无认证（推断，端口 8090 对外开放）。
- 矛盾：compose healthcheck 用 `/v1/ping`，本页用 `/v1/health`，两者关系页面未说明。

## 9. dev/backend/workflow.md
- 状态：READ_LOCAL
- 标题：推荐二次开发流程
- 摘要：修改 Makefile 镜像仓库 → 改 compose 的 image → `make deploy` 构建推送 → 服务器 `docker compose pull tangsengdaodaoserver; docker compose up -d`。
- 具体事实：命令 `make deploy`、`docker compose pull tangsengdaodaoserver;docker compose up -d`；老版本命令为 `docker-compose`。
- 安全观察：自建私有镜像仓库；页面未提镜像签名/鉴权。
- 缺口：文中"私有参考的镜像仓库"疑似笔误；未说明 Makefile 具体内容。

## 10. dev/backend/module.md
- 状态：READ_LOCAL
- 标题：（无，仅"待补充"）
- 摘要：占位页。
- 缺口：模块开发说明缺失（moduleimport.md 只讲引入）。

## 11. dev/backend/devenv.md
- 状态：READ_LOCAL
- 标题：开发环境搭建
- 摘要：克隆服务端仓库，用 testenv 的 compose 启动基础环境，`go run main.go`。
- 具体事实：`git clone https://github.com/TangSengDaoDao/TangSengDaoDaoServer.git`；`docker-compose -f testenv/docker-compose.yaml up -d`；`go run main.go`。
- 安全观察：无。
- 缺口：未给 Go 版本、未说明 testenv 内含哪些服务与端口、未说明配置文件如何指向它。

## 12. dev/backend/k8s.md
- 状态：READ_LOCAL
- 标题：k8s部署
- 摘要：仅一句"欢迎大佬贡献部署脚本"。
- 缺口：无 k8s 部署方案。

## 13. dev/backend/deploy.md
- 状态：READ_LOCAL
- 标题：一键部署（推荐）
- 摘要：用 TangSengDaoDaoCli（`tsdd`）一键安装，本质是自动配置好的 docker compose。
- 具体事实：要求 Linux 最低 2核8G，推荐 Ubuntu 22+，需 Docker；命令：`curl -sSL https://gitee.com/TangSengDaoDao/TangSengDaoDaoCli/raw/main/install.sh | sudo bash`、`sudo tsdd install --ip xx.xx.xx.xx`、`sudo tsdd start`、`sudo tsdd doctor`；API 地址 `http://xx.xx.xx.xx:8090/v1`；配置和数据均在 `~/tsdd/`。
- 安全观察：`curl | sudo bash`（从 gitee main 分支，无校验和/版本固定）；使用 HTTP 明文 API 地址；"自动配置好"意味着密码/密钥由安装器生成或使用默认值，页面未说明（推断）。
- 矛盾/缺口：question.md 序号 1000 推荐的系统包含 Ubuntu 20.04，而本页说推荐 22 或以上（"旧版本请使用其他方式安装"）；`~/tsdd` 与 sudo 运行下的 home 目录可能不一致（推断）；与 deploy-compose 的 `~/tsdd` 同目录名，混用风险（推断）。

## 14. dev/backend/rainbond.md
- 状态：READ_LOCAL
- 标题：Rainbond 部署
- 摘要：安装 Rainbond（在线脚本）后，从应用市场（Rainbond Cloud 应用 1676，选 1.5.1 版）安装唐僧叨叨，编辑 `tsdd-config` 配置组、修改对外访问 IP、网关开放端口。
- 具体事实：`curl -o install.sh https://get.rainbond.com && bash ./install.sh`；监控 `http://IP:5300/web`；管理后台默认 83；Web 默认 83；管理员账号"在截图中查看超级管理员帐号"（rainbond9.png，图片本地未解析）。
- 安全观察：同样 `curl | bash`；超级管理员账号由安装提示显示。
- 矛盾/缺口：Web 与管理后台同为 83（与 compose 的 82/83 矛盾）；监控图片复用 `1panel5.png`、`1panel*.png`，说明与 1Panel 页内容复制（推断）；"网关开发端口"疑为"开放"笔误；版本 1.5.1 与 compose 中 `tangsengdaodaoserver:v1.5` 的对应关系未说明。

## 15. dev/backend/1panel.md
- 状态：READ_LOCAL
- 标题：1Panel 部署
- 摘要：安装 1Panel，从应用商店先装 MySQL、Redis、MinIO，再用 TangSengDaoDao/appstore 仓库的 `1panel/tangsengdaodao` 作为本地应用导入并安装。
- 具体事实：`curl -sSL https://resource.fit2cloud.com/1panel/package/quick_start.sh -o quick_start.sh && sh quick_start.sh`；openEuler 用 `bash <(curl -sSL https://linuxmirrors.cn/docker.sh)` 装 docker；`git clone https://github.com/TangSengDaoDao/appstore.git`；`zip tangsengdaodao.zip tangsengdaodao`；上传到 `/opt/1panel/resource/apps/local` 并解压；监控 5300/web、后台 83、Web 83。
- 安全观察：多处 `curl | bash`（含第三方镜像站 linuxmirrors.cn）；MySQL/Redis/MinIO 由 1Panel 安装，凭据不在本页说明；appstore 压缩包来自 GitHub 主分支未固定版本。
- 矛盾/缺口：Web 端口写 83 与后台 83 冲突；与 compose 方案架构不同（MySQL/Redis/MinIO 外置于 1Panel 应用）但 appstore 内部配置本页未给。

## 16. dev/backend/deploy-compose.md
- 状态：READ_LOCAL
- 标题：Docker Compose方式部署(推荐)
- 摘要：给出完整 `docker-compose.yaml` 与 `.env`；目录 `~/tsdd`；启动 `docker-compose up -d`（新版 `docker compose up -d`）。页面称除 EXTERNAL_IP 必改外其他可默认。
- 具体事实：
  - 服务与镜像（registry.cn-shanghai.aliyuncs.com/wukongim/…）：`wukongim:v2`、`tangsengdaodaoserver:v1.5`（`command: api`）、`tangsengdaodaoweb:latest`、`tangsengdaodaomanager:latest`、`minio:RELEASE.2023-07-18T17-49-40Z`、`mysql:8.0.33`、`redis:7.2.3`、`adminer:latest`。
  - 发布端口：wukongim 5100(TCP)、5200(WS)、5300(监控)；5001 的映射**被注释**（"业务端调用，仅限内网开放"）；tsdd 8090；web `82:80`；manager `83:80`；minio 9000、9001（`--console-address ':9001'`）；adminer `8306:8080`；mysql、redis 未发布端口。
  - 健康检查：tsdd `wget http://localhost:8090/v1/ping`；minio `/minio/health/live`；mysql `mysqladmin ping`；redis `redis-cli ping`。web/manager `depends_on: tangsengdaodaoserver service_healthy`，`API_URL=http://tangsengdaodaoserver:8090/`。
  - volumes：`./wukongim:/root/wukongim`、`./tsdd:/home/tsdddata`、`./tsdd/configs:/home/configs`（yaml 配置，优先级 environment 高于文件）、`./miniodata:/data`、`./mysqldata:/var/lib/mysql`。mysql 使用 `--default-authentication-plugin=mysql_native_password`。
  - `.env` 默认值：`EXTERNAL_IP=xxx.xxx.xxx.xxx`；`MYSQL_ROOT_PASSWORD=Aa1234567`；`MYSQL_DATABASE=im`；`MINIO_ROOT_USER=minio`；`MINIO_ROOT_PASSWORD=Aa1234567`；`WK_MODE=release`；`WK_CONVERSATION_ON=true`；`WK_WEBHOOK_GRPCADDR=tangsengdaodaoserver:6979`；`WK_DATASOURCE_ADDR=http://tangsengdaodaoserver:8090/v1/datasource`；`WK_DATASOURCE_CHANNELINFOON=true`；`WK_TOKENAUTHON=true`；`WK_WHITELISTOFFOFPERSON=false`；`WK_JWT_SECRET=wk_secret_123`；`TS_MODE=release`；`TS_WUKONGIM_APIURL=http://wukongim:5001`；`TS_DB_REDISADDR=redis:6379`；`TS_SMSCODE=123456`；`TS_FILESERVICE=minio`；`TS_AVATAR_DEFAULTBASEURL=https://api.dicebear.com/8.x/avataaars/png?seed={avatar}&size=180`（备用 robohash）；`TS_ADMINPWD=admin1234567`（用户名 `superAdmin`）；`TS_MINIO_UPLOADURL=http://minio:9000`。
  - 页面建议修改：EXTERNAL_IP、MYSQL_ROOT_PASSWORD、MINIO_ROOT_PASSWORD、WK_JWT_SECRET（"为了安全建议修改"）、TS_ADMINPWD；`TS_SMSCODE` 为手机注册默认验证码。
- WuKongIM 集成点：gRPC webhook（`WK_WEBHOOK_GRPCADDR` → tsdd:6979）、datasource HTTP（`WK_DATASOURCE_ADDR`）、tsdd 调 WuKongIM `http://wukongim:5001`、`WK_TOKENAUTHON`、`WK_JWT_SECRET`、`WK_CONVERSATION_ON`、`WK_EXTERNAL_IP`。
- 拓扑：单机 docker compose，七八个容器同网络；MySQL/Redis 容器内网访问（tsdd DSN 用 `root` 账号）。
- 安全观察：
  1. **默认弱口令/默认密钥**：MySQL root `Aa1234567`、MinIO `minio/Aa1234567`、管理员 `superAdmin/admin1234567`、JWT 密钥 `wk_secret_123`；页面同时说"可使用默认值"与"建议修改"，表述自相矛盾，仅 EXTERNAL_IP 标"必须"。
  2. `TS_SMSCODE=123456`：固定验证码（页面所述：不为空则使用该值），默认保留则任何人可用 123456 通过手机验证（推断）。
  3. adminer（8306）发布到宿主机，镜像 `latest`，且页面自己注释"生产不要打开"，但默认 compose 启用。
  4. MinIO 控制台 9001 默认发布，而 port.md 标注 9001"不需要对外开放"。
  5. 5300 监控端口发布，port.md 未列出；监控 `/web` 是否带认证页面未说明。
  6. Redis 无密码（未发布端口，仅内网）；`WK_MANAGER_TOKEN` 类配置未出现 → WuKongIM 5001 API 无管理令牌（推断）；5001 虽未映射，但若用户取消注释则暴露。
  7. tsdd gRPC 6979 配置监听 0.0.0.0，但 compose 未发布，OK；web 到 API 走 HTTP 明文，无 TLS（页面全程 http://）。
  8. 镜像 `latest` 标签（web/manager/adminer）不可复现。
- 矛盾/缺口：未提 `TS_ADMINPWD` 在 fullconfig 中的键名；web 端口 82 与其他页"83"矛盾（见上）；`TS_DB_MYSQLADDR` 使用 root；`WK_WHITELISTOFFOFPERSON=false` 注释"默认开启…关闭则不判断好友关系"语义易混（键名含 OFF，false=白名单开启，推断）；无 TLS/nginx 说明（config-nginx.md 为空）。

## 17. dev/backend/port.md
- 状态：READ_LOCAL
- 标题：端口说明
- 摘要：端口与是否对外开放表。
- 具体事实：8090 API（是）；5001 WuKongIM API（否，"存在安全隐患"）；82 Web（是）；83 后台管理（是）；5100 TCP 长连接（是）；5200 WebSocket（是）；9000 MinIO 文件服务（是）；9001 MinIO 管理（否）；8306 mysql web 管理（否）。
- 安全观察：明确说 5001 不要给外网；83 管理后台、9000 要求对外开放（后台暴露公网，建议自行加固，推断）。
- 矛盾/缺口：未列 5300（监控，compose 已发布）、6979（gRPC）、3306（宝塔 compose 发布）；compose 实际发布了 9001、8306，表中"否"需用户自行不发布/防火墙拦截；Web=82 与多个页面的"83"不一致；对象存储 9000 需公网，页面未说明 TLS。

## 18. dev/backend/btpanle.md
- 状态：READ_LOCAL
- 标题：宝塔部署
- 摘要：安装宝塔面板，用 TangSengDaoDao/appstore 仓库 `btpanel` 目录，并手工在 `/home/tangsengdaodao` 建 `docker-compose.yaml` 与 `.env` 通过 Compose 模板安装。
- 具体事实：宝塔安装脚本带参数 `ed8484bec`（`install_6.0.sh ed8484bec`，推断为推广/渠道码）；compose 与 deploy-compose 类似但差异见下；`.env` 含 `CONTAINER_NAME=my`、`MYSQL_ROOT_PASSWORD=`（空）、`MINIO_ROOT_PASSWORD=`（空）、`TS_SMSCODE=123456`、`TS_ADMINPWD=admin1234567`、`TS_AVATAR_DEFAULTBASEURL=https://api.multiavatar.com/{avatar}.png`；超级管理账号 superAdmin，密码为 `TS_ADMINPWD`，默认 `admin1234567`；监控 `http://IP:5300/web`；后台 83；Web"默认端口 83"。
- 与 deploy-compose 的差异：minio 镜像为 `minio/minio:RELEASE.2024-04-06T05-26-02Z`（docker hub，非阿里云镜像）；mysql 额外发布 `3306:3306`；web/manager 的 `API_URL=http://${EXTERNAL_IP}:8090/`（走公网 IP，而非容器名）；web/manager 无 `depends_on`；tsdd 仅挂载 `./tsdd/logs:/home/logs`，无 `tsdddata`/configs 挂载；adminer 固定 `4.8.1`；`.env` 无 `WK_JWT_SECRET`、`TS_MINIO_UPLOADURL`；无 `docker-compose` 中的 wukongim 5001 映射（同样未映射）。
- 安全观察：**MySQL 3306 直接发布到宿主机**（root 密码来自 `.env`，示例为空——页面标注"必须修改"）；adminer 8306 发布；默认 superAdmin 密码 `admin1234567`；SMS 固定验证码 123456（页面把 TS_SMSCODE 列为"必须修改"，而 deploy-compose 称可选）；未设 `WK_JWT_SECRET`（使用 WuKongIM 默认，推断）；宝塔脚本 `wget … && bash` 带渠道码；port.md 未提醒 3306。
- 矛盾/缺口：见"与 deploy-compose 的差异"；`TS_AVATAR_DEFAULTBASEURL` 使用第三方头像服务 multiavatar（与 compose 的 dicebear 不同）；Web 端口 83 重复。

## 19. dev/backend/question.md
- 状态：READ_LOCAL
- 标题：常见问题
- 摘要：两个 FAQ。
- 具体事实：序号1000：`tsdd install` 报 `GLIBC_2.28' not found` 是系统过低，推荐 Ubuntu Lunar 23.04/Kinetic 22.10/Jammy 22.04/Focal 20.04（及"22.0 或以上"）；序号1001：MinIO 上传报 `The request signature we calculated does not match`，原因是 nginx 配置错误，`proxy_set_header` 需按 MinIO 官方 nginx 配置。
- 安全观察：无；暗示生产用 nginx 反代 MinIO。
- 矛盾/缺口：Ubuntu 20.04/22.10/23.04 与 deploy.md"推荐 22 或以上"表述不完全一致；未给 nginx 示例（config-nginx.md 为空）。

## 20. dev/backend/config-nginx.md
- 状态：READ_LOCAL
- 标题：无（文件大小 0 字节，空文件）
- 摘要：空页。
- 缺口：Nginx 配置内容缺失，而 question.md 序号1001 依赖 nginx 配置。

## 21. dev/backend/moduleimport.md
- 状态：READ_LOCAL
- 标题：模块引入
- 摘要：以朋友圈 `moments` 为例：复制模块目录到项目 `modules/`；在 `internal/modules.go` 增加空白导入。
- 具体事实：`_ "github.com/TangSengDaoDao/TangSengDaoDaoServer/modules/moments"`。
- 安全观察：模块以编译期空白导入引入，第三方模块会获得进程内全部权限（推断）。
- 缺口：与 structure.md 目录树不含 `modules/` 不一致；未说明 moments 模块的 SQL/配置。

---

# 二、api（7 页，WuKongIM 服务端 API，供业务系统调用）

> 注：下列 API 路径均以页面原样书写，页面未标注基础 URL；结合 port.md、api/index.md 的"5001 端口"提示，基址为 WuKongIM API（5001）属**推断**。所有页面均无鉴权头/令牌说明。

## 22. api/index.md
- 状态：READ_LOCAL
- 标题：基础
- 摘要：总述：所有 API 给自己业务系统调用，**不要暴露到外网**（把 5001 外网访问屏蔽）；HTTP 200 即成功，400 参数错误（返回 `{"msg","status":400}`），500 服务器错误；提供获取长连接地址的接口。
- 具体事实：
  - `GET /route?uid=xxxx` → `{"tcp_addr","ws_addr"}`
  - `POST /route/batch`，body 为 uid 数组 → `[{"tcp_addr","ws_addr","uids":[]}]`
- 安全观察：明确警告 5001 不得暴露；页面未提 token/签名；`/route` 的用途是"获取客户端连接悟空 IM 的地址"，客户端是否应直接调用（需公网）与"不要暴露"存在张力（推断）。
- 缺口：无鉴权章节；无 `managerToken` 的使用说明（fullconfig 有 `wukongIM.managerToken`）。

## 23. api/user.md
- 状态：READ_LOCAL
- 标题：用户
- 摘要：用户注册/登录令牌、在线状态、系统账号、踢设备。
- 具体事实：
  - `POST /user/token`：`{uid, token, device_flag(0 app/1 web), device_level(0 从/1 主)}`，存在则更新；同用户同设备标记的主设备互踢，从设备共存。
  - `POST /user/onlinestatus`：uid 数组 → 在线 uid 数组。
  - `POST /user/systemuids_add`、`POST /user/systemuids_remove`：`{uids:[]}`；系统账号"不受黑名单限制，无需在订阅列表里"。
  - `POST /user/device_quit`：`{uid, device_flag}`，-1 全部/0 app/1 web/2 pc。
- WuKongIM 集成点：`/user/token` 将业务端 token 写入 WuKongIM（`WK_TOKENAUTHON=true` 时 SDK 连接校验，推断）。
- 安全观察：token 由调用方自定义（"随机 uuid"），无 TTL/强度要求；`systemuids_add` 可授予全局发消息权限，一旦 5001 暴露则可冒充任意系统账号（推断）；无认证说明。
- 矛盾/缺口：device_flag 仅 0/1 与 device_quit 的 0/1/2（pc）不一致；webhook.md 也只有 0 app 1 web。

## 24. api/message.md
- 状态：READ_LOCAL
- 标题：消息
- 摘要：发送/批量发送消息，拉取频道消息，同步和回执离线命令消息。
- 具体事实：
  - `POST /message/send`：`header{no_persist,red_dot,sync_once}`、`from_uid`、`stream_no`、`channel_id`、`channel_type(1个人/2群)`、`payload`(base64)、`subscribers`（可选，指定接收人）。
  - `POST /message/sendbatch`：`from_uid`、`payload`、`subscribers`（建议每批 1000-10000）→ `{fail_uids, reason}`。
  - `POST /channel/messagesync`：`login_uid`、`channel_id`、`channel_type`、`start_message_seq`、`end_message_seq`、`limit`、`pull_mode`(0 向下/1 向上)；含详细拉取规则示例；响应 `{start_message_seq,end_message_seq,more,messages[]}`；消息字段 `message_id/client_msg_no/message_seq/from_uid/channel_id/channel_type/timestamp/payload/setting`。
  - `POST /message/sync`：`{uid, limit}` 离线命令消息（`sync_once=1`，建议仅 CMD 消息走写扩散）。
  - `POST /message/syncack`：`{uid, last_message_seq}`。
- 安全观察：`from_uid` 由调用方任意指定 → 能以任意用户身份发消息（若 5001 暴露，推断）；`channel/messagesync` 由 `login_uid` 参数声明身份，未提授权校验（推断：依赖 5001 内网隔离）；payload 引用 `/guide/proto#普通消息`（页面外）。
- 缺口：`/channel/messagesync` 放在消息页但路径为 `/channel/*`；message_seq 在本页注释为"用户唯一"，conversation.md 同字段注释为"频道唯一"；`sendbatch` 响应 JSON 示例含未加引号 uid 且带尾逗号（文档示例非严格 JSON）。

## 25. api/conversation.md
- 状态：READ_LOCAL
- 标题：最近会话
- 摘要：最近会话默认关闭，需配置 `conversation.on=true`；提供同步、设置未读、删除接口。
- 具体事实：
  - `POST /conversation/sync`：`{uid, version, last_msg_seqs("channelID:channelType:last_msg_seq|…"), msg_count}`；响应含 `channel_id/channel_type/unread/timestamp/last_msg_seq/last_client_msg_no/version/recents[]`。
  - `POST /conversations/setUnread`：`{uid, channel_id, channel_type, unread}`。
  - `POST /conversations/delete`：`{uid, channel_id, channel_type}`。
  - 对应 compose 的 `WK_CONVERSATION_ON=true`。
- 安全观察：同样以 `uid` 参数声明身份，无认证说明。
- 矛盾/缺口：路径单复数不一致（`/conversation/sync` 与 `/conversations/*`），需对照源码确认；频道类型此处出现 `3.客服`，其他页只有 1/2；引用 `/guide/fullconfig`、`/guide/initialize`（站外页面，不在审计范围）；超大群不维护最近会话（见 channel.md）。

## 26. api/channel.md
- 状态：READ_LOCAL
- 标题：频道
- 摘要：频道创建/删除、订阅者管理、黑名单与白名单管理。
- 具体事实：
  - `POST /channel`：`channel_id, channel_type, large(0/1，建议 500 成员以上，超大群不维护最近会话), ban, subscribers`。
  - `/channel/delete`（页面未写 POST）：`channel_id, channel_type`；提醒配置了 datasource 要避免返回已删除频道，否则重启后恢复。
  - `POST /channel/subscriber_add`（含 `reset`、`temp_subscriber`，临时频道订阅者重启后删除）、`POST /channel/subscriber_remove`。
  - `POST /channel/blacklist_add|blacklist_remove|blacklist_set`、`POST /channel/whitelist_add|whitelist_set`、`/channel/whitelist_remove`（页面未写 POST）。
  - 白名单机制可实现群禁言；黑名单禁止发言。
- 安全观察：频道/名单管理无鉴权说明；`ban` 可封禁频道（包括创建者）；黑/白名单语义若被篡改直接影响所有消息授权（推断）。
- 缺口：`/channel/delete` 与 `/channel/whitelist_remove` 未标注 HTTP 方法；`channel_type` 仅 1/2。

## 27. api/datasource.md
- 状态：READ_LOCAL
- 标题：Datasource
- 摘要：WuKongIM 通过配置的 URL 以 POST 调用业务系统，拉取订阅者、黑白名单、系统账号；仅在**重启后首次启动**时加载，要重新加载需重启。
- 具体事实：请求体 `{"cmd":"...","data":{...}}`；cmd：`getSubscribers`（`channel_type` 默认 2）、`getBlacklist`、`getWhitelist`（可实现群禁言/仅好友可发）、`getSystemUIDs`（无 data）；各自返回 uid 数组。compose 中对应 `WK_DATASOURCE_ADDR=http://tangsengdaodaoserver:8090/v1/datasource`、`WK_DATASOURCE_CHANNELINFOON=true`。
- WuKongIM 集成点：WuKongIM → 业务层 `/v1/datasource`（HTTP POST）。
- 安全观察：该端点在 8090 公网端口上（推断：同一进程 `/v1` 路由）；页面未描述对 WuKongIM 调用方的认证，若无认证则外部可枚举群成员/黑白名单（推断，需源码验证）。
- 缺口："数据源只在重启后第一次启动时加载"与 compose 注释（频道信息同步 `CHANNELINFOON`）的交互未说明；"获系统账号"标题笔误。

## 28. api/webhook.md
- 状态：READ_LOCAL
- 标题：Webhook
- 摘要：WuKongIM 将在线状态、离线推送消息、全量消息以 POST 回调第三方服务，事件名通过 query `event` 传入。
- 具体事实：
  - 示例地址 `http://example/webhook?event=user.onlinestatus`。
  - `user.onlinestatus`：数据 `[uid-设备标识-在线状态-连接ID-该设备标识在线数量-用户总在线设备数]`（开头示例却只有 3 段 `[uid1-0-1,…]`）。
  - `msg.offline`：消息数组，含 `to_uids`，用于调用厂商推送。
  - `msg.notify`：所有消息，批量推送，默认 500ms（`webhook.msgNotifyEventPushInterval`）；第三方可存 ES 做搜索；WuKongIM 自身始终保存消息。
  - Go 结构 `MessageResp`（`message_id`、`message_idstr`、`message_seq uint32`、`timestamp int32`、`payload []byte` 等）。
- WuKongIM 集成点：HTTP webhook（本页）vs compose 的 `WK_WEBHOOK_GRPCADDR`（gRPC 6979）。
- 安全观察：`msg.notify` 推送**全部**消息明文（payload base64，非加密）到第三方——数据面极宽；页面未描述回调签名/认证，接收端可被伪造请求（推断）。
- 矛盾/缺口：本页为 HTTP webhook，而 architecture.md 与 compose 使用 gRPC，两种模式并存关系未说明；在线状态示例格式（3 段 vs 6 段）不一致；`message_seq` 注释"用户唯一"；`MessageResp` 的 payload 类型 `[]byte` 注释 "base64"。

---

# Top security findings

1. **默认凭据与固定密钥直接写在官方 compose/.env 示例**（deploy-compose.md、btpanle.md）：MySQL root `Aa1234567`、MinIO `minio / Aa1234567`、后台 `superAdmin / admin1234567`、`WK_JWT_SECRET=wk_secret_123`；页面同时称"其他参数可使用默认值"。
2. **固定短信验证码 `TS_SMSCODE=123456`**，在示例中默认启用；页面所述该值非空即替代短信验证码（推断：默认部署存在手机号登录/注册绕过风险）。
3. **调试/管理端口默认公网发布**：adminer `8306`（MySQL 管理，且 compose 自注"生产不要打开"）、MinIO 控制台 `9001`、监控 `5300`（`/web`）；宝塔 compose 还发布 MySQL `3306`。port.md 标这些为"不需外开"，但示例 compose 实际发布它们。
4. **WuKongIM 5001 内部 API 无任何鉴权说明**（api/ 全部页面）；仅靠"不要暴露外网"。`managerToken` 为可选；`/message/send` 可任意设置 `from_uid`、`/user/systemuids_add` 可授予无限制发送权，一旦泄露可冒充任意用户/系统账号（推断）。
5. **全明文 HTTP**：所有地址示例均为 `http://`（API 8090、MinIO 9000、WuKongIM TCP/WS 5100/5200），无 TLS 方案；config-nginx.md 为空。
6. **`curl | sudo bash` 安装链**：deploy.md（gitee main 分支）、1panel、rainbond、btpanle（带渠道码）、小皮、linuxmirrors.cn，均无校验和/版本固定；镜像多处使用 `latest`。
7. **webhook/datasource 回调无认证说明**：`msg.notify` 把全部消息推给第三方；`/v1/datasource` 可返回群成员/黑白名单，若可被外部调用则泄露关系数据（推断）。
8. **管理后台（83）与 Web（82/83）对公网开放**，超级管理员单一固定用户名 `superAdmin`。
9. 明文密钥集中于 `tsdd.yaml`/`.env`（短信、OSS、推送、OAuth secret、APNs 证书路径）；无密钥管理建议。

# Facts to verify against source

1. Web 端口究竟是 82 还是 83（compose=82:80；1panel/rainbond/btpanle/xp-panel=83，与管理后台冲突）。
2. `TS_ADMINPWD` 在配置键中对应什么（fullconfig 无此键；`account.adminUID: admin` 与 `superAdmin` 的关系），以及未设置时是否有默认密码/随机密码。
3. `TS_SMSCODE` 是否在 `release` 模式下仍生效；固定验证码是否可用于注册、登录、重置密码。
4. WuKongIM 5001 API 是否校验 `managerToken`/`WK_MANAGER_TOKEN`；未设置时是否无认证；`WK_JWT_SECRET` 的实际用途（管理端 JWT 还是 token 校验）及宝塔 `.env` 缺省时的默认值。
5. WuKongIM Webhook 的实际传输：gRPC(6979，`WK_WEBHOOK_GRPCADDR`) 还是 HTTP(`?event=`)，两者是否同时支持；`/v1/datasource` 是否有来源校验。
6. `/v1/ping` 与 `/v1/health` 两个端点的真实存在与返回；是否无认证。
7. 5300 监控 `/web` 是否需要认证；5001 是否同在 5300 之外的监控接口上。
8. API 路径：`/conversation/sync` 与 `/conversations/setUnread|delete` 单复数是否真实；`/channel/delete`、`/channel/whitelist_remove` 的 HTTP 方法；`/route` 所在端口及是否面向客户端公开。
9. `message_seq` 是"用户唯一"还是"频道唯一"；`channel_type=3 客服` 是否存在；`device_flag=2 pc` 是否有效。
10. `modules/` 目录与 `internal/modules.go` 在 TangSengDaoDaoServer 仓库中的真实存在与 moments 模块的 SQL/配置依赖。
11. 镜像标签 `tangsengdaodaoserver:v1.5` 与 Rainbond "1.5.1" 的对应；`tsdd` CLI 安装器生成的口令/密钥策略；`~/tsdd` 目录权限。
12. `fullconfig` 的默认值（`mode`、`phoneSearchOff` 等）是源码默认还是示例值；`webhook.msgNotifyEventPushInterval` 的 WuKongIM 键名与默认 500ms。
13. 图片中信息（rainbond9.png、1panel7.png 等超级管理员账号提示）本次未解析，可能含密码展示方式，需人工查看（假定未读）。
