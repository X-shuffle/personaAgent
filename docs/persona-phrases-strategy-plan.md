# PERSONA_PHRASES Strategy Plan

## Context

当前 `PERSONA_PHRASES` 会在每轮 prompt 中形成强提示，导致“慢慢来/别着急”这类短语高频复用。目标是将其变为低频、自然的人设风格信号，而非每轮重复。

## Final approach

采用最小改动方案：**固定 sparse 门控 + builder 软文案**。

- 固定使用 `sparse`：`fnv32(sessionID + "\n" + message) % 4 == 0`（约 25%）
- 在 orchestrator 决定本轮是否传入 phrases
- builder 把短语段改为可选、自然、避免重复的软约束文案

## Planned implementation scope

1. `internal/config/config.go`

- 不再增加 `PersonaPhrasesStrategy` 与 `PERSONA_PHRASES_STRATEGY`
- 保持配置面最小化，不暴露策略开关

1. `cmd/server/main.go`

- 无需新增策略透传（保持现状）

1. `internal/agent/orchestrator.go`

- 固定使用 sparse 门控逻辑
- `Chat()` 在 `PromptBuilder.Build(...)` 前派生 `promptPersona := p`
- 按 `fnv32(sessionID + "\n" + message) % 4 == 0` 决定保留或清空 `promptPersona.Phrases`

1. `internal/prompt/builder.go`

- 仅在 phrases 非空时追加短语段
- 改成软约束文案（optional / natural / at most one / avoid repetition）

1. `.env.example`

- 不新增 `PERSONA_PHRASES_STRATEGY`（固定 sparse，不暴露开关）

## Test plan

1. `internal/config/config_test.go`

- 无需新增策略枚举解析测试

1. `internal/agent/orchestrator_test.go`

- 校验 sparse 门控下 phrases 的传入/阻断
- 校验同输入决策稳定

1. `internal/prompt/builder_test.go`

- phrases 为空时不出现短语段
- phrases 非空时出现软约束文案
- 不再断言 `Preferred phrases`

## Verification

1. `go test ./internal/config ./internal/agent ./internal/prompt`
1. `go test ./...`
1. 手工 `/chat` 冒烟验证 `off|always|sparse`

## Notes

此处为实施计划落档，暂不包含业务代码变更。
