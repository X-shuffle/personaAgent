# PERSONA_PHRASES Strategy Plan

## Context

当前 `PERSONA_PHRASES` 会在每轮 prompt 中形成强提示，导致“慢慢来/别着急”这类短语高频复用。目标是将其变为低频、自然的人设风格信号，而非每轮重复。

## Final approach

采用最小改动方案：**orchestrator 门控 + builder 软文案**。

- 新增 `PERSONA_PHRASES_STRATEGY=off|always|sparse`
- 在 orchestrator 决定本轮是否传入 phrases
- `sparse` 采用确定性门控：`fnv32(sessionID + "\n" + message) % 4 == 0`（约 25%）
- builder 把 `Preferred phrases` 改为可选、自然、避免重复的软约束文案
- 默认策略为 `sparse`，保留 `always` 兼容旧行为

## Planned implementation scope

1. `internal/config/config.go`

- 增加 `PersonaPhrasesStrategy string`
- 增加 `PERSONA_PHRASES_STRATEGY`（默认 `sparse`）
- 归一化为 `off|always|sparse`，非法值回落 `sparse`

1. `cmd/server/main.go`

- 将 `cfg.PersonaPhrasesStrategy` 传给 orchestrator

1. `internal/agent/orchestrator.go`

- 增加 `PersonaPhrasesStrategy` 字段
- `Chat()` 在 `PromptBuilder.Build(...)` 前派生 `promptPersona := p`
- 按策略清空或保留 `promptPersona.Phrases`

1. `internal/prompt/builder.go`

- 仅在 phrases 非空时追加短语段
- 改成软约束文案（optional / natural / at most one / avoid repetition）

1. `.env.example`

- 增加 `PERSONA_PHRASES_STRATEGY=sparse`
- 说明可选值 `off|always|sparse`

## Test plan

1. `internal/config/config_test.go`

- 校验 `off|always|sparse` 解析与默认值
- 非法值回落 `sparse`

1. `internal/agent/orchestrator_test.go`

- 校验三种策略下 phrases 的传入/阻断
- 校验 `sparse` 同输入决策稳定

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
