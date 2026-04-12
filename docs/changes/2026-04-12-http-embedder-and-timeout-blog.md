# HTTP Embedding 接入与超时配置化：从 hash 到语义检索的落地改造

## 背景
这次改动围绕两个核心诉求：

1. 记忆检索不再使用 hash embedder，统一切到云端 HTTP embedding（Gitee `/v1/embeddings`）。
2. LLM HTTP 调用超时不要硬编码，和情绪检测超时类似放到 `.env` 可配置。

对应地，代码从“本地 hash + 固定超时”转为“HTTP embedding + 超时配置化 + 检索链路日志化”。

## 改动点拆解

### 1) LLM 超时配置化（`.env` 驱动）

- 文件：`internal/config/config.go`
  - 新增 `Config.LLMTimeoutSeconds`
  - 新增 env 映射：`LLM_TIMEOUT_SECONDS`（默认 `20`）
  - 新增回退逻辑：当配置值 `<= 0` 时回退到 `20`

- 文件：`cmd/server/main.go`
  - `llm.HTTPClient` 注入自定义 `HTTPClient`：
    - `Timeout: time.Duration(cfg.LLMTimeoutSeconds) * time.Second`

- 文件：`.env.example`
  - 新增示例变量：`LLM_TIMEOUT_SECONDS=20`

这使得不同环境可以直接调大/调小超时，不需要改代码。

### 2) 记忆 embedding 切换为 HTTP-only

- 文件：`cmd/server/main.go`
  - `buildMemoryAndIngestionServices` 改为固定构建 `memory.HTTPEmbedder`
  - 参数来自配置：`MEMORY_EMBED_ENDPOINT`、`MEMORY_EMBED_API_KEY`、`MEMORY_EMBED_MODEL`、`MEMORY_EMBED_TIMEOUT_SECONDS`
  - `memory.NewService(...)` 同时注入 logger

- 文件：`internal/config/config.go`
  - 新增 `MEMORY_EMBED_*` 配置解析
  - `MEMORY_EMBED_ENDPOINT` 为空时 fail fast：`memory embed endpoint is required`

- 文件：`internal/memory/embedder_http.go`（新增）
  - 新增 HTTP embedder 实现：
    - endpoint 归一化（支持 `/v1` 自动补 `/v1/embeddings`）
    - 请求头支持 `Authorization: Bearer ...`
    - 固定发送 `X-Failover-Enabled: true`
    - 强校验：非 2xx、返回数量不一致、维度不一致都会报错

这部分是从“可运行”转到“可用于语义检索”的关键。

### 3) 检索可观测性增强：logger 注入 + 构造约束

- 文件：`internal/memory/service.go`
  - `NewService` 新增 `logger *zap.Logger` 参数
  - 构造时 `logger == nil` 直接 `panic("memory logger is nil")`
  - `Retrieve` 内增加 `raw matches / filtered / kept` 等 debug 日志

- 文件：`internal/memory/service_test.go`
  - 全部改为传 `zap.NewNop()` 以适配新构造签名

这解决了此前“为什么某条记忆没有进最终回答”缺少日志证据的问题。

### 4) ingest 错误回传调整

- 文件：`internal/api/ingest_handler.go`
  - 默认 `internal_error` 分支从固定文案改为返回 `err.Error()`

这让 embedding 维度不一致、Qdrant 维度不匹配等问题在接口层更直观。

## 关键代码路径

- `cmd/server/main.go`
- `internal/config/config.go`
- `.env.example`
- `internal/memory/embedder_http.go`
- `internal/memory/embedder_http_test.go`
- `internal/memory/service.go`
- `internal/memory/service_test.go`
- `internal/api/ingest_handler.go`

## 行为变化

- **LLM 超时**：从固定 20s 变为 `LLM_TIMEOUT_SECONDS` 可配置。
- **Memory embedding**：从 hash embedder 变为云端 HTTP embedding。
- **启动约束**：`MEMORY_EMBED_ENDPOINT` 现在是必填。
- **可观测性**：memory 检索阶段新增结构化日志。

## 测试验证

- `go test ./internal/memory ./cmd/server && go test ./...`
- `go test ./internal/config ./cmd/server`

结果均通过。

## 潜在问题与后续优化

1. 维度一致性仍是第一风险点：`MEMORY_VECTOR_DIM` 必须和 embeddings 输出、Qdrant collection schema 一致。
2. `ingest_handler` 直接透传内部错误适合排障期；若面向外部开放接口，建议后续做错误脱敏开关。
3. 当前日志已能看出检索过滤过程，后续可考虑补充 trace_id / request_id 以便跨模块串联。
