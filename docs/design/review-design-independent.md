# Veyrogram 设计交付物独立评审

## 1. 标志还原度

方法：用 headless Chrome 以 1024×1024 渲染 `brand/svg/logo-mark.svg`，再与 `brand/source/logo.png`（1024²）在同一区域裁切后左右并排对比。SVG 只有 6 个 path、约 2 KB，没有使用 filter。

| # | 差异 | 严重度 |
|---|---|---|
| 1 | **字母形态不一致**。原图 V、G 都是斜体（约 15–20° 倾斜），笔画末端是斜切的尖角，V 的两臂粗细不同。SVG 中 V 是直立的几何 V，两臂同粗，右臂顶端直接连到 G 的顶横，形成原图里没有的连字。 | 高 |
| 2 | **G 的构造完全不同**。原图 G 是斜体，顶部是斜切的弧形，内部横杠是一个斜切的平行四边形，下半碗形开口朝右上。SVG 的 G 是“矩形顶条（圆角端）+ 圆形碗 + 胶囊形横杠 + 右下方的斜切尾巴”拼起来的，左侧碗与 V 臂交接处有凹口，整体比例偏宽、偏重。 | 高 |
| 3 | **轨道弧（orbit）的路径和层次不对**。原图左下的弧是从气泡外侧绕进来的完整环带：它在左下形成闭合的回环，压在气泡描边上方，再从字母后面穿过，与右侧的弧连成一条连续的椭圆。SVG 中左弧在 V 下方（x≈520）收成尖头就断了，右弧从 G 右下角单独起笔，两段视觉上不连续。左端是圆头，不是原图那样的回环。 | 高 |
| 4 | **没有发光效果**。原图有明显的霓虹外发光（bloom），气泡环、字母和星都有。SVG 完全是平涂渐变，没有 glow/blur，气质从“霓虹”变成了“扁平”。 | 中（扁平化可以算有意简化，但文档需要说明） |
| 5 | **渐变分布不同**。原图字母内部是从青色到紫粉色的左右渐变，G 的右上偏粉紫。SVG 中字母整体偏蓝，紫色只出现在 G 的右下。气泡环原图在右下有洋红色高光，SVG 偏紫。 | 中 |
| 6 | **四角星**。原图星形的四个尖角是内凹曲线，带柔光，颜色接近白青。SVG 是直线构成的菱形星，颜色更饱和的青色。 | 低 |
| 7 | **气泡尾巴与底板**。尾巴的走向基本一致。底板圆角矩形的 rx=235 与原图接近，背景从上到下的蓝色渐变也接近。 | 低 |
| 8 | 三点、气泡环的位置和半径与原图高度吻合（这部分做得好）。 | — |

**结论**：整体构图（圆角底板、对话气泡、三点、轨道、星、VG）都对得上，缩到 48px 以下时辨认度足够。但字母（V/G）和轨道弧这两个识别核心与原图有明显结构差异，1024px 下并排一眼就能看出不是同一个标志。**以目前状态不能直接作为发布用的主标志（release art）**，可以当作“扁平化简化版/小尺寸版”的草稿。建议重新描摹斜体 VG 字形，并把轨道弧改成一条连续的椭圆环带（前后分层遮挡），再决定是否加一层可选的 glow 版本。

## 2. 平台资产正确性

检查方法：`sips -g hasAlpha -g pixelWidth`、`file`、`iconutil -c iconset`，用 PIL 测量 alpha/亮像素的包围盒，再把关键图拼成接触表目视检查。

| 资产 | 实测 | 判定 |
|---|---|---|
| iOS `AppIcon-1024.png` | 1024²，hasAlpha=no，满幅不透明（角像素是 #0A2058） | 通过 |
| Android 前景 / 单色 432² | 有 alpha，内容 bbox 为 (119,120)–(312,309)；66/108 安全区是 84–348 | 通过。内容只占画布约 45%，在启动器里会显得偏小，可以放大到约 58–60% |
| Android 背景 432² | 不透明 | 通过 |
| Play Store 512² | 不透明 | 通过 |
| Web maskable 512² | 不透明，亮像素离中心最远约 171px，小于 80% 安全圆半径 205px | 通过 |
| Web icon-192/512、apple-touch-180、favicon 16/32/48、og 1200×630 | 尺寸都正确，都不透明 | 尺寸通过；见下面 favicon 的问题 |
| `icon.icns` | `file` 识别为 Mac OS X icon；`iconutil` 能解出 10 个条目（16–512@2x） | 格式有效 |
| `icon.ico` | `file` 识别为 7 个 PNG 条目（16…），RGBA | 格式有效 |
| 托盘 template / white 16/22/32 | 有 alpha，单色（template 只有 1 种颜色），符合 macOS template 要求 | 格式通过 |

**发现的问题：**
1. **桌面图标是满幅正方形（中，macOS 上为高）**。`desktop/icon-*.png` 和 ICNS 的源图 hasAlpha=no，深蓝底一直铺到画布四角。macOS Big Sur 以后的规范要求图标主体是约 824/1024 的圆角方形，四周留透明边距，并带阴影。照现在的样子在 Dock 里会是一块硬边方块，和其他图标的尺寸、形状都不一致。Windows ICO 用满幅方块可以接受，但通常也会留透明圆角。
2. **favicon-16 不可辨认（中）**。16px 下 VG、轨道、星糊成一团蓝紫色噪点。建议给 16/32 单独出一个简化版（只保留气泡和 V，或者只保留气泡和三点），不要直接缩小完整标志。
3. **托盘 16px（低）**。气泡轮廓可以辨认，但三点在 16px 合成了一条横线，22px 稍好一些。这还算可以接受，但和完整标志的关联很弱（没有 VG），属于设计取舍，文档应写明。
4. Android 单色层只是完整标志的白色剪影，包含细小的星和三点，在 Android 13 主题图标（约 24dp 内容）下细节会丢失。建议做简化版。

## 3. UI 图标集

**一致性（通过）**：`brand/icons/ui` 下共 55 个独立 SVG 加 1 个 `sprite.svg`。sprite 里正好 55 个 `<symbol>`；`icons.json` 里 `count` 为 55，条目也是 55 个，与文件名一一对应，没有多余或缺失。所有图标都是 `viewBox="0 0 24 24"`、`stroke-width="1.75"`、`stroke="currentColor"`、`fill="none"`，参数统一。`icons.json` 自己写明了 status 为 “v0.1 hand-authored draft; designer review pending”，这一点是诚实的。

**文档引用**：EXPERIENCE.md 和 DESIGN.md 里用反引号写出的图标名有 `chat` `contacts` `moments` `discover` `me` `bell-off`，**全部存在**，没有引用了却缺失的图标。DESIGN.md 里其余的反引号标记都是令牌名，不是图标。

**语义歧义 / 视觉问题**（来自 64px 接触表）：
- `moments` 画的是相机。它会被理解为“拍照/相机”，而图标库里也没有单独的 camera 图标，聊天输入栏的拍照入口会和 Tab 撞车。（中）
- `users-admin` 画的是“人 + 加号”，通常读作“添加好友/添加成员”，不像“用户管理”。（中）
- `dashboard`（柱状图）和 `chart`（折线图）语义重叠；`audit`（文档行）和 `file` 很像；`contacts`（两个人）和 `group`（三个人）在 20px 下难以区分。（低）
- `bell-off` 的斜杠伸出了铃铛主体，左上角贴近画布边，超出了声明的 20px live area。`key` 是 45° 倾斜，视觉重量比其他图标大。`qr` 右下角的定位块不完整，小尺寸下看着像“残缺的 miniapp”。（低）

**覆盖缺口**（文档描述的交互需要，但图标库里没有）：camera、attach/回形针、emoji/键盘切换、copy、delete-for-everyone 以外的撤回、已读双勾（`check` 只有单勾）、warning/error/info 状态图标（DESIGN.md §148 要求“错误态同时显示图标”，但库里没有 alert 类图标）、offline/wifi-off（EXPERIENCE 提到离线状态）、refresh、chevron-right（列表进入箭头）、menu、logout、filter，以及桌面窗口控件。这些属于 v0.1 范围问题，但 DESIGN.md 声明导航和工具栏“只能用该图标库”，所以缺口会直接卡住实现。

## 4. 令牌与对比度

**一致性（通过）**：DESIGN.md frontmatter 中的 50 个颜色与 `tokens.json` 逐项比对（brand 10 个、light 20 个、dark 20 个，暗色主题在 frontmatter 里带 `-dark` 后缀），**0 处不一致，也没有多出来的键**。字号、间距（1–8）、圆角与 tokens 一致。

**已声明的 34 组对比度**：我用 WCAG 2.x 相对亮度公式自己重算，34 组的比值都与 `contrast_verified` 相差不超过 0.02，pass 标记也都正确。但要注意以下几点：
- 34 组里有 2 组是 `border` 对背景，门槛是**自定义的 1.2:1**（实测 1.33 / 1.51）。这不是 WCAG 的门槛：WCAG 1.4.11 对“理解界面所必需的”组件边界要求 3:1。如果输入框只靠 `border` 来界定边界，就**不达标**（light 时 border 对 surface 只有 1.42）。文档说“34 组全部通过 WCAG AA 门槛”，这个说法夸大了。
- 没有任何脚本在计算对比度。`packages/design-tokens/build.py`（18 行）、`tools/*.py` 中都找不到 contrast/luminance 相关代码，`contrast_verified` 是写死在 JSON 里的静态数据。所以 DESIGN-SYSTEM.md 第 5 行说的“由生成脚本在写入前校验……任何 token 改动必须重跑校验”**与事实不符**（见 §5）。

**我自己补算的组合（选取）**：

| 主题 | 前景 / 背景 | 比值 | 判定 |
|---|---|---|---|
| light | on-bubble-out / bubble-out | 7.82 | 通过 |
| light | on-bubble-in / bubble-in | 18.39 | 通过 |
| dark | on-bubble-out / bubble-out | 8.05 | 通过 |
| dark | on-bubble-in / bubble-in | 13.25 | 通过 |
| light / dark | link / surface | 9.68 / 9.97 | 通过 |
| light | link / primary-subtle（次按钮文字） | 7.60 | 通过 |
| light / dark | warning / surface | 5.93 / 10.74 | 通过 |
| light / dark | success / surface | 5.38 / 9.76 | 通过 |
| light | success / bg-canvas | 5.02 | 通过 |
| **light** | **text-disabled / surface** | **3.00** | 不足 4.5（禁用文字按 WCAG 1.4.3 可以豁免，但如果拿来当占位符就不能豁免） |
| **light** | **text-disabled / bg-canvas** | **2.80** | 同上，更低 |
| **light** | **text-disabled / surface-sunken** | **2.58** | 同上（搜索框通常就是 sunken 背景） |
| dark | text-disabled / surface | 3.38 | 同上 |
| **dark** | **text-disabled / bubble-in** | **2.94** | 低 |
| **light** | **text-secondary / bubble-out**（如果气泡内时间戳沿用 secondary） | **1.02** | **几乎不可见**：需要一个 `on-bubble-out-secondary` 令牌 |
| light | bubble-in / bg-canvas | 1.07 | 浅色模式下接收气泡（白）放在画布（#F4F7FF）上几乎没有边界，需要描边或阴影 |
| — | 白 / brand-violet-500、magenta-400、cyan-300 | 4.85 / 2.95 / 1.48 | 品牌色上不能放白字（洋红、青色） |

**问题汇总：**
1. 缺少占位符（placeholder）令牌。如果实现方拿 `text-disabled` 当占位符，light 主题下只有 2.58–3.0。建议新增 `text-placeholder`，值不低于 4.5:1（light 可以复用 `text-secondary`）。（中）
2. 缺少气泡内次要文字的令牌（时间戳、已读状态、引用回复的预览）。在出方气泡上用 secondary 会变得不可见。（中–高）
3. `border` 1.2:1 的门槛是自定义的，组件边界需要 3:1 或者用其他方式表达（比如 sunken 填充）。（中）
4. 浅色接收气泡与画布对比只有 1.07。（低–中）

## 5. 规则违规/未经证实的声明

**§0.7（导航禁用 Emoji）**：设计文档中没有违规，EXPERIENCE.md 的 Tab 和侧栏全部引用图标库里的名称。`tools/lint_no_emoji_nav.py` 在仓库根目录跑一遍得到 0 项、exit 0；对 `upstream/TangSengDaoDaoManager` 跑一遍也是 0 项，与文档的说法相符（Manager 里只有 2 行 Emoji，都在注释里：Breadcrumb.vue:33、router/index.ts:8）。我在 scratchpad 里做了自测：`nav/Menu.ts` 里放 🏠，被检出，exit 1，这一点**属实**。但这个 lint 有两处**实质缺陷**：
- **漏报**：它按文件路径做启发式匹配（nav|menu|tab|sidebar…），`src/router/index.ts` 里写 `title:"💬 Chats"` **不会被扫描**。可是 Vue/React 后台的菜单通常就是从 router 的 meta 生成的（上游 Manager 正是这种结构）。所以“由 lint 阻断”的保证覆盖不到最常见的场景。（高）
- **误报**：正则包含 U+231A–23FF，`⏎`（U+23CE）这种快捷键提示会被判成 Emoji；它还包含 U+2194–21AA 这类箭头。（低）

**§0.8 / 未经证实的声明**：文档在 E2EE、绝对安全、合规这些方面措辞克制（EXPERIENCE.md 第 41 行写的是“未经审计前不宣称已合规”），这一点合格。下列说法与文件实际情况**不符或言过其实**：

| 文档位置 | 声明 | 实际情况 | 严重度 |
|---|---|---|---|
| DESIGN.md:150 | “CI 以 `tools/lint_no_emoji_nav.py` 阻断” | 仓库里**没有任何 CI 配置**（没有 `.github/workflows`，也没有任何 yml/Makefile 引用这个 lint） | 高 |
| DESIGN-SYSTEM.md:5 | “对比度由生成脚本在写入前校验……任何 token 改动必须重跑校验” | 没有任何脚本计算对比度（build.py 只有 18 行，`tools/` 和 `packages/` 里 grep contrast/luminance 都只命中 tokens.json）。`contrast_verified` 是静态写入的数据 | 高 |
| DESIGN.md:125 / DESIGN-SYSTEM.md:5 / EXPERIENCE.md:41 | “34 组全部通过 WCAG AA 门槛” | 34 组的数值是对的。但其中 2 组（border）用的是自定义 1.2:1 门槛，不是 WCAG 门槛；text-disabled、占位符、气泡内次要文字都没有覆盖 | 中 |
| brand-guidelines.md:35 | “手工重绘版与原图结构一致，只是缺发光、字重略轻” | 低估了差距：VG 字形（斜体 vs 直立、G 的构造）和轨道弧（连续 vs 断成两段）都有**结构性**差异（见 §1） | 中 |
| brand-guidelines.md:28 | “Android 前景内容约 60% 居中” | 实测前景内容 bbox 为 193/432 ≈ **45%** 画布（占 66dp 安全区的 73%），与 60% 不符 | 低 |
| DESIGN-SYSTEM.md:23（平台映射表） | Android “自适应图标、通知小图标待绘” | 自适应图标的三层其实已经存在，与 brand-guidelines 自相矛盾（只有通知小图标真正缺失） | 低 |
| DESIGN.md:150 | “55 个原创 SVG……live area 20” | 55 属实；但 `bell-off` 的斜杠超出了 20px live area | 低 |

**核实属实的数字**：UI 图标 55 个（文件、json、sprite 三者一致）；ICO 7 个尺寸（16/24/32/48/64/128/256）；ICNS 10 个条目；对比度 34 组（明暗各 17 组），最低正文类 5.38（light success），重算一致；`logo.png` 是 1024² RGBA，四角透明，alpha bbox (60,65)–(965,968) 与文档完全一致；`dist/` 下 tokens.css、android、android-night、ios 四个产物都存在。

**上游样式复用**：设计文档和 `tools/` 中没有引用或拷贝 `upstream/` 的样式或资源，只有“不得复用”的声明。合格。

## 6. 与 MASTER §7 的差距

已产出：brand-guidelines.md、手工重绘的 SVG（草稿）、调色板、字体（作为假设）、间距、圆角、断点、令牌流水线（css/android/ios）、Android 自适应图标三层与 Play 512、iOS 1024、Web favicon（svg/16/32/48）/192/512/maskable/OG、桌面 ICO/ICNS/PNG/托盘、55 个 UI 图标与无障碍名称、EXPERIENCE/DESIGN 规范文本。

**尚未产出**（对照 §7 原文逐条列出）：
1. **发布级主标志**：现在的 SVG 与原图有结构差异（§1），文档也自认“需设计师精修”。
2. **Android**：通知小图标（纯白轮廓）、启动页（Splash）、应用内品牌图；按 Android 版本、深色模式核对的验证记录。
3. **iOS**：除 1024 以外的多尺寸 AppIcon 资产目录（`AppIcon.appiconset/Contents.json`；如果走 Xcode 单尺寸模式可以不要，但需要说明）、深色/着色图标变体、Launch Screen、隐私权限说明文案。
4. **Web/PWA**：**Web 用的 `favicon.ico`**（只有 desktop/icon.ico）、单独的 Twitter 卡图（可以复用 OG，但需要说明）、`manifest.webmanifest`。
5. **桌面**：通知图标、安装程序图形（NSIS/DMG 背景）、macOS 规范的圆角加留边图标（目前是满幅方块，§2）、Linux 的 hicolor 尺寸目录或 .desktop 约定。签名/更新渠道属于工程范畴。
6. **Manager/开发者平台/官网**：横版和竖版组合标志、品牌背景资产、落地页 OG、文档站标记（文档里写着“待排印确认”）。
7. **图标库状态集**：§7 要求“状态集”。目前只有文字描述（默认/选中/禁用/徽标），没有选中填充版或徽标叠加的资产；覆盖缺口见 §3（camera、attach、已读双勾、alert/info、offline、copy、chevron 等）。
8. **设计系统组件**：表单、弹层、列表、表格、空状态、错误状态、骨架屏、Toast、权限弹窗只有清单，没有规格或视觉稿；阴影和动效只有令牌。
9. **各端界面**：Android/iOS 全部页面、Web/桌面三栏、Manager 各页面、开放平台、官网——**全部没有高保真稿或原型**（DESIGN-SYSTEM §5 自己也承认门禁 1–4 都“未开始/未通过”）。
10. **UI 审核**：组件和流程原型评审、截图基线、视觉回归，都没有做。
11. **来源/授权**：Inter / Noto Sans SC / JetBrains Mono 的 OFL 授权仍然标为 [ASSUMPTION]。`docs/audit/license-and-ip.md` 存在，但我没有核对其中是否列出了品牌素材；商标检索未做（PRD OQ-1）。
12. 深浅背景下的标志对比校验报告（brand-guidelines 里已列为未做）。

---

## 总结

1. 标志 SVG 构图正确，但 VG 字形和轨道弧与原图有结构差异，也没有发光效果，**不能直接作为发布用的主标志**。
2. iOS 不透明、Android 安全区、maskable 80%、ICNS/ICO 格式全部通过。
3. 桌面 PNG/ICNS 是满幅方块，不符合 macOS 圆角加留边规范；favicon-16 无法辨认。
4. UI 图标 55 个，参数统一，与 json、sprite 一致，文档引用的图标名全部存在；`moments` 和 `users-admin` 有语义歧义，并缺少 camera、attach、alert、已读双勾等图标。
5. frontmatter 与 tokens.json 完全一致；我重算 34 组对比度，结果都正确。
6. 缺少占位符令牌和气泡内次要文字令牌：text-disabled 只有 2.58–3.0，secondary 放在出方气泡上只有 1.02。border 用了非 WCAG 的 1.2 门槛。
7. 有两处说法与事实不符：“CI 阻断”（仓库里没有 CI），以及“生成脚本校验对比度”（没有这样的脚本）。
8. Emoji lint 按路径匹配，会漏掉 router 里定义的菜单（实测漏报），还会误报 ⏎。
9. 克制的安全/合规措辞和不复用上游样式这两点合格。
10. §7 的大部分交付（高保真稿、组件规格、启动页、通知图标、组合标志、状态集、视觉回归）尚未产出。

VERDICT: conditional

