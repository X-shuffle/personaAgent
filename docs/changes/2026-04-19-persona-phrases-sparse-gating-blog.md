# 固定 sparse 门控的人设短语注入改造详解

## 背景

当前 `PERSONA_PHRASES` 在每轮 prompt 都以强提示出现，导致“慢慢来/别着急”等短语在连续对话中高频复读，风格信号变成了模板化输出。此次改造目标是：保留人设语气，但降低短语显式注入频率，让表达更自然。

## 改动点拆解

### 1) 在 orchestrator 固定 sparse 门控

文件：`internal/agent/orchestrator.go`

- 新增函数 `shouldIncludePersonaPhrases(sessionID, message)`。
- 门控算法：`fnv32a(sessionID + "\n" + message) % 4 == 0`。
- 含义：约 25% 轮次保留 `persona.Phrases`，其余轮次清空短语列表。

这样短语注入变为“低频、稳定、可复现”的采样，而不是每轮都出现。

### 2) 在 Chat 流程中先门控再构建 prompt

文件：`internal/agent/orchestrator.go`

- `Chat()` 中新增 `promptPersona := p`。
- 若未命中门控则执行 `promptPersona.Phrases = nil`。
- `PromptBuilder.Build(...)` 使用 `promptPersona` 而非原始 `p`。

这保证改造只影响“传入 prompt 的短语字段”，不影响 persona 其它属性（如 `Tone`、`Style`、`Values`）。

### 3) 将 builder 文案从强约束改为软提示

文件：`internal/prompt/builder.go`

- 去掉 `Preferred phrases` 这种强指令。
- 改为在 phrases 非空时追加：
  - `Optional phrase cues: ...`
  - `Use at most one phrase naturally when it fits. Avoid repetition across turns.`

这一步把短语从“必须使用”降为“可选提示”，与 sparse 门控形成叠加，减少复读。

## 关键代码路径

- `internal/agent/orchestrator.go`
- `internal/agent/orchestrator_test.go`
- `internal/prompt/builder.go`
- `internal/prompt/builder_test.go`
- `docs/persona-phrases-strategy-plan.md`

## 行为变化

- 变更前：`PERSONA_PHRASES` 每轮都会进入系统提示，短语高频出现。
- 变更后：短语仅在 sparse 门控命中时注入，且文案为“可选自然使用，最多一句，避免重复”。
- 稳定性：同一 `sessionID + message` 的门控结果固定，不会在重复请求中抖动。

## 测试验证

执行命令：

- `go test ./internal/agent ./internal/prompt`
- `go test ./...`

重点测试：

- `internal/agent/orchestrator_test.go`
  - `TestOrchestratorChat_PersonaPhrasesSparseGating`
  - `TestShouldIncludePersonaPhrases_Deterministic`
- `internal/prompt/builder_test.go`
  - 断言存在 `Optional phrase cues`
  - 断言 phrases 为空时不出现短语段

## 潜在问题与后续优化

- 当前 25% 是统计意义，单个用户短会话内可能感知到命中不均匀。
- 门控输入依赖消息文本，表达方式变化会带来命中变化，这是设计预期。
- 如需线上可观测性，可后续补充 debug 级别命中率日志（本次未引入新配置开关）。
