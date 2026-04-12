# 2026-04-12 Changelog

## Entry: 固定记忆存储为 Qdrant，移除 inmem/off 分支

### Summary
- 将内存存储初始化改为仅创建 Qdrant Store，删除运行时 `MemoryMode` 分支选择。
- 清理 `inmem/off` 相关配置与实现代码，减少分支复杂度。
- 更新示例环境变量，移除 `MEMORY_MODE` 配置项。

### Why
- 当前部署目标已统一为 Qdrant，保留多模式会增加维护成本与配置歧义。
- 删除无用分支后，启动路径更直接，减少“配置正确但行为不一致”的问题。

### Changed Files
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

### Validation
- 执行：`go test ./...`
- 结果：通过

### Risk / Notes
- 行为变化：不再支持 `inmem` / `off` 运行模式，运行时必须提供可用 Qdrant 配置（至少 `QDRANT_URL`）。
- 当前工作区仍存在与本次改动无关的未跟踪文件（如 `docs/hash-embedder-vs-semantic-embedder-qdrant.md` 等），未纳入本条变更说明。
