# Desktop 启动器尺寸自适配与文档收口：让大屏不再“显小”

## 背景

这次改动聚焦两个问题：

1. 启动器在大屏上看起来偏小，窗口尺寸“像固定值”；
2. desktop 开发路径分散在对话上下文，仓库文档不完整。

目标很明确：窗口尺寸按屏幕自适配，并把 desktop 的启动/调试/构建流程固化到仓库文档。

## 改动点拆解

### 1) 启动器尺寸：从固定观感到“比例 + 上下限”

关键文件：`apps/desktop/app.go`

- 新增尺寸参数：
  - `launcherWidthRatio = 0.35`
  - `launcherHeightRatio = 0.7`
  - `launcherMinWidth = 760`
  - `launcherMaxWidth = 1160`
  - `launcherMinHeight = 420`
  - `launcherMaxHeight = 860`
- `positionLauncherTopCenterLocked` 中先读取当前屏幕尺寸，再按比例计算目标宽高，并用 `clampInt` 做边界裁剪。

效果：大屏会比之前更自然地放大，小屏仍受最小值保护，不会出现“过窄/过矮”不可用状态。

### 2) 顶部定位：从固定像素偏移改为比例偏移

关键文件：`apps/desktop/app.go`

- 新增顶部偏移参数：
  - `launcherTopOffsetRatio = 0.07`
  - `launcherMinTopOffsetPx = 56`
  - `launcherMaxTopOffsetPx = 96`
- 通过当前屏幕高度计算偏移，再做 min/max 裁剪。

效果：不同分辨率与缩放下，窗口都能保持“顶部居中且不过分贴边/下沉”的一致体感。

### 3) UI 微调：继续贴近 Alfred 的低噪音风格

关键文件：`apps/desktop/frontend/src/App.css`

- `#app` 背景改为更克制的深色纯底，增加轻边框与柔和阴影；
- 输入区提高主视觉权重（更高输入高度与字体）；
- 历史面板、消息卡片的边框/间距/阴影继续统一。

效果：视觉层级更稳，输入焦点更明确，符合“即用即走”的 launcher 心智。

### 4) 文档与仓库集成收口

关键文件：

- `README.md`
- `CONTRIBUTING.md`
- `.gitignore`

具体内容：

- `README.md` 新增 desktop 开发、前端单独调试、构建说明；
- `CONTRIBUTING.md` 增加 desktop 启动流程与 desktop 变更后的前端 build 校验步骤；
- `.gitignore` 增加：
  - `apps/desktop/frontend/.vite/`
  - `apps/desktop/build/windows/nsis/`

效果：新同学可直接按仓库文档跑通 desktop，本地构建产物不会误入版本库。

## 关键代码路径

- `.gitignore`
- `CONTRIBUTING.md`
- `README.md`
- `apps/desktop/app.go`
- `apps/desktop/frontend/src/App.css`

## 行为变化

- launcher 尺寸不再是“固定观感”，会随当前屏幕尺寸自适应；
- 大屏场景下窗口更宽更高（受 `1160/860` 上限约束）；
- 顶部偏移在不同屏幕上更一致；
- desktop 开发/调试/构建流程在仓库文档中可直接查阅。

## 测试验证

- 代码逻辑复核：`currentScreenSizeLocked`、`screenSize`、`clampInt` 的尺寸计算与兜底链路完整；
- 文档复核：README/CONTRIBUTING 中 desktop 指令路径和命令可直接执行；
- 忽略规则复核：`.vite` 与 `build/windows/nsis` 已加入 `.gitignore`。

## 潜在问题与后续优化

1. `0.35/0.7` 为经验参数，后续可基于真实设备（超宽屏、4K 缩放、外接小屏）继续微调。
2. 若未来引入更多面板元素（如多会话列表），可能需要重新评估 `launcherMaxWidth`（例如提升到 `1240`）。
3. 可补一组窗口尺寸计算的单测，降低后续回归风险。
