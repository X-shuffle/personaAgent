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
