# memory 冷启动回补 Qdrant 最近记录并强化 Store 契约

## 背景

当前记忆检索链路是：向量检索命中优先，其次回退进程内 `shortTermBySess`。问题在于进程重启后短期缓存为空，导致会话首次请求可能拿不到历史记忆，出现“冷启动无记忆”的体验断层。

本次改动目标：在不改变主检索逻辑的前提下，为冷启动增加持久层回补路径，并把能力约束提升到接口层，避免实现分叉。

## 改动点拆解

1. 在 `Store` 主接口中引入 recent 能力（强约束）
   - 文件：`internal/memory/interfaces.go`
   - 新增方法：`RecentBySession(ctx, sessionID, limit)`
   - 含义：所有存储实现必须支持按 session 拉取最近记忆。

2. `Retrieve` 增加冷启动兜底路径
   - 文件：`internal/memory/service.go`
   - 逻辑顺序：
     - 向量检索 + 相似度过滤
     - short-term 缓存回退
     - 若 short-term 为空，调用 `s.store.RecentBySession(...)`
   - recent 查询失败时仅告警并降级返回空，避免打断主流程。

3. Qdrant 实现 recent 查询
   - 文件：`internal/memory/store_qdrant.go`
   - 新增 `RecentBySession`：
     - 使用 `points/scroll`
     - `session_id` 过滤
     - `timestamp desc` 排序
   - 增加本地二次 `timestamp` 倒序排序，降低后端返回顺序差异影响。

4. 复用 payload 映射逻辑
   - 文件：`internal/memory/store_qdrant.go`
   - 抽取 `memoryFromPayload`，统一 Search/Recent 的字段解析。

5. 测试补齐
   - 文件：`internal/memory/service_test.go`
     - 新增冷启动命中 recent、short-term 优先、recent 失败降级等用例。
   - 文件：`internal/memory/store_qdrant_test.go`
     - 新增 `RecentBySession` 请求体/排序/映射校验。
   - 文件：`internal/ingestion/service_test.go`
     - fakeStore 补齐 `RecentBySession` 以满足强约束接口。

## 关键代码路径

- `internal/memory/interfaces.go`
  - `Store`：新增 `RecentBySession` 强约束能力。
- `internal/memory/service.go`
  - `Retrieve`：新增冷启动 fallback 到 store recent。
- `internal/memory/store_qdrant.go`
  - `RecentBySession`：Qdrant scroll 查询 + 本地稳定排序。
  - `memoryFromPayload`：统一 payload 转 `model.Memory`。
- `internal/memory/service_test.go`
  - 冷启动与降级行为测试。
- `internal/memory/store_qdrant_test.go`
  - Qdrant recent 查询行为测试。
- `internal/ingestion/service_test.go`
  - fakeStore 接口补齐。

## 行为变化

- 变更前：
  - 重启后 short-term 空，若向量检索无命中，`Retrieve` 返回空记忆。
- 变更后：
  - 重启后 short-term 空时，`Retrieve` 还会从持久化 store 拉最近记忆作为兜底，提升首次请求上下文连续性。

## 测试验证

- `go test ./internal/memory/...`：通过
- `go test ./internal/memory/... ./internal/ingestion/...`：通过

## 潜在问题与后续优化

1. 对 Qdrant 能力的依赖
   - `RecentBySession` 依赖 `points/scroll` 与 `order_by`，已做本地二次排序兜底；后续可补跨版本兼容测试。

2. 回补结果是否应回填 short-term
   - 当前仅返回 recent 结果，不主动回填进程 short-term。后续可评估是否回填，以减少连续请求对持久层压力。

3. 接口强约束的扩展成本
   - 新增 Store 实现必须同时实现 `RecentBySession`；这是有意为之，但会提高新实现接入门槛。
