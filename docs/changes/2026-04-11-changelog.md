# Changelog (Team) — 2026-04-11

> 追加记录文件：同一天的后续变更请继续在本文件追加新条目，不覆盖已有内容。

## Entry — Prompt 时间锚点与记忆时间戳（Asia/Shanghai）

### Summary

- 修复日期推理偏差：在系统提示词中加入当前时间锚点与日期推理约束，避免模型自行假设错误年份。
- 增强记忆可读性：`Memories` 段落输出改为携带时间戳，且统一以 `Asia/Shanghai` 时区展示。

### Why

- 线上现象显示模型会把“明天/后天”解释到错误年份，导致回答与上下文不一致。
- 原始记忆仅有文本内容，缺少时间上下文，模型处理相对时间时稳定性不足。

### Changed Files

- `internal/prompt/builder.go`
  - 增加 `Current time` 注入。
  - 增加日期推理约束文案（基于当前时间与记忆时间，不假设其他年份）。
  - 记忆条目改为带时间戳格式（`YYYY-MM-DD HH:mm:ss +08:00 | content`）。
  - 时间展示统一使用 `Asia/Shanghai`（失败回退 `UTC`）。
- `internal/prompt/builder_test.go`
  - 新增对 `Current time` 与日期约束文案的断言。
  - 新增对记忆时间戳展示的断言（+08:00 格式）。

### Validation

- 已执行：`go test ./internal/prompt`
- 结果：通过
- 线上联调观察：`/chat` 请求中 system prompt 已出现 `Current time` 与带时间戳的 `Memories`，回复日期语义与预期一致。

### Risk / Notes

- 风险较低：仅改提示词构建与测试，不涉及存储协议与检索逻辑。
- 备注：时间显示依赖 `Asia/Shanghai`，若运行环境缺失时区数据会回退到 `UTC`。
