---
name: VeyroGram
description: 深色优先、青蓝紫渐变的跨端即时通讯品牌与设计系统（v0.1 草案，待设计评审）。
status: draft
colors:
  brand-ink-950: '#020B2A'
  brand-ink-900: '#0A2159'
  brand-cyan-400: '#19E2FC'
  brand-cyan-300: '#00EAFD'
  brand-azure-500: '#00BBFE'
  brand-blue-400: '#2AB0FD'
  brand-indigo-500: '#6171FE'
  brand-violet-600: '#4E2ED6'
  brand-violet-500: '#8548FC'
  brand-magenta-400: '#E458FE'
  bg-canvas: '#F4F7FF'
  surface: '#FFFFFF'
  surface-sunken: '#E8EEFF'
  border: '#CFD8F2'
  text-primary: '#0A1230'
  text-secondary: '#46527A'
  text-disabled: '#8A94B8'
  primary: '#4E2ED6'
  on-primary: '#FFFFFF'
  primary-subtle: '#E6E0FF'
  link: '#3B27B8'
  focus-ring: '#0066CC'
  success: '#0E7A4B'
  warning: '#8A5A00'
  danger: '#C42B31'
  info: '#005FA3'
  bubble-out: '#4E2ED6'
  on-bubble-out: '#FFFFFF'
  bubble-in: '#FFFFFF'
  on-bubble-in: '#0A1230'
  text-placeholder: '#5B6690'
  border-strong: '#7C88B3'
  on-bubble-out-secondary: '#DCD4FF'
  on-bubble-in-secondary: '#46527A'
  bg-canvas-dark: '#020B2A'
  surface-dark: '#0A1840'
  surface-sunken-dark: '#050F33'
  border-dark: '#1E2E66'
  text-primary-dark: '#EAF1FF'
  text-secondary-dark: '#A5B2DA'
  text-disabled-dark: '#5E6C9E'
  primary-dark: '#2AB0FD'
  on-primary-dark: '#020B2A'
  primary-subtle-dark: '#0F2A66'
  link-dark: '#6FD0FF'
  focus-ring-dark: '#19E2FC'
  success-dark: '#3DDC97'
  warning-dark: '#FFC24D'
  danger-dark: '#FF7A80'
  info-dark: '#5CC8FF'
  bubble-out-dark: '#2AB0FD'
  on-bubble-out-dark: '#020B2A'
  bubble-in-dark: '#10225A'
  on-bubble-in-dark: '#EAF1FF'
  text-placeholder-dark: '#8A98C6'
  border-strong-dark: '#5C70B3'
  on-bubble-out-secondary-dark: '#0A2A5C'
  on-bubble-in-secondary-dark: '#A5B2DA'
typography:
  display:
    fontFamily: "Inter, 'Noto Sans SC', system-ui, sans-serif"
    fontSize: 32px
    fontWeight: 700
    lineHeight: 40px
  title-1:
    fontFamily: "Inter, 'Noto Sans SC', system-ui, sans-serif"
    fontSize: 24px
    fontWeight: 700
    lineHeight: 32px
  title-2:
    fontFamily: "Inter, 'Noto Sans SC', system-ui, sans-serif"
    fontSize: 20px
    fontWeight: 600
    lineHeight: 28px
  body:
    fontFamily: "Inter, 'Noto Sans SC', system-ui, sans-serif"
    fontSize: 16px
    fontWeight: 400
    lineHeight: 24px
  body-sm:
    fontFamily: "Inter, 'Noto Sans SC', system-ui, sans-serif"
    fontSize: 14px
    fontWeight: 400
    lineHeight: 20px
  caption:
    fontFamily: "Inter, 'Noto Sans SC', system-ui, sans-serif"
    fontSize: 12px
    fontWeight: 500
    lineHeight: 16px
rounded:
  sm: 6px
  md: 12px
  lg: 20px
  xl: 28px
  bubble: 18px
  full: 9999px
spacing:
  '1': 4px
  '2': 8px
  '3': 12px
  '4': 16px
  '5': 24px
  '6': 32px
  '7': 48px
  '8': 64px
components:
  button-primary: { backgroundColor: '{colors.primary}', textColor: '{colors.on-primary}', rounded: '{rounded.md}', height: 44px, padding: '0 {spacing.4}' }
  button-secondary: { backgroundColor: '{colors.primary-subtle}', textColor: '{colors.link}', rounded: '{rounded.md}', height: 44px }
  bubble-outgoing: { backgroundColor: '{colors.bubble-out}', textColor: '{colors.on-bubble-out}', metaTextColor: '{colors.on-bubble-out-secondary}', rounded: '{rounded.bubble}' }
  bubble-incoming: { backgroundColor: '{colors.bubble-in}', textColor: '{colors.on-bubble-in}', metaTextColor: '{colors.on-bubble-in-secondary}', borderColor: '{colors.border}', rounded: '{rounded.bubble}' }
  input-field: { backgroundColor: '{colors.surface}', textColor: '{colors.text-primary}', placeholderColor: '{colors.text-placeholder}', rounded: '{rounded.md}', height: 44px, borderColor: '{colors.border-strong}' }
  list-item: { backgroundColor: '{colors.surface}', minHeight: 64px, padding: '{spacing.3} {spacing.4}' }
  tab-bar: { backgroundColor: '{colors.surface}', height: 56px, iconSize: 24px }
  sidebar-manager: { backgroundColor: '{colors.surface}', width: 248px, iconSize: 20px }
---

## Brand & Style

VeyroGram 的视觉来自唯一品牌源 `brand/source/logo.png`：深海军蓝底、青→蓝→紫渐变的聊天气泡环（左下带尾）、斜体 VG 字标、绕环的轨道弧、右上四角星与三个“正在输入”圆点。气质是**冷静、清晰、略带科技感的星际感**，而不是糖果色社交 App；渐变是品牌签名，只用于品牌时刻（启动页、空状态主图、发送按钮高光、品牌图），不铺满界面。

产品界面以**深色为主场、浅色为同等公民**：深色主题直接取自 Logo 底色；浅色主题改用高对比的靛紫作为主色，以保证文字可读。不得把上游 UI 换色后沿用，所有组件按本规范重新设计（BMAD-MASTER §7）。

## Colors

- **品牌色板（`brand-*`）**只在品牌资产与渐变中使用，不直接当作界面文字色：`brand-ink-950 #020B2A`、`brand-ink-900 #0A2159` 是 Logo 底色；`brand-cyan-300/400`、`brand-azure-500`、`brand-blue-400`、`brand-indigo-500`、`brand-violet-500/600`、`brand-magenta-400` 是渐变节点（均为从 logo.png 采样的值）。
- **语义色（`primary`、`link`、`success`、`warning`、`danger`、`info`）**为界面使用。浅色主题 `primary {colors.primary}`（取自 Logo 环右侧的紫 `#4E2ED6`）配白字；深色主题 `primary-dark {colors.primary-dark}`（天蓝）配深色字。
- **聊天气泡**：发出方 `bubble-out`，接收方 `bubble-in`；气泡内的次要文字（时间、已读）用 `on-bubble-out-secondary` / `on-bubble-in-secondary`，**不得沿用 `text-secondary`**（在发出方气泡上对比度仅约 1:1）。浅色主题接收方气泡与画布对比很低（约 1.07:1），因此接收方气泡必须带 1px `border`；气泡边界是装饰，信息由其内文字承载。不用渐变做气泡底。
- **对比度**：全部文本/界面色组合已用 WCAG 公式计算，44 项检查全部通过（正文类 ≥ 4.5:1；焦点环、主色边界与输入框强边框 ≥ 3:1；最低正文类比值 5.23:1）。规则在 `tokens.json` 的 `contrast_rules`，由 `packages/design-tokens/check_contrast.py` 实际计算，不通过则 `build.py` 拒绝产出令牌，结果写入 `dist/contrast-report.json`。`border` 仅为装饰性分隔，**不满足 3:1**；输入框等需识别边界的控件使用 `border-strong`。
- 不用：纯红/纯绿作为唯一状态指示（须同时有图标/文字）；渐变作为文字色；品牌洋红作为错误色。

## Typography

字体栈为 `Inter` + `Noto Sans SC`（均为 OFL 开源许可）[ASSUMPTION：用户未指定字体，待设计评审确认]。移动端遵循平台字体与字号缩放（Android sp / iOS Dynamic Type），不固定像素。字阶：`display 32/40`、`title-1 24/32`、`title-2 20/28`、`body 16/24`、`body-sm 14/20`、`caption 12/16`。中文行高不低于 1.5。聊天正文使用 `body`，会话时间/状态使用 `caption`。

## Layout & Spacing

4pt 基线：`1=4px … 8=64px`。手机单栏；平板双栏；桌面三栏（会话列表 / 聊天 / 详情），断点 `tablet 600`、`desktop 960`、`wide 1440`。触控目标不小于 Android 48dp / iOS 44pt / 桌面 32px。页面边距 16px（手机）、24px（桌面）。

## Elevation & Depth

深色主题以**色阶分层**表达层级（canvas → surface → raised），阴影仅用于浮层（`e1` 轻、`e2` 菜单/弹层、`e3` 对话框），浅色主题用细边框 + 极浅阴影。禁止模糊发光滥用；Logo 的发光效果只存在于品牌大图。

## Shapes

圆角语言取自 Logo 的圆润方块与气泡：`md 12` 为控件默认，`bubble 18` 为消息气泡，`lg 20` / `xl 28` 用于卡片与底部弹层，`full` 用于头像与徽标。消息气泡贴发送者一侧的底角收小（4px），呼应 Logo 的气泡尾巴。

## Components

组件的视觉令牌见上方 `components`；行为见 EXPERIENCE.md。通用规则：
- **按钮**：主按钮高 44，圆角 `md`；禁用态使用 `text-disabled` 而非降低整体不透明度，避免对比度失真。
- **输入框**：边框用 `border-strong`，占位符用 `text-placeholder`（**不得用 `text-disabled`**，其对比度不足 4.5:1），焦点环 2px `focus-ring`，错误态同时显示图标与文字。
- **列表项**：最小高 64，头像 48，未读徽标用 `primary`，免打扰用 `bell-off` 图标而非变灰。
- **图标**：来自 `brand/icons/ui`（69 个原创 SVG，24px 网格、1.75 描边、圆角端点、live area 20）。导航/工具栏/Tab/后台侧栏**只能用该图标库，禁止 Emoji 字符**；`tools/lint_no_emoji_nav.py` 已可本地运行（启发式，非证明），**CI 流水线尚未接入**（见 DESIGN-SYSTEM.md §3）。
- **Tab Bar / 侧栏**：图标 + 文本标签；选中态用 `primary` 色 + 加粗，不依赖颜色单独表示。

## Do's and Don'ts

Do：深色/浅色同时验收；所有状态（加载/空/错/无权限/离线）都有设计；文本放大到 200% 不裁切；新图标先按 24px 网格与 1.75 描边绘制。
Don't：不使用 Emoji 做导航或工具栏图标；不把渐变用作文字或长列表背景；不从 PNG 放大生成矢量；不复用上游页面样式；不在界面里宣称“安全/加密已审计”等未经验证的说法。
