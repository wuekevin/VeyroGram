# compat 路径清单（Story 1.23，NFR-12 / AD-8）

COMPAT_PATHS: none

VeyroGram 不提供上游 TangSengDaoDao 旧路径的 compat 层。决策与理由见 [ADR-0011](../architecture/adr/0011-upstream-compat-and-migration.md)。

- 本清单为空是**被测试守护的状态**：`tools/check_upstream_compat.py` 把 `packages/contracts/openapi/*.yaml` 的每个 (方法, 路径) 与上游端点清单
  （`docs/audit/endpoints-inventory.yaml`，234 项）比对，出现任何相同项即失败，防止旧路径悄悄混入新契约。
- 将来若重新决定提供 compat：在这里逐条列出路径、下线日期和责任人，并把上面的标记改为 `COMPAT_PATHS: listed`，同时更新检查脚本。
