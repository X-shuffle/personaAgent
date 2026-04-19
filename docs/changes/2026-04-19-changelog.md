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
