# Desktop MVP Phase B 详解：全局热键唤起与启动器窗口显隐链路

## 背景

Phase A 只把 `apps/desktop` 脚手架跑通，缺少 Alfred 式桌面入口最关键的两件事：

1. 任意应用中可通过全局快捷键唤起
2. 唤起后输入框立即可用，支持快速退出

本次围绕这条最短闭环，先把窗口状态管理、热键注册、前后端联动打通，为后续接入 chat/history 打基础。

## 改动点拆解

### 1) 后端增加启动器窗口控制 API

关键路径：

- `apps/desktop/app.go`

新增方法：

- `ShowLauncher()`
- `HideLauncher()`
- `ToggleLauncher()`

并在 `App` 结构体中维护：

- `visible`（当前窗口可见状态）
- `hotkey`/`hotkeyLabel`（当前热键及标识）
- `stopHotkey`/`doneHotkey`（热键监听协程生命周期）

这样可以避免前端直接操纵窗口细节，统一由 Go 端控制窗口行为。

### 2) 启动时注册全局热键 + 回退策略

关键路径：

- `apps/desktop/app.go`
- `apps/desktop/go.mod`
- `apps/desktop/go.sum`

通过引入 `golang.design/x/hotkey`：

- 主热键：`Option+Space`
- 回退热键：`Cmd+Shift+Space`

注册逻辑：

- 启动时先尝试主热键
- 失败后自动回退
- 两者都失败则记录 warning 日志

退出时在 `shutdown` 中释放热键，避免开发态反复重启产生监听残留。

### 3) 启动器前端从模板改为输入态占位 UI

关键路径：

- `apps/desktop/frontend/src/App.tsx`

核心行为改造：

- 监听 `launcher:focus-input` 事件后聚焦输入框
- Esc 采用逐级退出：
  1. 有输入时先清空
  2. 若在搜索态则退回输入态
  3. 最后调用 `HideLauncher()` 隐藏窗口
- 提供“输入态/搜索态”按钮切换（当前为占位）

目标是先建立用户操作节奏，后续只需把占位态替换为真实数据流。

### 4) 生命周期补齐与绑定更新

关键路径：

- `apps/desktop/main.go`
- `apps/desktop/frontend/wailsjs/go/main/App.d.ts`
- `apps/desktop/frontend/wailsjs/go/main/App.js`

改动点：

- `main.go` 新增 `OnShutdown: app.shutdown`
- Wails 绑定移除 `Greet`，改为导出 `ShowLauncher` / `HideLauncher` / `ToggleLauncher`

确保前端调用接口与 Go 端能力一致，避免壳层 API 漂移。

## 关键代码路径

- 窗口状态与热键主逻辑：`apps/desktop/app.go`
- 应用生命周期接线：`apps/desktop/main.go`
- 前端 launcher 占位交互：`apps/desktop/frontend/src/App.tsx`
- 绑定导出更新：
  - `apps/desktop/frontend/wailsjs/go/main/App.d.ts`
  - `apps/desktop/frontend/wailsjs/go/main/App.js`
- 新增依赖声明：
  - `apps/desktop/go.mod`
  - `apps/desktop/go.sum`

## 行为变化

用户视角：

- 应用启动后默认隐藏窗口
- 可通过全局热键唤起/隐藏启动器（若主热键冲突则使用回退热键）
- 唤起后输入框自动聚焦
- Esc 可按“清空 -> 退态 -> 隐藏”逐级退出

工程视角：

- 前端不再依赖模板 `Greet` 流程，转为 launcher 交互骨架
- 窗口控制集中在 Go 端，后续接入业务只需填充状态与请求逻辑

## 测试验证

本次未记录新增自动化测试执行。

建议在 `apps/desktop` 下手动验证以下路径：

1. 启动 `wails dev`
2. 触发全局快捷键，确认窗口可见性切换
3. 确认唤起后输入框自动聚焦
4. 输入任意字符后连续按 Esc，验证逐级退出链路

## 潜在问题与后续优化

1. `Option+Space` 在部分 macOS 输入法配置中可能冲突，当前已做回退但仍需在目标环境验证可用性。
2. 当前“搜索态”只是 UI 占位，无真实历史检索或结果面板。
3. `WindowCenter` 每次显示都执行，后续可按产品交互考虑记忆窗口位置。
4. 后续接入 chat/history 时需补自动化或最小集成验证，避免 UI 行为与数据请求耦合后回归。