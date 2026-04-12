# 2026-04-12 Changelog

## Entry: 固定记忆存储为 Qdrant，移除 inmem/off 分支

### Summary（Qdrant-only）

- 将内存存储初始化改为仅创建 Qdrant Store，删除运行时 `MemoryMode` 分支选择。
- 清理 `inmem/off` 相关配置与实现代码，减少分支复杂度。
- 更新示例环境变量，移除 `MEMORY_MODE` 配置项。

### Why（Qdrant-only）

- 当前部署目标已统一为 Qdrant，保留多模式会增加维护成本与配置歧义。
- 删除无用分支后，启动路径更直接，减少“配置正确但行为不一致”的问题。

### Changed Files（Qdrant-only）

- `cmd/server/main.go`
  - `buildMemoryAndIngestionServices` 直接使用 `memory.NewQdrantStore(...)`
  - 启动日志移除 `memory_mode` 字段
- `internal/config/config.go`
  - 删除 `MemoryMode` 配置字段与 env 映射
  - 删除 `normalizeMemoryMode` 逻辑
- `.env.example`
  - 删除 `MEMORY_MODE` 与 `off/inmem/qdrant` 注释
  - 保留并强调 Qdrant 必需配置
- `internal/memory/noop_service.go`（删除）
- `internal/memory/store_inmem.go`（删除）
- `internal/memory/store_inmem_test.go`（删除）

### Validation（Qdrant-only）

- 执行：`go test ./...`
- 结果：通过

### Risk / Notes（Qdrant-only）

- 行为变化：不再支持 `inmem` / `off` 运行模式，运行时必须提供可用 Qdrant 配置（至少 `QDRANT_URL`）。
- 当前工作区仍存在与本次改动无关的未跟踪文件（如 `docs/hash-embedder-vs-semantic-embedder-qdrant.md` 等），未纳入本条变更说明。

## Entry: HTTP Embedding 接入与 LLM 超时配置化

### Summary（HTTP Embedding + Timeout）

- 记忆向量生成从 hash embedder 切换为云端 HTTP embedding，统一通过 `MEMORY_EMBED_*` 配置驱动。
- LLM HTTP 调用超时从硬编码 20s 改为环境变量 `LLM_TIMEOUT_SECONDS` 配置。
- memory 检索链路改为注入 zap logger，并在构造阶段强制要求 logger 非 nil。

### Why（HTTP Embedding + Timeout）

- 需要接入语义 embedding（Gitee `/v1/embeddings`）以提升召回质量，hash embedder 无法满足语义检索场景。
- 超时配置外置后可以按环境灵活调参，避免代码变更才能调整请求时限。
- 统一日志注入能提升检索可观测性，排查“召回命中但未入回答”等问题更直接。

### Changed Files（HTTP Embedding + Timeout）

- `.env.example`
  - 新增 `LLM_TIMEOUT_SECONDS`、`MEMORY_EMBED_ENDPOINT`、`MEMORY_EMBED_API_KEY`、`MEMORY_EMBED_MODEL`、`MEMORY_EMBED_TIMEOUT_SECONDS`
  - `MEMORY_VECTOR_DIM` 示例值调整为 `1024`
- `cmd/server/main.go`
  - `llm.HTTPClient` 使用 `time.Duration(cfg.LLMTimeoutSeconds) * time.Second`
  - `buildMemoryAndIngestionServices` 改为使用 `memory.HTTPEmbedder`
  - `memory.NewService(...)` 注入 `logger`
- `internal/config/config.go`
  - 新增 `LLMTimeoutSeconds` 与 `LLM_TIMEOUT_SECONDS` 解析及默认值回退
  - 新增 `MEMORY_EMBED_*` 解析与 `MEMORY_EMBED_ENDPOINT` 必填校验
- `internal/config/config_test.go`
  - 新增 `TestLoad_LLMTimeoutDefault`
  - 增加 embedding endpoint 必填与 embedding timeout 默认值测试
- `internal/memory/embedder_http.go`（新增）
  - 实现 HTTP embeddings 请求、endpoint 归一化、返回数量/维度强校验
- `internal/memory/embedder_http_test.go`（新增）
  - 覆盖成功路径、非 2xx、坏 JSON、数量不一致、维度不一致与请求头断言
- `internal/memory/service.go`
  - `NewService` 增加 `logger *zap.Logger` 参数并在 nil 时 panic
  - 检索增加 raw/filter/kept 等 debug 日志
- `internal/memory/service_test.go`
  - 适配 `NewService(..., zap.NewNop(), ...)`
- `internal/api/ingest_handler.go`
  - ingest 失败时返回具体错误文本（`internal_error` body）

### Validation（HTTP Embedding + Timeout）

- 执行：`go test ./internal/memory ./cmd/server && go test ./...`
- 执行：`go test ./internal/config ./cmd/server`
- 结果：通过

### Risk / Notes（HTTP Embedding + Timeout）

- 运行依赖变化：启动前必须配置 `MEMORY_EMBED_ENDPOINT`，否则 `config.Load()` 直接返回错误。
- 向量维度必须与 Qdrant collection schema 一致；旧 collection 维度不匹配时需新建 collection 或重建。
- `internal/api/ingest_handler.go` 当前会直接返回内部错误详情，生产环境若需收敛错误暴露建议后续加开关。
