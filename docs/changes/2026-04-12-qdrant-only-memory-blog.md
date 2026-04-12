# 固定记忆层为 Qdrant：删除多存储模式后的代码收敛

## 背景
这次改动的目标很明确：项目内存存储不再支持多模式切换，只保留 Qdrant。此前代码同时支持 `qdrant / inmem / off`，在当前部署策略下已经成为额外复杂度来源。

## 改动点拆解

### 1) 服务启动路径改为单一 Qdrant 初始化
在服务启动时，记忆存储由“按 `MemoryMode` 分支选择”改成“固定创建 Qdrant Store”。

- 路径：`cmd/server/main.go`
- 关键变化：
  - 删除 `switch cfg.MemoryMode` 及 `inmem/off` 分支
  - 保留 `memory.NewQdrantStore(cfg.QdrantURL, cfg.QdrantCollection, cfg.QdrantAPIKey, cfg.MemoryVectorDim)`
  - 启动日志删除 `memory_mode` 字段

### 2) 配置层移除 MemoryMode
配置结构和 env 映射删除 `MemoryMode`，同时移除相关 normalize 逻辑，避免出现“可配但无效”的配置项。

- 路径：`internal/config/config.go`
- 关键变化：
  - 删除 `Config.MemoryMode`
  - 删除 `envConfig.MemoryMode`
  - 删除 `normalizeMemoryMode(...)`

### 3) 示例配置对齐为 Qdrant-only
环境变量示例不再出现 `MEMORY_MODE`，仅保留与 Qdrant 相关的必需参数。

- 路径：`.env.example`
- 关键变化：
  - 删除 `# off | inmem | qdrant`
  - 删除 `MEMORY_MODE=inmem`
  - 将 Qdrant 注释改为直接说明配置段用途

### 4) 删除已废弃实现与测试
既然不再支持 in-memory/noop 路径，对应实现与测试一并删除，避免死代码长期残留。

- 删除文件：
  - `internal/memory/store_inmem.go`
  - `internal/memory/store_inmem_test.go`
  - `internal/memory/noop_service.go`

## 关键代码路径
- `cmd/server/main.go`
- `internal/config/config.go`
- `.env.example`
- `internal/memory/store_inmem.go`（已删除）
- `internal/memory/store_inmem_test.go`（已删除）
- `internal/memory/noop_service.go`（已删除）

## 行为变化
- **之前**：可通过 `MEMORY_MODE` 切换 `qdrant/inmem/off`。
- **现在**：记忆层固定使用 Qdrant，不再支持 `inmem/off`。

这意味着部署和本地运行都需要确保 Qdrant 连接参数可用（特别是 `QDRANT_URL`）。

## 测试验证
- 执行命令：`go test ./...`
- 结果：通过

## 潜在问题与后续优化
- 由于移除了本地 `inmem` 回退，离线场景下的开发体验会更依赖本地 Qdrant 可用性。
- 若未来需要轻量本地开发模式，建议以“独立 dev profile”方式引入，而不是恢复主路径多分支逻辑，避免再次增加运行时分叉。
