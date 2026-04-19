package ingestion

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"persona_agent/internal/memory"
	"persona_agent/internal/model"
)

var (
	ErrInvalidInput      = errors.New("invalid ingest input")
	ErrUnsupportedFormat = errors.New("unsupported ingest format")
	ErrNoValidMessages   = errors.New("no valid messages")
	ErrDisabled          = errors.New("ingest disabled")
)

type Request struct {
	SessionID string
	Source    string
	Format    string
	Filename  string
	Data      []byte
	DryRun    bool
}

type Result struct {
	SessionID string
	Source    string
	Accepted  int
	Rejected  int
	Segments  int
	Stored    int
	DryRun    bool
	Warnings  []string
}

type Service interface {
	Ingest(ctx context.Context, req Request) (Result, error)
}

type Config struct {
	Enabled              bool
	SegmentMaxChars      int
	MergeWindowSeconds   int
	EmbedBatchSize       int
	DefaultSource        string
	AllowedExtensions    []string
}

type DefaultService struct {
	embedder memory.Embedder
	store    memory.Store
	cfg      Config
	now      func() time.Time
}

type NoopService struct{}

func (NoopService) Ingest(_ context.Context, _ Request) (Result, error) {
	return Result{}, ErrDisabled
}

func NewService(embedder memory.Embedder, store memory.Store, cfg Config) *DefaultService {
	if cfg.SegmentMaxChars <= 0 {
		cfg.SegmentMaxChars = 500
	}
	if cfg.MergeWindowSeconds <= 0 {
		cfg.MergeWindowSeconds = 90
	}
	if cfg.EmbedBatchSize <= 0 {
		cfg.EmbedBatchSize = 64
	}
	if strings.TrimSpace(cfg.DefaultSource) == "" {
		cfg.DefaultSource = "wechat"
	}
	return &DefaultService{
		embedder: embedder,
		store:    store,
		cfg:      cfg,
		now:      time.Now,
	}
}

type messageRecord struct {
	Speaker   string
	Content   string
	Timestamp int64
}

type segment struct {
	Content   string
	Timestamp int64
}

func (s *DefaultService) Ingest(ctx context.Context, req Request) (Result, error) {
	if !s.cfg.Enabled {
		return Result{}, ErrDisabled
	}
	req.SessionID = strings.TrimSpace(req.SessionID)
	if req.SessionID == "" || len(req.Data) == 0 {
		return Result{}, ErrInvalidInput
	}
	if !isAllowedFilename(req.Filename, s.cfg.AllowedExtensions) {
		return Result{}, ErrUnsupportedFormat
	}
	source := strings.TrimSpace(req.Source)
	if source == "" {
		source = s.cfg.DefaultSource
	}
	format := strings.ToLower(strings.TrimSpace(req.Format))
	if format == "" {
		format = "auto"
	}

	records, parseWarnings, err := parseRecords(req.Data, req.Filename, format, s.now)
	if err != nil {
		return Result{}, err
	}
	cleaned, rejected, cleanWarnings := cleanRecords(records)
	if len(cleaned) == 0 {
		return Result{}, ErrNoValidMessages
	}
	segs := segmentRecords(cleaned, s.cfg.SegmentMaxChars, s.cfg.MergeWindowSeconds)
	if len(segs) == 0 {
		return Result{}, ErrNoValidMessages
	}

	result := Result{
		SessionID: req.SessionID,
		Source:    source,
		Accepted:  len(cleaned),
		Rejected:  rejected,
		Segments:  len(segs),
		Stored:    0,
		DryRun:    req.DryRun,
		Warnings:  append(parseWarnings, cleanWarnings...),
	}
	if req.DryRun {
		return result, nil
	}

	texts := make([]string, 0, len(segs))
	for _, seg := range segs {
		texts = append(texts, seg.Content)
	}
	vectors, err := s.embedInBatches(ctx, texts)
	if err != nil {
		return Result{}, fmt.Errorf("embed ingest segments: %w", err)
	}
	if len(vectors) != len(segs) {
		return Result{}, fmt.Errorf("embed size mismatch")
	}

	memories := make([]model.Memory, 0, len(segs))
	for i, seg := range segs {
		memories = append(memories, model.Memory{
			ID:         newMemoryID(),
			SessionID:  req.SessionID,
			Type:       model.MemoryTypeEpisodic,
			Content:    seg.Content,
			Embedding:  vectors[i],
			Emotion:    "",
			Timestamp:  seg.Timestamp,
			Importance: scoreIngestImportance(seg.Content),
		})
	}
	if err := s.store.Upsert(ctx, memories); err != nil {
		return Result{}, fmt.Errorf("upsert ingest memories: %w", err)
	}
	result.Stored = len(memories)
	return result, nil
}

func (s *DefaultService) embedInBatches(ctx context.Context, texts []string) ([][]float64, error) {
	out := make([][]float64, 0, len(texts))
	batchSize := s.cfg.EmbedBatchSize
	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		vectors, err := s.embedder.Embed(ctx, texts[i:end])
		if err != nil {
			return nil, err
		}
		out = append(out, vectors...)
	}
	return out, nil
}

var (
	reWechatLineA = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\s+([^:：]+)[:：]\s*(.*)$`)
	reWechatLineB = regexp.MustCompile(`^\[(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\]\s+([^:：]+)[:：]\s*(.*)$`)
)

func isAllowedFilename(filename string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	name := strings.TrimSpace(filename)
	if name == "" {
		return true
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	if ext == "" {
		return false
	}
	for _, one := range allowed {
		if strings.ToLower(strings.TrimSpace(one)) == ext {
			return true
		}
	}
	return false
}

func parseRecords(data []byte, filename, format string, now func() time.Time) ([]messageRecord, []string, error) {
	switch format {
	case "wechat_txt", "txt":
		recs := parseWeChatTXT(data, now)
		return recs, nil, nil
	case "wechat_json", "json":
		recs, warnings, err := parseWeChatJSON(data, now)
		if err != nil {
			return nil, nil, err
		}
		return recs, warnings, nil
	case "auto":
		if strings.HasSuffix(strings.ToLower(strings.TrimSpace(filename)), ".json") || json.Valid(data) {
			recs, warnings, err := parseWeChatJSON(data, now)
			if err == nil {
				return recs, warnings, nil
			}
		}
		return parseWeChatTXT(data, now), nil, nil
	default:
		return nil, nil, ErrUnsupportedFormat
	}
}

func parseWeChatTXT(data []byte, now func() time.Time) []messageRecord {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
	out := make([]messageRecord, 0)
	var cur *messageRecord
	flush := func() {
		if cur == nil {
			return
		}
		cur.Content = strings.TrimSpace(cur.Content)
		out = append(out, *cur)
		cur = nil
	}
	for scanner.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "\ufeff"))
		if line == "" {
			continue
		}
		if ts, speaker, content, ok := parseWeChatLine(line); ok {
			flush()
			cur = &messageRecord{Speaker: speaker, Content: content, Timestamp: ts}
			continue
		}
		if cur != nil {
			cur.Content += "\n" + line
			continue
		}
		out = append(out, messageRecord{Speaker: "unknown", Content: line, Timestamp: now().Unix()})
	}
	flush()
	return out
}

func parseWeChatLine(line string) (int64, string, string, bool) {
	matches := reWechatLineA.FindStringSubmatch(line)
	if len(matches) == 0 {
		matches = reWechatLineB.FindStringSubmatch(line)
	}
	if len(matches) != 4 {
		return 0, "", "", false
	}
	t, err := time.ParseInLocation("2006-01-02 15:04:05", matches[1], time.Local)
	if err != nil {
		return 0, "", "", false
	}
	return t.Unix(), strings.TrimSpace(matches[2]), strings.TrimSpace(matches[3]), true
}

func parseWeChatJSON(data []byte, now func() time.Time) ([]messageRecord, []string, error) {
	var anyVal any
	if err := json.Unmarshal(data, &anyVal); err != nil {
		return nil, nil, fmt.Errorf("parse json: %w", err)
	}
	warnings := make([]string, 0)
	items := extractJSONItems(anyVal)
	out := make([]messageRecord, 0, len(items))
	for _, item := range items {
		speaker := firstString(item, "speaker", "from", "sender", "talker", "nickname")
		content := firstString(item, "content", "text", "msg", "message")
		ts := firstTimestamp(item, now)
		if strings.TrimSpace(speaker) == "" {
			speaker = "unknown"
		}
		if strings.TrimSpace(content) == "" {
			warnings = append(warnings, "skipped empty json message")
			continue
		}
		out = append(out, messageRecord{Speaker: speaker, Content: content, Timestamp: ts})
	}
	if len(out) == 0 {
		return nil, warnings, ErrNoValidMessages
	}
	return out, warnings, nil
}

func extractJSONItems(v any) []map[string]any {
	switch x := v.(type) {
	case []any:
		out := make([]map[string]any, 0, len(x))
		for _, one := range x {
			if m, ok := one.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	case map[string]any:
		for _, key := range []string{"messages", "data", "records", "list"} {
			if arr, ok := x[key].([]any); ok {
				out := make([]map[string]any, 0, len(arr))
				for _, one := range arr {
					if m, ok := one.(map[string]any); ok {
						out = append(out, m)
					}
				}
				return out
			}
		}
		return []map[string]any{x}
	default:
		return nil
	}
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		switch t := v.(type) {
		case string:
			return strings.TrimSpace(t)
		}
	}
	return ""
}

func firstTimestamp(m map[string]any, now func() time.Time) int64 {
	for _, k := range []string{"timestamp", "time", "create_time", "datetime"} {
		v, ok := m[k]
		if !ok {
			continue
		}
		switch t := v.(type) {
		case float64:
			return int64(t)
		case int64:
			return t
		case string:
			if ts, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64); err == nil {
				return ts
			}
			if parsed, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(t), time.Local); err == nil {
				return parsed.Unix()
			}
		}
	}
	return now().Unix()
}

func cleanRecords(records []messageRecord) ([]messageRecord, int, []string) {
	out := make([]messageRecord, 0, len(records))
	rejected := 0
	warnings := make([]string, 0)
	for _, r := range records {
		r.Speaker = strings.TrimSpace(r.Speaker)
		r.Content = normalizeContent(r.Content)
		if r.Speaker == "" {
			r.Speaker = "unknown"
		}
		if r.Content == "" {
			rejected++
			continue
		}
		if isLowValueContent(r.Content) {
			rejected++
			continue
		}
		out = append(out, r)
	}
	if rejected > 0 {
		warnings = append(warnings, fmt.Sprintf("%d messages filtered as low-value", rejected))
	}
	return out, rejected, warnings
}

func normalizeContent(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}

func isLowValueContent(s string) bool {
	lower := strings.ToLower(strings.TrimSpace(s))
	if lower == "" {
		return true
	}
	for _, v := range []string{"[图片]", "[语音]", "[表情]", "<image omitted>", "撤回了一条消息", "加入群聊", "移出群聊"} {
		if strings.Contains(lower, strings.ToLower(v)) {
			return true
		}
	}
	if strings.Trim(lower, ".!?~，。！？、…哈 ") == "" {
		return true
	}
	return false
}

func segmentRecords(records []messageRecord, maxChars, mergeWindowSeconds int) []segment {
	if len(records) == 0 {
		return nil
	}
	out := make([]segment, 0, len(records))
	push := func(content string, ts int64) {
		content = strings.TrimSpace(content)
		if content == "" {
			return
		}
		for len(content) > maxChars {
			out = append(out, segment{Content: strings.TrimSpace(content[:maxChars]), Timestamp: ts})
			content = strings.TrimSpace(content[maxChars:])
		}
		if content != "" {
			out = append(out, segment{Content: content, Timestamp: ts})
		}
	}
	currentSpeaker := records[0].Speaker
	currentTS := records[0].Timestamp
	currentContent := formatSegmentLine(records[0])
	for i := 1; i < len(records); i++ {
		r := records[i]
		canMerge := r.Speaker == currentSpeaker && (r.Timestamp-currentTS) <= int64(mergeWindowSeconds) && len(currentContent)+1+len(formatSegmentLine(r)) <= maxChars
		if canMerge {
			currentContent += "\n" + formatSegmentLine(r)
			currentTS = r.Timestamp
			continue
		}
		push(currentContent, currentTS)
		currentSpeaker = r.Speaker
		currentTS = r.Timestamp
		currentContent = formatSegmentLine(r)
	}
	push(currentContent, currentTS)
	return out
}

func formatSegmentLine(r messageRecord) string {
	return strings.TrimSpace(r.Speaker) + ": " + strings.TrimSpace(r.Content)
}

// scoreIngestImportance 为摄入文本计算重要性分数（0~1），用于检索阶段的重要性过滤。
func scoreIngestImportance(content string) float64 {
	text := strings.ToLower(strings.TrimSpace(content))
	if text == "" {
		return 0
	}

	score := 0.4
	if containsAny(text, "我喜欢", "我不喜欢", "偏好", "习惯", "希望") {
		score += 0.18
	}
	if containsAny(text, "计划", "准备", "打算", "目标", "明天", "下周", "deadline", "截至") {
		score += 0.14
	}
	if containsAny(text, "记得", "提醒", "一定", "必须", "承诺") {
		score += 0.1
	}
	if containsAny(text, "今天", "明天", "昨天", "年", "月", "日", "点", "号") {
		score += 0.07
	}
	if len(strings.Fields(text)) <= 5 {
		score -= 0.08
	}
	if containsAny(text, "你好", "谢谢", "好的", "嗯", "哈哈", "ok", "收到") {
		score -= 0.06
	}
	return clamp01(score)
}

// containsAny 判断文本是否包含任一关键词。
func containsAny(text string, keywords ...string) bool {
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}

// clamp01 将分数约束到 [0,1] 区间，保证重要性字段合法。
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func newMemoryID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "00000000-0000-0000-0000-000000000000"
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		buf[0:4],
		buf[4:6],
		buf[6:8],
		buf[8:10],
		buf[10:16],
	)
}
