# Desktop MVP Phase A 落地详解：先把 Wails 壳工程跑起来

## 背景

本次改动目标是落实 `docs/desktop-wails-mvp-plan.md` 的 A 阶段：

1. 初始化 `apps/desktop`（Wails + React + TS）
2. 建立 `apps/desktop/backend/chat`、`apps/desktop/backend/history` 基础目录
3. 达到 `wails dev` 可启动的验收线

仓库当前主线是 Go 后端（`cmd/server` + `internal/*`），此前没有 `apps/*` 与前端工程基线，因此先落地“可运行骨架”是后续迭代前提。

## 改动点拆解

### 1) 新增 desktop 工程骨架

关键路径：

- `apps/desktop/wails.json`
- `apps/desktop/main.go`
- `apps/desktop/app.go`
- `apps/desktop/go.mod`
- `apps/desktop/go.sum`

这部分由 Wails React+TS 模板初始化得到，作用是先保证桌面端有稳定入口与可开发态运行能力。

### 2) 新增前端模板工程（React + Vite + TS）

关键路径：

- `apps/desktop/frontend/package.json`
- `apps/desktop/frontend/index.html`
- `apps/desktop/frontend/src/main.tsx`
- `apps/desktop/frontend/src/App.tsx`
- `apps/desktop/frontend/vite.config.ts`
- `apps/desktop/frontend/tsconfig.json`

本阶段不做业务 UI 改造，只保留可运行模板，确保后续 chat/history 功能可以在该壳工程中增量实现。

### 3) 建立 backend 业务扩展目录

关键路径：

- `apps/desktop/backend/chat/doc.go`
- `apps/desktop/backend/history/doc.go`

通过占位包先把目录结构固定下来，后续阶段可直接在对应目录补 `client.go`、`store.go`、`search.go` 等实现，不需要再改目录设计。

### 4) 更新忽略规则，控制提交边界

关键路径：

- `.gitignore`

新增：

- `apps/desktop/frontend/node_modules/`
- `apps/desktop/frontend/dist/`
- `apps/desktop/build/bin/`

目的：避免把依赖目录和构建产物带进版本库，只保留源码与必要模板文件。

## 关键代码路径

- 配置与入口
  - `.gitignore`
  - `apps/desktop/wails.json`
  - `apps/desktop/main.go`
  - `apps/desktop/app.go`
- 前端脚手架
  - `apps/desktop/frontend/package.json`
  - `apps/desktop/frontend/src/App.tsx`
  - `apps/desktop/frontend/src/main.tsx`
- 后续能力预留目录
  - `apps/desktop/backend/chat/doc.go`
  - `apps/desktop/backend/history/doc.go`

## 行为变化

- 仓库从“纯后端服务”扩展为“后端 + desktop 子应用”的形态。
- 新开发者可以在 `apps/desktop` 直接进入 Wails 开发流程，而无需先自行搭建前端与桌面工程。
- Phase A 范围内尚未变更后端 API 行为，`/chat` 与 `/ingest` 路由逻辑保持不变。

## 测试验证

已执行并通过：

- `go test ./...`

已执行并观察结果：

- 在 `apps/desktop` 运行：
  - `go run github.com/wailsapp/wails/v2/cmd/wails@latest dev`
- 结果：
  - 依赖安装完成
  - 前端编译完成
  - Go 应用打包完成
  - 进入 Wails dev 模式（包含 dev server URL 输出）

## 潜在问题与后续优化

1. 当前为模板壳工程，尚未接入既有 `/chat` 契约（`internal/model/types.go`、`internal/api/chat_handler.go` 仅在后续阶段复用）。
2. 全局快捷键、窗口 show/hide/focus 逻辑尚未实现，属于 B 阶段范围。
3. 历史存储（SQLite）与历史查询 UI 尚未实现，属于 D/E 阶段范围。

下一步建议直接进入 B 阶段，优先完成窗口行为与快捷键链路，确保“即用即走”的核心体验先跑通。
