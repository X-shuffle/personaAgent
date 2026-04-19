# Phase 5 高级能力落地详解：记忆摘要、重要性评分与人设一致性

## 背景

本次改动聚焦 `Phase 5 — Advanced`，目标是把现有“可用”的对话链路升级为“长期更稳定”：

- 记忆不再只看相似度，还要看信息价值。
- 长对话中通过摘要压缩阶段信息，降低上下文漂移。
- 人设短语注入更贴合情绪语境，减少机械复读。

## 改动点拆解

### 1) 记忆重要性评分（turn + ingest）

#### turn 侧
- 文件：`internal/memory/service.go`
- 入口：`StoreTurn(...)`
- 变化：`Importance` 从固定常量改为 `scoreTurnImportance(userInput, assistantOutput, emotion)`。

评分信号包括：
- 偏好/承诺/计划/时间锚点等高价值信息加分。
- 寒暄、低信息密度内容降分。
- 情绪强度提供额外加权。
- 最终使用 `clamp01` 约束在 `[0,1]`。

#### ingest 侧
- 文件：`internal/ingestion/service.go`
- 入口：`Ingest(...)`
- 变化：摄入文本 `Importance` 改为 `scoreIngestImportance(seg.Content)`，不再固定 `0.4`。

### 2) 周期性摘要记忆写入

- 文件：`internal/memory/service.go`
- 入口：`tryStoreSummary(...)`

流程：
1. `shouldSummarize(sessionID)` 按轮次节奏控制触发。
2. 从 `loadShortTerm(sessionID, summaryWindow)` 获取近期窗口。
3. `buildSummaryContent(...)` 按时间排序并压缩文本。
4. 对摘要做 embedding 后写入 `MemoryTypeSummary`。
5. 摘要写入失败仅告警，不中断主流程（best-effort）。

### 3) 检索阶段限制 summary 占比

- 文件：`internal/memory/service.go`
- 函数：`capSummaryMemories(...)`

作用：避免检索结果被 summary 记忆挤占，保留更多 episodic 细节。

### 4) prompt 侧人设一致性增强

- 文件：`internal/prompt/builder.go`
- 变化：
  - 在 system prompt 增加 `Persona consistency rules`。
  - 记忆渲染拆分为 `Memory summaries` 与 `Episodic memories` 两段。

效果：
- 人设语气/价值观更稳定。
- 模型能区分“阶段摘要”和“具体细节”，减少混用。

### 5) 口头禅门控升级为“哈希 + 情绪”

- 文件：`internal/agent/orchestrator.go`
- 函数：`shouldIncludePersonaPhrases(sessionID, message, emotion)`

行为：
- 基于 `sessionID + message` 的哈希做确定性稀疏注入。
- 对高强度负向情绪提高放行概率。
- 保持稳定可预测，不引入随机抖动。

## 关键代码路径

- `internal/memory/service.go`
  - `StoreTurn(...)`
  - `tryStoreSummary(...)`
  - `shouldSummarize(...)`
  - `scoreTurnImportance(...)`
  - `scoreSummaryImportance(...)`
  - `capSummaryMemories(...)`
  - `buildSummaryContent(...)`

- `internal/ingestion/service.go`
  - `Ingest(...)`
  - `scoreIngestImportance(...)`

- `internal/prompt/builder.go`
  - `Build(...)`

- `internal/agent/orchestrator.go`
  - `Chat(...)`
  - `shouldIncludePersonaPhrases(...)`

## 行为变化

1. 记忆写入的 `importance` 不再是常量，随内容价值和情绪变化。
2. 长会话会周期性生成 summary 记忆，形成“细节 + 概览”并存。
3. 检索结果中 summary 数量受控，减少“只剩摘要”的情况。
4. prompt 明确区分 summary 与 episodic 记忆，增强上下文结构化。
5. 口头禅注入更贴合用户情绪，日常轮次更克制。

## 测试验证

已覆盖并通过：

- `internal/memory/service_test.go`
  - 重要性差异
  - 摘要触发
  - 摘要限流
- `internal/ingestion/service_test.go`
  - 摄入重要性差异
- `internal/prompt/builder_test.go`
  - 一致性规则注入
  - summary/episodic 分区渲染
- `internal/agent/orchestrator_test.go`
  - emotion-aware phrase gating

执行命令：
- `go test ./internal/memory ./internal/ingestion ./internal/prompt ./internal/agent`
- `go test ./...`

## 潜在问题与后续优化

1. 当前评分为启发式规则，不同业务语料可能需要调整关键词和权重。
2. 摘要是规则式压缩，稳定但表达能力有限；后续可评估引入更高质量摘要策略。
3. 当前无运行时开关，回退依赖代码回滚；若线上需要更强弹性，可在后续版本补配置开关。