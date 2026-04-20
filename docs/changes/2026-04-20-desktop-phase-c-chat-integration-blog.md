# Desktop MVP Phase C 详解：从启动器壳到可真实提问

## 背景

Phase B 已经把 desktop 启动器的窗口显隐、全局热键和输入聚焦打通，但还停留在 UI 占位阶段，无法真正请求后端。

Phase C 的目标是把主链路补齐：

- 输入问题
- 调用后端 `/chat`
- 展示回复
- 失败可重试

并保证和现有服务契约保持一致，不改服务端实现。

## 改动点拆解

### 1) 新增 desktop 侧 `/chat` HTTP client

关键文件：`apps/desktop/backend/chat/client.go`

实现点：

- 请求体严格使用 `{ session_id, message }`
- 成功响应严格读取 `{ response }`
- 非 200 响应解析 `error.code` / `error.message`
- 将错误统一为 desktop 内部 `chat.Error`（含 `status_code/code/message`）

这样前端不用直接处理 HTTP 细节，只关心“成功结果”或“结构化错误”。

### 2) App 层增加 SendChat 绑定与 session 复用

关键文件：`apps/desktop/app.go`

实现点：

- `startup` 时生成一次 `session_id`（`uuid.NewString()`），进程内复用
- 初始化 `chatClient` 并提供 `SendChat(message)` 供前端调用
- 未配置 `DESKTOP_CHAT_BASE_URL` 时自动回退到 `http://localhost:8080`

这让 Phase C 在本地开发环境可直接开箱运行，同时保留环境变量覆盖能力。

### 3) 前端接入 Enter 发送、loading、错误与重试

关键文件：`apps/desktop/frontend/src/App.tsx`

实现点：

- Enter 触发 `SendChat`
- 发送中状态禁用重复提交
- 成功显示 `response`
- 失败显示可理解错误文案并提供 Retry
- 记录 `lastMessage`，Retry 重发上一条

错误映射逻辑：

- 400：请求格式异常
- 422：输入不合法
- 502：模型服务暂不可用
- 500：服务内部异常
- 网络/配置错误：提示检查后端服务与地址

### 4) 输入法回车误触发补丁

关键文件：`apps/desktop/frontend/src/App.tsx`

问题：中文输入法候选确认时会按回车，之前会被误判为“发送”。

修复：

- 增加 `isComposing`
- 接入 `onCompositionStart/onCompositionEnd`
- Enter 前拦截组合输入态（`isComposing` / `nativeEvent.isComposing` / `keyCode===229`）

效果：组合输入期间回车只用于上屏，不触发发送。

## 关键代码路径

- `apps/desktop/backend/chat/client.go`
- `apps/desktop/backend/chat/client_test.go`
- `apps/desktop/app.go`
- `apps/desktop/frontend/src/App.tsx`
- `apps/desktop/frontend/src/App.css`
- `apps/desktop/frontend/wailsjs/go/main/App.d.ts`
- `apps/desktop/frontend/wailsjs/go/main/App.js`
- `apps/desktop/frontend/wailsjs/go/models.ts`

## 行为变化

- desktop 已从“仅启动器壳”升级到“可实际 chat 请求与返回展示”。
- 默认支持本地开发地址回退（`http://localhost:8080`）。
- 输入法组合输入回车不再误触发发送。

## 测试验证

已执行：

- `cd apps/desktop && go test ./...`
- `cd apps/desktop && npm run build --prefix frontend`
- `cd apps/desktop && go run github.com/wailsapp/wails/v2/cmd/wails@latest dev`
- `go test ./...`

验证结果：全部通过；`wails dev` 启动日志可见热键注册与地址回退提示。

## 潜在问题与后续优化

1. 地址回退仅适合本地开发，联调/生产仍建议显式设置 `DESKTOP_CHAT_BASE_URL`。
2. `session_id` 当前是“进程内复用”，重启后重建；会话持久化将在后续历史阶段处理。
3. 当前 `isSearchMode` 仍是占位态，后续 D/E 阶段再接真实历史能力。