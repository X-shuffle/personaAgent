# 短期记忆回填从 Qdrant 切换到进程内缓存（含独立容量配置）

## 背景

在记忆检索链路里，`internal/memory/service.go` 先做向量召回，再按 `minSimilarity` 做过滤。问题在于：当过滤后一条都不剩时，回答上下文会突然断档。当前需求是将补偿逻辑改为“仅使用进程内短期记忆”，不再依赖 Qdrant 的 recent 查询路径。

## 改动点拆解

### 1) 引入会话级短期缓存（线程安全）

在 `internal/memory/service.go` 的 `DefaultService` 中新增：

- `shortTermSize int`
- `cacheMu sync.RWMutex`
- `shortTermBySess map[string][]model.Memory`

含义：

- 按 `sessionID` 维护最近对话记忆。
- 新记录头插，保证“最新优先”。
- 超过容量按 `shortTermSize` 截断。

### 2) 调整 Retrieve 空结果补偿逻辑

`Retrieve(...)` 现在先走原有向量检索与阈值过滤；当 `out` 为空时：

- 改为调用 `loadShortTerm(sessionID, s.topK)`。
- 返回最近缓存而不是再次调用外部存储。

这样可以满足“相似度过滤失败时仍保留短期连续上下文”的目标。

### 3) StoreTurn 写入时同步维护缓存

`StoreTurn(...)` 构建好 `model.Memory` 后，在持久化前调用 `pushShortTerm(memory)`：

- 保证刚发生的对话可立刻用于回填。
- 即使后续外部存储检索结果不理想，仍有本地短期兜底。

### 4) 容量从 topK 解耦，新增独立配置

`internal/config/config.go` 新增配置项：

- `MEMORY_SHORT_TERM_SIZE`

并在 `cmd/server/main.go` 中透传给：

- `memory.NewService(store, embedder, logger, cfg.MemoryTopK, 0, cfg.MemorySimilarityThreshold, cfg.MemoryShortTermSize)`

默认策略：

- 若 `MEMORY_SHORT_TERM_SIZE <= 0`，回退为 `MemoryTopK`，避免异常配置导致缓存失效。

## 关键代码路径

- [internal/memory/service.go](internal/memory/service.go)
- [internal/memory/service_test.go](internal/memory/service_test.go)
- [internal/config/config.go](internal/config/config.go)
- [cmd/server/main.go](cmd/server/main.go)
- [.env.example](.env.example)

## 行为变化

- 变更前：过滤后为空时，补偿逻辑依赖外部存储路径。
- 变更后：过滤后为空时，仅从进程内短期缓存回填。
- 回填顺序：最新记录在前。
- 回填条数：受 `topK` 与 `shortTermSize` 共同约束。

## 测试验证

执行：

- `go test ./internal/memory ./internal/config ./cmd/server ./internal/ingestion`

重点覆盖：

- `TestServiceRetrieve_FallbackRecentWhenFilteredEmpty`
  - 验证过滤后空结果会回填短期缓存，且顺序为最新在前。
- `TestServiceRetrieve_FallbackUsesShortTermSize`
  - 验证 `shortTermSize=1` 时仅回填最近一条。

## 潜在问题与后续优化

- 当前短期缓存驻留内存，不跨进程、不持久化；在多实例部署时，每个实例只拥有本地会话片段。
- 如果后续需要跨实例一致的“短期记忆兜底”，可考虑引入轻量共享缓存层；但当前需求明确为“程序内部维护”，本次实现已对齐。
