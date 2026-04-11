# Changelog (Team) — 2026-04-11

## Emotion Detection

### ✅ Changed
- Default emotion detector switched from `LLMDetector` to local `RuleDetector`.
- Added config switch `EMOTION_DETECTOR_MODE` with values:
  - `rule` (default)
  - `llm` (fallback)
- Prompt context now includes both emotion label and intensity:
  - `Emotion: <label> (intensity=<value>)`

### ✅ Why
- Reduce recurring LLM cost for high-frequency classification.
- Improve reliability by removing external dependency from default path.
- Keep fast rollback path (`llm`) if production behavior needs comparison.

### ✅ Scope
- `internal/emotion/detector.go`
- `internal/config/config.go`
- `cmd/server/main.go`
- `internal/prompt/builder.go`
- `.env.example`
- tests in `internal/emotion` and `internal/prompt`

### ✅ Validation
- `go test ./internal/emotion ./internal/prompt ./internal/agent`
- `go test ./...`
- All passed.

### ⚠️ Notes
- Rule detector is heuristic and dictionary-based.
- Accuracy for sarcasm/mixed intent is limited.
- Next iteration can externalize dictionaries and add scoring telemetry.
