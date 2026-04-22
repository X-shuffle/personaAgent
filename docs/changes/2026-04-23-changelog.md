# 2026-04-23 Changelog

## Entry: desktop 启动器显隐链路收敛、历史浏览增强与消息复制能力

### Summary

- `apps/desktop/app.go` 收敛 launcher 显隐行为：移除发送成功/失败后的自动隐藏策略，保留 Esc、热键与窗口外点击（失焦）三条隐藏路径；并补充顶部居中定位与状态栏生命周期接入。
- `apps/desktop/statusbar_darwin.go` / `apps/desktop/statusbar_stub.go` 增加菜单栏入口实现与非 darwin 空实现。
- `apps/desktop/frontend/src/App.tsx` / `apps/desktop/frontend/src/App.css` 完成 UI 与交互修正：
  - 修复代码块长行溢出（`pre/code` 换行与横向滚动约束）；
  - 助手消息新增复制按钮（调用 Wails Clipboard API）；
  - 失焦隐藏兜底，修复“点击窗口外不隐藏”；
  - 清理未使用字段与样式。
- `apps/desktop/backend/history/search.go` 与 `apps/desktop/backend/history/search_test.go` 支持空关键词 recent 浏览并更新测试。
- `apps/desktop/backend/chat/client.go` 调整超时与错误日志细节；`apps/desktop/app_test.go` 随“成功才写库”策略更新断言。

### Why

- 之前存在“滚动时误隐藏 / 点击外侧不隐藏”的冲突体验，需要把显隐触发链路收敛到可解释、可复现的路径。
- 历史检索需要支持无输入直接浏览 recent，减少操作成本。
- 消息区在长代码行场景会越界，需要保证容器内展示稳定。
- 助手输出需要快捷复制能力，降低二次操作成本。

### Changed Files

- `apps/desktop/app.go`
- `apps/desktop/app_test.go`
- `apps/desktop/main.go`
- `apps/desktop/statusbar_darwin.go`
- `apps/desktop/statusbar_stub.go`
- `apps/desktop/backend/chat/client.go`
- `apps/desktop/backend/history/search.go`
- `apps/desktop/backend/history/search_test.go`
- `apps/desktop/frontend/package.json`
- `apps/desktop/frontend/package-lock.json`
- `apps/desktop/frontend/package.json.md5`
- `apps/desktop/frontend/src/App.tsx`
- `apps/desktop/frontend/src/App.css`
- `apps/desktop/frontend/src/style.css`
- `apps/desktop/frontend/src/features/history/HistorySearchPanel.tsx`
- `apps/desktop/frontend/src/features/history/types.ts`
- `apps/desktop/frontend/src/features/history/useHistorySearch.ts`
- `apps/desktop/README.md`
- `internal/agent/orchestrator.go`

### Validation

- `cd apps/desktop && go test ./...` 通过。
- `cd apps/desktop/frontend && npm run build` 通过（含 TypeScript 编译）。
- 手工验证：
  - 点击窗口外可隐藏；
  - 长代码行不再超出消息容器；
  - 助手消息复制按钮可写入剪贴板。

### Risk / Notes

- `statusbar_darwin.go` 使用 Cocoa 桥接，需在 macOS 真机持续观察右键菜单与多屏定位行为。
- 当前工作区同时包含 `docs/changes/2026-04-22-*` 历史文档草稿，本次未覆盖该文件内容。
- `internal/agent/orchestrator.go` 有少量改动，已随本次提交一并纳入，请在评审时关注其影响范围。
