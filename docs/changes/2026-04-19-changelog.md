# 2026-04-19 Changelog

## Entry: 短期记忆回填改为进程内缓存，并支持独立上限配置

### Summary

- 调整 `internal/memory/service.go`：当向量检索结果经过 `minSimilarity` 过滤后为空时，不再依赖向量库补偿，而是回填当前进程内维护的会话短期记忆。
- 新增短期缓存参数 `shortTermSize`，并通过配置项 `MEMORY_SHORT_TERM_SIZE` 独立控制缓存容量。
- 更新 `cmd/server/main.go` 与 `internal/config/config.go`，完成配置解析与依赖注入。
- 补充 `internal/memory/service_test.go` 用例，覆盖回填顺序与容量上限行为。

### Why

- 用户需求是“短期回填不走 Qdrant”，希望在相似度过滤后仍能返回最近上下文，避免回答上下文突然丢失。
- 将短期缓存容量从 `topK` 中解耦，可单独调节“回填上下文长度”，降低参数耦合带来的调优成本。

### Changed Files

- `.env.example`
  - 新增 `MEMORY_SHORT_TERM_SIZE=3` 示例与说明。
- `cmd/server/main.go`
  - `memory.NewService(...)` 增加 `cfg.MemoryShortTermSize` 传参。
- `internal/config/config.go`
  - `Config` 新增 `MemoryShortTermSize`。
  - `envConfig` 新增 `MEMORY_SHORT_TERM_SIZE` 解析。
  - `Load()` 新增默认回退逻辑：`<=0` 时回退到 `MemoryTopK`。
- `internal/memory/service.go`
  - `DefaultService` 新增线程安全短期缓存结构：`cacheMu + shortTermBySess`。
  - `NewService(...)` 新增 `shortTermSize` 参数。
  - `Retrieve(...)` 在过滤后为空时，回退到 `loadShortTerm(...)`。
  - `StoreTurn(...)` 增加 `pushShortTerm(...)` 写入短期缓存。
- `internal/memory/service_test.go`
  - 适配 `NewService` 新签名。
  - 新增 `TestServiceRetrieve_FallbackRecentWhenFilteredEmpty`。
  - 新增 `TestServiceRetrieve_FallbackUsesShortTermSize`。

### Validation

- 执行：`go test ./internal/memory ./internal/config ./cmd/server ./internal/ingestion`
- 结果：通过

### Risk / Notes

- 短期缓存是进程内状态，服务重启后不会保留；仅作为短期补偿，不替代持久化记忆。
- 回填读取限制目前仍使用 `topK`，实际可返回条数受 `shortTermSize` 与 `topK` 双重约束。

## Entry: 固定 sparse 门控控制 persona 短语注入频率

### Summary (Sparse Persona Phrases)

- 在 `internal/agent/orchestrator.go` 增加 `shouldIncludePersonaPhrases(sessionID, message)`，基于 `fnv32a` 计算 `sessionID + "\n" + message` 后按 `%4==0` 放行，固定约 25% 轮次注入短语。
- 在 `internal/agent/orchestrator.go` 的 `Chat()` 中将传给 builder 的 persona 改为 `promptPersona`，仅在命中门控时保留 `Phrases`，否则清空。
- 在 `internal/prompt/builder.go` 将 `Preferred phrases` 强提示改为 `Optional phrase cues` 软约束，仅在 phrases 非空时输出，并明确“最多一句、自然使用、避免重复”。
- 在 `internal/agent/orchestrator_test.go` 与 `internal/prompt/builder_test.go` 补充/更新测试，覆盖 sparse 行为、确定性与新文案断言。

### Why (Sparse Persona Phrases)

- 当前 `PERSONA_PHRASES` 每轮强提示会导致“慢慢来/别着急”等短语高频复读。
- 目标是保留人设风格信号，但把短语从“每轮强约束”降为“低频自然出现”。

### Changed Files (Sparse Persona Phrases)

- `internal/agent/orchestrator.go`
  - 新增 `shouldIncludePersonaPhrases(...)`。
  - `Chat()` 改为先门控再调用 `PromptBuilder.Build(...)`。
- `internal/agent/orchestrator_test.go`
  - `fakePromptBuilder` 增加 `lastPersona` 记录。
  - 新增 `TestOrchestratorChat_PersonaPhrasesSparseGating`。
  - 新增 `TestShouldIncludePersonaPhrases_Deterministic`。
- `internal/prompt/builder.go`
  - persona 文案改为可选短语提示，且仅在 phrases 非空时追加。
- `internal/prompt/builder_test.go`
  - 更新断言：检查 `Optional phrase cues` 与“Use at most one phrase naturally when it fits”。
  - 校验 phrases 为空时不出现短语段。
- `docs/persona-phrases-strategy-plan.md`
  - 方案调整为固定 sparse（移除 always 与策略配置项）。

### Validation (Sparse Persona Phrases)

- 执行：`go test ./internal/agent ./internal/prompt`
- 执行：`go test ./...`
- 结果：通过

### Risk / Notes (Sparse Persona Phrases)

- 门控输入是 `sessionID + message`，同一输入结果稳定，但不同输入命中率是统计意义上的 25%，单用户短期内可能有波动。
- 当前未增加运行时命中率指标；若后续需要观测实际效果，可补充 debug 级别采样日志。
