# 2026-04-20 Changelog

## Entry: 初始化 desktop 端 Wails + React 脚手架（Phase A）

### Summary（Phase A）

- 新增 `apps/desktop`，完成 Wails + React + TypeScript 的最小可运行桌面工程初始化。
- 新增目录 `apps/desktop/backend/chat` 与 `apps/desktop/backend/history`，并放置占位 Go 文件，建立后续阶段扩展路径。
- 更新根 `.gitignore`，补充 desktop 前端依赖与构建产物忽略规则，避免误提交生成文件。

### Why（Phase A）

- 对齐 `docs/desktop-wails-mvp-plan.md` 的 A 阶段目标，先落地“可启动壳工程 + 目录结构”，再进入窗口行为、chat 接入和历史存储等后续阶段。
- 仓库此前无 `apps/*` 与前端工具链，需先建立统一 desktop 基线，降低后续实现成本。

### Changed Files（Phase A）

- `.gitignore`
  - 新增 desktop 相关忽略项：
    - `apps/desktop/frontend/node_modules/`
    - `apps/desktop/frontend/dist/`
    - `apps/desktop/build/bin/`
- `apps/desktop/wails.json`
  - Wails 工程配置。
- `apps/desktop/main.go`
  - desktop 应用入口。
- `apps/desktop/app.go`
  - 默认绑定 App 结构。
- `apps/desktop/go.mod`
- `apps/desktop/go.sum`
- `apps/desktop/frontend/*`
  - React + Vite + TS 脚手架文件（含 `package.json`、`src/App.tsx`、`src/main.tsx`、`vite.config.ts` 等）。
- `apps/desktop/backend/chat/doc.go`
- `apps/desktop/backend/history/doc.go`

### Validation（Phase A）

- 执行：`go test ./...`
- 结果：通过。
- 执行：`go run github.com/wailsapp/wails/v2/cmd/wails@latest dev`（在 `apps/desktop`）
- 结果：前端依赖安装、前端编译、应用打包均完成，进入 Wails dev 模式。

### Risk / Notes（Phase A）

- 当前仅完成 Phase A 脚手架与目录骨架，尚未接入 `/chat`、快捷键与历史存储逻辑。
- 本次使用 Wails 默认模板，后续阶段需在保持可启动前提下逐步替换为业务实现。

## Entry: 增加全局热键与启动器窗口显隐链路（Phase B 基础）

### Summary（Phase B）

- 在 `apps/desktop/app.go` 增加启动器窗口 show/hide/toggle 能力，并维护窗口可见状态。
- 新增全局热键注册逻辑：优先 `Option+Space`，失败时回退到 `Cmd+Shift+Space`。
- 前端 `apps/desktop/frontend/src/App.tsx` 改为 launcher 输入界面，占位实现输入态/搜索态切换与 Esc 逐级退出行为。
- 更新 Wails 绑定文件与 `main.go` 生命周期钩子，确保热键在退出时释放。

### Why（Phase B）

- Phase A 仅完成壳工程，尚不具备 Alfred 式“全局唤起 + 即用即走”体验。
- 本次先打通窗口行为和快捷键链路，为后续接入 chat/history 能力提供稳定交互基线。

### Changed Files（Phase B）

- `apps/desktop/app.go`
  - 增加 `ShowLauncher` / `HideLauncher` / `ToggleLauncher`。
  - 使用 `golang.design/x/hotkey` 注册全局热键，监听按键后切换窗口显示状态。
  - 启动时设置窗口置顶并默认隐藏；退出时释放热键。
- `apps/desktop/main.go`
  - 新增 `OnShutdown: app.shutdown`，对齐热键清理生命周期。
- `apps/desktop/go.mod`
- `apps/desktop/go.sum`
  - 新增 `golang.design/x/hotkey` 及其依赖。
- `apps/desktop/frontend/src/App.tsx`
  - 监听 `launcher:focus-input` 事件自动聚焦输入框。
  - 新增 Esc 行为：清空输入 → 退出搜索态 → 调用 `HideLauncher`。
- `apps/desktop/frontend/wailsjs/go/main/App.d.ts`
- `apps/desktop/frontend/wailsjs/go/main/App.js`
  - 更新导出绑定，移除 `Greet`，新增 launcher 控制方法。

### Validation（Phase B）

- 本次未新增自动化测试执行记录。
- 代码层面已补齐 shutdown 清理与前后端绑定，建议在 `apps/desktop` 下执行 `wails dev` 手动验证：
  - 全局热键唤起/隐藏
  - 输入框自动聚焦
  - Esc 逐级退出

### Risk / Notes（Phase B）

- `Option+Space` 在部分系统环境可能被输入法占用，当前已实现 `Cmd+Shift+Space` 回退。
- 前端“搜索态”与“输入态”仍为占位，不包含真实 chat/history 数据流。
- 目前通过 runtime 事件驱动聚焦，后续接入复杂 UI 时需留意 focus 抢占时机。

## Entry: 接入 desktop Chat 请求链路与错误重试（Phase C）

### Summary（Phase C）

- 新增 `apps/desktop/backend/chat/client.go`，封装 `POST /chat` 调用并解析统一错误结构（`error.code` / `error.message`）。
- 在 `apps/desktop/app.go` 增加 `SendChat` 绑定方法，desktop 启动时生成并复用 `session_id`，并接入 chat client。
- 前端 `apps/desktop/frontend/src/App.tsx` 增加 `Enter` 发送、loading、回答展示、错误文案映射与 Retry 重发。
- 当未配置 `DESKTOP_CHAT_BASE_URL` 时，后端默认回退到 `http://localhost:8080` 并输出告警日志，避免本地开发阻塞。
- 更新 Wails 绑定导出与模型文件（含 `SendChat` 与 `ChatResult` 类型）。

### Why（Phase C）

- Phase B 已完成窗口与快捷键链路，但尚不能真正提问并拿到后端回复。
- 本次在保持后端 API 零改动的前提下，把 desktop MVP 主路径推进到“输入 -> 请求 -> 返回/报错 -> 可重试”。

### Changed Files（Phase C）

- `apps/desktop/backend/chat/client.go`
  - 新增 desktop 侧 chat HTTP client。
  - 严格复用契约：请求 `session_id/message`，响应 `response`。
  - 归一化错误结构（含状态码、错误码与消息）。
- `apps/desktop/backend/chat/client_test.go`
  - 新增 client 测试，覆盖成功响应、400/422/502/500 映射、配置缺失与输入校验。
- `apps/desktop/app.go`
  - 新增 `chatClient` 与 `sessionID` 状态。
  - `startup` 时生成 session 并初始化 chat client。
  - 新增 `SendChat(message)` 绑定方法给前端调用。
  - 未配置 `DESKTOP_CHAT_BASE_URL` 时回退到 `http://localhost:8080`。
- `apps/desktop/frontend/src/App.tsx`
  - 接入 `SendChat` 调用。
  - 增加 `isLoading` / `answer` / `errorText` / `lastMessage` 状态。
  - 新增 Enter 发送与错误重试按钮。
  - 新增 400/422/502/500 与网络/配置错误的用户文案映射。
- `apps/desktop/frontend/src/App.css`
  - 补充响应区、错误区、重试按钮和输入区样式。
- `apps/desktop/frontend/wailsjs/go/main/App.d.ts`
- `apps/desktop/frontend/wailsjs/go/main/App.js`
- `apps/desktop/frontend/wailsjs/go/models.ts`
  - 更新生成绑定，导出 `SendChat` 与 `ChatResult` / `chat.Error` 类型。
- `apps/desktop/go.mod`
  - 增加 `github.com/google/uuid` 依赖用于 session 生成。

### Validation（Phase C）

- 执行：`cd apps/desktop && go test ./...`
- 结果：通过（`backend/chat` 测试通过）。
- 执行：`cd apps/desktop && npm run build --prefix frontend`
- 结果：通过（前端构建成功）。
- 执行：`cd apps/desktop && go run github.com/wailsapp/wails/v2/cmd/wails@latest dev`
- 结果：启动成功，日志显示：
  - `DESKTOP_CHAT_BASE_URL is not set, fallback to http://localhost:8080`
  - `global hotkey registered: Option+Space`
- 执行：`go test ./...`（仓库根）
- 结果：通过。

### Risk / Notes（Phase C）

- 当前默认回退 `http://localhost:8080` 仅适用于本地开发；联调/部署环境仍建议显式设置 `DESKTOP_CHAT_BASE_URL`。
- `session_id` 仅在本次 desktop 进程内复用，重启后会重建；持久化会话属于后续历史阶段范围。
- `isSearchMode` 仍为占位状态，尚未接入真实历史搜索数据流。

## Entry: 修复输入法回车误触发发送（Phase C 补丁）

### Summary（Phase C Patch）

- 调整 `apps/desktop/frontend/src/App.tsx` 的 Enter 提交逻辑：输入法组合输入（IME composition）期间按回车不再触发发送。
- 增加 `isComposing` 状态，并接入 `onCompositionStart` / `onCompositionEnd`。
- Enter 触发前增加三重保护：`isComposing`、`event.nativeEvent.isComposing`、`keyCode===229`。

### Why（Phase C Patch）

- 中文输入法候选确认阶段会使用回车，若直接按 Enter 发送会造成误触发，影响聊天体验。

### Changed Files（Phase C Patch）

- `apps/desktop/frontend/src/App.tsx`
  - 新增 IME 组合输入状态。
  - 输入法组合期间屏蔽 Enter 发送。

### Validation（Phase C Patch）

- 执行：`cd apps/desktop && npm run build --prefix frontend`
- 结果：通过。

### Risk / Notes（Phase C Patch）

- 该修复仅处理“组合输入中的回车”，不会影响普通英文输入下的 Enter 发送行为。
