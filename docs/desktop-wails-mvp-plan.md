# Desktop Quick Launcher MVP Plan (Wails + React)

## Context

目标是在当前 Go 后端基础上增加一个“即用即走”的桌面前端体验（类似 Alfred）：
- 全局快捷键呼出
- 快速输入问题
- 快速返回结果
- 随时隐藏

已确认约束：
- 技术路线：**Wails + React（Go）**
- 平台：**macOS 优先**，后续补 Windows
- 后端：**保持不变**（继续复用现有 `/chat`）
- 历史：前端本地持久化并支持查询

## Recommended Approach

### 1) 在仓库内新增独立桌面端

新增目录：`apps/desktop`

- `apps/desktop/frontend`：React + Vite UI
- `apps/desktop/backend`：Wails Go 逻辑（窗口、快捷键、历史存储、chat 调用）

### 2) 严格复用现有后端 API（零改动）

复用契约：
- 请求：`{ session_id, message }`
- 响应：`{ response }`
- 错误：按现有 400/422/502/500 映射为前端提示

参考实现：
- `internal/model/types.go`
- `internal/api/chat_handler.go`
- `cmd/server/main.go`

### 3) MVP 交互规格（键盘优先）

- 全局快捷键呼出（默认 `Option+Space`，冲突时降级 `Cmd+Shift+Space`）
- 单窗口、无边框、置顶、居中，呼出即聚焦输入框
- `Enter` 发送，展示结果
- `Esc` 逐级：清空输入 -> 退出搜索 -> 隐藏窗口
- 单页结构（聊天 + 历史搜索切换）

### 4) 本地历史存储与查询（前端侧）

SQLite（Go 侧）存储：
- `sessions`：`id/title/created_at/updated_at`
- `messages`：`id/session_id/role/content/status/created_at/error_code`

查询策略：
- MVP：`LIKE/instr`（中文友好）
- 后续可升级 FTS

MVP 能力：
- 自动保存每次问答
- 关键词搜索历史
- 从搜索结果回跳到对应会话位置

## Implementation Phases

1. Scaffold：初始化 `apps/desktop`（Wails + React + TS）
2. Window/Hotkey：完成呼出/隐藏/焦点与置顶行为
3. Chat Integration：接入 `/chat`，补齐 loading/error
4. Local History：SQLite 持久化 + 搜索 UI + 跳转
5. Polish：键盘细节、空态、失败重试、自动隐藏策略
6. Docs：更新根 README/CONTRIBUTING（桌面端运行说明）

## Planned Files

### New
- `apps/desktop/wails.json`
- `apps/desktop/main.go`
- `apps/desktop/backend/app.go`
- `apps/desktop/backend/chat/client.go`
- `apps/desktop/backend/history/{store.go,schema.sql,search.go}`
- `apps/desktop/frontend/package.json`
- `apps/desktop/frontend/src/{App.tsx,main.tsx}`
- `apps/desktop/frontend/src/features/chat/*`
- `apps/desktop/frontend/src/features/history/*`
- `apps/desktop/frontend/src/styles.css`

### Update
- `README.md`
- `CONTRIBUTING.md`
- `.gitignore`

## Verification

### Manual
- 快捷键稳定呼出/隐藏
- 输入问题可返回结果
- 后端不可用时有清晰错误与重试
- 重启后历史仍可查
- 搜索可命中并跳转
- 全流程可只用键盘

### Automated
- Go：history store 单测（CRUD + search）
- 前端：关键状态与键盘交互单测
- 回归：`go test ./...`

## Risks & Mitigation

- 快捷键冲突：fallback 快捷键，后续支持自定义
- 中文检索效果：MVP 用 `LIKE/instr`，后续按规模升级 FTS
- 后端可用性依赖：提供状态提示、重试能力
- 窗口焦点边界：show/hide/focus 逻辑收敛到 Wails App 层并专项手测

## TODO Checklist（可执行实现清单）

> 用法建议：每完成一项就打勾，并在子项里补充实际文件路径与验收结果。

### A. 脚手架与工程结构

- [ ] 初始化 `apps/desktop`（Wails + React + TS）
  - 产出文件：`apps/desktop/wails.json`、`apps/desktop/main.go`、`apps/desktop/frontend/*`
  - DoD：`wails dev` 可正常启动桌面窗口
- [ ] 补齐桌面端基础目录
  - 产出目录：`apps/desktop/backend/chat`、`apps/desktop/backend/history`
  - DoD：目录结构与本计划一致，可进入下一阶段开发

### B. 窗口与快捷键（即用即走核心）

- [ ] 实现窗口 show/hide/focus 统一控制
  - 产出文件：`apps/desktop/backend/app.go`
  - DoD：可稳定呼出、隐藏，呼出后输入框自动聚焦
- [ ] 实现全局快捷键 + fallback
  - 默认：`Option+Space`
  - fallback：`Cmd+Shift+Space`
  - DoD：快捷键冲突时可自动降级并可用
- [ ] 实现 Esc 逐级行为
  - 清空输入 -> 退出搜索 -> 隐藏窗口
  - DoD：行为顺序稳定，符合预期

### C. Chat 接入（后端零改动）

- [ ] 封装 `/chat` HTTP 客户端
  - 产出文件：`apps/desktop/backend/chat/client.go`
  - DoD：请求/响应结构严格匹配现有后端契约
- [ ] 前端接入提问流程
  - 产出文件：`apps/desktop/frontend/src/features/chat/*`
  - DoD：`Enter` 可发送并展示结果
- [ ] 错误态映射
  - 覆盖：400/422/502/500
  - DoD：用户可看到可理解错误信息与重试入口

### D. 本地历史存储

- [ ] 设计并落地 SQLite schema
  - 产出文件：`apps/desktop/backend/history/schema.sql`
  - DoD：含 `sessions`、`messages` 与必要索引
- [ ] 实现历史存取（CRUD）
  - 产出文件：`apps/desktop/backend/history/store.go`
  - DoD：每次问答可自动落盘并读取
- [ ] 实现历史查询
  - 产出文件：`apps/desktop/backend/history/search.go`
  - 策略：`LIKE/instr`
  - DoD：可按关键词命中中文历史消息

### E. 历史搜索 UI 与跳转

- [ ] 增加历史搜索面板
  - 产出文件：`apps/desktop/frontend/src/features/history/*`
  - DoD：可输入关键词、显示匹配列表
- [ ] 增加“命中项 -> 会话位置”跳转
  - DoD：选中结果后定位到对应会话与消息
- [ ] 键盘导航搜索结果
  - 键位：`↑/↓` 选择，`Enter` 打开
  - DoD：无需鼠标即可完成查询与跳转

### F. UI 打磨与体验

- [ ] 单窗口视觉与布局优化（无边框、置顶、居中）
  - 产出文件：`apps/desktop/frontend/src/styles.css`
  - DoD：视觉简洁，符合“命令面板”体验
- [ ] 状态反馈完善
  - 场景：loading、空状态、网络异常
  - DoD：用户总能理解当前状态
- [ ] 自动隐藏策略（可配置）
  - DoD：提交后可按策略自动隐藏，且可关闭

### G. 文档与仓库集成

- [ ] 更新 `README.md`
  - 内容：桌面端启动与调试说明
  - DoD：新同学可按文档跑起 desktop
- [ ] 更新 `CONTRIBUTING.md`
  - 内容：desktop 开发与测试流程
  - DoD：贡献流程清晰
- [ ] 更新 `.gitignore`
  - 内容：Wails/Node 构建产物
  - DoD：无不必要产物被跟踪

### H. 测试与验收

- [ ] Go 单测：history store（CRUD + search）
  - DoD：关键路径通过
- [ ] 前端单测：状态管理与键盘交互
  - DoD：核心交互通过
- [ ] 回归测试：`go test ./...`
  - DoD：现有后端能力不受影响
- [ ] 手工验收清单逐条通过
  - 快捷键呼出/隐藏
  - 提问返回
  - 后端不可用提示与重试
  - 重启后历史可查
  - 历史搜索与跳转
  - 全流程键盘可操作

## Status

本文件仅为已确认实施方案存档，当前未开始编码实现。
