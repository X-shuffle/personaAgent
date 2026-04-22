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

## Entry: desktop 启动器尺寸自适配上限调整与文档集成收口

### Summary (sizing+docs)

- `apps/desktop/app.go` 将 launcher 尺寸从固定感知改为按当前屏幕比例计算，并通过上下限裁剪：宽度上限提升到 `1160`，高度上限提升到 `860`。
- `apps/desktop/app.go` 顶部偏移改为按屏幕高度比例计算并裁剪范围，减少不同分辨率下位置漂移。
- `apps/desktop/frontend/src/App.css` 继续收敛视觉层级：输入框、历史面板、消息卡片的间距/边框/阴影做统一微调，贴近 Alfred 式低噪音风格。
- `README.md`、`CONTRIBUTING.md` 补齐 desktop 开发/调试/构建指引；`.gitignore` 增加 Wails/Node 产物忽略规则。

### Why (sizing+docs)

- 大屏场景下固定尺寸会显得过小，影响可读性与沉浸感，需要按屏幕比例放大并保留边界。
- 顶部定位若使用固定像素偏移，在多屏和不同缩放比下观感不一致，需要按屏幕高度自适配。
- desktop 开发路径此前分散在对话上下文，仓库内文档需要一次性补齐，降低接手成本。

### Changed Files (sizing+docs)

- `.gitignore`
- `CONTRIBUTING.md`
- `README.md`
- `apps/desktop/app.go`
- `apps/desktop/frontend/src/App.css`

### Validation (sizing+docs)

- 代码层面完成静态检查式复核：窗口尺寸/位置计算链路可闭合，`clampInt` 与 `currentScreenSizeLocked` 调用关系一致。
- 文档层面复核：`README.md` 与 `CONTRIBUTING.md` 的 desktop 步骤可直接执行，列表编号使用 `1.` 避免 MD029 告警。

### Risk / Notes (sizing+docs)

- 比例参数当前为经验值（`0.35/0.7`），在超宽屏与小尺寸外接屏仍可能需要后续微调。
- 本次同时暂存了 `docs/changes/2026-04-22-*` 文档文件，commit 将按用户要求并入同一个提交。
