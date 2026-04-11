# Changelog（团队内部）— 2026-04-11 Ingestion Pipeline

## Summary
本次新增了从原始聊天数据到向量存储的完整 ingestion 链路，并开放 `POST /ingest` 写入接口。支持 `txt/json` 两种输入，提供数据解析、清洗、结构化、分块、embedding、落库能力，同时补充了配置项、接口模型与单测。

## Why
- 解决历史聊天数据无法批量入库的问题，补齐“离线导入 → 在线检索”的前置能力。
- 将数据入口标准化到单一 HTTP 接口，降低导入流程复杂度。
- 为后续记忆检索质量优化（分块策略、过滤策略、数据源治理）打基础。

## Changed Files
- `internal/api/ingest_handler.go`：新增 `/ingest` Handler，支持 JSON 与 multipart 文件上传，做错误码映射。
- `internal/ingestion/service.go`：新增 ingestion 主流程（解析、清洗、分块、批量 embedding、Upsert）。
- `internal/model/types.go`：新增 `IngestRequest` / `IngestResponse`。
- `internal/config/config.go`：新增 ingestion 配置项（开关、上传大小、扩展名、批次、分块参数）。
- `cmd/server/main.go`：注册 `/ingest` 路由并初始化 ingestion service。
- `.env.example`：补充 ingestion 相关环境变量示例。
- `internal/api/ingest_handler_test.go`：新增接口层测试（方法、错误映射、JSON/multipart 场景）。
- `internal/ingestion/service_test.go`：新增服务层测试（TXT/JSON、dry-run、禁用态、扩展名校验）。
- `scripts/list_qdrant_sessions.sh`：新增/完善 Qdrant session 列举脚本。
- `scripts/delete_qdrant_session.sh`：新增/完善单 session 删除脚本。
- `scripts/delete_all_qdrant_sessions.sh`：新增/完善批量 session 删除脚本。

## Validation
- 代码内已补充测试文件：
  - `internal/api/ingest_handler_test.go`
  - `internal/ingestion/service_test.go`
- 建议执行：
  - `go test ./internal/api ./internal/ingestion`
  - `go test ./...`
- 手工验证建议：
  - `POST /ingest`（JSON `data`）验证 `accepted/rejected/segments/stored` 字段。
  - `POST /ingest`（multipart `file`）验证 `txt/json` 上传与 `dry_run=true`。

## Risk / Notes
- `segmentRecords` 目前按“同 speaker + 时间窗 + 最大字符数”合并，超长内容按字符硬切，语义边界可能不稳定（`internal/ingestion/service.go`）。
- 文本清洗中对低价值内容做了规则过滤（如 `[图片]`、空白/语气词），可能误过滤边界内容。
- JSON 解析采用 key 兼容策略（`speaker/from/sender...`），对异构导出格式兼容较宽，但字段语义需持续校准。
- Qdrant 辅助脚本依赖 `.env` 中 `QDRANT_URL/QDRANT_COLLECTION`，生产执行前需确认环境隔离。
