# Desktop 菜单栏入口与 Alfred 风格交互收敛（phase-e polish）

## 背景

desktop 启动器已具备聊天与历史能力，但在“随时唤起 + 发送后收起 + 无输入浏览最近记录”这三条高频路径上仍有割裂：

1. 入口依赖全局热键，缺少菜单栏常驻入口；
2. 发送后隐藏行为不可配置；
3. 空输入无法直接查看 recent，历史浏览路径不顺畅。

本次改动聚焦 macOS 使用习惯与 Alfred 体验，补齐入口与交互闭环。

## 改动点拆解

### 1) 新增 macOS 菜单栏入口与右键菜单

关键文件：`apps/desktop/statusbar_darwin.go`、`apps/desktop/statusbar_stub.go`

- 在 darwin 下通过 Cocoa status item 创建菜单栏图标；
- 左键触发启动器显隐，右键弹出菜单（`Toggle Launcher` / `Quit`）；
- 非 darwin 提供同名空实现，保证其他平台可正常编译。

同时在 `apps/desktop/app.go` 启动与退出生命周期中接入：

- `startup` 调用 `startStatusBar()`；
- `shutdown` 调用 `stopStatusBar()`。

### 2) 启动器位置改为顶部居中并贴合当前屏幕

关键文件：`apps/desktop/app.go`

- 新增 `positionLauncherTopCenterLocked()`：先中心定位，再基于当前屏幕可见区域回推顶部位置；
- 使用 `launcherTopOffsetPx = 72` 控制与状态栏的视觉间距；
- 通过边界裁剪保证窗口不会超出可见区域（含多屏场景兜底）。

### 3) 增加发送后自动隐藏策略

关键文件：`apps/desktop/app.go`

- 新增环境变量：`DESKTOP_AUTO_HIDE_POLICY`；
- 支持策略值：`never`、`on_success`、`always`；
- 默认策略回退为 `on_success`；
- `SendChat` 在成功/失败后分别调用 `applyAutoHidePolicy(true|false)` 执行隐藏决策。

这样可以按场景切换“连续输入”或“即用即走”模式。

### 4) 历史搜索支持 recent 浏览（空关键词）

关键文件：`apps/desktop/backend/history/search.go`、`apps/desktop/backend/history/search_test.go`

- `SearchMessages` 在关键词为空时不再返回空列表；
- 改为直接按时间倒序返回最近消息，支持分页；
- 测试从“空关键词返回空”更新为“返回 recent 且顺序正确”。

这为前端“无输入浏览最近对话”提供后端能力。

### 5) 前端历史交互改为“方向键触发 recent + 自动上下文跳转”

关键文件：

- `apps/desktop/frontend/src/features/history/useHistorySearch.ts`
- `apps/desktop/frontend/src/features/history/HistorySearchPanel.tsx`
- `apps/desktop/frontend/src/App.tsx`

具体行为：

- 输入为空时按 `↑/↓`，进入 recent 浏览模式并发起搜索；
- 高亮项变化时自动加载命中上下文（通过既有 `onJump`）；
- 历史列表高亮项自动滚动到可见区域；
- `App.tsx` 增加窗口 `blur` 自动隐藏，强化 Alfred 式交互。

### 6) UI 样式与网络超时参数收敛

关键文件：

- `apps/desktop/frontend/src/App.css`
- `apps/desktop/frontend/src/style.css`
- `apps/desktop/backend/chat/client.go`
- `apps/desktop/main.go`

- 样式调整为更紧凑的深色浮层、细滚动条与消息紧凑态；
- `main.go` 窗口改为 `Frameless`，尺寸调整为 `820x620`；
- chat client 默认超时从 `15s` 提升到 `30s`，减少慢请求误失败。

## 关键代码路径

- `apps/desktop/app.go`
- `apps/desktop/main.go`
- `apps/desktop/statusbar_darwin.go`
- `apps/desktop/statusbar_stub.go`
- `apps/desktop/backend/chat/client.go`
- `apps/desktop/backend/history/search.go`
- `apps/desktop/backend/history/search_test.go`
- `apps/desktop/frontend/src/App.tsx`
- `apps/desktop/frontend/src/App.css`
- `apps/desktop/frontend/src/style.css`
- `apps/desktop/frontend/src/features/history/useHistorySearch.ts`
- `apps/desktop/frontend/src/features/history/HistorySearchPanel.tsx`

## 行为变化

- 应用启动后可通过菜单栏图标打开/隐藏启动器；
- 发送后窗口是否自动隐藏由策略控制：
  - `never`：不自动隐藏；
  - `on_success`：仅成功后隐藏（默认）；
  - `always`：无论成功失败都隐藏；
- 输入为空时，按方向键可直接浏览 recent；
- 历史高亮项切换时列表视口自动跟随；
- 窗口失焦自动隐藏，更贴近 Alfred 的“即用即走”。

## 测试验证

- `apps/desktop/backend/history/search_test.go` 更新覆盖：
  - 空关键词 recent 返回；
  - LIKE 转义；
  - 分页行为。
- 其余端到端构建/运行验证本次未在提交前新增命令执行记录。

## 潜在问题与后续优化

1. 当前状态栏按钮图标为代码绘制，后续可替换为统一品牌资产并适配深浅色细节。
2. recent 模式在消息量大时会增加查询频率，可按实际数据规模加入缓存层或更细粒度节流。
3. 自动隐藏策略目前通过环境变量配置，后续可在前端设置面板中提供可视化切换。
