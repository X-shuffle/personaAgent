# 2026-04-21 Changelog

## Entry: desktop 固定会话、Markdown 渲染与 IME 回车保护加固

### Summary

- `apps/desktop/app.go` 将 desktop 侧 `session_id` 改为固定常量，避免应用重启后切换新会话。
- `apps/desktop/frontend/src/App.tsx` 引入 `react-markdown` 渲染回复内容，支持标题、列表、代码块等 Markdown 展示。
- `apps/desktop/frontend/src/App.tsx` 调整 Enter 提交判定，组合输入期间同时检查 `isComposing`、`nativeEvent.isComposing` 与 `nativeEvent.keyCode===229`，减少输入法回车误发送。
- `apps/desktop/frontend/src/App.css` 增补 `.response` 下 markdown 元素样式，确保渲染可读性。
- `apps/desktop/frontend/package.json` / `package-lock.json` 增加 `react-markdown@^8.0.7` 依赖。

### Why

- 之前 `session_id` 在启动时动态生成，重启后会丢失会话连续性，不符合“先固定会话”的验证诉求。
- 纯文本渲染会丢失后端返回中的 Markdown 结构，影响回复可读性。
- 中文输入法候选确认回车仍会触发发送，需继续加固 Enter 触发条件。

### Changed Files

- `apps/desktop/app.go`
  - 新增 `fixedSessionID` 常量并在 `startup` 使用固定值。
  - 移除直接使用 `uuid.NewString()` 的会话生成逻辑。
- `apps/desktop/go.mod`
  - `github.com/google/uuid` 从直接依赖变为间接依赖（由依赖图变化导致）。
- `apps/desktop/frontend/src/App.tsx`
  - 新增 `ReactMarkdown` 渲染回答。
  - Enter 判定改为读取 `nativeEvent` 并增加 `keyCode===229` 保护。
- `apps/desktop/frontend/src/App.css`
  - 新增 `.response` 下 `p/ul/ol/pre/code` 等样式。
- `apps/desktop/frontend/package.json`
  - 新增 `react-markdown` 依赖。
- `apps/desktop/frontend/package-lock.json`
  - 锁定 `react-markdown` 及其传递依赖。
- `apps/desktop/frontend/package.json.md5`
  - 同步更新哈希。

### Validation

- 执行：`npm run build --prefix apps/desktop/frontend`
  - 结果：通过。
- 执行：`go test ./...`
  - 结果：通过。

### Risk / Notes

- 固定 `session_id` 是当前阶段的短期策略，不区分用户与场景，后续如需多会话需引入可配置会话管理。
- `keyCode` 在类型层面已标记为 deprecated，但该分支用于兼容部分输入法事件上报差异，当前保留以降低误触发概率。
