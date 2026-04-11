# 变更详解（Blog）— 2026-04-11

## 背景
这次修复针对一个真实线上问题：
- `/ingest` 已成功写入历史聊天；
- `/chat` 也能召回记忆；
- 但模型在“明天打球”这类相对时间表达上会出现年份漂移（例如错误脑补当前年份）。

根因不是检索失败，而是**提示词缺少稳定时间锚点**，且记忆片段缺少结构化时间展示。

## 改动点拆解

### 1) 给模型显式注入“当前时间”
文件：`internal/prompt/builder.go`

在系统提示词中新增：
- `Current time: ...`
- 行为约束：当用户提及日期/时间时，必须基于当前时间和记忆时间推理，不要假设其他年份。

这一步解决“模型自由脑补当前日期”的问题。

### 2) 记忆展示改为“时间 + 内容”
文件：`internal/prompt/builder.go`

原来 `Memories` 仅输出内容文本；现在若 `memory.Timestamp > 0`，输出为：
- `YYYY-MM-DD HH:mm:ss +08:00 | <memory content>`

这样模型在处理“明天/后天/上周”时有更清晰参照。

### 3) 统一时区为 Asia/Shanghai
文件：`internal/prompt/builder.go`

时间展示统一使用 `Asia/Shanghai`，避免日志中出现 UTC (`Z`) 导致人工排查与业务语义脱节。
若加载时区失败，回退到 `UTC`，保证可用性。

### 4) 测试补齐
文件：`internal/prompt/builder_test.go`

新增/更新断言：
- system prompt 包含 `Current time:`；
- 包含日期推理约束文案；
- memory 条目包含 `+08:00` 的时间戳输出。

## 关键代码路径
- [internal/prompt/builder.go](internal/prompt/builder.go)
- [internal/prompt/builder_test.go](internal/prompt/builder_test.go)

## 行为变化
变更前：
- 模型可能在日期理解上出现年份错位。
- `Memories` 无时间信息，模型对相对时间推理不稳定。

变更后：
- 模型获得明确“当前时间”锚点；
- 记忆包含时间上下文；
- 在同一会话下，日期相关回答更一致、可解释。

## 测试验证
- 命令：`go test ./internal/prompt`
- 结果：通过
- 线上日志验证：`llm http request` 的 system prompt 已带 `Current time` 与带时间戳的 `Memories`。

## 潜在问题与后续优化
- 当前约束仍属于提示词层，存在模型依从度波动。
- 后续可考虑：
  1. 在 memory 内容中加入更明确角色标签标准化（如 `Alice/Bob/User/Assistant`）；
  2. 在编排层增加轻量日期归一化（将“明天”映射为绝对日期后再注入）；
  3. 对日期一致性增加回归测试样例（跨月、跨年、时区边界）。
