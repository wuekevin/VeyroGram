# VeyroGram 品牌规范（v2.0，待签字 G1）

**标志方案（用户于 2026-11-19 选定“A + C”）：** 带尾巴的聊天气泡轮廓（渐变描边，青→蓝→靛→紫）+ 大号斜体粗体 **VG** 字母（青→紫渐变，从左到右）+ 字母下方三个“正在输入”圆点（青、蓝、靛，沿用原稿元素，与字母保持明显间距）+ 右上四角星；深海军蓝圆角方块底。**不含**方案 C 的下方弧线（“微笑”）。
这是**新设计**，不是对 `brand/source/logo.png` 的描摹：用户明确允许不贴近原稿，要求与当前 UI 语言一致（圆润几何、深色底 + 渐变）。该决定覆盖 BMAD-MASTER §7“logo.png 为唯一视觉源”的要求；原图仅留作历史参考。
字形来源：Poppins Black Italic 的 V、G 轮廓（SIL Open Font License 1.1，© The Poppins Project Authors；署名见 `docs/audit/license-and-ip.md`）。生成脚本 `tools/build_logo.py`（字形轮廓已固化为常量，无需字体文件即可重现）。方案对比图：`brand/svg/concepts/`（A–D、A+C 组合 E1/E2/E3；入选为 E1；随后按用户反馈补回三点，并按用户给出的参考图校正比例：字母不上移、圆点缩小并下移）。

## 1. 标志解剖
深色圆角方块底 → 气泡轮廓（圆 r=300 + 左下尾巴，描边 46）→ VG 字母（渐变填充，居中，宽约 440、高约 220）→ 三个圆点（r=19，间距 70，圆心 y=667，约在字母下方 80 单位）→ 四角星。识别基础是“气泡 + 斜体 VG + 圆点”。≤32px 使用不带圆点的小尺寸版本（`logo-mark-small.svg`）。

## 2. 采样色值（来自原稿的色板，仍沿用）
| 角色 | 值 |
|---|---|
| 底色上/下 | `#0A2159` → `#020B2A`/`#020B30` |
| 青 | `#00EAFD`、`#19E2FC` |
| 天蓝 | `#00BBFE`、`#2AB0FD` |
| 靛 / 紫 | `#6171FE`、`#8548FC`、`#4E2ED6` |
| 洋红（可选渐变末端） | `#E458FE` |
品牌渐变 135°：`#00EAFD → #2AB0FD → #6171FE → #8548FC`。**仅装饰与大字使用**；正文背景的渐变只能用 `gradient.action`（青→靛，最低 4.89:1）。字母渐变为 `#00EAFD → #4C9BFF → #A25BFF`。

## 3. 使用规则
- **留白**：标志四周至少留出“气泡直径的 1/4”。
- **最小尺寸**：完整标志 ≥ 48px；24–47px 用小尺寸版；托盘/菜单栏 16–22px 仅用气泡轮廓（三点在 22px 以下无法分辨）。
- **背景**：彩色标志放在深色（`ink-950/900`）或其官方底块上；浅色背景使用带底块的版本，或单色版（`logo-mono-black.svg`）。
- **禁止**：拉伸/旋转、改渐变色、加描边或阴影、替换字母、把位图放大冒充矢量、加入“安全/端到端加密”等未经验证的口号。
- **字标与口号**：产品名 “VeyroGram” 暂用 Inter Bold 排印 [ASSUMPTION，待确认]；口号保持中性（例：“Messaging for every screen”）。

## 4. 平台资产（已由 SVG 导出，路径 brand/icons/）
| 平台 | 产物 | 规则 |
|---|---|---|
| Android | `android/ic_launcher_{foreground,background,monochrome}-432.png`、`play-store-512.png`；SVG 源 `android-adaptive-*.svg` | 前景艺术占画布约 59%、居中，位于 66/108dp（61%）安全区内；背景为渐变底；单色层纯白剪影。**通知小图标需另绘纯白轮廓**（待办） |
| iOS | `ios/AppIcon-1024.png`（不透明、满版方形，由系统加圆角） | 无透明通道；主体 ≈98% 缩放；深/浅/着色变体待 Phase 3 出图 |
| Web/PWA | `web/favicon.svg`、`favicon.ico`、`site.webmanifest`、`favicon-{16,32,48}.png`（≤48px 使用“气泡 + 三点”简化图标，完整 VG 在此尺寸不可辨）、`apple-touch-icon-180.png`、`icon-{192,512}.png`、`maskable-512.png`、`og-1200x630.png` | maskable 内容位于 80% 安全区；OG 文案保持中性 |
| 桌面 | `desktop/icon.ico`（7 尺寸）、`icon.icns`（10 个条目）、`icon-{16…1024}.png`（≥48px 为带透明留边的圆角方块，≤32px 用简化图标）、`tray-template-*`/`tray-white-*` | 托盘 ≤22px 仅用气泡轮廓（三点在 22px 以下无法分辨）；macOS 模板图为纯黑 + alpha |
| Manager/开发者/官网 | `brand/svg/logo-mark.svg`（图标）；横版/竖版组合标志待排印确认后生成 | 文档与站点标记统一用 SVG |

## 4b. 追加资产（评审后补充）
- 启动页：`ios/launch-1290x2796.png`、`android/launch-1080x1920.png`（SVG 源 `brand/svg/launch-*.svg`，带发光标志）；Android 12+ 启动图标 `android/splash-icon-1152.png`。
- Android 状态栏通知小图标：`android/notification/drawable-{mdpi..xxxhdpi}/ic_stat_vg.png`（纯白 alpha 轮廓，24dp 基准）。
- iOS 变体：`ios/AppIcon-1024-dark.png`（更深底）、`ios/AppIcon-1024-tinted.png`（灰度，由系统着色）；均不透明。着色效果须在真机/模拟器上验证。
- 组合标志：`lockups/lockup-horizontal-{dark,light}`、`lockup-vertical-dark`（SVG 与 PNG）。**字标文字使用字体栈而非轮廓**，交付印刷/商标前须转为轮廓并确认字体。
- 发光变体仅用于大尺寸品牌场景，图标导出使用平面版。

## 5. 已知待办（诚实记录）
- 标志是我基于开源字体字形组合的**原创方案**，未经专业品牌设计师精修与商标检索；发布前须做商标近似检索（PRD OQ-1）并由设计师校验光学细节。
- 未做：品牌背景素材、字标转轮廓与字体确认、iOS 着色真机验证。
- 字母轮廓衍生自 OFL 字体：分发源码/资产时保留 OFL 署名；不得以 “Poppins” 名义分发修改后的字体文件（本项目只使用轮廓，不分发字体）。
