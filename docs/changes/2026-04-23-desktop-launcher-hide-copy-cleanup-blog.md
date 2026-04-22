# Desktop 启动器交互收敛：从“误隐藏”到可预期显隐 + 一键复制

## 背景

这轮改动的核心不是加新功能，而是把 desktop 启动器的高频行为变得可预期：

1. 发送消息、滚动消息、切换焦点时，窗口不该“神秘消失”；
2. 点击窗口外应该稳定隐藏；
3. 长代码输出不应撑破消息容器；
4. 助手回复要能直接复制。

在此前迭代中，`auto-hide`、前端焦点事件、历史浏览与消息滚动叠加在一起，造成体感不稳定。本次把这些链路逐一拆开并收敛。

## 改动点拆解

### 1) 显隐链路收敛：移除发送后自动隐藏，保留三条显式路径

关键文件：`apps/desktop/app.go`

- 移除 `DESKTOP_AUTO_HIDE_POLICY` 及其相关实现（`resolveAutoHidePolicy` / `applyAutoHidePolicy`）；
- `SendChat` 不再在成功/失败后触发隐藏；
- 隐藏触发统一为：
  - Esc（输入为空时）；
  - 全局热键 Toggle；
  - 前端失焦（点击窗口外）。

收益：用户能明确知道窗口何时会隐藏，不再受请求成功与否影响。

### 2) 点击窗口外隐藏：改用 blur 兜底跨窗口场景

关键文件：`apps/desktop/frontend/src/App.tsx`

- 保留 `window blur` 监听，在下一帧确认 `document.hasFocus()` 后调用 `HideLauncher()`；
- 解决 WebView 在跨窗口点击时收不到 `pointerdown` 的现实限制。

收益：点击其他应用/桌面空白处时，隐藏行为稳定生效。

### 3) 消息区长行溢出修复

关键文件：`apps/desktop/frontend/src/App.css`

- 对 `.message-content pre` 增加 `max-width: 100%`、`overflow-x: auto`、`white-space: pre-wrap`、`word-break: break-word`；
- `.message-content pre code` 继承换行策略，`.message-content code` 增加 `overflow-wrap: anywhere`。

收益：超长代码行不再越出消息容器，必要时横向滚动，视觉边界稳定。

### 4) 助手消息旁新增复制按钮

关键文件：`apps/desktop/frontend/src/App.tsx`、`apps/desktop/frontend/src/App.css`

- 在助手消息 meta 区域新增复制按钮；
- 通过 `ClipboardSetText` 写入剪贴板；
- 成功后短暂显示 `✓` 反馈。

收益：减少手动选中文本成本，提升回答复用效率。

### 5) 历史 recent 浏览与 UI 清理继续收口

关键文件：
- `apps/desktop/backend/history/search.go`
- `apps/desktop/backend/history/search_test.go`
- `apps/desktop/frontend/src/features/history/useHistorySearch.ts`
- `apps/desktop/frontend/src/features/history/HistorySearchPanel.tsx`
- `apps/desktop/frontend/src/features/history/types.ts`
- `apps/desktop/frontend/src/style.css`

要点：
- 空关键词支持 recent 返回；
- 前端方向键浏览 recent 并联动上下文跳转；
- 删除未使用的字段与样式（如 `HistoryJumpTarget.sessionId`、无效 class 与冗余样式）。

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
- `apps/desktop/frontend/src/features/history/HistorySearchPanel.tsx`
- `apps/desktop/frontend/src/features/history/types.ts`
- `apps/desktop/frontend/src/features/history/useHistorySearch.ts`

## 行为变化

- 发送消息后不再自动隐藏窗口；
- 点击窗口外稳定隐藏；
- 代码块超长行不会越界；
- 助手消息可一键复制；
- 空输入时方向键可直接浏览 recent 历史。

## 测试验证

- `cd apps/desktop && go test ./...` 通过；
- `cd apps/desktop/frontend && npm run build` 通过；
- 手工验证通过：外侧点击隐藏、长行展示、消息复制。

## 潜在问题与后续优化

1. 复制按钮当前使用文本符号（`⧉` / `✓`），后续可替换统一 icon 资产并做 hover/disabled 动效细化。
2. blur 隐藏属于窗口级策略，后续若引入“临时浮层工具窗”需评估是否要做白名单。
3. recent 浏览频率高时可继续优化搜索请求节流与缓存策略。
