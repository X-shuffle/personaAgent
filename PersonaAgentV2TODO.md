# 🚀 PersonaAgent V2 TODO

---

## 📌 背景说明

当前项目已完成：

* ✅ Agent Orchestrator（基础编排）
* ✅ Memory（RAG + summary + importance）
* ✅ Emotion Detection
* ✅ Data Ingestion
* ✅ MCP Tool 调用能力
* ✅ HTTP API（/chat /ingest）

👉 当前阶段问题：

* ❗ 决策逻辑（routing）不够显式
* ❗ Persona 仍然是静态配置
* ❗ Memory 检索不可解释
* ❗ Tool 使用缺少完整 demo
* ❗ 系统可观测性不足

---

## 🎯 V2 目标

构建一个：

> 🔥 **可解释、可决策、可扩展的 AI Agent 系统**

核心升级：

* Routing（决策层）
* Persona Extraction（从数据生成 persona）
* Retrieval Explainability（记忆可解释）
* Debug Trace（可观测性）
* Tool Demo（对外展示能力）

---

# 🧱 P0（必须优先完成）

---

## 1️⃣ Routing Layer（核心升级 ⭐）

### 🎯 目标

增加显式决策层，判断：

* 是否需要 memory
* 是否需要 tool
* 使用哪个 tool

---

### 📦 任务

* [ ] 新增 `internal/agent/router.go`
* [ ] 定义：

```go
type RouteDecision struct {
    NeedMemory bool
    NeedTool   bool
    ToolName   string
    QueryType  string
    Reason     string
}
```

* [ ] 实现 `DecideRoute(input string) RouteDecision`
* [ ] 在 orchestrator 中接入 router
* [ ] 替换原有隐式判断逻辑

---

### ✅ 验收标准

* 问“你还记得我吗” → memory
* 问“今天比赛怎么样” → tool
* 闲聊 → 不调用 tool
* route decision 可输出（日志或 debug）

---

---

## 2️⃣ Debug Trace（可观测性 ⭐）

### 🎯 目标

让每次对话都有“依据链路”

---

### 📦 任务

* [ ] 定义 `DebugTrace` 结构
* [ ] 支持 `/chat?debug=1`
* [ ] 输出：

```json
{
  "route": {...},
  "emotion": {...},
  "memories": [...],
  "tools": [...]
}
```

* [ ] 限制 debug 输出大小

---

### ✅ 验收标准

* debug=false → 行为不变
* debug=true → 可看到完整链路
* 可用于 demo 展示

---

---

## 3️⃣ 单一 Tool Demo（非常重要 ⭐）

### 🎯 目标

做一个清晰可演示的真实能力

---

### 📦 任务

* [ ] 选择一个：

  * sports（比赛）
  * weather（天气）

* [ ] 实现 tool（或接 MCP）

* [ ] 在 router 中正确触发

* [ ] 在 README 添加 demo 示例

---

### ✅ 验收标准

* 输入实时问题 → 调用 tool
* tool 结果进入 prompt
* 输出符合 persona 风格
* README 有完整流程说明

---

# 🟡 P1（重要升级）

---

## 4️⃣ Persona Extraction（关键升级 ⭐）

### 🎯 目标

让 persona 来自真实数据，而不是静态配置

---

### 📦 任务

* [ ] 新增 `internal/persona/extractor.go`
* [ ] 新增 API：

```
POST /persona/extract
```

* [ ] 输入：历史聊天记录
* [ ] 输出：结构化 persona JSON

---

### 📄 Persona 示例

```json
{
  "tone": "warm",
  "style": "concise",
  "values": ["family"],
  "phrases": ["慢慢来"]
}
```

---

### ✅ 验收标准

* 能从历史数据生成 persona
* persona 能影响后续对话
* 支持 session 级覆盖

---

---

## 5️⃣ Retrieval Reranking（记忆优化）

### 🎯 目标

统一 memory 检索评分逻辑

---

### 📦 任务

* [ ] 新增 `internal/memory/rerank.go`
* [ ] 实现评分：

```text
score = similarity * 0.55 +
        importance * 0.25 +
        recency * 0.20
```

* [ ] 返回 score breakdown

---

### ✅ 验收标准

* memory 按综合评分排序
* debug 输出 score 细节
* 检索稳定性提升

---

# 🔵 P2（进阶能力）

---

## 6️⃣ Semantic Memory（长期记忆）

### 🎯 目标

区分：

* episodic（对话）
* summary（总结）
* semantic（稳定事实）

---

### 📦 任务

* [ ] 定义 semantic memory 结构
* [ ] ingestion 阶段提取稳定事实
* [ ] retrieval 时优先 semantic

---

### 示例

```text
用户喜欢钓鱼
用户不喜欢正式语气
```

---

### ✅ 验收标准

* 问“我喜欢什么” → semantic
* 问“最近怎么样” → episodic/summary

---

---

## 7️⃣ Orchestrator Refactor（结构优化）

### 🎯 目标

避免 orchestrator 过重

---

### 📦 任务

* [ ] 拆分：

```
router.go
tool_executor.go
trace.go
```

* [ ] orchestrator 只负责编排

---

### ✅ 验收标准

* orchestrator < 300 行
* 逻辑职责清晰
* 易扩展

---

---

## 8️⃣ Memory Explainability（加分项）

### 🎯 目标

解释“为什么选中这条记忆”

---

### 📦 任务

* [ ] 每条 memory 输出：

```json
{
  "score": 0.82,
  "reason": "high similarity + recent"
}
```

---

### ✅ 验收标准

* debug 可解释 memory 选择原因

---

---

# 🧠 推荐开发顺序

```text
1. Routing ⭐⭐⭐
2. Debug Trace ⭐⭐⭐
3. Tool Demo ⭐⭐⭐
4. Persona Extraction ⭐⭐
5. Rerank ⭐⭐
6. Semantic Memory ⭐
```

---

# 🚀 最终系统能力

```text
Input
  ↓
Routing（决策）
  ↓
Memory / Tool / Persona
  ↓
Emotion（调节）
  ↓
LLM
  ↓
Response + Debug Trace
```

---

# 🔥 最终目标总结

> PersonaAgent V2 =
> **有决策能力 + 有记忆体系 + 有真实数据驱动 persona + 可解释的 AI Agent**

---
