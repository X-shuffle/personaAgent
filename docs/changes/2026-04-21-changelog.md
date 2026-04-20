# 2026-04-21 Changelog

## Entry: desktop 固定会话、Markdown 渲染与 IME 回车保护加固

### Summary

- `apps/desktop/app.go` 将 desktop 侧 `session_id` 改为固定常量，避免应用重启后切换新会话。
- `apps/desktop/frontend/src/App.tsx` 引入 `react-markdown` 渲染回复内容，支持标题、列表、代码块等 Markdown 展示。
- `apps/desktop/frontend/src/App.tsx` 调整 Enter 提交判定，组合输入期间同时检查 `isComposing`、`nativeEvent.isComposing` 与 `nativeEvent.keyCode===229`，减少输入法回车误发送。
- `apps/desktop/frontend/src/App.css` 增补 `.response` 下 markdown 元素样式，确保渲染可读性。
- `apps/desktop/frontend/package.json` / `package-lock.json` 增加 `react-markdown@^8.0.7` 依赖。

### Why

- 之前 `session_id` 在启动时动态生成，重启后会丢失会话连续性，不符合“先固定会话”的验证诉求。
- 纯文本渲染会丢失后端返回中的 Markdown 结构，影响回复可读性。
- 中文输入法候选确认回车仍会触发发送，需继续加固 Enter 触发条件。

### Changed Files

- `apps/desktop/app.go`
  - 新增 `fixedSessionID` 常量并在 `startup` 使用固定值。
  - 移除直接使用 `uuid.NewString()` 的会话生成逻辑。
- `apps/desktop/go.mod`
  - `github.com/google/uuid` 从直接依赖变为间接依赖（由依赖图变化导致）。
- `apps/desktop/frontend/src/App.tsx`
  - 新增 `ReactMarkdown` 渲染回答。
  - Enter 判定改为读取 `nativeEvent` 并增加 `keyCode===229` 保护。
- `apps/desktop/frontend/src/App.css`
  - 新增 `.response` 下 `p/ul/ol/pre/code` 等样式。
- `apps/desktop/frontend/package.json`
  - 新增 `react-markdown` 依赖。
- `apps/desktop/frontend/package-lock.json`
  - 锁定 `react-markdown` 及其传递依赖。
- `apps/desktop/frontend/package.json.md5`
  - 同步更新哈希。

### Validation

- 执行：`npm run build --prefix apps/desktop/frontend`
  - 结果：通过。
- 执行：`go test ./...`
  - 结果：通过。

### Risk / Notes

- 固定 `session_id` 是当前阶段的短期策略，不区分用户与场景，后续如需多会话需引入可配置会话管理。
- `keyCode` 在类型层面已标记为 deprecated，但该分支用于兼容部分输入法事件上报差异，当前保留以降低误触发概率。

## Entry: memory 冷启动回补 Qdrant 最近记录并强化 Store 契约

### Summary（memory）

- `internal/memory/service.go` 在向量检索与 short-term 缓存都未命中时，新增从 `Store.RecentBySession` 拉取最近记忆的冷启动兜底。
- `internal/memory/interfaces.go` 将 `RecentBySession` 纳入 `Store` 主接口，改为强约束能力。
- `internal/memory/store_qdrant.go` 新增 `RecentBySession` 实现，通过 `points/scroll` + `session_id` 过滤拉取最近记录，并在本地按时间倒序再排序保证稳定性。
- `internal/memory/service_test.go`、`internal/memory/store_qdrant_test.go`、`internal/ingestion/service_test.go` 补齐接口与行为测试。

### Why（memory）

- 仅依赖进程内 `shortTermBySess` 会在服务重启后丢失短期缓存，导致首次请求拿不到历史记忆。
- 通过 Qdrant 回补最近记录，可以在冷启动时恢复会话连续性，减少“无记忆开场”。
- 将 recent 能力并入 `Store` 主接口，避免可选接口分叉，明确所有存储实现都必须支持该能力。

### Changed Files（memory）

- `internal/memory/interfaces.go`
  - 在 `Store` 中新增 `RecentBySession(ctx, sessionID, limit)` 并补充中文注释。
- `internal/memory/service.go`
  - `Retrieve` 新增冷启动兜底路径：short-term 空时调用 `s.store.RecentBySession`，异常按降级处理。
- `internal/memory/store_qdrant.go`
  - 新增 `RecentBySession`。
  - 抽取 `memoryFromPayload` 复用 Search/Recent 的 payload 映射。
  - 增加中文注释说明 scroll 查询与本地二次排序目的。
- `internal/memory/service_test.go`
  - fakeStore 增加 recent 能力桩。
  - 新增冷启动命中 recent、short-term 优先、recent 失败降级等用例。
- `internal/memory/store_qdrant_test.go`
  - 新增 `RecentBySession` 请求体与结果顺序/映射校验。
- `internal/ingestion/service_test.go`
  - fakeStore 补齐 `RecentBySession`，满足强约束接口。

### Validation（memory）

- 执行：`go test ./internal/memory/...`
  - 结果：通过。
- 执行：`go test ./internal/memory/... ./internal/ingestion/...`
  - 结果：通过。

### Risk / Notes（memory）

- `RecentBySession` 依赖 Qdrant `points/scroll` 与 `order_by` 行为，已增加本地 `timestamp` 倒序排序兜底以降低后端差异风险。
- 强约束接口会要求后续新增 Store 实现必须提供 recent 能力，否则编译期即失败（这是预期约束）。
