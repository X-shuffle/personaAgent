package tooling

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	mcptypes "github.com/mark3labs/mcp-go/mcp"
	"go.uber.org/zap"

	"persona_agent/internal/agent/ports"
	"persona_agent/internal/model"
)

const (
	ToolNameSeparator = "__MCP__"
	encodedPartPrefix = "b64_"
)

// EncodeFunctionName 将 server/tool 名编码为 provider 兼容的函数名。
// 采用 base64url + 分隔符，确保可逆且满足严格命名正则。
func EncodeFunctionName(serverName, toolName string) string {
	return encodeFunctionNamePart(serverName) + ToolNameSeparator + encodeFunctionNamePart(toolName)
}

// DecodeFunctionName 解析并还原编码后的函数名，返回原始 server/tool 名。
func DecodeFunctionName(functionName string) (string, string, error) {
	parts := strings.SplitN(strings.TrimSpace(functionName), ToolNameSeparator, 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid function name %q", functionName)
	}
	serverName, err := decodeFunctionNamePart(parts[0])
	if err != nil {
		return "", "", fmt.Errorf("invalid server name in function name %q: %w", functionName, err)
	}
	toolName, err := decodeFunctionNamePart(parts[1])
	if err != nil {
		return "", "", fmt.Errorf("invalid tool name in function name %q: %w", functionName, err)
	}
	if serverName == "" || toolName == "" {
		return "", "", fmt.Errorf("invalid function name %q", functionName)
	}
	return serverName, toolName, nil
}

func encodeFunctionNamePart(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	return encodedPartPrefix + base64.RawURLEncoding.EncodeToString([]byte(trimmed))
}

func decodeFunctionNamePart(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	if !strings.HasPrefix(trimmed, encodedPartPrefix) {
		return trimmed, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(trimmed, encodedPartPrefix))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(decoded)), nil
}

// ParseCallArgs 解析工具调用参数，并兼容部分上游返回的异常 JSON 形态（如 "{}{...}"）。
func ParseCallArgs(raw string) (map[string]any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]any{}, nil
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(raw), &args); err == nil {
		if args == nil {
			args = map[string]any{}
		}
		return args, nil
	}

	// Some upstream models may prepend an empty object before the real arguments, e.g. "{}{...}".
	if fixed, ok := salvageJSONObject(raw); ok {
		if err := json.Unmarshal([]byte(fixed), &args); err == nil {
			if args == nil {
				args = map[string]any{}
			}
			return args, nil
		}
	}

	return nil, fmt.Errorf("invalid tool call arguments: %s", raw)
}

func salvageJSONObject(raw string) (string, bool) {
	for i := len(raw) - 1; i >= 0; i-- {
		if raw[i] != '{' {
			continue
		}
		candidate := strings.TrimSpace(raw[i:])
		if strings.HasSuffix(candidate, "}") {
			return candidate, true
		}
	}
	return "", false
}

func FormatToolResultText(result *mcptypes.CallToolResult) string {
	if result == nil {
		return "(empty result)"
	}

	textParts := make([]string, 0, len(result.Content))
	for _, c := range result.Content {
		if tc, ok := c.(mcptypes.TextContent); ok {
			textParts = append(textParts, tc.Text)
		}
	}
	joined := strings.TrimSpace(strings.Join(textParts, "\n"))

	if joined == "" && result.StructuredContent != nil {
		if b, err := json.Marshal(result.StructuredContent); err == nil {
			joined = string(b)
		}
	}
	if joined == "" {
		joined = "(empty result)"
	}
	return joined
}

// NormalizeToolCallID 统一 tool_call_id 形态，去除部分 provider 注入的 fc_ 前缀。
func NormalizeToolCallID(raw string) string {
	id := strings.TrimSpace(raw)
	if strings.HasPrefix(id, "fc_") {
		id = strings.TrimPrefix(id, "fc_")
	}
	return id
}

func NormalizeToolCalls(calls []model.LLMToolCall) []model.LLMToolCall {
	if len(calls) == 0 {
		return nil
	}
	out := make([]model.LLMToolCall, len(calls))
	copy(out, calls)
	for i := range out {
		out[i].ID = NormalizeToolCallID(out[i].ID)
	}
	return out
}

// ExecuteSingleCall 执行单个工具调用，并将结果格式化为 LLM 可消费的 tool 消息。
func ExecuteSingleCall(ctx context.Context, round int, call model.LLMToolCall, caller ports.ToolCaller, logger *zap.Logger, timeout time.Duration) model.LLMMessage {
	resultMsg := model.LLMMessage{
		Role:       "tool",
		ToolCallID: NormalizeToolCallID(call.ID),
	}

	serverName, toolName, err := DecodeFunctionName(call.Function.Name)
	if err != nil {
		resultMsg.Content = fmt.Sprintf("tool call decode failed: %v", err)
		return resultMsg
	}

	args, err := ParseCallArgs(call.Function.Arguments)
	if err != nil {
		resultMsg.Content = fmt.Sprintf("tool call args parse failed: %v", err)
		return resultMsg
	}

	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	toolResult, callErr := caller.CallTool(callCtx, serverName, toolName, args)
	if callErr != nil {
		logger.Warn("mcp tool call failed", zap.Int("round", round), zap.String("server", serverName), zap.String("tool", toolName), zap.Error(callErr))
		resultMsg.Content = "mcp tool call failed: " + callErr.Error()
		return resultMsg
	}

	formatted := FormatToolResultText(toolResult)
	if toolResult != nil && toolResult.IsError {
		resultMsg.Content = "mcp tool error: " + formatted
		return resultMsg
	}
	resultMsg.Content = formatted
	return resultMsg
}
