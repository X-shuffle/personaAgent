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

## Entry: desktop phase-d 历史自动落盘与搜索接口打通

### Summary（desktop phase-d）

- `apps/desktop/app.go` 在 desktop 启动阶段接入 history store 初始化（打开 SQLite、执行 schema 迁移、预创建固定 session），在退出阶段关闭 store。
- `apps/desktop/app.go` 将 `SendChat` 改为完整落盘链路：发送前持久化 user 消息，成功后持久化 assistant 消息，失败时持久化 assistant error（含 `error_code`）。
- `apps/desktop/app.go` 新增 Wails 暴露方法 `SearchHistory(keyword, limit, offset)`，把 history 检索能力提供给前端调用。
- `apps/desktop/backend/history/{schema.sql,store.go,search.go}` 与对应测试文件落地，覆盖 schema/CRUD/中文关键词检索（`LIKE + instr`）。
- `apps/desktop/frontend/wailsjs/go/main/App.{d.ts,js}` 与 `apps/desktop/frontend/wailsjs/go/models.ts` 同步生成 `SearchHistory` 绑定。
- `apps/desktop/go.mod` / `apps/desktop/go.sum` 新增 SQLite driver 依赖 `github.com/mattn/go-sqlite3`。

### Why（desktop phase-d）

- 之前 desktop 能调用 `/chat`，但没有把对话结果可靠写入本地历史，导致“可聊天但不可追溯/检索”。
- 搜索能力虽在 history 包内实现，但未暴露到 Wails App 层，前端无法联调调用。
- 需要先打通 D 阶段后端链路，再进入 E 阶段搜索 UI（面板、键盘导航、跳转定位）。

### Changed Files

- `apps/desktop/app.go`
  - 新增 history store 生命周期管理、`SearchHistory` 方法、`SendChat` 自动落盘与失败落盘。
  - 新增历史库路径解析：环境变量 `DESKTOP_HISTORY_DB_PATH` 优先，默认 `~/.persona-agent/desktop/history.sqlite`。
- `apps/desktop/app_test.go`
  - 新增 app 层端到端测试：成功落盘、失败落盘、搜索接口返回。
- `apps/desktop/backend/history/schema.sql`
  - 新建 `sessions/messages` 表与索引。
- `apps/desktop/backend/history/store.go`
  - 实现 SQLite 打开、迁移、会话/消息 CRUD、自动落盘 helper。
- `apps/desktop/backend/history/search.go`
  - 实现参数化 `LIKE + instr` 搜索。
- `apps/desktop/backend/history/store_test.go`
  - 覆盖迁移、CRUD、自动落盘、排序行为。
- `apps/desktop/backend/history/search_test.go`
  - 覆盖中文关键词命中、转义、分页。
- `apps/desktop/frontend/wailsjs/go/main/App.d.ts`
- `apps/desktop/frontend/wailsjs/go/main/App.js`
- `apps/desktop/frontend/wailsjs/go/models.ts`
  - 同步 Wails 绑定模型与方法定义。
- `apps/desktop/go.mod`
- `apps/desktop/go.sum`
  - 更新 Go 依赖。

### Validation

- 执行：`go -C apps/desktop test ./...`
  - 结果：通过。

### Risk / Notes

- 当前仍使用固定 `session_id`（`desktop-default-session`），后续如进入多会话能力需拆分。
- 本次仅打通 D 阶段调用链，E 阶段 UI（历史搜索面板、↑/↓、Enter 跳转）尚未实现。
- `github.com/mattn/go-sqlite3` 依赖 CGO，CI/打包环境需确保编译链可用。
