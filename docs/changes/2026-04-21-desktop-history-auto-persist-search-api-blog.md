# Desktop Phase D：接通 Chat 自动落盘与历史搜索接口（端到端）

## 背景

在前一阶段，desktop 已经能通过 `SendChat` 调用后端 `/chat`，也已经完成了 history 存储层（SQLite schema、CRUD、`LIKE/instr` 搜索能力）。

但当时还缺“最后一公里”：

1. `SendChat` 成功/失败结果没有统一接入 history 自动落盘；
2. history 搜索能力没有通过 Wails App 对前端暴露；
3. desktop 进程生命周期没有管理 SQLite store（启动迁移/退出关闭）。

这次改动把这三段链路打通，满足 D 阶段“每次问答自动落盘并可检索”的端到端要求。

## 改动点拆解

### 1) App 生命周期接入 history store

关键文件：`apps/desktop/app.go`

新增能力：

- 启动阶段初始化本地历史库：
  - 解析历史库路径（环境变量优先）
  - 打开 SQLite store
  - 执行 `schema.sql` 迁移
  - 预创建固定 session
- 退出阶段关闭 store，避免句柄泄漏。

路径策略：

- 环境变量：`DESKTOP_HISTORY_DB_PATH`（优先）
- 默认路径：`~/.persona-agent/desktop/history.sqlite`

涉及函数：

- `openHistoryStore`
- `resolveHistoryDBPath`
- `shutdown` 中 close 逻辑

### 2) SendChat 接入“用户+助手(成功/失败)”自动落盘

关键文件：`apps/desktop/app.go`

`SendChat` 逻辑调整为：

1. 校验输入（空消息直接 `invalid_argument`）；
2. 先落盘用户消息（`PersistUserTurn`）；
3. 调用 chat client；
4. 成功：落盘助手回复（`PersistAssistantTurn`）；
5. 失败：落盘助手错误消息（`PersistAssistantError`，带 `error_code`）。

这使得失败路径也有历史痕迹，不再只记录成功回合。

### 3) 暴露前端可调用搜索接口

关键文件：`apps/desktop/app.go`

新增 Wails 绑定方法：

- `SearchHistory(keyword, limit, offset)`

返回 DTO：`HistorySearchItem`，字段含：

- `message_id`
- `session_id`
- `session_title`
- `role`
- `content`
- `status`
- `error_code`
- `created_at`

前端绑定代码同步生成：

- `apps/desktop/frontend/wailsjs/go/main/App.d.ts`
- `apps/desktop/frontend/wailsjs/go/main/App.js`
- `apps/desktop/frontend/wailsjs/go/models.ts`

### 4) 新增 history 模块实现与测试

关键文件：

- `apps/desktop/backend/history/schema.sql`
- `apps/desktop/backend/history/store.go`
- `apps/desktop/backend/history/search.go`
- `apps/desktop/backend/history/store_test.go`
- `apps/desktop/backend/history/search_test.go`
- `apps/desktop/app_test.go`

能力覆盖：

- schema 初始化 + 索引
- session/message CRUD
- 自动落盘 helper
- 中文关键词检索（`LIKE + instr`）
- app 层成功/失败持久化链路测试
- app 层搜索方法测试

### 5) 依赖更新

关键文件：`apps/desktop/go.mod` / `apps/desktop/go.sum`

新增 SQLite driver：

- `github.com/mattn/go-sqlite3 v1.14.24`

说明：该驱动依赖 CGO，后续 CI/打包环境需保证编译链可用。

## 关键代码路径

- `apps/desktop/app.go`
- `apps/desktop/app_test.go`
- `apps/desktop/backend/history/schema.sql`
- `apps/desktop/backend/history/store.go`
- `apps/desktop/backend/history/search.go`
- `apps/desktop/backend/history/store_test.go`
- `apps/desktop/backend/history/search_test.go`
- `apps/desktop/frontend/wailsjs/go/main/App.d.ts`
- `apps/desktop/frontend/wailsjs/go/main/App.js`
- `apps/desktop/frontend/wailsjs/go/models.ts`
- `apps/desktop/go.mod`
- `apps/desktop/go.sum`

## 行为变化

- desktop 在本地自动记录每次提问与回复；
- 后端错误也会记录为 assistant error 消息；
- 前端可直接调用 `SearchHistory` 获取命中结果（为 E 阶段搜索 UI 做好后端准备）；
- 历史库默认落到用户目录，不写入仓库。

## 测试验证

已执行：

- `go -C apps/desktop test ./...`

结果：通过。

## 潜在问题与后续优化

1. 当前 session 仍是固定 ID（`desktop-default-session`），更适合 MVP；后续 E/F 阶段建议引入多会话切换。
2. 搜索已可调用，但尚未做完整 UI（结果列表、键盘导航、跳转定位）。
3. 使用 `go-sqlite3` 需要 CGO；若后续在部分环境受限，可评估切回纯 Go 驱动并处理镜像源可用性问题。
