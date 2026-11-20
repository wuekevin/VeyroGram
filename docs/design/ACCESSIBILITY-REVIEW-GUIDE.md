# 无障碍复核指南（你可以亲手做的检查）

目标：WCAG 2.1 AA。**自动化脚本只覆盖一部分**，下面这些需要人来做。记录结果请用 `a11y-manual-test-log.md`。

## 0. 先决条件
- 打开原型：`docs/design/prototypes/*.html`（在 Chrome 里直接打开；`?theme=dark` 看暗色，`?ts=2` 看 200% 文字，`login.en.html` 看英文）。
- 手机上看原型：在 Mac 终端执行 `python3 -m http.server 8000 --directory docs/design/prototypes`，手机连同一 Wi-Fi，浏览器打开 `http://<Mac 的局域网 IP>:8000/login.html`。
- 注意：原型用固定像素字号，**不会响应手机系统的“更大字体”**；真实 App 必须用系统字体缩放。200% 文字缩放在原型里用 `?ts=2` 模拟。

## 1. 15 分钟快速复核（Mac + Chrome）
1. **Lighthouse / axe**：Chrome 开发者工具 → Lighthouse → 只勾 Accessibility → 运行；或安装 “axe DevTools” 扩展。预期：我们的脚本已经 0 违规，如果你看到新问题请记录。
2. **只用键盘**：按 `Tab` / `Shift+Tab` 走完一页。检查：每个控件都能到达；焦点环清晰可见（2px 蓝色外框）；顺序符合视觉顺序；按 `Enter`/`Space` 可操作；弹层（miniapp-consent、message-read-detail）里不应能 Tab 到背景内容。
3. **缩放**：`Cmd +` 放大到 200%、400%；检查没有内容被裁切、没有横向滚动。
4. **减少动态效果**：开发者工具 → Rendering → Emulate CSS `prefers-reduced-motion: reduce`，确认无动画。
5. **颜色**：用 “Colour Contrast Analyser” 或 Chrome 取色工具，对任意文字抽查 3–5 处；对比度报告在 `packages/design-tokens/dist/contrast-report.json`。
6. **只看灰度**：开发者工具 → Rendering → Emulate vision deficiencies → Achromatopsia，确认状态不只靠颜色区分。

## 2. 读屏测试（最重要，必须人工）
### macOS VoiceOver
1. `Cmd+F5` 开启。用 `Ctrl+Option+方向键` 逐项阅读；`Ctrl+Option+U` 打开转子，分别看 **标题 / 地标 / 表单控件 / 链接** 列表。
2. 重点页面与预期：
   - `login.html`：读出“登录 VeyroGram”标题；“手机号”“6 位验证码”输入框有名称；“登录”按钮可操作。
   - `chat.html`：消息列表是 log 区域；返回、通话、更多、附件、发送按钮有名称；发出/收到的消息能区分；撤回提示被读出。
   - `group-settings.html` / `settings.html`：每个开关读出名称与“开/关”状态（role=switch）。
   - `miniapp-consent.html`：读出对话框标题；背景不可达。
3. 记录：读错、漏读、顺序错、没名称的元素。

### iPhone VoiceOver / Android TalkBack
1. iOS：设置 → 辅助功能 → VoiceOver。Android：设置 → 无障碍 → TalkBack。用手机打开上面的局域网地址。
2. 单指滑动逐项朗读；双击激活。完成：登录、发送消息（在 chat 页点发送）、切换群开关。
3. 说明：这是对**原型**的检查；真实 App 上线前必须用真实构建重做。

## 3. 屏幕与设备
- 手机竖屏/横屏、平板；放大“更大字体”（真实 App）；开启“增加对比度/粗体文本”（真实 App）。
- Windows 高对比度模式（Web/桌面端）。

## 4. 文本间距测试（1.4.12，Web/后台）
在页面上注入下面样式，确认没有内容被裁切或重叠：`* { line-height:1.5 !important; letter-spacing:.12em !important; word-spacing:.16em !important } p { margin-bottom:2em !important }`。

## 5. 由谁来做
- 你自己可以做 §1（无需专业知识）。
- §2 读屏测试建议由**有读屏使用经验的人**做（最好是视障用户）；正式发布前建议委托**第三方无障碍审计**并出具 ACR/VPAT。
- 我（AI）做不到的：用真实读屏/真机验证、判断“听起来是否合理”、真人可用性测试。

## 6. 记录与签字
把结果填进 `a11y-manual-test-log.md`；全部 MANUAL 条目通过后，在 `APPROVALS.md` 的 G5 签字。**在产品实现完成并通过各端清单前，不得宣称“符合 WCAG 2.1 AA”。**
