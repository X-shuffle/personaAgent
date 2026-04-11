# 从 LLM 到本地规则：情绪识别模块改造记录（2026-04-11）

- Commit: `00239dd`
- 主题：默认切换到本地规则情绪识别，并保留 LLM 回退

## 背景

项目原有情绪检测路径是：

`/chat -> ChatHandler -> Orchestrator -> LLMDetector -> 上游模型`

该设计在功能上可用，但在高频场景存在两个问题：

1. 成本偏高：情绪识别属于短文本、固定标签任务，不一定需要每次调用 LLM。
2. 外部依赖强：上游波动会放大到主链路。

本次改造目标：

- 默认使用本地规则分类器（RuleDetector）
- 保留 LLM 作为回退模式
- 不改变 orchestrator 依赖接口
- 保持输出 `label + intensity`

---

## 改造原则

1. **接口稳定**：不改 `emotion.Detector` 接口，最小化调用方变更。
2. **可回退**：通过配置切换 `rule|llm`，出现异常可快速回滚。
3. **可解释**：规则命中与强度计算透明，便于调试与学习。
4. **测试先行**：新增规则测试，确保改动可验证。

---

## 核心改动

## 1) 新增 RuleDetector（本地规则分类）

文件：`internal/emotion/detector.go`

新增内容：

- `RuleDetector` 实现 `Detect(ctx, userInput)`
- 内置关键词词典：`sad/angry/anxious/happy`
- 程度词增强：如“非常/特别/很/太”
- 否定词处理：如“不开心”会降低 happy 并补偿 sad
- 冲突决策：分差小于阈值时按优先级决策（`angry > anxious > sad > happy`）
- 强度计算：按命中分数映射并 clamp 到 `[0, 1]`

同时保留原 `LLMDetector`，用于回退。

---

## 2) 配置增加情绪检测模式

文件：`internal/config/config.go`

新增字段：

- `Config.EmotionDetectorMode`
- `env: EMOTION_DETECTOR_MODE`，默认 `rule`

新增规范化函数：

- `normalizeEmotionMode(mode string)`
- 合法值：`rule`, `llm`
- 非法值回落到 `rule`

保留既有 `EMOTION_DETECT_TIMEOUT_SECONDS`，用于 `llm` 模式。

---

## 3) 启动注入逻辑改为按配置切换

文件：`cmd/server/main.go`

新增：

- `buildEmotionDetector(cfg, llmClient)`

行为：

- `EMOTION_DETECTOR_MODE=rule` -> 注入 `emotion.RuleDetector{}`
- `EMOTION_DETECTOR_MODE=llm` -> 注入 `emotion.LLMDetector{...}`

`Orchestrator` 本身不需要改接口，保持原链路。

---

## 4) Prompt 增加 intensity 信息

文件：`internal/prompt/builder.go`

原格式：

- `Emotion: <label>`

新格式：

- `Emotion: <label> (intensity=<0..1>)`

新增 `normalizeIntensity` 保证强度边界合法。

这让后续回复策略可以同时利用情绪类别和强度。

---

## 5) 环境变量示例更新

文件：`.env.example`

新增：

```env
# rule | llm
EMOTION_DETECTOR_MODE=rule
EMOTION_DETECT_TIMEOUT_SECONDS=20
```

---

## 6) 测试更新

### Emotion 测试

文件：`internal/emotion/detector_test.go`

新增 RuleDetector 用例：

- 空输入 -> neutral
- sad 命中
- 焦虑+愤怒并存时优先级决策
- 否定 happy 场景（不开心）
- 强度 clamp 边界

并保留 LLMDetector 原有测试覆盖。

### Prompt 测试

文件：`internal/prompt/builder_test.go`

更新断言，要求系统提示中包含：

- `Emotion: neutral (intensity=0.00)`

---

## 改造收益

1. **成本下降**：默认不依赖上游模型做情绪识别。
2. **稳定性提升**：本地规则减少外部调用失败面。
3. **性能更稳**：无网络调用，延迟更可控。
4. **工程可控**：保留 `llm` 模式，回退简单。

---

## 局限与后续建议

当前局限：

- 规则词表覆盖受限
- 复杂语义（反讽、多句混合情绪）识别有限
- 强度为启发式计算

后续建议：

1. 将关键词词表配置化（便于运营调参）
2. 增加可观测性日志（命中词、得分、决策路径）
3. 基于真实语料做离线评估（准确率/召回）
4. 未来可加轻量分类模型作为规则补充

---

## 验证

本次改造验证通过：

```bash
go test ./internal/emotion ./internal/prompt ./internal/agent
go test ./...
```

结果：全部通过。

---

## 结论

本次改造以“最小变更 + 可回退”为核心，顺利把情绪识别从默认 LLM 切到本地规则，降低成本并提升链路稳定性；同时通过配置开关保留了工程弹性，适合作为后续情绪理解能力持续迭代的基础版本。
