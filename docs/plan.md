# Semantic Embedding Plan (bge-large-zh)

## Context

当前召回不准的根因是：向量由 `HashEmbedder` 生成，不是语义向量。Qdrant 本身没问题，它只是向量存储与检索层。

你已确认：

- 新项目，不需要向后兼容
- 使用 `bge-large-zh`
- 优先本地方案，避免按次付费

## Final approach

采用**本地 embedding 服务 + Qdrant**：

- 模型：`bge-large-zh`
- 调用方式：HTTP 到本地服务（如 Ollama/OpenAI-compatible local endpoint）
- 结果：向量语义化，检索质量提升；不依赖外部计费 API

## Implementation steps

1. 新增本地语义 embedding 客户端

- 新文件：`internal/memory/embedder_http.go`
- 实现 `memory.Embedder` 接口
- 协议：OpenAI-compatible embeddings（`/v1/embeddings`，`model` + `input`）
- 支持批量输入（对 ingestion 生效）
- 增加失败场景错误包装（非 2xx、空向量、格式错误）
- 增加维度校验（与 `MEMORY_VECTOR_DIM` 一致）

1. 配置改为“仅语义 embedding”

- 修改：`internal/config/config.go`
- 增加 embedding 配置：
  - `EMBEDDING_ENDPOINT`（本地服务地址）
  - `EMBEDDING_MODEL`（默认 `bge-large-zh`）
  - `EMBEDDING_API_KEY`（本地可为空）
  - `EMBEDDING_TIMEOUT_SECONDS`（默认 15）
- 移除/废弃 hash 模式分支（不做兼容开关）

1. 启动装配直接使用语义 embedding

- 修改：`cmd/server/main.go`
- `buildMemoryAndIngestionServices` 中不再使用 `NewHashEmbedder`
- 统一改为 `NewHTTPEmbedder(...)`
- memory 与 ingestion 继续复用同一个 embedder

1. 更新示例配置

- 修改：`.env.example`
- 增加本地 `bge-large-zh` 示例：
  - `EMBEDDING_ENDPOINT=http://127.0.0.1:11434/v1/embeddings`（或兼容地址）
  - `EMBEDDING_MODEL=bge-large-zh`

1. 测试

- 新增：`internal/memory/embedder_http_test.go`
  - 正常返回、批量返回
  - 非 2xx
  - 响应格式异常
  - 维度不匹配
- 修改：`internal/config/config_test.go`
  - 新增 embedding 配置解析与默认值测试

## Qdrant side handling

- 保持现有 Qdrant 逻辑（`store_qdrant.go`）
- 重点处理维度：`MEMORY_VECTOR_DIM` 必须与 `bge-large-zh` 输出维度一致
- 若当前 collection 维度不一致，直接新建 collection（例如 `persona_memories_v2`）并重新写入数据

## Critical files

- `internal/memory/embedder_http.go`（new）
- `internal/memory/embedder_http_test.go`（new）
- `internal/config/config.go`
- `internal/config/config_test.go`
- `cmd/server/main.go`
- `.env.example`

## Verification

1. 运行测试

- `go test ./internal/config ./internal/memory ./internal/ingestion`

1. 本地联调

- 启动本地 embedding 服务并加载 `bge-large-zh`
- 配置 `EMBEDDING_ENDPOINT`、`EMBEDDING_MODEL`、`MEMORY_VECTOR_DIM`
- 调用 `/chat` 连续提问同主题与跨主题问题，检查召回相关性

1. 维度验证

- 故意设置错误 `MEMORY_VECTOR_DIM`，应立即报错，避免脏写入 Qdrant
