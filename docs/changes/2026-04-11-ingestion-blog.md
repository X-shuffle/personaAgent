# 从文件上传到向量入库：Ingestion 全链路落地（2026-04-11）

## 背景
现有系统已经具备聊天与记忆检索能力，但缺少“历史数据批量导入”入口。也就是说，用户手上的聊天记录（txt/json）无法稳定转成可检索的 memory 向量。

本次改动围绕 Data Ingestion 做了一条端到端流水线：
- 文件上传接口（txt/json）
- 数据解析（speaker/timestamp）
- 数据清洗（过滤无效内容）
- 数据结构化（统一 schema）
- 分块（segmentation）
- embedding 封装
- 向量数据库接入
- 写入接口

## 改动点拆解

### 1) 写入接口：`POST /ingest`
关键文件：`internal/api/ingest_handler.go`

- 支持两种输入：
  - `application/json`（`session_id/source/format/data/dry_run`）
  - `multipart/form-data`（`file` + 表单字段）
- 对 ingestion 领域错误做 HTTP 映射：
  - `invalid_input` → 422
  - `unsupported_format` → 400
  - `no_valid_messages` → 422
  - `disabled` → 503

这一步把导入能力统一成一个稳定入口，便于后续接入后台任务、管理台或脚本化导入。

### 2) Ingestion 主流程服务
关键文件：`internal/ingestion/service.go`

`Ingest()` 主流程包含：
1. 输入校验（session_id、data、扩展名白名单）
2. 解析：`txt/json/auto`
3. 清洗：去空、去低价值内容
4. 分块：按 speaker + 时间窗口 + 字符上限聚合
5. embedding：按 batch 调用 embedder
6. 写入：`store.Upsert()` 持久化 memory

`Result` 返回导入统计：`accepted/rejected/segments/stored/warnings`，可直接用于观测导入质量。

### 3) 数据解析与 schema 统一
关键文件：`internal/ingestion/service.go`

- TXT 支持两类行格式（含时间 + 说话人 + 内容），并支持跨行内容拼接。
- JSON 支持数组或对象包装，兼容常见字段别名：
  - speaker：`speaker/from/sender/talker/nickname`
  - content：`content/text/msg/message`
  - time：`timestamp/time/create_time/datetime`
- 统一转为内部 `messageRecord{speaker, content, timestamp}`。

这一步的价值是把异构输入尽量收敛成单一中间结构，后面清洗和分块逻辑就能复用。

### 4) 数据清洗与分块策略
关键文件：`internal/ingestion/service.go`

- 清洗：`normalizeContent()` + `isLowValueContent()`
  - 过滤空白、图片占位、纯语气词等低信息密度内容。
- 分块：`segmentRecords()`
  - 条件：同 speaker 且时间差在 `MergeWindowSeconds` 内且长度不超 `SegmentMaxChars`。
  - 超长文本按 `maxChars` 切段。

该策略偏工程稳健，能快速控制段长和噪声比例，但后续仍可按语义边界继续优化。

### 5) embedding 封装与向量库接入
关键路径：
- `internal/ingestion/service.go`（`embedInBatches` + `store.Upsert`）
- `cmd/server/main.go`（`buildMemoryAndIngestionServices`）

服务复用现有 `memory.Embedder` 与 `memory.Store` 抽象：
- `MEMORY_MODE=qdrant` 时落 Qdrant
- `MEMORY_MODE=inmem` 时落内存
- `MEMORY_MODE=off` 时返回 `NoopService`

这意味着 ingestion 与现有 memory 架构保持一致，没有引入平行技术栈。

### 6) 配置与环境变量
关键文件：`internal/config/config.go`、`.env.example`

新增配置项：
- `INGEST_ENABLED`
- `INGEST_MAX_UPLOAD_BYTES`
- `INGEST_ALLOWED_EXT`
- `INGEST_EMBED_BATCH_SIZE`
- `INGEST_SEGMENT_MAX_CHARS`
- `INGEST_SEGMENT_MERGE_WINDOW_SECONDS`

默认值已内置，便于本地直接启动；生产可按数据规模调整分块与批量参数。

### 7) Qdrant 运维脚本补齐
关键文件：

- `scripts/list_qdrant_sessions.sh`
- `scripts/delete_qdrant_session.sh`
- `scripts/delete_all_qdrant_sessions.sh`

补充 session 维度的列举与删除能力，便于导入验证和回滚清理。

## 关键代码路径
- 接口入口：`internal/api/ingest_handler.go`
- 业务编排：`internal/ingestion/service.go`
- 服务装配：`cmd/server/main.go`
- 配置加载：`internal/config/config.go`
- 请求/响应模型：`internal/model/types.go`
- 接口测试：`internal/api/ingest_handler_test.go`
- 服务测试：`internal/ingestion/service_test.go`

## 行为变化
- 服务新增 `POST /ingest` 路由（此前无该能力）。
- 在 `INGEST_ENABLED=true` 时可直接写入 memory store；`dry_run=true` 时只计算不落库。
- `txt/json` 文件可直接导入，结果可观测（accepted/rejected/segments/stored）。

## 测试验证
已新增测试：
- `internal/api/ingest_handler_test.go`
  - MethodNotAllowed
  - BadJSON
  - JSON_OK
  - MultipartMissingFile
  - ErrorMapping
- `internal/ingestion/service_test.go`
  - TXT_OK
  - JSON_DryRun
  - UnsupportedFileExt
  - Disabled

建议回归命令：
- `go test ./internal/api ./internal/ingestion`
- `go test ./...`

## 潜在问题与后续优化
1. **分块边界优化**：当前是字符级硬切，可引入语义边界（句号/换行/轮次）减少信息断裂。
2. **清洗规则可配置化**：低价值词表应外置，按业务域动态调整。
3. **来源格式插件化**：目前偏 WeChat 语料，后续可抽象 parser registry 支持更多平台导出格式。
4. **导入可观测性**：建议新增 ingest 指标（成功率、过滤率、平均段长、Upsert 耗时）用于线上调优。
