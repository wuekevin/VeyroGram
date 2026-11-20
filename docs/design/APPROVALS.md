# 设计门禁签字（APPROVALS）

> **怎么签字：** 这是一份由**你（产品负责人）或指定设计师**填写的记录。两种方式任选其一：
> 1. 直接编辑本文件：把对应行的“决定”改为 `APPROVED` / `REJECTED` / `WAIVED`，填写“签字人”“日期”“备注”，保存即可；
> 2. 或在对话里写“批准 G1、G2；驳回 G3，原因……”，我会**原样记录你的话**到本表（注明“据用户在对话中的指示记录”），但我不会自己填 APPROVED。
>
> `tools/check_gates.py` 会读取本表：任何必需门禁不是 `APPROVED` 或 `WAIVED`，退出码为 1。Phase 3（各端批量编码）开始前应通过它。
> `WAIVED` 表示你明知风险、书面豁免，需写明理由。

| 门禁 | 内容 | 评审材料 | 必需 | 决定 | 签字人 | 日期 | 备注 |
|---|---|---|---|---|---|---|---|
| G1 | 品牌标志与资产（Logo v2.0 = 方案 A+C，含三点；颜色、字标、各平台图标） | `brand/svg/logo-mark.svg`、`brand/svg/concepts/`、`brand/icons/`、`docs/design/brand-guidelines.md` | 是 | PENDING | | | 用户已选定 A+C，并按其参考图校正比例；用户在对话中表示“现在好了”（据用户指示记录，非签字）。G1 还含全套平台资产，需你明确回复“G1 批准”或在本行填写后才算通过 |
| G2 | 设计令牌与组件规格 | `packages/design-tokens/tokens.json`、`docs/design/COMPONENT-SPECS.md`、`DESIGN.md` | 是 | PENDING | | | |
| G3 | 关键流程高保真原型 | `docs/design/prototypes/*.html`（`?theme=dark` 看暗色） | 是 | PENDING | | | |
| G4 | 截图基线（手机/平板/桌面、明/暗、中/英、200% 字号） | `docs/design/baselines/` | 是 | PENDING | | | |
| G5 | 无障碍验收标准与复核方法（WCAG 2.1 AA 清单） | `docs/design/WCAG-2.1-AA-CHECKLIST.md`、`docs/design/ACCESSIBILITY-REVIEW-GUIDE.md` | 是 | PENDING | | | |
| G6 | 字体选择（Inter + Noto Sans SC，OFL） | `docs/design/DESIGN.md` Typography | 是 | PENDING | | | |

## 评审建议顺序
1. 先看 G1（Logo），这决定所有资产；选定后我才会批量重导出。
2. 打开原型页逐个看，重点：聊天、群管理、朋友圈、通话、后台、开发者控制台、设置、设备批准。
3. 对照 WCAG 清单，按 `ACCESSIBILITY-REVIEW-GUIDE.md` 用 VoiceOver / 键盘 / 缩放亲自试几个页面。

## 变更记录
- 2026-11-19 创建；全部 PENDING。
- 2026-11-19 Logo 经多轮反馈定稿为 v2.0（A+C，三点保留，比例按用户参考图，无弧线），成品待签字。
