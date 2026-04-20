# 2026-04-20 Changelog

## Entry: 初始化 desktop 端 Wails + React 脚手架（Phase A）

### Summary

- 新增 `apps/desktop`，完成 Wails + React + TypeScript 的最小可运行桌面工程初始化。
- 新增目录 `apps/desktop/backend/chat` 与 `apps/desktop/backend/history`，并放置占位 Go 文件，建立后续阶段扩展路径。
- 更新根 `.gitignore`，补充 desktop 前端依赖与构建产物忽略规则，避免误提交生成文件。

### Why

- 对齐 `docs/desktop-wails-mvp-plan.md` 的 A 阶段目标，先落地“可启动壳工程 + 目录结构”，再进入窗口行为、chat 接入和历史存储等后续阶段。
- 仓库此前无 `apps/*` 与前端工具链，需先建立统一 desktop 基线，降低后续实现成本。

### Changed Files

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

### Validation

- 执行：`go test ./...`
- 结果：通过。
- 执行：`go run github.com/wailsapp/wails/v2/cmd/wails@latest dev`（在 `apps/desktop`）
- 结果：前端依赖安装、前端编译、应用打包均完成，进入 Wails dev 模式。

### Risk / Notes

- 当前仅完成 Phase A 脚手架与目录骨架，尚未接入 `/chat`、快捷键与历史存储逻辑。
- 本次使用 Wails 默认模板，后续阶段需在保持可启动前提下逐步替换为业务实现。
