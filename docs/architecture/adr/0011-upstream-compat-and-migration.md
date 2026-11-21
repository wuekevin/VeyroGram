# ADR-0011 上游兼容与数据迁移：不提供 compat 层，不提供数据迁移，只支持全新部署
状态：Accepted（v1，可被推翻，见“重新打开的条件”）｜日期：2026-11-22｜Story 1.23｜绑定：NFR-12、AD-8、AD-18

## 决策
1. **不提供 compat 层。** 上游 TangSengDaoDao 的旧路径不在 VeyroGram 提供；compat 路径清单为空（`docs/migration/COMPAT-PATHS.md`），因此没有兼容窗口，也没有下线日期可写。
2. **不提供从上游数据库的迁移脚本。** 受支持的安装方式只有**全新部署**（Story 1.21 安装向导：配置检查 → 主机预检 → 迁移 → 首个管理员；`install run` 可重复执行，库里记录“已初始化”后拒绝再次初始化）。
3. 上述两条由测试守护：`tools/check_upstream_compat.py` 禁止新契约出现与上游端点清单相同的 (方法, 路径)，并要求上游每一张表都有书面处置（`docs/migration/upstream-tables.yaml`）。负面测试 `tests/ops/test_upstream_compat.py`。

## 依据（对上游源码 fc81513d 的审计，FOUND_IN_SOURCE）
| 事实 | 证据 | 对迁移的影响 |
|---|---|---|
| 口令为 `MD5(MD5(明文))`，无盐 | `modules/user/api.go` 985、1991、2453、2601 行 | 不能原样导入：VeyroGram 用户不使用口令登录（手机号+短信，AD-9），管理员用 Argon2id；导入弱哈希等于引入已知可撞库的凭据。 |
| 手机号明文存于 `user.phone`、`user.zone` | `modules/user/sql/user-20191106-01.sql` | VeyroGram 只存 HMAC 盲索引和后四位（0003）。可转换，但需要租户 HMAC 密钥且一次性明文过手，属于单独的数据处理决定。 |
| `uid` 为 VARCHAR(40)，我们是 CHAR(32) | 同上 / `0003_identity.up.sql` | 需要编号映射表，且 IM 侧频道 id 全部依赖 uid。 |
| 登录令牌是 Redis 里的不透明缓存项 | `api.go` 1105、1111 行 | 不可迁移；用户重新登录即可。 |
| 消息历史在 WuKongIM 自己的存储中（MySQL 的 message1..4 等是上游业务侧副本） | 上游表清单；我们的 IM 是打了补丁的固定构建，个人频道用组合 id，`msg.notify` | 历史导入要改写频道 id、重放序号并与 `message_ref` 对齐；没有 WuKongIM 官方导入接口可依赖，风险高，不做。 |
| 上游客户端使用旧路径；VeyroGram 客户端基于 `/v1` 契约生成（AD-8、AD-17） | `docs/audit/endpoints-inventory.yaml`（234 项） | 旧客户端无法直接连新服务；compat 层要长期背负两套语义，违背“新功能不进 compat”。 |
| 与上游路径无重叠 | 检查脚本：28 个契约操作 vs 234 个上游端点，0 冲突 | 现在没有历史包袱。 |
| 上游 60 张表 | `upstream-tables.yaml` | replaced 13、im-owned 9、import-candidate 11、out-of-scope 27（表项以文件为准）。 |

另一个前提：**VeyroGram 尚未发布，没有需要保护的线上部署**。这是做出本决策的主要原因：现在不做，成本接近零；等有了用户再补，成本会很高。

## 后果
- 想把现有 TangSengDaoDao 线上用户搬到 VeyroGram 的人，目前没有官方路径；需要重新打开本决策。
- 回滚：不适用（没有迁移可回滚）。全新部署的回滚走 Story 1.21/1.22 的发布回滚与备份恢复。
- 文档须明确“仅支持全新部署”：已写入 `services/api/README.md` 和本 ADR。

## 重新打开的条件（任一成立即应重新评估）
1. 存在要迁入的 TangSengDaoDao 现网部署（产品负责人确认，见 `docs/ops/MANUAL-CHECKLIST.md` C11）。
2. 出现必须继续使用旧客户端的合作方。
3. 采购/合规要求保留旧系统的历史消息。

若重新打开，最小可行方案（未实施）：只迁 `user`（经盲索引转换）、`friend`、`group*` 这些 `import-candidate`；不迁口令、令牌、消息历史；迁移前做 1.22 全量备份；脚本必须幂等、可回滚、产出校验报告（逐表行数和抽样比对）；按 `DATA-MODEL.md` 的 expand → migrate → contract 两步走。
