# 2026-04-22 Changelog

## Entry: desktop 菜单栏入口、自动隐藏策略与历史最近浏览体验完善

### Summary

- `apps/desktop/statusbar_darwin.go` 新增 macOS 菜单栏状态图标与右键菜单，支持点击显隐启动器与快捷退出。
- `apps/desktop/statusbar_stub.go` 增加非 macOS 平台空实现，保持跨平台编译行为稳定。
- `apps/desktop/app.go` 增加 `DESKTOP_AUTO_HIDE_POLICY` 自动隐藏策略（`never` / `on_success` / `always`），并在发送请求后按结果执行隐藏。
- `apps/desktop/app.go` 调整窗口显示定位逻辑，改为顶部居中并限制在当前屏幕可见区域；同时接入状态栏生命周期管理。
- `apps/desktop/backend/history/search.go` 支持空关键词返回最近消息，前端可在无输入时浏览 recent。
- `apps/desktop/frontend/src/features/history/useHistorySearch.ts` 增加“空输入 + 方向键”触发最近浏览，并在首条结果可用时自动跳转上下文。
- `apps/desktop/frontend/src/App.tsx` 与 `apps/desktop/frontend/src/App.css` 做 Alfred 风格 UI/交互收敛：失焦自动隐藏、历史浮层按需展示、消息区紧凑态与加载态提示。
- `apps/desktop/backend/chat/client.go` 默认请求超时由 15s 调整为 30s，降低慢响应场景下误超时。

### Why

- 当前 desktop 入口仍依赖全局热键，缺少常驻菜单栏入口，不符合 macOS 常驻工具习惯。
- 启动器发送后是否自动收起需要可配置，避免“一刀切”影响不同使用场景。
- 历史搜索在空输入时直接返回空结果，无法支持“最近消息浏览”这一高频路径。
- 近期 UI 需要进一步贴近 Alfred 的“即用即走 + 低视觉噪音”目标，减少操作负担。

### Changed Files

- `apps/desktop/statusbar_darwin.go`
- `apps/desktop/statusbar_stub.go`
- `apps/desktop/app.go`
- `apps/desktop/main.go`
- `apps/desktop/backend/chat/client.go`
- `apps/desktop/backend/history/search.go`
- `apps/desktop/backend/history/search_test.go`
- `apps/desktop/frontend/src/App.tsx`
- `apps/desktop/frontend/src/App.css`
- `apps/desktop/frontend/src/style.css`
- `apps/desktop/frontend/src/features/history/HistorySearchPanel.tsx`
- `apps/desktop/frontend/src/features/history/useHistorySearch.ts`

### Validation

- 本次提交前未新增执行命令；沿用当前工作区已完成联调结果。
- `apps/desktop/backend/history/search_test.go` 已补充/更新空关键词 recent、分页与转义相关用例。

### Risk / Notes

- `statusbar_darwin.go` 依赖 Cocoa 与 Objective-C 桥接，仅在 darwin 构建标签下生效，后续需在真机上验证多显示器与右键菜单行为。
- 自动隐藏策略默认回退为 `on_success`；如用户期望“始终驻留”，需显式设置 `DESKTOP_AUTO_HIDE_POLICY=never`。
- 空关键词 recent 模式会增加 history 查询频次，后续可按数据量评估缓存或分页策略。
