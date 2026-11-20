# COMPONENT-SPECS — VeyroGram（v0.1，与原型 `prototypes/*.html` 同源）

数值来自 `packages/design-tokens/tokens.json` 与原型 CSS；引用写作 `{token}`。组件在原型中以真实 DOM/CSS 实现，评审以截图基线为准（`baselines/`）。

| 组件 | 尺寸 / 间距 | 颜色与形状 | 状态 | 无障碍 |
|---|---|---|---|---|
| 主按钮 Button | 高 44（小号 36），水平内边距 20，字重 600 | 底 `{primary}`，字 `{on-primary}`，圆角 `{radius.md}`；品牌时刻用 `gradient.brand` + 深色字 | default / pressed(.72) / disabled(底 `text-disabled`) / loading | 焦点环 2px `{focus-ring}`；按钮文字对比 ≥ 4.5 |
| 次按钮 / 文字按钮 / 危险按钮 | 同上 | 次：`primary-subtle` + `link`；危险：透明 + `danger` 描边 | 同上 | 危险操作需二次确认 |
| 输入框 Field | 高 48（聊天输入栏 44，圆角 22），内边距 14 | 底 `surface`，边 `border-strong`，占位 `text-placeholder`，圆角 `{radius.md}` | default / focus(2px `focus-ring`) / error(图标+文字) / disabled | 占位符不替代 label；错误不只靠颜色 |
| 头像 Avatar | 36 / 48 / 72 | 圆形；无图时取名称首字，背景为固定 6 色之一 | online 点（受隐私设置） | 替代文本为昵称 |
| 会话列表项 | 最小高 72，内边距 12×16，头像 48 | 底 `surface`，分割线 `border` | 置顶 `pin`、免打扰 `bell-off`（图标，非仅变灰）、未读徽标（激活 `primary`；静音 `text-secondary`） | 读屏朗读“名称、末条、时间、未读数、是否置顶/免打扰” |
| 消息气泡 | 最大宽 78%，内边距 8×12，圆角 `{radius.bubble}`，发送者一侧底角 4 | 发出 `bubble-out`；接收 `bubble-in` + 1px `border`；次要文字用 `on-bubble-*-secondary` | 发送中 / 失败(重试) / 已送达 / 已读(`check-double`，受隐私设置) | 读屏朗读发送者、时间、状态；未知类型显示降级文案 |
| 引用块 / 系统消息 / 日期条 | 引用：左 3px 竖线；系统：圆角 12 胶囊，字 12 | `surface-sunken` + `text-secondary` | — | 系统消息使用 `role=status` 语义 |
| 附件卡（文件/位置/语音） | 文件卡宽 250；位置卡含 90px 预览 | 同气泡 | 文件：待扫描 / 已扫描 / 已隔离 | 文件名、大小、扫描状态可朗读 |
| 输入栏 | 高 44 控件，外边距 8，圆角 22 | 底 `surface`，上边 `border` | 空 / 输入中(发送按钮 `primary`) / 录音中 | 附件、表情、发送均有 `aria-label`（见 icons.json） |
| 底部 Tab | 高 64，图标 24 + 11px 文字 | 选中 `primary` + 加粗 + 图标填充(`states/*-selected.svg`) | default / selected / badge | 不单靠颜色：选中同时加粗与填充 |
| 左侧栏（桌面）/ 管理后台侧栏 | 桌面窄栏 72；后台 248，导航项高 40 | 选中 `primary-subtle` + `link` 加粗 | default / hover / selected / collapsed | 键盘可达；`nav` 地标 |
| 开关 Toggle | 48×28，滑块 22 | 开 `primary`，关 `border-strong` | on / off / disabled | `role=switch` + 状态文字 |
| 标签 Chip / 状态胶囊 | 高 24，圆角 12，字 12/600 | `ok/warn/bad` 用 `success/warning/danger` 描边（深浅主题均验证对比） | — | 文本表达状态，颜色辅助 |
| 数据表格（后台） | 行 `padding 12×16`，表头 12/600 | 表头 `surface-sunken` | 加载骨架 / 空 / 错 / 选中行 / 批量选择 | 表头 `th scope`；操作列有读屏文本“操作” |
| 详情抽屉（后台） | 宽 380，内边距 20，`elevation.e3` | 底 `surface` | 打开 / 加载 / 无权限 | 焦点陷阱与 Esc 关闭 |
| 一次性密钥块（开发者） | 等宽字体 13 | 掩码显示 + “轮换” | 创建时展示一次 / 已隐藏 | 说明文字常驻，复制按钮有反馈 |
| 横幅 Banner | 高 ≥ 36，内边距 8×16 | `primary-subtle` | info / offline / 网络较弱 | `role=status` |
| 骨架屏 / 空 / 错 / 离线 / 无权限 / 降级 | 见 `prototypes/states.html` | — | — | 错误含原因 + 重试，不只靠图标 |

## 通用规则
- 图标 24px 网格；导航与工具栏只用 `brand/icons/ui`，禁止 Emoji。
- 触控目标 ≥ 48dp(Android) / 44pt(iOS) / 32px(桌面)。
- 动效 120/200/320ms，`prefers-reduced-motion` 改为淡入淡出。
- 状态与错误不只靠颜色表达。
