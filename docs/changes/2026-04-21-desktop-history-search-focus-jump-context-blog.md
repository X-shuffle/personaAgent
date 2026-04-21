# Desktop 历史搜索：输入即搜、命中即聚焦、上下文直达

## 背景

desktop 已经具备历史落盘与搜索接口能力，但交互上仍有三类问题：

1. 需要手动切换“输入态/搜索态”，打断连续输入；
2. 命中历史后与当前消息流混排，注意力被分散；
3. 首次点击命中偶发不生效，需要重复点击。

本次改动目标是把搜索与定位变成“低阻力 + 高确定性”的单路径体验。

## 改动点拆解

### 1) 输入即搜，去掉搜索态切换

关键文件：`apps/desktop/frontend/src/App.tsx`

- 移除 `isSearchMode` 相关状态与按钮。
- 输入框文本变更后，直接触发历史搜索（保留 debounce）。
- `Enter` 语义统一为“发送聊天请求”，不再承担搜索跳转。

对应历史搜索模块：

- `apps/desktop/frontend/src/features/history/useHistorySearch.ts`
- `apps/desktop/frontend/src/features/history/HistorySearchPanel.tsx`

### 2) 命中后加载上下文并聚焦展示

关键文件：

- `apps/desktop/app.go`
- `apps/desktop/backend/history/store.go`
- `apps/desktop/frontend/src/features/history/api.ts`
- `apps/desktop/frontend/src/App.tsx`

新增后端方法：

- `LoadMessageContext(messageID)`

返回策略：

- 命中 user：返回 `[user, next assistant]`（若存在）；
- 命中 assistant：返回 `[prev user, assistant]`（若存在）；
- 无相邻消息则只返回命中本身。

前端行为：

- 选择命中后调用 `loadMessageContext(messageId)`；
- 消息区直接替换为该命中的上下文（聚焦视图），不与当前聊天流混排；
- 定位到命中消息并短暂高亮。

### 3) 修复“首次点击偶发不生效”

关键文件：`apps/desktop/frontend/src/App.tsx`

- 去掉 `jumpTarget + useEffect` 的间接触发链路；
- 改为点击后直接异步加载上下文；
- 增加 `jumpRequestSeqRef` 请求序号保护，只应用最后一次点击结果，避免并发请求覆盖。

### 4) 新增前端历史模块

新增文件：

- `apps/desktop/frontend/src/features/history/types.ts`
- `apps/desktop/frontend/src/features/history/api.ts`
- `apps/desktop/frontend/src/features/history/useHistorySearch.ts`
- `apps/desktop/frontend/src/features/history/HistorySearchPanel.tsx`

职责拆分：

- `types.ts`：前端历史 DTO；
- `api.ts`：`SearchHistory` / `LoadMessageContext` 映射；
- `useHistorySearch.ts`：自动搜索、键盘上下选择、请求时序防抖；
- `HistorySearchPanel.tsx`：结果面板渲染与点击回调。

## 关键代码路径

- `apps/desktop/app.go`
- `apps/desktop/backend/history/store.go`
- `apps/desktop/frontend/src/App.tsx`
- `apps/desktop/frontend/src/App.css`
- `apps/desktop/frontend/src/features/history/types.ts`
- `apps/desktop/frontend/src/features/history/api.ts`
- `apps/desktop/frontend/src/features/history/useHistorySearch.ts`
- `apps/desktop/frontend/src/features/history/HistorySearchPanel.tsx`
- `apps/desktop/frontend/wailsjs/go/main/App.d.ts`
- `apps/desktop/frontend/wailsjs/go/main/App.js`

## 行为变化

- 输入框输入内容会自动搜索历史；
- `↑/↓` 选择历史结果，点击结果后直接聚焦到命中上下文；
- `Enter` 恒定发送消息；
- 命中定位更稳定，不需要重复点击；
- 界面从“混排展示”改为“命中优先的聚焦展示”。

## 测试验证

已执行：

- `npm --prefix apps/desktop/frontend run build`
- `go -C apps/desktop test ./...`
- `go test ./...`

结果：均通过。

## 潜在问题与后续优化

1. 当前聚焦展示会覆盖消息区内容；若需要回看刚才聊天内容，可增加“返回当前会话视图”入口。
2. `LoadMessageContext` 目前是单跳相邻 Q/A，不是多轮窗口；后续可按需要扩展上下文窗口大小。
3. 自动搜索与发送共用同一输入框，后续如搜索量增长可考虑增加最小触发长度与更细粒度节流策略。